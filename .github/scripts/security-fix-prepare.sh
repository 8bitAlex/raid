#!/usr/bin/env bash
# The first step of Lane 2 (security-fix.yml): decides whether Claude may try
# to adapt the code of a security-update PR to the new dependency version,
# BEFORE any Claude run is paid for. Read-only except one marker comment.
#
# Claude is started only when all of these hold:
#   - the kill switches are on: SECURITY_AUTOMERGE and SECURITY_CLAUDE_FIX
#     are both `true`;
#   - the PR is open, into the default branch, from a Dependabot branch (or a
#     security/DATE branch of Lane 3), and its head is exactly SHA, the
#     commit the caller assessed;
#   - that commit was assessed: the PR carries a comment by github-actions[bot]
#     with the marker <!-- security-eligible:SHA --> (security-escalate.sh and
#     Lane 3 leave it; a person who dispatches this by hand for another PR
#     gets no run);
#   - the PR still passes security-assess.sh in `deps` mode (allowed files,
#     manifests and commits) with CLASSIFY=no;
#   - Claude has not tried on this PR before: a comment by github-actions[bot]
#     with <!-- security-fix-attempt --> means one attempt was made. The
#     marker is left now, before the Claude job, so a failure cannot loop.
#
# Outputs: proceed=true|false, reason=, head_sha=, head_ref=
#
# Environment:
#   PR, SHA            the PR and the commit it was assessed at (required)
#   SECURITY_AUTOMERGE, SECURITY_CLAUDE_FIX
#   BASE_BRANCH        default main
#   GITHUB_REPOSITORY, GH_TOKEN
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pr=${PR:?PR}
sha=${SHA:?SHA}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

stop() {
  echo "Claude does not try: $1"
  out proceed false
  out reason "$1"
  summarize "### Security fix: PR #$pr, no attempt" "$1"
  exit 0
}

[[ $pr =~ ^[0-9]{1,7}$ && $sha =~ ^[0-9a-f]{40}$ ]] || { echo "::error::bad PR or commit"; exit 1; }
[[ ${SECURITY_AUTOMERGE:-} == true ]] || stop "the kill switch SECURITY_AUTOMERGE is not \`true\`"
[[ ${SECURITY_CLAUDE_FIX:-} == true ]] || stop "SECURITY_CLAUDE_FIX is not \`true\` (Claude is not allowed to change code here)"

# The marker comments, by the workflow's own bot only.
bot_comments() { # jq filter on the body
  gh api --paginate "repos/$repo/issues/$pr/comments?per_page=100" \
    --jq ".[] | select(.user.login == \"github-actions[bot]\") | select(.body | $1) | .id"
}
[[ -n $(bot_comments "contains(\"<!-- security-eligible:$sha -->\")") ]] \
  || stop "no assessment of ${sha:0:8} is recorded on the PR (the Lane 1 or Lane 3 run leaves one)"
[[ -z $(bot_comments 'contains("<!-- security-fix-attempt -->")') ]] \
  || stop "Claude already tried on this PR once"

# The PR as it is now, assessed like Lane 1 does, without the security test.
if ! GITHUB_OUTPUT=$work/assess.out CLASSIFY=no EXPECT_SHA=$sha PR=$pr bash "$here/security-assess.sh" >"$work/assess.log" 2>&1; then
  cat "$work/assess.log"
  stop "the PR could not be assessed"
fi
assess_out=$(cat "$work/assess.out")
get() { sed -n "s/^$1=//p" <<<"$assess_out" | tail -n 1; }
[[ $(get eligible) == true ]] || stop "the PR is not eligible: $(get reason)"
[[ $(get mode) == deps ]] || stop "the PR already has a fix on top of the update"

gh pr comment "$pr" --body "<!-- security-fix-attempt -->
Claude is trying to adapt the code to this update (one attempt per PR). If it works, the fix is pushed here and the same CI and checks decide the merge."

out proceed true
out head_sha "$sha"
out head_ref "$(get head_ref)"
summarize "### Security fix: PR #$pr" "Claude will try on \`${sha:0:8}\`."

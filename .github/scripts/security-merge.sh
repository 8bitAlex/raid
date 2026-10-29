#!/usr/bin/env bash
# Merges (squash) one PR, as the workflow, once CI has passed. The last
# gate: the kill switch, and that the PR's head is still the commit that was
# assessed and tested (`--match-head-commit` makes GitHub refuse the merge if
# it moved). It does not decide anything else: security-assess.sh and
# security-wait-ci.sh did, on that same commit.
#
# After the merge, CI is started on the default branch, because a merge made
# with the workflow's token starts no workflow by itself (so build.yml's push
# run, and docs.yml, would not run): build.yml always, docs.yml when the
# change touched site/.
#
# Outputs: merged=true|false, reason=
#
# Environment:
#   PR, SHA        the PR and the head commit to merge (required)
#   TOUCHES_SITE   true: also deploy the docs
#   SECURITY_AUTOMERGE  must be `true`
#   BASE_BRANCH    default main
#   GITHUB_REPOSITORY, GH_TOKEN
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

pr=${PR:?PR}
sha=${SHA:?SHA}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
base=${BASE_BRANCH:-main}

no() {
  echo "not merged: $1"
  out merged false
  out reason "$1"
  summarize "### Security update: PR #$pr not merged" "$1"
  exit 0
}

[[ ${SECURITY_AUTOMERGE:-} == true ]] || no "the kill switch is off: SECURITY_AUTOMERGE is not \`true\`"
[[ $pr =~ ^[0-9]{1,7}$ && $sha =~ ^[0-9a-f]{40}$ ]] || { echo "::error::bad PR or commit"; exit 1; }

now=$(gh api "repos/$repo/pulls/$pr" --jq .head.sha)
[[ $now == "$sha" ]] || no "the PR's head moved on to ${now:0:8} after ${sha:0:8} was tested"

if ! msg=$(gh pr merge "$pr" --squash --delete-branch --match-head-commit "$sha" 2>&1); then
  no "GitHub refused the merge: $msg"
fi
echo "merged #$pr at ${sha:0:8}"
out merged true
summarize "### Security update: PR #$pr merged" "Squash-merged at \`${sha:0:8}\` by the workflow."

# CI on the default branch, which the merge did not start (best effort).
gh workflow run build.yml --ref "$base" || echo "::warning::could not start build.yml on $base"
if [[ ${TOUCHES_SITE:-} == true ]]; then
  gh workflow run docs.yml --ref "$base" || echo "::warning::could not start docs.yml on $base"
fi

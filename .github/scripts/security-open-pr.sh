#!/usr/bin/env bash
# Lane 3, the PR half (security-scan.yml, job "open"): turns the patch of
# security-scan.sh into ONE PR, waits for CI, and merges it. It runs none of
# the code of the patch or of the dependencies; the patch came from a job
# that ran their install scripts and tests, so it is checked here as data:
# it may only change go.mod, go.sum, site/package.json and
# site/package-lock.json, within the size limits, and the manifest checks
# (no new direct dependency, no major jumps, registry only, ...) run again on
# the result.
#
# Branch and title are built from fixed words, the date and a number; the
# package names go only into the PR text, as code spans, after a pattern
# check. The PR is `chore(deps): fix security advisories (N)` on the branch
# `security/YYYY-MM-DD`, labelled dependencies and security.
#
# Then:
#   - no merge unless SECURITY_AUTOMERGE is `true` (then it stops after
#     opening the PR);
#   - needs_fix (the update broke the build): leaves the marker comment and
#     starts security-fix.yml (Lane 2) for Claude, which does the rest;
#   - otherwise the CI is started on the branch (a push with the workflow's
#     token starts nothing), security-assess.sh runs on the PR (the
#     allowlist, commits, manifests), security-wait-ci.sh waits for the
#     required checks, and security-merge.sh squash-merges. A CI failure or a
#     timeout reports to a person: a comment on the PR and ONE issue.
# No second PR while a security/ PR of this automation is open.
#
# Outputs: pr=, result=opened|skipped|merged|reported, reason=
#
# Environment:
#   PATCH, CHANGES   the patch and changes.txt of security-scan.sh
#   NEEDS_FIX        true: the update needs a code change
#   PUSH_TOKEN       optional: a token whose pushes/PRs start workflows
#   PUSH_URL         the remote to push to (default: this repository over
#                    https with the token); for the tests
#   SECURITY_AUTOMERGE, SECURITY_CLAUDE_FIX
#   RUN_URL, BASE_BRANCH, TODAY (default: today, UTC), GITHUB_REPOSITORY,
#   GH_TOKEN, INTERVAL, TIMEOUT_MINUTES
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pol() { node "$here/security-policy.mjs" "$@"; }
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
base=${BASE_BRANCH:-main}
patch=${PATCH:-scan-output/fix.patch}
changes_file=${CHANGES:-scan-output/changes.txt}
needs_fix=${NEEDS_FIX:-false}
today=${TODAY:-$(date -u +%F)}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
patch=$(realpath "$patch")
changes_file=$(realpath "$changes_file" 2>/dev/null || echo "$changes_file")

result() {
  echo "result: $1 ($2)"
  out result "$1"
  out reason "$2"
  summarize "### Security scan: $1" "$2"
  exit 0
}
report() {
  PR=${prn:-} REASON=$1 bash "$here/security-report.sh" || true
  result reported "$1"
}

[[ $today =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "::error::bad date"; exit 1; }
[[ -s $patch ]] || result skipped "there is no patch"
(($(wc -c <"$patch") <= 4194304)) || result skipped "the patch is larger than 4 MiB"

# No second PR of the automation while one is open.
open_pr=$(gh api "repos/$repo/pulls?state=open&per_page=100" --jq '[.[] | select(.head.ref | startswith("security/"))][0].number // empty')
[[ -z $open_pr ]] || result skipped "the security PR #$open_pr is still open"

# --- The patch, as data -------------------------------------------------------
[[ -z $(git status --porcelain) ]] || result skipped "the checkout is not clean"
if ! git apply --check --numstat --summary "$patch" >"$work/stat.txt" 2>"$work/apply.err"; then
  report "the patch does not apply: $(head -c 300 "$work/apply.err")"
fi
if ! pol patch-files deps <"$work/stat.txt" >"$work/files.txt" 2>"$work/err"; then
  report "the patch was refused: $(sed 's/^reject: //' "$work/err" | paste -sd ';' - | sed 's/;/; /g')"
fi
git apply "$patch"
mkdir -p "$work/base"
for f in go.mod site/package.json site/package-lock.json; do
  if git cat-file -e "HEAD:$f" 2>/dev/null; then
    mkdir -p "$work/base/$(dirname "$f")"
    git show "HEAD:$f" >"$work/base/$f"
  fi
done
if ! pol manifests "$work/base" . --no-major >"$work/changes.txt" 2>"$work/err"; then
  git checkout -q -- .
  report "the manifests were refused: $(sed 's/^reject: //' "$work/err" | paste -sd ';' - | sed 's/;/; /g')"
fi

# --- The text: only validated names, as code spans -------------------------
count=0
: >"$work/list.md"
while IFS=$'\t' read -r eco name old new; do
  [[ $eco == go || $eco == npm ]] || continue
  [[ $name =~ ^[A-Za-z0-9@._/+~-]{1,200}$ ]] || continue
  [[ -z $old || $old =~ ^[A-Za-z0-9._+-]{1,60}$ ]] || continue
  [[ -z $new || $new =~ ^[A-Za-z0-9._+-]{1,60}$ ]] || continue
  count=$((count + 1))
  ((count > 60)) || echo "- ${eco}: \`$name\` ${old:+\`$old\` }to \`${new:-removed}\`" >>"$work/list.md"
done <"$changes_file"
((count > 0)) || { git checkout -q -- .; result skipped "the patch changes no package"; }
((count < 1000)) || count=999
title="chore(deps): fix security advisories ($count)"

# --- Branch, commit, push ---------------------------------------------------
branch="security/$today"
token=${PUSH_TOKEN:-${GH_TOKEN:-}}
url=${PUSH_URL:-https://x-access-token:${token}@github.com/${repo}.git}
n=1
while git ls-remote --exit-code --heads "$url" "refs/heads/$branch" >/dev/null 2>&1; do
  n=$((n + 1))
  ((n < 10)) || result skipped "too many branches for $today"
  branch="security/$today-$n"
done
git checkout -q -b "$branch"
git add -- go.mod go.sum site/package.json site/package-lock.json
bot_git commit -q -m "$title" -m "Automated by the security-scan workflow: known vulnerabilities fixed without a breaking change (see the pull request)."
sha=$(git rev-parse HEAD)
if ! git push -q "$url" "HEAD:refs/heads/$branch" 2>"$work/push.err"; then
  err=$(head -c 300 "$work/push.err")
  [[ -z $token ]] || err=${err//"$token"/***}
  report "the push of $branch was refused: $err"
fi

# --- The PR -----------------------------------------------------------------
{
  echo "Known vulnerabilities in the dependencies, fixed without a breaking change by the security-scan workflow (\`npm audit fix\` limited to the vulnerable packages; \`go get module@fixed\` and \`go mod tidy\`). The docs site builds, and for Go the build and tests pass, in the job that made this change."
  echo
  head -n 60 "$work/list.md"
  ((count <= 60)) || echo "- and $((count - 60)) more"
  echo
  echo "Merged automatically only when the repository's CI passes on this exact commit and \`SECURITY_AUTOMERGE\` is \`true\`. Vulnerabilities with a breaking fix are left to a person. See docs/AUTOMATION.md, \"Security updates\"."
} >"$work/body.md"
gh label create security --color B60205 --description "Security updates and advisories" >/dev/null 2>&1 || true
if ! url_out=$(GH_TOKEN=$token gh pr create --base "$base" --head "$branch" --title "$title" --body-file "$work/body.md" --label dependencies --label security 2>&1); then
  # Often: Settings > Actions > General > "Allow GitHub Actions to create and approve pull requests" is off.
  gh api -X DELETE "repos/$repo/git/refs/heads/$branch" >/dev/null 2>&1 || true
  report "the PR could not be created ($(head -c 300 <<<"$url_out")). If it says GitHub Actions is not permitted to create pull requests, turn on Settings > Actions > General > Workflow permissions > \"Allow GitHub Actions to create and approve pull requests\"."
fi
prn=$(grep -oE '/pull/[0-9]+' <<<"$url_out" | tail -n 1 | grep -oE '[0-9]+' || true)
[[ $prn =~ ^[0-9]+$ ]] || report "the PR was created but its number is unknown"
out pr "$prn"
echo "opened #$prn on $branch at ${sha:0:8}"

# --- Merge, or hand over ----------------------------------------------------
if [[ ${SECURITY_AUTOMERGE:-} != true ]]; then
  result opened "PR #$prn opened; not merged: the kill switch SECURITY_AUTOMERGE is not \`true\`"
fi

gh pr comment "$prn" --body "<!-- security-eligible:$sha -->
Opened by the security-scan workflow. Checked: only the dependency manifests changed, within the size limits." >/dev/null

if [[ $needs_fix == true ]]; then
  if [[ ${SECURITY_CLAUDE_FIX:-} == true ]]; then
    gh workflow run security-fix.yml --ref "$base" -f "pr=$prn" -f "sha=$sha"
    result opened "PR #$prn opened; the update needs a code change, so security-fix.yml was started for Claude"
  fi
  report "the update in PR #$prn needs a code change, and SECURITY_CLAUDE_FIX is not \`true\`"
fi

# The workflow's own push started no CI.
if [[ -z ${PUSH_TOKEN:-} ]]; then
  gh workflow run build.yml --ref "$branch"
  gh workflow run codecov.yml --ref "$branch"
fi

if ! GITHUB_OUTPUT=$work/assess.out CLASSIFY=no EXPECT_SHA=$sha PR=$prn bash "$here/security-assess.sh" >"$work/assess.log" 2>&1; then
  cat "$work/assess.log"
  report "the assessment of PR #$prn failed"
fi
assess_out=$(cat "$work/assess.out")
get() { sed -n "s/^$1=//p" <<<"$assess_out" | tail -n 1; }
[[ $(get eligible) == true ]] || report "PR #$prn is not eligible: $(get reason)"

GITHUB_OUTPUT=$work/ci.out SHA=$sha PR=$prn REQUIRED=$(get required) bash "$here/security-wait-ci.sh" >"$work/ci.log" 2>&1 || true
cat "$work/ci.log"
ci=$(sed -n 's/^ci=//p' "$work/ci.out" | tail -n 1)
ci_reason=$(sed -n 's/^reason=//p' "$work/ci.out" | tail -n 1)
case $ci in
  green) ;;
  moved) result skipped "PR #$prn changed under the run: $ci_reason" ;;
  *) report "CI did not pass on PR #$prn ($ci): $ci_reason" ;;
esac

GITHUB_OUTPUT=$work/merge.out PR=$prn SHA=$sha TOUCHES_SITE=$(get touches_site) bash "$here/security-merge.sh"
if [[ $(sed -n 's/^merged=//p' "$work/merge.out" | tail -n 1) == true ]]; then
  result merged "PR #$prn (${count} packages) was squash-merged after CI passed"
fi
report "PR #$prn was not merged: $(sed -n 's/^reason=//p' "$work/merge.out" | tail -n 1)"

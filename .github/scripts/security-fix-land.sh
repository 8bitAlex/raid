#!/usr/bin/env bash
# The last part of Lane 2 (security-fix.yml): Claude changed a checkout of
# the PR's head and produced a PATCH; nothing Claude wrote is trusted. This
# script (which runs no code from the PR and none of Claude's) checks the
# patch, applies it to a clean checkout of the same commit, pushes it to the
# PR's branch as one commit, waits for CI, runs the same assessment as Lane 1
# (now in `fix` mode), and only then merges.
#
# The patch is refused when it: is empty (then there is nothing to land:
# Claude found no change to make), is over 1 MiB, touches anything outside
# src/, main.go, site/docs/, README.md and llms.txt (so nothing under
# .github/, and not go.mod, go.sum, site/package.json, site/package-lock.json:
# a fix may not add or change dependencies), is binary, a symlink, an
# executable file, a rename or a mode change, changes more than 600 lines or
# 20 files, or changes src/ without raising the version in
# src/resources/app.properties (and nothing else in that file): CI's
# version-check job fails a PR that touches src/ otherwise, and a dispatched
# CI run does not check it, so security-assess.sh does.
#
# Outputs: result=merged|no-change|rejected|ci-red|ci-timeout|not-merged|
# moved, reason=
#
# Environment:
#   PR, SHA           the PR and the commit Claude worked from (required)
#   PATCH             the patch file (default fix.patch)
#   PR_DIR            a checkout of that commit (default pr)
#   PUSH_TOKEN        optional: a token that makes the push start workflows
#                     (then CI starts by itself; otherwise it is dispatched)
#   PUSH_URL          the remote to push to (default: this repository over
#                     https with the token); for the tests
#   SECURITY_AUTOMERGE, SECURITY_CLAUDE_FIX
#   RUN_URL, BASE_BRANCH, GITHUB_REPOSITORY, GH_TOKEN
#   INTERVAL, TIMEOUT_MINUTES   for security-wait-ci.sh
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pr=${PR:?PR}
sha=${SHA:?SHA}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
patch=${PATCH:-fix.patch}
dir=${PR_DIR:-pr}
base=${BASE_BRANCH:-main}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
patch=$(realpath "$patch" 2>/dev/null || echo "$patch")

result() {
  echo "result: $1 ($2)"
  out result "$1"
  out reason "$2"
  summarize "### Security fix: PR #$pr $1" "$2"
  exit 0
}

[[ $pr =~ ^[0-9]{1,7}$ && $sha =~ ^[0-9a-f]{40}$ ]] || { echo "::error::bad PR or commit"; exit 1; }
[[ ${SECURITY_AUTOMERGE:-} == true && ${SECURITY_CLAUDE_FIX:-} == true ]] || result rejected "a kill switch is off"

[[ -s $patch ]] || result no-change "Claude changed nothing: either CI's failure could not be reproduced, or no change was found"
(($(wc -c <"$patch") <= 1048576)) || result rejected "the patch is larger than 1 MiB"

cd "$dir"
[[ $(git rev-parse HEAD) == "$sha" ]] || result rejected "the checkout is not at ${sha:0:8}"
[[ -z $(git status --porcelain) ]] || result rejected "the checkout is not clean"

# What the patch touches, by the same rules as a PR's files.
if ! git apply --check --numstat --summary "$patch" >"$work/stat.txt" 2>"$work/apply.err"; then
  result rejected "the patch does not apply: $(head -c 300 "$work/apply.err")"
fi
if ! res=$(node "$here/security-policy.mjs" patch-files fix <"$work/stat.txt" 2>"$work/err"); then
  result rejected "$(sed 's/^reject: //' "$work/err" | paste -sd ';' - | sed 's/;/; /g')"
fi
touches_src=$(sed -n 's/^touches_src=//p' <<<"$res")

git apply "$patch"
git add -A
# What is staged is exactly what the patch lists.
want=$(sed -n 's/^files=//p' <<<"$res" | tr ',' '\n' | sort)
have=$(git diff --cached --name-only | sort)
[[ $want == "$have" ]] || result rejected "the patch changed other files than it lists"

if [[ $touches_src == true ]]; then
  # Only the version line of app.properties, and it must have changed.
  changed=$(git diff --cached -U0 -- src/resources/app.properties | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' || true)
  if [[ $(grep -c . <<<"$changed") != 2 ]] || ! grep -qE '^-version=[0-9.]+$' <<<"$changed" || ! grep -qE '^\+version=[0-9]+\.[0-9]+\.[0-9]+$' <<<"$changed"; then
    result rejected "src/ changed without a version bump (and only that) in src/resources/app.properties"
  fi
fi

# One commit on top of the PR's head. Its subject is what security-policy.mjs
# knows as this automation's fix.
bot_git commit -q -m "fix(deps): adapt code to the dependency update" \
  -m "Automated fix (Claude, security-fix workflow) so CI passes with the updated dependency. It changes no dependency and nothing under .github/."

token=${PUSH_TOKEN:-${GH_TOKEN:-}}
url=${PUSH_URL:-https://x-access-token:${token}@github.com/${repo}.git}
gh_pr="repos/$repo/pulls/$pr"
ref=$(gh api "$gh_pr" --jq .head.ref)
[[ $ref =~ ^dependabot/(go_modules|npm_and_yarn)/[A-Za-z0-9._/@+-]{1,200}$ || $ref =~ ^security/[0-9]{4}-[0-9]{2}-[0-9]{2}(-[0-9]{1,2})?$ ]] \
  || result rejected "the PR's branch is not one this automation may push to"
if ! git push -q --force-with-lease="refs/heads/$ref:$sha" "$url" "HEAD:refs/heads/$ref" 2>"$work/push.err"; then
  # (The token is in the remote's URL: never show it.)
  err=$(head -c 300 "$work/push.err")
  [[ -z $token ]] || err=${err//"$token"/***}
  result moved "the push to $ref was refused (the branch moved, or a rule blocked it): $err"
fi
new=$(git rev-parse HEAD)
echo "pushed ${new:0:8} to $ref"

# The API takes a moment to show the new head.
for _ in 1 2 3 4 5 6 7 8 9 10 11 12; do
  [[ $(gh api "$gh_pr" --jq .head.sha) == "$new" ]] && break
  sleep "${INTERVAL:-5}"
done

# The same assessment as Lane 1, now with the fix on top.
if ! GITHUB_OUTPUT=$work/assess.out CLASSIFY=no EXPECT_SHA=$new PR=$pr bash "$here/security-assess.sh" >"$work/assess.log" 2>&1; then
  cat "$work/assess.log"
  result rejected "the assessment of the fixed PR failed"
fi
assess_out=$(cat "$work/assess.out")
get() { sed -n "s/^$1=//p" <<<"$assess_out" | tail -n 1; }
[[ $(get eligible) == true ]] || result rejected "the fixed PR is not eligible: $(get reason)"

# A push made with the workflow's token starts no workflow: start the CI.
if [[ -z ${PUSH_TOKEN:-} ]]; then
  gh workflow run build.yml --ref "$ref"
  gh workflow run codecov.yml --ref "$ref"
fi

GITHUB_OUTPUT=$work/ci.out SHA=$new PR=$pr REQUIRED=$(get required) bash "$here/security-wait-ci.sh" >"$work/ci.log" 2>&1 || true
cat "$work/ci.log"
ci=$(sed -n 's/^ci=//p' "$work/ci.out" | tail -n 1)
ci_reason=$(sed -n 's/^reason=//p' "$work/ci.out" | tail -n 1)
case $ci in
  green) ;;
  red) result ci-red "CI failed with Claude's fix: $ci_reason" ;;
  timeout) result ci-timeout "$ci_reason" ;;
  moved) result moved "$ci_reason" ;;
  *) result ci-red "CI could not be read: $ci_reason" ;;
esac

GITHUB_OUTPUT=$work/merge.out PR=$pr SHA=$new TOUCHES_SITE=$(get touches_site) bash "$here/security-merge.sh"
if [[ $(sed -n 's/^merged=//p' "$work/merge.out" | tail -n 1) == true ]]; then
  result merged "the fix was pushed (${new:0:8}), CI passed and the PR was squash-merged"
fi
result not-merged "$(sed -n 's/^reason=//p' "$work/merge.out" | tail -n 1)"

#!/usr/bin/env bash
# Waits for the repository's CI to finish on one commit, reading check runs
# and commit statuses through the API (read-only), and says whether it
# passed. The merge never happens on anything but `ci=green`.
#
# green: every required check run exists, completed and succeeded, and
#        nothing else that ran (check run or status) failed;
# red:   a required check did not succeed, or any check or status failed;
# timeout: still waiting after TIMEOUT_MINUTES;
# moved: the PR's head is no longer SHA (a new run will look at the new one).
# The checks of this automation's own jobs ("Security: ...") are ignored.
# CodeQL, when it has started, is waited for as well: the ruleset wants its
# results before a merge into the default branch.
#
# Environment:
#   SHA              the commit (required)
#   REQUIRED         the check runs that must succeed, comma separated
#   PR               the pull request: also stop when its head moves
#   WATCH            checks waited for when present, default CodeQL
#   TIMEOUT_MINUTES  default 45
#   INTERVAL         seconds between looks, default 30
#   GITHUB_REPOSITORY, GH_TOKEN
# Outputs: ci=green|red|timeout|moved, reason=
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
sha=${SHA:?SHA}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
required=${REQUIRED:-build (ubuntu-latest)}
watch=${WATCH:-CodeQL}
pr=${PR:-}
timeout_min=${TIMEOUT_MINUTES:-45}
interval=${INTERVAL:-30}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

[[ $sha =~ ^[0-9a-f]{40}$ ]] || { echo "::error::not a commit: $sha"; exit 1; }
[[ -z $pr || $pr =~ ^[0-9]{1,7}$ ]] || { echo "::error::not a PR number: $pr"; exit 1; }

finish_with() {
  echo "CI on ${sha:0:8}: $1 ($2)"
  out ci "$1"
  out reason "$2"
  summarize "### Security update: CI on \`${sha:0:8}\`" "$1: $2"
  exit 0
}

deadline=$((SECONDS + timeout_min * 60))
while :; do
  if [[ -n $pr ]]; then
    now=$(gh api "repos/$repo/pulls/$pr" --jq .head.sha)
    [[ $now == "$sha" ]] || finish_with moved "the PR's head is now ${now:0:8}"
  fi
  gh api --paginate "repos/$repo/commits/$sha/check-runs?per_page=100" --jq '.check_runs[] | {name, status, conclusion}' >"$work/runs.json"
  gh api "repos/$repo/commits/$sha/status" --jq '.statuses[] | {context, state}' >"$work/statuses.json"
  result=$(node "$here/security-policy.mjs" ci "$required" "$watch" "$work/runs.json" "$work/statuses.json")
  state=$(sed -n 's/^ci=//p' <<<"$result")
  reason=$(sed -n 's/^reason=//p' <<<"$result")
  case $state in
    green | red) finish_with "$state" "$reason" ;;
  esac
  if ((SECONDS >= deadline)); then
    finish_with timeout "still waiting after $timeout_min minutes: $reason"
  fi
  echo "waiting: $reason"
  sleep "$interval"
done

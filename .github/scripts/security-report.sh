#!/usr/bin/env bash
# Says, once, that a security update needs a person: a comment on the PR
# (when there is one) and ONE issue labelled `security` with the reason. An
# issue for the same PR (or, without one, the rolling "Security scan needs
# attention" issue) that is still open gets a comment instead of a second
# issue, so a problem that repeats stays one issue.
#
# It writes only: one comment on the PR, one issue or one comment on it, and
# the `security` label (created if the repository has none). The title is
# built from fixed words and the validated PR number; the reason is cut,
# stripped of control characters and put in a code block.
#
# Environment:
#   REASON    why a person is needed (required)
#   PR        the pull request number (optional)
#   RUN_URL   the run, linked in the text
#   GITHUB_REPOSITORY, GH_TOKEN
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

pr=${PR:-}
reason=${REASON:?REASON}
run_url=${RUN_URL:-}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if [[ -n $pr && ! $pr =~ ^[0-9]{1,7}$ ]]; then
  echo "::error::not a PR number: $pr"
  exit 1
fi

# The reason as plain text: no control characters, at most 1500 characters,
# no backtick fences that could end the block.
clean=$(printf '%s' "$reason" | LC_ALL=C tr -d '\000-\010\013-\037\177' | head -c 1500 | sed 's/```/` ` `/g')
link=
[[ $run_url =~ ^https://github\.com/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+/actions/runs/[0-9]+$ ]] && link=" ([run]($run_url))"

if [[ -n $pr ]]; then
  title="Security update needs attention: PR #$pr"
else
  title="Security scan needs attention"
fi

{
  echo "**The automatic security update stopped and needs a person**$link."
  echo
  echo '```text'
  echo "$clean"
  echo '```'
  echo
  echo "Nothing was merged by this step. See docs/AUTOMATION.md, \"Security updates\", for what each lane does and how to turn it off."
} >"$work/body.md"

gh label create security --color B60205 --description "Security updates and advisories" >/dev/null 2>&1 || true

if [[ -n $pr ]]; then
  gh pr comment "$pr" --body-file "$work/body.md"
fi

existing=$(gh issue list --state open --label security --search "\"$title\" in:title" --json number,title \
  --jq "[.[] | select(.title == \"$title\")][0].number // empty" 2>/dev/null || true)
if [[ $existing =~ ^[0-9]+$ ]]; then
  gh issue comment "$existing" --body-file "$work/body.md"
  out issue "$existing"
  summarize "### Security update: needs a person" "Commented on the open issue #$existing."
else
  gh issue create --title "$title" --label security --body-file "$work/body.md"
  out issue new
  summarize "### Security update: needs a person" "Opened an issue: $title"
fi

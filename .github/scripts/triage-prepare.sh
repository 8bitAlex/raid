#!/usr/bin/env bash
# The first step of the issue triage (triage.yml): decides whether this run
# may triage the issue, and gathers what Claude reads into triage-input/,
# with read-only GitHub calls and no Claude. The folder is uploaded as an
# artifact for the triage job.
#
#   issue.json     the issue: number, title, body, labels, state, author,
#                  author_is_bot, created_at (e-mail addresses withheld)
#   issues.json    every open issue and those closed in the last 90 days:
#                  number, title, state, labels, closedAt (for duplicates)
#   releases.txt   the latest 15 release tags with their dates
#   version.txt    the version in src/resources/app.properties of the code
#                  Claude reads (the default branch)
#   history.txt    the commit subjects of the checked-out branch, since
#                  Claude may not run git
#
# raid is a public repository: the issue text comes from strangers and every
# run costs money. So this script is the second lock after the job's `if`:
#   - an `issues` event is triaged only when the person who caused it has
#     write access to the repository (GitHub permission admin or write, the
#     same test claude-code-action applies): for `opened`, the author of the
#     issue, who must also be an OWNER, MEMBER or COLLABORATOR; for
#     `labeled`, the person who added the label `triage` (a maintainer
#     asking for an outsider's issue to be triaged). Anything else stops.
#   - bots, and issues labelled triaged, dependencies or security, are
#     skipped (by hand, `workflow_dispatch`, only `triaged` and a closed
#     issue stop the run, unless FORCE is true).
#
# Writes proceed=true|false and reason= to GITHUB_OUTPUT: a run that has
# nothing to triage stops quietly. Nothing is printed from the issue's text,
# only counts, and nothing is ever written to GitHub.
#
# Environment:
#   ISSUE         the issue number (required)
#   EVENT_NAME    issues (opened or labeled) or workflow_dispatch (by hand)
#   EVENT_ACTION  opened | labeled (for issues)
#   LABEL         the label that was added (for labeled)
#   ACTOR         the login of the person who opened the issue or added the
#                 label (the event's sender)
#   ASSOCIATION   the issue author's author_association (for opened)
#   FORCE         true: triage by hand again an issue that has `triaged`
#   TRIGGER_LABEL default triage
#   OUT_DIR       default triage-input
#   GH_TOKEN, GITHUB_REPOSITORY   for gh
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
shape() { node "$here/triage-shape.mjs" "$@"; }

issue=${ISSUE:?ISSUE}
event=${EVENT_NAME:-workflow_dispatch}
action=${EVENT_ACTION:-}
label=${LABEL:-}
actor=${ACTOR:-}
association=${ASSOCIATION:-}
force=${FORCE:-false}
trigger_label=${TRIGGER_LABEL:-triage}
dir=${OUT_DIR:-triage-input}

# Issues another workflow or a person already owns, or that are not about
# the project's code: never triaged on their own events.
skip_labels=(triaged dependencies security)

stop() {
  echo "nothing to triage: $1"
  out proceed false
  out reason "$1"
  summarize "### Issue triage: skipped" "#$issue: $1"
  exit 0
}

if [[ ! $issue =~ ^[0-9]{1,7}$ ]]; then
  echo "::error::not an issue number: $issue"
  exit 1
fi

# --- Who may start a triage -------------------------------------------------
if [[ $event == issues ]]; then
  case $action in
    opened)
      case $association in
        OWNER | MEMBER | COLLABORATOR) ;;
        *) stop "its author is not a maintainer (a maintainer adds the \`$trigger_label\` label to have it triaged)" ;;
      esac
      ;;
    labeled)
      [[ $label == "$trigger_label" ]] || stop "the label added was not \`$trigger_label\`"
      ;;
    *) stop "the issue event \`$action\` does not start a triage" ;;
  esac
  # The login goes into an API path: only what a login can be.
  if [[ ! $actor =~ ^[A-Za-z0-9]([A-Za-z0-9-]{0,37}[A-Za-z0-9])?$ ]]; then
    stop "the person who started it has no usable login"
  fi
  permission=$(gh api "repos/$GITHUB_REPOSITORY/collaborators/$actor/permission" --jq .permission 2>/dev/null || echo none)
  case $permission in
    admin | write) ;;
    *) stop "the person who started it does not have write access ($permission)" ;;
  esac
fi

rm -rf "$dir"
mkdir -p "$dir"

# --- The issue --------------------------------------------------------------
gh issue view "$issue" --json number,title,body,labels,author,createdAt,state | shape issue >"$dir/issue.json"
labels=$(shape get labels <"$dir/issue.json")
state=$(shape get state <"$dir/issue.json")
is_bot=$(shape get author_is_bot <"$dir/issue.json")

has_label() { grep -qxF -- "$1" <<<"$labels"; }

if [[ $event == issues ]]; then
  [[ $is_bot != true ]] || stop "its author is a bot"
  for l in "${skip_labels[@]}"; do
    has_label "$l" && stop "it carries the \`$l\` label"
  done
fi
if [[ $force != true ]]; then
  has_label triaged && stop "it is already triaged (force it to triage again)"
  [[ $state == open ]] || stop "it is $state"
fi

# --- The other issues, for duplicates ---------------------------------------
gh issue list --state all --limit 1000 --json number,title,state,labels,closedAt | shape issues 90 >"$dir/issues.json"

# --- The releases and what changed since ------------------------------------
git tag --sort=-creatordate --format='%(refname:short) %(creatordate:short)' | grep -E '^v[0-9]' | head -n 15 >"$dir/releases.txt" || true
app_version >"$dir/version.txt"
{
  echo "# Commits of the checked-out branch, newest first:"
  git log -n 300 --format='%h %as%d %s' HEAD 2>/dev/null | cut -c1-300 || true
} >"$dir/history.txt"

out proceed true
summarize "### Issue triage: input for #$issue" "$(
  echo "- issue: $(wc -c <"$dir/issue.json") bytes, $(grep -c . <<<"$labels" || true) labels"
  echo "- other issues: $(grep -c '"number"' "$dir/issues.json" || true)"
  echo "- releases: $(wc -l <"$dir/releases.txt")"
)"

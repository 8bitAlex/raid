#!/usr/bin/env bash
# The last step of the issue triage (triage.yml): checks Claude's verdict
# and, only if it is valid, labels the issue and comments on it. Claude
# wrote the verdict from an outsider's text, so nothing in it is trusted:
# triage-verdict.mjs validates it against a strict schema and rewrites its
# text so that it can only be read (no HTML, links, images, mentions or
# e-mail addresses).
#
# What it does to GitHub, and nothing else, to the one issue $ISSUE:
#   - creates the labels it owns (triaged, needs-info, priority: ...);
#   - posts ONE comment, "Automated triage";
#   - adds labels: triaged, the category, the priority, needs-info when the
#     reporter is asked something, duplicate when it names a duplicate. It
#     also takes off an older `priority:` label of its own, and the
#     `triage` label that a maintainer used to ask for this run.
# It never closes, reopens, assigns or deletes, never edits the issue's
# text, never adds `claude`, and touches no other
# issue. A verdict that fails validation posts nothing and adds no
# `triaged` label; the reasons go to the run summary and exit 1. No verdict
# at all (the triage job failed) is not this script's failure: exit 0.
#
# Environment:
#   ISSUE        the issue number (required)
#   IN_DIR       triage-prepare.sh's folder, default triage-input
#   VERDICT      Claude's verdict, default triage-output/verdict.json
#   REPO_ROOT    the checkout the suspected files are looked up in, default .
#   FORCE        true: triage an issue that already has `triaged` again
#   TRIGGER_LABEL  the label that asks for a triage, default triage
#   RUN_URL, SHA the run and commit, named in the comment
#   GH_TOKEN, GITHUB_REPOSITORY   for gh
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
issue=${ISSUE:?ISSUE}
in_dir=${IN_DIR:-triage-input}
verdict=${VERDICT:-triage-output/verdict.json}
root=${REPO_ROOT:-.}
force=${FORCE:-false}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if [[ ! $issue =~ ^[0-9]{1,7}$ ]]; then
  echo "::error::not an issue number: $issue"
  exit 1
fi

if [[ ! -e $verdict && ! -L $verdict ]]; then
  echo "no verdict at $verdict: nothing to apply"
  summarize "### Issue triage: #$issue" "Claude wrote no verdict, so nothing was posted and the issue is not marked \`triaged\`. See the triage job."
  exit 0
fi

if ! node "$here/triage-verdict.mjs" "$issue" "$in_dir" "$verdict" "$root" "$work" 2>"$work/reasons"; then
  reasons=$(cat "$work/reasons")
  echo "::error::the triage verdict for #$issue was refused"
  cat "$work/reasons" >&2
  summarize "### Issue triage: #$issue not posted" "Claude's verdict failed validation, so nothing was posted and the issue is not marked \`triaged\`:

\`\`\`text
$reasons
\`\`\`"
  out refused true
  exit 1
fi

# The issue as it is now: another run (or a person) may have triaged it.
gh issue view "$issue" --json labels,state | node "$here/triage-shape.mjs" issue >"$work/now.json"
now_labels=$(node "$here/triage-shape.mjs" get labels <"$work/now.json")
if [[ $force != true ]] && grep -qxF triaged <<<"$now_labels"; then
  echo "#$issue was triaged meanwhile"
  summarize "### Issue triage: #$issue" "The issue was triaged while this run worked, so nothing was posted."
  exit 0
fi

mapfile -t add <"$work/labels.txt"

# Labels this workflow owns exist before they are added. The others (the
# category labels, duplicate) are the repository's; one that is missing is
# created, one that exists is left as it is.
ensure() { gh label create "$1" --force --color "$2" --description "$3"; }
ensure_default() { gh label create "$1" >/dev/null 2>&1 || true; }
ensure "triaged" C5DEF5 "Read by the automatic issue triage"
ensure "needs-info" FEF2C0 "Waiting for the reporter to answer"
ensure "priority: high" B60205 "Blocks a core flow, or looks broken for a default setup"
ensure "priority: medium" FBCA04 "Wrong or awkward, with a way around it"
ensure "priority: low" 0E8A16 "Cosmetic or rare"
for l in "${add[@]}"; do
  case $l in
    triaged | needs-info | priority:*) ;;
    *) ensure_default "$l" ;;
  esac
done

# The comment first: a label added without its comment would hide the issue
# from a second try.
gh issue comment "$issue" --body-file "$work/comment.md"

edit=(gh issue edit "$issue")
for l in "${add[@]}"; do edit+=(--add-label "$l"); done
while IFS= read -r old; do
  case $old in
    "priority: "*) printf '%s\n' "${add[@]}" | grep -qxF -- "$old" || edit+=(--remove-label "$old") ;;
    "${TRIGGER_LABEL:-triage}") edit+=(--remove-label "$old") ;;
  esac
done <<<"$now_labels"
"${edit[@]}"

echo "triaged #$issue: ${add[*]}"
summarize "### Issue triage: #$issue" "Posted the comment and added: $(printf '`%s` ' "${add[@]}")"
out triaged true

#!/usr/bin/env bash
# What the scripts in this folder share: where a step's outputs and the
# run's summary go, the bot's identity, and small text helpers. Sourced,
# never run:
#
#   source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
#
# Outside GitHub Actions (the tests, a shell) the outputs and the summary
# go nowhere, so every script runs the same way there.

# A step output, GITHUB_OUTPUT: `out name value`. A value is one line.
output=${GITHUB_OUTPUT:-/dev/null}
out() {
  echo "$1=$(one_line "$2")" >>"$output"
}

# A section of the run summary, GITHUB_STEP_SUMMARY: a heading, a blank
# line, then the text (which may span lines).
summary=${GITHUB_STEP_SUMMARY:-/dev/null}
summarize() {
  {
    echo "$1"
    echo
    echo "$2"
  } >>"$summary"
}

# Text on one line, for an output or an annotation.
one_line() {
  local s=$1
  s=${s//$'\r'/ }
  printf '%s' "${s//$'\n'/ }"
}

# Git as github-actions, for the commits the workflows make.
bot_name="github-actions[bot]"
bot_email="41898282+github-actions[bot]@users.noreply.github.com"
bot_git() {
  git -c user.name="$bot_name" -c user.email="$bot_email" "$@"
}

# The version in src/resources/app.properties of the checkout (empty
# without one).
app_version() {
  sed -n 's/^version=\([0-9][0-9.]*\)\r\{0,1\}$/\1/p' "${1:-src/resources/app.properties}" 2>/dev/null | head -n 1 || true
}

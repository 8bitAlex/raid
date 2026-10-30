#!/usr/bin/env bash
# The harness the script tests share (test-triage.sh, test-security.sh,
# test-claude-auth.sh): a scratch folder that goes at exit, git as a tester
# reading none of the machine's config, a fake-command folder on PATH, the
# counting checks, and how a script under test is run and its outputs read.
# Sourced first thing:
#
#   source "$(dirname "$0")/test-lib.sh"
#
# Sets scripts (this folder), tmp (the scratch folder), passed and failed.

scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
passed=0
failed=0

# Git as "Tester", with a fresh global config and no system one; $tmp/bin
# comes first on PATH for the fakes (gh, curl, npm, ...).
isolate_git() {
  export HOME=$tmp/home GIT_CONFIG_NOSYSTEM=1
  mkdir -p "$HOME" "$tmp/bin"
  unset GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL
  git config --global user.name Tester
  git config --global user.email tester@example.com
  git config --global init.defaultBranch main
  git config --global advice.detachedHead false
  git config --global commit.gpgsign false
  git config --global tag.gpgsign false
  git config --global core.autocrlf false
  export PATH="$tmp/bin:$PATH"
}

# check NAME GOT WANT: one check, counted; a mismatch is printed.
check() {
  if [[ $2 == "$3" ]]; then
    passed=$((passed + 1))
  else
    failed=$((failed + 1))
    echo "FAIL: $1: expected [$3], got [$2]"
  fi
}

# contains NAME TEXT PART: a check that TEXT has PART in it.
contains() {
  if grep -qF -- "$3" <<<"$2"; then
    passed=$((passed + 1))
  else
    failed=$((failed + 1))
    echo "FAIL: $1: [$3] not in:"
    sed 's/^/    /' <<<"$2"
  fi
}

# lacks NAME TEXT PART: a check that TEXT does not have PART in it.
lacks() {
  if grep -qF -- "$3" <<<"$2"; then
    failed=$((failed + 1))
    echo "FAIL: $1: [$3] found in:"
    sed 's/^/    /' <<<"$2"
  else
    passed=$((passed + 1))
  fi
}

# Runs script $1 in $tmp/work with the rest as its environment; sets code
# (its exit code) and out (its GITHUB_OUTPUT). Its log is $tmp/log, its
# summary $tmp/summary; VERBOSE=1 prints the log.
run() {
  local script=$1
  shift
  cd "$tmp/work"
  : >"$tmp/out"
  : >"$tmp/summary"
  code=0
  env GITHUB_OUTPUT="$tmp/out" GITHUB_STEP_SUMMARY="$tmp/summary" "$@" \
    bash "$scripts/$script" >"$tmp/log" 2>&1 || code=$?
  out=$(cat "$tmp/out")
  if [[ -n ${VERBOSE:-} ]]; then
    echo "  \$ $script (exit $code)"
    sed 's/^/    /' "$tmp/log"
  fi
}

# The last value of output $1 of the last run.
out_value() {
  sed -n "s/^$1=//p" <<<"$out" | tail -n 1
}

# The tally; exits 1 when a check failed.
finish() {
  echo
  echo "$passed passed, $failed failed"
  ((failed == 0))
}

#!/usr/bin/env bash
# Lane 3, the scanning half (security-scan.yml, job "scan"): finds known
# vulnerabilities that can be fixed without a breaking change, makes the
# deterministic fix in the working tree, proves the repository still builds,
# and leaves a patch. It has NO write access to GitHub and makes no commit;
# security-open-pr.sh (another job) turns the patch into a PR.
#
#   npm  (site/): `npm audit --json --package-lock-only` reads the lock
#        without installing anything. A vulnerability is fixable when npm
#        says a fix exists that is not a semver-major bump. `npm audit fix
#        --package-lock-only` then changes only what the vulnerable packages
#        need; the result must pass the manifest checks (no new direct
#        dependency, few new packages, nothing off the registry, no package
#        moved to another major), and `npm ci && npm run build` must succeed:
#        otherwise the change is thrown away and a problem is recorded (the
#        docs site is never put at risk).
#   Go   `govulncheck -scan module -json ./...` lists vulnerable modules with
#        the version that fixes them; each fixable one (same major, not the
#        standard library) gets `go get module@fixed`, then `go mod tidy`,
#        the manifest checks, and `go build ./... && go test ./...`. When
#        that fails the update needs a code change: the bump is kept for
#        Claude (needs_fix=true) only if SECURITY_CLAUDE_FIX is `true` and
#        nothing else is in the patch; otherwise it is thrown away and a
#        problem is recorded.
# Vulnerabilities an open Dependabot PR already mentions are left to that PR
# (Lane 1). Skipped ones (major fixes, no fix, the Go toolchain) go to the run
# summary only.
#
# Writes to OUT_DIR (default scan-output):
#   fix.patch      the change (only go.mod, go.sum, site/package.json,
#                  site/package-lock.json), when there is one
#   changes.txt    `go|npm  name  old  new` lines, for the PR text
#   problems.txt   what a person should look at, one line each
#   skipped.txt    what was left alone and why
# and the outputs changed=true|false and needs_fix=true|false.
#
# Environment: SECURITY_CLAUDE_FIX, MIN_SEVERITY (low|moderate|high|critical,
# default low), SITE_DIR (default site), OUT_DIR, GH_TOKEN (read-only, to list
# Dependabot's open PRs), GITHUB_REPOSITORY. npm, go and govulncheck are on
# PATH (the workflow installs govulncheck at a pinned version).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pol() { node "$here/security-policy.mjs" "$@"; }
site=${SITE_DIR:-site}
min=${MIN_SEVERITY:-low}
out_dir=${OUT_DIR:-scan-output}
claude_fix=${SECURITY_CLAUDE_FIX:-false}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

rm -rf "$out_dir"
mkdir -p "$out_dir" "$work/base"
: >"$out_dir/problems.txt"
: >"$out_dir/skipped.txt"
: >"$out_dir/changes.txt"
problem() { echo "$1" >>"$out_dir/problems.txt"; echo "::warning::$1"; }
skip() { echo "$1" >>"$out_dir/skipped.txt"; }
# One line of text from a log, safe to keep in a report.
tail_of() { tail -n 8 "$1" | LC_ALL=C tr -d '\000-\010\013-\037\177' | cut -c1-200 | paste -sd '|' -; }

# The manifests as committed: the base of every comparison.
for f in go.mod go.sum "$site/package.json" "$site/package-lock.json" src/resources/app.properties; do
  if git cat-file -e "HEAD:$f" 2>/dev/null; then
    mkdir -p "$work/base/$(dirname "$f")"
    git show "HEAD:$f" >"$work/base/$f"
  fi
done

# --- What Dependabot already has a PR for -----------------------------------
gh pr list --state open --author 'app/dependabot' --limit 100 --json title,headRefName,body \
  --jq '.[] | .title + " " + .headRefName + " " + ((.body // "")[0:800])' >"$work/covered.txt" 2>/dev/null || : >"$work/covered.txt"
covered() { # name
  local esc
  esc=$(printf '%s' "$1" | sed 's/[][\.*^$/+?(){}|]/\\&/g')
  grep -qiE "(^|[^A-Za-z0-9_.@-])$esc([^A-Za-z0-9_.]|\$)" "$work/covered.txt"
}

# --- npm --------------------------------------------------------------------
npm_changed=false
if [[ -f $site/package-lock.json ]]; then
  (cd "$site" && npm audit --json --package-lock-only >"$work/audit.json" 2>"$work/audit.err") || true
  names=()
  if [[ -s $work/audit.json ]] && pol audit "$min" <"$work/audit.json" >"$work/audit.tsv" 2>"$work/audit.reject"; then
    while IFS=$'\t' read -r kind name sev reason; do
      [[ -n $kind ]] || continue
      if [[ $kind == skipped ]]; then
        skip "npm $name ($sev): $reason"
      elif covered "$name"; then
        skip "npm $name ($sev): an open Dependabot PR covers it"
      else
        names+=("$name")
      fi
    done <"$work/audit.tsv"
  else
    problem "npm audit gave no readable report: $(tail_of "$work/audit.err")"
  fi
  if ((${#names[@]})); then
    echo "npm: fixable: ${names[*]}"
    if ! (cd "$site" && npm audit fix --package-lock-only --no-audit --no-fund >"$work/npm-fix.log" 2>&1); then
      problem "npm audit fix failed: $(tail_of "$work/npm-fix.log")"
      git checkout -q -- "$site/package.json" "$site/package-lock.json"
    elif git diff --quiet -- "$site/package.json" "$site/package-lock.json"; then
      skip "npm ${names[*]}: npm audit fix changed nothing"
    elif ! pol manifests "$work/base" . --no-major --only=npm >"$work/npm-changes.txt" 2>"$work/npm-reject.txt"; then
      problem "npm audit fix was thrown away, it needs a person: $(sed 's/^reject: //' "$work/npm-reject.txt" | paste -sd ';' -)"
      git checkout -q -- "$site/package.json" "$site/package-lock.json"
    elif ! (cd "$site" && npm ci --no-audit --no-fund && npm run build) >"$work/npm-build.log" 2>&1; then
      problem "npm audit fix for ${names[*]} broke the docs build, so it was thrown away: $(tail_of "$work/npm-build.log")"
      git checkout -q -- "$site/package.json" "$site/package-lock.json"
    else
      npm_changed=true
      sed -n 's/^change: //p' "$work/npm-changes.txt" | tr ' ' '\t' >>"$out_dir/changes.txt"
    fi
  fi
fi

# --- Go ---------------------------------------------------------------------
go_changed=false
needs_fix=false
if [[ -f go.mod ]] && command -v govulncheck >/dev/null 2>&1; then
  govulncheck -scan module -json ./... >"$work/vuln.json" 2>"$work/vuln.err" || true
  mods=()
  if pol vuln <"$work/vuln.json" >"$work/vuln.tsv" 2>"$work/vuln.reject"; then
    while IFS=$'\t' read -r kind module from to ids; do
      [[ -n $kind ]] || continue
      if [[ $kind == skipped ]]; then
        skip "go $module: $from"
      elif covered "$module"; then
        skip "go $module: an open Dependabot PR covers it"
      else
        mods+=("$module@$to")
      fi
    done <"$work/vuln.tsv"
  else
    problem "govulncheck gave no readable report: $(tail_of "$work/vuln.err")"
  fi
  if ((${#mods[@]})); then
    echo "go: fixable: ${mods[*]}"
    go_ok=true
    for m in "${mods[@]}"; do
      go get "$m" >>"$work/go-fix.log" 2>&1 || go_ok=false
    done
    $go_ok && { go mod tidy >>"$work/go-fix.log" 2>&1 || go_ok=false; }
    if ! $go_ok; then
      problem "go get/tidy failed for ${mods[*]}: $(tail_of "$work/go-fix.log")"
      git checkout -q -- go.mod go.sum
    elif git diff --quiet -- go.mod go.sum; then
      skip "go ${mods[*]}: go get changed nothing"
    elif ! pol manifests "$work/base" . --no-major --only=go >"$work/go-changes.txt" 2>"$work/go-reject.txt"; then
      problem "the Go update was thrown away, it needs a person: $(sed 's/^reject: //' "$work/go-reject.txt" | paste -sd ';' -)"
      git checkout -q -- go.mod go.sum
    elif (go build ./... && go test ./...) >"$work/go-test.log" 2>&1; then
      go_changed=true
      sed -n 's/^change: //p' "$work/go-changes.txt" | tr ' ' '\t' >>"$out_dir/changes.txt"
    elif [[ $claude_fix == true && $npm_changed == false ]]; then
      go_changed=true
      needs_fix=true
      sed -n 's/^change: //p' "$work/go-changes.txt" | tr ' ' '\t' >>"$out_dir/changes.txt"
      skip "go ${mods[*]}: the update needs a code change, so Claude is asked to adapt it (build/test output: $(tail_of "$work/go-test.log"))"
    else
      problem "the Go update for ${mods[*]} needs a code change (build or tests fail), so it was thrown away: $(tail_of "$work/go-test.log")"
      git checkout -q -- go.mod go.sum
    fi
  fi
else
  [[ -f go.mod ]] && skip "go: govulncheck is not installed, Go was not scanned"
fi

# --- The patch --------------------------------------------------------------
changed=false
if [[ $npm_changed == true || $go_changed == true ]]; then
  # Only the manifests, by name: nothing the builds left behind.
  paths=()
  for f in go.mod go.sum "$site/package.json" "$site/package-lock.json"; do
    git diff --quiet -- "$f" || paths+=("$f")
  done
  git add -- "${paths[@]}"
  git diff --cached --binary >"$out_dir/fix.patch"
  git reset -q
  changed=true
fi

out changed "$changed"
out needs_fix "$needs_fix"
out problems "$(grep -c . "$out_dir/problems.txt" || true)"
summarize "### Security scan" "$(
  echo "- patch: $changed (needs a code fix: $needs_fix)"
  echo "- changes: $(grep -c . "$out_dir/changes.txt" || true)"
  echo "- problems: $(grep -c . "$out_dir/problems.txt" || true)"
  if [[ -s $out_dir/skipped.txt ]]; then
    echo
    echo "Left alone:"
    sed 's/^/- /' "$out_dir/skipped.txt"
  fi
)"

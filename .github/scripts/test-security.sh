#!/usr/bin/env bash
# Tests the security-update scripts (security-update.yml, security-fix.yml,
# security-scan.yml) against a fake GitHub: `gh` (test-fake-gh.mjs) answers
# from a throwaway git repository that stands for the remote, `curl` stands
# for OSV, and fake npm, go and govulncheck stand for the toolchains. Real
# git does the patching and pushing, to a local bare repository. Nothing
# leaves the temp folder; no network.
#
# What is checked: a Dependabot PR is merged only when it is a security
# update, not a major bump of a direct dependency, touches only the
# manifests, comes from Dependabot's commits, and CI is green on that exact
# commit; every refusal makes no write; the kill switches; Claude's patch is
# refused when it touches .github/, the manifests, other files, or src/
# without a version bump; the scan throws away a fix that breaks the docs
# build; only one PR; one issue.
#
#   bash .github/scripts/test-security.sh
set -euo pipefail
source "$(dirname "$0")/test-lib.sh"
isolate_git

fake=$tmp/fake
export FAKE_DIR=$fake GITHUB_REPOSITORY=example/repo
export OSV_URL=http://osv.test/v1/query INTERVAL=0
cat >"$tmp/bin/gh" <<EOF
#!/usr/bin/env bash
exec node "$scripts/test-fake-gh.mjs" "\$@"
EOF
# OSV: the answer for the version in the request body is $FAKE_DIR/osv-VERSION.json, else {}.
cat >"$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
out=; body=
while (($#)); do
  case $1 in
    -o) out=$2; shift ;;
    --data) body=$2; shift ;;
  esac
  shift
done
echo "curl $body" >>"$FAKE_DIR/calls"
v=$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' <<<"$body")
if [[ -f $FAKE_DIR/osv-$v.json ]]; then cp "$FAKE_DIR/osv-$v.json" "$out"; else echo '{}' >"$out"; fi
EOF
chmod +x "$tmp/bin/gh" "$tmp/bin/curl"
mkdir -p "$tmp/work"

# --- A repository ---------------------------------------------------------------
write_base() { # dir
  mkdir -p "$1/src/resources" "$1/src/cmd" "$1/site/docs" "$1/.github/workflows"
  cat >"$1/go.mod" <<'EOT'
module github.com/8bitAlex/raid

go 1.25.5

require (
	github.com/gofrs/flock v0.13.0
	github.com/spf13/cobra v1.9.1
)

require github.com/inconshreveable/mousetrap v1.1.0 // indirect
EOT
  echo "github.com/gofrs/flock v0.13.0 h1:aaa" >"$1/go.sum"
  cat >"$1/site/package.json" <<'EOT'
{ "name": "site", "scripts": { "build": "docusaurus build" }, "dependencies": { "react": "19.0.0" } }
EOT
  cat >"$1/site/package-lock.json" <<'EOT'
{ "name": "site", "lockfileVersion": 3, "packages": {
  "": { "name": "site" },
  "node_modules/react": { "version": "19.0.0", "resolved": "https://registry.npmjs.org/react/-/react-19.0.0.tgz" },
  "node_modules/ws": { "version": "7.5.0", "resolved": "https://registry.npmjs.org/ws/-/ws-7.5.0.tgz" } } }
EOT
  printf 'version=0.18.0\nenvironment=development\n' >"$1/src/resources/app.properties"
  seq 1 40 | sed 's/^/line /' >"$1/src/cmd/raid.go"
  echo "name: ci" >"$1/.github/workflows/ci.yml"
  echo "docs" >"$1/site/docs/a.mdx"
}
w=$tmp/w
# A fresh fake GitHub: the remote with main, a working clone, no PR yet.
fresh() {
  rm -rf "$fake" "$w"
  mkdir -p "$fake"
  git init -q --bare "$fake/remote.git"
  git clone -q "$fake/remote.git" "$w" 2>/dev/null
  git -C "$w" checkout -q -b main
  write_base "$w"
  git -C "$w" add -A
  git -C "$w" commit -q -m "base"
  git -C "$w" push -q origin main
  : >"$fake/calls"
  echo "v0.18.0" >"$fake/tags"
}
# commit_as dependabot|bot|human MESSAGE: commits the working tree on the PR branch and pushes it.
commit_as() {
  local name email
  case $1 in
    dependabot) name='dependabot[bot]'; email='49699333+dependabot[bot]@users.noreply.github.com' ;;
    bot) name='github-actions[bot]'; email='41898282+github-actions[bot]@users.noreply.github.com' ;;
    *) name='A Person'; email='person@example.com' ;;
  esac
  git -C "$w" add -A
  git -C "$w" -c user.name="$name" -c user.email="$email" commit -q -m "$2"
  git -C "$w" push -q -f origin "HEAD:refs/heads/$ref"
}
# A PR branch from main: sets ref and the PR's state. branch AUTHOR REF
ref=
branch() {
  ref=$2
  git -C "$w" checkout -q -B "$ref" main
  node -e '
    const [author, ref] = process.argv.slice(1);
    console.log(JSON.stringify({ number: 7, author, ref, base: "main", state: "open", draft: false, repo: "example/repo" }));
  ' "$1" "$ref" >"$fake/pr.json"
}
head_sha() { git -C "$fake/remote.git" rev-parse "refs/heads/$ref"; }
calls() { cat "$fake/calls"; }
# The calls that write (everything but reads).
writes() { grep -E '^(pr (merge|comment|create)|issue (create|comment)|label |workflow run|api -X)' "$fake/calls" || true; }
count() { grep -c -- "$1" "$fake/calls" || true; }
bump_flock() { sed -i 's/flock v0.13.0/flock v0.13.1/' "$w/go.mod"; echo "github.com/gofrs/flock v0.13.1 h1:bbb" >>"$w/go.sum"; }
dref=dependabot/go_modules/github.com/gofrs/flock-0.13.1

deps_json() { # eco name type update prev next
  node -e '
    const [eco, name, type, update, prev, next] = process.argv.slice(1);
    console.log(JSON.stringify([{ dependencyName: name, dependencyType: type, updateType: "version-update:semver-" + update,
      directory: "/", packageEcosystem: eco, targetBranch: "main", prevVersion: prev, newVersion: next, alertState: "", ghsaId: "", cvss: 0 }]));
  ' "$@"
}
flock_json=$(deps_json go_modules github.com/gofrs/flock direct:production patch 0.13.0 0.13.1)

assess() {
  : >"$fake/calls"
  run security-assess.sh PR=7 SECURITY_AUTOMERGE=true "UPDATED_JSON=$flock_json" "$@"
}
not_eligible() { # name want-in-reason
  check "$1: exit" "$code" 0
  check "$1: not eligible" "$(out_value eligible)" false
  contains "$1: reason" "$(out_value reason)" "$2"
  check "$1: no write" "$(writes)" ""
}

# --- security-assess.sh -----------------------------------------------------------
echo "== assess: a Dependabot PR that fixes an advisory"
fresh
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.0.json"
branch 'dependabot[bot]' "$dref"
bump_flock
commit_as dependabot "chore(deps): bump github.com/gofrs/flock from 0.13.0 to 0.13.1"
assess
check "exit" "$code" 0
check "eligible" "$(out_value eligible)" true
check "mode" "$(out_value mode)" deps
check "head" "$(out_value head_sha)" "$(head_sha)"
check "a Go change needs the Go builds" "$(out_value required)" "build (ubuntu-latest),build (macos-latest),build (windows-latest),build"
check "no docs build for Go" "$(out_value touches_site)" false
contains "names the advisory" "$(out_value advisories)" "GHSA-aaaa-bbbb-cccc"
check "it only reads" "$(writes)" ""
check "it asked OSV about the old and the new version" "$(count '^curl')" 2
check "no version bump needed for a manifest-only change" "$(out_value touches_src)" false

echo "== assess: the kill switch and the PR itself"
assess SECURITY_AUTOMERGE=false
not_eligible "switch false" "kill switch is off"
assess SECURITY_AUTOMERGE=
not_eligible "switch unset" "kill switch is off"
check "...asked nothing" "$(calls)" ""
assess EXPECT_SHA=$(printf 'e%.0s' {1..40})
not_eligible "the head moved" "head moved"
assess "PR=abc"
check "a bad PR number: exit" "$code" 1
node -e 'const f=process.argv[1],p=JSON.parse(require("fs").readFileSync(f));p.author="octocat";require("fs").writeFileSync(f,JSON.stringify(p))' "$fake/pr.json"
assess
not_eligible "a person's PR" "is not dependabot[bot] or github-actions[bot]"
node -e 'const f=process.argv[1],p=JSON.parse(require("fs").readFileSync(f));p.author="dependabot[bot]";p.state="closed";require("fs").writeFileSync(f,JSON.stringify(p))' "$fake/pr.json"
assess
not_eligible "a closed PR" "the PR is closed"
node -e 'const f=process.argv[1],p=JSON.parse(require("fs").readFileSync(f));p.state="open";p.repo="fork/repo";require("fs").writeFileSync(f,JSON.stringify(p))' "$fake/pr.json"
assess
not_eligible "a fork" "another repository"

echo "== assess: is it a security update"
node -e 'const f=process.argv[1],p=JSON.parse(require("fs").readFileSync(f));p.repo="example/repo";require("fs").writeFileSync(f,JSON.stringify(p))' "$fake/pr.json"
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.1.json"
assess
not_eligible "the advisory is still in the new version" "not a security update"
rm -f "$fake/osv-0.13.1.json" "$fake/osv-0.13.0.json"
assess
not_eligible "no advisory at all (a plain version update)" "not a security update"
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.0.json"
assess "UPDATED_JSON=$(deps_json go_modules github.com/gofrs/flock direct:production major 0.13.0 1.0.0)"
not_eligible "a major bump of a direct dependency" "semver-major bump of a direct dependency"
assess "UPDATED_JSON="
not_eligible "no metadata" "metadata is empty"
assess "UPDATED_JSON=[{\"dependencyName\":\"x;y\",\"packageEcosystem\":\"go_modules\"}]"
not_eligible "a bad name" "not a valid Go package name"
assess CLASSIFY=no
check "CLASSIFY=no does not look for an advisory (a fix PR was classified before)" "$(out_value eligible)" true
check "...and asks OSV nothing" "$(count '^curl')" 0

echo "== assess: what the PR changes"
fresh
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.0.json"
branch 'dependabot[bot]' "$dref"
bump_flock
echo "name: evil" >>"$w/.github/workflows/ci.yml"
commit_as dependabot "chore(deps): bump flock"
assess
not_eligible "a workflow changed" "under .github/"
branch 'dependabot[bot]' "$dref"
bump_flock
echo "echo hi" >>"$w/src/cmd/raid.go"
commit_as dependabot "chore(deps): bump flock"
assess
not_eligible "code changed in a dependency update" "is not a dependency manifest"
branch 'dependabot[bot]' "$dref"
bump_flock
commit_as dependabot "chore(deps): bump flock"
echo "tweak" >>"$w/go.sum"
commit_as human "tweak"
assess
not_eligible "a person's commit on top" "only Dependabot's and this automation's commits"
branch 'dependabot[bot]' "$dref"
printf '\nrequire github.com/evil/new v1.0.0\n' >>"$w/go.mod"
sed -i 's/flock v0.13.0/flock v0.13.1/' "$w/go.mod"
commit_as dependabot "chore(deps): bump flock"
assess
not_eligible "a new direct dependency" "github.com/evil/new is a new direct dependency"
branch 'dependabot[bot]' "$dref"
sed -i 's/^go 1.25.5/go 1.26.0/' "$w/go.mod"
bump_flock
commit_as dependabot "chore(deps): bump flock"
assess
not_eligible "the go directive moved" "directive other than require changed"

echo "== assess: the npm side"
fresh
nref=dependabot/npm_and_yarn/site/npm_and_yarn-b2fd70aae3
branch 'dependabot[bot]' "$nref"
sed -i 's/"version": "7.5.0"/"version": "7.5.1"/; s#ws-7.5.0#ws-7.5.1#' "$w/site/package-lock.json"
commit_as dependabot "chore(deps): bump ws"
echo '{"vulns":[{"id":"GHSA-ws00-ws00-ws00"}]}' >"$fake/osv-7.5.0.json"
ws_json=$(deps_json npm_and_yarn ws indirect patch 7.5.0 7.5.1)
assess "UPDATED_JSON=$ws_json"
check "eligible" "$(out_value eligible)" true
check "needs the docs build only" "$(out_value required)" docs
check "touches the site" "$(out_value touches_site)" true
check "asks OSV in the npm ecosystem" "$(grep -c '"ecosystem":"npm"' "$fake/calls" || true)" 2

echo "== assess: a PR with a fix on top (mode fix)"
fresh
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.0.json"
branch 'dependabot[bot]' "$dref"
bump_flock
commit_as dependabot "chore(deps): bump flock"
echo "fixed" >>"$w/src/cmd/raid.go"
commit_as bot "fix(deps): adapt code to the dependency update"
assess CLASSIFY=no
not_eligible "src/ changed without a version bump" "needs a higher version"
sed -i 's/0.18.0/0.18.1/' "$w/src/resources/app.properties"
commit_as bot "fix(deps): adapt code to the dependency update"
assess CLASSIFY=no
check "with the bump: eligible" "$(out_value eligible)" true
check "...mode fix" "$(out_value mode)" fix
check "...touches src" "$(out_value touches_src)" true
echo "v0.18.1" >>"$fake/tags"
assess CLASSIFY=no
not_eligible "the tag of the new version exists" "tag v0.18.1 exists"
rm -f "$fake/tags"; echo v0.18.0 >"$fake/tags"
sed -i 's/0.18.1/0.17.0/' "$w/src/resources/app.properties"
commit_as bot "fix(deps): adapt code to the dependency update"
assess CLASSIFY=no
not_eligible "a lower version" "needs a higher version"

# --- security-wait-ci.sh ------------------------------------------------------------
echo "== wait for CI"
fresh
branch 'dependabot[bot]' "$dref"
bump_flock
commit_as dependabot "chore(deps): bump flock"
sha=$(head_sha)
req='build (ubuntu-latest),build (macos-latest),build (windows-latest),build'
checks() { # N STATUS CONCLUSION...: writes checks.N.json for the four required checks
  local n=$1 status=$2 conclusion=$3
  node -e '
    const [status, conclusion] = process.argv.slice(1);
    console.log(JSON.stringify(["build (ubuntu-latest)", "build (macos-latest)", "build (windows-latest)", "build"].map((name) =>
      ({ name, status, conclusion: conclusion === "null" ? null : conclusion }))));
  ' "$status" "$conclusion" >"$fake/checks.$n.json"
}
waitci() {
  rm -f "$fake"/checks.*.json "$fake/check-looks"
  : >"$fake/calls"
}
waitci; checks 0 completed success
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7
check "green at once: ci" "$(out_value ci)" green
check "...one look" "$(count 'check-runs')" 1
waitci; checks 0 in_progress null; checks 2 completed success
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7
check "running, then green: ci" "$(out_value ci)" green
check "...three looks" "$(count 'check-runs')" 3
waitci; checks 0 completed failure
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7
check "failed: ci" "$(out_value ci)" red
contains "...says which" "$(out_value reason)" "build (ubuntu-latest) (failure)"
waitci; checks 0 queued null
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7 TIMEOUT_MINUTES=0
check "never finishes: ci" "$(out_value ci)" timeout
waitci; : >"$fake/checks.0.json"
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7 TIMEOUT_MINUTES=0
check "no check ever starts: timeout" "$(out_value ci)" timeout
contains "...says what is missing" "$(out_value reason)" "has not started"
waitci; checks 0 completed success
run security-wait-ci.sh "SHA=$sha" "REQUIRED=$req" PR=7 FAKE_HEAD_MOVES=1
check "the PR's head moved: ci" "$(out_value ci)" moved
check "...no look at the checks" "$(count 'check-runs')" 0
run security-wait-ci.sh "SHA=notasha" "REQUIRED=$req"
check "a bad commit: exit" "$code" 1
check "it only reads" "$(writes)" ""

# --- security-merge.sh ----------------------------------------------------------------
echo "== merge"
merge() { : >"$fake/calls"; run security-merge.sh PR=7 "SHA=$sha" SECURITY_AUTOMERGE=true "$@"; }
merge
check "exit" "$code" 0
check "merged" "$(out_value merged)" true
check "squash, pinned to the tested commit" "$(grep '^pr merge' "$fake/calls")" "pr merge 7 --squash --delete-branch --match-head-commit $sha"
check "CI started on main" "$(grep '^workflow run' "$fake/calls")" "workflow run build.yml --ref main"
merge TOUCHES_SITE=true
check "a site change also deploys the docs" "$(grep -c '^workflow run docs.yml --ref main' "$fake/calls" || true)" 1
merge SECURITY_AUTOMERGE=false
check "switch off: not merged" "$(out_value merged)" false
check "...no write" "$(writes)" ""
merge FAKE_HEAD_MOVES=1
check "head moved: not merged" "$(out_value merged)" false
check "...no merge call" "$(count '^pr merge')" 0
merge FAKE_MERGE_FAIL=1
check "refused by GitHub: not merged" "$(out_value merged)" false
contains "...says why" "$(out_value reason)" "GitHub refused the merge"
check "...no CI started" "$(count '^workflow run')" 0
run security-merge.sh PR=7 SHA=short SECURITY_AUTOMERGE=true
check "a bad commit: exit" "$code" 1

# --- security-report.sh -----------------------------------------------------------------
echo "== report"
report() { : >"$fake/calls"; rm -f "$fake"/issue-*.md "$fake"/comment.* "$fake/comments.json"; run security-report.sh "$@"; }
echo '[]' >"$fake/issues.json"
report PR=7 "REASON=CI failed: build (ubuntu-latest) (failure)" RUN_URL=https://github.com/example/repo/actions/runs/9
check "exit" "$code" 0
check "a comment on the PR" "$(count '^pr comment 7')" 1
check "one issue, labelled security" "$(grep '^issue create' "$fake/calls")" "issue create --title Security update needs attention: PR #7 --label security --body-file $(grep -o -- '--body-file [^ ]*' "$fake/calls" | tail -n 1 | cut -d' ' -f2)"
contains "the reason is in the issue" "$(cat "$fake/issue-create.md")" "CI failed: build (ubuntu-latest) (failure)"
contains "the run is linked" "$(cat "$fake/issue-create.md")" "([run](https://github.com/example/repo/actions/runs/9))"
check "the label is created if missing" "$(count '^label create security')" 1
echo '[{"number":12,"title":"Security update needs attention: PR #7"}]' >"$fake/issues.json"
report PR=7 "REASON=again"
check "an open issue for the PR gets a comment, not a second issue" "$(count '^issue create')" 0
check "...on that issue" "$(count '^issue comment 12')" 1
report "REASON=the scan broke"
check "without a PR: no PR comment" "$(count '^pr comment')" 0
contains "...the rolling issue" "$(calls)" "issue create --title Security scan needs attention"
report PR=7 "REASON=$(printf 'bad\001\033[31m ```evil``` end')"
body=$(cat "$fake/issue-comment.md" "$fake/comment.1" 2>/dev/null || true)
check "control characters are dropped" "$(grep -c "$(printf '\033')" <<<"$body" || true)" 0
lacks "a code fence cannot be closed early" "$body" "\`\`\`evil"
report PR=abc "REASON=x"
check "a bad PR number: exit" "$code" 1
check "...no write" "$(writes)" ""

# --- security-escalate.sh ---------------------------------------------------------------
echo "== escalate: CI is not green"
echo '[]' >"$fake/issues.json"
escalate() { : >"$fake/calls"; rm -f "$fake"/issue-*.md "$fake"/comment.* "$fake/comments.json"; run security-escalate.sh PR=7 "SHA=$sha" SECURITY_AUTOMERGE=true "$@"; }
escalate CI_STATE=red "CI_REASON=failed: build (ubuntu-latest) (failure)" TOUCHES_GO=true CLAUDE_FIX=true
check "red, Go, Claude allowed: claude" "$(out_value escalated)" claude
check "...starts the fix workflow for the PR and commit" "$(grep '^workflow run' "$fake/calls")" "workflow run security-fix.yml --ref main -f pr=7 -f sha=$sha"
contains "...leaves the assessment marker" "$(cat "$fake/comment.1")" "<!-- security-eligible:$sha -->"
check "...no issue yet" "$(count '^issue create')" 0
escalate CI_STATE=red "CI_REASON=failed: docs (failure)" TOUCHES_GO=false CLAUDE_FIX=true
check "red without Go: a person" "$(out_value escalated)" person
check "...no Claude" "$(count '^workflow run')" 0
check "...one issue" "$(count '^issue create')" 1
escalate CI_STATE=red "CI_REASON=x" TOUCHES_GO=true CLAUDE_FIX=false
check "red, Claude not allowed: a person" "$(out_value escalated)" person
escalate CI_STATE=timeout "CI_REASON=still waiting" TOUCHES_GO=true CLAUDE_FIX=true
check "timeout: a person" "$(out_value escalated)" person
escalate CI_STATE=moved TOUCHES_GO=true CLAUDE_FIX=true
check "moved: nothing" "$(writes)" ""
escalate CI_STATE=red SECURITY_AUTOMERGE=false TOUCHES_GO=true CLAUDE_FIX=true
check "switch off: nothing" "$(writes)" ""

# --- security-fix-prepare.sh --------------------------------------------------------------
echo "== fix prepare: whether Claude may try"
fresh
echo '[]' >"$fake/issues.json"
branch 'dependabot[bot]' "$dref"
bump_flock
commit_as dependabot "chore(deps): bump flock"
sha=$(head_sha)
marker="<!-- security-eligible:$sha -->"
prepare_fix() { : >"$fake/calls"; run security-fix-prepare.sh PR=7 "SHA=$sha" SECURITY_AUTOMERGE=true SECURITY_CLAUDE_FIX=true "$@"; }
echo "[{\"user\":\"github-actions[bot]\",\"body\":\"$marker\\nCI failed\"}]" >"$fake/comments.json"
prepare_fix
check "everything in place: proceed" "$(out_value proceed)" true
check "...the branch" "$(out_value head_ref)" "$dref"
contains "...the attempt marker is left first" "$(cat "$fake/comment.2")" "<!-- security-fix-attempt -->"
prepare_fix
check "the second time: Claude already tried" "$(out_value proceed)" false
contains "...says so" "$(out_value reason)" "already tried"
echo "[{\"user\":\"github-actions[bot]\",\"body\":\"$marker\"}]" >"$fake/comments.json"
prepare_fix SECURITY_CLAUDE_FIX=false
check "Claude's own switch off" "$(out_value proceed)" false
prepare_fix SECURITY_AUTOMERGE=false
check "the main switch off" "$(out_value proceed)" false
check "...no write" "$(writes)" ""
echo "[{\"user\":\"octocat\",\"body\":\"$marker\"}]" >"$fake/comments.json"
prepare_fix
check "a marker by a person does not count" "$(out_value proceed)" false
contains "...says no assessment" "$(out_value reason)" "no assessment"
echo "[{\"user\":\"github-actions[bot]\",\"body\":\"<!-- security-eligible:$(printf 'c%.0s' {1..40}) -->\"}]" >"$fake/comments.json"
prepare_fix
check "a marker for another commit does not count" "$(out_value proceed)" false
echo "[{\"user\":\"github-actions[bot]\",\"body\":\"$marker\"}]" >"$fake/comments.json"
prepare_fix "SHA=$(printf 'd%.0s' {1..40})"
check "the head is not the assessed commit" "$(out_value proceed)" false
echo "fixed" >>"$w/src/cmd/raid.go"
sed -i 's/0.18.0/0.18.1/' "$w/src/resources/app.properties"
commit_as bot "fix(deps): adapt code to the dependency update"
sha=$(head_sha)
echo "[{\"user\":\"github-actions[bot]\",\"body\":\"<!-- security-eligible:$sha -->\"}]" >"$fake/comments.json"
prepare_fix "SHA=$sha"
check "a PR that already has a fix: not again" "$(out_value proceed)" false

# --- security-fix-land.sh ------------------------------------------------------------------
echo "== fix land: Claude's patch is data"
land_setup() { # a PR branch with the Dependabot bump, checked out in $tmp/pr
  fresh
  echo '[]' >"$fake/issues.json"
  echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$fake/osv-0.13.0.json"
  branch 'dependabot[bot]' "$dref"
  bump_flock
  commit_as dependabot "chore(deps): bump flock"
  sha=$(head_sha)
  rm -rf "$tmp/pr"
  git clone -q "$fake/remote.git" "$tmp/pr" 2>/dev/null
  git -C "$tmp/pr" checkout -q "$sha"
  echo "[{\"user\":\"github-actions[bot]\",\"body\":\"<!-- security-eligible:$sha -->\"}]" >"$fake/comments.json"
  for n in 0; do checks $n completed success; done
  : >"$fake/calls"
}
# make_patch: runs the code of $1 in $tmp/pr, keeps the diff as the patch, restores the checkout.
make_patch() {
  (cd "$tmp/pr" && eval "$1" && git add -A && git diff --cached --binary >"$tmp/fix.patch" && git reset -q --hard && git clean -fdq)
}
land() { : >"$fake/calls"; run security-fix-land.sh PR=7 "SHA=$sha" "PATCH=$tmp/fix.patch" "PR_DIR=$tmp/pr" "PUSH_URL=$fake/remote.git" SECURITY_AUTOMERGE=true SECURITY_CLAUDE_FIX=true TIMEOUT_MINUTES=0 "$@"; }
refused() { # name want
  check "$1: result" "$(out_value result)" rejected
  contains "$1: reason" "$(out_value reason)" "$2"
  check "$1: nothing pushed" "$(git -C "$fake/remote.git" rev-parse "refs/heads/$ref")" "$sha"
  check "$1: no write" "$(writes)" ""
}

land_setup
make_patch 'echo "// adapted" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land
check "a good patch: merged" "$(out_value result)" merged
new=$(git -C "$fake/remote.git" rev-parse "refs/heads/$ref")
check "the branch moved on by one commit" "$(git -C "$fake/remote.git" rev-list --count "$sha..$new")" 1
check "the commit's subject" "$(git -C "$fake/remote.git" log -1 --format=%s "$new")" "fix(deps): adapt code to the dependency update"
check "authored by the workflow's bot" "$(git -C "$fake/remote.git" log -1 --format=%an "$new")" "github-actions[bot]"
check "the fix is in" "$(git -C "$fake/remote.git" show "$new:src/cmd/raid.go" | tail -n 1)" "// adapted"
check "the manifests are the Dependabot's" "$(git -C "$fake/remote.git" diff "$sha" "$new" --name-only | sort | paste -sd ',' -)" "src/cmd/raid.go,src/resources/app.properties"
check "CI was started on the branch" "$(grep '^workflow run' "$fake/calls" | head -n 2 | paste -sd ',' -)" "workflow run build.yml --ref $dref,workflow run codecov.yml --ref $dref"
check "the merge was of the pushed commit" "$(grep '^pr merge' "$fake/calls")" "pr merge 7 --squash --delete-branch --match-head-commit $new"
land PUSH_TOKEN=tok
check "(with a PUSH_TOKEN the CI is not dispatched: the push starts it)" "$(count '^workflow run build.yml --ref dependabot')" 0

land_setup
: >"$tmp/fix.patch"
land
check "an empty patch: nothing to land" "$(out_value result)" no-change
check "...no write" "$(writes)" ""
land_setup
rm -f "$tmp/fix.patch"
land
check "no patch at all: nothing to land" "$(out_value result)" no-change

land_setup
make_patch 'echo "x" >>.github/workflows/ci.yml; echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land
refused "a workflow in the patch" "under .github/"
land_setup
make_patch 'sed -i "s/0.13.1/0.13.2/" go.mod; echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land
refused "go.mod in the patch" "may not change the dependency manifests"
land_setup
make_patch 'echo x >>site/package-lock.json'
land
refused "the npm lock in the patch" "may not change the dependency manifests"
land_setup
make_patch 'echo "#!/bin/sh" >install.sh'
land
refused "a file outside the allowlist" "outside what a fix may change"
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go'
land
refused "src/ without a version bump" "without a version bump"
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties; echo "debug=true" >>src/resources/app.properties'
land
refused "app.properties changed beyond the version" "without a version bump"
land_setup
make_patch 'head -c 2000 /dev/urandom >src/cmd/blob.bin; echo "// x" >>src/cmd/raid.go'
land
refused "a binary file" "binary"
land_setup
make_patch 'ln -s /etc/passwd src/cmd/link.go 2>/dev/null || true; echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
if grep -q '120000' "$tmp/fix.patch"; then
  land
  refused "a symlink" "symbolic link"
else
  echo "(no symlinks here: the symlink case is skipped)"
fi
land_setup
make_patch 'seq 1 700 >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land
refused "a patch that is too big" "more than 600"
land_setup
printf 'this is not a patch\n' >"$tmp/fix.patch"
land
check "a patch that does not apply: result" "$(out_value result)" rejected
check "...nothing pushed" "$(git -C "$fake/remote.git" rev-parse "refs/heads/$ref")" "$sha"
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land SECURITY_CLAUDE_FIX=false
refused "Claude's switch off" "kill switch"
land SECURITY_AUTOMERGE=false
refused "the main switch off" "kill switch"

land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
echo "1" >"$fake/tags2"; echo v0.18.1 >>"$fake/tags"
land
check "the tag of the version exists: not pushed on, rejected" "$(out_value result)" rejected
contains "...says so" "$(out_value reason)" "tag v0.18.1 exists"
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
checks 0 completed failure
land
check "CI red with the fix: result" "$(out_value result)" ci-red
check "...nothing merged" "$(count '^pr merge')" 0
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
checks 0 queued null
land
check "CI never finishes: result" "$(out_value result)" ci-timeout
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
land FAKE_MERGE_FAIL=1
check "GitHub refuses the merge: result" "$(out_value result)" not-merged
# Someone pushed to the branch meanwhile: the push is refused.
land_setup
make_patch 'echo "// x" >>src/cmd/raid.go; sed -i "s/0.18.0/0.18.1/" src/resources/app.properties'
(cd "$w" && git checkout -q "$ref" && echo "late" >>go.sum && git add -A && git -c user.name=P -c user.email=p@example.com commit -q -m late && git push -q -f origin "HEAD:refs/heads/$ref")
land
check "the branch moved: the push is refused" "$(out_value result)" moved
check "...nothing merged" "$(count '^pr merge')" 0

# --- security-scan.sh -------------------------------------------------------------------------
echo "== scan: finding and fixing, on the runner"
scan_setup() {
  fresh
  rm -rf "$tmp/scan"
  git clone -q "$fake/remote.git" "$tmp/scan" 2>/dev/null
  rm -f "$fake"/npm-* "$fake"/vuln.json "$fake/dependabot_prs.txt"
  : >"$fake/calls"
}
cat >"$tmp/bin/npm" <<'EOF'
#!/usr/bin/env bash
echo "npm $*" >>"$FAKE_DIR/calls"
case "$*" in
  "audit --json --package-lock-only") cat "$FAKE_DIR/npm-audit.json" 2>/dev/null || echo '{}'; exit 1 ;;
  "audit fix --package-lock-only --no-audit --no-fund") [[ -f $FAKE_DIR/npm-fixed-lock.json ]] && cp "$FAKE_DIR/npm-fixed-lock.json" package-lock.json; exit 0 ;;
  "ci --no-audit --no-fund") exit 0 ;;
  "run build") [[ -z ${FAKE_NPM_BUILD_FAIL:-} ]] || { echo "Error: broken link on /docs/x"; exit 1; } ;;
  *) echo "unexpected npm call: $*" >&2; exit 99 ;;
esac
EOF
cat >"$tmp/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "go $*" >>"$FAKE_DIR/calls"
case "$1 ${2:-}" in
  "get "*) mod=${2%@*}; ver=${2#*@}; sed -i "s#\(${mod//\//\\/} \)v[0-9.]*#\1$ver#" go.mod ;;
  "mod tidy") ;;
  "build ./..."|"test ./...") [[ -z ${FAKE_GO_FAIL:-} ]] || { echo "src/cmd/raid.go:3: undefined: flock.New"; exit 1; } ;;
  *) echo "unexpected go call: $*" >&2; exit 99 ;;
esac
EOF
cat >"$tmp/bin/govulncheck" <<'EOF'
#!/usr/bin/env bash
echo "govulncheck $*" >>"$FAKE_DIR/calls"
cat "$FAKE_DIR/vuln.json" 2>/dev/null || true
EOF
chmod +x "$tmp/bin/npm" "$tmp/bin/go" "$tmp/bin/govulncheck"
write_fixed_lock() { sed 's/"version": "7.5.0"/"version": "7.5.1"/; s#ws-7.5.0#ws-7.5.1#' "$tmp/scan/site/package-lock.json" >"$fake/npm-fixed-lock.json"; }
scan() { (cd "$tmp/scan" && rm -rf scan-output) ; run_in "$tmp/scan" security-scan.sh "$@"; }
# run in another folder than $tmp/work
run_in() {
  local dir=$1 script=$2
  shift 2
  : >"$tmp/out"
  : >"$tmp/summary"
  code=0
  (cd "$dir" && env GITHUB_OUTPUT="$tmp/out" GITHUB_STEP_SUMMARY="$tmp/summary" "$@" bash "$scripts/$script") >"$tmp/log" 2>&1 || code=$?
  out=$(cat "$tmp/out")
}
echo '{"vulnerabilities":{"ws":{"severity":"high","fixAvailable":true},"webpack-dev-server":{"severity":"moderate","fixAvailable":{"name":"webpack-dev-server","version":"6.0.0","isSemVerMajor":true}}}}' >"$tmp/audit.json"
vuln_line='{"finding":{"osv":"GO-2026-0001","fixed_version":"v0.13.1","trace":[{"module":"github.com/gofrs/flock","version":"v0.13.0"}]}}'

scan_setup
cp "$tmp/audit.json" "$fake/npm-audit.json"
write_fixed_lock
scan
check "npm: exit" "$code" 0
check "npm: changed" "$(out_value changed)" true
check "npm: no code fix needed" "$(out_value needs_fix)" false
check "npm: the change" "$(cat "$tmp/scan/scan-output/changes.txt")" $'npm\tws\t7.5.0\t7.5.1'
check "npm: the patch touches only the lock" "$(grep '^diff --git' "$tmp/scan/scan-output/fix.patch" | paste -sd ',' -)" "diff --git a/site/package-lock.json b/site/package-lock.json"
contains "npm: the major fix is left alone, and said so" "$(cat "$tmp/scan/scan-output/skipped.txt")" "webpack-dev-server (moderate): the fix is a semver-major bump"
check "npm: the docs were built after the fix" "$(grep -c '^npm run build' "$fake/calls" || true)" 1
check "npm: nothing else touched in the checkout's index" "$(git -C "$tmp/scan" diff --cached --name-only)" ""
check "npm: no problems" "$(out_value problems)" 0

scan_setup
cp "$tmp/audit.json" "$fake/npm-audit.json"
write_fixed_lock
scan FAKE_NPM_BUILD_FAIL=1
check "the docs build breaks: nothing changed" "$(out_value changed)" false
contains "...a problem says so" "$(cat "$tmp/scan/scan-output/problems.txt")" "broke the docs build"
contains "...with the end of the log" "$(cat "$tmp/scan/scan-output/problems.txt")" "broken link"
check "...the lock is back as it was" "$(git -C "$tmp/scan" status --porcelain | grep -v '^?? scan-output/' || true)" ""
check "...no patch" "$([[ -e $tmp/scan/scan-output/fix.patch ]] && echo yes || echo no)" no

scan_setup
cp "$tmp/audit.json" "$fake/npm-audit.json"
sed 's/"version": "7.5.0"/"version": "8.0.0"/' "$tmp/scan/site/package-lock.json" >"$fake/npm-fixed-lock.json"
scan
check "a fix that moves a package to another major is thrown away" "$(out_value changed)" false
contains "...a person is told" "$(cat "$tmp/scan/scan-output/problems.txt")" "ws jumps from 7.5.0 to 8.0.0"
check "...and the docs were not even built" "$(grep -c '^npm run build' "$fake/calls" || true)" 0

scan_setup
cp "$tmp/audit.json" "$fake/npm-audit.json"
write_fixed_lock
echo "chore(deps): bump ws from 7.5.0 to 7.5.1 dependabot/npm_and_yarn/site/ws-7.5.1" >"$fake/dependabot_prs.txt"
scan
check "an open Dependabot PR covers the package: left to it" "$(out_value changed)" false
contains "...said in the summary" "$(cat "$tmp/scan/scan-output/skipped.txt")" "npm ws (high): an open Dependabot PR covers it"
check "...npm audit fix was not run" "$(grep -c 'audit fix' "$fake/calls" || true)" 0
scan_setup
cp "$tmp/audit.json" "$fake/npm-audit.json"
write_fixed_lock
echo "chore(deps): bump news-reader from 1.0.0 to 1.0.1 dependabot/npm_and_yarn/site/news-reader-1.0.1" >"$fake/dependabot_prs.txt"
scan
check "a PR for another package with a similar name does not cover it" "$(out_value changed)" true

scan_setup
scan
check "nothing to fix: no change" "$(out_value changed)" false
check "...no problem" "$(out_value problems)" 0

echo "== scan: Go"
scan_setup
echo "$vuln_line" >"$fake/vuln.json"
scan
check "go: changed" "$(out_value changed)" true
check "go: the change" "$(cat "$tmp/scan/scan-output/changes.txt")" $'go\tgithub.com/gofrs/flock\tv0.13.0\tv0.13.1'
check "go: built and tested" "$(grep -c -e '^go build' -e '^go test' "$fake/calls" || true)" 2
check "go: asked govulncheck at module level" "$(grep -c '^govulncheck -scan module -json' "$fake/calls" || true)" 1
check "go: the patch touches go.mod" "$(grep -c '^diff --git a/go.mod' "$tmp/scan/scan-output/fix.patch" || true)" 1
scan_setup
echo "$vuln_line" >"$fake/vuln.json"
scan FAKE_GO_FAIL=1
check "go: the build breaks, Claude not allowed: thrown away" "$(out_value changed)" false
contains "go: ...a person is told" "$(cat "$tmp/scan/scan-output/problems.txt")" "needs a code change"
check "go: ...go.mod is back" "$(git -C "$tmp/scan" status --porcelain | grep -v '^?? scan-output/' || true)" ""
scan_setup
echo "$vuln_line" >"$fake/vuln.json"
scan FAKE_GO_FAIL=1 SECURITY_CLAUDE_FIX=true
check "go: the build breaks, Claude allowed: kept for Claude" "$(out_value changed)" true
check "go: ...needs_fix" "$(out_value needs_fix)" true
check "go: ...not reported as a problem" "$(out_value problems)" 0
scan_setup
echo "$vuln_line" >"$fake/vuln.json"
cp "$tmp/audit.json" "$fake/npm-audit.json"
write_fixed_lock
scan FAKE_GO_FAIL=1 SECURITY_CLAUDE_FIX=true
check "go broken with npm changes pending: npm goes on alone" "$(out_value changed)" true
check "go: ...no Claude yet" "$(out_value needs_fix)" false
contains "go: ...told" "$(cat "$tmp/scan/scan-output/problems.txt")" "thrown away"
scan_setup
echo '{"finding":{"osv":"GO-2026-0003","fixed_version":"go1.25.9","trace":[{"module":"stdlib","version":"go1.25.5"}]}}' >"$fake/vuln.json"
scan
check "go: a standard library fix is not ours" "$(out_value changed)" false
contains "go: ...said in the summary" "$(cat "$tmp/scan/scan-output/skipped.txt")" "Go toolchain"

# --- security-open-pr.sh ----------------------------------------------------------------------
echo "== open the PR"
open_setup() { # a patch from the npm scan, and a fresh clone of main to open the PR from
  scan_setup
  cp "$tmp/audit.json" "$fake/npm-audit.json"
  write_fixed_lock
  scan
  rm -rf "$tmp/openw"
  git clone -q "$fake/remote.git" "$tmp/openw" 2>/dev/null
  : >"$fake/calls"
  rm -f "$fake/pr.json" "$fake/comments.json" "$fake/open_security_pr"
  echo '{"number":31,"author":"github-actions[bot]","ref":"security/2026-10-29","base":"main","state":"open","draft":false,"repo":"example/repo"}' >"$fake/pr.json"
  echo '[]' >"$fake/issues.json"
  checks 0 completed success
  node -e '
    const names = ["docs"];
    require("fs").writeFileSync(process.argv[1], JSON.stringify(names.map((name) => ({ name, status: "completed", conclusion: "success" }))));
  ' "$fake/checks.0.json"
}
open_pr() {
  : >"$fake/calls"
  run_in "$tmp/openw" security-open-pr.sh "PATCH=$tmp/scan/scan-output/fix.patch" "CHANGES=$tmp/scan/scan-output/changes.txt" "PUSH_URL=$fake/remote.git" \
    TODAY=2026-10-29 SECURITY_AUTOMERGE=true SECURITY_CLAUDE_FIX=false TIMEOUT_MINUTES=0 "$@"
}
open_setup
open_pr
check "exit" "$code" 0
check "merged" "$(out_value result)" merged
check "the branch is named for the date" "$(git -C "$fake/remote.git" branch --list 'security/*' | tr -d ' ')" "security/2026-10-29"
check "one commit on main's tip, with the title" "$(git -C "$fake/remote.git" log -1 --format=%s security/2026-10-29)" "chore(deps): fix security advisories (1)"
check "the PR title" "$(node -e 'console.log(JSON.parse(require("fs").readFileSync(process.argv[1])).title)' "$fake/pr-created")" "chore(deps): fix security advisories (1)"
check "base and head" "$(node -e 'const p=JSON.parse(require("fs").readFileSync(process.argv[1]));console.log(p.base+" "+p.head)' "$fake/pr-created")" "main security/2026-10-29"
contains "the package is in the PR text as code" "$(cat "$fake/pr-body.md")" '- npm: `ws` `7.5.0` to `7.5.1`'
contains "labels" "$(grep '^pr create' "$fake/calls")" "--label dependencies --label security"
check "CI started on the branch" "$(grep -c '^workflow run build.yml --ref security/2026-10-29' "$fake/calls" || true)" 1
check "it waited for the docs build" "$(count 'check-runs')" 1
check "and merged" "$(count '^pr merge 31 --squash')" 1

open_setup
open_pr SECURITY_AUTOMERGE=false
check "switch off: the PR is opened" "$(out_value result)" opened
check "...but not merged" "$(count '^pr merge')" 0
check "...and no CI is started" "$(count '^workflow run')" 0

open_setup
echo 29 >"$fake/open_security_pr"
open_pr
check "a security PR is still open: skipped" "$(out_value result)" skipped
check "...nothing written" "$(writes)" ""

open_setup
open_pr FAKE_PR_CREATE_FAIL=1
check "PRs cannot be created: reported" "$(out_value result)" reported
contains "...the issue says what to switch on" "$(cat "$fake/issue-create.md")" "Allow GitHub Actions to create and approve pull requests"
check "...the branch is deleted again" "$(count '^api -X DELETE')" 1

open_setup
checks 0 completed failure
node -e 'require("fs").writeFileSync(process.argv[1], JSON.stringify([{name:"docs",status:"completed",conclusion:"failure"}]))' "$fake/checks.0.json"
open_pr
check "CI fails: reported, not merged" "$(out_value result)" reported
check "...no merge" "$(count '^pr merge')" 0
check "...one issue" "$(count '^issue create')" 1

open_setup
open_pr NEEDS_FIX=true
check "needs a code fix, Claude not allowed: reported" "$(out_value result)" reported
open_setup
open_pr NEEDS_FIX=true SECURITY_CLAUDE_FIX=true
check "needs a code fix, Claude allowed: the fix workflow starts" "$(out_value result)" opened
check "...for this PR and commit" "$(grep '^workflow run security-fix.yml' "$fake/calls" | sed 's/ -f sha=.*/ -f sha=SHA/')" "workflow run security-fix.yml --ref main -f pr=31 -f sha=SHA"
contains "...with the assessment marker" "$(cat "$fake/comment.1")" "<!-- security-eligible:"
check "...and no merge by this script" "$(count '^pr merge')" 0

open_setup
cat "$tmp/scan/scan-output/fix.patch" >"$tmp/evil.patch"
sed -i 's#a/site/package-lock.json#a/.github/workflows/ci.yml#g; s#b/site/package-lock.json#b/.github/workflows/ci.yml#g' "$tmp/evil.patch"
run_in "$tmp/openw" security-open-pr.sh "PATCH=$tmp/evil.patch" "CHANGES=$tmp/scan/scan-output/changes.txt" "PUSH_URL=$fake/remote.git" TODAY=2026-10-29 SECURITY_AUTOMERGE=true
check "a patch that is not a manifest change is refused" "$(out_value result)" reported
check "...nothing pushed" "$(git -C "$fake/remote.git" branch --list 'security/*')" ""
open_setup
printf '%s\n' '{"x":1}' >"$tmp/scan/scan-output/changes.txt"
open_pr
check "a patch with no recognisable package changes: skipped" "$(out_value result)" skipped
open_setup
printf 'npm\tws"; rm -rf /\t7.5.0\t7.5.1\nnpm\tws\t7.5.0\t7.5.1\n' >"$tmp/scan/scan-output/changes.txt"
open_pr
body=$(cat "$fake/pr-body.md")
lacks "a bad package name is not put in the PR text" "$body" 'rm -rf'
contains "...the good one is" "$body" '`ws`'
open_setup
git -C "$fake/remote.git" branch security/2026-10-29 main
open_pr
check "the branch name is taken: the next one" "$(git -C "$fake/remote.git" branch --list 'security/*' | tr -d ' ' | paste -sd ',' -)" "security/2026-10-29,security/2026-10-29-2"

finish

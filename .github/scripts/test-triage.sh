#!/usr/bin/env bash
# Tests the issue triage's scripts (triage.yml) with a fake `gh` that records
# its calls, in a throwaway repository:
#   - triage-prepare.sh: who may start a triage (the public-repository
#     policy: maintainers' issues, the `triage` label added by a maintainer,
#     by hand), who and what is skipped, what it gathers (e-mail withheld,
#     the other issues, tags, history), and that it only reads;
#   - triage-apply.sh with triage-verdict.mjs: a valid verdict gives exactly
#     the labels and one comment; every kind of invalid verdict gives no
#     write at all; the text is made safe; only the one issue is touched.
# Nothing leaves the temp folder; no network.
#
#   bash .github/scripts/test-triage.sh
set -euo pipefail
source "$(dirname "$0")/test-lib.sh"
isolate_git

fake=$tmp/fake
mkdir -p "$fake"
export FAKE_DIR=$fake GITHUB_REPOSITORY=example/repo

# The fake gh: logs each call to $FAKE_DIR/calls, answers the reads from the
# files in $FAKE_DIR (issue_view.json, issue_list.json), answers the
# permission lookup with $FAKE_PERMISSION (default write), keeps the body of
# each comment, and refuses any call the scripts should not make.
cat >"$tmp/bin/gh" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_DIR/calls"
case "$1 ${2:-}" in
  "issue view") cat "$FAKE_DIR/issue_view.json" ;;
  "issue list") cat "$FAKE_DIR/issue_list.json" ;;
  "issue comment")
    n=$(ls "$FAKE_DIR"/comment.* 2>/dev/null | wc -l)
    while (($#)); do
      [[ $1 == --body-file ]] && cp "$2" "$FAKE_DIR/comment.$((n + 1))"
      shift
    done
    ;;
  "label create" | "issue edit") ;;
  "api repos/example/repo/collaborators/"*)
    [[ ${FAKE_PERMISSION:-write} != fail ]] || { echo "HTTP 404" >&2; exit 1; }
    echo "${FAKE_PERMISSION:-write}"
    ;;
  *) echo "unexpected gh call: $*" >&2; exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/gh"

# --- A repository: two releases, a preview tag and a commit after them -------
mkdir -p "$tmp/work/src/cmd" "$tmp/work/src/resources" "$tmp/work/docs"
cd "$tmp/work"
git init -q -b main
seq 1 50 | sed 's/^/line /' >src/cmd/refit.go
echo "design" >docs/DESIGN.md
printf 'version=0.1.0\r\nenvironment=development\r\n' >src/resources/app.properties
git add -A && git commit -qm "First"
GIT_COMMITTER_DATE="2026-10-01T10:00:00" git tag -a v0.1.0 -m 0.1.0
echo more >>docs/DESIGN.md && git commit -qam "Second"
GIT_COMMITTER_DATE="2026-10-02T10:00:00" git tag -a v0.2.0 -m 0.2.0
GIT_COMMITTER_DATE="2026-10-03T10:00:00" git tag -a v0.3.0-preview -m preview
echo more >>docs/DESIGN.md && git commit -qam "Unreleased fix"

# --- Helpers ------------------------------------------------------------------
# The fake's issue 77: issue_json LABELS(comma separated) BODY [STATE] [AUTHOR].
issue_json() {
  node -e '
    const [labels, body, state, author] = process.argv.slice(1);
    console.log(JSON.stringify({
      number: 77, title: "[bug] Something", body, state,
      labels: labels ? labels.split(",").map((name) => ({ name })) : [],
      author: { login: author, is_bot: author.endsWith("[bot]") }, createdAt: "2026-10-28T11:37:12Z",
    }));
  ' "$1" "$2" "${3:-OPEN}" "${4:-8bitAlex}" >"$fake/issue_view.json"
}

# Starts a call: forgets the last one's calls and comments.
reset() {
  rm -f "$fake"/calls "$fake"/comment.*
  : >"$fake/calls"
}
calls() { cat "$fake/calls"; }
# The calls that write to GitHub (none of the reads).
writes() { grep -E '^(label|issue (comment|edit|close|reopen|delete|lock|unlock|transfer|pin|unpin|develop|create))' "$fake/calls" || true; }
count() { grep -c -- "$1" "$fake/calls" || true; }

# A new issue opened by a maintainer.
prepare() {
  reset
  run triage-prepare.sh ISSUE=77 EVENT_NAME=issues EVENT_ACTION=opened ACTOR=8bitAlex ASSOCIATION=OWNER "$@"
}
apply() {
  reset
  run triage-apply.sh ISSUE=77 RUN_URL=https://github.com/example/repo/actions/runs/42 SHA=0123456789abcdef0123 "$@"
}

# --- triage-prepare.sh --------------------------------------------------------
echo "== prepare: an issue a maintainer opened"
body=$'### Description\n\nIt breaks. Mail me at me@example.com.\n\nVersion 0.2.0 on macOS.\n'
issue_json bug "$body"
node -e '
  const day = 86400000, ago = (d) => new Date(Date.now() - d * day).toISOString();
  console.log(JSON.stringify([
    { number: 1, title: "open one", state: "OPEN", labels: [{ name: "bug" }], closedAt: null },
    { number: 2, title: "closed long ago", state: "CLOSED", labels: [], closedAt: ago(200) },
    { number: 3, title: "closed lately", state: "CLOSED", labels: [{ name: "bug" }], closedAt: ago(5) },
    { number: 77, title: "[bug] Something", state: "OPEN", labels: [], closedAt: null },
  ]));
' >"$fake/issue_list.json"
prepare
check "exit" "$code" 0
check "proceeds" "$(out_value proceed)" true
for f in issue.json issues.json releases.txt version.txt history.txt; do
  [[ -s triage-input/$f ]] && check "writes $f" ok ok || check "writes $f" missing ok
done
issue=$(cat triage-input/issue.json)
contains "issue keeps the description" "$issue" "It breaks."
contains "issue keeps the labels" "$issue" '"bug"'
check "no e-mail in the issue" "$(grep -c 'me@example.com' triage-input/issue.json || true)" 0
contains "e-mail withheld" "$issue" "[email withheld]"
issues=$(cat triage-input/issues.json)
contains "open issue listed" "$issues" '"number": 1,'
contains "recently closed issue listed" "$issues" '"number": 3,'
check "old closed issue left out" "$(grep -c '"number": 2,' triage-input/issues.json || true)" 0
check "newest release first" "$(head -n 1 triage-input/releases.txt)" "v0.3.0-preview 2026-10-03"
contains "releases has the older one" "$(cat triage-input/releases.txt)" "v0.1.0 2026-10-01"
check "the version of the code" "$(cat triage-input/version.txt)" "0.1.0"
history=$(cat triage-input/history.txt)
contains "history has the commits" "$history" "Unreleased fix"
check "it only reads" "$(writes)" ""
check "only reads: issue view/list and the permission" "$(grep -vcE '^(issue view 77 |issue list |api repos/example/repo/collaborators/8bitAlex/permission)' "$fake/calls" || true)" 0
contains "asks for the permission of the actor" "$(calls)" "api repos/example/repo/collaborators/8bitAlex/permission --jq .permission"

echo "== prepare: who may start a triage on a public repository"
issue_json bug "$body" OPEN stranger
for assoc in OWNER MEMBER COLLABORATOR; do
  prepare ASSOCIATION=$assoc
  check "opened by a $assoc: proceeds" "$(out_value proceed)" true
done
for assoc in NONE FIRST_TIME_CONTRIBUTOR FIRST_TIMER CONTRIBUTOR MANNEQUIN ""; do
  prepare ASSOCIATION=$assoc ACTOR=stranger
  check "opened by [$assoc]: exit" "$code" 0
  check "opened by [$assoc]: stops" "$(out_value proceed)" false
  check "opened by [$assoc]: reads nothing" "$(calls)" ""
done
prepare ASSOCIATION=COLLABORATOR FAKE_PERMISSION=read
check "a collaborator with read access only: stops" "$(out_value proceed)" false
contains "says why" "$(cat "$tmp/summary")" "does not have write access"
check "...and reads no issue" "$(count '^issue ')" 0
prepare ASSOCIATION=COLLABORATOR FAKE_PERMISSION=triage
check "a collaborator with triage access only: stops" "$(out_value proceed)" false
prepare FAKE_PERMISSION=fail
check "permission lookup fails: stops" "$(out_value proceed)" false
prepare FAKE_PERMISSION=admin
check "admin: proceeds" "$(out_value proceed)" true
prepare ACTOR=
check "no actor: stops" "$(out_value proceed)" false
for bad in 'a b' 'x/../y' 'a;b' '-x' 'x-' "$(printf 'a%.0s' {1..40})"; do
  prepare "ACTOR=$bad"
  check "actor [$bad]: stops" "$(out_value proceed)" false
  check "actor [$bad]: no api call" "$(count '^api ')" 0
done
prepare EVENT_ACTION=edited
check "other issue events: stop" "$(out_value proceed)" false
prepare EVENT_ACTION=assigned
check "assigned: stops" "$(out_value proceed)" false

echo "== prepare: the triage label, added by a maintainer, on a stranger's issue"
issue_json bug,triage "$body" OPEN stranger
prepare EVENT_ACTION=labeled LABEL=triage ASSOCIATION=NONE ACTOR=8bitAlex
check "a maintainer adds triage: proceeds" "$(out_value proceed)" true
contains "...and the permission checked is the labeler's" "$(calls)" "collaborators/8bitAlex/permission"
prepare EVENT_ACTION=labeled LABEL=triage ASSOCIATION=NONE ACTOR=stranger FAKE_PERMISSION=read
check "a user without write access adds triage: stops" "$(out_value proceed)" false
prepare EVENT_ACTION=labeled LABEL=triage ASSOCIATION=NONE ACTOR=stranger FAKE_PERMISSION=none
check "a user with no access: stops" "$(out_value proceed)" false
prepare EVENT_ACTION=labeled LABEL=bug
check "another label added: stops" "$(out_value proceed)" false
prepare EVENT_ACTION=labeled LABEL=Triage
check "label names are exact: stops" "$(out_value proceed)" false
prepare EVENT_ACTION=labeled LABEL=triage TRIGGER_LABEL=needs-look
check "the trigger label can be changed: the old one stops" "$(out_value proceed)" false
prepare EVENT_ACTION=labeled LABEL=needs-look TRIGGER_LABEL=needs-look
check "the trigger label can be changed: the new one proceeds" "$(out_value proceed)" true

echo "== prepare: who is skipped"
for l in triaged dependencies security; do
  issue_json "bug,$l" "$body"
  prepare
  check "a new issue labelled $l: exit" "$code" 0
  check "a new issue labelled $l: skipped" "$(out_value proceed)" false
  prepare EVENT_ACTION=labeled LABEL=triage
  check "triage label on an issue labelled $l: skipped" "$(out_value proceed)" false
done
issue_json bug,idea "$body"
prepare
check "other labels proceed" "$(out_value proceed)" true
issue_json bug "$body" OPEN "dependabot[bot]"
prepare
check "an issue by a bot: skipped" "$(out_value proceed)" false
issue_json bug "$body" OPEN "app/some-app"
prepare
check "an issue by an app: skipped" "$(out_value proceed)" false
issue_json bug "$body" CLOSED
prepare
check "a closed issue: skipped" "$(out_value proceed)" false

echo "== prepare: by hand"
issue_json triaged "$body"
prepare EVENT_NAME=workflow_dispatch EVENT_ACTION= ACTOR= ASSOCIATION=
check "by hand: triaged is skipped" "$(out_value proceed)" false
check "by hand: no permission lookup (GitHub requires write to dispatch)" "$(count '^api ')" 0
prepare EVENT_NAME=workflow_dispatch EVENT_ACTION= ACTOR= ASSOCIATION= FORCE=true
check "by hand with force: triaged proceeds" "$(out_value proceed)" true
issue_json security,dependencies "$body" OPEN stranger
prepare EVENT_NAME=workflow_dispatch EVENT_ACTION= ACTOR= ASSOCIATION=
check "by hand: any author and labels proceed" "$(out_value proceed)" true
issue_json bug "$body" CLOSED
prepare EVENT_NAME=workflow_dispatch EVENT_ACTION= ACTOR= ASSOCIATION=
check "by hand: closed is skipped" "$(out_value proceed)" false
prepare EVENT_NAME=workflow_dispatch EVENT_ACTION= ACTOR= ASSOCIATION= FORCE=true
check "by hand with force: closed proceeds" "$(out_value proceed)" true
prepare ISSUE=abc
check "not an issue number: exit" "$code" 1
prepare 'ISSUE=1;rm'
check "a command in the number: exit" "$code" 1
check "a command in the number: no call" "$(calls)" ""

# --- triage-apply.sh ----------------------------------------------------------
# The prepared input apply checks the verdict against, and the checkout.
mkdir -p triage-input triage-output
echo '[{"number":5,"title":"x","state":"open","labels":[]},{"number":77,"title":"y","state":"open","labels":[]}]' >triage-input/issues.json
printf 'v0.2.0 2026-10-02\nv0.1.0 2026-10-01\n' >triage-input/releases.txt
issue_json bug "ignored"

# Writes triage-output/verdict.json: a valid verdict `v`, changed by the
# JavaScript in $1 (which may also make v a string to write it as it is).
verdict() {
  node -e '
    let v = {
      category: "bug", priority: "high", summary: "Export fails for the default setup.",
      analysis: "The check in `refit_garments` fails before any refit.",
      suspected_files: [{ path: "src/cmd/refit.go", line: 12 }],
      duplicate_of: null, fixed_in: null, needs_info: [],
      next_step: "Dedupe the paths in the wardrobe.", claude_can_attempt: true,
    };
    eval(process.argv[1]);
    require("fs").writeFileSync(process.argv[2], typeof v === "string" ? v : JSON.stringify(v));
  ' "$1" triage-output/verdict.json
}

echo "== apply: a valid verdict"
verdict ''
apply
check "exit" "$code" 0
check "one comment" "$(count '^issue comment')" 1
check "the comment goes to the issue" "$(grep '^issue comment' "$fake/calls" | cut -d' ' -f1-3)" "issue comment 77"
check "one edit, exactly these labels" "$(grep '^issue edit' "$fake/calls")" \
  "issue edit 77 --add-label triaged --add-label bug --add-label priority: high"
check "labels created with --force" "$(grep -c '^label create .* --force ' "$fake/calls")" 5
contains "owns triaged" "$(calls)" "label create triaged --force"
contains "owns needs-info" "$(calls)" "label create needs-info --force"
contains "owns priority: high" "$(calls)" "label create priority: high --force"
check "never adds claude" "$(grep -c -- '--add-label claude' "$fake/calls" || true)" 0
check "no other write" "$(writes | grep -vcE '^(label create|issue comment 77|issue edit 77 )' || true)" 0
check "only issue 77 touched" "$(grep '^issue ' "$fake/calls" | awk '{ print $3 }' | sort -u)" 77
check "triaged output" "$(out_value triaged)" true
comment=$(cat "$fake/comment.1")
contains "heading" "$comment" "## Automated triage"
contains "category and priority" "$comment" "**Bug, priority high.** Export fails for the default setup."
contains "the analysis" "$comment" 'The check in `refit_garments` fails'
contains "suspected files as code" "$comment" '- `src/cmd/refit.go:12`'
contains "next step" "$comment" "**Next step.** Dedupe the paths in the wardrobe."
contains "fix attempt hint" "$comment" 'automated fix attempt; a maintainer decides.'
contains "says it is automated" "$comment" "automated Claude run"
contains "says it may be wrong" "$comment" "It may be wrong"
contains "names the run" "$comment" "(https://github.com/example/repo/actions/runs/42)"
contains "names the commit" "$comment" '`01234567`'

echo "== apply: every optional part"
verdict 'v.category = "enhancement"; v.priority = "low"; v.duplicate_of = 5; v.fixed_in = "0.2.0"; v.needs_info = ["Which body?", "Is a mod loaded?"]; v.claude_can_attempt = false; v.suspected_files = []'
apply
check "exit" "$code" 0
check "labels" "$(grep '^issue edit' "$fake/calls")" \
  "issue edit 77 --add-label triaged --add-label enhancement --add-label priority: low --add-label needs-info --add-label duplicate"
comment=$(cat "$fake/comment.1")
contains "duplicate" "$comment" "**Possible duplicate of** #5."
contains "fixed in" "$comment" "**Fixed in** 0.2.0"
contains "questions" "$comment" "**Questions for the reporter**"
contains "a question" "$comment" "- Which body?"
check "no fix hint when it cannot" "$(grep -c 'automated fix attempt' "$fake/comment.1" || true)" 0
check "no code section without files" "$(grep -c 'Suspected code' "$fake/comment.1" || true)" 0

echo "== apply: an older priority label of ours is taken off"
issue_json bug,priority:\ low x
verdict ''
apply
check "exit" "$code" 0
contains "removes the old priority" "$(grep '^issue edit' "$fake/calls")" "--remove-label priority: low"
issue_json bug,priority:\ high x
apply
check "keeps the same priority" "$(grep -c -- '--remove-label' "$fake/calls" || true)" 0
issue_json bug x

echo "== apply: the label that asked for the triage is taken off"
issue_json bug,triage x
apply
check "exit" "$code" 0
check "removes the trigger label" "$(grep '^issue edit' "$fake/calls")" \
  "issue edit 77 --add-label triaged --add-label bug --add-label priority: high --remove-label triage"
issue_json bug,triage-later x
apply
check "only that label" "$(grep -c -- '--remove-label' "$fake/calls" || true)" 0
issue_json bug x

echo "== apply: a verdict that fails validation posts nothing"
expect_refused() {
  local name=$1 js=$2
  verdict "$js"
  apply
  check "$name: exit" "$code" 1
  check "$name: no write" "$(writes)" ""
  check "$name: refused" "$(out_value refused)" true
  contains "$name: says so" "$(cat "$tmp/summary")" "failed validation"
  contains "$name: nothing" "$(cat "$tmp/summary")" "not marked"
}
expect_refused "bad category" 'v.category = "urgent"'
expect_refused "category case" 'v.category = "Bug"'
expect_refused "bad priority" 'v.priority = "critical"'
expect_refused "priority number" 'v.priority = 1'
expect_refused "summary too long" 'v.summary = "x".repeat(201)'
expect_refused "empty summary" 'v.summary = "  "'
expect_refused "summary not a string" 'v.summary = ["a"]'
expect_refused "analysis too long" 'v.analysis = "x".repeat(3501)'
expect_refused "next_step too long" 'v.next_step = "x".repeat(301)'
expect_refused "too many needs_info" 'v.needs_info = ["a", "b", "c", "d", "e", "f", "g"]'
expect_refused "needs_info item too long" 'v.needs_info = ["x".repeat(201)]'
expect_refused "needs_info item not text" 'v.needs_info = [3]'
expect_refused "needs_info not a list" 'v.needs_info = "why"'
expect_refused "too many files" 'v.suspected_files = Array(9).fill({ path: "src/cmd/refit.go", line: 1 })'
expect_refused "duplicate_of unknown" 'v.duplicate_of = 999'
expect_refused "duplicate_of the issue itself" 'v.duplicate_of = 77'
expect_refused "duplicate_of a string" 'v.duplicate_of = "5"'
expect_refused "duplicate_of zero" 'v.duplicate_of = 0'
expect_refused "missing file" 'v.suspected_files = [{ path: "src/cmd/nope.go", line: 1 }]'
expect_refused "folder, not a file" 'v.suspected_files = [{ path: "src/cmd", line: 1 }]'
expect_refused "dot dot path" 'v.suspected_files = [{ path: "../../etc/passwd", line: 1 }]'
expect_refused "dot dot inside" 'v.suspected_files = [{ path: "src/../docs/DESIGN.md", line: 1 }]'
expect_refused "absolute path" 'v.suspected_files = [{ path: "/etc/passwd", line: 1 }]'
expect_refused "windows path" 'v.suspected_files = [{ path: "src\\cmd\\refit.go", line: 1 }]'
expect_refused "git folder" 'v.suspected_files = [{ path: ".git/HEAD", line: 1 }]'
expect_refused "triage-input" 'v.suspected_files = [{ path: "triage-input/issue.json", line: 1 }]'
expect_refused "line beyond the file" 'v.suspected_files = [{ path: "src/cmd/refit.go", line: 51 }]'
expect_refused "line zero" 'v.suspected_files = [{ path: "src/cmd/refit.go", line: 0 }]'
expect_refused "line a string" 'v.suspected_files = [{ path: "src/cmd/refit.go", line: "3" }]'
expect_refused "file item with a key more" 'v.suspected_files = [{ path: "src/cmd/refit.go", line: 3, note: "x" }]'
expect_refused "file item a string" 'v.suspected_files = ["src/cmd/refit.go"]'
expect_refused "fixed_in not released" 'v.fixed_in = "9.9.9"'
expect_refused "fixed_in not a version" 'v.fixed_in = "v0.2.0"'
expect_refused "fixed_in text" 'v.fixed_in = "soon"'
expect_refused "claude_can_attempt text" 'v.claude_can_attempt = "yes"'
expect_refused "an extra key" 'v.labels = ["claude"]'
expect_refused "a missing key" 'delete v.next_step'
expect_refused "an array" 'v = [v]'
expect_refused "null" 'v = "null"'
expect_refused "not JSON" 'v = "this is not JSON"'
expect_refused "empty file" 'v = ""'

# Where a symlink can be made (not on every Windows setup), one out of the tree.
if ln -s /etc/hostname src/cmd/link.go 2>/dev/null && [[ -L src/cmd/link.go ]]; then
  expect_refused "symlink" 'v.suspected_files = [{ path: "src/cmd/link.go", line: 1 }]'
  rm -f src/cmd/link.go
else
  rm -f src/cmd/link.go
  echo "(no symlinks here: the symlink case is skipped)"
fi

echo "== apply: the text can only be read"
verdict '
  const c = String.fromCharCode;
  v.summary = "Line one\nline two @bob";
  v.analysis = "Hi @octocat and @org/team.\n"
    + "<img src=x onerror=alert(1)> <!-- hidden --> <<b>script>x\n"
    + "[click](https://evil.example/x) ![p](https://evil.example/p.png) see https://evil.example/raw and www.evil.example, mail a.b@example.com.\n"
    + "Bell:" + c(7) + " RLO:" + c(0x202e) + "x ZW:" + c(0x200b) + "y Esc:" + c(27) + "[31m. Fix #12, example/other#3, src/a.rs:3.\n"
    + "A `Vec<String>` and `<img src=x>` in code.\n"
    + "```\nunclosed fence";
  v.needs_info = ["Ask <b>this</b> @carol?"];
  v.next_step = "Run [it](http://evil.example) now";
'
apply
check "exit" "$code" 0
comment=$(cat "$fake/comment.1")
zwsp=$(printf '\xe2\x80\x8b')
contains "one-line summary" "$comment" "Line one line two @${zwsp}bob"
contains "mention neutralised" "$comment" "@${zwsp}octocat"
contains "team mention neutralised" "$comment" "@${zwsp}org/team"
check "no live mention" "$(grep -cE '@[A-Za-z0-9_]' "$fake/comment.1" || true)" 0
check "no html tag (but our own footer)" "$(grep -vF '<sub>' "$fake/comment.1" | grep -cE '<[A-Za-z/!]' || true)" 0
check "no html comment" "$(grep -c 'hidden' "$fake/comment.1" || true)" 0
check "no onerror" "$(grep -c 'onerror' "$fake/comment.1" || true)" 0
check "no link or image" "$(grep -c 'evil.example' "$fake/comment.1" || true)" 0
check "no e-mail" "$(grep -c 'a.b@example.com' "$fake/comment.1" || true)" 0
contains "a link keeps its words" "$comment" "click"
contains "issue reference stays" "$comment" "Fix #12"
contains "file reference stays" "$comment" "src/a.rs:3"
check "no bell" "$(grep -c "$(printf '\a')" "$fake/comment.1" || true)" 0
check "no escape" "$(grep -c "$(printf '\033')" "$fake/comment.1" || true)" 0
check "no direction override" "$(grep -c "$(printf '\xe2\x80\xae')" "$fake/comment.1" || true)" 0
check "no stray zero width space but the mentions'" \
  "$(sed "s/@${zwsp}//g" "$fake/comment.1" | grep -c "$zwsp" || true)" 0
check "the open fence is closed" "$(($(grep -c '^```' "$fake/comment.1") % 2))" 0
contains "angle brackets in code stay readable" "$comment" "Vec$(printf '\xe2\x80\xb9')String>"
check "no tag in code either" "$(grep -vF '<sub>' "$fake/comment.1" | grep -c '<img' || true)" 0
contains "a question, as text" "$comment" "- Ask this @${zwsp}carol?"
contains "next step, as text" "$comment" "**Next step.** Run it now"

echo "== apply: no verdict, or already triaged"
rm -f triage-output/verdict.json
apply
check "no verdict: exit" "$code" 0
check "no verdict: no call" "$(calls)" ""
verdict ''
issue_json bug,triaged x
apply
check "triaged meanwhile: exit" "$code" 0
check "triaged meanwhile: no write" "$(writes)" ""
apply FORCE=true
check "forced: exit" "$code" 0
check "forced: one comment" "$(count '^issue comment')" 1
apply ISSUE=abc
check "not an issue number: exit" "$code" 1
check "not an issue number: no call" "$(calls)" ""

finish

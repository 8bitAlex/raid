# Maintainer automation

Workflows that use Claude, or merge by themselves. For maintainers; nothing here changes how the CLI behaves, so it needs no release (and no change to `src/resources/app.properties`: CI's version-check only looks at PRs that touch `src/`).

| Workflow | What it does | Runs when | Needs |
|---|---|---|---|
| `triage.yml` | Labels an issue and posts one comment (cause with `path:line`, duplicate, questions, next step) | a maintainer opens an issue, a maintainer adds the label `triage`, or by hand | Claude's credentials |
| `security-update.yml` (Lane 1) | Merges a Dependabot PR that fixes a security vulnerability, once CI is green | a Dependabot PR is opened or updated | `SECURITY_AUTOMERGE=true` |
| `security-fix.yml` (Lane 2) | Claude adapts the code when a security update breaks CI; the same checks decide the merge | started by Lane 1 or 3 | `SECURITY_AUTOMERGE=true`, `SECURITY_CLAUDE_FIX=true`, Claude's credentials |
| `security-scan.yml` (Lane 3) | Fixes vulnerabilities Dependabot opened no PR for, in one PR a day | daily, or by hand | `SECURITY_AUTOMERGE=true` (for the schedule) |
| `automation-tests.yml` | Tests the scripts behind all of these (fake `gh`, `curl`, `npm`, `go`) | a change under `.github/` | nothing |

`claude.yml` (the `@claude` mention, OAuth token) is a separate thing and unchanged.

GitHub runs `issues`, `pull_request_target`, `schedule` and `workflow_dispatch` workflows from the **default branch**, so none of the new workflows does anything until it is on `main`.

## Switching it on (checklist)

The merging is **off** until you do step 4. The triage needs steps 1 and 2.

1. **Claude's credentials**: the federation rule and the repository variables below.
2. **Create the label** `triage` (`gh label create triage --description "Ask for an automatic triage of this issue"`). The labels `triaged`, `needs-info` and `priority: high|medium|low` are created by the first triage; `bug`, `enhancement`, `question`, `documentation`, `invalid`, `duplicate` and `dependencies` exist already.
3. **Let Actions open PRs** (only Lane 3 needs it; it is off today): Settings > Actions > General > Workflow permissions > "Allow GitHub Actions to create and approve pull requests", or
   `gh api -X PUT repos/8bitAlex/raid/actions/permissions/workflow -f default_workflow_permissions=read -F can_approve_pull_request_reviews=true`.
4. **Turn the merging on**: `gh variable set SECURITY_AUTOMERGE --body true -R 8bitAlex/raid`. Optionally also `SECURITY_CLAUDE_FIX` (Lane 2, below).
5. Optional secrets, see "Optional secrets".

| Variable | Value | Effect |
|---|---|---|
| `SECURITY_AUTOMERGE` | `true` | **The kill switch.** Anything else (unset, `false`): Lane 1 and Lane 2 are skipped, the scheduled Lane 3 is skipped, and every script that could merge refuses again on its own. Set it to `false` (or delete it) to stop everything at once. |
| `SECURITY_CLAUDE_FIX` | `true` | Allows Lane 2: Claude may change code on a security PR, and that change is merged if CI passes. Separate from the kill switch because it costs money and is the only place Claude's output is merged. Needs `SECURITY_AUTOMERGE` too. |
| `ANTHROPIC_FEDERATION_RULE_ID`, `ANTHROPIC_ORGANIZATION_ID` | `fdrl_...`, a UUID | Claude's credentials (also read as secrets of the same name; a variable wins). |
| `ANTHROPIC_SERVICE_ACCOUNT_ID`, `ANTHROPIC_WORKSPACE_ID` | `svac_...`, `wrkspc_...` | Only if the rule needs them. |

Never put the rule id in the repository. Optional secrets:

| Secret | Why |
|---|---|
| `SECURITY_ALERTS_TOKEN` | A fine-grained token with "Dependabot alerts: read" for this repository. `GITHUB_TOKEN` cannot read Dependabot alerts, so `dependabot/fetch-metadata`'s `alert-state` is empty without it. With it, Lane 1 also accepts a PR whose alert is `OPEN`. Without it Lane 1 decides with OSV (below), which also works. |
| `SECURITY_BOT_TOKEN` | A token (fine-grained PAT or App) that may push branches and open PRs. Only needed if the ruleset's CodeQL rule blocks the merges of PRs the workflow token made (see "Verifying it on main"). Pushes and PRs made with it start workflows by themselves, so CI and CodeQL run natively and the manual CI start is skipped. The merge is still done by the workflow token. |

## Claude's credentials

Claude authenticates by Workload Identity Federation: the job asks GitHub for an OIDC token (`permissions: id-token: write`, on the Claude job only) and Anthropic exchanges it. There is no API key or OAuth token secret. `.github/scripts/claude-auth.sh` is the first step of every Claude job: it checks the variables are set, that GitHub gives the job a token, and that Anthropic accepts the exchange, and **prints the token's claims**, so a refusal says exactly what the rule did not match. The Claude Console's Workload identity page has the authentication history.

Facts of this repository (verified with `gh api`): repository id `966580385`, owner id `4384235` (`8bitAlex`). The OIDC subject is the GitHub **default** form; "immutable" subjects are not enabled (`gh api repos/8bitAlex/raid/actions/oidc/customization/sub`).

Claims the rule must accept (issuer `https://token.actions.githubusercontent.com`, audience `https://api.anthropic.com`):

| Claim | Value |
|---|---|
| `aud` | `https://api.anthropic.com` |
| `repository_owner` | `8bitAlex` |
| `repository` | `8bitAlex/raid` |
| `sub`, today | `repo:8bitAlex/raid:ref:refs/heads/main` |
| `sub`, if immutable subjects are turned on | `repo:8bitAlex@4384235/raid@966580385:ref:refs/heads/main` |

Which `ref` and `job_workflow_ref` each workflow presents. Every Claude job runs from `main` (issues, schedules and dispatches use the default branch), so the `sub` is the same one; the `job_workflow_ref` tells them apart:

| Workflow | Event | `ref` | `job_workflow_ref` |
|---|---|---|---|
| `triage.yml` | issue opened, label `triage` added, or `gh workflow run triage.yml --ref main` | `refs/heads/main` | `8bitAlex/raid/.github/workflows/triage.yml@refs/heads/main` |
| `security-fix.yml` | `workflow_dispatch` (by Lane 1 or Lane 3 with the workflow token, or by hand with `--ref main`) | `refs/heads/main` | `8bitAlex/raid/.github/workflows/security-fix.yml@refs/heads/main` |

`security-update.yml` (Lane 1, a `pull_request_target` run started by Dependabot) and `security-scan.yml` (Lane 3) **never ask for an OIDC token** (no `id-token` permission) and never call Claude themselves: Lane 1 hands a failing PR to `security-fix.yml` by `workflow_dispatch`, which is why Lane 2 is a separate workflow (a run started by Dependabot gets no `id-token`).

The narrowest rule: `sub` = `repo:8bitAlex/raid:ref:refs/heads/main` (exact), `repository_owner` = `8bitAlex`, audience as above; optionally also `job_workflow_ref` one of the two values. Never match on the repository without the `ref`: any branch someone pushes a workflow to would get a token. Running a workflow from another branch (`--ref some-branch`) presents another `ref` and is refused, which is intended.

**`claude-code-action` and who the actor is.** The action refuses an actor without write access (`src/github/validation/permissions.ts`: the `admin` or `write` collaborator permission; bots with a `[bot]` suffix pass). For issue events the actor is whoever caused the event: for `opened` the author, for `labeled` the person who added the label. For `workflow_dispatch` the action does not check (GitHub requires write access to dispatch). So the "maintainer adds the `triage` label to a stranger's issue" flow passes the action's check with the **maintainer** as the actor. `allowed_non_write_users` is therefore **not used** (it would also switch on a best-effort scrub of the environment and is described as risky for untrusted content), and neither is `allowed_bots`. The prepare job checks the same permission itself, through the API, before anything is read.

## Issue triage

`triage.yml`: three jobs, each with only the permissions it needs.

1. **prepare** (read-only; `triage-prepare.sh`) decides whether this run is allowed (below), then writes `triage-input/`: the issue (e-mail addresses withheld), the other open and recently closed issues, the release tags, the version in `app.properties`, the commit subjects.
2. **triage** (`contents: read`, `id-token: write`; model `claude-sonnet-5-5`, at most 25 turns, 20 minutes) runs Claude with `Read`, `Grep`, `Glob` and `Write` only: no shell, no web, no edits, and a read-only token. It reads the input, `CLAUDE.md`, `docs/`, `site/docs/` and `src/`, and writes `triage-output/verdict.json`.
3. **apply** (`issues: write`, no `id-token`; `triage-apply.sh`, `triage-verdict.mjs`) trusts nothing Claude wrote. It validates the verdict against a strict schema (enums, lengths, `duplicate_of` an issue in the list and not itself, `fixed_in` a release tag, every suspected file a real file of the checkout and not a link out of it, with a line it has), rewrites the text so it can only be read (no HTML, links, images, mentions or e-mail addresses), then creates the labels it owns, posts **one** comment and adds `triaged`, the category, `priority: ...` and, when they apply, `needs-info` and `duplicate`. A verdict that fails validation posts nothing; the run summary lists the reasons. It never closes, assigns or edits an issue, and removes the `triage` label it was asked with.

### The public-repository policy

raid is public: an issue is text from a stranger, and a run costs money. So:

* **Automatic** only for an issue opened by an `OWNER`, `MEMBER` or `COLLABORATOR` (`github.event.issue.author_association`, in the job `if`), and the script then checks, with the API, that the author has `admin` or `write` access (a read-only collaborator is refused).
* **A stranger's issue** is triaged when a maintainer adds the label `triage` (the `labeled` event). The person who added it must have write access, checked in the first job.
* **By hand**: `gh workflow run triage.yml --ref main -f issue=NN` (add `-f force=true` to triage an issue that already has `triaged`).
* **Skipped with no runner**: bots, and issues labelled `triaged`, `dependencies` or `security`. Closed issues are skipped unless forced.
* **One run per issue** at a time (the `concurrency` group; other labels get a group of their own, so they cannot push out a queued triage).
* **Hostile text**: Claude is told the input is data; it has no shell or network and no write token; the apply job is the only one that writes and it validates strictly. The worst a hostile issue can do is make Claude produce a verdict the validator rejects, or a wrong but harmless comment. Every comment says it was written by an automated Claude run and may be wrong.

**Widening it** (one line each, in `triage.yml`'s `prepare` job `if`): to triage every new issue, add `github.event.action == 'opened'` as an alternative to the association test (a stranger can then cost one run per issue; the actor would be the stranger, and `claude-code-action` would refuse an actor without write access, so you would also have to pass `allowed_non_write_users: '*'` with the read-only token, which is why this is not the default). To change the label that asks for a triage, change `'triage'` in the two places in the `if` and `TRIGGER_LABEL` in the scripts' environment. To skip other labels, add them to the `if` and to `skip_labels` in `triage-prepare.sh`.

**To triage the open backlog** by hand, one run each:

```sh
for n in $(gh issue list --state open --limit 100 --json number,labels \
    -q '.[] | select(any(.labels[].name; . == "triaged" or . == "dependencies" or . == "security") | not) | .number'); do
  gh workflow run triage.yml --ref main -f issue=$n
done
```

**Cost**: one Claude run per accepted issue: at most 25 turns, reading a few files. **Turning it off**: `gh workflow disable triage.yml`, or delete the file. Labels and comments it made stay.

## Security updates

Raid is PR-based and Dependabot already opens PRs (version updates for Go weekly; security updates for Go and for `site/` npm). Three lanes, all behind `SECURITY_AUTOMERGE`:

### Lane 1: a Dependabot PR that fixes a vulnerability (`security-update.yml`)

Runs on `pull_request_target` (the workflow file of the default branch, with a normal token), for PRs by `dependabot[bot]` from this repository.

**Why `pull_request_target`, and why it is safe.** A `pull_request` run started by Dependabot gets a read-only token and no OIDC token, so it could not merge or comment. `pull_request_target` removes that limit, and its danger is running the PR's code with the token. This workflow never does: its checkout has no `ref:` (it gets the default branch, which holds the scripts), and the PR is read **only through the API**: its commits, its file list, and the contents of four manifests, which are parsed and never executed (`security-assess.sh`, `security-policy.mjs`). `workflow_run` after CI was the alternative; it fires once per CI workflow (there are two: `build.yml` and `codecov.yml`), has to find the PR from the run, and the CI result is read through the API either way, so it adds plumbing and no safety.

What has to be true before it merges (each is a refusal otherwise, listed in the run summary):

1. The PR is open, not a draft, into `main`, from a `dependabot/go_modules/...` or `dependabot/npm_and_yarn/...` branch of this repository, by `dependabot[bot]`.
2. **Every commit** is by Dependabot (`dependabot/fetch-metadata` itself only verifies the first one). A commit by anyone else stops it.
3. It is a **security update**, not a version update. Either Dependabot's alert for it is `OPEN` (`fetch-metadata`'s `alert-state`; needs `SECURITY_ALERTS_TOKEN`), or **OSV** (`api.osv.dev`, which has the GitHub advisories for Go and npm) lists an advisory for the old version that no longer affects the new one. Plain version bumps are left alone.
4. **No semver-major bump of a direct dependency** (`dependency-type` and `update-type` of each updated dependency; `mcp-go` 0.55.1 to 1.1.0 in #154 is refused). Indirect dependencies may go up a major.
5. **Only these files change**: `go.mod`, `go.sum`, `site/package.json`, `site/package-lock.json`, within 8000 changed lines; nothing under `.github/`, ever.
6. **The manifests, compared with the merge base**: no new direct dependency, at most 5 new indirect ones, nothing in the npm lock that does not come from `registry.npmjs.org`, no changed `go`, `toolchain`, `replace` or `exclude` line in `go.mod`, no changed script or added or removed dependency in `site/package.json`.
7. **CI is green on that exact commit**: `build (ubuntu-latest)`, `build (macos-latest)`, `build (windows-latest)` and the coverage workflow's `build` for Go changes, `docs` for `site/` changes, nothing that ran may have failed, no status may be `failure`, and CodeQL is waited for if it started. It polls for up to 45 minutes.
8. Then `gh pr merge --squash --match-head-commit <sha>` as the workflow (GitHub refuses it if the PR moved since the commit was tested), and CI is started on `main` (a merge by the workflow token starts no workflow: `build.yml`, and `docs.yml` when `site/` changed).

**The version bump.** CI's `version-check` job (`build.yml`) only runs for PRs that touch `src/`. Dependency-only PRs (#119, #116, #117) change `go.mod`/`go.sum` or `site/package-lock.json` and pass without touching `app.properties`, so Lane 1 needs no bump and creates no release. A change to `src/` (only Lane 2 can make one) needs a higher version in `src/resources/app.properties` whose tag does not exist: Lane 2 is told to do it, and `security-assess.sh` checks it, because a dispatched CI run does not run `version-check`.

If CI fails and the change touches Go, and `SECURITY_CLAUDE_FIX` is `true`, Lane 2 is started; otherwise a comment on the PR and one `security` issue say why.

### Lane 2: Claude adapts the code (`security-fix.yml`)

Started with the PR and the assessed commit. Both switches must be `true`. Steps:

1. **prepare** (no Claude): the PR must carry a comment by `github-actions[bot]` recording the assessment of exactly that commit (`<!-- security-eligible:SHA -->`), pass the same checks (`security-assess.sh`, without the "is it a security update" test), have no fix on it yet, and not have been tried before (`<!-- security-fix-attempt -->`, left before Claude runs, so a failure cannot loop). One attempt per PR.
2. **fix**: `claude-opus-5-5`, `--max-turns 60`, a checkout of the PR head. Tools: `Read`, `Grep`, `Glob`, `Edit`, `Write` and `Bash` limited to `go build`, `go vet`, `go test`, `go list`, `gofmt` and read-only `git` (`diff`, `status`, `log`, `show`, `tag -l`). No web tools; no push credentials (the checkout keeps no token); `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1` asks the CLI to keep credentials out of the commands it runs. The prompt does not include any text from the PR: Claude reproduces the failure itself, and is told the dependency's text is data. The job's only output is a patch artifact.
3. **land** (no Claude; none of the PR's or Claude's code is run): the patch must be non-empty and under 1 MiB; touch only `src/`, `main.go`, `site/docs/`, `README.md`, `llms.txt`; not the manifests (a fix adds no dependency) or `.github/`; no binary, symlink, executable, rename or mode change; at most 600 changed lines in 20 files; and if it changes `src/` it must raise the version in `app.properties` (and change nothing else in that file). It is applied to a clean checkout, committed as `fix(deps): adapt code to the dependency update` by `github-actions[bot]` and pushed with `--force-with-lease` (refused if the branch moved). CI is started on the branch (`build.yml`, `codecov.yml`; a push with the workflow token starts nothing), Lane 1's assessment runs again on the result (mode `fix`, with the version check), CI must be green, then it squash-merges.
4. **report**: if it did not end in a merge (nothing to change, patch refused, CI red, timeout, merge refused), a comment on the PR and **one** issue labelled `security` with the reason (an open issue for the same PR gets a comment instead of a second issue).

### Lane 3: vulnerabilities with no Dependabot PR (`security-scan.yml`)

A best-effort **complement** to Dependabot, daily at 05:17 UTC and by hand. It needs no extra token.

* **scan** (read-only): `npm audit --json --package-lock-only` in `site/` (reads the lock, installs nothing), then `npm audit fix --package-lock-only` limited to what the vulnerable packages need; `govulncheck -scan module -json ./...` (installed with `go install ...@v1.1.4`), then `go get <module>@<fixed>` and `go mod tidy`. The manifest checks of Lane 1 run on the result, and no package may move to another major. Then `npm ci && npm run build` in `site/` (the docs site must still build) and `go build ./... && go test ./...`. **If a build fails, the change is thrown away and an issue says so; no PR.** A Go update that only fails because it needs a code change is kept for Lane 2 when `SECURITY_CLAUDE_FIX` is `true` and nothing else is in the PR.
* **open**: treats the patch as data, checks it again, opens `security/YYYY-MM-DD` as `chore(deps): fix security advisories (N)` (title and branch from fixed words, the date and a count; package names only in the PR text, after a pattern check), labelled `dependencies` and `security`, starts CI on the branch, waits, and squash-merges when the required checks pass. No second PR while a `security/` PR is open.
* **report**: one `security` issue when something needs a person.

**What it skips** (listed in the run summary, never fixed): fixes that are semver-major or change a package's major version; vulnerabilities with no fix; Go standard library and toolchain vulnerabilities (they need a newer Go, which is a person's change to `go.mod`/CI); anything an open Dependabot PR already mentions (Lane 1 handles it); an update that adds a direct dependency, more than 5 new packages, a `replace` or a changed `go` line.

By hand: `gh workflow run security-scan.yml` only scans and fixes on the runner and shows the result in the run summary (a safe first look); `gh workflow run security-scan.yml -f open_pr=true` opens the PR (merged only if `SECURITY_AUTOMERGE` is `true`).

The "Allow GitHub Actions to create and approve pull requests" setting must be on for this lane to open a PR; until it is, the lane reports exactly that in its one issue.

### What stops a malicious dependency bump

A compromised release, or a look-alike package, has to get past all of: it must be a *published advisory fix* (OSV or an open alert), so a random new release is not merged; not a major bump of a direct dependency; no new direct dependency and at most 5 new indirect ones; nothing off the registry; no changed scripts, `replace` or toolchain lines; all commits Dependabot's; green CI on three operating systems at the exact commit, with the head pinned at merge time. What it does **not** stop: a malicious *patch release that is itself the advisory fix*, whose code CI does not detect (CI runs the dependency's code, and `npm ci` runs its install scripts, in jobs without secrets or write tokens, but a bump is still trusted code). That is the residual risk of any auto-merge of dependency updates; the mitigations are the switch, the narrow rules above, and the fact that Dependabot only offers fixes for published advisories. Lane 2 adds a second one: Claude's change is merged on CI alone; keep `SECURITY_CLAUDE_FIX` off if you do not want that.

### Safety rules shared by all lanes

* Workflow-level `permissions: {}`; each job lists the few it needs. `id-token: write` is only on the Claude jobs (`triage`, `fix`).
* The merge is always done by the workflow token; `.github/` is never touched and a PR whose files fall outside the allowlist is never merged.
* The scripts that write are separate from those that read, and each refuses on its own when the kill switch is off.
* Actions pinned like the rest of the repository (tags); `dependabot/fetch-metadata` and `govulncheck` are pinned to an exact commit/version.

### Running and stopping by hand

```sh
gh workflow run security-scan.yml                      # look only
gh workflow run security-scan.yml -f open_pr=true      # open the PR
gh workflow run security-fix.yml -f pr=NN -f sha=<head commit of the PR>   # only acts on a PR with a recorded assessment
gh workflow disable security-update.yml                # off, per workflow (also security-fix.yml, security-scan.yml, triage.yml)
gh variable set SECURITY_AUTOMERGE --body false        # off, for all of the merging at once
```

### Verifying it on `main`

None of this can run before it is on `main`. After it is:

```sh
# the credentials, with the triage on a maintainer issue (look at "Check Claude's credentials" in the run)
gh workflow run triage.yml --ref main -f issue=<n>
gh run list --workflow triage.yml -R 8bitAlex/raid
# a first look at Lane 3
gh workflow run security-scan.yml -R 8bitAlex/raid && gh run watch
# then switch the merging on and see Lane 1 act on the next Dependabot PR (or re-run one)
gh variable set SECURITY_AUTOMERGE --body true -R 8bitAlex/raid
gh api repos/8bitAlex/raid/dependabot/alerts --jq 'length'   # the alerts still open
```

Things to check on the first real runs: (1) that the `pull_request_target` run started by Dependabot can merge (the token is expected to be read/write there); (2) that a PR opened by the workflow token can be merged although the ruleset has a CodeQL rule (the workflow token starts no CodeQL run; if the merge is refused with "waiting for code scanning results", set `SECURITY_BOT_TOKEN`); (3) the names of the CI checks in `security-policy.mjs` (`requiredChecks`) against `gh pr checks`.

## Tests

`bash .github/scripts/test-triage.sh`, `test-claude-auth.sh`, `test-security-policy.sh` and `test-security.sh` run the scripts against a fake `gh` (`test-fake-gh.mjs`, backed by a throwaway git repository), `curl`, `npm`, `go` and `govulncheck`. They check, among other things, that each refusal makes no write, that the exact calls of a merge are made, that a patch touching `.github/` or the manifests is refused, and that hostile text in a verdict cannot mention, link or inject HTML. `automation-tests.yml` runs them (and `bash -n`, `node --check` and a YAML parse of every workflow) when something under `.github/` changes.

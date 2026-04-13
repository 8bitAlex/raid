Go 1.24.4 CLI. Cobra+Viper. yaml.v3 parsing, jsonschema/v6 validation. GoReleaser (.goreleaser.yaml stable, .goreleaser.preview.yaml preview).

Build: `go build -o raid .` Test: `go test ./...` Run: `go run . <cmd>`

Layout: main.go→src/cmd. src/cmd/raid.go=root cmd+subcommand registration+version check. Reserved built-in subcmds: doctor/, env/, install/, profile/ (user cmds w/ same name ignored w/ warning). src/raid/=core domain (profile loading, env resolution, cmd execution). src/internal/=lib/ (shared types), sys/ (OS helpers, GitHub release checks), utils/. schemas/=JSON schemas (raid-repo.schema.json, raid-profile.schema.json, raid-defs.schema.json). src/resources/=embedded assets (app.properties, profile-template, repo-template) via go:embed; resources.go exposes them. site/=Docusaurus source (merged from docsite-source 2026-04-10); builds to gh-pages via .github/workflows/docs.yml on site/** changes.

Config: raid.yaml=per-repo (environments+tasks: Shell|Script|HTTP|Wait|Template|Group|Git|Prompt|Confirm|Set|Print). profile.raid.yml=user profile (tracked repos, global settings).

Non-obvious:
- applyConfigFlag in src/cmd/raid.go scans os.Args for --config/-c BEFORE Cobra parses, because config must load before subcommand registration
- Async version check goroutine on every invocation; info cmds (help/version/completion) wait up to 1.5s, others non-blocking
- Preview channel: EnvironmentPreview→LatestGitHubPreRelease; stable→LatestGitHubRelease; baseVersion() strips -preview suffix
- Info-cmd fast path: QuietGetCommands() does read-only load (no config creation/warnings) so --help works without valid config
- User commands registered at runtime from config via registerUserCommands; not in source
- WriteProfileFile and CreateRepoConfigs prepend embedded templates (src/resources/profile-template, repo-template) to new files; schema URL constants live in the templates, not in Go code

CI: .github/workflows/ — build.yml (build+test), deploy.yml (release), preview.yml (preview releases), codecov.yml (coverage), docs.yml (deploy Pages from site/), docs-build.yml (PR build check for site/)

Every change must:
1. Include full-coverage tests — unit tests for new/changed functions, edge cases, and error paths. Run `go test ./...` and confirm all pass before finishing.
2. Update documentation — if the change affects user-facing behavior, update the relevant docsite pages under site/docs/ (features, usage, references, examples) and the README if applicable. Run `npm run build` in site/ to verify no broken links.
3. Keep the docsite build green — never leave broken cross-references or missing pages.

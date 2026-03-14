[![Build and Test](https://github.com/8bitAlex/raid/actions/workflows/build.yml/badge.svg)](https://github.com/8bitAlex/raid/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/8bitAlex/raid/graph/badge.svg?token=Z75V7I2TLW)](https://codecov.io/github/8bitAlex/raid)
[![Go Report Card](https://goreportcard.com/badge/github.com/8bitAlex/raid)](https://goreportcard.com/report/github.com/8bitAlex/raid)

# Raid — Distributed Development Orchestration
![Windows](https://img.shields.io/badge/Windows-Yes-blue?logo=windows)
![macOS](https://img.shields.io/badge/macOS-Yes-lightgrey?logo=apple)
![Linux](https://img.shields.io/badge/Linux-Yes-yellow?logo=linux)

`Raid` is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.

Tribal knowledge codified into the repo itself — onboarding becomes a single command.

📖 For a deeper look at the goals and design, see the [design proposal blog post](https://alexsalerno.dev/blog/raid-design-proposal?utm_source=chatgpt.com).

## Key Features

- **Portable YAML Configurations** — define environments, tasks, and dependencies in version-controlled YAML files that live alongside your code.
- **Multiple Profiles** — switch between project setups or team configurations with isolated profiles.
- **Rich Task Runner** — 12 built-in task types covering shell commands, scripts, HTTP downloads, service health checks, git operations, template rendering, user prompts, and more.
- **Environment Management** — define and apply consistent development environments for all contributors.
- **Custom Commands** — codify repeated operational tasks (patch, proxy, verify, deploy) as first-class `raid <name>` subcommands that live alongside your configuration.

## Development Status

`Raid` is currently in the **prototype stage**. Core functionality is still being explored and iterated on — expect frequent changes and incomplete features.

Feedback, issues, and contributions are welcome as the project takes shape.

---

## Getting Started

### Installation

```bash
brew install raid  # macOS — Linux and Windows coming soon
```

### Quickstart

```bash
raid profile create   # interactive wizard: name your profile and add repositories
raid install          # clone repos and run install tasks
```

You can also write a profile file manually (see [Configuration](#configuration)) and register it with `raid profile add <file>`.

---

## Commands

### `raid profile`

Manage profiles. A profile is a named collection of repositories and environments.

- `raid profile create` — interactive wizard to create and register a new profile
- `raid profile add <file>` — register profiles from a YAML or JSON file
- `raid profile list` — list all registered profiles
- `raid profile <name>` — switch the active profile
- `raid profile remove <name>` — remove a profile

### `raid install`

Clone all repositories in the active profile and run any configured install tasks. Already-cloned repos are skipped. Use `-t` to limit concurrent clone threads.

### `raid env`

- `raid env <name>` — apply a named environment: writes `.env` files into each repo and runs environment tasks
- `raid env` — show the currently active environment
- `raid env list` — list available environments

### `raid doctor`

Check the current configuration for issues and get suggestions for fixing them. Useful after initial setup or when something isn't working as expected.

### `raid <command>`

Run a custom command defined in the active profile or any of its repositories.

```bash
raid build        # run the "build" command
raid deploy       # run the "deploy" command
```

Custom commands appear alongside built-in commands in `raid --help`. Commands defined in a profile take priority over same-named commands from repositories.

---

## Configuration

### Profile (`*.raid.yaml`)

A profile defines the repositories, environments, and tasks for a project. The `$schema` annotation enables autocomplete and validation in editors like VS Code. See [Tasks](#tasks) for available task types.

Supported formats: `.yaml`, `.yml`, `.json`

Example `my-project.raid.yaml`:
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-profile.schema.json

name: my-project

repositories:
  - name: frontend
    path: ~/Developer/frontend
    url: https://github.com/myorg/frontend
  - name: backend
    path: ~/Developer/backend
    url: https://github.com/myorg/backend

environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
      - name: DATABASE_URL
        value: postgresql://localhost:5432/myproject
    tasks:
      - type: Print
        message: "Applying dev environment..."
        color: green
      - type: Shell
        cmd: docker compose up -d
      - type: Wait
        url: localhost:5432
        timeout: 30s

install:
  tasks:
    - type: Shell
      cmd: brew install node

task_groups:
  verify-services:
    - type: Wait
      url: http://localhost:3000
      timeout: 10s
    - type: Wait
      url: localhost:5432
      timeout: 10s

commands:
  - name: sync
    usage: "Pull latest on all repos and restart services"
    tasks:
      - type: Git
        op: pull
        path: ~/Developer/frontend
      - type: Git
        op: pull
        path: ~/Developer/backend
      - type: Shell
        cmd: docker compose restart
      - type: Group
        ref: verify-services
```

Multiple profiles can be defined in a single file using YAML document separators (`---`) or a JSON array.

### Repository (`raid.yaml`)

Individual repositories can carry their own `raid.yaml` at their root to define repo-specific environments and tasks. These are merged with the profile configuration at load time. Committing this file to each repo is the recommended way to share knowledge with your team.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-repo.schema.json

name: my-service
branch: main

environments:
  - name: dev
    tasks:
      - type: Shell
        cmd: npm install
      - type: Shell
        cmd: npm run build

commands:
  - name: test
    usage: "Run the test suite"
    tasks:
      - type: Shell
        cmd: npm test
```

---

## Tasks

Tasks are the unit of work in raid. They appear in environments, install steps, commands, and task groups. Each task has a `type` and type-specific fields.

| Type | Description |
|------|-------------|
| `Shell` | Run a shell command |
| `Script` | Execute a script file |
| `Git` | Run a git operation (`pull`, `clone`, etc.) |
| `HTTP` | Download a file from a URL |
| `Wait` | Poll a URL or address until it responds |
| `Template` | Render a template file |
| `Print` | Print a message to the console |
| `Prompt` | Prompt the user for input and store it in a variable |
| `Confirm` | Prompt the user for a yes/no confirmation |
| `Group` | Execute a named task group by `ref` |

All task types support two optional modifiers:

```yaml
concurrent: true   # run in parallel with other concurrent tasks
condition:         # skip this task unless all conditions are met
  platform: darwin # only on this OS (darwin, linux, windows)
  exists: ~/.config/myapp  # only if this path exists
  cmd: which docker        # only if this command exits 0
```

### Shell

Run a command string in a configurable shell.

```yaml
- type: Shell
  cmd: echo "hello $USER"
  shell: bash      # optional: bash (default), sh, zsh, powershell, cmd
  literal: false   # optional: skip env var expansion before passing to shell
  path: ~/project  # optional: working directory. Defaults to ~ for profile tasks, repo dir for repo tasks
```

### Script

Execute a script file directly.

```yaml
- type: Script
  path: ./scripts/setup.sh
  runner: bash     # optional: bash, sh, zsh, python, python3, node, powershell
```

### HTTP

Download a file from a URL.

```yaml
- type: HTTP
  url: https://example.com/config.json
  dest: ~/.config/myapp/config.json
```

### Wait

Poll an HTTP(S) URL or TCP address until it responds, then continue.

```yaml
- type: Wait
  url: http://localhost:8080/health  # or TCP: localhost:5432
  timeout: 60s                       # optional, default: 30s
```

### Template

Render a file by substituting `$VAR` and `${VAR}` references with environment variable values.

```yaml
- type: Template
  src: ./config/app.env.template
  dest: ~/.config/myapp/app.env
```

### Group

Execute a named task group defined in the profile's `task_groups`. Supports optional parallel and retry modifiers.

```yaml
- type: Group
  ref: verify-services
  parallel: true   # optional: run all tasks in the group concurrently
  attempts: 3      # optional: retry the group on failure
  delay: 5s        # optional: delay between retries (default: 1s)
```

### Git

Perform a git operation in a repository directory.

```yaml
- type: Git
  op: pull          # pull, checkout, fetch, reset
  branch: main      # required for checkout; optional for pull, fetch, reset
  path: ~/Developer/myrepo  # optional, defaults to current directory
```

### Print

Print a formatted message to stdout. Useful for labelling steps in long task sequences.

```yaml
- type: Print
  message: "Deploying $APP_VERSION to production..."
  color: yellow    # optional: red, green, yellow, blue, cyan, white
  literal: false   # optional: skip env var expansion
```

### Prompt

Ask the user for input and store the result in an environment variable for use by downstream tasks.

```yaml
- type: Prompt
  var: TARGET_ENV
  message: "Which environment? (dev/staging/prod)"
  default: dev     # optional: used when user presses enter with no input
```

### Confirm

Pause and require explicit confirmation (`y` or `yes`) before continuing. Useful before destructive operations.

```yaml
- type: Confirm
  message: "This will reset the production database. Continue?"
```

---

## Commands Configuration

Custom commands are defined in the `commands` array of a profile or repository `raid.yaml`. They become first-class `raid <name>` subcommands at runtime.

```yaml
commands:
  - name: deploy
    usage: "Build and deploy all services"   # shown in raid --help
    tasks:
      - type: Confirm
        message: "Deploy to production?"
      - type: Shell
        cmd: make deploy
    out:                   # optional — defaults to full stdout+stderr when omitted
      stdout: true
      stderr: false
      file: $DEPLOY_LOG    # also write all output here; supports $VAR expansion
```

**`name`** (required) — the subcommand name; e.g. `name: deploy` is invoked as `raid deploy`. Cannot shadow built-in names (`profile`, `install`, `env`).

**`usage`** (optional) — short description shown next to the command in `raid --help`.

**`tasks`** (required) — the task sequence to run. All standard task types are supported.

**`out`** (optional) — controls output handling. When omitted, stdout and stderr behave normally. When present:
- `stdout` — show task stdout (default: `true` when `out` is omitted; set explicitly when using `out`)
- `stderr` — show task stderr (default: `true` when `out` is omitted; set explicitly when using `out`)
- `file` — additionally write all output to this path; supports `$VAR` expansion

**Priority** — when a profile and one of its repositories define a command with the same name, the profile's definition wins.

---

## Best Practices

**Commit `raid.yaml` to each repo.** This is how setup knowledge gets shared — anyone with raid can run `raid install` and get a working environment without reading a wiki.

**Use `commands` to codify team workflows.** Repeated operational tasks — patching, proxying, deploying, verifying — belong in `commands`, not in Slack messages or shared scripts. Anyone on the team can run `raid deploy` without knowing the steps. Use `groups` for reusable internal sequences that commands and other tasks compose from.

**Gate destructive steps with `Confirm`.** Any task sequence that resets data, force-pushes, or modifies production should begin with a `Confirm` task to prevent accidental runs.

**Use `Print` to structure long sequences.** Clear section headers make install and deploy output readable at a glance, especially for new team members.

**Keep profiles in a dotfiles repo.** Profile files reference your repos and environments. Storing them in a private dotfiles repo keeps them version-controlled and accessible across machines.

**Never commit secrets.** Use environment variable references or keep sensitive values in private profiles — never hardcode credentials in a committed raid file.

---

## Contributing

Contributions are welcome. See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for details.

## License

Licensed under the **GNU General Public License v3.0**. See [LICENSE](LICENSE) for the full text.

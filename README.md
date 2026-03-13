[![Build and Test](https://github.com/8bitAlex/raid/actions/workflows/build.yml/badge.svg)](https://github.com/8bitAlex/raid/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/8bitAlex/raid/graph/badge.svg?token=Z75V7I2TLW)](https://codecov.io/github/8bitAlex/raid)
[![Go Report Card](https://goreportcard.com/badge/github.com/8bitAlex/raid)](https://goreportcard.com/report/github.com/8bitAlex/raid◊)

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
- **Automated Task Execution** — orchestrate shell commands and scripts across multiple repositories with a single command.
- **Environment Management** — define and apply consistent development environments for all contributors.

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

First, create a profile file (see [Configuration](#configuration) below), then:

```bash
raid profile add my-project.raid.yaml  # register and activate a profile
raid install                           # clone repos and run install tasks
raid env dev                           # apply the dev environment
```

---

## Commands

### `raid profile`

Manage profiles. A profile is a named collection of repositories and environments.

- `raid profile add <file>` — register profiles from a YAML or JSON file; the first added profile is set as active automatically
- `raid profile list` — list all registered profiles
- `raid profile <name>` — pass a profile name as an argument to switch the active profile
- `raid profile remove <name>` — remove a profile

### `raid install`

Clone all repositories in the active profile and run any configured install tasks. Already-cloned repos are skipped. Use `-t` to limit concurrent clone threads.

### `raid env`

- `raid env <name>` — apply a named environment: writes `.env` files into each repo and runs environment tasks
- `raid env` — show the currently active environment
- `raid env list` — list available environments

---

## Configuration

Tasks can be defined under `install` or within any environment, in both profile and repo configs.

### Profile (`*.raid.yaml`)

A profile defines the repositories and environments for a project. The `$schema` annotation enables autocomplete and validation in editors like VS Code.

```yaml
# yaml-language-server: $schema=schemas/raid-profile.schema.json

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
      - type: Shell
        cmd: echo "dev environment ready"
      - type: Script
        path: ./scripts/setup-dev.sh
        runner: bash

install:
  tasks:
    - type: Shell
      cmd: echo "installing..."
```

Multiple profiles can be defined in a single file using YAML document separators (`---`) or a JSON array.

### Repository (`raid.yaml`)

Individual repositories can carry their own `raid.yaml` at their root to define repo-specific environments and install tasks. These are merged with the profile configuration at load time. Committing this file to each repo is the recommended way to share setup knowledge with your team.

```yaml
# yaml-language-server: $schema=schemas/raid-repo.schema.json

name: my-service
branch: main

environments:
  - name: dev
    tasks:
      - type: Shell
        cmd: npm install
      - type: Shell
        cmd: npm test
```

### Tasks

Two task types are supported:

**Shell** — run a command string in a configurable shell:
```yaml
- type: Shell
  cmd: echo "hello"
  shell: bash        # optional: bash (default), sh, zsh, powershell
  literal: false     # optional: skip env var expansion before passing to shell
  concurrent: true   # optional: run concurrently with other tasks
```

**Script** — execute a script file:
```yaml
- type: Script
  path: ./scripts/setup.sh
  runner: bash       # optional: interpreter to use
  concurrent: false
```

---

## Best Practices

**Commit `raid.yaml` to each repo.** This is how setup knowledge gets shared — anyone with raid can run `raid install` and get a working environment without reading a wiki.

**Keep profiles in a dotfiles repo.** Profile files reference your repos and environments. Storing them in a private dotfiles repo keeps them version-controlled and accessible across machines.

**Never commit secrets.** Use environment variable references or keep sensitive values in private profiles — never hardcode credentials in a committed raid file.

---

## Contributing

Contributions are welcome. See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for details.

## License

Licensed under the **GNU General Public License v3.0**. See [LICENSE](LICENSE) for the full text.

---
sidebar_position: 2
---

# Profile Configuration

A profile is a YAML file that describes your full development environment: which repositories to clone, what to run during install, how to handle environments, and what custom commands are available to the team.

## File format

A profile file can contain one or more profile documents separated by `---`. Each document defines one profile.

```yaml
name: my-team
path: ~/profiles/my-team.yaml

repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
  - name: frontend
    url: git@github.com:my-org/frontend.git
    path: ~/dev/frontend

install:
  tasks:
    - type: print
      message: "All repos cloned. Running global setup..."
    - type: shell
      cmd: brew bundle --file=~/Brewfile

commands:
  - name: test-all
    description: Run tests across all repos
    tasks:
      - type: shell
        cmd: npm test
        dir: ~/dev/api
      - type: shell
        cmd: npm test
        dir: ~/dev/frontend
```

## Top-level fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Unique profile identifier |
| `path` | Yes | Absolute path to this profile file |
| `repositories` | No | List of repositories to manage |
| `install` | No | Tasks to run after all repos are cloned |
| `commands` | No | Custom commands available via `raid <name>` |
| `environments` | No | Named environment configurations |
| `groups` | No | Reusable task sequences |

## Repositories

Each repository entry defines a repo to clone and optionally what to run after cloning.

```yaml
repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
    install:
      tasks:
        - type: shell
          cmd: npm install
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Used to reference the repo (e.g. `raid install api`) |
| `url` | Yes | Git remote URL |
| `path` | Yes | Local clone destination |
| `install.tasks` | No | Tasks to run after this repo is cloned |
| `commands` | No | Commands defined in this repo's `raid.yaml` are merged here automatically |

If a repository already exists at `path`, cloning is skipped.

## Per-repo configuration (`raid.yaml`)

Individual repositories can define their own commands and install tasks by committing a `raid.yaml` file at the root of the repo. These are automatically merged into the active profile when it loads.

```yaml title="~/dev/api/raid.yaml"
commands:
  - name: migrate
    description: Run database migrations
    tasks:
      - type: shell
        cmd: go run ./cmd/migrate
```

## Install tasks

`install.tasks` at the profile level runs after all repositories have been cloned and their own install tasks have completed. Use this for global setup that depends on all repos being present.

```yaml
install:
  tasks:
    - type: shell
      cmd: ./scripts/link-configs.sh
```

## Commands

Custom commands are available as `raid <name>` from anywhere. They run the defined task sequence.

```yaml
commands:
  - name: deploy
    description: Deploy all services
    tasks:
      - type: shell
        cmd: ./deploy.sh
        dir: ~/dev/api
```

See [Task Types](./tasks) for everything a task can do.

## Groups

Groups are reusable task sequences you can reference from multiple commands using a `group` task.

```yaml
groups:
  install-deps:
    - type: shell
      cmd: npm install

commands:
  - name: setup
    tasks:
      - type: group
        ref: install-deps
```

## Registering and switching profiles

```bash
raid profile add ./my-profile.yaml   # register a profile file
raid profile list                     # list all registered profiles
raid my-team                          # switch to the 'my-team' profile
raid profile remove my-team          # remove a profile
```

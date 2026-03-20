---
sidebar_position: 2
---

# Profile Configuration

A profile is a YAML file that describes your full development environment: which repositories to clone, what to run during install, how to handle environments, and what custom commands are available to the team.

## File format

```yaml
name: my-team

repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
  - name: frontend
    url: git@github.com:my-org/frontend.git
    path: ~/dev/frontend

environments:
  - name: local
    variables:
      - name: LOG_LEVEL
        value: debug
    tasks:
      - type: Shell
        cmd: docker-compose up -d
  - name: staging
    variables:
      - name: LOG_LEVEL
        value: info

install:
  tasks:
    - type: Print
      message: "All repos cloned. Running global setup..."
    - type: Shell
      cmd: brew bundle --file=~/Brewfile

commands:
  - name: test-all
    usage: Run tests across all repos
    tasks:
      - type: Shell
        cmd: npm test
        path: ~/dev/api
      - type: Shell
        cmd: npm test
        path: ~/dev/frontend
```

## Top-level fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Unique profile identifier |
| `repositories` | No | List of repositories to manage |
| `environments` | No | Named environment configurations |
| `install` | No | Tasks to run after all repos are cloned |
| `commands` | No | Custom commands available via `raid <name>` |
| `task_groups` | No | Reusable task sequences |

## Repositories

Each repository entry defines a repo to clone and optionally what to run after cloning.

```yaml
repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
    install:
      tasks:
        - type: Shell
          cmd: npm install
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Used to reference the repo (e.g. `raid install api`) |
| `url` | Yes | Git remote URL |
| `path` | Yes | Local clone destination |
| `install.tasks` | No | Tasks to run after this repo is cloned |

If a repository already exists at `path`, cloning is skipped.

## Environments

Environments are defined as a list at the **top level of the profile**. Each environment has a `name`, optional `variables`, and optional `tasks` to run when it is applied.

```yaml
environments:
  - name: local
    variables:
      - name: API_HOST
        value: localhost
    tasks:
      - type: Shell
        cmd: docker-compose up -d db
  - name: production
    variables:
      - name: API_HOST
        value: api.my-org.com
    tasks:
      - type: Confirm
        message: "Switch to production?"
```

Individual repositories can define their own environment configuration in their `raid.yaml`. These are merged with the profile-level environment when applied. See [Environments](./environments) for details.

## Install tasks

`install.tasks` runs after all repositories have been cloned and their own install tasks have completed. Use this for global setup that depends on all repos being present.

```yaml
install:
  tasks:
    - type: Shell
      cmd: ./scripts/link-configs.sh
```

## Commands

Custom commands are available as `raid <name>`. They run the defined task sequence.

```yaml
commands:
  - name: deploy
    usage: Deploy all services
    tasks:
      - type: Shell
        cmd: ./deploy.sh
        path: ~/dev/api
```

See [Task Types](./tasks) for everything a task can do.

## Task groups

Task groups are reusable task sequences referenced from commands using a `Group` task.

```yaml
task_groups:
  install-deps:
    - type: Shell
      cmd: npm install

commands:
  - name: setup
    usage: Install dependencies
    tasks:
      - type: Group
        ref: install-deps
```

## Per-repo configuration (`raid.yaml`)

Repositories can define their own commands and environments by committing a `raid.yaml` at their root. These are automatically merged into the active profile when it loads.

```yaml title="~/dev/api/raid.yaml"
commands:
  - name: migrate
    usage: Run database migrations
    tasks:
      - type: Shell
        cmd: go run ./cmd/migrate

environments:
  - name: local
    variables:
      - name: DATABASE_URL
        value: postgres://localhost:5432/api_dev
  - name: staging
    variables:
      - name: DATABASE_URL
        value: postgres://staging-db.internal:5432/api
```

## Registering and switching profiles

```bash
raid profile add ./my-profile.yaml   # register a profile file
raid profile list                     # list all registered profiles
raid my-team                          # switch to the 'my-team' profile
raid profile remove my-team          # remove a profile
```

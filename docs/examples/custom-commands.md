---
sidebar_position: 3
---

# Custom Commands

Define shared commands like `test`, `patch`, or `proxy` that every developer on the team can run globally without knowing the underlying scripts. This example shows several patterns: simple commands, variable passing between tasks, conditional steps, and parallel execution.

## Simple command

A command that runs a deploy script with a confirmation gate:

```yaml title="profile.raid.yaml"
commands:
  - name: "deploy"
    usage: "Deploy the API to staging"
    tasks:
      - type: Confirm
        message: "Deploy API to staging?"
      - type: Script
        path: "./scripts/deploy.sh staging"
      - type: Print
        message: "Deployed."
        color: green
```

```bash
raid deploy
```

---

## Passing variables between tasks

Use `Set` to define a variable, then reference it in later tasks or future command executions. Use `Shell` exports to capture dynamic values like git SHAs or generated tokens.

```yaml
commands:
  - name: "release"
    usage: "Tag and push a release"
    tasks:
      - type: Prompt
        var: "VERSION"
        message: "Release version (e.g. 1.4.0)"
      - type: Shell
        cmd: |
          git tag v$VERSION
          git push origin v$VERSION
          export VERSION=$VERSION
        path: "~/dev/api"
      - type: Shell
        cmd: |
          export COMMIT=$(git rev-parse --short HEAD)
        path: "~/dev/api"
      - type: Print
        message: "Released v$VERSION at $COMMIT"
```

```bash
raid release
# > Release version (e.g. 1.4.0): 1.4.0
# > Released v1.4.0 at 3fa9c12
```

---

## Conditional tasks

Run tasks only on certain platforms or when a file or command exists:

```yaml
install:
    tasks:
      - type: Shell
        cmd: "brew install postgresql"
        condition:
          platform: darwin
          cmd: "! which psql"
      - type: Shell
        cmd: "sudo apt-get install -y postgresql"
        condition:
          platform: linux
          cmd: "! which psql"
      - type: Shell
        cmd: "createdb api_dev"
        condition:
          cmd: "! psql -lqt | grep api_dev"
      - type: Print
        message: "Dev database ready"
```

---

## Parallel tasks

Mark adjacent tasks `concurrent: true` to run them in parallel. Raid waits for all concurrent tasks to finish before moving to the next step.

```yaml
commands:
  - name: "test-all"
    usage: "Run tests across all services in parallel"
    tasks:
      - type: Print
        message: "Running tests..."
      - type: Shell
        cmd: "go test ./..."
        path: "~/dev/api"
        concurrent: true
      - type: Shell
        cmd: "npm test"
        path: "~/dev/frontend"
        concurrent: true
      - type: Shell
        cmd: "pytest"
        path: "~/dev/data-service"
        concurrent: true
      - type: Print
        message: "All tests passed"
        color: green
```

```bash
raid test-all
# Running tests...
# [api, frontend, data-service running in parallel]
# All tests passed
```

---

## Reusable task groups

Extract repeated sequences into a named group and reference them from multiple commands:

```yaml
task_groups:
  pull-all:
    - type: Shell
      cmd: "git pull"
      path: "~/dev/api"
      concurrent: true
    - type: Shell
      cmd: "git pull"
      path: "~/dev/frontend"
      concurrent: true

commands:
  - name: "sync"
    usage: "Pull all repos and reinstall dependencies"
    tasks:
      - type: Group
        ref: pull-all
      - type: Shell
        cmd: "npm install"
        path: "~/dev/frontend"

  - name: "deploy"
    usage: "Pull latest and deploy"
    tasks:
      - type: Group
        ref: pull-all
      - type: Shell
        cmd: "./scripts/deploy.sh"
```

---

## Per-repo commands

Commands can also live in individual repository `raid.yaml` files, scoped to that service. They are automatically available when the profile is loaded.

```yaml title="~/dev/api/raid.yaml"
commands:
  - name: "migrate"
    usage: "Run pending database migrations"
    tasks:
      - type: Set
        var: ENV
        value: "local"
      - type: Confirm
        message: "Run migrations against $ENV?"
      - type: Shell
        cmd: "go run ./cmd/migrate --env=$ENV"

  - name: "seed"
    usage: "Seed the database with test data"
    tasks:
      - type: Shell
        cmd: "go run ./cmd/seed"
        condition:
          cmd: "psql $DATABASE_URL -c '\\dt' | grep -q users"
```

```bash
raid migrate
# > Run migrations against local? (y/N) y

raid seed
```

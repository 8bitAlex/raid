---
sidebar_position: 2
---

# Environment Switching

Managing multiple environments — local, staging, production — across many repositories is painful without a consistent tool. This example shows how to define environments once and switch them all with a single command.

## Scenario

A platform has two services. Each service needs different variables per environment, and switching to staging or production requires additional steps like confirming intent.

## Profile

Profile-level environments define shared variables and tasks that apply across all repos. Repo-specific variables and tasks are defined in each repo's own `raid.yaml`.

```yaml title="platform.raid.yaml"
name: "platform"

repositories:
  - name: "api"
    url: "git@github.com:my-org/api.git"
    path: "~/dev/api"
  - name: "frontend"
    url: "git@github.com:my-org/frontend.git"
    path: "~/dev/frontend"

environments:
  - name: "local"
    tasks:
      - type: Print
        message: "Switched to local"
  - name: "staging"
    tasks:
      - type: Print
        message: "Switched to staging"
  - name: "production"
    tasks:
      - type: Confirm
        message: "Switch ALL services to production?"
```

## Per-repo environment config

Each repo defines its own environment variables and additional tasks in its `raid.yaml`:

```yaml title="~/dev/api/raid.yaml"
name: "api"
branch: "main"

environments:
  - name: "local"
    variables:
      - name: "DATABASE_URL"
        value: "postgres://localhost:5432/api_dev"
      - name: "API_HOST"
        value: "localhost"
      - name: "API_PORT"
        value: "3000"
      - name: "LOG_LEVEL"
        value: "debug"
    tasks:
      - type: Shell
        cmd: "docker-compose up -d db"
  - name: "staging"
    variables:
      - name: "DATABASE_URL"
        value: "postgres://staging-db.internal:5432/api"
      - name: "API_HOST"
        value: "api.staging.my-org.com"
      - name: "API_PORT"
        value: "443"
      - name: "LOG_LEVEL"
        value: "info"
  - name: "production"
    variables:
      - name: "DATABASE_URL"
        value: "postgres://prod-db.internal:5432/api"
      - name: "API_HOST"
        value: "api.my-org.com"
      - name: "API_PORT"
        value: "443"
      - name: "LOG_LEVEL"
        value: "warn"
    tasks:
      - type: Confirm
        message: "Point API at production database?"
      - type: Shell
        cmd: "./scripts/rotate-api-key.sh"
```

```yaml title="~/dev/frontend/raid.yaml"
name: "frontend"
branch: "main"

environments:
  - name: "local"
    variables:
      - name: "VITE_API_URL"
        value: "http://localhost:3000"
      - name: "VITE_ENV"
        value: "local"
  - name: "staging"
    variables:
      - name: "VITE_API_URL"
        value: "https://api.staging.my-org.com"
      - name: "VITE_ENV"
        value: "staging"
  - name: "production"
    variables:
      - name: "VITE_API_URL"
        value: "https://api.my-org.com"
      - name: "VITE_ENV"
        value: "production"
    tasks:
      - type: Confirm
        message: "Point frontend at production APIs?"
```

## Usage

```bash
# Switch everything to staging
raid env staging

# Check which environment is active
raid env

# List available environments
raid env list
```

For production, the `Confirm` tasks in the profile and each repo's `raid.yaml` will gate the switch before any variables or tasks are applied.

## Dynamic local environments

If local values differ between developers (ports, paths, credentials), use `Prompt` and `Template` tasks instead of hardcoded `variables` to generate values at apply-time:

```yaml title="~/dev/api/raid.yaml"
name: "api"
branch: "main"

environments:
  - name: "local"
    tasks:
      - type: Prompt
        var: "DB_PORT"
        message: "Local database port"
        default: "5432"
      - type: Prompt
        var: "API_PORT"
        message: "Local API port"
        default: "3000"
      - type: Template
        src: "./envs/local.env.tmpl"
        dest: "~/dev/api/.env"
      - type: Shell
        cmd: "docker-compose up -d db"
```

```bash title="~/dev/api/envs/local.env.tmpl"
DATABASE_URL=postgres://localhost:$DB_PORT/api_dev
API_PORT=$API_PORT
LOG_LEVEL=debug
```

When a developer runs `raid env local`, they are prompted for their preferred ports and the `.env` is generated from the template. The committed template is shared; the generated `.env` stays out of source control.

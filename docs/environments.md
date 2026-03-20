---
sidebar_position: 4
---

# Environments

Environments let you define named configurations — sets of variables and tasks — that can be applied across all repositories at once. Switching from local to staging to production is a single command.

## Define environments

Environments are defined at the **top level of the profile**, as a list. Each environment has a `name`, optional `variables`, and optional `tasks`.

```yaml title="profile.yaml"
name: platform

environments:
  - name: local
    variables:
      - name: LOG_LEVEL
        value: debug
    tasks:
      - type: Shell
        cmd: docker-compose up -d db
  - name: staging
    variables:
      - name: LOG_LEVEL
        value: info
    tasks:
      - type: Print
        message: "Switched to staging"
  - name: production
    variables:
      - name: LOG_LEVEL
        value: warn
    tasks:
      - type: Confirm
        message: "Switch to production?"

repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
```

Individual repositories can also define their own environments in their `raid.yaml`. These are merged with the profile-level environments when applied.

```yaml title="~/dev/api/raid.yaml"
environments:
  - name: local
    variables:
      - name: DATABASE_URL
        value: postgres://localhost:5432/api_dev
      - name: API_PORT
        value: "3000"
    tasks:
      - type: Shell
        cmd: docker-compose up -d db
  - name: staging
    variables:
      - name: DATABASE_URL
        value: postgres://staging-db.internal:5432/api
      - name: API_PORT
        value: "443"
  - name: production
    variables:
      - name: DATABASE_URL
        value: postgres://prod-db.internal:5432/api
      - name: API_PORT
        value: "443"
    tasks:
      - type: Confirm
        message: "Point API at production database?"
      - type: Shell
        cmd: ./scripts/rotate-api-key.sh
```

| Field | Description |
|---|---|
| `name` | Environment name used with `raid env <name>` |
| `variables` | List of `{name, value}` pairs to set when the environment is activated |
| `tasks` | Tasks to run when this environment is applied |

## Apply an environment

```bash
raid env staging
```

## Check the active environment

```bash
raid env
```

## List available environments

```bash
raid env list
```

## Tips

**Use `Confirm` tasks for production.** A confirmation gate before switching to a production environment prevents accidents.

**Use `Prompt` and `Template` for developer-specific values.** If values differ per developer (e.g. local ports), prompt for them at apply-time.

```yaml title="~/dev/api/raid.yaml"
environments:
  - name: local
    tasks:
      - type: Prompt
        var: DB_PORT
        message: "Local database port"
        default: "5432"
      - type: Prompt
        var: API_PORT
        message: "Local API port"
        default: "3000"
      - type: Template
        src: ./envs/local.env.tmpl
        dest: ~/dev/api/.env
```

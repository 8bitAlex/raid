---
sidebar_position: 4
---

# Environments

Environments let you define named configurations — sets of `.env` files and tasks — that can be applied across all repositories at once. Switching from local to staging to production is a single command.

## Define environments

Environments are defined in the profile under each repository.

```yaml
repositories:
  - name: api
    url: git@github.com:my-org/api.git
    path: ~/dev/api
    environments:
      local:
        envFile: ./envs/local.env
        tasks:
          - type: shell
            cmd: docker-compose up -d
      staging:
        envFile: ./envs/staging.env
        tasks:
          - type: shell
            cmd: echo "Pointed at staging"
      production:
        envFile: ./envs/production.env
        tasks:
          - type: confirm
            message: "You are switching to production. Continue?"
```

| Field | Description |
|---|---|
| `envFile` | Path to a `.env` file to write into the repository root |
| `tasks` | Tasks to run when this environment is applied |

## Apply an environment

```bash
raid env staging
```

This iterates over every repository in the active profile and for each one:
1. Writes the `envFile` contents to `.env` in the repository root
2. Runs the environment's `tasks`

## Check the active environment

```bash
raid env
```

## List available environments

```bash
raid env list
```

## Tips

**Keep `envFile` paths relative to the profile file.** This makes the profile portable across machines.

**Use `confirm` tasks for production.** A confirmation gate before switching to a production environment prevents accidents.

```yaml
production:
  envFile: ./envs/production.env
  tasks:
    - type: confirm
      message: "Switch ALL repos to production?"
    - type: shell
      cmd: ./scripts/rotate-creds.sh
```

**Use `set` and `template` tasks for dynamic values.** If environment values differ per developer (e.g. local ports), generate them from a template rather than committing the final `.env`.

```yaml
local:
  tasks:
    - type: prompt
      message: "Local API port"
      var: API_PORT
      default: "3000"
    - type: template
      src: ./envs/local.env.tmpl
      dest: ~/dev/api/.env
```

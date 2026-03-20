---
sidebar_position: 1
---

# New Developer Onboarding

The most common raid use case: a developer joins the team and needs to get their machine set up from scratch.

## Scenario

The team has three repositories — an API, a frontend, and a shared config library. Each needs dependencies installed and the dev environment configured. Without raid, this takes an afternoon and a long Confluence page. With raid, it's one command.

## Profile

The team commits this file to an internal `dev-environment` repo:

```yaml title="team-profile.yaml"
name: acme-platform

repositories:
  - name: api
    url: git@github.com:acme/api.git
    path: ~/dev/acme/api
    install:
      tasks:
        - type: Shell
          cmd: go mod download

  - name: frontend
    url: git@github.com:acme/frontend.git
    path: ~/dev/acme/frontend
    install:
      tasks:
        - type: Shell
          cmd: npm install

  - name: shared-config
    url: git@github.com:acme/shared-config.git
    path: ~/dev/acme/shared-config

environments:
  - name: local
    tasks:
      - type: Print
        message: "Local environment applied"

install:
  tasks:
    - type: Shell
      cmd: brew bundle
      condition:
        platform: darwin
    - type: Print
      message: "Setup complete. Run 'raid env local' to configure your environment."
```

## Per-repo configs

Each repo ships its own `raid.yaml` defining the commands and environment variables specific to that service:

```yaml title="~/dev/acme/api/raid.yaml"
commands:
  - name: api-dev
    usage: Start the API in development mode
    tasks:
      - type: Shell
        cmd: go run ./cmd/server --env=local

  - name: api-test
    usage: Run API tests
    tasks:
      - type: Shell
        cmd: go test ./...

environments:
  - name: local
    variables:
      - name: DATABASE_URL
        value: postgres://localhost:5432/api_dev
      - name: API_PORT
        value: "3000"
      - name: LOG_LEVEL
        value: debug
    tasks:
      - type: Shell
        cmd: docker-compose up -d db
```

```yaml title="~/dev/acme/frontend/raid.yaml"
commands:
  - name: fe-dev
    usage: Start the frontend dev server
    tasks:
      - type: Shell
        cmd: npm run dev

environments:
  - name: local
    variables:
      - name: VITE_API_URL
        value: http://localhost:3000
      - name: VITE_ENV
        value: local
```

## New developer workflow

```bash
# 1. Register the team profile
raid profile add ~/profiles/acme-platform.yaml

# 2. Clone everything and run install tasks
raid install

# 3. Apply the local environment
raid env local

# 4. Start working
raid api-dev
raid fe-dev
```

### What `raid install` does

```
Cloning repositories...
✓ api             → ~/dev/acme/api
✓ frontend        → ~/dev/acme/frontend
✓ shared-config   → ~/dev/acme/shared-config

Running install tasks...
✓ go mod download       (api)
✓ npm install           (frontend)
✓ brew bundle           (profile)

Setup complete. Run 'raid env local' to configure your environment.
```

All three repos clone concurrently. If one already exists, it is skipped.

## Adding new team members later

No changes needed. The same four commands work on every machine. When a new service is added to the profile, new developers get it automatically, and existing developers can run `raid install new-service` to clone just the new repo without re-running everything.

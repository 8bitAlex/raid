---
sidebar_position: 1
---

# New Developer Onboarding

Get a new developer from a blank machine to a fully running environment with a single command.

## Scenario

The team has three repositories — an API, a frontend, and a shared config library. Each needs dependencies installed and the dev environment configured. Without raid, this takes an afternoon and a long Confluence page. With raid, it's one command.

## Profile

The team commits this file to an internal `dev-environment` repo:

```yaml title="acme.raid.yaml"
# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-profile.schema.json
name: acme-platform

repositories:
  - name: api
    url: git@github.com:acme/api.git
    path: ~/dev/acme/api

  - name: frontend
    url: git@github.com:acme/frontend.git
    path: ~/dev/acme/frontend

  - name: shared-config
    url: git@github.com:acme/shared-config.git
    path: ~/dev/acme/shared-config

environments:
  - name: local
    variables:
      - name: API_URL
        value: http://localhost:3000
      - name: FRONTEND_PORT
        value: "8080"
    tasks:
      - type: Print
        message: "Local environment applied"

install:
  tasks:
    - type: Shell
      cmd: brew bundle
      condition:
        platform: darwin
    - type: Shell
      cmd: raid env local
    - type: Print
      message: "Setup complete."

```

## Per-repo configs

Each repo ships its own `raid.yaml` defining the commands and environment variables specific to that service:

```yaml title="~/dev/acme/api/raid.yaml"
# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-repo.schema.json
name: api
branch: main

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
# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-repo.schema.json
name: frontend
branch: main

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
raid profile add ~/profiles/acme.raid.yaml

# 2. Clone everything and run install tasks
raid install

# 3. Apply the local environment
raid env local

# 4. Start working
raid api-dev
raid fe-dev
```

All three repos clone concurrently. If one already exists it is skipped. Profile-level install tasks run next, followed by repo-level install tasks.

## Adding new team members later

No changes needed. The same four commands work on every machine. When a new service is added to the profile, new developers get it automatically, and existing developers can run `raid install new-service` to clone just the new repo without re-running everything.

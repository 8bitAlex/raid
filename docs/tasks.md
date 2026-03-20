---
sidebar_position: 3
---

# Task Types

Tasks are the unit of work in raid. They appear in install steps, commands, environments, and groups. Every task has a `type` field and type-specific fields.

## Common fields

These fields apply to all task types.

| Field | Description |
|---|---|
| `type` | Task type (required) |
| `concurrent` | Run this task in parallel with adjacent concurrent tasks |
| `condition` | Only run this task if the condition passes |

### Conditions

```yaml
- type: shell
  cmd: brew install nvm
  condition:
    platform: darwin          # darwin, linux, or windows
    exists: /usr/local/bin/nvm   # skip if path already exists
    cmd: which nvm              # skip if command succeeds
```

All specified condition fields must pass for the task to run.

---

## shell

Run a shell command.

```yaml
- type: shell
  cmd: npm install
```

```yaml
- type: shell
  cmd: |
    echo "Building..."
    npm run build
  dir: ~/dev/frontend     # working directory (defaults to home dir or repo root)
  shell: /bin/zsh         # override the shell (default: /bin/sh)
  literal: true           # skip variable expansion
```

**Variable expansion** — `$VAR` and `${VAR}` references in `cmd` are expanded using:
1. Variables set by `set` tasks (highest priority)
2. Variables exported by earlier `shell` tasks in the same command session
3. OS environment variables

Shell-local variables and parameter expansions like `${FOO:-default}` are passed through to the shell intact.

**Exit codes** — if the command exits non-zero, task execution stops and raid exits with that same code.

**Exporting variables** — use `export` in your script to make a variable available to later tasks in the same command run:

```yaml
- type: shell
  cmd: |
    export API_URL=$(cat .env | grep API_URL | cut -d= -f2)
- type: shell
  cmd: echo "API is at $API_URL"
```

---

## set

Set a variable that persists for the duration of the command session and can be used in subsequent tasks.

```yaml
- type: set
  var: ENVIRONMENT
  value: production
```

```yaml
- type: set
  var: REGION
  default: us-east-1   # used if the variable is not already set
```

Variables set this way take precedence over exported shell variables and OS environment variables.

---

## print

Print a message to the terminal.

```yaml
- type: print
  message: "Installing dependencies..."
```

```yaml
- type: print
  message: "Done!"
  color: green    # green, red, yellow, blue, cyan, magenta, white
```

Use `print` to structure long task sequences with clear section headers.

---

## template

Render a template file and write it to a destination.

```yaml
- type: template
  src: ./configs/app.config.tmpl
  dest: ~/dev/api/.env
```

Template files support `$VAR` and `${VAR}` substitution using the same variable lookup order as `shell` tasks. Variables that are not set expand to an empty string.

---

## script

Run a script file.

```yaml
- type: script
  path: ./scripts/setup.sh
```

```yaml
- type: script
  path: ./setup.py
  runner: python3
```

If `runner` is omitted, the script is executed directly (requires a shebang line or executable permission).

---

## git

Perform a git operation on a repository.

```yaml
- type: git
  op: pull
  dir: ~/dev/api
```

```yaml
- type: git
  op: checkout
  branch: main
  dir: ~/dev/api
```

| `op` | Description |
|---|---|
| `pull` | Pull latest changes |
| `checkout` | Checkout a branch |

---

## group

Execute a named group of tasks defined in the profile's `groups` section.

```yaml
- type: group
  ref: install-deps
```

```yaml
- type: group
  ref: build-all
  parallel: true    # run the group's tasks in parallel
```

---

## prompt

Prompt the user for input and store the result in a variable.

```yaml
- type: prompt
  message: "Enter your API key"
  var: API_KEY
```

```yaml
- type: prompt
  message: "Enter your username"
  var: USERNAME
  default: admin
```

---

## confirm

Ask the user to confirm before continuing.

```yaml
- type: confirm
  message: "This will reset your database. Continue?"
```

If the user answers no, the remaining tasks are skipped.

---

## http

Download a file from a URL.

```yaml
- type: http
  url: https://example.com/config.json
  dest: ~/dev/api/config.json
```

---

## wait

Pause execution for a duration.

```yaml
- type: wait
  timeout: 5s
```

---

## Concurrent tasks

Mark adjacent tasks with `concurrent: true` to run them in parallel. Raid collects consecutive concurrent tasks into a batch and waits for all of them before moving on.

```yaml
tasks:
  - type: shell
    cmd: npm install
    dir: ~/dev/api
    concurrent: true
  - type: shell
    cmd: npm install
    dir: ~/dev/frontend
    concurrent: true
  - type: print
    message: "Dependencies installed"   # runs after both npm installs finish
```

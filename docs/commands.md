---
sidebar_position: 5
---

# Command Reference

## raid install

Clone all repositories in the active profile and run install tasks.

```bash
raid install [repo] [-t <threads>]
```

**Behaviour:**
1. Clone all repositories concurrently (throttled by `-t` if set)
2. Run profile-level install tasks
3. Run each repository's install tasks in profile order

If a repository already exists at its configured path, cloning is skipped.

| Flag | Description |
|---|---|
| `[repo]` | Install only the named repository (profile-level tasks are not run) |
| `-t, --threads` | Max concurrent clone threads (default: unlimited) |

---

## raid env

Manage and apply environments.

```bash
raid env              # show the active environment
raid env <name>       # apply a named environment to all repos
raid env list         # list available environments
```

Applying an environment writes each repository's configured `.env` file and runs its environment tasks.

---

## raid profile

Manage profiles.

```bash
raid profile create           # interactive wizard to create a new profile
raid profile add <file>       # register a YAML or JSON profile file
raid profile list             # list all registered profiles
raid profile <name>           # switch the active profile
raid profile remove <name>    # remove a profile
```

---

## raid doctor

Check the active configuration for issues.

```bash
raid doctor
```

Doctor inspects the current profile and reports findings at three severity levels:

| Level | Meaning |
|---|---|
| `OK` | No issues |
| `WARN` | Something looks off but raid can still function |
| `ERROR` | A problem that will prevent raid from working correctly |

Run `raid doctor` after initial setup or whenever something isn't working as expected.

---

## raid \<command\>

Run a custom command defined in the active profile or any of its repositories.

```bash
raid deploy
raid migrate
raid test-all
```

Custom commands are defined in `commands` sections of the profile or in individual repository `raid.yaml` files. Run `raid --help` to see all available commands.

Custom command names cannot shadow built-in names (`profile`, `install`, `env`, `doctor`).

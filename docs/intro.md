---
sidebar_position: 1
slug: /intro
---

# Getting Started

Raid is a CLI tool that lets you define your entire development environment — repositories, install steps, environment configs, and team commands — in a single YAML profile. Check it in, share it with the team, and anyone can go from a blank machine to a fully running environment with one command.

## Install

**Homebrew**
```bash
brew install 8bitalex/tap/raid
```

**Script**
```bash
curl -fsSL https://raw.githubusercontent.com/8bitalex/raid/main/install.sh | bash
```

## Create a profile

Run the interactive wizard to create your first profile:

```bash
raid profile create
```

The wizard asks for a profile name and walks you through adding repositories. Each repository needs a name, a Git URL, and a local path.

You can also write the profile file manually and register it:

```bash
raid profile add ./my-profile.yaml
```

See [Profile Configuration](./profile) for the full file format.

## Install your environment

Once a profile is active, clone all repositories and run their install tasks:

```bash
raid install
```

Raid clones all repositories concurrently, then runs profile-level install tasks followed by each repository's install tasks in profile order.

To install a single repository:

```bash
raid install <repo-name>
```

## Run a command

If your profile or any of its repositories define custom commands, they are available immediately:

```bash
raid <command>
```

Run `raid --help` to see all available commands, including any defined in the active profile.

## Next steps

- [Profile Configuration](./profile) — repositories, environments, and commands
- [Task Types](./tasks) — everything a task can do
- [Environments](./environments) — switch between dev, staging, and production
- [Command Reference](./commands) — built-in commands

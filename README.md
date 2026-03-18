[![Build and Test](https://github.com/8bitAlex/raid/actions/workflows/build.yml/badge.svg)](https://github.com/8bitAlex/raid/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/8bitAlex/raid/graph/badge.svg?token=Z75V7I2TLW)](https://codecov.io/github/8bitAlex/raid)
[![Go Report Card](https://goreportcard.com/badge/github.com/8bitAlex/raid)](https://goreportcard.com/report/github.com/8bitAlex/raid)
[![Release](https://img.shields.io/github/v/release/8bitAlex/raid)](https://github.com/8bitAlex/raid/releases)
![Windows](https://img.shields.io/badge/Windows-supported-blue?logo=windows)
![macOS](https://img.shields.io/badge/macOS-supported-lightgrey?logo=apple)
![Linux](https://img.shields.io/badge/Linux-supported-yellow?logo=linux)

# raid

**Your entire multi-repo development environment, defined in a single YAML file.**

Cloning a new repo shouldn't take days. Onboarding a new teammate shouldn't require a Notion page, three Slack threads, and a prayer. `raid` lets you codify your environments, tasks, and dependencies directly into your repositories — so `raid install` just works.

📖 [Documentation](https://8bitalex.github.io/raid) · [Design Proposal](https://alexsalerno.dev/blog/raid-design-proposal)

---

## The problem

Most dev tools manage tasks inside a single repository. But real development happens across *multiple* repos — microservices, shared libraries, platform tooling — each with its own setup steps, environment variables, and tribal knowledge that lives in someone's head.

`raid` treats your entire project ecosystem as a single, manageable unit.

---

## How it works

Define a profile that describes your repos, environments, and tasks:

```yaml
# my-project.raid.yaml
name: my-project

repositories:
  - name: api
    path: ~/Developer/api
    url: https://github.com/myorg/api
  - name: frontend
    path: ~/Developer/frontend
    url: https://github.com/myorg/frontend

environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
    tasks:
      - type: Shell
        cmd: make deps
      - type: Wait
        url: http://localhost:8080
```

Then:

```bash
raid profile add my-project.raid.yaml   # load your profile
raid install                            # clone all repos, run install tasks
raid env dev                            # spin up your dev environment
```

New teammate? Same three commands. Every time.

---

## Why not just use Taskfile or Just?

Those are great tools for automating tasks *within* a repo. `raid` solves a different problem: orchestrating *across* repos.

|  | raid | Taskfile | Just |
|---|:---:|:---:|:---:|
| Run tasks in a repo | ✅ | ✅ | ✅ |
| **Multi-repo orchestration** | ✅ | ❌ | ❌ |
| **Clone & set up repos** | ✅ | ❌ | ❌ |
| **Profile switching** | ✅ | ❌ | ❌ |
| **Environment management** | ✅ | Partial | ❌ |
| Custom subcommands | ✅ | ✅ | ✅ |
| Cross-platform | ✅ | ✅ | ✅ |

---

## Key features

- **Multi-repo profiles** — define a group of repos, their paths, URLs, and setup steps in one file. Switch between projects with `raid profile use <name>`.
- **12 task types** — Shell, Script, HTTP, Git, Template, Wait, Prompt, Confirm, Print, Group, Parallel, Retry. Mix and match with conditional guards and concurrent execution.
- **Custom subcommands** — define `commands:` in your profile and they register as first-class `raid <name>` subcommands at startup.
- **Environment management** — inject variables and run setup tasks scoped to an environment. `raid env dev` or `raid env prod`.
- **Version-controlled** — profiles and repo configs live alongside your code. Tribal knowledge becomes a YAML file.

---

## Installation

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/8bitalex/raid/main/install.sh | bash
```

### Windows

Download the latest `.zip` from [Releases](https://github.com/8bitAlex/raid/releases/latest), extract, and add `raid.exe` to your PATH.

### Build from source

```bash
git clone https://github.com/8bitAlex/raid
cd raid/src
go build -o raid .
```

---

## Quick start

```bash
# 1. Create a profile
cat > my-project.raid.yaml << 'EOF'
name: my-project
repositories:
  - name: my-repo
    path: ~/Developer/my-repo
    url: https://github.com/yourorg/my-repo
EOF

# 2. Add and activate it
raid profile add my-project.raid.yaml

# 3. Install your repos
raid install
```

---

## Status

`raid` is in **active beta**. Core functionality is stable and being hardened. Expect API changes before v1.0. Feedback, issues, and contributions are very welcome.

---

## Contributing

See [CONTRIBUTING.md](docs/CONTRIBUTING.md).

## License

GPL-3.0 — see [LICENSE](LICENSE).

# Raid - Distributed Development Orchestration

`Raid` is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.

If you have ever pulled a repo (or repos) that require days of configuration just to get a passing build,
or have onboarded to a new team that has no documentation, or have a folder of scripts to automate your tasks but haven't
shared them yet, then you are probably a software engineer in need of this. 

`Raid` handles the pain of error-prone knowledge-dependent tasks and management of your development environment. You no longer need
to worry about wasted time onboarding new contributors. Tribal knowledge can be codified into the repo itself. And you will
never miss running that one test ever again.

📖 For a deeper look at the goals and design of raid, see the [design proposal blog post](https://alexsalerno.dev/blog/raid-design-proposal?utm_source=chatgpt.com).

[Getting Started](#getting-started) • [Best Practices](#⚠-best-practices) • [Documentation](#usage--documentation)

## Key Features

- **Portable YAML Configurations**: Define your development environment using simple, version-controlled YAML files
- **Multiple Raid Profiles**: Manage different project configurations and environments with separate profiles
- **Distributed Repository Management**: Automatically clone, update, and manage multiple repositories across your development environment
- **Development Environment Automation**: Streamline setup, dependency installation, and environment configuration
- **Test Runner**: Robust testing framework with automatic error recovery and retry mechanisms
- **Configurable Global Commands**: Extend functionality and automate common tasks with user-defined commands that work across all managed repositories

## Project Status

`Raid` is currently in the **prototype stage**. Core functionality is still being explored and iterated on, so expect frequent changes and incomplete features.

Feedback, issues, and contributions are welcome as the project takes shape.

| Platform | Supported |
|----------|:---------:|
| Linux    | ✅        |
| Mac      | ✅        |
| Windows  | ✅        |

## Getting Started

### Installation

```bash
# Installation instructions will be added here
```

### Configuration

1. Create a profile configuration file (e.g., `my-project.raid.yaml`)
2. Define your repositories and dependencies
3. Configure environment settings

### Execution

```bash
raid install    # Clone repos and setup environment
raid test       # Run tests across all repositories
```

## ⚠ Best Practices 

- **Keep raid profiles private:** Avoid committing raid profiles to public repositories to protect sensitive configuration details.
- **Store profiles securely:** Place raid profiles in a secure, private location separate from your public codebase.
- **Store secrets securely:** Place sensitive information such as secrets or credentials only in private raid profiles, never in public repositories.

## Usage & Documentation

[Commands](#commands) • [Profile Configuration](#profile-configuration)• [Repository Configuration](#repository-configuration)

## Commands

[profile](#raid-profile-options) • [install](#raid-install) • [test](#raid-test) • [env](#raid-env)

### `raid profile`

Manage raid profiles. If there are no non-option arguments, available profiles are listed.

#### Subcommands

##### `raid profile add <filepath>`

Add profile(s) from a YAML (.yaml, .yml) or JSON (.json) file. The file will be validated against the raid profile schema.

**Features:**
- **Multiple Profiles Support**: Add multiple profiles from a single file using YAML document separators (`---`) or JSON arrays
- **Auto-Activation**: If no active profile exists, the first profile is automatically set as active
- **Duplicate Handling**: Existing profiles are detected and reported, only new profiles are added
- **Validation**: Each profile is validated against the JSON schema

**Examples:**
```bash
# Add a single profile
raid profile add my-project.raid.yaml

# Add multiple profiles from YAML with document separators
raid profile add multiple-profiles.yaml

# Add multiple profiles from JSON array
raid profile add multiple-profiles.json
```

**Example Output:**
```bash
# Single profile (auto-activated)
Profile 'my-project' has been successfully added from my-project.raid.yaml and set as active

# Multiple profiles (first auto-activated)
Profiles development, personal, open-source have been successfully added from multiple-profiles.yaml. Profile 'development' has been set as active

# Some profiles already exist
Profiles already exist: development
Profiles personal, open-source have been successfully added from multiple-profiles.yaml
```

##### `raid profile list`

List all available profiles and show the currently active profile.

##### `raid profile use <profile-name>`

Set a specific profile as the active profile.

#### Legacy Options

`<profile name>`

The name of the profile to set as current (legacy syntax, use `raid profile use <profile-name>` instead).

`-l <path>, --load=<path>`

The filepath to one or more profile configuration files. Loads all profiles found. If a profile name is provided, new profiles are loaded first then it will try and set that profile as current (legacy syntax, use `raid profile add <filepath>` instead).

### `raid install`

Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance.

**Prerequisites:**
- An active profile must be set using `raid profile use <profile-name>`
- The active profile must contain valid repository definitions

**Features:**
- **Concurrent Cloning**: All repositories are cloned simultaneously for faster installation
- **Concurrency Control**: Limit the number of concurrent clones with `--concurrency` flag
- **Path Expansion**: Supports `~` for home directory and environment variables
- **Smart Cloning**: Skips repositories that already exist
- **Error Handling**: Provides clear error messages for missing profiles or invalid configurations
- **Progress Feedback**: Shows cloning progress for each repository

**Options:**
- `--concurrency, -c`: Maximum number of concurrent repository clones (default: 0 = unlimited)

**Examples:**
```bash
# Set an active profile first
raid profile use my-project

# Install all repositories with unlimited concurrency (default)
raid install

# Install with limited concurrency (max 3 concurrent clones)
raid install --concurrency 3

# Install with limited concurrency using short flag
raid install -c 5
```

**Example Output:**
```bash
Installing profile 'my-project' with 3 repositories...
Starting to clone repository 'frontend'...
Starting to clone repository 'backend'...
Starting to clone repository 'shared-libs'...
Successfully cloned repository 'frontend'
Successfully cloned repository 'backend'
Successfully cloned repository 'shared-libs'
Successfully installed all repositories for profile 'my-project'
```

**Concurrency Guidelines:**
- **Unlimited (default)**: Best for fast networks and when you want maximum speed
- **Limited (3-5)**: Good for slower networks or when you want to avoid overwhelming the system
- **Very Limited (1-2)**: Useful for very slow connections or when you need to minimize resource usage

### `raid test`

Runs tests across all managed repositories with automatic error recovery and retry mechanisms.

### `raid env`



## Profile Configuration

A profile configuration file follows the naming pattern `*.raid.yaml` and defines the properties of a raid profile—a group of repositories and their dependencies.

### Single Profile Configuration

```yaml
name: my-project

repositories:
  - name: frontend
    url: https://github.com/myorg/frontend
    branch: main
    
  - name: backend
    url: https://github.com/myorg/backend
    branch: master

dependencies:
  - name: database
    type: docker
    image: postgres:latest
    
environment:
  variables:
    NODE_ENV: development
    DATABASE_URL: postgresql://localhost:5432/myproject
```

### Multiple Profiles in a Single File

You can define multiple profiles in a single file using YAML document separators (`---`) or JSON arrays.

#### YAML with Document Separators

```yaml
name: development
repositories:
  - name: frontend
    url: https://github.com/company/frontend
    branch: main
  - name: backend
    url: https://github.com/company/backend
    branch: master
---
name: personal
repositories:
  - name: blog
    url: https://github.com/username/blog
    branch: main
  - name: dotfiles
    url: https://github.com/username/dotfiles
    branch: main
---
name: open-source
repositories:
  - name: raid
    url: https://github.com/8bitAlex/raid
    branch: main
```

#### JSON with Arrays

```json
[
  {
    "name": "development",
    "repositories": [
      {
        "name": "frontend",
        "url": "https://github.com/company/frontend",
        "branch": "main"
      },
      {
        "name": "backend",
        "url": "https://github.com/company/backend",
        "branch": "master"
      }
    ]
  },
  {
    "name": "personal",
    "repositories": [
      {
        "name": "blog",
        "url": "https://github.com/username/blog",
        "branch": "main"
      }
    ]
  }
]
```

### Profile Management Features

- **Auto-Activation**: The first profile added is automatically set as active if no active profile exists
- **Duplicate Detection**: Existing profiles are detected and reported when adding new profiles
- **Schema Validation**: Each profile is validated against the JSON schema
- **Backward Compatibility**: Single profile files continue to work as before

## Repository Configuration

A repository configuration file named `raid.yaml` defines the properties of an individual repository. This file should be located in the root directory of a git repository.

### Example Repository Configuration

```yaml
name: my-service

build:
  commands:
    - npm install
    - npm run build

test:
  commands:
    - npm test
  retries: 3
  timeout: 300

dependencies:
  - name: redis
    type: docker
    image: redis:alpine
```

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](docs/CONTRIBUTING.md) for details.

## License

[License information to be added]


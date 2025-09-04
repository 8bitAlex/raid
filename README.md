# Raid - Distributed Development Orchestration
[![codecov](https://codecov.io/github/8bitAlex/raid/graph/badge.svg?token=Z75V7I2TLW)](https://codecov.io/github/8bitAlex/raid)

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

- **Portable YAML Configurations**: Define your development environments, tasks, and dependencies using simple, version-controlled YAML files.
- **Multiple Profiles**: Easily switch between different project setups or team configurations with isolated profiles.
- **Automated Task Execution**: Orchestrate shell commands, scripts, and custom tasks across multiple repositories with a single command.
- **Environment Management**: Define, share, and execute complex development environments to ensure consistency for all contributors.

| Platform | Supported |
|----------|:---------:|
| Linux    | ✅        |
| Mac      | ✅        |
| Windows  | ✅        |

## Development

`Raid` is currently in the **prototype stage**. Core functionality is still being explored and iterated on, so expect frequent changes and incomplete features.

Feedback, issues, and contributions are welcome as the project takes shape.

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
raid profile add my-project.raid.yaml  # Add and activate a profile
raid install                           # Clone repos and setup environment
raid env dev                           # Execute development environment (if configured)
```

## ⚠ Best Practices 

- **Store profiles securely:** If your raid profile contains sensitive configuration or secrets, keep it in a secure, private location outside of your public codebase.
- **Never commit secrets:** Always keep secrets and credentials in private raid profiles. Do not store them in public repositories.

## Usage & Documentation

**Note:** Raid is currently in the prototype stage. Some features may be incomplete or in development.

[Commands](#commands) • [Profile Configuration](#profile-configuration) • [Repository Configuration](#repository-configuration) • [JSON Schema Specifications](#json-schema-specifications)

## Commands

[profile](#raid-profile) • [install](#raid-install) • [env](#raid-env)

### `raid profile`

Manage raid profiles. If there are no non-option arguments, the currently active profile is displayed.

#### Subcommands

##### `raid profile add <filepath>`

Add profile(s) from a YAML (.yaml, .yml) or JSON (.json) file. The file will be validated against the raid profile schema.

**Features:**
- **Multiple Profiles Support**: Add multiple profiles from a single file using YAML document separators (`---`) or JSON arrays
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
Profile 'my-project' has been successfully added from my-project.raid.yaml

# Multiple profiles (first auto-activated)
Profiles:
	development
	personal
	open-source
have been successfully added from multiple-profiles.yaml
Profile 'development' set as active

# Some profiles already exist
Profiles already exist with names:
	development

Profiles:
	personal
	open-source
have been successfully added from multiple-profiles.yaml
```

##### `raid profile list`

List all available profiles and show the currently active profile.

**Example Output:**
```bash
Available profiles:
	my-project (active)	~/.raid/profiles/my-project.raid.yaml
	development		~/.raid/profiles/development.raid.yaml
	personal		~/.raid/profiles/personal.raid.yaml
```

##### `raid profile use <profile-name>`

Set a specific profile as the active profile.

**Example:**
```bash
raid profile use my-project
# Output: Profile 'my-project' is now active.
```

##### `raid profile remove <profile-name> [profile-name...]`

Remove one or more profiles. You can specify multiple profile names to remove them all at once.

**Examples:**
```bash
# Remove a single profile
raid profile remove old-project

# Remove multiple profiles
raid profile remove project1 project2 project3
```

**Example Output:**
```bash
Profile 'old-project' has been removed.
Profile 'project1' has been removed.
Profile 'project2' has been removed.
```

### `raid install`

Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance.

**Prerequisites:**
- An active profile must be set using `raid profile use <profile-name>`
- The active profile must contain valid repository definitions

**Features:**
- **Concurrent**: All repositories are cloned simultaneously for faster installation

**Options:**
- `--threads, -t`: Maximum number of concurrent repository clones (default: 0 = unlimited)

**Examples:**
```bash
# Set an active profile first
raid profile use my-project

# Install all repositories with unlimited concurrency (default)
raid install

# Install with limited concurrency (max 3 concurrent clones)
raid install --threads 3

# Install with limited concurrency using short flag
raid install -t 5
```

**Concurrency Guidelines:**
- **Unlimited (default)**: Best for fast networks and when you want maximum speed
- **Limited (3-5)**: Good for slower networks or when you want to avoid overwhelming the system
- **Very Limited (1-2)**: Useful for very slow connections or when you need to minimize resource usage

## Profile Configuration

A profile configuration file follows the naming pattern `*.raid.yaml` and defines the properties of a raid profile—a group of repositories and their environments.

### Single Profile Configuration

```yaml
# yaml-language-server: $schema=schemas/raid-profile.schema.json

name: my-project

repositories:
  - name: frontend
    path: ~/Developer/frontend
    url: https://github.com/myorg/frontend
    
  - name: backend
    path: ~/Developer/backend
    url: https://github.com/myorg/backend

environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
      - name: DATABASE_URL
        value: postgresql://localhost:5432/myproject
    tasks:
      - type: Shell
        cmd: echo "Setting up development environment..."
      - type: Script
        path: ./scripts/setup-dev.sh
```

### Multiple Profiles in a Single File

You can define multiple profiles in a single file using YAML document separators (`---`) or JSON arrays. Each profile in the file is individually validated against the schema.

#### YAML with Document Separators

```yaml
# yaml-language-server: $schema=schemas/raid-profile.schema.json

name: development
repositories:
  - name: frontend
    path: ~/Developer/company/frontend
    url: https://github.com/company/frontend
  - name: backend
    path: ~/Developer/company/backend
    url: https://github.com/company/backend
---
name: personal
repositories:
  - name: blog
    path: ~/Developer/blog
    url: https://github.com/username/blog
  - name: dotfiles
    path: ~/Developer/dotfiles
    url: https://github.com/username/dotfiles
---
name: open-source
repositories:
  - name: raid
    path: ~/Developer/raid
    url: https://github.com/8bitAlex/raid
```

#### JSON with Arrays

```json
[
  {
    "$schema": "schemas/raid-profile.schema.json",
    "name": "development",
    "repositories": [
      {
        "name": "frontend",
        "path": "~/Developer/company/frontend",
        "url": "https://github.com/company/frontend"
      },
      {
        "name": "backend",
        "path": "~/Developer/company/backend",
        "url": "https://github.com/company/backend"
      }
    ]
  },
  {
    "name": "personal",
    "repositories": [
      {
        "name": "blog",
        "path": "~/Developer/blog",
        "url": "https://github.com/username/blog"
      }
    ]
  }
]
```



### Profile Management Features

- **Schema Validation**: Each profile is validated against the JSON schema
- **Multiple Format Support**: YAML and JSON files are both supported
- **IDE Integration**: Use `$schema` references for autocomplete and validation

**Note:** For detailed schema information, see the [JSON Schema Specifications](#json-schema-specifications) section.

## Repository Configuration

A repository configuration file named `raid.yaml` defines the properties of an individual repository. This file should be located in the root directory of a git repository.

**Note:** Repository configurations follow the `raid-repo.schema.json` schema. See the [JSON Schema Specifications](#json-schema-specifications) section for detailed schema information.

### Example Repository Configuration

```yaml
# yaml-language-server: $schema=schemas/raid-repo.schema.json

name: my-service
branch: main

environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
    tasks:
      - type: Shell
        cmd: npm install
      - type: Shell
        cmd: npm run build
      - type: Shell
        cmd: npm test
```

## JSON Schema Specifications

Raid uses **JSON Schema Draft 2020-12** for configuration validation. The schema system consists of three main files:

- **`raid-profile.schema.json`** - Main profile configuration schema
- **`raid-defs.schema.json`** - Shared definitions for environments and tasks
- **`raid-repo.schema.json`** - Individual repository configuration schema

### Schema Validation

All profile and repo configurations are validated against the **JSON Schema Draft 2020-12** specification. This ensures your configuration files have the correct structure and required fields.

### IDE Integration

For the best development experience, include schema references in your configuration files:

```yaml
# yaml-language-server: $schema=schemas/raid-profile.schema.json
```

This provides:
- ✅ **Autocomplete** for field names and values
- ✅ **Real-time validation** of your configuration
- ✅ **Error highlighting** for invalid configurations
- ✅ **Documentation tooltips** for each field

### Schema Structure Details

#### Profile Schema (`raid-profile.schema.json`)
A raid profile configuration must contain:

- **`name`** (string, required) - The name of the raid profile
- **`repositories`** (array, required) - Array of repository configurations
  - Each repository must have:
    - `name` (string, required) - The name of the repository
    - `path` (string, required) - The local path to the repository
    - `url` (string, required) - The URL of the repository
- **`environments`** (array, optional) - Array of environment configurations

#### Repository Schema (`raid-repo.schema.json`)
A repository configuration must contain:

- **`name`** (string, required) - The name of the repository
- **`branch`** (string, required) - The branch to checkout
- **`environments`** (array, optional) - Array of environment configurations (follows `raid-defs.schema.json`)

#### Definitions Schema (`raid-defs.schema.json`)
Environments and tasks follow this shared schema:

**Environment Schema:**
- **`name`** (string, required) - The name of the environment
- **`variables`** (array, optional) - Environment variables to set
  - Each variable must have:
    - `name` (string, required) - The name of the variable
    - `value` (string, required) - The value of the variable
- **`tasks`** (array, optional) - Tasks to be executed

**Task Schema:**
Tasks support two types:

**Shell Tasks:**
```yaml
- type: Shell
  cmd: echo "Hello World"
  concurrent: true  # Optional: execute concurrently with other tasks
```

**Script Tasks:**
```yaml
- type: Script
  path: ./scripts/setup.sh
  concurrent: false  # Optional: execute sequentially
```

### Technical Details

- **Schema Compatibility**: Fully compatible with JSON Schema Draft 2020-12
- **Validation Engine**: Uses `github.com/santhosh-tekuri/jsonschema/v6` library for validation
- **File Format Support**: Both YAML and JSON files are supported
- **Multiple Profiles**: Each profile in a multi-profile file is individually validated

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](docs/CONTRIBUTING.md) for details.

## License

This project is licensed under the **GNU General Public License v3.0** (GPL-3.0).

### Key License Highlights

**What you can do:**
- ✅ **Use** the software for any purpose
- ✅ **Study** how the software works
- ✅ **Modify** the software to suit your needs
- ✅ **Distribute** copies of the software
- ✅ **Distribute** modified versions

**What you must do:**
- 📋 **License your modifications** under the same GPL-3.0 license
- 📋 **Include source code** when distributing the software
- 📋 **State changes** you made to the software
- 📋 **Include the license** and copyright notices

**What you cannot do:**
- ❌ **Make the software proprietary** - modifications must remain open source
- ❌ **Remove the license** or copyright notices
- ❌ **Sublicense** under different terms

### Full License Text

The complete license text is available in the [LICENSE](LICENSE) file. For more information about the GNU GPL, visit [https://www.gnu.org/licenses/](https://www.gnu.org/licenses/).

### Contributing

By contributing to this project, you agree that your contributions will be licensed under the same GPL-3.0 license

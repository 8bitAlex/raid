# Raid - Distributed Development Orchestration

*Raid* is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.

If you have ever pulled a repo (or repos) that require days of configuration just to get a passing build,
or have onboarded to a new team that has no documentation, or have a folder of scripts to automate your tasks but haven't
shared them yet, then you are probably a software engineer in need of this. 

*Raid* handles the pain of error-prone knowledge-dependent tasks and management of your development environment. You no longer need
to worry about wasted time onboarding new contributors. Tribal knowledge can be codified into the repo itself. And you will
never miss running that one test ever again.

[Getting Started](#getting-started) • [Resources](#resources) • [Documentation](#usage--documentation)

## Key Features

- **Portable YAML Configurations**: Define your development environment using simple, version-controlled YAML files
- **Multiple Raid Profiles**: Manage different project configurations and environments with separate profiles
- **Distributed Repository Management**: Automatically clone, update, and manage multiple repositories across your development environment
- **Development Environment Automation**: Streamline setup, dependency installation, and environment configuration
- **Self-Healing Test Runner**: Robust testing framework with automatic error recovery and retry mechanisms
- **Custom Global Commands**: Extend functionality and automate common tasks with user-defined commands that work across all managed repositories

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

## Usage & Documentation

- [Commands](#commands)
- [Profile Configuration](#profile-configuration)
- [Repository Configuration](#repository-configuration)

## Commands

### `raid profile [options]`

List, load, or switch profiles. If there are no non-option arguments, available profiles are listed.

#### Options

`<profile name>`

The name of the profile to set as current.

`-l <path>, --load=<path>`

The filepath to one or more profile configuration files. Loads all profiles found. If a profile name is provided, new profiles are loaded first then it will try and set that profile as current.

### `raid install`

Clones all repositories, builds dependencies, and configures the development environment.

### `raid test`

Runs tests across all managed repositories with automatic error recovery and retry mechanisms.

### `raid update`

Updates all managed repositories to their latest versions.

## Profile Configuration

A profile configuration file follows the naming pattern `*.raid.yaml` and defines the properties of a raid profile—a group of repositories and their dependencies.

### Example Profile Configuration

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
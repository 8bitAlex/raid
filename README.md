# raid - distributed development orchestration

*Raid* is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.

Jump to [[Getting Started](#getting-started) • [Documentation](#usage--documentation)]

## Key Features

- Portable YAML Configurations
- Multiple *raid* Profiles
- Automate & Manage Distributed Repositories
- Automate & Manage Development Environment
- Robust Self-Healing Test Runner
- Custom Global Commands

# Getting Started

## Install

## Configure

## Execute

# Usage & Documentation

- [Commands](#commands)
- [Profile Configuration File](#profile-configuration)
- [Repo Configuration File](#repo-configuration)

## Commands

`Install`

Clones all repos, builds any dependencies, and configures the environment.

## Profile Configuration File

A file with the name pattern `*.raid.yaml` that defines the properties of a raid profile (group of repositories and their dependencies).

## Repo Configuration File

A file with the name `raid.yaml` that defines the properties of an individual repository. Located in the root folder of a git repository.
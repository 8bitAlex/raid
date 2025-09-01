# Raid - Distributed Development Orchestration

`Raid` is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.

If you have ever pulled a repo (or repos) that require days of configuration just to get a passing build,
or have onboarded to a new team that has no documentation, or have a folder of scripts to automate your tasks but haven't
shared them yet, then you are probably a software engineer in need of this. 

`Raid` handles the pain of error-prone knowledge-dependent tasks and management of your development environment. You no longer need
to worry about wasted time onboarding new contributors. Tribal knowledge can be codified into the repo itself. And you will
never miss running that one test ever again.

📖 For a deeper look at the goals and design of raid, see the [design proposal blog post](https://alexsalerno.dev/blog/raid-design-proposal?utm_source=chatgpt.com).

## Key Features

- **Portable YAML Configurations**: Define your development environment using simple, version-controlled YAML files
- **Multiple Raid Profiles**: Manage different project configurations and environments with separate profiles
- **Distributed Repository Management**: Automatically clone, update, and manage multiple repositories across your development environment
- **Development Environment Automation**: Streamline setup, dependency installation, and environment configuration
- **Self-Healing Test Runner**: Robust testing framework with automatic error recovery and retry mechanisms
- **Custom Global Commands**: Extend functionality and automate common tasks with user-defined commands that work across all managed repositories

## Project Status

`Raid` is currently in the **prototype stage**. Core functionality is still being explored and iterated on, so expect frequent changes and incomplete features.

If you’d like to follow the most up-to-date work, check out the ['alpha'](https://github.com/8bitAlex/raid/tree/alpha) branch. This is where active development of the prototype is happening.

Feedback, issues, and contributions are welcome as the project takes shape.


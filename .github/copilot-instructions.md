# Raid - Distributed Development Orchestration

## Project Overview

Raid is a Go-based CLI tool that orchestrates development tasks, environments, and dependencies across distributed repositories. It uses YAML/JSON profile configurations to define multi-repo environments and automates setup/execution workflows.

## Architecture

### Core Components
- **CLI Layer**: `src/cmd/` - Cobra-based command structure with subcommands for profiles, installation, and environments
- **Business Logic**: `src/raid/` - High-level API layer that delegates to internal libraries
- **Internal Implementation**: `src/internal/lib/` - Core functionality for profiles, repositories, environments, and task execution
- **System Utilities**: `src/internal/sys/` and `src/utils/` - System-level operations and shared utilities

### Key Design Patterns

#### Configuration Management
- Uses **Viper** for configuration with global state in `src/internal/lib/config.go`
- **Context singleton** pattern: `lib.Context` struct caches active profile and environment
- **Lazy loading**: `Load()` uses cached context, `ForceLoad()` rebuilds from scratch
- Configuration path customizable via `--config/-c` flag

#### Profile System
- Profiles define collections of repositories and environments
- **Multi-document YAML** support using `---` separators for multiple profiles per file
- **JSON Schema validation** against `schemas/raid-profile.schema.json`
- Profile state managed in Viper config under `"profiles"` key

#### Repository Management
- Concurrent cloning with optional thread limits (`--threads/-t` flag)
- Uses Go routines with semaphore pattern for concurrency control
- Repository validation and error aggregation across parallel operations

#### Environment Execution
- Environments contain tasks (Shell commands or Script files) and environment variables
- Task execution supports concurrent execution flag per task
- Environment variables set globally during environment execution

### Key Files & Patterns

#### Entry Points
- `main.go` - Simple delegator to `cmd.Execute()`
- `src/cmd/raid.go` - Root Cobra command with initialization lifecycle

#### Command Structure
```
src/cmd/
├── raid.go          # Root command, global flags, initialization
├── profile/         # Profile management (add, list, use, remove)
├── install/         # Repository installation 
└── env/             # Environment execution
```

#### Core Business Logic Flow
1. **Initialize**: `raid.Initialize()` → `lib.InitConfig()` → `lib.Load()`
2. **Profile Management**: Viper-backed persistence with JSON schema validation
3. **Repository Installation**: Concurrent git cloning with error aggregation  
4. **Environment Execution**: Task orchestration with variable setting

### Development Workflows

#### Building & Testing
```bash
go build -o raid                 # Build binary
go test ./...                    # Run tests
go test -coverprofile=coverage.out ./...  # Generate coverage
```

#### JSON Schema Integration
- Schemas in `schemas/` directory define validation rules
- Use `github.com/santhosh-tekuri/jsonschema/v6` for validation
- YAML language server integration with `# yaml-language-server: $schema=...` comments

#### Configuration Files
- **Profile configs**: YAML/JSON files following `schemas/raid-profile.schema.json`
- **Multi-profile files**: Use YAML `---` document separators
- **Examples**: See `docs/examples/` for reference configurations

### Common Patterns

#### Error Handling
- Use `fmt.Errorf()` for wrapped errors with context
- Aggregate errors from concurrent operations into slices
- CLI commands print errors to stderr via `cmd.PrintErrln()`

#### Concurrent Operations
```go
// Semaphore pattern for limiting concurrency
semaphore := make(chan struct{}, maxThreads)
var wg sync.WaitGroup
errorChan := make(chan error, len(items))

// In goroutine:
semaphore <- struct{}{}
defer func() { <-semaphore }()
```

#### Viper Configuration
- Global config management via `viper.GetString()`, `viper.Set()`
- Nested keys accessed with dot notation: `viper.GetStringMapString("profiles")`
- Automatic config file discovery and loading

### Testing & Quality
- Uses standard Go testing with coverage reporting  
- GitHub Actions CI/CD pipeline defined (`.github/workflows/build.yml`)
- Codecov integration for coverage tracking
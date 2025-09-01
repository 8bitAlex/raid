# Raid Environment Command

The `raid env` command allows you to execute environments defined in your raid profiles and repository configurations. Environments can contain environment variables and tasks that are executed to set up your development environment.

## Usage

```bash
raid env [environment-name] [flags]
```

### Arguments

- `environment-name` (required): The name of the environment to execute

### Flags

- `-t, --threads int`: Maximum number of concurrent task executions (0 = unlimited, default: 0)

## How It Works

1. **Profile First**: The command first looks for the environment in the active profile's configuration
2. **Repository Search**: Then it searches for the environment in each repository's configuration file (`raid.yaml`, `raid.yml`, or `raid.json`)
3. **Environment Variables**: Sets any defined environment variables globally
4. **Task Execution**: Executes tasks concurrently using the task runner

## Environment Configuration

Environments are defined in your raid profile or repository configuration files:

```yaml
environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
      - name: DATABASE_URL
        value: postgresql://localhost:5432/dev_db
    tasks:
      - type: Shell
        cmd: echo "Setting up development environment..."
      - type: Script
        path: ./scripts/setup-dev.sh
```

### Environment Structure

- **name**: The name of the environment (required)
- **variables**: Array of environment variables to set (optional)
- **tasks**: Array of tasks to execute (optional)

### Variable Structure

- **name**: The name of the environment variable
- **value**: The value to set

### Task Types

#### Shell Tasks
Execute shell commands:

```yaml
- type: Shell
  cmd: echo "Hello, World!"
```

#### Script Tasks
Execute script files:

```yaml
- type: Script
  path: ./scripts/setup.sh
```

## Examples

### Basic Usage

```bash
# Execute the 'dev' environment
raid env dev

# Execute with limited concurrency
raid env dev --threads 3
```

### Example Profile

```yaml
name: my-project

repositories:
  - name: frontend
    path: ~/Developer/my-project-frontend
    url: https://github.com/myorg/frontend.git
  - name: backend
    path: ~/Developer/my-project-backend
    url: https://github.com/myorg/backend.git

environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
      - name: API_URL
        value: http://localhost:3000
    tasks:
      - type: Shell
        cmd: echo "Setting up development environment..."
      - type: Shell
        cmd: npm install
      - type: Script
        path: ./scripts/start-dev.sh

  - name: prod
    variables:
      - name: NODE_ENV
        value: production
      - name: API_URL
        value: https://api.myproject.com
    tasks:
      - type: Shell
        cmd: echo "Setting up production environment..."
      - type: Shell
        cmd: npm run build
      - type: Script
        path: ./scripts/deploy.sh
```

## Execution Order

1. Environment variables are set globally first
2. Tasks are executed concurrently in the order they are defined
3. If an environment is found in the profile, it's executed first
4. Then each repository is checked for the environment and executed in order

## Error Handling

- If no environment is found, the command will exit with an error
- If any task fails, the command will exit with an error
- Environment variables are set before task execution, so tasks can use them

## Tips

- Use descriptive environment names like `dev`, `staging`, `prod`, `test`
- Keep tasks focused and specific to the environment
- Use environment variables to configure different environments
- Script tasks should be executable and handle their own error cases
- Use the `--threads` flag to control concurrency for better performance or resource management

# Contributing to Raid

Thank you for your interest in contributing to Raid! This document provides guidelines and information for contributors.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Review Process](#code-review-process)
- [Reporting Issues](#reporting-issues)
- [Feature Requests](#feature-requests)
- [Community Guidelines](#community-guidelines)

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- Basic understanding of Go development

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/raid.git
   cd raid
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/8bitAlex/raid.git
   ```

## Development Setup

### Install Dependencies

```bash
go mod download
```

### Build the Project

```bash
go build -o raid ./main.go
```

### Run Tests

```bash
go test ./...
```

### Run with Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Code Style

### Go Code Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format your code
- Run `go vet` before submitting
- Use meaningful variable and function names
- Add comments for exported functions and types

### Project Structure

```
src/
├── cmd/           # Command-line interface commands
├── internal/      # Internal packages (not importable)
│   ├── lib/       # Core library functionality
│   ├── sys/       # System-specific code
│   └── utils/     # Utility functions
└── raid/          # Public packages

schemas/           # JSON Schema definitions (root level)
├── raid-profile.schema.json
├── raid-defs.schema.json
├── raid-repo.schema.json
└── README.md
```

### Commit Messages

Use conventional commit format:

```
type(scope): description
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Maintenance tasks

Examples:
```
feat(profile): add support for multiple profile formats

fix(install): resolve concurrent repository cloning issue

docs(readme): update JSON schema specifications section
```

## Testing

### Writing Tests

- Write tests for new functionality
- Aim for good test coverage
- Use descriptive test names
- Test both success and error cases

### Test Structure

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    
    // Act
    result, err := FunctionName(input)
    
    // Assert
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }
    if result != "expected output" {
        t.Errorf("Expected 'expected output', got '%s'", result)
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests in a specific package
go test ./src/internal/lib

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...
```

## Submitting Changes

### Workflow

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and commit them with clear messages

3. **Push to your fork:**
   ```bash
   git push origin feature/your-feature-name
   ```

4. **Create a Pull Request** on GitHub

### Pull Request Guidelines

- **Title**: Clear, descriptive title
- **Description**: Explain what the PR does and why
- **Related Issues**: Link to any related issues
- **Testing**: Describe how you tested the changes
- **Breaking Changes**: Note any breaking changes

### PR Template

```markdown
## Description
Brief description of what this PR accomplishes.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Code refactoring
- [ ] Test addition/update

## Testing
- [ ] Tests pass locally
- [ ] Added new tests for new functionality
- [ ] Updated existing tests if needed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated if needed
- [ ] No breaking changes introduced

## Related Issues
Closes #(issue number)
```

## Code Review Process

### Review Guidelines

- Be respectful and constructive
- Focus on the code, not the person
- Ask questions if something is unclear
- Suggest improvements when possible
- Approve only when you're satisfied

### Review Checklist

- [ ] Code follows project conventions
- [ ] Tests are included and pass
- [ ] Documentation is updated if needed
- [ ] No obvious bugs or issues
- [ ] Performance considerations addressed
- [ ] Security implications considered

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

- **Description**: Clear description of the problem
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Expected Behavior**: What you expected to happen
- **Actual Behavior**: What actually happened
- **Environment**: OS, Go version, Raid version
- **Additional Context**: Any other relevant information

### Issue Template

```markdown
## Bug Description
[Clear description of the bug]

## Steps to Reproduce
1. [Step 1]
2. [Step 2]
3. [Step 3]

## Expected Behavior
[What you expected to happen]

## Actual Behavior
[What actually happened]

## Environment
- OS: [e.g., macOS 14.0, Ubuntu 22.04]
- Go Version: [e.g., go version go1.21.0 darwin/amd64]
- Raid Version: [e.g., 1.0.0-Alpha]

## Additional Context
[Any other context about the problem]
```

## Feature Requests

### Feature Request Guidelines

- **Clear Description**: Explain what you want to achieve
- **Use Case**: Describe the problem this feature would solve
- **Proposed Solution**: Suggest how it might be implemented
- **Alternatives**: Consider if there are existing ways to achieve this

### Feature Request Template

```markdown
## Feature Description
[Clear description of the feature you're requesting]

## Problem Statement
[Describe the problem this feature would solve]

## Proposed Solution
[Describe your proposed solution]

## Alternatives Considered
[Describe any alternatives you've considered]

## Additional Context
[Any other context about the feature request]
```

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Help others learn and grow

### Communication

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Pull Requests**: For code contributions

### Getting Help

- Check existing documentation first
- Search existing issues and discussions
- Ask questions in GitHub Discussions
- Be patient and respectful

## License

By contributing to Raid, you agree that your contributions will be licensed under the same [GNU General Public License v3.0](LICENSE) that covers the project.

## Recognition

Contributors will be recognized in:
- Project README
- Release notes
- Contributor statistics

---

Thank you for contributing to Raid! Your contributions help make this project better for everyone.

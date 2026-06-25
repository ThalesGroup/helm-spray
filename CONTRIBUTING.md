# Contributing to Helm Spray

Thank you for your interest in contributing to Helm Spray! This document provides guidelines and instructions for contributing.

## Filing Issues

File issues using the standard GitHub issue tracker for the repo. When filing an issue, please include:

- A clear description of the problem or feature request
- Steps to reproduce (for bugs)
- Expected behavior vs actual behavior
- Helm version and Kubernetes version
- Any relevant logs or error messages

## Development Setup

### Prerequisites

- Go 1.24 or later
- Helm 3.x or 4.x
- kubectl configured for your cluster
- Make (optional, for build targets)

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/helm-spray.git
   cd helm-spray
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/ThalesGroup/helm-spray.git
   ```
4. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

### Building

```bash
# Build for current platform
go build -o helm-spray .

# Build for all platforms
make dist

# Build for specific platform
make dist_linux
make dist_darwin
make dist_windows
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Code Quality

```bash
# Run go vet
go vet ./...

# Format code (if gofmt is configured)
gofmt -s -w .

# Run linter (if golangci-lint is installed)
golangci-lint run
```

## How to Become a Contributor and Submit Your Own Code

### Contributing A Patch

1. Submit an issue describing your proposed change to the repo in question.
2. The repo owner will respond to your issue promptly.
3. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
4. Fork the desired repo, develop and test your code changes.
5. Ensure your code follows the project's coding standards.
6. Write or update tests for your changes.
7. Update documentation if needed.
8. Submit a pull request.

### Pull Request Guidelines

- Keep pull requests focused on a single change
- Include a clear description of what the change does and why
- Reference any related issues
- Ensure all tests pass
- Add tests for new functionality
- Update documentation if needed
- Follow the existing code style

### Code Style

- Follow standard Go conventions
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and small
- Handle errors appropriately

### Commit Messages

- Use clear, descriptive commit messages
- Start with a verb in imperative mood (e.g., "Add", "Fix", "Update")
- Keep the subject line under 72 characters
- Reference issue numbers when applicable

## Community

- Join the discussion in GitHub Issues
- Be respectful and constructive
- Help others when possible

# Contributing to renovate-scheduler

Thank you for your interest in contributing to renovate-scheduler! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.26 or later
- Docker (for building container images)
- Git

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/mieliespoor/renovate-scheduler.git
   cd renovate-scheduler
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests locally:
   ```bash
   go test -race -cover ./...
   ```

4. Run linters:
   ```bash
   golangci-lint run ./...
   ```

5. Build the binary:
   ```bash
   go build -o renovate-scheduler .
   ```

6. Build Docker image locally:
   ```bash
   docker build -t renovate-scheduler:dev .
   ```

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) to enable automated changelog generation and versioning.

Format: `<type>(<scope>): <subject>`

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `perf`: A performance improvement
- `docs`: Documentation changes
- `refactor`: Code refactoring without feature changes
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, or tooling changes
- `ci`: CI/CD pipeline changes

**Examples:**
```
feat(scheduler): add support for custom polling intervals
fix(ingest): correct digest calculation for file changes
docs: update README with configuration examples
test(dispatch): add integration tests for concurrent job handling
```

## Pull Request Process

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit using conventional commit messages

3. Push your branch and create a Pull Request

4. Ensure all CI checks pass:
   - ✅ Lint (golangci-lint)
   - ✅ Tests (go test -race)
   - ✅ Build (docker build)
   - ✅ Security scan (Trivy)

5. Request review from maintainers

6. Once approved, your PR will be merged

## Release Process

The release process is automated using [release-please](https://github.com/googleapis/release-please-action):

1. **Release PR Creation**: When commits are merged to `main`, release-please automatically creates a PR that:
   - Bumps the version number (semantic versioning)
   - Updates `CHANGELOG.md` with unreleased changes
   - Groups changes by type (features, fixes, etc.)

2. **Release Merge**: When the release PR is merged:
   - A Git tag is created with the version number
   - A GitHub Release is created with the changelog as release notes
   - Release artifacts are built and attached

3. **Artifact Building**: The release build workflow:
   - Builds multi-platform binaries (Linux/macOS/Windows × amd64/arm64)
   - Builds and pushes Docker images to GHCR

## CI/CD Workflows

### Continuous Integration (`ci.yml`)
Runs on every PR and push to `main`:
- **Lint**: Code style and quality checks (golangci-lint)
- **Test**: Unit tests with race condition detection and coverage reporting
- **Build**: Binary compilation
- **Docker**: Docker image build
- **Security**: Vulnerability scanning (Trivy)

### Release Workflow (`release.yml`)
Runs on every push to `main`:
- Creates release PRs with version bumps and changelog updates

### Build Release Artifacts (`release-build.yml`)
Triggered on Git tags:
- Builds multi-platform binaries
- Builds and pushes Docker images (amd64 and arm64)
- Creates GitHub Release with artifacts

### Dependency Updates (`dependabot.yml`)
- Go dependencies: Updated weekly
- GitHub Actions: Updated weekly
- Docker base image: Updated weekly

## Testing Guidelines

- Write tests for new features and bug fixes
- Maintain test coverage above 80%
- Use `t.Run()` for test subtests
- Use `t.TempDir()` for temporary test files
- Run `go test -race` to detect race conditions

Example:
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {name: "basic", input: "test", want: "expected"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Feature(tt.input)
            if got != tt.want {
                t.Fatalf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

## Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and modular

## Documentation

- Update `README.md` for user-facing changes
- Update `CHANGELOG.md` for notable changes (release-please handles this)
- Add inline code comments for complex logic
- Include examples in documentation

## Getting Help

- Open an issue for bug reports or feature requests
- Discuss major changes in issues before implementing
- Ask questions in pull request comments

## Code of Conduct

Be respectful and professional in all interactions. We're committed to providing a welcoming environment for all contributors.

---

Thank you for contributing! 🎉

# CI/CD Pipeline Guide

This document describes the automated CI/CD pipeline for renovate-scheduler.

## Overview

The project uses GitHub Actions to automate:
- **Code Quality**: Linting and testing on every PR
- **Security**: Vulnerability scanning
- **Releases**: Automated versioning, changelog generation, and artifact creation
- **Dependencies**: Automated dependency updates

## Workflows

### 1. Continuous Integration (`.github/workflows/ci.yml`)

**Trigger**: Every PR to `main` and every push to `main`

**Jobs**:

#### Lint
- Runs `golangci-lint` for code quality checks
- Verifies `go.mod`/`go.sum` consistency
- Enforces code style standards

**Configuration**: 5-minute timeout

#### Test
- Runs `go test -race` to detect race conditions
- Collects code coverage metrics
- Uploads coverage to Codecov
- Fails if critical issues detected

**Coverage Target**: Tracks and reports coverage changes

#### Build
- Compiles the binary
- Verifies successful build
- Binary set with version commit SHA

#### Docker
- Builds Docker image using multi-stage builds
- Tests image build success
- Caches layers for speed

#### Security Scan
- Runs Trivy vulnerability scanner on filesystem
- Scans for critical and high severity issues
- Uploads results to GitHub Security tab (SARIF format)
- Allows high-confidence false positives

### 2. Release Automation (`.github/workflows/release.yml`)

**Trigger**: Every push to `main`

**Functionality**:
- Uses [release-please-action](https://github.com/googleapis/release-please-action)
- Automatically creates release PRs when changes are detected
- Groups changes by type (features, fixes, docs, etc.)
- Bumps version using semantic versioning:
  - **major** (breaking): `v1.0.0`
  - **minor** (feature): `v1.1.0`
  - **patch** (fix): `v1.0.1`

**Changelog Categories**:
- Features (visible)
- Bug Fixes (visible)
- Performance (visible)
- Documentation (visible)
- Tests (hidden, internal)
- Refactoring (hidden, internal)
- Miscellaneous/Chore (hidden, internal)

### 3. Build Release Artifacts (`.github/workflows/release-build.yml`)

**Trigger**: Push of any git tag matching `v*` (e.g., `v1.0.0`)

**Multi-platform Builds**:

#### Binaries
Builds for all major platforms:
- Linux: amd64, arm64
- macOS: amd64, arm64
- Windows: amd64 (as .exe)

Archives:
- `.tar.gz` for Unix-like systems
- `.zip` for Windows

#### Docker Images
- Builds multi-platform images (amd64, arm64)
- Pushes to GitHub Container Registry (GHCR)
- Tags: `vX.Y.Z` and `latest`
- Image repository: `ghcr.io/mieliespoor/renovate-scheduler`

#### Release Notes
- Extracts changelog section from `CHANGELOG.md`
- Includes all changes from release PR
- Attached to GitHub Release

#### GitHub Release
- Created automatically when tag is pushed
- Includes release notes from changelog
- Contains all binary artifacts

### 4. Dependency Management (`.github/dependabot.yml`)

**Automated Dependency Updates**:

#### Go Modules
- Weekly updates (Mondays at 02:00 UTC)
- Max 5 open PRs
- Label: `dependencies`, `go`
- Commit prefix: `chore(deps)`

#### GitHub Actions
- Weekly updates (Mondays at 03:00 UTC)
- Max 5 open PRs
- Label: `dependencies`, `github-actions`
- Commit prefix: `ci(actions)`

#### Docker Base Image
- Weekly updates (Tuesdays at 02:00 UTC)
- Max 3 open PRs
- Label: `dependencies`, `docker`
- Commit prefix: `chore(docker)`

All PRs are assigned to `mieliespoor` for review.

## Complete Release Flow Example

### Step 1: Feature Development
```bash
git checkout -b feat/new-feature
# ... make changes ...
git commit -m "feat(scheduler): add new polling interval configuration"
git push origin feat/new-feature
```

### Step 2: Pull Request Review
- GitHub Actions runs automatically
- ✅ Lint check passes
- ✅ All tests pass
- ✅ Security scan passes
- PR is reviewed and merged

### Step 3: Automatic Release PR (triggered on merge)
- release-please creates PR with:
  - Version bump from `v0.1.0` → `v0.2.0` (minor version)
  - Updated `CHANGELOG.md` with feature
  - Organized by change type

### Step 4: Release PR Review & Merge
- Review and merge release PR
- This triggers release-build workflow

### Step 5: Release Build (triggered on tag)
- Git tag `v0.2.0` is created
- Workflow builds:
  - Multi-platform binaries
  - Docker images for both architectures
- Uploads artifacts to GitHub Release
- Release published with changelog as release notes

## Commit Message Format

The release process depends on [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>
```

**Types** (impacts version bump):
- `feat` → Minor version bump
- `fix` → Patch version bump
- `BREAKING CHANGE` → Major version bump
- `docs`, `test`, `chore`, `refactor` → No version bump (release-please omits)

**Examples**:
```
feat(scheduler): add support for custom polling intervals
fix(ingest): correct digest calculation for file changes
perf(dispatch): optimize concurrent job claiming
docs: update README with configuration examples
test(integration): add end-to-end loop tests
chore(deps): update golangci-lint to v1.60.0
```

## Configuration Files

### `.github/workflows/ci.yml`
Main CI pipeline for quality gates

### `.github/workflows/release.yml`
Automated release PR creation

### `.github/workflows/release-build.yml`
Artifact building on tag creation

### `.github/dependabot.yml`
Automated dependency updates

### `.goreleaser.yaml`
Optional GoReleaser configuration (alternative release tooling)

### `CHANGELOG.md`
Hand-maintained changelog for pre-automation releases (release-please updates this)

## Secrets Required

None! The pipelines use:
- `GITHUB_TOKEN` (automatically provided by GitHub)
- Public image registry (GHCR)

## Notifications & Monitoring

### Status Checks
- All checks must pass before merging PRs
- Branch protection rules enforce this

### Coverage Tracking
- Codecov integration tracks coverage trends
- Reports on PR diffs

### Vulnerability Alerts
- GitHub automatically monitors security advisories
- Trivy scans on each build

### Dependabot Alerts
- Auto-PRs for dependency updates
- Review and merge to keep dependencies current

## Adding New Workflows

When adding new workflows:

1. Create file in `.github/workflows/new-workflow.yml`
2. Define triggers (on: pull_request, push, schedule, etc.)
3. Add jobs with clear names
4. Use consistent style and error handling
5. Document in this guide

## Troubleshooting

### PR checks fail
- Check workflow logs in GitHub Actions tab
- Common issues:
  - Linting: Run `golangci-lint run` locally
  - Tests: Run `go test -race ./...` locally
  - Build: Check `go build ./...`

### Release not created
- Check if commits follow conventional commit format
- Verify commit is on `main` branch
- Release-please requires specific commit patterns

### Docker image not pushed
- Verify GHCR credentials are configured
- Check GitHub Actions have package:write permission

### Dependabot PRs not created
- Check `.github/dependabot.yml` is valid YAML
- Verify schedule times (consider timezone)
- Enable Dependabot in repository settings

## Best Practices

1. **Commit Messages**: Use conventional commits for accurate versioning
2. **Testing**: Write tests for all new features
3. **Dependencies**: Review Dependabot PRs timely to stay current
4. **Releases**: Let release-please handle versioning (don't manually tag)
5. **Security**: Address security scan findings before merge
6. **Documentation**: Keep CHANGELOG updated with user-facing changes

## Links

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [release-please](https://github.com/googleapis/release-please-action)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [golangci-lint](https://golangci-lint.run/)
- [Trivy](https://github.com/aquasecurity/trivy)
- [Dependabot](https://docs.github.com/en/code-security/dependabot)

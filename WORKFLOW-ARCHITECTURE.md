# GitHub Actions Workflow Architecture

## High-Level Pipeline Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Code Changes                                 │
└────────┬────────────────────────────────────────┬───────────────┘
         │                                        │
         │                                        │
    ┌────▼─────────────┐               ┌─────────▼──────────┐
    │  Pull Request    │               │  Push to main      │
    │   to main        │               │                    │
    └────┬─────────────┘               └─────────┬──────────┘
         │                                        │
         └────────────┬─────────────────────────┘
                      │
                      ▼
            ┌─────────────────────┐
            │   CI Pipeline       │  (.github/workflows/ci.yml)
            │  (ci.yml)           │
            └─────────┬───────────┘
                      │
        ┌─────────────┼─────────────┬──────────────┬───────────┐
        │             │             │              │           │
        ▼             ▼             ▼              ▼           ▼
    ┌─────────┐ ┌─────────┐ ┌────────────┐ ┌──────────┐ ┌──────────┐
    │  Lint   │ │  Test   │ │   Build    │ │  Docker  │ │Security  │
    │ golang  │ │ go test │ │    go      │ │  build   │ │  Trivy   │
    │ -lint   │ │ -race   │ │   build    │ │   image  │ │  scan    │
    └────┬────┘ └────┬────┘ └────┬───────┘ └────┬─────┘ └────┬─────┘
         │           │           │               │           │
         └───────────┼───────────┼───────────────┼───────────┘
                     │
                     ▼
            ┌──────────────────┐
            │  All Checks ✓    │
            │  Merge Approved  │
            └────────┬─────────┘
                     │
                     ▼
    ┌────────────────────────────────────┐
    │  Release Please Auto PR            │  (.github/workflows/release.yml)
    │  (on every main push)              │
    │                                    │
    │  - Bumps version (semver)          │
    │  - Updates CHANGELOG.md            │
    │  - Groups changes by type          │
    └────────┬──────────────────────────┘
             │
             ▼
    ┌──────────────────────┐
    │  Review Release PR   │
    │  Approve & Merge     │
    └────────┬─────────────┘
             │
             ▼
    ┌──────────────────────────────────┐
    │  Git Tag Created                 │
    │  (e.g., v0.2.0)                  │
    │  by release-please               │
    └────────┬─────────────────────────┘
             │
             ▼
    ┌──────────────────────────────────┐
    │  Release Build Pipeline          │  (.github/workflows/release-build.yml)
    │  (triggered on tag)              │
    └────────┬────────────────────────┘
             │
        ┌────┴────┬─────────┬──────────┐
        │          │         │          │
        ▼          ▼         ▼          ▼
    ┌──────┐ ┌──────────┐ ┌─────┐ ┌──────────┐
    │Build │ │Push to   │ │Push │ │Generate  │
    │Binary│ │GHCR      │ │Tags │ │Release   │
    │(6x)  │ │(2 arch)  │ │(1x) │ │Notes     │
    └────┬─┘ └────┬─────┘ └──┬──┘ └────┬─────┘
         │        │          │         │
         └────────┼──────────┼────────┘
                  │          │
                  ▼          ▼
        ┌──────────────────────────┐
        │  GitHub Release          │
        │  - Artifacts             │
        │  - Docker images         │
        │  - Release notes         │
        │  - Checksums             │
        └──────────────────────────┘
```

## Parallel Dependencies

```
Every Push to main → Release-Please PR Creation
Every PR/Push      → CI Tests (all run in parallel)
Weekly             → Dependabot Updates (Go, Actions, Docker)
Tagged Push        → Multi-platform Release Build
```

## Commit Message Impact on Versioning

```
Previous Release: v0.1.0

Commit: "feat(scheduler): new feature"
  → Version bump: MINOR
  → Next release: v0.2.0
  → Type: FEATURE (visible in changelog)

Commit: "fix(ingest): bug fix"
  → Version bump: PATCH
  → Next release: v0.2.1
  → Type: BUG FIX (visible in changelog)

Commit: "test: add tests"
  → Version bump: NONE (skip release)
  → No changelog entry
  → Type: TEST (hidden)

Commit: "BREAKING CHANGE: renamed API field"
  → Version bump: MAJOR
  → Next release: v1.0.0
  → Type: BREAKING (visible in changelog)
```

## Detailed CI Job Flow

```
┌─ Lint Job ─────────────────────┐
│ 1. Checkout                     │
│ 2. Setup Go 1.26               │
│ 3. Run golangci-lint           │
│ 4. Verify go.mod/go.sum sync   │
│ Duration: ~2-3 minutes         │
└────────────────────────────────┘

┌─ Test Job ─────────────────────┐
│ 1. Checkout                     │
│ 2. Setup Go 1.26               │
│ 3. Run go test -race -cover    │
│ 4. Upload coverage (Codecov)   │
│ Duration: ~3-4 minutes         │
└────────────────────────────────┘

┌─ Build Job ─────────────────────┐
│ 1. Checkout                      │
│ 2. Setup Go 1.26                │
│ 3. Compile binary               │
│ 4. Verify success               │
│ Duration: ~1-2 minutes          │
└─────────────────────────────────┘

┌─ Docker Job ────────────────────┐
│ 1. Checkout                      │
│ 2. Setup Buildx                 │
│ 3. Build image                  │
│ 4. Cache layers (GHA)           │
│ Duration: ~3-5 minutes          │
└─────────────────────────────────┘

┌─ Security Scan ─────────────────┐
│ 1. Checkout                      │
│ 2. Setup Go 1.26                │
│ 3. Run Trivy scan               │
│ 4. Upload SARIF                 │
│ Duration: ~2-3 minutes          │
└─────────────────────────────────┘

All run in PARALLEL → Total: ~5-10 min
```

## Release Build Job Details (Multi-Platform)

```
Build Binary × 6 Configurations (parallel):
  ├─ Linux x86_64        → renovate-scheduler-linux-amd64.tar.gz
  ├─ Linux ARM64         → renovate-scheduler-linux-arm64.tar.gz
  ├─ macOS x86_64        → renovate-scheduler-darwin-amd64.tar.gz
  ├─ macOS ARM64         → renovate-scheduler-darwin-arm64.tar.gz
  ├─ Windows x86_64      → renovate-scheduler-windows-amd64.zip
  └─ Windows ARM64       → renovate-scheduler-windows-arm64.zip

Build Docker Image × 2 Architectures (parallel):
  ├─ linux/amd64  → ghcr.io/mieliespoor/renovate-scheduler:v0.2.0
  └─ linux/arm64  → ghcr.io/mieliespoor/renovate-scheduler:v0.2.0
                    (both tagged as multi-arch manifest)

Additional:
  ├─ Generate checksums.txt
  ├─ Extract changelog section
  └─ Create GitHub Release with all artifacts
```

## Dependabot Update Schedule

```
Weekly Schedule (UTC):

Monday 02:00 → Go module updates
Monday 03:00 → GitHub Actions updates
Tuesday 02:00 → Docker base image updates

All create separate PRs with:
  - Auto-generated commit message (chore/ci prefix)
  - Labels: dependencies + category
  - Assigned to: mieliespoor
```

## Status Checks & Protections

```
Before merging to main, require:
  ✓ CI / Lint
  ✓ CI / Test
  ✓ CI / Build
  ✓ CI / Docker
  ✓ CI / Security
  ✓ Approved by maintainer
  ✓ No merge conflicts

Automatic on release-please PR:
  - Version bump approved
  - Changelog reviewed
  - Merge triggers release build
```

## Environment & Resources

```
Runs On:
  - ubuntu-latest (GitHub hosted runner)

Go Version:
  - 1.26

Registries:
  - GitHub Container Registry (GHCR)

Integrations:
  - Codecov (coverage)
  - GitHub Security (SARIF)
  - Dependabot (dependencies)
```

## Artifact Retention & Distribution

```
GitHub Release Artifacts (permanent):
  - v0.2.0-linux-amd64.tar.gz
  - v0.2.0-linux-arm64.tar.gz
  - v0.2.0-darwin-amd64.tar.gz
  - v0.2.0-darwin-arm64.tar.gz
  - v0.2.0-windows-amd64.zip
  - v0.2.0-windows-arm64.zip
  - checksums.txt
  - Release notes (from CHANGELOG)

Docker Images (permanent):
  - ghcr.io/mieliespoor/renovate-scheduler:v0.2.0
  - ghcr.io/mieliespoor/renovate-scheduler:latest (updated each release)

Workflow Artifacts (30 days):
  - Build outputs
  - Coverage reports
  - Scan results
```

## Key Metrics & Monitoring

```
Per Workflow Run:
  - Build time: 5-10 minutes
  - Job count: 5 (CI) / 3 (Release build)
  - Success rate: Should be 100%

Coverage:
  - Tracked per PR
  - Reported in PR comments
  - Historical trends on Codecov

Dependency Age:
  - Dependabot keeps within 1 week
  - Security updates: ASAP

Security:
  - Trivy scans all code changes
  - Results: GitHub Security tab
  - No secrets in logs
```

# Quick Reference: GitHub Actions Setup

## 📁 Files Created (13 new/updated)

### Workflows (3 files)
```
.github/workflows/ci.yml              → Lint, Test, Build, Docker, Security
.github/workflows/release.yml         → Auto release PR creation
.github/workflows/release-build.yml   → Build artifacts on tag
```

### Configuration (2 files)
```
.github/dependabot.yml                → Weekly dependency updates
.goreleaser.yaml                      → GoReleaser config (alternative)
```

### Documentation (5 files)
```
CHANGELOG.md                          → Release notes (auto-updated)
CONTRIBUTING.md                       → Developer guide
CI-CD-GUIDE.md                       → Complete CI/CD docs
GITHUB-ACTIONS-SETUP.md              → Setup overview
WORKFLOW-ARCHITECTURE.md             → Visual pipeline diagrams
SETUP-CHECKLIST.md                   → Verification checklist
```

### Templates (3 files)
```
.github/ISSUE_TEMPLATE/bug.md        → Bug report template
.github/ISSUE_TEMPLATE/feature_request.md → Feature request template
.github/pull_request_template.md     → PR template
```

### Updated (2 files)
```
main.go                               → Added -version flag
.gitignore                            → Build/release artifacts
```

---

## 🚀 Quick Start

### Before Pushing
```bash
# Make sure tests pass
go test ./... -v

# Verify build works
go build -o renovate-scheduler .

# Check version flag
./renovate-scheduler -version
```

### Push to GitHub
```bash
git add .github/ CHANGELOG.md CONTRIBUTING.md CI-CD-GUIDE.md \
        GITHUB-ACTIONS-SETUP.md WORKFLOW-ARCHITECTURE.md \
        SETUP-CHECKLIST.md main.go .gitignore

git commit -m "ci: add comprehensive GitHub Actions CI/CD pipeline"
git push
```

### Watch CI Run
- Go to GitHub repository
- Click "Actions" tab
- Watch workflows execute
- All 5 checks should pass ✅

---

## 📋 Release Workflow (Simple)

```
1. Make code changes
2. Commit: git commit -m "feat(name): description"
3. PR & merge to main
4. Wait ~1 min → release-please creates release PR
5. Merge release PR → git tag created
6. Wait ~20 min → artifacts built & released
```

---

## 🔑 Key Commit Message Formats

```
feat(...)     → Minor version bump (feature)
fix(...)      → Patch version bump (bug fix)
BREAKING ...  → Major version bump (breaking change)
test(...) / chore(...) / docs(...) → No version bump
```

---

## 📊 Pipeline Performance

| Stage | Duration | Details |
|-------|----------|---------|
| **CI** | 5-10 min | Lint, test, build, docker, security (parallel) |
| **Release PR** | 1-2 min | Automatic on merge to main |
| **Build Release** | 15-20 min | 6 binaries + docker images + release |

---

## ✅ What Gets Automated

| Task | Status | When |
|------|--------|------|
| Linting | ✅ Automated | Every PR/push |
| Testing | ✅ Automated | Every PR/push |
| Security Scan | ✅ Automated | Every PR/push |
| Code Coverage | ✅ Automated | Every PR/push |
| Version Bump | ✅ Automated | Release PR merge |
| Changelog | ✅ Automated | Release PR merge |
| Binary Build | ✅ Automated | Tag push |
| Docker Build | ✅ Automated | Tag push |
| Release Creation | ✅ Automated | Tag push |
| Dependency Updates | ✅ Automated | Weekly |

---

## 🛠️ Common Commands

```bash
# Run CI locally before pushing
go test -race ./...
golangci-lint run ./...
go build ./...

# View version
./renovate-scheduler -version

# Build specific platform
GOOS=linux GOARCH=amd64 go build -o renovate-scheduler .

# Build Docker image
docker build -t renovate-scheduler:dev .
```

---

## 📖 Documentation Hierarchy

**Start Here:**
- `README.md` (main project doc)

**For Developers:**
- `CONTRIBUTING.md` (how to contribute)
- `CI-CD-GUIDE.md` (CI/CD details)

**For Understanding:**
- `WORKFLOW-ARCHITECTURE.md` (visual diagrams)
- `GITHUB-ACTIONS-SETUP.md` (complete overview)

**For Verification:**
- `SETUP-CHECKLIST.md` (before deployment)

---

## 🎯 What Each Workflow Does

### `ci.yml` (5 jobs, ~10 min)
✅ Lint code
✅ Run tests with race detection
✅ Build binary
✅ Build Docker image
✅ Scan for vulnerabilities

### `release.yml` (1 job, ~1 min)
✅ Create release PR
✅ Auto-bump version
✅ Auto-generate changelog

### `release-build.yml` (3 jobs, ~20 min)
✅ Build 6 platform binaries
✅ Build multi-arch Docker image
✅ Create GitHub Release
✅ Upload all artifacts

### `dependabot.yml` (config)
✅ Weekly Go module updates
✅ Weekly GitHub Actions updates
✅ Weekly Docker base image updates

---

## 🔐 Security Features

✅ Trivy vulnerability scanning
✅ SARIF integration with GitHub
✅ Dependabot for dependencies
✅ Branch protection rules
✅ Required PR reviews
✅ Status checks enforcement
✅ No hardcoded secrets

---

## 💡 Pro Tips

1. **Commit Messages Matter**
   - Use conventional commits for correct versioning
   - `feat:` → Feature release, `fix:` → Bug fix

2. **Review Dependabot PRs**
   - Keep dependencies up-to-date
   - Merge security updates quickly

3. **Watch Release PR**
   - Verify changelog looks good
   - Double-check version bump
   - Then merge to create release

4. **Monitor Release Build**
   - Check Actions for build success
   - Verify artifacts on GitHub Release
   - Docker images pushed to GHCR

---

## 🆘 Troubleshooting

**CI Fails?**
- Run `go test ./...` locally
- Run `golangci-lint run` locally
- Check workflow logs on Actions tab

**Release PR Not Created?**
- Check commits use conventional format
- Ensure push is to main branch
- Wait 1-2 minutes

**Artifacts Missing?**
- Verify git tag was created
- Check release-build workflow logs
- Ensure GHCR permissions configured

---

## 📞 Quick Links

- **GitHub Actions Docs**: https://docs.github.com/en/actions
- **release-please**: https://github.com/googleapis/release-please-action
- **Conventional Commits**: https://www.conventionalcommits.org/
- **golangci-lint**: https://golangci-lint.run/

---

**Status**: ✅ Ready to deploy
**Last Updated**: 2026-06-12
**Go Version**: 1.26
**Total Workflows**: 3 active + 1 config

# Phase 6: Build & CI Hardening - Working Notes

## What Was Done

### Makefile
- Created `Makefile` with targets: `build`, `test`, `vet`, `lint`, `clean`, `run`, `docker`, `tools`, `vuln`, `all`
- VERSION injection via `git describe --tags --always --dirty` with fallback to `dev`
- `lint` target installs golangci-lint if missing
- `vuln` target installs govulncheck if missing
- `docker` target builds with VERSION arg

### .golangci.yml
- Enabled linters: errcheck, govet, staticcheck, unused, ineffassign, gosec, gosimple, gocritic
- Excluded G304 (file path taint) from gosec since we validate save_path in Phase 4
- Relaxed errcheck for blank checks, excluded errcheck/gosec from test files

### Dockerfile
- Multi-stage build: `golang:1.25-alpine` builder → `gcr.io/distroless/static-debian12:nonroot`
- CGO_ENABLED=0 for static binary
- VERSION build arg with ldflags injection
- Runs as nonroot:nonroot user

### CI Workflow (.github/workflows/ci.yml)
- Triggers on push to main/dev and PRs to main
- Three jobs: test (go test -race), lint (golangci-lint), vuln (govulncheck)
- Uses `go-version-file: 'go.mod'` so CI always matches project Go version

### Dependabot (.github/dependabot.yml)
- Weekly schedule for both `gomod` and `github-actions` ecosystems
- 5 open PR limit per ecosystem

### Release Workflow Updates
- Updated `.github/workflows/release.yml` to inject VERSION via ldflags: `-X main.version=${VERSION}`
- Changed from hardcoded `go-version: '1.21'` to `go-version-file: 'go.mod'` (done in Phase 1)

## Findings Addressed
- F15: CI version mismatch → go-version-file: 'go.mod'
- Build/CI hardening items from audit

## Verification
- `go build` ✓
- `go vet ./...` ✓
- `go test -race ./...` ✓ (all pass)

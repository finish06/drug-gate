# Spec: Version Endpoint

**Version:** 0.1.0
**Created:** 2026-03-16
**PRD Reference:** docs/prd.md
**Status:** Complete

## 1. Overview

A public endpoint that returns build version metadata — version tag, git commit hash, git branch, and Go version. Mirrors the `/version` endpoint provided by cash-drugs for consistency across the stack. Useful for deployment verification, debugging, and monitoring dashboards.

### User Story

As an **operator**, I want to **query `/version` on drug-gate and see the exact build running**, so that **I can verify deployments, diagnose issues, and confirm which commit is in production without checking container logs**.

## 2. Acceptance Criteria

| ID | Criterion | Priority |
|----|-----------|----------|
| AC-001 | `GET /version` returns 200 with JSON containing `version`, `git_commit`, `git_branch`, and `go_version` fields | Must |
| AC-002 | `version` field matches the build tag (e.g., `v0.5.0`, `beta-abc1234d`) | Must |
| AC-003 | `git_commit` is the short (8-char) commit hash at build time | Must |
| AC-004 | `git_branch` is the branch name at build time | Must |
| AC-005 | `go_version` is the Go runtime version (from `runtime.Version()`) | Must |
| AC-006 | Endpoint is public (no auth required) | Must |
| AC-007 | All fields default to `"dev"` or `"unknown"` for local builds without ldflags | Should |
| AC-008 | Values are injected at build time via `-ldflags`, not read from git at runtime | Must |

## 3. User Test Cases

### TC-001: Version endpoint returns build info

**Precondition:** drug-gate is running
**Steps:**
1. Send `GET /version`
2. Observe response
**Expected Result:** 200 OK with:
```json
{
  "version": "v0.5.0",
  "git_commit": "abc1234d",
  "git_branch": "main",
  "go_version": "go1.26"
}
```
**Maps to:** TBD

### TC-002: No auth required

**Steps:**
1. Send `GET /version` without any auth headers
2. Observe response
**Expected Result:** 200 OK (not 401)
**Maps to:** TBD

### TC-003: Local dev build defaults

**Precondition:** Running locally without ldflags (`go run ./cmd/server`)
**Steps:**
1. Send `GET /version`
2. Observe response
**Expected Result:** 200 OK with `version: "dev"`, `git_commit: "unknown"`, `git_branch: "unknown"`, `go_version: "go1.26"`
**Maps to:** TBD

## 4. Data Model

### VersionResponse

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| version | string | Yes | Build version tag (e.g., `v0.5.0`, `beta-abc1234d`, `dev`) |
| git_commit | string | Yes | Short git commit hash (8 chars) |
| git_branch | string | Yes | Git branch at build time |
| go_version | string | Yes | Go runtime version |

## 5. API Contract

### GET /version

**Description:** Returns build version metadata. Public, no authentication required.

**Response (200):**
```json
{
  "version": "v0.5.0",
  "git_commit": "abc1234d",
  "git_branch": "main",
  "go_version": "go1.26"
}
```

## 6. Build Integration

### ldflags injection

The existing `Version` variable is set via `-ldflags`. Two new variables (`GitCommit`, `GitBranch`) need to be added to `internal/version/version.go` and injected the same way.

**Dockerfile:**
```dockerfile
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG GIT_BRANCH=unknown
RUN CGO_ENABLED=0 go build -ldflags="-s -w \
  -X github.com/finish06/drug-gate/internal/version.Version=${VERSION} \
  -X github.com/finish06/drug-gate/internal/version.GitCommit=${GIT_COMMIT} \
  -X github.com/finish06/drug-gate/internal/version.GitBranch=${GIT_BRANCH}" \
  -o /server ./cmd/server
```

**CI (GitHub Actions):**
```yaml
build-args: |
  VERSION=${{ steps.version.outputs.tag }}
  GIT_COMMIT=${{ github.sha }}
  GIT_BRANCH=${{ github.ref_name }}
```

## 7. Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Local build without ldflags | All fields default (`dev`, `unknown`, `unknown`). `go_version` always populated from `runtime.Version()`. |
| HEAD is detached (CI) | `git_branch` may be `HEAD` or the ref name — depends on CI checkout. Use `github.ref_name` in CI. |

## 8. Dependencies

- `internal/version` package (already exists, needs `GitCommit` and `GitBranch` vars)
- Dockerfile and CI workflow need updated build args

## 9. Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-03-16 | 0.1.0 | calebdunn | Initial spec |

# Implementation Plan: Docker Build & Publish

**Spec Version**: 0.1.0
**Created**: 2026-03-08
**Status**: Complete (retroactive)
**Team Size**: Solo
**Estimated Duration**: 1 hour (actual: ~45 min)

## Overview

Add automated Docker image build and publish to CI pipeline. Images push to private registry with `:beta` on main and `:vX.Y.Z` + `:latest` on git tags. Health endpoint reports embedded build version.

## Implementation Phases

### Phase 1: Version Package

| Task ID | Description | Status |
|---------|-------------|--------|
| TASK-001 | Create `internal/version/version.go` with `Version` var defaulting to `"dev"` | Done |
| TASK-002 | Write `version_test.go` verifying default value | Done |

### Phase 2: Health Endpoint Update

| Task ID | Description | Status |
|---------|-------------|--------|
| TASK-003 | Import version package in `health.go` | Done |
| TASK-004 | Add `version` field to health JSON response | Done |
| TASK-005 | Write `TestHealthCheck_AC003_IncludesVersion` | Done |

### Phase 3: Build Configuration

| Task ID | Description | Status |
|---------|-------------|--------|
| TASK-006 | Update Dockerfile `-ldflags` to use `internal/version.Version` | Done |
| TASK-007 | Update Makefile with `VERSION` variable and lint target | Done |
| TASK-008 | Update Go version in Dockerfile (1.24 → 1.26) | Done |

### Phase 4: CI Publish Job

| Task ID | Description | Status |
|---------|-------------|--------|
| TASK-009 | Add `publish` job to `ci.yml` with `needs: test` | Done |
| TASK-010 | Add version detection step (tag vs beta-SHA) | Done |
| TASK-011 | Add registry login via `docker/login-action` | Done |
| TASK-012 | Add buildx setup for `linux/amd64` | Done |
| TASK-013 | Add beta push step (main branch only) | Done |
| TASK-014 | Add release push step (v* tags: versioned + latest) | Done |
| TASK-015 | Configure GitHub secrets (REGISTRY_USERNAME, REGISTRY_PASSWORD) | Done |
| TASK-016 | Update Go version in CI (1.24 → 1.26) | Done |
| TASK-017 | Fix coverage step to exclude cmd/ (covdata tool error) | Done |
| TASK-018 | Fix .gitignore `server` pattern matching cmd/server/ | Done |

## Issues Encountered

1. `.gitignore` had `server` pattern that matched `cmd/server/` directory — fixed to `/server`
2. Go version mismatch: go.mod required 1.25.5, Dockerfile used 1.24 — updated to 1.26
3. `go tool covdata` not available in CI runner — excluded cmd/ from coverage run

## Verification

- CI pipeline: test + publish both green
- Beta image pushed to `dockerhub.calebdunn.tech/finish06/drug-gate:beta`
- 34 tests passing, golangci-lint clean

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/finish06/drug-gate/internal/version"
)

// AC-016: All pre-existing fields remain present with correct values.
func TestVersion_AC016_PreservesExistingFields(t *testing.T) {
	origVersion := version.Version
	origCommit := version.GitCommit
	origBranch := version.GitBranch
	defer func() {
		version.Version = origVersion
		version.GitCommit = origCommit
		version.GitBranch = origBranch
	}()

	version.Version = "v0.5.0"
	version.GitCommit = "abc1234d"
	version.GitBranch = "main"

	rr := httptest.NewRecorder()
	VersionInfo(rr, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Version != "v0.5.0" {
		t.Errorf("version = %q, want v0.5.0", resp.Version)
	}
	if resp.GitCommit != "abc1234d" {
		t.Errorf("git_commit = %q, want abc1234d", resp.GitCommit)
	}
	if resp.GitBranch != "main" {
		t.Errorf("git_branch = %q, want main", resp.GitBranch)
	}
	if resp.GoVersion != runtime.Version() {
		t.Errorf("go_version = %q, want %q", resp.GoVersion, runtime.Version())
	}
}

// AC-017, AC-018: os and arch are sourced from runtime.
func TestVersion_AC017_OSArchFromRuntime(t *testing.T) {
	rr := httptest.NewRecorder()
	VersionInfo(rr, httptest.NewRequest(http.MethodGet, "/version", nil))

	var resp VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.OS != runtime.GOOS {
		t.Errorf("os = %q, want %q", resp.OS, runtime.GOOS)
	}
	if resp.Arch != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", resp.Arch, runtime.GOARCH)
	}
}

// AC-019: build_time is sourced from version.BuildTime.
func TestVersion_AC019_BuildTimeFromLdflags(t *testing.T) {
	orig := version.BuildTime
	version.BuildTime = "2026-04-11T12:00:00Z"
	defer func() { version.BuildTime = orig }()

	rr := httptest.NewRecorder()
	VersionInfo(rr, httptest.NewRequest(http.MethodGet, "/version", nil))

	var resp VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.BuildTime != "2026-04-11T12:00:00Z" {
		t.Errorf("build_time = %q, want 2026-04-11T12:00:00Z", resp.BuildTime)
	}
}

// AC-019: Default (no ldflag) falls through to "unknown".
func TestVersion_AC019_BuildTimeDefaultUnknown(t *testing.T) {
	orig := version.BuildTime
	version.BuildTime = "unknown"
	defer func() { version.BuildTime = orig }()

	rr := httptest.NewRecorder()
	VersionInfo(rr, httptest.NewRequest(http.MethodGet, "/version", nil))

	var resp VersionResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.BuildTime != "unknown" {
		t.Errorf("build_time = %q, want unknown", resp.BuildTime)
	}
}

// Sanity: endpoint is unauthenticated.
func TestVersion_NoAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	VersionInfo(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

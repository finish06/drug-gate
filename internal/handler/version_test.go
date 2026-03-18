package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/finish06/drug-gate/internal/version"
)

func TestVersionHandler_ReturnsAllFields(t *testing.T) {
	// Set test values
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

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	VersionInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if resp["version"] != "v0.5.0" {
		t.Errorf("version = %q, want %q", resp["version"], "v0.5.0")
	}
	if resp["git_commit"] != "abc1234d" {
		t.Errorf("git_commit = %q, want %q", resp["git_commit"], "abc1234d")
	}
	if resp["git_branch"] != "main" {
		t.Errorf("git_branch = %q, want %q", resp["git_branch"], "main")
	}
	if resp["go_version"] != runtime.Version() {
		t.Errorf("go_version = %q, want %q", resp["go_version"], runtime.Version())
	}
}

func TestVersionHandler_DefaultValues(t *testing.T) {
	origVersion := version.Version
	origCommit := version.GitCommit
	origBranch := version.GitBranch
	defer func() {
		version.Version = origVersion
		version.GitCommit = origCommit
		version.GitBranch = origBranch
	}()

	version.Version = "dev"
	version.GitCommit = "unknown"
	version.GitBranch = "unknown"

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	VersionInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if resp["version"] != "dev" {
		t.Errorf("version = %q, want %q", resp["version"], "dev")
	}
	if resp["git_commit"] != "unknown" {
		t.Errorf("git_commit = %q, want %q", resp["git_commit"], "unknown")
	}
	// go_version is always populated from runtime
	if resp["go_version"] == "" {
		t.Error("go_version should never be empty")
	}
}

func TestVersionHandler_NoAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	VersionInfo(w, req)

	// Should return 200, not 401 — public endpoint
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (no auth required)", w.Code)
	}
}

package version

import (
	"testing"
)

// AC-002: Version is set via ldflags at build time
func TestVersion_Default(t *testing.T) {
	// When built without -ldflags, version should default to "dev"
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Version != "dev" {
		t.Errorf("Version = %q, want %q (default)", Version, "dev")
	}
}

// AC-020: BuildTime defaults to "unknown" when not injected via ldflags.
func TestVersion_BuildTimeDefault(t *testing.T) {
	if BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if BuildTime != "unknown" {
		t.Errorf("BuildTime = %q, want %q (default)", BuildTime, "unknown")
	}
}

package service

import (
	"testing"
	"time"
)

func TestSetCacheTTL_OverridesDefault(t *testing.T) {
	// Save and restore original
	original := CacheTTL
	defer func() { CacheTTL = original }()

	SetCacheTTL(30 * time.Minute)
	if CacheTTL != 30*time.Minute {
		t.Errorf("expected 30m, got %v", CacheTTL)
	}
}

func TestDefaultCacheTTL_Is60Minutes(t *testing.T) {
	if DefaultCacheTTL != 60*time.Minute {
		t.Errorf("expected 60m default, got %v", DefaultCacheTTL)
	}
}

func TestRxNormTTL_ScalesFromBase(t *testing.T) {
	original := CacheTTL
	defer func() { CacheTTL = original }()

	// Default base: 60m → search=24h, lookup=7d
	CacheTTL = 60 * time.Minute
	if rxnormSearchTTL() != 24*time.Hour {
		t.Errorf("search TTL at 60m base: expected 24h, got %v", rxnormSearchTTL())
	}
	if rxnormLookupTTL() != 7*24*time.Hour {
		t.Errorf("lookup TTL at 60m base: expected 168h, got %v", rxnormLookupTTL())
	}

	// Halved base: 30m → search=12h, lookup=3.5d
	CacheTTL = 30 * time.Minute
	if rxnormSearchTTL() != 12*time.Hour {
		t.Errorf("search TTL at 30m base: expected 12h, got %v", rxnormSearchTTL())
	}
	if rxnormLookupTTL() != 84*time.Hour {
		t.Errorf("lookup TTL at 30m base: expected 84h, got %v", rxnormLookupTTL())
	}
}

func TestCacheTTL_UsedByServices(t *testing.T) {
	original := CacheTTL
	defer func() { CacheTTL = original }()

	// Verify CacheTTL is the variable used (not a const)
	SetCacheTTL(15 * time.Minute)
	if CacheTTL != 15*time.Minute {
		t.Errorf("SetCacheTTL did not update CacheTTL: got %v", CacheTTL)
	}
}

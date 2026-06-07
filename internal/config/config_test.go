package config

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_Empty_AppliesDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultProfile != ProfileCoding {
		t.Fatalf("default profile: want %q got %q", ProfileCoding, cfg.DefaultProfile)
	}
	if cfg.StoreDir == "" {
		t.Fatal("storeDir must not be empty after defaults")
	}
	if len(cfg.Profiles) == 0 {
		t.Fatal("profiles must be populated from defaults")
	}
}

func TestLoad_EmptyJSON_AppliesDefaults(t *testing.T) {
	cfg, err := Load([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultProfile != ProfileCoding {
		t.Fatalf("want coding default, got %q", cfg.DefaultProfile)
	}
}

func TestLoad_OverrideDefaultProfile(t *testing.T) {
	json := []byte(`{"defaultProfile":"ci"}`)
	cfg, err := Load(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultProfile != ProfileCI {
		t.Fatalf("want ci got %q", cfg.DefaultProfile)
	}
}

func TestLoad_OverrideStoreDir(t *testing.T) {
	json := []byte(`{"storeDir":"/tmp/sextant"}`)
	cfg, err := Load(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StoreDir != "/tmp/sextant" {
		t.Fatalf("want /tmp/sextant got %q", cfg.StoreDir)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	_, err := Load([]byte(`{not json}`))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate_DefaultConfig_OK(t *testing.T) {
	cfg, _ := Load(nil)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidate_MissingStoreDir(t *testing.T) {
	cfg, _ := Load(nil)
	cfg.StoreDir = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty storeDir")
	}
}

func TestValidate_MissingDefaultProfile(t *testing.T) {
	cfg, _ := Load(nil)
	cfg.DefaultProfile = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty defaultProfile")
	}
}

func TestValidate_UnknownDefaultProfile(t *testing.T) {
	cfg, _ := Load(nil)
	cfg.DefaultProfile = "nonexistent"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown defaultProfile")
	}
}

func TestValidate_BadSemanticWeight(t *testing.T) {
	cfg, _ := Load(nil)
	p := cfg.Profiles[ProfileCoding]
	p.SemanticWeight = 1.5
	cfg.Profiles[ProfileCoding] = p
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for semanticWeight > 1")
	}
}

func TestValidate_NegativeSemanticWeight(t *testing.T) {
	cfg, _ := Load(nil)
	p := cfg.Profiles[ProfileCoding]
	p.SemanticWeight = -0.1
	cfg.Profiles[ProfileCoding] = p
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative semanticWeight")
	}
}

func TestValidate_NegativeMaxResults(t *testing.T) {
	cfg, _ := Load(nil)
	p := cfg.Profiles[ProfileCI]
	p.MaxResults = -1
	cfg.Profiles[ProfileCI] = p
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative maxResults")
	}
}

// ---------------------------------------------------------------------------
// Profile constants
// ---------------------------------------------------------------------------

func TestProfileConstants_AllPresent(t *testing.T) {
	cfg, _ := Load(nil)
	for _, name := range []string{ProfileCoding, ProfileInvestigation, ProfileCI, ProfilePlatform} {
		if _, ok := cfg.Profile(name); !ok {
			t.Errorf("profile %q missing from defaults", name)
		}
	}
}

func TestProfile_MissingReturnsNotFound(t *testing.T) {
	cfg, _ := Load(nil)
	_, ok := cfg.Profile("unknown-profile")
	if ok {
		t.Fatal("expected not-found for unknown profile")
	}
}

func TestProfile_KnownReturnsValue(t *testing.T) {
	cfg, _ := Load(nil)
	p, ok := cfg.Profile(ProfileInvestigation)
	if !ok {
		t.Fatal("investigation profile must exist")
	}
	if p.MaxResults <= 0 {
		t.Fatalf("investigation maxResults should be > 0, got %d", p.MaxResults)
	}
}

// ---------------------------------------------------------------------------
// Profile semantic values sanity check
// ---------------------------------------------------------------------------

func TestCIProfile_HasZeroSemanticWeight(t *testing.T) {
	cfg, _ := Load(nil)
	p, _ := cfg.Profile(ProfileCI)
	if p.SemanticWeight != 0.0 {
		t.Fatalf("ci profile semanticWeight should be 0, got %v", p.SemanticWeight)
	}
}

func TestInvestigationProfile_EnablesCostTracking(t *testing.T) {
	cfg, _ := Load(nil)
	p, _ := cfg.Profile(ProfileInvestigation)
	if !p.EnableCostTracking {
		t.Fatal("investigation profile must enable cost tracking")
	}
}

// ---------------------------------------------------------------------------
// Load with partial profiles override
// ---------------------------------------------------------------------------

func TestLoad_PartialProfileOverride(t *testing.T) {
	json := []byte(`{"profiles":{"coding":{"maxResults":99}}}`)
	cfg, err := Load(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, ok := cfg.Profile(ProfileCoding)
	if !ok {
		t.Fatal("coding profile missing")
	}
	if p.MaxResults != 99 {
		t.Fatalf("want maxResults=99 got %d", p.MaxResults)
	}
	// investigation profile dropped — JSON fully replaces the profiles map.
	// This is expected merge behaviour: we don't deep-merge sub-maps.
	_ = strings.Contains("", "") // keep import
}

package platformconfig_test

import (
	"encoding/json"
	"strings"
	"testing"

	platformconfig "github.com/geoffmilleraz/agentic-kit/foundations/environment/platform-config"
)

// ── Config.Validate ───────────────────────────────────────────────────────────

func TestConfig_Validate_valid(t *testing.T) {
	cases := []struct {
		name string
		cfg  platformconfig.Config
	}{
		{
			"minimal",
			platformconfig.Config{Environment: "dev", Settings: map[string]string{}},
		},
		{
			"with settings",
			platformconfig.Config{
				Environment: "production",
				Settings:    map[string]string{"log_level": "warn", "region": "us-east-1"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestConfig_Validate_invalid(t *testing.T) {
	cases := []struct {
		name    string
		cfg     platformconfig.Config
		wantErr string
	}{
		{
			"empty Environment",
			platformconfig.Config{Environment: "", Settings: map[string]string{}},
			"Environment must be non-empty",
		},
		{
			"whitespace Environment",
			platformconfig.Config{Environment: "   ", Settings: map[string]string{}},
			"Environment must be non-empty",
		},
		{
			"nil Settings",
			platformconfig.Config{Environment: "dev", Settings: nil},
			"Settings must be non-nil",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── Load ──────────────────────────────────────────────────────────────────────

func TestLoad_valid(t *testing.T) {
	input := platformconfig.Config{
		Environment: "staging",
		Settings:    map[string]string{"feature_x": "on"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := platformconfig.Load(data)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if got.Environment != input.Environment {
		t.Errorf("Environment: got %q, want %q", got.Environment, input.Environment)
	}
	if len(got.Settings) != len(input.Settings) {
		t.Errorf("Settings len: got %d, want %d", len(got.Settings), len(input.Settings))
	}
	if got.Settings["feature_x"] != "on" {
		t.Errorf("Settings[feature_x]: got %q, want %q", got.Settings["feature_x"], "on")
	}
}

func TestLoad_emptyInput(t *testing.T) {
	_, err := platformconfig.Load([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestLoad_invalidJSON(t *testing.T) {
	_, err := platformconfig.Load([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "JSON decode") {
		t.Errorf("error %q does not mention JSON decode", err.Error())
	}
}

func TestLoad_failsValidation(t *testing.T) {
	// Valid JSON but fails Config.Validate (missing Environment).
	data := []byte(`{"Environment":"","Settings":{}}`)
	_, err := platformconfig.Load(data)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Environment") {
		t.Errorf("error %q does not mention Environment", err.Error())
	}
}

func TestLoad_nilSettingsJSON(t *testing.T) {
	// Settings field explicitly set to JSON null.
	data := []byte(`{"Environment":"dev","Settings":null}`)
	_, err := platformconfig.Load(data)
	if err == nil {
		t.Fatal("expected validation error for null Settings")
	}
	if !strings.Contains(err.Error(), "Settings") {
		t.Errorf("error %q does not mention Settings", err.Error())
	}
}

func TestLoad_JSONRoundTrip(t *testing.T) {
	original := platformconfig.Config{
		Environment: "production",
		Settings:    map[string]string{"a": "1", "b": "2"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	restored, err := platformconfig.Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if restored.Environment != original.Environment {
		t.Errorf("Environment mismatch: got %q, want %q", restored.Environment, original.Environment)
	}
	for k, v := range original.Settings {
		if restored.Settings[k] != v {
			t.Errorf("Settings[%q]: got %q, want %q", k, restored.Settings[k], v)
		}
	}
}

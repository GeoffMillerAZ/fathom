package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The CUE executable on PATH is the same one used by the cue-vet gate.
func TestControlSchema(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("cue executable unavailable")
	}
	advisory := map[string]any{
		"id": "review", "event": "before-tool", "class": "advisory",
		"minSamplePPM": 100, "maxSamplePPM": 900, "defaultSamplePPM": 500,
		"maxWorkMillis": 2000, "onFault": "warn", "allowMaintenance": true,
		"tunableBy": []string{"installation"},
	}
	mandatoryControl := map[string]any{
		"id": "guard", "event": "before-tool", "class": "mandatory",
		"minSamplePPM": 1000000, "maxSamplePPM": 1000000, "defaultSamplePPM": 1000000,
		"maxWorkMillis": 2000, "onFault": "hold", "allowMaintenance": false,
		"tunableBy": []string{},
	}
	cases := []struct {
		name  string
		base  map[string]any
		field string
		value any
		valid bool
	}{
		{"advisory", advisory, "", nil, true},
		{"mandatory", mandatoryControl, "", nil, true},
		{"mandatory minimum", mandatoryControl, "minSamplePPM", 999999, false},
		{"mandatory maximum", mandatoryControl, "maxSamplePPM", 999999, false},
		{"mandatory default", mandatoryControl, "defaultSamplePPM", 999999, false},
		{"mandatory fault", mandatoryControl, "onFault", "warn", false},
		{"mandatory maintenance", mandatoryControl, "allowMaintenance", true, false},
		{"mandatory tuner", mandatoryControl, "tunableBy", []string{"installation"}, false},
		{"advisory minimum out of range", advisory, "minSamplePPM", -1, false},
		{"advisory maximum out of range", advisory, "maxSamplePPM", 1000001, false},
		{"advisory maximum below minimum", advisory, "maxSamplePPM", 99, false},
		{"advisory default below minimum", advisory, "defaultSamplePPM", 99, false},
		{"advisory default above maximum", advisory, "defaultSamplePPM", 901, false},
		{"unknown field", advisory, "unknown", true, false},
		{"unknown class", advisory, "class", "other", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := make(map[string]any, len(tc.base)+1)
			for k, v := range tc.base {
				value[k] = v
			}
			if tc.field != "" {
				value[tc.field] = tc.value
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var parsed control
			goValid := strictJSON(data, &parsed) == nil && validRoot(parsed) &&
				(parsed.Class != "mandatory" || mandatory(parsed.MinSamplePPM, parsed.MaxSamplePPM, parsed.DefaultSamplePPM, parsed.OnFault, parsed.AllowMaintenance, parsed.TunableBy))
			if goValid != tc.valid {
				t.Fatalf("Go accepted=%t, want %t", goValid, tc.valid)
			}
			path := filepath.Join(t.TempDir(), "control.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(cue, "vet", "-c", "schema.cue", path, "-d", "#Control").CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("cue vet valid=%t: %v\n%s", tc.valid, err, strings.TrimSpace(string(out)))
			}
		})
	}
}

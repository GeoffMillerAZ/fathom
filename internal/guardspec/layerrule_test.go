package guardspec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLayerRules_TopLevelFieldAbsent_IsEmptyNotError(t *testing.T) {
	path := writeGuards(t, `guards: []`)
	set, err := LoadLayerRules(path)
	if err != nil {
		t.Fatalf("LoadLayerRules: %v", err)
	}
	if len(set.Rules) != 0 {
		t.Errorf("Rules = %v, want none", set.Rules)
	}
}

func TestLoadLayerRules_ExplicitEmpty(t *testing.T) {
	path := writeGuards(t, `
guards: []
layer_rules: []
`)
	set, err := LoadLayerRules(path)
	if err != nil {
		t.Fatalf("LoadLayerRules: %v", err)
	}
	if len(set.Rules) != 0 {
		t.Errorf("Rules = %v, want none", set.Rules)
	}
}

func TestLoadLayerRules_OnePerKind(t *testing.T) {
	path := writeGuards(t, `
guards: []
layer_rules: [
	{id: "a", kind: "import_direction", sentence: "s1", from: ["store"], to: ["extractor"]},
	{id: "b", kind: "no_cycles", sentence: "s2", scope: ["internal/**"]},
	{id: "c", kind: "boundary", sentence: "s3", allow: ["payment"], guarded: ["ledger"]},
	{id: "d", kind: "cross_service_direction", sentence: "s4", severity: "warn", from_service: "payment-service", to_service: "ledger-service"},
]
`)
	set, err := LoadLayerRules(path)
	if err != nil {
		t.Fatalf("LoadLayerRules: %v", err)
	}
	if len(set.Rules) != 4 {
		t.Fatalf("Rules = %d, want 4", len(set.Rules))
	}
	if set.Rules[0].Severity != SeverityError {
		t.Errorf("default severity = %q, want error", set.Rules[0].Severity)
	}
	if set.Rules[3].Severity != SeverityWarn {
		t.Errorf("explicit severity = %q, want warn", set.Rules[3].Severity)
	}
	if set.Rules[1].Kind != NoCycles || len(set.Rules[1].Scope) != 1 {
		t.Errorf("rule b = %+v", set.Rules[1])
	}
	if set.Rules[2].Kind != Boundary || set.Rules[2].Allow[0] != "payment" || set.Rules[2].Guarded[0] != "ledger" {
		t.Errorf("rule c = %+v", set.Rules[2])
	}
	if set.Rules[3].FromService != "payment-service" || set.Rules[3].ToService != "ledger-service" {
		t.Errorf("rule d = %+v", set.Rules[3])
	}
}

func TestLoadLayerRules_Negatives(t *testing.T) {
	cases := []struct {
		name       string
		cue        string
		wantSubstr string
	}{
		{"missing id", `layer_rules: [{kind: "no_cycles", sentence: "s", scope: ["internal/**"]}]`, "id"},
		{"missing sentence", `layer_rules: [{id: "a", kind: "no_cycles", scope: ["internal/**"]}]`, "sentence"},
		{"duplicate id", `layer_rules: [
			{id: "a", kind: "no_cycles", sentence: "s1", scope: ["internal/**"]},
			{id: "a", kind: "no_cycles", sentence: "s2", scope: ["cmd/**"]},
		]`, "duplicate"},
		{"import_direction missing from/to", `layer_rules: [{id: "a", kind: "import_direction", sentence: "s"}]`, "from and to"},
		{"no_cycles missing scope", `layer_rules: [{id: "a", kind: "no_cycles", sentence: "s"}]`, "scope"},
		{"boundary missing allow/guarded", `layer_rules: [{id: "a", kind: "boundary", sentence: "s"}]`, "allow and guarded"},
		{"cross_service_direction missing services", `layer_rules: [{id: "a", kind: "cross_service_direction", sentence: "s"}]`, "from_service and to_service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGuards(t, "guards: []\n"+tc.cue)
			_, err := LoadLayerRules(path)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestLoadLayerRules_MissingFile(t *testing.T) {
	if _, err := LoadLayerRules(filepath.Join(t.TempDir(), "nope.cue")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

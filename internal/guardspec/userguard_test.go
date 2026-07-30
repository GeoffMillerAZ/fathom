package guardspec

import "testing"

func TestLoadUserGuards_Valid(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		note: "confirm before an apply"
		command_scopes: ["terraform apply*", "kubectl apply*"]
	},
]`)
	gs, err := LoadUserGuards(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Guards) != 1 {
		t.Fatalf("want 1 guard, got %d", len(gs.Guards))
	}
	g := gs.Guards[0]
	if g.Note != "[user] confirm before an apply" {
		t.Errorf("Note = %q, want tagged with [user]", g.Note)
	}
	if len(g.CommandScopes) != 2 {
		t.Errorf("CommandScopes = %v", g.CommandScopes)
	}
	if len(g.Scopes) != 0 {
		t.Errorf("Scopes must never be set from a user-scope file, got %v", g.Scopes)
	}
}

func TestLoadUserGuards_NoteOnlyStillTagged(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		note: "double-check this"
		command_scopes: ["rm -rf*"]
	},
]`)
	gs, err := LoadUserGuards(path)
	if err != nil {
		t.Fatal(err)
	}
	if gs.Guards[0].Note != "[user] double-check this" {
		t.Errorf("Note = %q", gs.Guards[0].Note)
	}
}

func TestLoadUserGuards_ScopesForbidden(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		scopes: ["internal/store/**"]
		note: "nope"
	},
]`)
	_, err := LoadUserGuards(path)
	assertLoadUserGuardsForbidsField(t, err, "scopes")
}

func TestLoadUserGuards_MaxRulesForbidden(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		command_scopes: ["terraform apply*"]
		max_rules: 2
	},
]`)
	_, err := LoadUserGuards(path)
	assertLoadUserGuardsForbidsField(t, err, "max_rules")
}

func TestLoadUserGuards_IncludeModulesForbidden(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		command_scopes: ["terraform apply*"]
		include_modules: true
	},
]`)
	_, err := LoadUserGuards(path)
	assertLoadUserGuardsForbidsField(t, err, "include_modules")
}

func TestLoadUserGuards_MinTierForbidden(t *testing.T) {
	// min_tier is not a real #Guard field (guard_schema.cue has none), but
	// LoadUserGuards checks for it defensively — LoadGuards' lenient
	// reader would otherwise silently ignore an unknown field, exactly
	// the silent-widening risk law 13 exists to prevent.
	path := writeGuards(t, `guards: [
	{
		command_scopes: ["terraform apply*"]
		min_tier: "high"
	},
]`)
	_, err := LoadUserGuards(path)
	assertLoadUserGuardsForbidsField(t, err, "min_tier")
}

func TestLoadUserGuards_NoCommandScopes_Rejected(t *testing.T) {
	path := writeGuards(t, `guards: [
	{
		note: "governs nothing"
	},
]`)
	if _, err := LoadUserGuards(path); err == nil {
		t.Fatal("expected an error — a user-scope guard with no command_scopes governs nothing")
	}
}

func TestLoadUserGuards_MissingFile(t *testing.T) {
	if _, err := LoadUserGuards("/nonexistent/guard.cue"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func assertLoadUserGuardsForbidsField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error naming forbidden field %q", field)
	}
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("err = %v (%T), want *LoadError", err, err)
	}
	for _, fe := range le.Fields {
		if fe.Field == field || (len(fe.Field) > len(field) && fe.Field[len(fe.Field)-len(field):] == field) {
			return
		}
	}
	t.Fatalf("LoadError fields = %v, want one naming %q", le.Fields, field)
}

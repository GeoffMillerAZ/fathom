package capability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegistryEveryEntryExplainsItsAbsence pins CR-1, and it is the reason
// this package exists. Against a ZERO probe — a repository fathom has
// never touched — every capability must report off WITH a reason and
// either a command or an explicit note. A capability that cannot say why
// it is off cannot be registered, and this test is what enforces that on
// the next person to add one.
func TestRegistryEveryEntryExplainsItsAbsence(t *testing.T) {
	r := Default()
	if len(r.IDs()) == 0 {
		t.Fatal("the registry is empty")
	}
	for _, c := range r.All() {
		t.Run(c.ID, func(t *testing.T) {
			if c.Summary == "" || c.Cost == "" || c.Seam == "" {
				t.Errorf("%q is missing summary, cost or seam: %+v", c.ID, c)
			}
			if c.Detect == nil || c.Remedy == nil {
				t.Fatalf("%q must have both Detect and Remedy", c.ID)
			}
			st := c.Detect(Probe{})
			if st.Operating {
				t.Fatalf("%q claims to be operating against a zero probe — detection is reading configuration, not observation", c.ID)
			}
			if strings.TrimSpace(st.Reason) == "" {
				t.Errorf("%q is off with no reason", c.ID)
			}
			rem := c.Remedy(Probe{})
			if rem.Cmd == "" && rem.Note == "" {
				t.Errorf("%q is off with neither a command nor an explanation", c.ID)
			}
		})
	}
}

// TestDetectNeverReturnsBareBoolean pins CR-2 from the other side: an
// operating capability must carry evidence a reader can check, so a green
// tick is never fathom's unsupported word.
func TestDetectNeverReturnsBareBoolean(t *testing.T) {
	full := Probe{
		Service: "svc", IndexedSymbols: 120, IndexFresh: true,
		HooksReferenced: true, StanzaPresent: true, VerifyInLefthook: true,
		GuardRules: 2, ModuleBindings: 1, TechPacks: 1, LayerPacks: []string{"ui-appkit"},
		ObligationRules: 6, ObligationsCompiled: true, LedgerSessions: 4,
		AgentSurfaceTools: 17, AgentSchemaBytes: 11839,
		IntentRouterPresent: true, IntentDecisionsHome: "ADR.md", IntentConditions: 67,
		AssessRuns: 3,
	}
	for _, c := range Default().All() {
		st := c.Detect(full)
		if !st.Operating {
			t.Errorf("%q reports off against a fully-provisioned probe: %s", c.ID, st.Reason)
			continue
		}
		if strings.TrimSpace(st.Detail) == "" {
			t.Errorf("%q reports operating with no evidence", c.ID)
		}
	}
}

func TestGuardInputProblemIsBrokenNotAbsent(t *testing.T) {
	guards, ok := Default().Get("guards")
	if !ok {
		t.Fatal("guards capability is not registered")
	}
	const problem = "guard.cue is invalid: dead guard"
	state := guards.Detect(Probe{GuardProblem: problem})
	if state.Operating || !state.Broken || state.Reason != problem {
		t.Fatalf("guard problem state = %+v, want broken with the original reason", state)
	}
	remedy := guards.Remedy(Probe{GuardProblem: problem})
	if strings.Contains(remedy.Cmd, "init --guards") {
		t.Fatalf("guard problem remedy may not overwrite the input: %+v", remedy)
	}
}

// TestResolveProvenanceAndRefusals pins CR-3: resolution order, the scope
// that supplied the value travelling with it, and law 13's asymmetry
// enforced loudly rather than by silent drop.
func TestResolveProvenanceAndRefusals(t *testing.T) {
	r := Default()

	t.Run("built-in wins when nobody speaks", func(t *testing.T) {
		c, _ := r.Get("index")
		got := Resolution{}.For(c)
		if !got.On || got.Scope != ScopeBuiltIn {
			t.Errorf("got %+v, want the built-in default", got)
		}
	})

	t.Run("user sets a default for repositories that have not spoken", func(t *testing.T) {
		c, _ := r.Get("assess")
		got := Resolution{User: Settings{"assess": true}}.For(c)
		if !got.On || got.Scope != ScopeUser {
			t.Errorf("got %+v, want on from user scope", got)
		}
	})

	t.Run("project governs over user", func(t *testing.T) {
		c, _ := r.Get("assess")
		got := Resolution{
			User:    Settings{"assess": true},
			Project: Settings{"assess": false},
		}.For(c)
		if got.On || got.Scope != ScopeProject {
			t.Errorf("got %+v, want off from project scope — project governs (law 13)", got)
		}
	})

	t.Run("user scope may not set a project-only capability", func(t *testing.T) {
		c, _ := r.Get("verify-gate")
		if c.Scopes != ScopeProjectOnly {
			t.Fatalf("verify-gate must be project-only; it is a gate, not a preference")
		}
		// Validation reports it...
		errs := r.ValidateSettings(Settings{"verify-gate": false}, ScopeUser)
		if len(errs) != 1 {
			t.Fatalf("want exactly one violation, got %v", errs)
		}
		var sv *ScopeViolationError
		if !errors.As(errs[0], &sv) || sv.ID != "verify-gate" {
			t.Errorf("the violation must NAME the key, not drop it: %v", errs[0])
		}
		// ...and resolution fails safe if a caller skipped validation.
		got := Resolution{User: Settings{"verify-gate": false}}.For(c)
		if !got.On || got.Scope != ScopeBuiltIn {
			t.Errorf("got %+v, want the built-in default — a scope may not govern beyond its entitlement", got)
		}
	})

	t.Run("an unknown id is refused with a suggestion", func(t *testing.T) {
		errs := r.ValidateSettings(Settings{"oblgations": false}, ScopeProject)
		if len(errs) != 1 {
			t.Fatalf("want one error, got %v", errs)
		}
		var ue *UnknownCapabilityError
		if !errors.As(errs[0], &ue) {
			t.Fatalf("want UnknownCapabilityError, got %T", errs[0])
		}
		if ue.Suggest != "obligations" {
			t.Errorf("suggestion = %q, want %q — a typo that silently disables a safety feature is the worst failure here", ue.Suggest, "obligations")
		}
	})

	t.Run("every violation is reported, not just the first", func(t *testing.T) {
		errs := r.ValidateSettings(Settings{"nope": true, "alsonope": true, "verify-gate": false}, ScopeUser)
		if len(errs) != 3 {
			t.Errorf("want 3 violations, got %d: %v", len(errs), errs)
		}
	})
}

// TestLoaderRefusesOptionalKnownField pins CR-4 — the phase-O
// trust-boundary inversion, caught where it does damage. `on?: false` in a
// config file reads as "switch this off" and does nothing at all.
func TestLoaderRefusesOptionalKnownField(t *testing.T) {
	r := Default()
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "capabilities.cue")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("optional field is refused with the fix in the message", func(t *testing.T) {
		p := write("capabilities: {\n\tassess: on?: false\n}\n")
		_, err := r.LoadSettings(p, ScopeProject)
		if err == nil {
			t.Fatal("an optional-field declaration must be refused, not silently ignored")
		}
		var oe *OptionalFieldError
		if !errors.As(err, &oe) {
			t.Fatalf("want OptionalFieldError, got %v", err)
		}
		if !strings.Contains(err.Error(), "without the question mark") {
			t.Errorf("the message must say how to fix it: %v", err)
		}
	})

	t.Run("a concrete setting loads", func(t *testing.T) {
		p := write("capabilities: {\n\tassess: on: true\n}\n")
		got, err := r.LoadSettings(p, ScopeProject)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v, ok := got["assess"]; !ok || !v {
			t.Errorf("got %+v, want assess=true", got)
		}
	})

	t.Run("an entry that sets nothing is refused, not dropped", func(t *testing.T) {
		p := write("capabilities: {\n\tassess: {}\n}\n")
		if _, err := r.LoadSettings(p, ScopeProject); err == nil {
			t.Error("an entry its author believed did something must say that it does not")
		}
	})
}

// TestLoadThreeStates pins the O22 lesson for this file: absent, an
// all-commented scaffold, and malformed are three different outcomes.
func TestLoadThreeStates(t *testing.T) {
	r := Default()
	dir := t.TempDir()

	t.Run("absent file is an empty config, not an error", func(t *testing.T) {
		got, err := r.LoadSettings(filepath.Join(dir, "nope.cue"), ScopeProject)
		if err != nil || len(got) != 0 {
			t.Errorf("got (%v, %v), want (empty, nil)", got, err)
		}
	})

	t.Run("the ACTUAL init scaffold loads as empty", func(t *testing.T) {
		p := filepath.Join(dir, "scaffold.cue")
		// Load the real template bytes, not a hand-written approximation
		// of them — approximating the scaffold is how O22 shipped.
		if err := os.WriteFile(p, ExampleTemplate(r), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := r.LoadSettings(p, ScopeProject)
		if err != nil {
			t.Fatalf("the scaffold fathom itself writes must load cleanly: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %+v, want no settings from an all-commented file", got)
		}
		if err := ValidateSchema(p); err != nil {
			t.Errorf("the scaffold must also pass schema validation: %v", err)
		}
	})

	t.Run("malformed is loud", func(t *testing.T) {
		p := filepath.Join(dir, "bad.cue")
		if err := os.WriteFile(p, []byte("capabilities: { assess: on: \"yes\" }\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := r.LoadSettings(p, ScopeProject); err == nil {
			t.Error("a non-bool setting must fail loudly")
		}
	})
}

// TestDetectAllRespectsConfiguration pins the interaction: a capability
// switched off by configuration is never probed, so a reader is never told
// "no guard rules compiled" about something they deliberately disabled.
func TestDetectAllRespectsConfiguration(t *testing.T) {
	r := Default()
	reports := r.DetectAll(Probe{}, Resolution{Project: Settings{"guards": false}})
	var seen bool
	for _, rep := range reports {
		if rep.Capability.ID != "guards" {
			continue
		}
		seen = true
		if rep.Enabled {
			t.Error("guards must resolve to disabled")
		}
		if rep.Scope != ScopeProject {
			t.Errorf("scope = %q, want project", rep.Scope)
		}
		if !strings.Contains(rep.State.Reason, "switched off") {
			t.Errorf("reason = %q, want it to name the switch, not a missing rule file", rep.State.Reason)
		}
		if rep.Remedy.Cmd == "" {
			t.Error("a deliberately-off capability still says how to turn it back on")
		}
	}
	if !seen {
		t.Fatal("guards missing from DetectAll")
	}
}

// TestReportsAreSorted pins the house determinism contract.
func TestReportsAreSorted(t *testing.T) {
	reports := Default().DetectAll(Probe{}, Resolution{})
	for i := 1; i < len(reports); i++ {
		if reports[i-1].Capability.ID > reports[i].Capability.ID {
			t.Fatalf("reports are unsorted at %d: %q then %q",
				i, reports[i-1].Capability.ID, reports[i].Capability.ID)
		}
	}
}

// TestAssessIsOffByDefaultAndPriced pins the assess lane's registry entry
// (ADR-046): a priced lane is never on without a switch someone flipped,
// it says what it costs, and its remedy is the dry run that spends nothing.
func TestAssessIsOffByDefaultAndPriced(t *testing.T) {
	c, ok := Default().Get("assess")
	if !ok {
		t.Fatal("assess is not registered")
	}
	if c.Default {
		t.Error("assess defaults ON — a priced lane must never spend without a switch someone flipped")
	}
	if !c.Switchable {
		t.Error("assess must be a real switch: cmdAssess reads it and refuses when off")
	}
	if !strings.Contains(c.Cost, "token") {
		t.Errorf("cost %q must say it spends model tokens", c.Cost)
	}
	got := Resolution{}.For(c)
	if got.On {
		t.Error("no configuration resolved assess ON")
	}
	if rem := c.Remedy(Probe{}); !strings.Contains(rem.Cmd, "--dry-run") {
		t.Errorf("remedy %q should be the dry run, which spends nothing", rem.Cmd)
	}
}

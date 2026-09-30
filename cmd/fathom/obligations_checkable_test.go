package main

// obligations_checkable_test.go — SR-45w (docs/plans/surface-reckoning.md,
// F-26): the `obligations` answer marks each `.fathom/guard.cue` layer rule
// that governs the asked path checkable or not (guardspec.
// LayerRuleIsCheckable), and `fathom obligations promote <rule>` compiles a
// checkable one to an enforce:true guard through guardspec.
// GuardFromCheckableRule — a dry run by default, `--write` to apply, a
// not-checkable rule refused with its reason.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

const checkableGuardCue = `// a project guard file: two advisory layer rules and one no_cycles rule
guards: []

layer_rules: [
	{
		// payments stay off the store
		id:       "pay-no-store"
		kind:     "import_direction"
		sentence: "the payment package must not import internal/store."
		from: ["."]
		to: ["internal/store"]
	},
	{
		id:       "pay-no-cycles"
		kind:     "no_cycles"
		sentence: "no import cycles at the payment root."
		scope: ["."]
	},
	{
		id:       "other-dir-only"
		kind:     "import_direction"
		sentence: "internal/other must not import cmd/."
		from: ["internal/other"]
		to: ["cmd/**"]
	},
]
`

type checkableJSON struct {
	LayerRules struct {
		Source string `json:"source"`
		Note   string `json:"note"`
		Rules  []struct {
			ID        string `json:"id"`
			Kind      string `json:"kind"`
			Enforce   bool   `json:"enforce"`
			Checkable bool   `json:"checkable"`
			Why       string `json:"why"`
		} `json:"rules"`
	} `json:"layer_rules"`
}

func TestObligationsMarksCheckableRule(t *testing.T) {
	f := newConsoleFixture(t)
	root := t.TempDir()
	guard := filepath.Join(root, ".fathom", "guard.cue")
	if err := os.MkdirAll(filepath.Dir(guard), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, guard, checkableGuardCue)

	// --json: each layer rule governing payment.go is marked; a rule for
	// another directory is not served for this file.
	out, errb, code := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("obligations --json: exit %d: %s", code, errb)
	}
	var got checkableJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	byID := map[string]int{}
	for i, r := range got.LayerRules.Rules {
		byID[r.ID] = i
	}
	if len(got.LayerRules.Rules) != 2 {
		t.Fatalf("layer_rules = %+v, want the two rules governing payment.go (not other-dir-only)", got.LayerRules)
	}
	store, ok := byID["pay-no-store"]
	if !ok || !got.LayerRules.Rules[store].Checkable || got.LayerRules.Rules[store].Enforce {
		t.Errorf("pay-no-store = %+v, want checkable, not yet enforced", got.LayerRules.Rules)
	}
	if !strings.Contains(got.LayerRules.Rules[store].Why, "fathom obligations promote pay-no-store") {
		t.Errorf("a checkable advisory rule names its promote command: %q", got.LayerRules.Rules[store].Why)
	}
	cyc, ok := byID["pay-no-cycles"]
	if !ok || got.LayerRules.Rules[cyc].Checkable || got.LayerRules.Rules[cyc].Why == "" {
		t.Errorf("pay-no-cycles = %+v, want not checkable with its reason", got.LayerRules.Rules)
	}
	if got.LayerRules.Source != guard {
		t.Errorf("layer_rules.source = %q, want %q", got.LayerRules.Source, guard)
	}

	// Text: the same marks.
	text, _, _ := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--root", root)
	for _, want := range []string{"pay-no-store", "checkable", "pay-no-cycles", "not checkable"} {
		if !strings.Contains(text, want) {
			t.Errorf("text answer lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "other-dir-only") {
		t.Errorf("text answer serves a layer rule that does not govern payment.go:\n%s", text)
	}

	before, _ := os.ReadFile(guard)

	// promote, dry run: prints the enforce:true stanza, writes nothing.
	var o, e bytes.Buffer
	if code := run([]string{"obligations", "promote", "pay-no-store", "--root", root}, &o, &e); code != 0 {
		t.Fatalf("promote (dry run): exit %d: %s", code, e.String())
	}
	if !strings.Contains(o.String(), "enforce:  true") || !strings.Contains(o.String(), `id:       "pay-no-store"`) {
		t.Errorf("dry run does not print the promoted stanza:\n%s", o.String())
	}
	if after, _ := os.ReadFile(guard); !bytes.Equal(after, before) {
		t.Fatalf("a dry run changed %s", guard)
	}

	// promote --write: the guard file now enforces the rule, every other
	// byte (comments, the other rules) unchanged but the one new line.
	o.Reset()
	e.Reset()
	if code := run([]string{"obligations", "promote", "pay-no-store", "--root", root, "--write"}, &o, &e); code != 0 {
		t.Fatalf("promote --write: exit %d: %s", code, e.String())
	}
	rules, err := guardspec.LoadLayerRules(guard)
	if err != nil {
		t.Fatalf("the promoted guard file does not load: %v", err)
	}
	for _, r := range rules.Rules {
		if want := r.ID == "pay-no-store"; r.Enforce != want {
			t.Errorf("after promote %s.Enforce = %v, want %v", r.ID, r.Enforce, want)
		}
	}
	after, _ := os.ReadFile(guard)
	if !strings.Contains(string(after), "// payments stay off the store") || !strings.Contains(string(after), "// a project guard file") {
		t.Errorf("promote --write lost a comment:\n%s", after)
	}
	if got, want := strings.Count(string(after), "\n"), strings.Count(string(before), "\n")+1; got != want {
		t.Errorf("promote --write changed %d lines, want exactly one added:\n%s", got-want+1, after)
	}

	// The answer now reads the rule as enforced.
	out, _, _ = runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--root", root, "--json")
	got = checkableJSON{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, r := range got.LayerRules.Rules {
		if r.ID == "pay-no-store" && !r.Enforce {
			t.Errorf("after promote the answer reads pay-no-store unenforced: %+v", r)
		}
	}

	// Promoting it again is a no-op, not an error.
	o.Reset()
	e.Reset()
	if code := run([]string{"obligations", "promote", "pay-no-store", "--root", root, "--write"}, &o, &e); code != 0 || !strings.Contains(o.String(), "already") {
		t.Errorf("re-promote: exit %d, out %q, err %q; want a named no-op", code, o.String(), e.String())
	}

	// A not-checkable rule is refused with its reason, and nothing is written.
	o.Reset()
	e.Reset()
	snapshot, _ := os.ReadFile(guard)
	if code := run([]string{"obligations", "promote", "pay-no-cycles", "--root", root, "--write"}, &o, &e); code == 0 {
		t.Fatalf("promote of a no_cycles rule exited 0: %s", o.String())
	}
	if !strings.Contains(e.String(), "not checkable") || !strings.Contains(e.String(), "no_cycles") {
		t.Errorf("refusal does not name its reason: %q", e.String())
	}
	if now, _ := os.ReadFile(guard); !bytes.Equal(now, snapshot) {
		t.Errorf("a refused promote changed %s", guard)
	}

	// An unknown rule is refused by name.
	o.Reset()
	e.Reset()
	if code := run([]string{"obligations", "promote", "no-such-rule", "--root", root}, &o, &e); code == 0 || !strings.Contains(e.String(), "no-such-rule") {
		t.Errorf("unknown rule: exit %d, err %q", code, e.String())
	}
}

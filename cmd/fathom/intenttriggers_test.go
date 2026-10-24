package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIntentTriggersUnevaluableBucket(t *testing.T) {
	var out bytes.Buffer
	renderIntentTriggers(&out, []intent.TriggerVerdict{
		{Record: "ADR-001", Label: "Revisit when", State: intent.TriggerHolding, Instrument: "plans_failing", Threshold: "> 0"},
		{Record: "ADR-002", Label: "Revisit when", State: intent.TriggerUnevaluated},
		{Record: "ADR-003", Label: "Revisit when", State: intent.TriggerUnevaluable, Instrument: "plans_failing", Threshold: "> 0", Reason: "cross-repo refused"},
		{Record: "G1", Label: "Done when", State: intent.TriggerUnevaluable, Reason: "escapes the repository root"},
	})
	for _, want := range []string{"2 revisit/promote condition(s)", "1 not fired", "1 unevaluated (1 prose)", "2 unevaluable"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q:\n%s", want, out.String())
		}
	}
}

// TestCmdIntentAuditEvaluatesComputableTriggers (CX-21): `fathom intent
// audit` reads a computable trigger's instrument when the operator runs it
// — here the ingest-time violation trend in --store — and prints the
// verdict with its reading; a prose trigger is listed unevaluated, and a
// trigger over an instrument the store holds no reading for is unevaluated
// with the reason, never "not fired".
func TestCmdIntentAuditEvaluatesComputableTriggers(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	adrDoc := "# Decisions\n\n## ADR-001 — Layers\n\n**Revisit when.** the layers erode {instrument: arch_violations, threshold: \">= 3\"}\n\n" +
		"## ADR-002 — One rule\n\n**Revisit when.** a -> b holds {instrument: \"arch_violations:a->b\", threshold: \"> 5\"}\n\n" +
		"## ADR-003 — Prose\n\n**Revisit when.** the thing stops working.\n"
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(adrDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "trend.sqlite")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendArchViolations(context.Background(), "svc", "abc123", 1, []store.ArchRuleCount{{Rule: "a -> b", Count: 2}, {Rule: "c -> d", Count: 2}}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{"audit", "--read-only", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc", "--store", dbPath, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var doc struct {
		Triggers []intent.TriggerVerdict `json:"triggers"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	by := map[string]intent.TriggerVerdict{}
	for _, v := range doc.Triggers {
		by[v.Record] = v
	}
	if v := by["ADR-001"]; v.State != intent.TriggerFired || v.Value != 4 || !strings.Contains(v.Reading, "abc123") {
		t.Errorf("ADR-001 = %+v, want fired at 4 from the ingest of abc123", v)
	}
	if v := by["ADR-002"]; v.State != intent.TriggerHolding || v.Value != 2 {
		t.Errorf("ADR-002 = %+v, want not fired at 2 (a -> b)", v)
	}
	if v := by["ADR-003"]; v.State != intent.TriggerUnevaluated || v.Instrument != "" {
		t.Errorf("ADR-003 = %+v, want an unevaluated prose trigger", v)
	}

	// The text report names the fired decision and its reading.
	out.Reset()
	if code := cmdIntent([]string{"audit", "--read-only", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc", "--store", dbPath, "--kind", "triggers"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	for _, want := range []string{"1 fired", "FIRED ADR-001", "arch_violations >= 3", "4 violation(s) over 2 rule(s)", "prose trigger(s) stay human-judged"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, out.String())
		}
	}

	// A service with no reading: unevaluated with the reason, never 0.
	out.Reset()
	if code := cmdIntent([]string{"audit", "--read-only", "--root", root, "--url", "http://127.0.0.1:1", "--service", "other", "--store", dbPath, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	doc.Triggers = nil
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, v := range doc.Triggers {
		if v.Record == "ADR-001" && (v.State != intent.TriggerUnevaluated || !strings.Contains(v.Reason, "no violation reading")) {
			t.Errorf("no reading: ADR-001 = %+v, want unevaluated naming the missing reading", v)
		}
	}
}

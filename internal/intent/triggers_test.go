package intent_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

func triggersFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n| Decisions | `../../ADR.md` |\n",
		"ADR.md": "# Decisions\n\n" +
			"## ADR-001 — Layers\n\n**Revisit when.** the layers erode {instrument: arch_violations, threshold: \">= 3\"}\n\n" +
			"## ADR-002 — Store\n\n**Revisit when.** the store outgrows one file.\n\n" +
			"## ADR-003 — Gate\n\n**Revisit when.** the gate fails often {instrument: plans_failing, threshold: \"> 5\"}\n\n" +
			"## ADR-004 — Odd\n\n**Revisit when.** a thing moves {instrument: no_such_instrument, threshold: \"> 1\"}\n\n" +
			"## ADR-005 — Words\n\n**Revisit when.** it feels slow {instrument: arch_violations, threshold: \"a lot\"}\n\n" +
			"## ADR-006 — Broken reader\n\n**Revisit when.** the reader breaks {instrument: flaky, threshold: \"== 0\"}\n",
		"docs/intent/futures.md": "# Futures\n\n## F1. A future.\n\n" +
			"Promote when: violations reach zero {instrument: \"arch_violations:a -> b\", threshold: \"== 0\"}\n\n" +
			"## F2. Another.\n\nPromote when: someone asks in writing.\n",
	})
	return root
}

// TestComputableTriggersEvaluatedOnDemand (CX-21, design §5 step 3, F9's
// computable subset): a Revisit when / Promote when condition carrying a
// bet-block-shaped {instrument, threshold} is evaluated only when the
// operator runs the audit — loading the corpus reads no instrument, and
// RunAudit reads each named instrument exactly then. A prose trigger is
// listed as unevaluated, never read and never "fired"; an unknown
// instrument, a threshold that is not a comparison, or a reader error is
// unevaluated with its reason, never a verdict either way.
func TestComputableTriggersEvaluatedOnDemand(t *testing.T) {
	root := triggersFixture(t)
	var reads []string
	reader := func(instrument string) (intent.InstrumentReading, bool, error) {
		reads = append(reads, instrument)
		switch instrument {
		case "arch_violations":
			return intent.InstrumentReading{Value: 4, Reading: "4 violations at ingest c3"}, true, nil
		case "arch_violations:a -> b":
			return intent.InstrumentReading{Value: 2, Reading: "2 violations of a -> b at ingest c3"}, true, nil
		case "plans_failing":
			return intent.InstrumentReading{Value: 1, Reading: "1 failing row"}, true, nil
		case "flaky":
			return intent.InstrumentReading{}, true, errors.New("store unreadable")
		}
		return intent.InstrumentReading{}, false, nil
	}

	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reads) != 0 {
		t.Fatalf("loading the corpus read instruments: %v", reads)
	}
	for _, cond := range c.Conditions {
		if cond.Annotated() {
			t.Errorf("a trigger block counted as an oracle annotation (K0 inflated): %+v", cond)
		}
	}
	if m := intent.MisplacedBets(c); len(m) != 0 {
		t.Errorf("a computable trigger is read, so it is not a misplaced bet: %+v", m)
	}

	a := intent.RunAudit(intent.AuditInput{Corpus: c, Instruments: reader, Now: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)})
	if len(reads) != 5 {
		t.Errorf("RunAudit read %d instruments (%v), want each computable trigger's once (5)", len(reads), reads)
	}
	got := map[string]intent.TriggerVerdict{}
	for _, v := range a.Triggers {
		got[v.Record] = v
	}
	want := map[string]struct{ state, reason string }{
		"ADR-001": {intent.TriggerFired, ""},
		"ADR-002": {intent.TriggerUnevaluated, "prose"},
		"ADR-003": {intent.TriggerHolding, ""},
		"ADR-004": {intent.TriggerUnevaluated, "not an instrument"},
		"ADR-005": {intent.TriggerUnevaluated, "not a comparison"},
		"ADR-006": {intent.TriggerUnevaluated, "store unreadable"},
		"F1":      {intent.TriggerHolding, ""},
		"F2":      {intent.TriggerUnevaluated, "prose"},
	}
	if len(a.Triggers) != len(want) {
		t.Fatalf("triggers = %+v, want %d", a.Triggers, len(want))
	}
	for rec, w := range want {
		v, ok := got[rec]
		if !ok {
			t.Errorf("%s: no verdict", rec)
			continue
		}
		if v.State != w.state || (w.reason != "" && !strings.Contains(v.Reason, w.reason)) {
			t.Errorf("%s: %s (%q), want %s (%q)", rec, v.State, v.Reason, w.state, w.reason)
		}
		if v.File == "" || v.Line == 0 || v.Label == "" {
			t.Errorf("%s: verdict carries no citation: %+v", rec, v)
		}
		if v.State != intent.TriggerUnevaluated && v.Reading == "" {
			t.Errorf("%s: an evaluated trigger must carry its reading", rec)
		}
	}
	if f := got["ADR-001"]; f.Value != 4 || f.Instrument != "arch_violations" || f.Threshold != ">= 3" {
		t.Errorf("fired verdict = %+v", f)
	}
	// Deterministic: sorted by file then line.
	for i := 1; i < len(a.Triggers); i++ {
		p, q := a.Triggers[i-1], a.Triggers[i]
		if p.File > q.File || (p.File == q.File && p.Line > q.Line) {
			t.Fatalf("triggers not sorted: %+v before %+v", p, q)
		}
	}

	// No reader: the computable triggers are unevaluated, and the audit says
	// why — never read, never guessed.
	reads = nil
	a2 := intent.RunAudit(intent.AuditInput{Corpus: c, Now: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)})
	for _, v := range a2.Triggers {
		if v.State != intent.TriggerUnevaluated {
			t.Errorf("no reader: %s evaluated to %s", v.Record, v.State)
		}
	}
	if !strings.Contains(strings.Join(a2.Uncomputable, "\n"), "computable triggers") {
		t.Errorf("no reader must be named uncomputable: %v", a2.Uncomputable)
	}
}

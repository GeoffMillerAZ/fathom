package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestConsoleDriftRadar (CX-22, design §5 "Output"): the Intent tab's drift
// radar lists the decisions due for review (computable triggers that fired
// at this audit, with their reading and citation) and the architecture
// eroding here (deny rules whose ingest-time violation count rose, with the
// commit ranges that added violations). No computable trigger, no reading,
// or an unreadable trend is an absence or an error with its reason, never 0.
func TestConsoleDriftRadar(t *testing.T) {
	triggers := []intent.TriggerVerdict{
		{ID: "ADR-001#1", Record: "ADR-001", Label: "Revisit when", Text: "the layers erode", File: "ADR.md", Line: 5, Instrument: "arch_violations", Threshold: ">= 3", State: intent.TriggerFired, Value: 4, Reading: "4 violation(s) over 2 rule(s) at the ingest of c3"},
		{ID: "ADR-002#1", Record: "ADR-002", Label: "Revisit when", Text: "the store outgrows one file", File: "ADR.md", Line: 9, State: intent.TriggerUnevaluated, Reason: "prose trigger — human-judged"},
		{ID: "ADR-003#1", Record: "ADR-003", Label: "Revisit when", File: "ADR.md", Line: 13, Instrument: "plans_failing", Threshold: "> 5", State: intent.TriggerHolding, Value: 1, Reading: "1 failing"},
		{ID: "F1#1", Record: "F1", Label: "Promote when", File: "docs/intent/futures.md", Line: 4, Instrument: "arch_violations:x -> y", Threshold: "== 0", State: intent.TriggerUnevaluated, Reason: `rule "x -> y" is not in the newest reading`},
	}
	trend := []store.ArchViolationReading{
		{Seq: 1, Commit: "c1", Rule: "a -> b", Count: 2},
		{Seq: 1, Commit: "c1", Rule: "c -> d", Count: 2},
		{Seq: 2, Commit: "c2", PrevCommit: "c1", Rule: "a -> b", Count: 2},
		{Seq: 2, Commit: "c2", PrevCommit: "c1", Rule: "c -> d", Count: 1, Delta: -1},
		{Seq: 3, Commit: "c3", PrevCommit: "c2", Rule: "a -> b", Count: 3, Delta: 1},
		{Seq: 3, Commit: "c3", PrevCommit: "c2", Rule: "c -> d", Count: 1},
	}
	m := buildDriftRadar(triggers, trend, nil)
	if m.Due.State != console.StatePresent || m.Due.V != 1 || len(m.Decisions) != 1 || m.Decisions[0].Record != "ADR-001" {
		t.Fatalf("due = %+v, decisions %+v", m.Due, m.Decisions)
	}
	if m.Prose != 1 || len(m.Unevaluated) != 1 {
		t.Errorf("prose %d, unevaluated %+v", m.Prose, m.Unevaluated)
	}
	if m.Eroding.State != console.StatePresent || m.Eroding.V != 1 || m.Readings != 3 {
		t.Fatalf("eroding = %+v over %d readings", m.Eroding, m.Readings)
	}
	if r := m.Rules[0]; r.Rule != "a -> b" || !r.Rising || r.Latest != 3 || len(r.Added) != 1 || r.Added[0] != (driftRadarAdded{From: "c2", To: "c3", Delta: 1}) {
		t.Errorf("eroding rule first = %+v", r)
	}
	if r := m.Rules[1]; r.Rule != "c -> d" || r.Rising || fmt.Sprint(r.Counts) != "[2 1 1]" {
		t.Errorf("holding rule = %+v", r)
	}
	page := driftRadarPanel("svc", "fathom intent audit --read-only --json", "audited now", m)
	for _, want := range []string{
		`id="intent-radar"`, "Decisions due for review", "Architecture eroding here", "never by the daemon on its own",
		`data-radar-due`, "ADR-001", "arch_violations &gt;= 3", "4 violation(s) over 2 rule(s)", "ADR.md:5",
		`data-radar-eroding`, "a -&gt; b", "eroding", "+1 in c2..c3", "<svg", "unevaluated — F1",
		"1 prose trigger(s) stay human-judged",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("radar lacks %q", want)
		}
	}

	// Nothing to read: absences with reasons, never a 0.
	empty := buildDriftRadar([]intent.TriggerVerdict{triggers[1]}, nil, nil)
	if empty.Due.State != console.StateAbsent || !strings.Contains(empty.Due.Reason, "human-judged") {
		t.Errorf("no computable trigger: due = %+v", empty.Due)
	}
	if empty.Eroding.State != console.StateAbsent || !strings.Contains(empty.Eroding.Reason, "archconformance.cue") {
		t.Errorf("no reading: eroding = %+v", empty.Eroding)
	}
	one := buildDriftRadar(nil, trend[:2], nil)
	if one.Eroding.State != console.StateAbsent || !strings.Contains(one.Eroding.Reason, "second ingest") {
		t.Errorf("one reading: eroding = %+v", one.Eroding)
	}
	if bad := buildDriftRadar(nil, nil, errors.New("disk I/O error")); bad.Eroding.State != console.StateError {
		t.Errorf("unreadable trend: eroding = %+v", bad.Eroding)
	}
	if p := driftRadarPanel("svc", "", "", empty); !strings.Contains(p, "no violation reading recorded") || strings.Contains(p, ">0<") {
		t.Errorf("absent radar must say why and draw no zero:\n%s", p)
	}

	// On the Intent tab: the radar reads the audit's triggers and this
	// daemon's store. The fixture service has no rule pack, so the
	// arch_violations trigger is unevaluated, saying why.
	root := t.TempDir()
	writeCorpusFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte("# Decisions\n\n## ADR-001 — Layers\n\n**Revisit when.** the layers erode {instrument: arch_violations, threshold: \">= 3\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(state, "intent-ledger.jsonl"))
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(state, "verify-ledger.jsonl"))
	f := newConsoleFixture(t)
	seedProjectRoot(t, f, "payment-service", root)
	resetIntentAuditMemo()
	t.Cleanup(resetIntentAuditMemo)
	code, _, body := f.get(t, "/projects/payment-service/intent")
	if code != http.StatusOK {
		t.Fatalf("intent tab = %d: %.300s", code, body)
	}
	for _, want := range []string{`id="intent-radar"`, "Drift radar", "unevaluated — ADR-001 arch_violations", "no violation reading recorded for payment-service"} {
		if !strings.Contains(body, want) {
			t.Errorf("intent tab radar lacks %q", want)
		}
	}
	env := f.pageJSON(t, "/projects/payment-service/intent")
	var pm projectIntentModel
	if err := json.Unmarshal(env.Model, &pm); err != nil || pm.Radar == nil || pm.Radar.Eroding.State != console.StateAbsent {
		t.Fatalf("intent tab JSON radar = %+v (%v)", pm.Radar, err)
	}
}

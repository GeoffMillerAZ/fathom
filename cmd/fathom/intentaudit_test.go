package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// TestCmdIntentAuditSectionsAndBeat (EP-9): the verb renders the three
// sections, records its run in the intent ledger and a beat, and — the part
// that matters — reports what it could NOT compute instead of printing a
// clean zero. A daemon-less run must never look like "nothing is wrong".
func TestCmdIntentAuditSectionsAndBeat(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	ledger := filepath.Join(root, "intent-ledger.jsonl")
	t.Setenv("FATHOM_INTENT_LEDGER", ledger)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(root, "verify-ledger.jsonl"))

	var out, errBuf bytes.Buffer
	// Port 1 is closed: the index is unavailable, which must be NAMED.
	code := cmdIntent([]string{"audit", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("audit exit %d: %s", code, errBuf.String())
	}
	got := out.String()

	for _, want := range []string{"intent corpus for", "conditions:", "invariants:", "unproven claims:"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing section %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "NOT COMPUTED") {
		t.Errorf("an unreachable index must be named, never rendered as no findings:\n%s", got)
	}
	if !strings.Contains(got, "every condition is human-judged") {
		t.Errorf("with no annotations the verb must say nothing is evaluated:\n%s", got)
	}

	// The run left a debt row carrying K0's demand signal.
	body, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("no intent ledger written: %v", err)
	}
	var sawDebt bool
	for _, ln := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var row struct {
			Kind      string         `json:"kind"`
			ByKind    map[string]int `json:"by_kind"`
			Annotated int            `json:"annotated"`
		}
		if json.Unmarshal([]byte(ln), &row) == nil && row.Kind == "debt" {
			sawDebt = true
		}
	}
	if !sawDebt {
		t.Error("every audit must append a debt row — it is the only source for a +k delta and for K0's annotated count")
	}
}

// TestCmdIntentAuditJSON: --json carries the same facts a machine needs,
// including the uncomputable list (a JSON consumer must not see silence
// where a source failed).
func TestCmdIntentAuditJSON(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(root, "l.jsonl"))
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(root, "v.jsonl"))

	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{
		"audit", "--root", root, "--url", "http://127.0.0.1:1",
		"--service", "svc", "--json",
	}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var payload struct {
		Tier         string   `json:"tier"`
		Conditions   int      `json:"conditions"`
		Invariants   int      `json:"invariants"`
		Uncomputable []string `json:"uncomputable"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("--json must emit valid JSON: %v\n%s", err, out.String())
	}
	if payload.Tier != "router" {
		t.Errorf("tier = %q, want router", payload.Tier)
	}
	if payload.Conditions == 0 || payload.Invariants == 0 {
		t.Errorf("fixture corpus not parsed: %+v", payload)
	}
	if len(payload.Uncomputable) == 0 {
		t.Error("an unreachable index must appear in uncomputable, not as an empty finding list")
	}
}

// TestCmdIntentAudit_LearningSection is GL-14's "announced ... in fathom
// intent audit" half: a promotion/retirement recorded by
// learningAnnouncements shows up in BOTH the text report and --json, read
// off the SAME ledger row the attention line counts — never re-derived.
func TestCmdIntentAudit_LearningSection(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	ledgerPath := filepath.Join(root, "intent-ledger.jsonl")
	t.Setenv("FATHOM_INTENT_LEDGER", ledgerPath)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(root, "verify-ledger.jsonl"))

	if err := intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindLearning, RepoRoot: root, ID: "cl-learned-1", Verdict: "active",
		Evidence: "3 corroborations from 2 distinct sessions, 0 refutes over 30d",
	}); err != nil {
		t.Fatalf("AppendLedger: %v", err)
	}

	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{"audit", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errBuf); code != 0 {
		t.Fatalf("audit exit %d: %s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{"learning:", "cl-learned-1", "active", "3 corroborations from 2 distinct sessions"} {
		if !strings.Contains(got, want) {
			t.Errorf("text report missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	errBuf.Reset()
	if code := cmdIntent([]string{"audit", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc", "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("audit --json exit %d: %s", code, errBuf.String())
	}
	var payload struct {
		Learning []intent.LedgerRow `json:"learning"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("--json must emit valid JSON: %v\n%s", err, out.String())
	}
	if len(payload.Learning) != 1 || payload.Learning[0].ID != "cl-learned-1" {
		t.Errorf("--json learning = %+v, want one row for cl-learned-1", payload.Learning)
	}
}

// TestExportGuardProposesNeverWrites (EP-21): the edit-time channel for
// intent is a PROPOSAL, printed for a human to paste. Fathom must not write
// .fathom/guard.cue — law 15, and the custodian's PROPOSE precedent.
func TestExportGuardProposesNeverWrites(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	guardPath := filepath.Join(root, ".fathom", "guard.cue")

	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{"export", "--root", root, "--guard"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "guards: [") || !strings.Contains(got, "scopes:") {
		t.Errorf("export must print guard entries:\n%s", got)
	}
	if !strings.Contains(got, "internal/store/**") {
		t.Errorf("the fixture's surface glob must appear:\n%s", got)
	}
	if !strings.Contains(got, "intent audit") {
		t.Errorf("a proposed note should point back at the verb that produced it:\n%s", got)
	}
	if _, err := os.Stat(guardPath); err == nil {
		t.Error("export must NEVER write .fathom/guard.cue (law 15) — it prints a proposal")
	}

	// Every entry carries provenance, so a pasted entry's staleness stays
	// computable after the router moves on.
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, sf := range c.Surfaces {
		if !strings.Contains(got, "@ "+intent.SurfaceStamp(sf)) {
			t.Fatalf("surface %v carries no provenance stamp:\n%s", sf.Globs, got)
		}
	}
}

// A pasted entry goes STALE when its source row changes; an unstamped
// human-authored guard is never policed (it is not fathom's to maintain).
func TestGuardEntryStaleness(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Surfaces) == 0 {
		t.Fatal("fixture has no sensitive surface")
	}
	scopesOf := func(string) []string { return c.Surfaces[0].Globs }

	fresh := "intent: governed by I2 (fathom intent audit) @ " + intent.SurfaceStamp(c.Surfaces[0])
	for _, cl := range intent.GuardEntryStale(c, []string{fresh, "a note I wrote myself, no stamp"}, scopesOf) {
		if strings.HasPrefix(cl.Ref, ".fathom/guard.cue") {
			t.Fatalf("a fresh stamp and an unstamped note must not fire: %s", cl.Detail)
		}
	}

	stale := "intent: governed by I2 (fathom intent audit) @ 0123456789ab"
	found := false
	for _, cl := range intent.GuardEntryStale(c, []string{stale}, scopesOf) {
		if strings.HasPrefix(cl.Ref, ".fathom/guard.cue") && strings.Contains(cl.Detail, "0123456789ab") {
			found = true
		}
	}
	if !found {
		t.Fatal("a stamp matching no router row must fire as stale")
	}
}

// writeCorpusFixture builds the smallest corpus that exercises the router,
// an invariant table, a sensitive surface and a condition.
func writeCorpusFixture(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "intent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	router := `# Project intent — router

## Docs map

| Question | Read |
|---|---|
| Decisions | ` + "`../../ADR.md`" + ` |

## Sensitive surfaces and the evidence "done" needs

| Surface | Why it is sensitive | Done needs |
|---|---|---|
| ` + "`internal/store/**`" + ` | store purity (I1) | ` + "`go test -race ./internal/store`" + `; TestStorePurity |
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(router), 0o644); err != nil {
		t.Fatal(err)
	}
	invariants := `# Invariants

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I1 | The store is pure Go. | test ` + "`TestStorePurity`" + ` | ADR-025 |
| I2 | Nobody checks this one. | ` + "`stated`" + ` | none |

Revisit when: a denylist test exists (I2).
`
	if err := os.WriteFile(filepath.Join(dir, "invariants.md"), []byte(invariants), 0o644); err != nil {
		t.Fatal(err)
	}
	futures := `# Future directions

## F1. Something later

- Promote when: a second consumer appears.
- Drop if: never.
`
	if err := os.WriteFile(filepath.Join(dir, "futures.md"), []byte(futures), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := `# Scope

## Current goals

**G1. Ship it.** Done when: the gate is green.
`
	if err := os.WriteFile(filepath.Join(dir, "scope.md"), []byte(scope), 0o644); err != nil {
		t.Fatal(err)
	}
	adrDoc := `# Decisions

## ADR-001 — The first decision

**Decision.** Do the thing.

**Revisit when.** The thing stops working.
`
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(adrDoc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Package mcp_test — safe_delete_check (ADR-035 decision 1, plan VB-2):
// TDD tests plus the golden fixture-fleet test the plan row asks for.
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── golden: internal/connector/testdata/fauxtown, the deterministic
// fixture fleet (CLAUDE.md "Working conventions") ───────────────────────────
//
// payment-service (internal/connector/testdata/fauxtown/payment-service/
// payment.go): Charge calls validateCharge exactly once and nothing else
// in the fixture calls either of them — validateCharge has one real
// referent, Charge has none. Ingested through the real connector +
// extractor + fusion pipeline (indexbench.NewFauxtownIndex), so this
// exercises the tool against a genuinely produced index, not a hand-typed
// fixture.

func TestSafeDeleteCheck_Fauxtown_OneReferentOneNone(t *testing.T) {
	ctx := context.Background()
	c, _, err := indexbench.NewFauxtownIndex(ctx)
	if err != nil {
		t.Fatalf("NewFauxtownIndex: %v", err)
	}
	srv := mcp.NewServer(c)

	t.Run("validateCharge has one referent", func(t *testing.T) {
		got := callTool(t, srv, "safe_delete_check", map[string]any{
			"service": "payment-service", "symbol": "validateCharge",
		})
		referents, ok := got["referents"].([]any)
		if !ok {
			t.Fatalf("referents = %v (%T), want a non-empty array — validateCharge is called once by Charge", got["referents"], got["referents"])
		}
		if len(referents) != 1 {
			t.Fatalf("referents = %v, want exactly 1 (Charge)", referents)
		}
		row := referents[0].(map[string]any)
		if row["symbol"] != "Charge" {
			t.Errorf("referent symbol = %v, want Charge", row["symbol"])
		}
		if row["tier"] != "typed" && row["tier"] != "heuristic" {
			t.Errorf("referent tier = %v, want typed or heuristic", row["tier"])
		}
		if _, ok := row["file"].(string); !ok {
			t.Errorf("referent missing file: %v", row)
		}
		if got["checked_edges"] != "typed" && got["checked_edges"] != "heuristic" {
			t.Errorf("checked_edges = %v, want typed or heuristic", got["checked_edges"])
		}
	})

	t.Run("Charge has none — proven-zero receipt", func(t *testing.T) {
		got := callTool(t, srv, "safe_delete_check", map[string]any{
			"service": "payment-service", "symbol": "Charge",
		})
		referents, ok := got["referents"].([]any)
		if !ok || len(referents) != 0 {
			t.Fatalf("referents = %v (%T), want an empty array — Charge is never called in the fixture", got["referents"], got["referents"])
		}
		if count, _ := got["referent_count"].(float64); count != 0 {
			t.Errorf("referent_count = %v, want 0", got["referent_count"])
		}
		if got["checked_edges"] != "typed" && got["checked_edges"] != "heuristic" {
			t.Errorf("checked_edges = %v, want typed or heuristic", got["checked_edges"])
		}
		if _, ok := got["index_commit"]; !ok {
			t.Errorf("proven-zero receipt missing index_commit: %v", got)
		}
	})
}

// ─── typed vs heuristic tier, and test referents kept separate ─────────────
//
// navFake (resolved_nav_test.go): Real calls Target (typed-confirmed);
// Ghost's Calls list also names Target but the typed pass never confirmed
// it (the exact shape of the syntax pass's false positives); TestReal
// calls Real (typed-confirmed).

func TestSafeDeleteCheck_TypedAndHeuristicTiers(t *testing.T) {
	srv := mcp.NewServer(newNavFake())

	got := callTool(t, srv, "safe_delete_check", map[string]any{"service": "svc", "symbol": "Target"})
	referents, ok := got["referents"].([]any)
	if !ok || len(referents) != 2 {
		t.Fatalf("referents = %v, want 2 rows (Real typed, Ghost heuristic)", got["referents"])
	}
	// Typed edges first (VB-2's own ordering).
	first := referents[0].(map[string]any)
	second := referents[1].(map[string]any)
	if first["symbol"] != "Real" || first["tier"] != "typed" {
		t.Errorf("first referent = %v, want {symbol: Real, tier: typed}", first)
	}
	if second["symbol"] != "Ghost" || second["tier"] != "heuristic" {
		t.Errorf("second referent = %v, want {symbol: Ghost, tier: heuristic}", second)
	}
	if _, dup := got["test_referents"]; dup {
		t.Errorf("Target has no test callers, test_referents must be absent: %v", got)
	}
}

// TestSafeDeleteCheck_TestReferentsSeparate: Real's only caller is
// TestReal — a test function — so the code-referent list is empty (a
// proven-zero receipt) while test_referents still names the test that
// needs updating.
func TestSafeDeleteCheck_TestReferentsSeparate(t *testing.T) {
	srv := mcp.NewServer(newNavFake())

	got := callTool(t, srv, "safe_delete_check", map[string]any{"service": "svc", "symbol": "Real"})
	referents, ok := got["referents"].([]any)
	if !ok || len(referents) != 0 {
		t.Fatalf("referents = %v, want an empty array — Real's only caller is a test", got["referents"])
	}
	testRefs, ok := got["test_referents"].([]any)
	if !ok || len(testRefs) != 1 {
		t.Fatalf("test_referents = %v, want exactly 1 (TestReal)", got["test_referents"])
	}
	row := testRefs[0].(map[string]any)
	if row["symbol"] != "TestReal" {
		t.Errorf("test referent symbol = %v, want TestReal", row["symbol"])
	}
}

// ─── interface referents: an implementer is a referent even though it
// never "calls" the interface (writerFixtureNodes /
// find_symbol_implementations_test.go) ──────────────────────────────────────

func TestSafeDeleteCheck_InterfaceListsImplementations(t *testing.T) {
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(writerInterfaceSource), Lang: "go"}},
	}
	srv := mcp.NewServer(be)

	got := callTool(t, srv, "safe_delete_check", map[string]any{"service": "svc", "symbol": "Writer"})
	referents, ok := got["referents"].([]any)
	if !ok || len(referents) != 2 {
		t.Fatalf("referents = %v, want 2 rows (FileWriter, NopWriter)", got["referents"])
	}
	names := map[string]bool{}
	for _, r := range referents {
		row := r.(map[string]any)
		names[row["symbol"].(string)] = true
		if row["tier"] != "heuristic" {
			t.Errorf("implementer %v tier = %v, want heuristic (method_set, no typed edges in this fixture)", row["symbol"], row["tier"])
		}
	}
	if !names["FileWriter"] || !names["NopWriter"] {
		t.Errorf("referents = %v, want FileWriter and NopWriter", referents)
	}
}

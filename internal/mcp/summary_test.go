// Package mcp_test — wave L1 (vision 28 §3 envelopes) summary field tests:
// one golden-free assertion per verb on the `summary` field's actual
// content, plus H1-T05's exact regression case. Five of the nine verbs
// (find_callers, references, blast_radius, obligations, localize) are
// exercised against the real fauxtown fixture fleet
// (internal/connector/testdata/fauxtown, ingested through
// indexbench.NewFauxtownIndex — the same fixture l1-replay.json's golden
// entries already use), matching this package's own established
// convention (see TestSafeDeleteCheck_Fauxtown_OneReferentOneNone). The
// remaining four (blast_radius kind:"rename", test_impact, blast_radius
// kind:"delete", search — the first and third retired as standalone
// rename_impact/safe_delete_check tools by SR-24) reuse the lightweight
// fake backends already established for those tools in this package
// (richFakeBackend, the fauxtown safe_delete_check fixture, newSearchServer).
package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// newFauxtownServer ingests the fauxtown fixture fleet through the real
// connector + extractor + fusion pipeline AND seeds its fixture obligation
// claims (indexbench.NewFixtureServer) — the same construction
// internal/indexbench/testdata/l1-replay.json's golden entries replay
// against, so these assertions and that golden describe the same fixture.
func newFauxtownServer(t *testing.T) *mcp.Server {
	t.Helper()
	srv, err := indexbench.NewFixtureServer(context.Background())
	if err != nil {
		t.Fatalf("NewFixtureServer: %v", err)
	}
	return srv
}

// ─── H1-T05's exact case ─────────────────────────────────────────────────
//
// docs/bench/readings/vs-pilot-6-second-reading.md: find_callers listed the
// direct caller correctly, and the model's final answer named a caller
// three hops out instead. graphBackend (depth_tools_test.go) has exactly
// this shape: Target's direct (depth-1) callers are Direct1/Direct2; Deep
// calls Direct1 (depth 2); TestDeep calls Deep (depth 3).

func TestFindCallersSummary_NamesDirectNeverDepth2(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "find_callers", map[string]any{
		"service": "svc", "symbol": "Target", "depth": 0, "include_heuristic": true,
	})
	summary, _ := payload["summary"].(string)
	if summary == "" {
		t.Fatalf("summary missing: %v", payload)
	}
	if !strings.Contains(summary, "Direct1") || !strings.Contains(summary, "Direct2") {
		t.Errorf("summary = %q, want it to name both direct callers Direct1 and Direct2", summary)
	}
	// The defect H1-T05 found: naming a depth-2+ caller as if it were
	// direct. Deep is depth 2, TestDeep depth 3 — neither belongs in the
	// "direct callers of Target: …" clause, only (optionally) in the
	// "more at depth N" tail.
	directClause, _, _ := strings.Cut(summary, ";")
	if strings.Contains(directClause, "Deep") {
		t.Errorf("summary's direct-callers clause = %q, must never name Deep (a depth-2 caller) as direct", directClause)
	}
	if !strings.Contains(summary, "2 direct callers of Target") {
		t.Errorf("summary = %q, want a count of 2 direct callers", summary)
	}
}

// ─── per-verb summary content, fauxtown-backed ──────────────────────────

func TestFindCallersSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)
	payload := callTool(t, srv, "find_callers", map[string]any{"service": "ledger-service", "symbol": "validateEntry"})
	summary, _ := payload["summary"].(string)
	want := "1 direct caller of validateEntry: Post (ledger-service/ledger.go:21)"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestReferencesSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)
	payload := callTool(t, srv, "references", map[string]any{"query": "Charge", "services": []string{"payment-service"}})
	summary, _ := payload["summary"].(string)
	if !strings.HasPrefix(summary, `1 definition, 12 occurrences of "Charge"; first: `) {
		t.Errorf("summary = %q, want counts-by-kind prefix with first locations", summary)
	}
	if !strings.Contains(summary, "payment-service/payment.go:") {
		t.Errorf("summary = %q, want it to name a payment.go location", summary)
	}
}

func TestBlastRadiusSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)
	payload := callTool(t, srv, "blast_radius", map[string]any{
		"service": "ledger-service", "changed_files": []string{"ledger-service/ledger.go"},
	})
	summary, _ := payload["summary"].(string)
	// 13: ledger.go's 7 declarations and, since K17 (wave lf2), Entry's
	// 4 fields and Ledger's 2 methods.
	want := "depth 0: 13 changed symbols; affected tests: 0"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestObligationsSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)
	payload := callTool(t, srv, "obligations", map[string]any{"service": "ledger-service", "file": "ledger-service/ledger.go"})
	summary, _ := payload["summary"].(string)
	if !strings.HasPrefix(summary, "governing rule for ledger-service/ledger.go: ") {
		t.Errorf("summary = %q, want the governing-rule-for-file prefix", summary)
	}
	if !strings.Contains(summary, "doc-claimed") || !strings.Contains(summary, "docs/ledger.md#L") {
		t.Errorf("summary = %q, want it to cite tier and source:line", summary)
	}
}

func TestLocalizeSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)
	payload := callTool(t, srv, "localize", map[string]any{
		"service": "ledger-service", "stack_trace": "panic: nil ledger\n\tat ledger-service/ledger.go:22\n\tin Post",
	})
	summary, _ := payload["summary"].(string)
	want := "top candidate: ledger-service/ledger.go (Post) — tier typed; stack frame ledger-service/ledger.go:22"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestSafeDeleteCheckSummary_Fauxtown(t *testing.T) {
	srv := newFauxtownServer(t)

	got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "payment-service", "symbol": "validateCharge"})
	want := "1 referent for validateCharge: payment-service/payment.go:26"
	if s, _ := got["summary"].(string); s != want {
		t.Errorf("validateCharge summary = %q, want %q", s, want)
	}

	// Charge is never called in the fixture — the proven-zero receipt line.
	got2 := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "payment-service", "symbol": "Charge"})
	s2, _ := got2["summary"].(string)
	if !strings.HasPrefix(s2, "0 referents for Charge — proven zero (") {
		t.Errorf("Charge summary = %q, want the proven-zero receipt line", s2)
	}
}

// ─── per-verb summary content, lightweight fixtures ─────────────────────

func TestRenameImpactSummary(t *testing.T) {
	srv := newAnalysisSrv()
	payload := callTool(t, srv, "blast_radius", map[string]any{"kind": "rename", "service": "svc", "symbol": "B"})
	summary, _ := payload["summary"].(string)
	want := "1 call site for B: svc::pkg.A"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestRenameImpactSummary_NoCallers(t *testing.T) {
	srv := newAnalysisSrv()
	payload := callTool(t, srv, "blast_radius", map[string]any{"kind": "rename", "service": "svc", "symbol": "NoSuchFunc"})
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "0 call sites") {
		t.Errorf("summary = %q, want it to say 0 call sites", summary)
	}
}

func TestTestImpactSummary(t *testing.T) {
	srv := newAnalysisSrv()
	payload := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "A"})
	summary, _ := payload["summary"].(string)
	want := "1 test reaches A; nearest: svc::pkg.TestFoo"
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
}

func TestTestImpactSummary_Orphan(t *testing.T) {
	srv := newAnalysisSrv()
	payload := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "Orphan"})
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "no tests reach Orphan") {
		t.Errorf("summary = %q, want it to say no tests reach Orphan", summary)
	}
}

func TestSearchSummary(t *testing.T) {
	srv, _ := newSearchServer(t)
	payload := callTool(t, srv, "search", map[string]any{"service": "svc", "pattern": `func [AB]\(`})
	summary, _ := payload["summary"].(string)
	if !strings.HasPrefix(summary, "2 hits in 1 file; first: pkg/a.go:") {
		t.Errorf("summary = %q, want the hit/file counts with the first location", summary)
	}
}

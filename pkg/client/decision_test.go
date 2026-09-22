package client_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestClient_WithDecision_RankedBy pins DP-9's client-level wiring
// (docs/plans/decision-port.md): a Client that never calls WithDecision, or
// is explicitly wired with decision.None, serves obligations with
// ranked_by:"none" (obligationsCore's own untouched path) — a Client wired
// with a real decision.Decider AND rank_within_tier on serves ranked_by
// naming that Decider, proving Obligations is actually routed through
// semantic.ObligationsForOpts and not the pre-DP-9 ObligationsFor.
func TestClient_WithDecision_RankedBy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	if _, err := c.CompileObligations(ctx, "svc", false); err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}

	// Baseline: WithDecision never called — ranked_by "none".
	obl, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (no decision port): %v", err)
	}
	if len(obl.Rules) == 0 {
		t.Fatal("expected at least one rule for pkg/foo.go — test fixture is broken")
	}
	if obl.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (no WithDecision call)", obl.RankedBy, "none")
	}

	// decision.None wired explicitly — still "none": None must never rank.
	c.WithDecision(decision.None{}, config.Decide{RankWithinTier: true}, nil)
	oblNone, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (None decider): %v", err)
	}
	if oblNone.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (decision.None wired)", oblNone.RankedBy, "none")
	}

	// A real (mock) Decider with rank_within_tier on — ranked_by names it.
	mock := decision.NewMockDecider("mock-decider", decision.MockEntry{
		Response: decision.Response{Provider: "mock-decider"},
	})
	c.WithDecision(mock, config.Decide{RankWithinTier: true}, nil)
	oblMock, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (mock decider): %v", err)
	}
	if oblMock.RankedBy != "mock-decider" {
		t.Errorf("RankedBy = %q, want %q (mock decider wired, rank_within_tier on)", oblMock.RankedBy, "mock-decider")
	}
	if len(mock.Calls()) != 1 {
		t.Errorf("mock decider called %d times, want 1", len(mock.Calls()))
	}

	// rank_within_tier OFF gates the call even with a real Decider wired —
	// both gates (Decider != nil AND RankWithinTier) must hold.
	c.WithDecision(mock, config.Decide{RankWithinTier: false}, nil)
	oblGated, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (rank_within_tier off): %v", err)
	}
	if oblGated.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (rank_within_tier off)", oblGated.RankedBy, "none")
	}
}

// TestClient_WithDecision_NoneLeavesCompileUntouched pins the CompileLayer
// half of DP-9: a Client wired with decision.None (or never wired at all)
// compiles with SectionsPrefiltered == 0 and no LLM prefilter call — the
// prefilter's own identity fast path, reached only because
// Client.decisionActive gates Compiler.Decider/PrefilterThreshold off for a
// None decider instead of wiring it through unconditionally.
func TestClient_WithDecision_NoneLeavesCompileUntouched(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	c.WithDecision(decision.None{}, config.Decide{CompileThreshold: 0.9}, nil)

	run, err := c.CompileObligations(ctx, "svc", false)
	if err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}
	if run.SectionsPrefiltered != 0 {
		t.Errorf("SectionsPrefiltered = %d, want 0 (decision.None must never skip a section)", run.SectionsPrefiltered)
	}
	if len(run.Decisions) != 0 {
		t.Errorf("Decisions = %d rows, want 0 (decision.None makes no call, records no decision)", len(run.Decisions))
	}
}

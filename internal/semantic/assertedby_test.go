package semantic

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestCompile_AssertedByIsTheRunID (SU-22, ADR-041 decision 7): every claim
// a compile run mints carries core.AssertedBy("compile", run.RunID) — the
// PROV-style link to the run that asserted it.
func TestCompile_AssertedByIsTheRunID(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":3,"cite_end":3}
]}`,
		Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
	}})
	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.RunID == "" {
		t.Fatal("run.RunID is empty")
	}
	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v, %d claims", err, len(claims))
	}
	want := core.AssertedBy("compile", run.RunID)
	if claims[0].AssertedBy != want {
		t.Errorf("AssertedBy = %q, want %q", claims[0].AssertedBy, want)
	}
	if !strings.HasPrefix(claims[0].AssertedBy, "compile:") {
		t.Errorf("AssertedBy = %q, want a compile: prefix", claims[0].AssertedBy)
	}
}

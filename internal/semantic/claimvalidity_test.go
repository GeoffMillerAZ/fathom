package semantic

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestNoClaimEndingPathDeletes is GL-15's gate (docs/plans/governance-learning.md,
// design doc §7 "Validity windows everywhere a fact ends"): every path that
// ends a claim — closeClaimValidity itself, and compile reconciliation's
// supersede loop, the one in-package caller — closes the bi-temporal
// validity window (ValidTo) instead of deleting the row. It proves this two
// ways: closeClaimValidity's own direct effect, and Compile()'s real
// reconciliation path (the exact code compile.go's supersede loop runs),
// asserting the claim count never shrinks and every ended claim keeps a
// non-zero ValidTo.
func TestNoClaimEndingPathDeletes(t *testing.T) {
	ctx := context.Background()

	t.Run("closeClaimValidity_direct", func(t *testing.T) {
		st := store.NewMemoryStore()
		claim := core.SemanticClaim{
			ServiceID: "svc", ClaimID: "cl-direct", Layer: "obligations", Kind: "obligation",
			Subject: "CLAUDE.md#rule", Rule: "Use task validate before pushing.",
			Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
			Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
		}
		if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		before, err := st.ClaimsForService(ctx, "svc", "")
		if err != nil {
			t.Fatalf("ClaimsForService before: %v", err)
		}

		at := time.Unix(2, 0).UTC()
		if err := closeClaimValidity(ctx, st, "svc", "cl-direct", core.ClaimRefuted, at); err != nil {
			t.Fatalf("closeClaimValidity: %v", err)
		}

		after, err := st.ClaimsForService(ctx, "svc", "")
		if err != nil {
			t.Fatalf("ClaimsForService after: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("claim count after close = %d, want %d (never deletes)", len(after), len(before))
		}
		if after[0].Status != core.ClaimRefuted {
			t.Errorf("status = %s, want refuted", after[0].Status)
		}
		if !after[0].ValidTo.Equal(at) {
			t.Errorf("valid_to = %v, want %v (window closed)", after[0].ValidTo, at)
		}
	})

	t.Run("compile_supersede_path", func(t *testing.T) {
		st := store.NewMemoryStore()
		c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
		}})
		if _, err := c.Compile(ctx, compileInput(false)); err != nil {
			t.Fatalf("first Compile: %v", err)
		}
		before, err := st.ClaimsForService(ctx, "svc", LayerObligations)
		if err != nil || len(before) != 1 {
			t.Fatalf("claims after first compile = %d, %v, want 1", len(before), err)
		}

		// The doc leaves the corpus selector entirely — compile.go's
		// supersede loop is the ONLY path that retires it (the sweep
		// can't: the doc still exists on disk and its citation still
		// hash-matches).
		c2 := newCompiler(t, st)
		in := compileInput(false)
		in.Docs = nil
		if _, err := c2.Compile(ctx, in); err != nil {
			t.Fatalf("second Compile: %v", err)
		}

		after, err := st.ClaimsForService(ctx, "svc", LayerObligations)
		if err != nil {
			t.Fatalf("ClaimsForService after: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("claim count after supersede = %d, want %d (never deletes)", len(after), len(before))
		}
		got := after[0]
		if got.Status != core.ClaimSuperseded {
			t.Fatalf("status = %s, want superseded", got.Status)
		}
		if got.ValidTo.IsZero() {
			t.Errorf("valid_to is zero after supersede, want closed at %v (the fix this test guards)", fixedNow())
		}
		if !got.ValidTo.Equal(fixedNow()) {
			t.Errorf("valid_to = %v, want %v (closed at the injected clock's instant)", got.ValidTo, fixedNow())
		}
	})
}

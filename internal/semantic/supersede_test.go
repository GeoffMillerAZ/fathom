package semantic

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func seedFindingClaim(t *testing.T, st store.Store, id string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc", Layer: "findings", Kind: "finding",
		Subject: "arch_conformance#pkg.A", Rule: "rule " + id, Scope: []string{"pkg/a.go"},
		Tier: core.TierVerified, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: "review-verb/v1", ReproKey: id, InputDigest: id,
		CompiledAt: time.Now(), ValidFrom: time.Now(),
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

// TestSupersessionChain_WalksThroughTwoSupersedes (SU-20): a native
// re-review that supersedes twice (C1 -> C2 -> C3) produces a chain
// SupersessionChain(C3) can walk back through both predecessors.
func TestSupersessionChain_WalksThroughTwoSupersedes(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedFindingClaim(t, st, "c1")
	seedFindingClaim(t, st, "c2")
	seedFindingClaim(t, st, "c3")

	if err := WriteSupersedes(ctx, st, "svc", "c2", "c1", "review run 2: rule text changed"); err != nil {
		t.Fatalf("WriteSupersedes c2->c1: %v", err)
	}
	if err := WriteSupersedes(ctx, st, "svc", "c3", "c2", "review run 3: rule text changed again"); err != nil {
		t.Fatalf("WriteSupersedes c3->c2: %v", err)
	}

	chain, err := SupersessionChain(ctx, st, "svc", "c3")
	if err != nil {
		t.Fatalf("SupersessionChain: %v", err)
	}
	if !reflect.DeepEqual(chain, []string{"c2", "c1"}) {
		t.Errorf("chain = %v, want [c2 c1]", chain)
	}

	// A claim that supersedes nothing (c1, the oldest) returns an empty
	// chain, never an error.
	empty, err := SupersessionChain(ctx, st, "svc", "c1")
	if err != nil {
		t.Fatalf("SupersessionChain(c1): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("SupersessionChain(c1) = %v, want empty", empty)
	}
}

// TestSupersessionChain_UnknownClaimIsEmpty pins the honest-absence
// contract: a claim id with no supersedes link (or that does not exist at
// all) returns an empty chain, never an error.
func TestSupersessionChain_UnknownClaimIsEmpty(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	chain, err := SupersessionChain(ctx, st, "svc", "no-such-claim")
	if err != nil {
		t.Fatalf("SupersessionChain: %v", err)
	}
	if len(chain) != 0 {
		t.Errorf("chain = %v, want empty", chain)
	}
}

// TestWriteSupersedes_PreservesExistingLinks (SU-20): appending a
// supersedes link must never drop the successor's own cites/scopes links
// written at mint — the whole reason AppendClaimLink exists instead of
// reusing PutClaims' wholesale replace.
func TestWriteSupersedes_PreservesExistingLinks(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	claim := core.SemanticClaim{
		ClaimID: "c2", ServiceID: "svc", Layer: "findings", Kind: "finding",
		Subject: "arch_conformance#pkg.A", Rule: "rule c2", Scope: []string{"pkg/a.go"},
		Tier: core.TierVerified, Status: core.ClaimFresh, ReproKey: "c2",
	}
	links := []core.ClaimLink{
		{ClaimID: "c2", Rel: core.RelCites, TargetRef: "pkg/a.go#L1-L2", TargetHash: "h"},
		{ClaimID: "c2", Rel: core.RelScopes, TargetRef: "pkg/a.go"},
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := WriteSupersedes(ctx, st, "svc", "c2", "c1", "fixed"); err != nil {
		t.Fatalf("WriteSupersedes: %v", err)
	}
	got, err := st.ClaimLinks(ctx, "svc", []string{"c2"})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("links after WriteSupersedes = %+v, want 3 (2 original + supersedes)", got)
	}
}

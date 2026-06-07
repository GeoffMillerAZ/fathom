package query_test

import (
	"context"
	"testing"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/query"
	"github.com/geoffmilleraz/sextant/internal/vector"
)

// buildHybridEngine creates an Engine with a vector.Indexer backed by
// MockEmbedder and a pre-populated MemoryVectorStore.
func buildHybridEngine(t *testing.T, obs []core.Observation, semanticWt float64) *query.Engine {
	t.Helper()

	ms := newStore(t, obs)

	// Build a vector Indexer using a deterministic mock embedder.
	mock := embeddingclient.NewMockEmbedder(0) // DefaultMockDim=16
	vs := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, vs)

	// Index all observations as SymbolNodes.
	nodes := make([]core.SymbolNode, 0, len(obs))
	seen := make(map[string]bool)
	for _, o := range obs {
		key := o.Symbol + "|" + o.SourceRef
		if seen[key] {
			continue
		}
		seen[key] = true
		nodes = append(nodes, core.SymbolNode{
			Service:   o.ServiceID,
			Name:      o.Symbol,
			Kind:      o.SourceKind,
			FilePath:  o.SourceRef,
			Signature: o.Claim,
			Calls:     o.Calls,
			Tier:      o.EvidenceTier,
		})
	}
	if err := idx.Index(context.Background(), nodes); err != nil {
		t.Fatalf("idx.Index: %v", err)
	}

	return query.New(ms, query.WithVector(idx))
}

// TestHybrid_NoVectorConfigKeywordOnly verifies that an engine without a vector
// Indexer behaves exactly as keyword-only even when SemanticWt > 0.
func TestHybrid_NoVectorConfigKeywordOnly(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Alpha", Claim: "auth fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Beta", Claim: "billing fn", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms) // no WithVector

	result, err := e.Query(context.Background(), core.Query{
		ServiceID:  "svc",
		ConcernID:  "c",
		Keywords:   []string{"auth"},
		SemanticWt: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return nodes (keyword-only path).
	if len(result.Nodes) == 0 {
		t.Error("expected nodes from keyword-only fallback when no indexer configured")
	}
	// Alpha matches the keyword "auth"; it should rank first.
	if result.Nodes[0].Name != "Alpha" {
		t.Errorf("expected Alpha first (keyword match), got %q", result.Nodes[0].Name)
	}
}

// TestHybrid_ZeroSemanticWtKeywordOnly verifies that SemanticWt=0 always
// uses the keyword-only path regardless of whether an indexer is wired.
func TestHybrid_ZeroSemanticWtKeywordOnly(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Alpha", Claim: "auth fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Beta", Claim: "billing fn", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
	}
	e := buildHybridEngine(t, obs, 0)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID:  "svc",
		ConcernID:  "c",
		Keywords:   []string{"auth"},
		SemanticWt: 0, // keyword-only
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) == 0 {
		t.Fatal("expected nodes, got none")
	}
	if result.Nodes[0].Name != "Alpha" {
		t.Errorf("keyword-only: expected Alpha first, got %q", result.Nodes[0].Name)
	}
}

// TestHybrid_BlendedScoreReturnsBothNodes verifies that hybrid mode returns
// nodes from both keyword and vector paths.
func TestHybrid_BlendedScoreReturnsBothNodes(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "ValidateToken", Claim: "validates JWT auth token", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "ChargeCard", Claim: "charges credit card payment", EvidenceTier: core.TierVerified, SourceRef: "pay.go", IngestedAt: epoch},
	}
	e := buildHybridEngine(t, obs, 0.5)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID:  "svc",
		ConcernID:  "c",
		Keywords:   []string{"auth", "token"},
		SemanticWt: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes in hybrid result, got %d", len(result.Nodes))
	}
	// ValidateToken matches both keywords; it should rank above ChargeCard.
	if result.Nodes[0].Name != "ValidateToken" {
		t.Errorf("expected ValidateToken first (keyword+vector), got %q", result.Nodes[0].Name)
	}
}

// TestHybrid_Deterministic verifies that hybrid results are stable across
// repeated calls (MockEmbedder is deterministic, so cosine scores are fixed).
func TestHybrid_Deterministic(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Alpha", Claim: "alpha fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Beta", Claim: "beta fn", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Gamma", Claim: "gamma fn", EvidenceTier: core.TierVerified, SourceRef: "c.go", IngestedAt: epoch},
	}
	e := buildHybridEngine(t, obs, 0.5)

	q := core.Query{ServiceID: "svc", ConcernID: "c", Keywords: []string{"fn"}, SemanticWt: 0.5}

	first, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 10; i++ {
		result, err := e.Query(context.Background(), q)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(result.Nodes) != len(first.Nodes) {
			t.Fatalf("iteration %d: node count %d != %d", i, len(result.Nodes), len(first.Nodes))
		}
		for j := range result.Nodes {
			if result.Nodes[j].Name != first.Nodes[j].Name {
				t.Errorf("iteration %d node[%d]: got %q want %q", i, j, result.Nodes[j].Name, first.Nodes[j].Name)
			}
		}
	}
}

// TestHybrid_TopNRespected verifies that TopN truncation applies in hybrid mode.
func TestHybrid_TopNRespected(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "B", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "C", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "c.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "D", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "d.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "E", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "e.go", IngestedAt: epoch},
	}
	e := buildHybridEngine(t, obs, 0.5)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID:  "svc",
		ConcernID:  "c",
		Keywords:   []string{"fn"},
		SemanticWt: 0.5,
		TopN:       3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 3 {
		t.Errorf("expected 3 nodes (TopN=3), got %d", len(result.Nodes))
	}
}

// TestHybrid_StalenessAndContradictionsStillPresent verifies that staleness and
// contradiction detection still work correctly in hybrid mode.
func TestHybrid_StalenessAndContradictionsStillPresent(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Stripe", EvidenceTier: core.TierVerified, SourceRef: "code.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Braintree", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
	}
	e := buildHybridEngine(t, obs, 0.5)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID:  "svc",
		ConcernID:  "c",
		Keywords:   []string{"payment"},
		SemanticWt: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contradictions) == 0 {
		t.Error("expected contradiction detected in hybrid mode")
	}
	if len(result.Staleness.Sources) == 0 {
		t.Error("expected staleness sources in hybrid mode")
	}
}

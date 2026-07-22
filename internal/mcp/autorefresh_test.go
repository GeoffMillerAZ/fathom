package mcp

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// refreshFake is a QueryInterface whose ServiceMeta reports a fixed indexed
// commit and which counts IngestService calls.
type refreshFake struct {
	path, indexed string
	ingests       int
}

func (f *refreshFake) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return f.path, f.indexed, true, nil
}
func (f *refreshFake) IngestService(_ context.Context, _ string) error { f.ingests++; return nil }
func (f *refreshFake) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *refreshFake) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *refreshFake) Services(_ context.Context) ([]string, error) { return nil, nil }

func (f *refreshFake) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (f *refreshFake) ListCommits(_ context.Context, _ string) ([]CommitInfo, error) { return nil, nil }

func (f *refreshFake) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) { return nil, nil }

func newAR(fake QueryInterface, head string) *autoRefreshBackend {
	return &autoRefreshBackend{
		QueryInterface: fake,
		headFn:         func(string) string { return head },
		seen:           make(map[string]string),
	}
}

// TestAutoRefresh_ReingestsOnDriftOnce: HEAD drifted from the indexed commit, so
// the first read re-ingests; subsequent reads are debounced (at most one ingest
// per (service, commit)).
func TestAutoRefresh_ReingestsOnDriftOnce(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "oldsha"}
	ar := newAR(fake, "newsha")
	ctx := context.Background()
	ar.ListAllSymbols(ctx, "svc")
	ar.FindSymbol(ctx, "svc", "X")
	ar.QuerySymbols(ctx, core.Query{ServiceID: "svc"})
	ar.Sources(ctx, "svc")
	ar.TypedEdges(ctx, "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests = %d, want 1 (debounced)", fake.ingests)
	}
}

// TestAutoRefresh_TypedEdges_TriggersEnsure confirms TypedEdges is one of
// the intercepted per-service reads (ADR-021 §3): a fresh wrapper (no prior
// debounce) must refresh before answering, exactly like the other reads.
func TestAutoRefresh_TypedEdges_TriggersEnsure(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "oldsha"}
	ar := newAR(fake, "newsha")
	if _, err := ar.TypedEdges(context.Background(), "svc"); err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if fake.ingests != 1 {
		t.Fatalf("ingests = %d, want 1 (TypedEdges must trigger ensure)", fake.ingests)
	}
}

// TestAutoRefresh_NoReingestWhenFresh: HEAD == indexed commit → no ingest.
func TestAutoRefresh_NoReingestWhenFresh(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "same"}
	ar := newAR(fake, "same")
	ar.ListAllSymbols(context.Background(), "svc")
	if fake.ingests != 0 {
		t.Fatalf("ingests = %d, want 0 (fresh)", fake.ingests)
	}
}

// TestAutoRefresh_NoReingestWhenHeadUnknown: git HEAD unavailable → leave the
// index as-is rather than risk a failed query.
func TestAutoRefresh_NoReingestWhenHeadUnknown(t *testing.T) {
	fake := &refreshFake{path: "/repo", indexed: "old"}
	ar := newAR(fake, "")
	ar.ListAllSymbols(context.Background(), "svc")
	if fake.ingests != 0 {
		t.Fatalf("ingests = %d, want 0 (head unknown)", fake.ingests)
	}
}

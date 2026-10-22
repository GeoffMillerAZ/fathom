package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// With no store identity the typed loads cannot be cached. The second read
// happens inside the graph build, making graph failures injectable without
// changing memo budgets, tenancy or deadlines.
type reviewFallbackBackend struct {
	patchFake
	edges   []core.Edge
	reads   int
	failAt  int
	failure error
	cancel  context.CancelFunc
}

func (*reviewFallbackBackend) StoreID() string { return "" }

func (b *reviewFallbackBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	b.reads++
	if b.reads == b.failAt {
		if b.cancel != nil {
			b.cancel()
		}
		return nil, b.failure
	}
	return b.edges, nil
}

func newReviewFallbackBackend() *reviewFallbackBackend {
	b := &reviewFallbackBackend{}
	b.nodes = []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Target", FilePath: "pkg/target.go", Line: 1},
		{Service: "svc", Package: "pkg", Name: "Caller", FilePath: "pkg/caller.go", Line: 1},
	}
	b.edges = []core.Edge{{FromSymbol: b.nodes[1].Key(), ToSymbol: b.nodes[0].Key(), Kind: core.EdgeDirect}}
	return b
}

func TestReviewMemoFailurePreservesTypedImpact(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failAt int
		err    error
	}{
		{"typed-admission", 1, memo.ErrWouldNotFinish},
		{"graph-build", 2, errors.New("graph typed load failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			b := newReviewFallbackBackend()
			b.failAt, b.failure = tc.failAt, tc.err
			in, corpus, rpcErr := buildReviewInput(ctx, b, "svc", []string{"pkg/target.go"}, "", "", nil)
			if rpcErr != nil {
				t.Fatalf("optional memo failure aborted review: %+v", rpcErr)
			}
			if in.ImpactGraph != nil || len(in.TypedEdges) != 1 || corpus.typedNote != "" {
				t.Errorf("fallback lost typed impact: graph=%v edges=%v note=%q", in.ImpactGraph != nil, in.TypedEdges, corpus.typedNote)
			}
			if b.reads != tc.failAt+1 {
				t.Errorf("typed reads=%d, want %d including direct fallback", b.reads, tc.failAt+1)
			}
			baseline := newReviewFallbackBackend()
			wantIn, _, rpcErr := buildReviewInput(ctx, baseline, "svc", []string{"pkg/target.go"}, "", "", nil)
			if rpcErr != nil {
				t.Fatal(rpcErr)
			}
			got, _ := json.Marshal(review.Run(in))
			want, _ := json.Marshal(review.Run(wantIn))
			if !bytes.Equal(got, want) {
				t.Fatalf("fallback changed verdict bytes:\n%s\n%s", got, want)
			}
		})
	}
}

func TestReviewMemoFailureAfterCancellationReturnsNavError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newReviewFallbackBackend()
	b.failAt, b.failure, b.cancel = 2, errors.New("graph typed load failed"), cancel
	_, _, rpcErr := buildReviewInput(ctx, b, "svc", []string{"pkg/target.go"}, "", "", nil)
	if rpcErr == nil {
		t.Fatal("cancelled caller received a review")
	}
	if b.reads != 2 {
		t.Fatalf("cancelled caller retried typed load: reads=%d", b.reads)
	}
}

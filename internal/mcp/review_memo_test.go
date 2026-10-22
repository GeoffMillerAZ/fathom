package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

type reviewMemoBackend struct {
	patchFake
	typedReads atomic.Int32
	posts      []core.Edge
}

func (b *reviewMemoBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	b.typedReads.Add(1)
	return append([]core.Edge(nil), b.posts...), nil
}

func reviewGraphMisses() int64 {
	for _, s := range memo.Stats() {
		if s.Name == "graphstore" {
			return s.Misses
		}
	}
	return 0
}

func TestReviewDiffReusesMemoGraphAndTypedEdges(t *testing.T) {
	ctx := context.Background()
	b := &reviewMemoBackend{}
	for i := range 512 {
		b.nodes = append(b.nodes, core.SymbolNode{Service: "svc", Package: "pkg", Name: fmt.Sprintf("F%d", i), FilePath: fmt.Sprintf("pkg/f%d.go", i), Line: 1})
	}
	b.posts = []core.Edge{{FromSymbol: b.nodes[1].Key(), ToSymbol: b.nodes[0].Key(), Kind: core.EdgeDirect}}
	s := NewServer(b)
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"review_diff","arguments":{"service":"svc","changed_files":["pkg/f0.go"]}}}`)
	first, ok := s.HandleRequest(ctx, request)
	if !ok || bytes.Contains(first, []byte(`"error":`)) {
		t.Fatalf("first review: %s", first)
	}
	before := reviewGraphMisses()
	b.typedReads.Store(0)
	second, ok := s.HandleRequest(ctx, request)
	if !ok || !bytes.Equal(first, second) {
		t.Fatalf("review bytes changed:\n%s\n%s", first, second)
	}
	if got := b.typedReads.Load(); got != 0 {
		t.Errorf("warm backend TypedEdges calls=%d, want 0", got)
	}
	if got := reviewGraphMisses() - before; got != 0 {
		t.Errorf("warm graph builds=%d, want 0", got)
	}
	in, _, rpcErr := buildReviewInput(ctx, b, "svc", []string{"pkg/f0.go"}, "", "", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	snap, err := serviceSnapshot(ctx, b, "svc")
	if err != nil {
		t.Fatal(err)
	}
	g, err := memo.GraphFor(ctx, snap, memo.GraphBoth, typedLoader(b, "svc"))
	if err != nil || in.ImpactGraph != g {
		t.Errorf("review input did not reuse the memo graph: %v", err)
	}
	got, _ := json.Marshal(review.Run(in))
	in.ImpactGraph = nil
	want, _ := json.Marshal(review.Run(in))
	if !bytes.Equal(got, want) {
		t.Fatalf("shared graph changed dimension bytes:\n%s\n%s", got, want)
	}
	allocs := testing.AllocsPerRun(3, func() {
		raw, ok := s.HandleRequest(ctx, request)
		if !ok || !bytes.Equal(raw, second) {
			t.Fatal("warm review response changed")
		}
	})
	t.Logf("warm review allocations: %.0f", allocs)
	base := testing.AllocsPerRun(3, func() { _ = review.Run(in) })
	in.ImpactGraph = g
	shared := testing.AllocsPerRun(3, func() { _ = review.Run(in) })
	t.Logf("base=%.0f shared=%.0f impact allocations", base, shared)
	if shared >= base*0.85 {
		t.Fatalf("shared graph allocations=%.0f, want <85%% of base graph %.0f", shared, base)
	}
}

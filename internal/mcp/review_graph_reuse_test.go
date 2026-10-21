package mcp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

type reviewMemoFake struct {
	refreshFake
	testStore
	nodes      []core.SymbolNode
	edges      []core.Edge
	listErr    error
	typedErr   error
	loads      atomic.Int32
	typedLoads atomic.Int32
	mark       atomic.Value
	token      atomic.Value
}

func (f *reviewMemoFake) StoreWatermark(context.Context, string) (string, error) {
	if v := f.mark.Load(); v != nil {
		return v.(string), nil
	}
	return "w1", nil
}
func (f *reviewMemoFake) SuppressionToken(string) string {
	if v := f.token.Load(); v != nil {
		return v.(string)
	}
	return ""
}
func (f *reviewMemoFake) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	f.loads.Add(1)
	return f.nodes, f.listErr
}
func (f *reviewMemoFake) TypedEdges(context.Context, string) ([]core.Edge, error) {
	f.typedLoads.Add(1)
	return f.edges, f.typedErr
}

func TestBuildReviewInputSharesOneCompleteSnapshotAndGraph(t *testing.T) {
	f := &reviewMemoFake{nodes: []core.SymbolNode{{Service: "svc", Package: "p", Name: "F", Kind: core.KindGoFunc, FilePath: "p/f.go"}}}
	ctx := context.Background()
	a, _, err := buildReviewInput(ctx, f, "svc", []string{"p/f.go"}, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := buildReviewInput(ctx, f, "svc", []string{"p/f.go"}, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.GraphBoth == nil || a.GraphBoth != b.GraphBoth || &a.Nodes[0] != &b.Nodes[0] {
		t.Fatal("review did not reuse its complete immutable graph and snapshot")
	}
	if f.loads.Load() != 1 || f.typedLoads.Load() != 1 {
		t.Fatalf("loads=%d typed=%d; want one each", f.loads.Load(), f.typedLoads.Load())
	}
}

func TestBuildReviewInputConcurrentCallsShareCompleteGraph(t *testing.T) {
	f := &reviewMemoFake{nodes: []core.SymbolNode{{Service: "svc", Package: "p", Name: "F", Kind: core.KindGoFunc, FilePath: "p/f.go"}}}
	var wg sync.WaitGroup
	graphs := make(chan any, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in, _, err := buildReviewInput(context.Background(), f, "svc", []string{"p/f.go"}, "", "", nil)
			if err != nil {
				t.Errorf("review input: %v", err)
				return
			}
			graphs <- in.GraphBoth
		}()
	}
	wg.Wait()
	close(graphs)
	var first any
	for g := range graphs {
		if first == nil {
			first = g
		} else if first != g {
			t.Fatal("concurrent reviews built distinct graphs")
		}
	}
	if f.loads.Load() != 1 || f.typedLoads.Load() != 1 {
		t.Fatalf("concurrent loads=%d typed=%d; want one each", f.loads.Load(), f.typedLoads.Load())
	}
}

func TestBuildReviewInputRefusesIncompleteGraphInputs(t *testing.T) {
	for _, tc := range []struct {
		name              string
		listErr, typedErr error
	}{
		{"symbols", errors.New("symbols unavailable"), nil},
		{"typed", nil, errors.New("typed unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &reviewMemoFake{nodes: []core.SymbolNode{{Service: "svc", Package: "p", Name: "F", FilePath: "p/f.go"}}, listErr: tc.listErr, typedErr: tc.typedErr}
			in, _, rpcErr := buildReviewInput(context.Background(), f, "svc", []string{"p/f.go"}, "", "", nil)
			if rpcErr == nil || !strings.Contains(rpcErr.Message, "unavailable") || in.GraphBoth != nil {
				t.Fatalf("incomplete input was accepted: graph=%v error=%v", in.GraphBoth, rpcErr)
			}
		})
	}
}

func TestBuildReviewInputRefusesCancelledRead(t *testing.T) {
	f := &reviewMemoFake{nodes: []core.SymbolNode{{Service: "svc", Package: "p", Name: "F", FilePath: "p/f.go"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in, _, err := buildReviewInput(ctx, f, "svc", []string{"p/f.go"}, "", "", nil)
	if err == nil || in.GraphBoth != nil {
		t.Fatalf("cancelled read accepted: graph=%v error=%v", in.GraphBoth, err)
	}
}

func TestReviewInputIdentityRefusesPatchWatermarkAndSuppressionDrift(t *testing.T) {
	for _, change := range []string{"patch", "watermark", "suppression"} {
		t.Run(change, func(t *testing.T) {
			f := &reviewMemoFake{nodes: []core.SymbolNode{{Service: "svc", Package: "p", Name: "F", Kind: core.KindGoFunc, FilePath: "p/f.go"}}}
			_, rc, err := buildReviewInput(context.Background(), f, "svc", []string{"p/f.go"}, "", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "patch":
				memo.PatchPackage(rc.indexScope, "p", []core.SymbolNode{{Service: "svc", Package: "p", Name: "New", Kind: core.KindGoFunc, FilePath: "p/f.go"}}, nil)
			case "watermark":
				f.mark.Store("w2")
			case "suppression":
				f.token.Store("hidden")
			}
			if rc.indexCurrent(context.Background(), f, "svc") || !rc.windowUnverified || (!strings.Contains(rc.partial, "index identity changed") && !strings.Contains(rc.partial, "graph publication changed")) {
				t.Fatalf("%s drift accepted: %+v", change, rc)
			}
		})
	}
}

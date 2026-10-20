package mcp

// cold_resolve_test.go pins O1446's open half: symbol_context resolves the
// name before it waits on the call graph, and a service with no build
// measured since the process started answers the cold partial at once.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// coldAnswerBound is how long a call that must not wait for the call graph
// may take: the one snapshot hold (memo.NavKeepAfter plus a margin) with
// room for -race. The waits these tests rule out run to the deadline less
// its reserve, 10 s under the 12 s deadline they use.
const coldAnswerBound = 5 * time.Second

// markedBackend is a slowSnapshotBackend whose watermark the test moves, as
// an ingest of the service does (the memo's scope moves with it, its
// measured build costs stay).
type markedBackend struct {
	*slowSnapshotBackend
	markMu sync.Mutex
	mark   string
}

func (b *markedBackend) StoreWatermark(context.Context, string) (string, error) {
	b.markMu.Lock()
	defer b.markMu.Unlock()
	return b.mark, nil
}

func (b *markedBackend) setMark(m string) {
	b.markMu.Lock()
	defer b.markMu.Unlock()
	b.mark = m
}

// gateCloser closes a slowSnapshotBackend's gate once, at the end of the
// test at the latest, so no gated load outlives it.
func gateCloser(t *testing.T, b *slowSnapshotBackend) func() {
	t.Helper()
	var once sync.Once
	b.mu.Lock()
	gate := b.gate
	b.mu.Unlock()
	closeGate := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(closeGate)
	return closeGate
}

// TestSymbolContextAnswersAmbiguousNameWithoutTheGraph pins O1446's name
// half: on a service whose call graph is cold and whose measured build fits
// the deadline, a name with more than one definition answers its
// candidates and a name with none answers not_found, both at once from the
// store lookup find_symbol uses; neither waits for a graph its answer never
// reads. Only a unique name waits for the graph and answers in full.
func TestSymbolContextAnswersAmbiguousNameWithoutTheGraph(t *testing.T) {
	b := newSlowSnapshotBackend(t)
	b.nodes = append(b.nodes, core.SymbolNode{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/tool/main.go", Line: 3, EndLine: 3, Tier: core.TierVerified})
	b.gate = nil
	mb := &markedBackend{slowSnapshotBackend: b, mark: "w1"}
	srv := NewServer(mb)

	// One full build at w1 measures the service's build cost: it is small,
	// so at w2 the build fits any deadline and the graph path would wait.
	if warm := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Run"}); warm["partial"] != nil {
		t.Fatalf("the measuring build answered partial: %+v", warm)
	}
	mb.setMark("w2")
	b.mu.Lock()
	b.gate = make(chan struct{})
	b.mu.Unlock()
	closeGate := gateCloser(t, b)

	call := func(symbol string) (map[string]any, time.Duration) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		start := time.Now()
		got := callToolWithin(t, ctx, srv, "symbol_context", map[string]any{"service": "svc", "symbol": symbol})
		return got, time.Since(start)
	}

	amb, took := call("main")
	if took > coldAnswerBound {
		t.Errorf("an ambiguous name waited %v for a call graph its answer never reads", took)
	}
	if amb["ambiguous"] != true || amb["count"] != float64(2) {
		t.Errorf("main = %+v, want the ambiguity answer with its 2 candidates", amb)
	}
	if _, ok := amb["partial"]; ok {
		t.Errorf("the ambiguity answer is complete, yet says partial: %+v", amb["partial"])
	}

	miss, took := call("zzzNoSuchSymbolQ")
	if took > coldAnswerBound {
		t.Errorf("an unknown name waited %v for a call graph its answer never reads", took)
	}
	if miss["not_found"] != true {
		t.Errorf("zzzNoSuchSymbolQ = %+v, want not_found", miss)
	}
	for _, k := range []string{"def", "callers", "callees", "related_tests"} {
		if _, ok := miss[k]; ok {
			t.Errorf("an unknown name carries %q: %+v", k, miss[k])
		}
	}

	time.AfterFunc(200*time.Millisecond, closeGate)
	full, _ := call("Run")
	if _, ok := full["partial"]; ok {
		t.Fatalf("a unique name with a build that fits answered partial instead of waiting for the graph: %+v", full)
	}
	if callers, _ := full["callers"].([]any); len(callers) == 0 {
		t.Errorf("callers = %#v, want main once the graph is built", full["callers"])
	}
}

// TestSymbolContextUnknownBuildCostAnswersPartialAtOnce pins O1446's
// restart half: a service with no build measured since the process started
// (memo.NavBuildCost falls back to ExpectedCost, which says nothing about a
// large service) is treated as a build longer than the deadline. The call
// answers the cold partial at once instead of waiting to the deadline's
// reserve, and the build it started keeps running for the next caller.
func TestSymbolContextUnknownBuildCostAnswersPartialAtOnce(t *testing.T) {
	b := newSlowSnapshotBackend(t)
	closeGate := gateCloser(t, b)
	srv := NewServer(b)

	// 12 s leaves a 10 s wait, past the summed ExpectedCost (9 s): the
	// unmeasured build "fits", the shape the deployed daemon waited out.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	start := time.Now()
	got := callToolWithin(t, ctx, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Run"})
	if took := time.Since(start); took > coldAnswerBound {
		t.Errorf("an unmeasured build was waited for %v instead of answering the partial at once", took)
	}
	p, _ := got["partial"].(string)
	if !strings.HasPrefix(p, "call graph not built (cold)") || !strings.Contains(p, "build time is unknown") || !strings.Contains(p, "the build keeps running") {
		t.Errorf("partial = %q, want the cold call graph, its build time unknown and its build kept", p)
	}
	if def, _ := got["def"].(map[string]any); def["name"] != "Run" {
		t.Errorf("def = %+v, want Run from the store lookup", got["def"])
	}
	if got["callers"] != notComputedColdGraph {
		t.Errorf("callers = %#v, want %q", got["callers"], notComputedColdGraph)
	}

	closeGate()
	if err := b.waitLoad(t); err != nil {
		t.Fatalf("the build the partial call started was cancelled (%v): the next caller pays for it again", err)
	}
	full := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Run"})
	if _, ok := full["partial"]; ok {
		t.Fatalf("the call after the build is still partial: %+v", full)
	}
}

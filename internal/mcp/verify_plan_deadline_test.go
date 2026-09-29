package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// coldGraphBackend serves a small service at a settable watermark. While
// gate is non-nil, TypedEdges blocks until it is closed: the cold graph
// build the first verify_plan after an ingest pays (O130).
type coldGraphBackend struct {
	QueryInterface
	storeID string
	nodes   []core.SymbolNode
	edges   []core.Edge

	mu         sync.Mutex
	mark       string
	gate       chan struct{}
	typedReads map[string]int // watermark -> TypedEdges calls
}

func (b *coldGraphBackend) StoreID() string { return b.storeID }

func (b *coldGraphBackend) StoreWatermark(context.Context, string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mark, nil
}

func (b *coldGraphBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return b.nodes, nil
}

func (b *coldGraphBackend) TypedEdges(ctx context.Context, _ string) ([]core.Edge, error) {
	b.mu.Lock()
	b.typedReads[b.mark]++
	gate := b.gate
	b.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return b.edges, nil
}

func (b *coldGraphBackend) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b *coldGraphBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range b.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

// TestVerifyPlanFirstCallAfterIngestAnswers pins EW-22 (O130): the first
// verify_plan after the watermark moves answers within its deadline even
// when the graph it needs is still building — a named partial with every
// row elided, never "abandoned: the call's deadline passed" — and the build
// it started is kept, so the next call is served from it instead of
// starting over.
func TestVerifyPlanFirstCallAfterIngestAnswers(t *testing.T) {
	b := &coldGraphBackend{
		storeID: "ew22-" + t.Name(),
		mark:    "w1",
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/app/main.go", Line: 1, Calls: []string{"work.Run"}, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Run", Kind: core.KindGoFunc, FilePath: "work/run.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestRun", Kind: core.KindGoFunc, FilePath: "work/run_test.go", Line: 1, Calls: []string{"Run"}, Tier: core.TierVerified},
		},
		edges:      []core.Edge{{FromService: "svc", FromFile: "cmd/app/main.go", FromSymbol: "svc::cmd/app/main.main", ToSymbol: "svc::work/work.Run", Kind: core.EdgeDirect}},
		typedReads: map[string]int{},
	}
	srv := NewServer(b)
	budget, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	srv.SetGate(tenancy.NewGate(tenancy.GateConfig{Budget: budget, Mode: tenancy.ModeEnforce, InFlight: tenancy.NewInFlight(nil),
		Level: func() tenancy.Level { return tenancy.LevelNormal }}))

	call := func(ctx context.Context) rpcResp {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "verify_plan", "arguments": map[string]any{
				"service": "svc",
				"tasks":   []any{map[string]any{"id": "R-1", "symbol": "Run", "claimed": true}},
			}}})
		resp, ok := srv.HandleRequest(ctx, raw)
		if !ok {
			t.Fatal("no response")
		}
		var out rpcResp
		if err := json.Unmarshal(resp, &out); err != nil {
			t.Fatalf("%v: %s", err, resp)
		}
		return out
	}
	payload := func(r rpcResp) map[string]any {
		t.Helper()
		if r.Error != nil {
			t.Fatalf("verify_plan error: %+v", r.Error)
		}
		content, _ := r.Result["content"].([]any)
		if len(content) == 0 {
			t.Fatalf("no content: %+v", r.Result)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	// A large safety bound, never a latency assertion: every call here
	// returns as soon as its answer is ready.
	const bound = 2 * time.Minute

	// Warm at w1.
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	if got := payload(call(ctx)); len(got["tasks"].([]any)) != 1 {
		t.Fatalf("warm call: %+v", got)
	}

	// An ingest moves the watermark; the typed edges the new graph needs
	// are slow to arrive.
	gate := make(chan struct{})
	b.mu.Lock()
	b.mark, b.gate = "w2", gate
	b.mu.Unlock()

	short, cancelShort := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShort()
	first := call(short)
	if first.Error != nil {
		t.Fatalf("the first call after the ingest failed instead of answering within its deadline: %+v", first.Error)
	}
	got := payload(first)
	if tasks, _ := got["tasks"].([]any); len(tasks) != 0 {
		t.Fatalf("no row can be verified before the graph exists, got %+v", tasks)
	}
	el, _ := got["elided"].(map[string]any)
	if el == nil || el["count"] != float64(1) {
		t.Fatalf("the unverified row must be elided and counted, got %+v", got)
	}
	if p, _ := got["partial"].(string); !strings.Contains(p, "still building") {
		t.Fatalf("the partial answer must name the build in flight, got %q", p)
	}

	// The build the first call started is kept: once its input arrives the
	// next call is answered from it, with no second read.
	close(gate)
	ctx2, cancel2 := context.WithTimeout(context.Background(), bound)
	defer cancel2()
	second := payload(call(ctx2))
	tasks, _ := second["tasks"].([]any)
	if len(tasks) != 1 || tasks[0].(map[string]any)["status"] != "pass" {
		t.Fatalf("second call: %+v", second)
	}
	b.mu.Lock()
	reads := b.typedReads["w2"]
	b.mu.Unlock()
	if reads != 1 {
		t.Fatalf("typed edges read %d times at w2, want 1: the first call's build was abandoned and started over", reads)
	}
}

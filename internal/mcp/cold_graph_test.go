package mcp

// cold_graph_test.go pins the answers symbol_context and search give on a
// cold service (O1446, O1447) and symbol_context's answer to a name it
// cannot resolve (O1451).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// slowSnapshotBackend is a cacheable backend (a store id and a watermark)
// whose symbol listing blocks until gate is closed: a service whose
// snapshot and call graph are not resident and take long to build.
type slowSnapshotBackend struct {
	QueryInterface
	storeID string
	root    string
	nodes   []core.SymbolNode

	mu       sync.Mutex
	gate     chan struct{}
	loads    int
	loadDone chan struct{} // closed when the first gated listing returns
	loadErr  error         // the first gated listing's ctx error when it returned
}

func (b *slowSnapshotBackend) StoreID() string { return b.storeID }

func (b *slowSnapshotBackend) StoreWatermark(context.Context, string) (string, error) {
	return "w1", nil
}

func (b *slowSnapshotBackend) ListAllSymbols(ctx context.Context, _ string) ([]core.SymbolNode, error) {
	b.mu.Lock()
	gate := b.gate
	b.loads++
	first := b.loads == 1
	b.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	if first && b.loadDone != nil {
		b.mu.Lock()
		b.loadErr = ctx.Err()
		b.mu.Unlock()
		defer close(b.loadDone)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b.nodes, nil
}

func (b *slowSnapshotBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range b.nodes {
		if n.Name == name || strings.HasSuffix(name, "."+n.Name) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (b *slowSnapshotBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	return nil, nil
}

func (b *slowSnapshotBackend) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b *slowSnapshotBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return b.root, "c0ffee", true, nil
}

func newSlowSnapshotBackend(t *testing.T) *slowSnapshotBackend {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"work/run.go":      "package work\n\nfunc Run() {\n\t// Needle\n}\n",
		"work/run_test.go": "package work\n\nfunc TestRun() { Run() }\n",
		"cmd/app/main.go":  "package main\n\nfunc main() { work.Run() }\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &slowSnapshotBackend{
		storeID: "cold-" + t.Name(),
		root:    root,
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/app/main.go", Line: 3, EndLine: 3, Calls: []string{"work.Run"}, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Run", Kind: core.KindGoFunc, FilePath: "work/run.go", Line: 3, EndLine: 5, Signature: "func Run()", Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestRun", Kind: core.KindGoFunc, FilePath: "work/run_test.go", Line: 3, EndLine: 3, Calls: []string{"Run"}, Tier: core.TierVerified},
		},
		gate:     make(chan struct{}),
		loadDone: make(chan struct{}),
	}
}

// callToolWithin calls a tool under ctx and returns its decoded payload,
// failing on an rpc error.
func callToolWithin(t *testing.T, ctx context.Context, srv *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args}})
	resp, ok := srv.HandleRequest(ctx, raw)
	if !ok {
		t.Fatalf("%s: no response", name)
	}
	var out rpcResp
	if err := json.Unmarshal(resp, &out); err != nil {
		t.Fatalf("%v: %s", err, resp)
	}
	if out.Error != nil {
		t.Fatalf("%s: rpc error: %+v", name, out.Error)
	}
	content, _ := out.Result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("%s: no content: %+v", name, out.Result)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// waitLoad waits for the gated listing to return and reports whether its
// flight was cancelled (the build a later caller needs).
func (b *slowSnapshotBackend) waitLoad(t *testing.T) error {
	t.Helper()
	select {
	case <-b.loadDone:
	case <-time.After(time.Minute):
		t.Fatal("the snapshot load never returned")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loadErr
}

// TestSymbolContextColdGraphAnswersPartial pins O1446's partial answer: a
// symbol_context whose call graph is not resident and cannot be built
// inside the call's deadline answers the definition from the store lookup
// find_symbol uses, marks itself partial, lists callers, callees and tests
// as not computed (never empty), and leaves the build running for the next
// call, which answers the full neighbourhood.
func TestSymbolContextColdGraphAnswersPartial(t *testing.T) {
	b := newSlowSnapshotBackend(t)
	srv := NewServer(b)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	start := time.Now()
	got := callToolWithin(t, ctx, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Run"})
	if ctx.Err() != nil {
		t.Fatalf("answered only after the deadline (%v): %+v", time.Since(start), got)
	}
	if p, _ := got["partial"].(string); !strings.HasPrefix(p, "call graph not built (cold)") {
		t.Errorf("partial = %q, want it to name the cold call graph", p)
	}
	def, _ := got["def"].(map[string]any)
	if def["name"] != "Run" || def["file_path"] != "work/run.go" || def["signature"] != "func Run()" {
		t.Errorf("def = %+v, want Run's definition from the store lookup", def)
	}
	for _, k := range []string{"callers", "callees", "related_tests"} {
		if got[k] != "not computed (cold graph)" {
			t.Errorf("%s = %#v, want \"not computed (cold graph)\"", k, got[k])
		}
	}
	if _, ok := got["resolution"]; ok {
		t.Errorf("a partial answer carries a resolution mix over edges it never computed: %+v", got["resolution"])
	}
	if got["index_commit"] != "c0ffee" {
		t.Errorf("index_commit = %#v, want the store's commit", got["index_commit"])
	}

	close(b.gate)
	if err := b.waitLoad(t); err != nil {
		t.Fatalf("the snapshot build the partial call joined was cancelled (%v): the next caller pays for it again", err)
	}
	full := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Run"})
	if _, ok := full["partial"]; ok {
		t.Fatalf("the call after the build is still partial: %+v", full)
	}
	callers, _ := full["callers"].([]any)
	if len(callers) == 0 {
		t.Errorf("callers = %#v, want main once the graph is built", full["callers"])
	}
	b.mu.Lock()
	loads := b.loads
	b.mu.Unlock()
	if loads != 1 {
		t.Errorf("the snapshot was listed %d times, want once: the partial call's build serves the next call", loads)
	}
}

// TestSearchColdServiceAnswersWithoutSnapshot pins O1447: a service search
// whose symbol snapshot is not resident answers ripgrep's hits without
// waiting for it, omits sym/kind and says why; the load it started is kept,
// and a later search carries the receipts.
func TestSearchColdServiceAnswersWithoutSnapshot(t *testing.T) {
	b := newSlowSnapshotBackend(t)
	srv := NewServer(b)

	// A safety bound only: the gated listing never returns before the gate
	// closes, so a search that waited for it would hit this.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got := callToolWithin(t, ctx, srv, "search", map[string]any{"service": "svc", "pattern": "Needle"})
	if ctx.Err() != nil {
		t.Fatal("search waited for the symbol snapshot")
	}
	hits, _ := got["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("hits = %+v, want the one Needle line", got["hits"])
	}
	if h := hits[0].(map[string]any); h["sym"] != nil || h["kind"] != nil {
		t.Errorf("hit = %+v, want no receipt while the snapshot loads", h)
	}
	if note, _ := got["receipts_omitted"].(string); !strings.Contains(note, "loading") {
		t.Errorf("receipts_omitted = %q, want it to say the index is loading", note)
	}

	close(b.gate)
	if err := b.waitLoad(t); err != nil {
		t.Fatalf("the snapshot load the search started was cancelled (%v)", err)
	}
	warm := callToolWithin(t, context.Background(), srv, "search", map[string]any{"service": "svc", "pattern": "Needle"})
	hits, _ = warm["hits"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["sym"] != "Run" {
		t.Errorf("warm hits = %+v, want the receipt sym Run", warm["hits"])
	}
	if _, ok := warm["receipts_omitted"]; ok {
		t.Errorf("a warm search still says receipts were omitted: %+v", warm)
	}
}

// TestSymbolContextUnresolvedNamesCandidates pins O1451: a name that
// resolves to nothing answers not_found with the nearest find_symbol
// candidates and no empty callers/callees/tests, and the repo-relative key
// `dir/file.Name` an ambiguous answer lists resolves within the service.
func TestSymbolContextUnresolvedNamesCandidates(t *testing.T) {
	b := newSlowSnapshotBackend(t)
	b.gate = nil
	srv := NewServer(b)

	miss := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "run"})
	if miss["not_found"] != true {
		t.Fatalf("not_found = %#v, want true: %+v", miss["not_found"], miss)
	}
	for _, k := range []string{"def", "callers", "callees", "related_tests"} {
		if _, ok := miss[k]; ok {
			t.Errorf("an unresolved name carries %q: %+v", k, miss[k])
		}
	}
	cands, _ := miss["candidates"].([]any)
	if len(cands) == 0 || cands[0].(map[string]any)["symbol_key"] != "svc::work/work.Run" {
		t.Errorf("candidates = %+v, want Run, the case-insensitive match", miss["candidates"])
	}

	none := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "zzzNoSuchSymbolQ"})
	if none["not_found"] != true {
		t.Errorf("zzzNoSuchSymbolQ: not_found = %#v, want true", none["not_found"])
	}
	if _, ok := none["def"]; ok {
		t.Errorf("zzzNoSuchSymbolQ carries def: %+v", none)
	}

	for _, q := range []string{"work/work.Run", "svc::work/work.Run"} {
		got := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": q})
		def, _ := got["def"].(map[string]any)
		if def["name"] != "Run" {
			t.Errorf("%s: def = %+v, want Run", q, got["def"])
		}
		if callers, _ := got["callers"].([]any); len(callers) == 0 {
			t.Errorf("%s: callers = %#v, want main", q, got["callers"])
		}
	}
}

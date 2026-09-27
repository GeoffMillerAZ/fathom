package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// rootedReferencesBackend gives each fake service its own root on disk.
type rootedReferencesBackend struct {
	*referencesFakeBackend
	roots map[string]string
}

func (r *rootedReferencesBackend) ServiceMeta(_ context.Context, svc string) (string, string, bool, error) {
	root, ok := r.roots[svc]
	return root, "", ok, nil
}

func newRootedReferencesSrv(t *testing.T) (*mcp.Server, map[string]string) {
	t.Helper()
	base := t.TempDir()
	roots := map[string]string{"svcA": filepath.Join(base, "a"), "svcB": filepath.Join(base, "b")}
	for _, r := range roots {
		if err := os.MkdirAll(filepath.Join(r, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	be := referencesBackendForTest()
	return mcp.NewServer(&rootedReferencesBackend{referencesFakeBackend: be, roots: roots}), roots
}

// callRefs calls references under ctx and returns (payload, error object).
func callRefs(t *testing.T, ctx context.Context, srv *mcp.Server, args map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	in, out := &bytes.Buffer{}, &bytes.Buffer{}
	sendLine(t, in, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "references", "arguments": args}})
	_ = srv.Serve(ctx, in, out)
	var resp map[string]any
	if err := json.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e, ok := resp["error"].(map[string]any); ok {
		return nil, e
	}
	return extractPayload(t, resp), nil
}

func occServices(p map[string]any) map[string]bool {
	out := map[string]bool{}
	if s, ok := p["occurrence_service"].(string); ok {
		out[s] = true
	}
	occs, _ := p["occurrences"].([]any)
	for _, o := range occs {
		if s, ok := o.(map[string]any)["svc"].(string); ok {
			out[s] = true
		}
	}
	return out
}

// RT-86 (§4.5, Breaking): with no `services`, references searches the
// caller's own service — the one whose root holds the caller's cwd — and
// `services: ["*"]` is the explicit fleet opt-in.
func TestReferences_DefaultsToOwnServiceFleetOptIn(t *testing.T) {
	srv, roots := newRootedReferencesSrv(t)
	ctx := mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1, Cwd: filepath.Join(roots["svcB"], "sub")})

	p, errObj := callRefs(t, ctx, srv, map[string]any{"query": "FindMe"})
	if errObj != nil {
		t.Fatalf("references with a resolvable caller refused: %v", errObj)
	}
	if p["scope"] != "caller-service" {
		t.Fatalf("scope = %v, want caller-service", p["scope"])
	}
	if got := occServices(p); len(got) != 1 || !got["svcB"] {
		t.Fatalf("occurrence services = %v, want only the caller's svcB", got)
	}

	p, errObj = callRefs(t, ctx, srv, map[string]any{"query": "FindMe", "services": []any{"*"}})
	if errObj != nil {
		t.Fatalf("fleet references refused: %v", errObj)
	}
	if p["scope"] != "fleet" {
		t.Fatalf("scope = %v, want fleet", p["scope"])
	}
	if got := occServices(p); !got["svcA"] || !got["svcB"] {
		t.Fatalf("fleet occurrence services = %v, want svcA and svcB", got)
	}
	if got := srv.CostClassOfForTest("references", `{"query":"x","services":["*"]}`); got != "fleet" {
		t.Fatalf(`services ["*"] class = %s, want fleet`, got)
	}
}

// RT-159 (§4.5, Breaking): no `services` and no resolvable caller identity
// (an HTTP agent sends no cwd) is a named -32602 listing the indexed
// services and the ["*"] opt-in — never a fleet scan, never an empty result.
func TestReferences_NoIdentityNamesChoices(t *testing.T) {
	srv, _ := newRootedReferencesSrv(t)
	for name, ctx := range map[string]context.Context{
		"no caller":          context.Background(),
		"caller without cwd": mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1}),
		"cwd outside roots":  mcp.WithCaller(context.Background(), mcp.CallerInfo{PID: 1, Cwd: t.TempDir()}),
	} {
		p, errObj := callRefs(t, ctx, srv, map[string]any{"query": "FindMe"})
		if errObj == nil {
			t.Fatalf("%s: references answered %v, want a -32602 naming the choices", name, p)
		}
		if code, _ := errObj["code"].(float64); code != -32602 {
			t.Fatalf("%s: code = %v, want -32602", name, errObj["code"])
		}
		msg, _ := errObj["message"].(string)
		for _, want := range []string{"svcA", "svcB", `services: ["*"]`} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: message %q does not name %q", name, msg, want)
			}
		}
	}
}

// budgetStoppedBackend reports every scan as stopped at its byte budget.
type budgetStoppedBackend struct {
	*rootedReferencesBackend
	budgets []corpus.Budget
}

func (b *budgetStoppedBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	b.budgets = append(b.budgets, bud)
	cov, err := b.rootedReferencesBackend.ScanSources(ctx, svc, bud, fn)
	cov.Total++
	cov.Stopped = corpus.StopBytes
	return cov, err
}

// RT-86: the text scan runs under a byte and wall budget, and a scan the
// budget stopped is a named lower bound; the services it never reached are
// counted, not silently dropped.
func TestReferences_BudgetedScanNamesPartial(t *testing.T) {
	_, roots := newRootedReferencesSrv(t)
	be := &budgetStoppedBackend{rootedReferencesBackend: &rootedReferencesBackend{referencesFakeBackend: referencesBackendForTest(), roots: roots}}
	srv := mcp.NewServer(be)
	p, errObj := callRefs(t, context.Background(), srv, map[string]any{"query": "FindMe", "services": []any{"*"}})
	if errObj != nil {
		t.Fatal(errObj)
	}
	if len(be.budgets) != 1 || be.budgets[0].Bytes <= 0 || be.budgets[0].Wall <= 0 {
		t.Fatalf("scan budgets = %+v, want one bounded scan (the second service is never reached)", be.budgets)
	}
	partial, _ := p["partial"].(string)
	for _, want := range []string{"≥ 2 occurrences", "budget bytes", "1 service(s) not scanned"} {
		if !strings.Contains(partial, want) {
			t.Fatalf("partial = %q, want it to contain %q", partial, want)
		}
	}
}

// costCountingRefsBackend counts, per service, the whole-service reads a
// references call makes (symbol lists, typed edges, named-file reads), and
// names its store and watermark so the shared memo can hold them.
type costCountingRefsBackend struct {
	*budgetStoppedBackend
	storeID                 string
	mu                      sync.Mutex
	lists, typed, readFiles map[string]int
	readBytes               int
	iface                   []byte
}

func newCostCountingRefsBackend(t *testing.T) *costCountingRefsBackend {
	_, roots := newRootedReferencesSrv(t)
	inner := referencesBackendForTest()
	// svcA declares a Go interface (the RC-21 candidate a bare query reads)
	// and has typed edges; svcB, never scanned, has both too.
	inner.nodes = append(inner.nodes,
		core.SymbolNode{Service: "svcA", Package: "alpha", Name: "Closer", Kind: core.KindGoInterface, FilePath: "pkg/iface.go", Line: 3, Tier: core.TierVerified},
		core.SymbolNode{Service: "svcB", Package: "beta", Name: "FindMe", Kind: core.KindGoFunc, FilePath: "b.go", Line: 1, Tier: core.TierVerified},
		core.SymbolNode{Service: "svcB", Package: "beta", Name: "Opener", Kind: core.KindGoInterface, FilePath: "b/iface.go", Line: 1, Tier: core.TierVerified},
	)
	inner.typedEdges = []core.Edge{{FromService: "svcA", FromFile: "pkg/other.go", FromSymbol: "svcA::alpha.Other", ToSymbol: "svcA::alpha.FindMe", Kind: core.EdgeDirect}}
	return &costCountingRefsBackend{
		budgetStoppedBackend: &budgetStoppedBackend{rootedReferencesBackend: &rootedReferencesBackend{referencesFakeBackend: inner, roots: roots}},
		storeID:              "refs-cost-" + t.Name(),
		lists:                map[string]int{}, typed: map[string]int{}, readFiles: map[string]int{},
		iface: []byte("package alpha\n\ntype Closer interface {\n\tFindMe() error\n}\n" + strings.Repeat("// pad\n", 1000)),
	}
}

func (c *costCountingRefsBackend) StoreID() string { return c.storeID }
func (c *costCountingRefsBackend) StoreWatermark(context.Context, string) (string, error) {
	return "wm1", nil
}

func (c *costCountingRefsBackend) ListAllSymbols(_ context.Context, svc string) ([]core.SymbolNode, error) {
	c.mu.Lock()
	c.lists[svc]++
	c.mu.Unlock()
	var out []core.SymbolNode
	for _, n := range c.nodes {
		if n.Service == svc {
			out = append(out, n)
		}
	}
	return out, nil
}

func (c *costCountingRefsBackend) FindSymbol(_ context.Context, svc, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range c.nodes {
		if n.Service == svc && n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (c *costCountingRefsBackend) TypedEdges(_ context.Context, svc string) ([]core.Edge, error) {
	c.mu.Lock()
	c.typed[svc]++
	c.mu.Unlock()
	var out []core.Edge
	for _, e := range c.typedEdges {
		if e.FromService == svc {
			out = append(out, e)
		}
	}
	return out, nil
}

func (c *costCountingRefsBackend) ReadSourceFiles(_ context.Context, svc string, paths []string) ([]core.SourceFile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readFiles[svc]++
	var out []core.SourceFile
	for _, p := range paths {
		out = append(out, core.SourceFile{Path: p, Content: c.iface, Lang: "go"})
		c.readBytes += len(c.iface)
	}
	return out, nil
}

// RT-86 (§4.5): a service the budget never scans costs the call nothing
// but its definitions' point lookup — no symbol list, no typed edges, no
// interface files — and the scanned service's symbols and typed edges come
// from the watermark's memo, not a per-call load.
func TestReferences_UnscannedServicesCostNothing(t *testing.T) {
	be := newCostCountingRefsBackend(t)
	srv := mcp.NewServer(be)
	for i := 0; i < 2; i++ {
		p, errObj := callRefs(t, context.Background(), srv, map[string]any{"query": "FindMe", "services": []any{"*"}})
		if errObj != nil {
			t.Fatal(errObj)
		}
		// svcB's definition still answers, from the point lookup.
		defs, _ := p["definitions"].([]any)
		var sawB bool
		for _, d := range defs {
			if d.(map[string]any)["service"] == "svcB" {
				sawB = true
			}
		}
		if !sawB {
			t.Fatalf("call %d: definitions %v lack svcB's FindMe", i, defs)
		}
	}
	be.mu.Lock()
	defer be.mu.Unlock()
	if be.lists["svcB"] != 0 || be.typed["svcB"] != 0 || be.readFiles["svcB"] != 0 {
		t.Fatalf("unscanned svcB: lists=%d typed=%d readFiles=%d, want none", be.lists["svcB"], be.typed["svcB"], be.readFiles["svcB"])
	}
	if be.lists["svcA"] != 1 || be.typed["svcA"] != 1 {
		t.Fatalf("scanned svcA over two calls at one watermark: lists=%d typed=%d, want 1 each (memoized)", be.lists["svcA"], be.typed["svcA"])
	}
}

// RT-86 (§4.5): the interface files a bare query reads count against the
// call's one byte budget, so the budget bounds every read the call makes.
func TestReferences_InterfaceFilesChargeTheBudget(t *testing.T) {
	be := newCostCountingRefsBackend(t)
	restore := mcp.SetReferencesScanBytesForTest(int64(len(be.iface)))
	defer restore()
	srv := mcp.NewServer(be)
	p, errObj := callRefs(t, context.Background(), srv, map[string]any{"query": "FindMe", "services": []any{"svcA"}})
	if errObj != nil {
		t.Fatal(errObj)
	}
	if len(be.budgets) != 0 {
		t.Fatalf("text scans = %+v, want none once the interface file spent the byte budget", be.budgets)
	}
	if partial, _ := p["partial"].(string); !strings.Contains(partial, "budget bytes") {
		t.Fatalf("partial = %q, want the byte budget named", partial)
	}
}

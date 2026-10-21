package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// The normal ingest's IndexedAt watermark changes even when only the CUE pass
// epoch changed. Exercise the same snapshot/typed graph memo used by MCP so a
// deleted policy-denied row cannot survive in a warm graph at an unchanged tree.
func TestCUEPolicyEpochIngestRefreshesWarmTypedGraph(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("cue.mod/module.cue", "module: \"example.org/policy@v0\"\nlanguage: version: \"v0.16.1\"\n")
	write("schema/schema.cue", "package schema\n#Base: string\n")
	write("consumer/main.cue", "package consumer\nimport \"example.org/policy/schema\"\n#Copy: schema.#Base\n")
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	c := New(conn, st)
	const service = "cue-policy-memo"
	t.Cleanup(func() { memo.EvictStoreService(c.StoreID(), service) })
	svc := ServiceRef{ID: service, Path: root}
	if report, err := c.Ingest(ctx, svc); err != nil || report.CUEEdgesDirect != 1 {
		t.Fatalf("initial real local typed edge missing: report=%+v err=%v", report, err)
	}

	write("cue.mod/module.cue", "module: \"example.org/policy@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n")
	write("consumer/main.cue", "package consumer\nimport \"example.org/remote/pkg\"\n#Copy: pkg.#Name\n")
	// Reproduce a pre-policy store: current syntax indexed without a native
	// pass, while prior CUE typed rows and an old completion stamp survive.
	c.pipe = fusion.New(conn, st, fusion.Config{DisableTypedEdges: true})
	if _, err := c.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTypedEdgePackageFingerprint(ctx, service, "typed-pass-state:cue", "ok"); err != nil {
		t.Fatal(err)
	}
	c.pipe = fusion.New(conn, st, fusion.Config{})
	from := core.SymbolKey(service, "consumer/consumer", "#Copy")
	to := core.SymbolKey(service, "schema/schema", "#Base")
	graph := func() (string, *callgraph.Graph) {
		t.Helper()
		watermark, err := c.StoreWatermark(ctx, service)
		if err != nil || watermark == "" {
			t.Fatalf("store watermark unavailable: %q %v", watermark, err)
		}
		snapshot, err := memo.SnapshotFor(ctx, memo.Scope{Store: c.StoreID(), Service: service, Watermark: watermark}, func(ctx context.Context) ([]core.SymbolNode, error) {
			return c.ListAllSymbols(ctx, service)
		})
		if err != nil {
			t.Fatal(err)
		}
		graph, err := memo.GraphFor(ctx, snapshot, memo.GraphResolved, func(ctx context.Context) ([]core.Edge, error) { return c.TypedEdges(ctx, service) })
		if err != nil {
			t.Fatal(err)
		}
		return watermark, graph
	}
	before, oldGraph := graph()
	if !callgraph.Reachable(oldGraph, from, to) {
		t.Fatal("fixture did not establish the old typed row in a warm memo")
	}
	if report, err := c.Ingest(ctx, svc); err != nil || report.Extracted != 0 || report.CUEPackagesFailed == 0 {
		t.Fatalf("epoch-only native retry did not run on unchanged syntax: report=%+v err=%v", report, err)
	}
	if state, _, err := st.TypedEdgePackageFingerprint(ctx, service, "typed-pass-state:cue"); err != nil || state != "partial@epoch-1" {
		t.Fatalf("synthetic loader failure was falsely stamped clean: state=%q err=%v", state, err)
	}
	after, freshGraph := graph()
	if before == after || callgraph.Reachable(freshGraph, from, to) {
		t.Fatalf("normal ingest served policy-denied typed evidence from its memo: before=%q after=%q reachable=%v", before, after, callgraph.Reachable(freshGraph, from, to))
	}
}

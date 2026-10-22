package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type reviewKeyCountingStore struct {
	store.Store
	symbolReads, typedReads int
}

func (s *reviewKeyCountingStore) ListSymbolNodes(ctx context.Context, service string) ([]core.SymbolNode, error) {
	s.symbolReads++
	return s.Store.ListSymbolNodes(ctx, service)
}

func (s *reviewKeyCountingStore) TypedEdges(ctx context.Context, service string) ([]core.Edge, error) {
	s.typedReads++
	return s.Store.TypedEdges(ctx, service)
}

func keyGraphMisses() int64 {
	for _, s := range memo.Stats() {
		if s.Name == "graphstore" {
			return s.Misses
		}
	}
	return 0
}

func TestReviewKeyReusesWarmResolvedGraph(t *testing.T) {
	ctx := context.Background()
	st := &reviewKeyCountingStore{Store: store.NewMemoryStore()}
	t.Cleanup(func() { _ = st.Close() })
	root := t.TempDir()
	certifyIngestWrite(t, root, "a.go", "package x\nfunc A() {}\n")
	certifyIngestWrite(t, root, "b.go", "package x\nfunc B() {}\n")
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "head", IndexedAt: now.UnixNano()}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "svc", Symbol: "A", Package: "x", SourceRef: "a.go", IngestedAt: now},
		{ServiceID: "svc", Symbol: "B", Package: "x", SourceRef: "b.go", IngestedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutTypedEdges(ctx, "svc", "x", []core.Edge{{FromSymbol: "svc::x.A", ToSymbol: "svc::x.B", Kind: core.EdgeDirect}}); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	req := reviewkey.Request{Service: "svc", Manifest: []reviewkey.ManifestEntry{{Path: "a.go", SHA256: "a"}}}
	want, err := c.ReviewKey(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	wm, err := c.StoreWatermark(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	sc := memo.Scope{Store: c.StoreID(), Service: "svc", Watermark: wm}
	snap, err := memo.SnapshotFor(ctx, sc, func(ctx context.Context) ([]core.SymbolNode, error) { return c.ListAllSymbols(ctx, "svc") })
	if err != nil {
		t.Fatal(err)
	}
	typed := func(ctx context.Context) ([]core.Edge, error) { return c.TypedEdges(ctx, "svc") }
	if _, err := memo.GraphFor(ctx, snap, memo.GraphResolved, typed); err != nil {
		t.Fatal(err)
	}
	before := keyGraphMisses()
	st.symbolReads, st.typedReads = 0, 0
	got, err := c.ReviewKey(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	if !bytes.Equal(a, b) {
		t.Fatalf("warm key bytes changed:\n%s\n%s", a, b)
	}
	if st.symbolReads != 0 || st.typedReads != 0 || keyGraphMisses() != before {
		t.Errorf("warm symbol reads=%d typed reads=%d graph builds=%d; want 0,0,0", st.symbolReads, st.typedReads, keyGraphMisses()-before)
	}
}

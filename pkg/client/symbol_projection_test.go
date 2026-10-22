package client_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type projectionStore struct {
	*store.MemoryStore
	nodes []core.SymbolNode
	err   error
}

func (s *projectionStore) ListSymbolNodes(context.Context, string) ([]core.SymbolNode, error) {
	return s.nodes, s.err
}

var clientProjectionSink []client.SymbolNode

func TestListAllSymbolsUsesProjectionAllocationBound(t *testing.T) {
	s := &projectionStore{MemoryStore: store.NewMemoryStore(), nodes: []core.SymbolNode{{Service: "svc", Name: "Run"}}}
	c := client.New(connector.NewLocalConnector(), s)
	t.Cleanup(func() { _ = c.Close() })
	if err := s.PutObservations(context.Background(), []core.Observation{{ServiceID: "svc", Symbol: "Legacy"}}); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(3, func() {
		var err error
		clientProjectionSink, err = c.ListAllSymbols(context.Background(), "svc")
		if err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 1 {
		t.Fatalf("projection dispatch allocations=%.0f, bound 1", allocs)
	}
	if !reflect.DeepEqual(clientProjectionSink, s.nodes) {
		t.Fatalf("projection was bypassed: %v", clientProjectionSink)
	}
	s.err = errors.New("projection failed")
	if _, err := c.ListAllSymbols(context.Background(), "svc"); !errors.Is(err, s.err) {
		t.Fatalf("projection error bypassed: %v", err)
	}
}

func TestListAllSymbolsProjectionAndFallbackParity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package sample\nfunc Run() {}\nfunc Caller() { Run() }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(context.Background(), "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	want := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		want = append(want, o.ToSymbolNode())
	}
	if len(want) == 0 {
		t.Fatal("generated repository has no symbols")
	}
	got, err := c.ListAllSymbols(context.Background(), "svc")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("sqlite projection: %v", err)
	}
	ms := store.NewMemoryStore()
	if err := ms.PutObservations(context.Background(), obs); err != nil {
		t.Fatal(err)
	}
	fallback := client.New(connector.NewLocalConnector(), ms)
	t.Cleanup(func() { _ = fallback.Close() })
	got, err = fallback.ListAllSymbols(context.Background(), "svc")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("memory fallback: %v", err)
	}
}

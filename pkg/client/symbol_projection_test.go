package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Embedding only Store deliberately hides SQLite's optional projection. This
// exercises the unchanged original QueryObservations -> ToSymbolNode path.
type observationOnlyStore struct{ store.Store }

type projectionCapabilityStore struct {
	store.Store
	nodes []core.SymbolNode
	err   error
}

func (s projectionCapabilityStore) ListSymbolNodes(context.Context, string) ([]core.SymbolNode, error) {
	return s.nodes, s.err
}

func TestListAllSymbolsProjectionCapabilityAndFallback(t *testing.T) {
	want := []core.SymbolNode{{Service: "svc", Name: "AllFields", Package: "p", FilePath: "a.go", Signature: "complete header", Calls: []string{"p.Target"}, Line: 7, EndLine: 11, Complexity: 3}}
	fast := &Client{st: projectionCapabilityStore{nodes: want}}
	got, err := fast.ListAllSymbols(context.Background(), "svc")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("capability result = %#v, %v", got, err)
	}
	sentinel := errors.New("projection terminal failure")
	fast.st = projectionCapabilityStore{nodes: want, err: sentinel}
	got, err = fast.ListAllSymbols(context.Background(), "svc")
	if !errors.Is(err, sentinel) || got != nil || err.Error() != "client.ListAllSymbols: projection terminal failure" {
		t.Fatalf("partial/failure escaped: %#v, %v", got, err)
	}
	memory := store.NewMemoryStore()
	obs := []core.Observation{{ServiceID: "svc", Symbol: "Fallback", SourceRef: "b.go"}}
	if err := memory.PutObservations(context.Background(), obs); err != nil {
		t.Fatal(err)
	}
	legacy := &Client{st: observationOnlyStore{memory}}
	got, err = legacy.ListAllSymbols(context.Background(), "svc")
	if err != nil || !reflect.DeepEqual(got, []core.SymbolNode{obs[0].ToSymbolNode()}) {
		t.Fatalf("Store fallback changed: %#v, %v", got, err)
	}
}

func symbolProjectionFixture(tb testing.TB) (*Client, *Client, int) {
	tb.Helper()
	s, err := store.NewSQLiteStore(filepath.Join(tb.TempDir(), "symbols.sqlite"))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = s.Close() })
	const count = 16384
	rows := make([]core.Observation, count)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := range rows {
		rows[i] = core.Observation{ServiceID: "svc", SourceRef: fmt.Sprintf("data/nested/%05d.json", i), SourceKind: core.SourceKind("JSONKey"), Symbol: fmt.Sprintf("Root.Section.Entry%05d", i), Package: "data", Claim: strings.Repeat("payload", 48), IngestedAt: now.Add(time.Duration(i%17) * time.Second)}
		if i%4 == 0 {
			rows[i].Calls = []string{"data.Target"}
		}
	}
	if err := s.PutObservations(context.Background(), rows); err != nil {
		tb.Fatal(err)
	}
	return &Client{st: s}, &Client{st: observationOnlyStore{s}}, count
}

func TestListAllSymbolsProjectionFullCorpusDifferential(t *testing.T) {
	fast, legacy, count := symbolProjectionFixture(t)
	a, err := fast.ListAllSymbols(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	b, err := legacy.ListAllSymbols(context.Background(), "svc")
	if err != nil || len(a) != count || !reflect.DeepEqual(a, b) {
		t.Fatalf("full corpus/order/fields changed: fast=%d legacy=%d err=%v", len(a), len(b), err)
	}
	var calls int
	for _, n := range a {
		calls += len(n.Calls)
	}
	if calls != count/4 {
		t.Fatalf("raw calls changed: %d", calls)
	}
}

var symbolProjectionSink []core.SymbolNode

func projectionAllocated(t *testing.T, client *Client, count int) uint64 {
	t.Helper()
	runtime.GC() // Measurement boundary only; production GC policy is unchanged.
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	nodes, err := client.ListAllSymbols(context.Background(), "svc")
	runtime.ReadMemStats(&after)
	if err != nil || len(nodes) != count {
		t.Fatalf("allocation workload changed: %d, %v", len(nodes), err)
	}
	symbolProjectionSink = nodes
	return after.TotalAlloc - before.TotalAlloc
}

func TestListAllSymbolsProjectionAllocationBound(t *testing.T) {
	fast, legacy, count := symbolProjectionFixture(t)
	for i := 0; i < 3; i++ {
		oldBytes := projectionAllocated(t, legacy, count)
		newBytes := projectionAllocated(t, fast, count)
		saved := int64(oldBytes) - int64(newBytes)
		t.Logf("sample=%d nodes=%d raw_calls=%d legacy_B=%d projected_B=%d saved_B=%d", i, count, count/4, oldBytes, newBytes, saved)
		// A falsifiable conservative hypothesis, not an observed saving.
		if saved < 1<<20 {
			t.Fatalf("projection did not save the fixed 1MiB floor: %d", saved)
		}
	}
}

func BenchmarkListAllSymbolsProjection(b *testing.B) {
	fast, legacy, count := symbolProjectionFixture(b)
	for _, item := range []struct {
		name   string
		client *Client
	}{{"legacy", legacy}, {"projected", fast}} {
		b.Run(item.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				nodes, err := item.client.ListAllSymbols(context.Background(), "svc")
				if err != nil || len(nodes) != count {
					b.Fatalf("incomplete workload: %d, %v", len(nodes), err)
				}
				symbolProjectionSink = nodes
			}
			b.ReportMetric(float64(count), "nodes/op")
			b.ReportMetric(float64(count/4), "raw-calls/op")
		})
	}
}

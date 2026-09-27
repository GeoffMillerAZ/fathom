package main

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// symbolLoadCountingStore counts whole-service observation loads and
// aggregate queries, and can hold the aggregate query open.
type symbolLoadCountingStore struct {
	*store.MemoryStore
	loads      atomic.Int64
	aggregates atomic.Int64
	gate       chan struct{}
}

func (s *symbolLoadCountingStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	s.loads.Add(1)
	return s.MemoryStore.QueryObservations(ctx, serviceID, concernID)
}

func (s *symbolLoadCountingStore) SourceKindCounts(ctx context.Context, serviceID string) ([]store.SourceKindCount, error) {
	s.aggregates.Add(1)
	if s.gate != nil {
		<-s.gate
	}
	return s.MemoryStore.SourceKindCounts(ctx, serviceID)
}

// RT-144 (docs/design/responsible-tenant.md §4.4): a service's symbol
// count and source kinds come from SQL aggregates, cached per watermark and
// single-flighted — never a whole-service symbol load (the profile charged
// 133.8 GB of allocation to that load on 2026-09-26). RT-79 moved
// list_services itself onto service_summary rows
// (TestServiceSummaries_ReadsSummaryRowsOnly); the aggregate still answers
// IsIndexed (RT-80).
func TestServiceSummaries_AggregatesNeverLoadSymbols(t *testing.T) {
	ctx := context.Background()
	st := &symbolLoadCountingStore{MemoryStore: store.NewMemoryStore()}
	now := time.Unix(1, 0)
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "demo", SourceRef: "a.go", SourceKind: core.KindGoFunc, Symbol: "A", IngestedAt: now},
		{ServiceID: "demo", SourceRef: "a.go", SourceKind: core.KindGoFunc, Symbol: "B", IngestedAt: now},
		{ServiceID: "demo", SourceRef: "b.py", SourceKind: core.KindPyFunc, Symbol: "c", IngestedAt: now},
		{ServiceID: "demo", SourceRef: "c.md", SourceKind: core.SourceKind("Markdown"), Symbol: "doc", IngestedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "demo", Path: "", IndexedCommit: "c1", IndexedAt: 1}); err != nil {
		t.Fatal(err)
	}
	fa := newTestFleetAdapter(newCountingConnector(), st)

	// Concurrent cold callers share one aggregate query.
	st.gate = make(chan struct{})
	const callers = 16
	var wg sync.WaitGroup
	sums := make([]client.SymbolAggregates, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sums[i], _ = fa.c.ServiceSymbolAggregates(ctx, "demo")
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	close(st.gate)
	wg.Wait()

	want := client.SymbolAggregates{Count: 4, Kinds: []core.SourceKind{core.KindGoFunc, core.SourceKind("Markdown"), core.KindPyFunc}}
	for i, got := range sums {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("caller %d: ServiceSummaries = %+v, want %+v", i, got, want)
		}
	}
	if n := st.loads.Load(); n != 0 {
		t.Fatalf("list_services loaded the whole symbol table %d time(s); it must answer from aggregates", n)
	}
	if n := st.aggregates.Load(); n != 1 {
		t.Fatalf("%d concurrent cold callers ran %d aggregate queries, want 1 (single-flight)", callers, n)
	}

	// Cached per watermark: a warm call runs no query; an ingest (a new
	// watermark) earns exactly one more.
	_, _ = fa.c.ServiceSymbolAggregates(ctx, "demo")
	if n := st.aggregates.Load(); n != 1 {
		t.Fatalf("a warm call re-ran the aggregate (%d queries)", n)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "demo", IndexedCommit: "c2", IndexedAt: 2}); err != nil {
		t.Fatal(err)
	}
	_, _ = fa.c.ServiceSymbolAggregates(ctx, "demo")
	if n := st.aggregates.Load(); n != 2 {
		t.Fatalf("after a new watermark, aggregate queries = %d, want 2", n)
	}
}

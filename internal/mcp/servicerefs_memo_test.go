package mcp

import (
	"context"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// refsCountingBackend serves stored service refs at a settable watermark
// and counts the store reads.
type refsCountingBackend struct {
	QueryInterface
	storeID string
	mu      sync.Mutex
	mark    string
	reads   map[string]int
}

func (b *refsCountingBackend) StoreID() string { return b.storeID }

func (b *refsCountingBackend) StoreWatermark(context.Context, string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mark, nil
}

func (b *refsCountingBackend) ServiceRefsFor(_ context.Context, svc string) (store.ServiceRefs, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads[svc]++
	if svc == "catalog" {
		return store.ServiceRefs{Routes: []store.ServiceRoute{{File: "c.go", Line: 1, Method: "GET", Path: "/items"}}}, true, nil
	}
	return store.ServiceRefs{Calls: []store.ServiceHTTPCall{{File: "k.go", Line: 2, Method: "GET", Template: "http://catalog/items"}}}, true, nil
}

// RT-89 (§6): the stored routes and client calls are read once per
// (store, service, watermark), not on every blast_radius / review_diff
// call; a new watermark, or a per-file refresh (which replaces one file's
// rows without moving the watermark), reads them again.
func TestServiceRefs_ReadPerWatermarkNotPerCall(t *testing.T) {
	b := &refsCountingBackend{storeID: "refs-memo-" + t.Name(), mark: "w1", reads: map[string]int{}}
	svcs := []string{"catalog", "checkout"}
	hops := func() int {
		h, _ := fleetServiceRefHops(context.Background(), b, svcs)
		return len(h)
	}
	first := hops()
	if again := hops(); again != first {
		t.Fatalf("hops %d then %d", first, again)
	}
	if b.reads["catalog"] != 1 || b.reads["checkout"] != 1 {
		t.Fatalf("reads after two calls at one watermark = %v, want 1 each", b.reads)
	}
	b.mu.Lock()
	b.mark = "w2"
	b.mu.Unlock()
	hops()
	if b.reads["catalog"] != 2 {
		t.Fatalf("reads after a watermark bump = %v, want catalog read again", b.reads)
	}
	dropServiceRefs(b, "catalog")
	hops()
	if b.reads["catalog"] != 3 || b.reads["checkout"] != 2 {
		t.Fatalf("reads after dropping catalog's refs = %v, want only catalog read again", b.reads)
	}
}

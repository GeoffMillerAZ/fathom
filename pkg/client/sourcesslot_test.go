package client_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// gatedConnector blocks Discover until gate closes and records the peak
// number of concurrent Discover calls.
type gatedConnector struct {
	gate    chan struct{}
	inside  atomic.Int64
	peak    atomic.Int64
	entered chan struct{}
}

func (g *gatedConnector) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	n := g.inside.Add(1)
	for {
		p := g.peak.Load()
		if n <= p || g.peak.CompareAndSwap(p, n) {
			break
		}
	}
	g.entered <- struct{}{}
	<-g.gate
	g.inside.Add(-1)
	return []core.SourceRef{{Service: svc.ID, Path: "a.go"}}, nil
}

func (g *gatedConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	return []byte("package a\n"), nil
}

func (g *gatedConnector) Watermark(context.Context, core.ServiceRef) (string, error) {
	return "", nil
}

// RT-146 (docs/design/responsible-tenant.md §4.6): at most two
// client.Sources whole-tree reads run at once process-wide; a hook-origin
// caller never waits for a slot and gets a named skip, while any other
// caller waits (bounded by its own ctx).
func TestSourcesSlotHooksNeverWait(t *testing.T) {
	conn := &gatedConnector{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	c := client.New(conn, store.NewMemoryStore())
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Sources(ctx, "svc"); err != nil {
				t.Errorf("holder Sources: %v", err)
			}
		}()
	}
	<-conn.entered
	<-conn.entered

	// A hook never waits: an immediate, named skip.
	start := time.Now()
	_, err := c.Sources(client.WithHookOrigin(ctx), "svc")
	if !errors.Is(err, client.ErrSourcesBusy) {
		t.Fatalf("hook-origin Sources with both slots held = %v, want ErrSourcesBusy", err)
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("a hook-origin caller waited %v for a slot", d)
	}
	if want := "coverage skipped: whole-tree read in progress"; err.Error() != want {
		t.Fatalf("skip text = %q, want %q", err.Error(), want)
	}

	// Anyone else waits, and gives up only when its own ctx does.
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := c.Sources(short, "svc"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a third Sources with both slots held = %v, want it to wait until its deadline", err)
	}
	if p := conn.peak.Load(); p != 2 {
		t.Fatalf("peak concurrent whole-tree reads = %d, want 2", p)
	}

	close(conn.gate)
	wg.Wait()
	// Slots released: a hook-origin read now runs.
	files, err := c.Sources(client.WithHookOrigin(ctx), "svc")
	if err != nil || len(files) != 1 {
		t.Fatalf("hook Sources after release = %d files, %v", len(files), err)
	}
}

// RT-146 × RT-143/RT-81 (integration): the obligations enrichments a hook's
// /rpc call reaches no longer take a whole-tree read at all — layer
// coverage reads the layer's guides from a names-only walk and the raw
// fallback reads the edited file. So with both whole-tree slots held they
// still answer, and never report the RT-146 skip. (The skip itself stays
// wired through the tool and hooks for a backend that does report it —
// TestObligationsTool_SourcesBusyIsNamed.)
func TestSourcesBusyReachesObligationsCallers(t *testing.T) {
	conn := &gatedConnector{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	c := client.New(&firstTwoGated{gated: conn}, store.NewMemoryStore())
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = c.Sources(ctx, "svc") }()
	}
	<-conn.entered
	<-conn.entered
	defer func() { close(conn.gate); wg.Wait() }()

	hook := client.WithHookOrigin(ctx)
	if _, _, err := c.ObligationsLayerCoverage(hook, "svc"); errors.Is(err, client.ErrSourcesBusy) {
		t.Errorf("ObligationsLayerCoverage took a whole-tree slot: err = %v", err)
	}
	if _, _, _, err := c.RawFallbackObligations(hook, "svc", "a.go", 3); errors.Is(err, client.ErrSourcesBusy) {
		t.Errorf("RawFallbackObligations took a whole-tree slot: err = %v", err)
	}
	if !errors.Is(client.ErrSourcesBusy, core.ErrSourcesBusy) {
		t.Error("client.ErrSourcesBusy must be core.ErrSourcesBusy so internal/mcp can name it")
	}
}

// firstTwoGated parks the first two Discover calls (the whole-tree reads
// holding both slots) on the gated connector and answers later ones — the
// obligations path's names-only walk — at once.
type firstTwoGated struct {
	gated *gatedConnector
	n     atomic.Int32
}

func (f *firstTwoGated) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	if f.n.Add(1) <= 2 {
		return f.gated.Discover(ctx, svc)
	}
	return []core.SourceRef{{Service: svc.ID, Path: "a.go"}}, nil
}
func (f *firstTwoGated) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	return f.gated.Fetch(ctx, ref)
}
func (f *firstTwoGated) Watermark(ctx context.Context, svc core.ServiceRef) (string, error) {
	return f.gated.Watermark(ctx, svc)
}

// instantConnector answers Discover/Fetch at once.
type instantConnector struct{}

func (instantConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	return []core.SourceRef{{Service: svc.ID, Path: "a.go"}}, nil
}
func (instantConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	return []byte("package a\n"), nil
}
func (instantConnector) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

// RT-146 review: the slot bounds whole trees in memory, not only the read
// phase. Inside a request scope (the daemon's /rpc) the slot is held until
// the request ends, so two finished-but-still-rendering requests keep the
// third reader out; a second Sources in the same scope reuses the held slot
// (no self-deadlock).
func TestSourcesScopeHoldsSlotUntilDone(t *testing.T) {
	c := client.New(instantConnector{}, store.NewMemoryStore())
	ctx := context.Background()
	s1, done1 := client.WithSourcesScope(ctx, nil)
	s2, done2 := client.WithSourcesScope(ctx, nil)
	for _, s := range []context.Context{s1, s2} {
		if _, err := c.Sources(s, "svc"); err != nil {
			t.Fatalf("scoped Sources: %v", err)
		}
	}
	// Both scopes still hold their trees: a hook is skipped by name.
	if _, err := c.Sources(client.WithHookOrigin(ctx), "svc"); !errors.Is(err, client.ErrSourcesBusy) {
		t.Fatalf("hook Sources while two scopes hold their slots = %v, want ErrSourcesBusy", err)
	}
	// Re-entrant: the same scope reads again without waiting.
	short, cancel := context.WithTimeout(s1, 200*time.Millisecond)
	defer cancel()
	if _, err := c.Sources(short, "svc"); err != nil {
		t.Fatalf("second Sources in the same scope = %v, want it to reuse the held slot", err)
	}
	done1()
	done1() // idempotent
	if _, err := c.Sources(client.WithHookOrigin(ctx), "svc"); err != nil {
		t.Fatalf("hook Sources after one scope ended = %v, want a free slot", err)
	}
	done2()
}

// A waiter whose request was abandoned (client disconnect) stops waiting —
// the daemon's /rpc ctx has no deadline of its own.
func TestSourcesScopeAbandonEndsWait(t *testing.T) {
	c := client.New(instantConnector{}, store.NewMemoryStore())
	ctx := context.Background()
	s1, done1 := client.WithSourcesScope(ctx, nil)
	s2, done2 := client.WithSourcesScope(ctx, nil)
	defer done1()
	defer done2()
	for _, s := range []context.Context{s1, s2} {
		if _, err := c.Sources(s, "svc"); err != nil {
			t.Fatalf("scoped Sources: %v", err)
		}
	}
	abandon := make(chan struct{})
	s3, done3 := client.WithSourcesScope(ctx, abandon)
	defer done3()
	errc := make(chan error, 1)
	go func() { _, err := c.Sources(s3, "svc"); errc <- err }()
	time.Sleep(20 * time.Millisecond)
	close(abandon)
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("abandoned waiter err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an abandoned request kept waiting for a whole-tree slot")
	}
}

// ctxBlockingConnector's Discover blocks until its ctx ends — a tree walk
// that only a cancelled ctx stops.
type ctxBlockingConnector struct{ entered chan struct{} }

func (b ctxBlockingConnector) Discover(ctx context.Context, _ core.ServiceRef) ([]core.SourceRef, error) {
	b.entered <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (ctxBlockingConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	return nil, nil
}

func (ctxBlockingConnector) Watermark(context.Context, core.ServiceRef) (string, error) {
	return "", nil
}

// RT-71 kept RT-146's contract: a ScanSources whose request is abandoned
// mid-read stops its walk and fetch loop, not only its wait for a slot —
// the daemon's /rpc ctx is detached from the connection.
func TestScanSourcesAbandonStopsWalk(t *testing.T) {
	conn := ctxBlockingConnector{entered: make(chan struct{}, 1)}
	c := client.New(conn, store.NewMemoryStore())
	abandon := make(chan struct{})
	s, done := client.WithSourcesScope(context.Background(), abandon)
	defer done()
	errc := make(chan error, 1)
	go func() {
		_, err := c.ScanSources(s, "svc", corpus.Budget{}, func(client.SourceFile) bool { return false })
		errc <- err
	}()
	<-conn.entered
	close(abandon)
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("abandoned scan err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an abandoned ScanSources kept walking the tree")
	}
}

package client_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
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

// RT-146 / I38: the obligations enrichments a hook's /rpc call reaches
// return the named skip instead of swallowing it — a skipped whole-tree
// read is never rendered as an absent coverage block or an empty raw
// fallback.
func TestSourcesBusyReachesObligationsCallers(t *testing.T) {
	conn := &gatedConnector{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	c := client.New(conn, store.NewMemoryStore())
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
	if _, _, err := c.ObligationsLayerCoverage(hook, "svc"); !errors.Is(err, client.ErrSourcesBusy) {
		t.Errorf("ObligationsLayerCoverage under a busy hook read: err = %v, want ErrSourcesBusy", err)
	}
	if _, _, _, err := c.RawFallbackObligations(hook, "svc", "a.go", 3); !errors.Is(err, client.ErrSourcesBusy) {
		t.Errorf("RawFallbackObligations under a busy hook read: err = %v, want ErrSourcesBusy", err)
	}
	if !errors.Is(client.ErrSourcesBusy, core.ErrSourcesBusy) {
		t.Error("client.ErrSourcesBusy must be core.ErrSourcesBusy so internal/mcp can name it")
	}
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

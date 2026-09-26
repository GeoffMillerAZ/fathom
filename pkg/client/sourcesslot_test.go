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

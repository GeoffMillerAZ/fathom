package client_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// countingConnector serves n files and flips the ladder after `flipAt`
// fetches.
type countingConnector struct {
	n, flipAt int
	fetched   atomic.Int64
	level     tenancy.Level
}

func (c *countingConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	refs := make([]core.SourceRef, c.n)
	for i := range refs {
		refs[i] = core.SourceRef{Service: svc.ID, Path: string(rune('a'+i%26)) + ".go"}
	}
	return refs, nil
}

func (c *countingConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	if int(c.fetched.Add(1)) == c.flipAt {
		tenancy.PublishLevel(c.level)
	}
	return []byte("package a\n"), nil
}

func (c *countingConnector) Watermark(context.Context, core.ServiceRef) (string, error) {
	return "", nil
}

// RT-153: (*Client).Sources asks the producer governor before each file and
// returns ErrOverBudget — never a silently shorter corpus — when the ladder
// reaches refuse mid-walk.
func TestSourcesHaltsAtRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	conn := &countingConnector{n: 30, flipAt: 5, level: tenancy.LevelRefuse}
	c := client.New(conn, store.NewMemoryStore())
	files, err := c.Sources(context.Background(), "svc")
	if !errors.Is(err, tenancy.ErrOverBudget) || files != nil {
		t.Fatalf("Sources must stop with ErrOverBudget, not return a partial corpus as whole: %d files, %v", len(files), err)
	}
	if got := conn.fetched.Load(); got != 5 {
		t.Fatalf("halted within one file of the rung: fetched %d, want 5", got)
	}

	// An ingest is registered as ingest-class work the exit rung can halt.
	tenancy.PublishLevel(tenancy.LevelNormal)
	ctx, done := tenancy.BeginIngest(context.Background(), "svc")
	defer done()
	if tenancy.ProducerClassOf(ctx, tenancy.ClassPoint) != tenancy.ClassIngest {
		t.Fatalf("BeginIngest marks ingest class")
	}
	found := false
	for _, e := range tenancy.DefaultInFlight.Snapshot() {
		if e.Class == tenancy.ClassIngest && e.Service == "svc" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the ingest is registered: %+v", tenancy.DefaultInFlight.Snapshot())
	}
}

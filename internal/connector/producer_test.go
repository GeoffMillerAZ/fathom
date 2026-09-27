package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-153 (design §2.1): every byte producer reads the ladder's atomic level
// before each unit of work. At refuse, tree, fleet and ingest producers
// return ErrOverBudget; at the exit mark every producer does. A runaway
// fetch loop halts within one file of the rung, whatever the sampling
// latency.
func TestProducerHaltsWithinOneFile(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	dir := t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := NewLocalConnector()
	svc := core.ServiceRef{ID: "svc", Path: dir}
	refs, err := c.Discover(context.Background(), svc)
	if err != nil || len(refs) != 40 {
		t.Fatalf("discover: %d refs, %v", len(refs), err)
	}

	// An ingest-class loop (the fusion fetch workers' shape): the ladder
	// reaches refuse after the 10th file.
	ctx := tenancy.WithProducerClass(context.Background(), tenancy.ClassIngest)
	read := 0
	var stop error
	for i, ref := range refs {
		if i == 10 {
			tenancy.PublishLevel(tenancy.LevelRefuse)
		}
		if _, err := c.Fetch(ctx, ref); err != nil {
			stop = err
			break
		}
		read++
	}
	if read != 10 || !errors.Is(stop, tenancy.ErrOverBudget) {
		t.Fatalf("the ingest producer must halt at the first file after refuse: read %d, err %v", read, stop)
	}

	// A point-class read still serves at refuse (only tree, fleet and
	// ingest producers halt there)...
	pctx := tenancy.WithProducerClass(context.Background(), tenancy.ClassPoint)
	if _, err := c.Fetch(pctx, refs[0]); err != nil {
		t.Fatalf("a point read at refuse: %v", err)
	}
	// ...and a caller that names no class is treated as a whole-tree walk.
	if _, err := c.Fetch(context.Background(), refs[0]); !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("an unclassed fetch at refuse is a tree producer: %v", err)
	}
	// At the exit mark every producer halts.
	tenancy.PublishLevel(tenancy.LevelExit)
	if _, err := c.Fetch(pctx, refs[0]); !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("a point read at the exit mark: %v", err)
	}
	// Back to normal: everything reads.
	tenancy.PublishLevel(tenancy.LevelNormal)
	if _, err := c.Fetch(ctx, refs[0]); err != nil {
		t.Fatalf("normal: %v", err)
	}
	// The class table itself.
	for _, tc := range []struct {
		level tenancy.Level
		class tenancy.Class
		allow bool
	}{
		{tenancy.LevelShed, tenancy.ClassTree, true},
		{tenancy.LevelRefuse, tenancy.ClassTree, false},
		{tenancy.LevelRefuse, tenancy.ClassFleet, false},
		{tenancy.LevelRefuse, tenancy.ClassIngest, false},
		{tenancy.LevelRefuse, tenancy.ClassIndex, true},
		{tenancy.LevelRefuse, tenancy.ClassEgress, true},
		{tenancy.LevelExit, tenancy.ClassIndex, false},
	} {
		tenancy.PublishLevel(tc.level)
		if got := tenancy.ProducerAllows(tc.class) == nil; got != tc.allow {
			t.Errorf("%s at %s: allowed %v, want %v", tc.class, tc.level, got, tc.allow)
		}
	}
}

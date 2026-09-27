package client_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-153 review finding 2: the obligations corpus (layerCorpus) is a tree
// producer — at refuse it reads no guide and the call fails with the named
// over-budget error, never an "ok=false, nothing to add" that hides the halt.
func TestObligationsCorpusHaltsAtRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	ctx := context.Background()
	dir := t.TempDir()
	tree := map[string]string{
		"CLAUDE.md":   "# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts belong under workspace/.\n",
		"cmd/emit.go": "package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n\treturn os.WriteFile(dir, nil, 0o644)\n}\n",
	}
	for i := 0; i < 5; i++ {
		tree[fmt.Sprintf("lib/l%02d.go", i)] = fmt.Sprintf("package lib\n\nfunc L%02d() {}\n", i)
	}
	writeTree(t, dir, tree)
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}

	tenancy.PublishLevel(tenancy.LevelRefuse)
	stop := client.ObserveReadsUnder(dir)
	rows, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1)
	reads := stop()
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("RawFallbackObligations at refuse = rows %d ok %v err %v, want ErrOverBudget", len(rows), ok, err)
	}
	for _, r := range reads {
		if r == filepath.Join(dir, "CLAUDE.md") {
			t.Fatalf("the layer guide was read at refuse: %v", reads)
		}
	}

	// Back at normal the same call serves its raw row.
	tenancy.PublishLevel(tenancy.LevelNormal)
	if rows, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1); err != nil || !ok || len(rows) != 1 {
		t.Fatalf("at normal: rows=%d ok=%v err=%v, want one raw row", len(rows), ok, err)
	}
}

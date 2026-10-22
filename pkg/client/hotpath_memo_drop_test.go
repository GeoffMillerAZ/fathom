package client_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-34 review finding 3: refuse drops every cache, the obligations hot
// path's memo (guide contents per stat, edited-file tokens and hashes)
// included — after DropHotPathMemos the next call reads the guide again.
func TestDropHotPathMemosForgetsGuideContents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"CLAUDE.md":   "# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts must remain under workspace/.\n",
		"cmd/emit.go": "package cmd\n\nimport \"os\"\n\nfunc emit(dir string) error {\n\treturn os.WriteFile(dir, nil, 0o644)\n}\n",
	})
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	guide := filepath.Join(dir, "CLAUDE.md")
	emit := filepath.Join(dir, "cmd", "emit.go")
	reads := func() (nGuide, nEmit int) {
		stop := client.ObserveReadsUnder(dir)
		if _, _, ok, err := c.RawFallbackObligations(ctx, "svc", "cmd/emit.go", 1); err != nil || !ok {
			t.Fatalf("raw fallback: ok=%v err=%v", ok, err)
		}
		for _, r := range stop() {
			switch r {
			case guide:
				nGuide++
			case emit:
				nEmit++
			}
		}
		return
	}
	if g, e := reads(); g != 1 || e != 1 {
		t.Fatalf("cold call read guide %d, edited %d times; want 1 and 1", g, e)
	}
	if g, e := reads(); g != 0 || e != 0 {
		t.Fatalf("warm call read guide %d, edited %d times; want the memo (0, 0)", g, e)
	}
	c.DropHotPathMemos()
	if g, e := reads(); g != 1 || e != 1 {
		t.Fatalf("after DropHotPathMemos read guide %d, edited %d times; want 1 and 1 (memo dropped)", g, e)
	}
}

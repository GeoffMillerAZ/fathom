package client_test

// portability_test.go — TestStorePortability_RoundTrip is the ADR-017
// contract: a store built against a corpus at one location keeps answering —
// symbols, sources (citation resolution), freshness — after the corpus moves
// and the service root is remapped, with zero re-ingest. This is what makes a
// fathom store a portable expert module (docs/design/expert-modules.md §1).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func writeCorpusFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const portGoSrc = `package mod

// Anchor is the symbol the round-trip asserts on.
func Anchor() int { return 42 }
`

func TestStorePortability_RoundTrip(t *testing.T) {
	ctx := context.Background()

	// Build the store against corpus location A (a non-git corpus: the
	// manifest watermark path is exercised too).
	rootA := t.TempDir()
	writeCorpusFile(t, filepath.Join(rootA, "mod", "anchor.go"), portGoSrc)
	writeCorpusFile(t, filepath.Join(rootA, "docs", "spec.md"), "# spec\nAnchor returns 42.\n")

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "module", Path: rootA}); err != nil {
		t.Fatalf("Ingest at A: %v", err)
	}

	// Birth property: no machine-local absolute anchors in the store.
	obs, err := st.QueryObservations(ctx, "module", "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("no observations ingested")
	}
	for _, o := range obs {
		if filepath.IsAbs(o.SourceRef) {
			t.Fatalf("absolute anchor in store: %q (ADR-017 violated at birth)", o.SourceRef)
		}
	}

	wmA, err := c.ServiceWatermark(ctx, "module")
	if err != nil {
		t.Fatalf("ServiceWatermark at A: %v", err)
	}
	if !strings.HasPrefix(wmA, "manifest:") {
		t.Fatalf("watermark = %q, want manifest: (non-git corpus)", wmA)
	}

	// "Move" the corpus: copy to location B, delete A, remap the store's
	// service root. This simulates shipping the .sqlite store + corpus to
	// another machine.
	rootB := t.TempDir()
	if err := os.CopyFS(rootB, os.DirFS(rootA)); err != nil {
		t.Fatalf("copy corpus: %v", err)
	}
	if err := os.RemoveAll(rootA); err != nil {
		t.Fatalf("remove old corpus root: %v", err)
	}
	meta, found, err := st.GetServiceMeta(ctx, "module")
	if err != nil || !found {
		t.Fatalf("GetServiceMeta: found=%v err=%v", found, err)
	}
	meta.Path = rootB
	if err := st.SetServiceMeta(ctx, meta); err != nil {
		t.Fatalf("remap service root: %v", err)
	}

	// 1. Queries answer unchanged (they never depended on location).
	nodes, err := c.FindSymbol(ctx, "module", "Anchor")
	if err != nil {
		t.Fatalf("FindSymbol after move: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("Anchor not found after move")
	}
	if filepath.IsAbs(nodes[0].FilePath) {
		t.Errorf("FilePath leaked an absolute path: %q", nodes[0].FilePath)
	}

	// 2. Source fetch (citation resolution) works at the new root.
	files, err := c.Sources(ctx, "module")
	if err != nil {
		t.Fatalf("Sources after move: %v", err)
	}
	var foundSpec bool
	for _, f := range files {
		if f.Path == "docs/spec.md" && strings.Contains(string(f.Content), "Anchor returns 42") {
			foundSpec = true
		}
	}
	if !foundSpec {
		t.Error("docs/spec.md did not resolve with intact content at the new root")
	}

	// 3. Freshness is location-independent: the watermark at B equals the
	// one computed at A, so the store reads as fresh, not falsely drifted.
	wmB, err := c.ServiceWatermark(ctx, "module")
	if err != nil {
		t.Fatalf("ServiceWatermark at B: %v", err)
	}
	if wmB != wmA {
		t.Errorf("watermark drifted across a pure move: %q → %q", wmA, wmB)
	}
	if meta.IndexedCommit != wmB {
		t.Errorf("stored watermark %q != current %q — store would read as stale after move", meta.IndexedCommit, wmB)
	}

	// 4. Re-ingest at the new root succeeds and keeps anchors relative
	// (hash-gated: nothing changed, nothing re-extracted).
	report, err := c.Ingest(ctx, client.ServiceRef{ID: "module", Path: rootB})
	if err != nil {
		t.Fatalf("re-Ingest at B: %v", err)
	}
	if report.Extracted != 0 {
		t.Errorf("re-ingest extracted %d files, want 0 (pure move must be hash-gated)", report.Extracted)
	}
	if report.PurgedLegacyAnchors {
		t.Error("re-ingest triggered the legacy self-heal on a born-portable store")
	}

	if err := st.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
}

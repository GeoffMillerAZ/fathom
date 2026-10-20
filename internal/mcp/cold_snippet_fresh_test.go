package mcp

// cold_snippet_fresh_test.go pins O1466: symbol_context's cold answer
// (O1446) applies the same changed-file check to its snippet as the warm
// answer (O1450), so a cold answer never pairs the index's line numbers
// with the working tree's text.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// statSlowBackend adds the indexed file_stats rows to slowSnapshotBackend.
type statSlowBackend struct {
	*slowSnapshotBackend
	rows []store.FileStat
}

func (b *statSlowBackend) FileStats(context.Context, string) ([]store.FileStat, bool, error) {
	return b.rows, len(b.rows) > 0, nil
}

func TestSymbolContextColdAnswerWithholdsSnippetOfChangedFile(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "unchanged"
		if changed {
			name = "changed"
		}
		t.Run(name, func(t *testing.T) {
			slow := newSlowSnapshotBackend(t)
			slow.gate = nil // the snapshot loads at once; only the graph is cold
			full := filepath.Join(slow.root, "work", "run.go")
			data, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(full)
			if err != nil {
				t.Fatal(err)
			}
			row := store.FileStat{Path: "work/run.go", Size: fi.Size(), MtimeNS: fi.ModTime().UnixNano(),
				ContentHash: fusion.ExtractorEpoch + "::" + fusion.FingerprintHash(data)}
			if changed {
				if err := os.WriteFile(full, []byte("package work\n\n// moved\n// down\nfunc Other() {}\n\nfunc Run() {\n}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			be := &statSlowBackend{slow, []store.FileStat{row}}
			ctx := context.Background()
			sc := navScope(ctx, be, "svc")
			if _, err := serviceSnapshotAt(ctx, be, "svc", sc); err != nil {
				t.Fatal(err)
			}
			res, rerr := symbolContextColdAnswer(ctx, be, "svc", "Run", true, &coldGraph{scope: sc})
			if rerr != nil {
				t.Fatalf("rpc error: %+v", rerr)
			}
			got := res.(map[string]any)
			_, hasSnip := got["snippet"]
			why, _ := got["snippet_withheld"].(string)
			if changed {
				if hasSnip || !strings.HasPrefix(why, "file changed since the index") {
					t.Fatalf("cold answer: want snippet withheld, got snippet=%v withheld=%q", hasSnip, why)
				}
			} else if !hasSnip || why != "" {
				t.Fatalf("cold answer: want snippet served, got snippet=%v withheld=%q", hasSnip, why)
			}
		})
	}
}

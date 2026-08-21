package main

// codexdoc_test.go — WF-11 (docs/plans/work-frontier.md): docs/adoption/
// codex.md §6 must state the measurement boundary — Codex sessions are
// outside the impact observatory's instrumentation, adoption there is
// unmeasured by design, and law 12 forbids extrapolating Claude-arm
// numbers to Codex — so the honesty line cannot silently vanish under a
// future edit. Doc-content-read pattern follows
// internal/store/format_v0_test.go's TestStoreFormatV0_DocMatchesGoSource:
// a relative path from the package directory (go test's cwd), two levels
// up to the repo root.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexDocNamesMeasurementBoundary(t *testing.T) {
	docPath := filepath.Join("..", "..", "docs", "adoption", "codex.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docPath, err)
	}
	doc := string(raw)

	for _, want := range []string{
		"outside the impact observatory's instrumentation",
		"unmeasured by design",
		"law 12",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("%s missing the measurement-boundary honesty line — must contain %q", docPath, want)
		}
	}
}

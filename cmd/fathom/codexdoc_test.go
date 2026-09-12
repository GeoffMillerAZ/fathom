package main

// codexdoc_test.go — WF-11 (docs/plans/work-frontier.md): docs/adoption/
// codex.md §6 must state the measurement boundary — Codex sessions are
// outside the impact observatory's instrumentation, adoption there is
// unmeasured by design, and law 12 forbids extrapolating Claude-arm
// numbers to Codex — so the honesty line cannot silently vanish under a
// future edit. HP-10 (docs/plans/harness-port.md): §6's capability table
// is a RENDERING of the registry (harness.RenderCapabilityTable), pinned
// between markers, so the doc can never promise a vehicle the descriptor
// does not declare. Doc-content-read pattern follows
// internal/store/format_v0_test.go's TestStoreFormatV0_DocMatchesGoSource:
// a relative path from the package directory (go test's cwd), two levels
// up to the repo root.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func readCodexDoc(t *testing.T) (string, string) {
	t.Helper()
	docPath := filepath.Join("..", "..", "docs", "adoption", "codex.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docPath, err)
	}
	return docPath, string(raw)
}

func TestCodexDocNamesMeasurementBoundary(t *testing.T) {
	docPath, doc := readCodexDoc(t)

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

// TestCodexDocCapabilityTableIsGenerated pins §6 to
// harness.RenderCapabilityTable("codex"): the bytes between the markers
// are the generated table, verbatim, and every row's status is whatever
// the descriptor's verified_against actually earns — `verified` since
// HP-34 measured a real headless session on this workstation, and
// `unverified on this machine` before it. The table is a rendering, so it
// changes when the descriptor does; the point of the test is that the doc
// can never say something the registry does not.
// Regenerate with `fathom harness capabilities --harness codex`.
func TestCodexDocCapabilityTableIsGenerated(t *testing.T) {
	docPath, doc := readCodexDoc(t)
	const begin = "<!-- fathom:harness-capabilities codex begin -->"
	const end = "<!-- fathom:harness-capabilities codex end -->"
	i := strings.Index(doc, begin)
	j := strings.Index(doc, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("%s §6 must carry the %s / %s markers", docPath, begin, end)
	}
	got := strings.TrimSpace(doc[i+len(begin) : j])
	// A fixed clock: the codex descriptor is verified, so no demotion
	// clock runs, and the rendering is the same on any day.
	want := strings.TrimSpace(harness.RenderCapabilityTable(harness.Codex, time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)))
	if got != want {
		t.Errorf("%s §6 is not the generated table — regenerate with `fathom harness capabilities --harness codex`:\n--- doc ---\n%s\n--- generated ---\n%s", docPath, got, want)
	}
	// Every row still carries an explicit status word — the honesty column
	// may never be blank — and the word must match the descriptor's state
	// rather than a status hand-typed into the doc.
	d, _ := harness.Lookup(harness.Codex)
	status := "verified"
	if d.Unverified() {
		status = "unverified on this machine"
	}
	for _, line := range strings.Split(got, "\n")[2:] {
		if !strings.Contains(line, status) {
			t.Errorf("%s §6 row lacks the honesty status %q: %s", docPath, status, line)
		}
	}
}

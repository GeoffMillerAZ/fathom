package harness

// adoption_test.go — RP-18: docs/adoption/other-harnesses.md's matrix is
// the registry rendering, verbatim, between its markers; a drift fails
// with the regeneration command. The clock is fixed so the rendering is
// the same on any day (no descriptor with a codec is unverified, so the
// demotion clock never moves this table; the fixed date pins that too).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// AdoptionMatrixURL is the daemon URL the committed doc is rendered with.
const AdoptionMatrixURL = "http://127.0.0.1:7575"

func TestAdoptionDocMatrixIsGenerated(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	docPath := filepath.Join(filepath.Dir(file), "..", "..", "docs", "adoption", "other-harnesses.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docPath, err)
	}
	doc := string(raw)
	const begin = "<!-- fathom:harness-matrix begin -->"
	const end = "<!-- fathom:harness-matrix end -->"
	i := strings.Index(doc, begin)
	j := strings.Index(doc, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("%s must carry the %s / %s markers", docPath, begin, end)
	}
	got := strings.TrimSpace(doc[i+len(begin) : j])
	want := strings.TrimSpace(RenderAdoptionMatrix(AdoptionMatrixURL, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)))
	if got != want {
		t.Errorf("%s matrix is not the generated table — regenerate with `fathom harness matrix --url %s` and paste it between the markers:\n--- doc ---\n%s\n--- generated ---\n%s", docPath, AdoptionMatrixURL, got, want)
	}
}

func TestRenderAdoptionMatrixHonesty(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	table := RenderAdoptionMatrix(AdoptionMatrixURL, now)
	rows := AdoptionRows(AdoptionMatrixURL, now)
	if len(rows) != len(registry) {
		t.Fatalf("rows = %d, want one per descriptor (%d)", len(rows), len(registry))
	}
	for _, r := range rows {
		if !strings.Contains(table, "`"+r.ID+"`") {
			t.Errorf("table lacks %s", r.ID)
		}
		d, _ := Lookup(r.ID)
		if d.Unverified() && !strings.Contains(table, "| **unverified**") {
			t.Errorf("unverified %s must render as unverified", r.ID)
		}
		if !strings.Contains(r.MCPAdd, AdoptionMatrixURL) && !strings.Contains(r.MCPAdd, "register") {
			t.Errorf("%s mcp add line must carry the url: %q", r.ID, r.MCPAdd)
		}
	}
	// Deterministic.
	if RenderAdoptionMatrix(AdoptionMatrixURL, now) != table {
		t.Error("rendering is not deterministic")
	}
}

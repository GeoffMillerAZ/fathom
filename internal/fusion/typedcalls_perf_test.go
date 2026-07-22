package fusion_test

// TestTypedEdges_PerformanceGate is CT-5 (docs/plans/callgraph-types.md):
// measures the typed enrichment pass's first-run wall-clock against this
// repo (fathom dogfooding itself) relative to a plain syntax-only ingest
// baseline, and asserts the kill criterion from vision 10 — first-run typed
// load must be ≤ 2× the baseline ingest wall-clock. Also measures a cached
// (unchanged) second run to confirm the per-package content-hash cache
// makes re-ingest incremental.
//
// This exercises the WHOLE repo (hundreds of packages, Tests:true) so it is
// deliberately excluded from -short runs — task test-unit stays fast; task
// validate (which runs the full `go test`) still covers it.
import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestTypedEdges_PerformanceGate(t *testing.T) {
	if testing.Short() {
		t.Skip("whole-repo typed load — excluded from -short")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	// Baseline: plain syntax-only ingest wall-clock for this repo (no typed
	// pass — a fresh Pipeline against a fresh MemoryStore, so the hash-gate
	// cannot short-circuit any file).
	ctx := context.Background()
	baselineStore := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), baselineStore, fusion.Config{Concurrency: 4})
	svc := core.ServiceRef{ID: "fathom-selfhost", Path: repoRoot}

	baselineStart := time.Now()
	report, err := pipe.Ingest(ctx, svc)
	baselineElapsed := time.Since(baselineStart)
	if err != nil {
		t.Fatalf("baseline Ingest: %v", err)
	}
	t.Logf("CT-5 baseline: syntax-only Ingest of %d files (%d symbols) took %s",
		report.Discovered, report.Symbols, baselineElapsed)

	// First (cold-cache) typed-edge run against a SEPARATE fresh store.
	typedStore := store.NewMemoryStore()
	firstStart := time.Now()
	firstReport, err := fusion.EnrichTypedEdges(ctx, typedStore, "fathom-selfhost", repoRoot)
	firstElapsed := time.Since(firstStart)
	if err != nil {
		t.Fatalf("first EnrichTypedEdges: %v", err)
	}
	t.Logf("CT-5 first run: %d packages (%d loaded, %d cached, %d failed), "+
		"%d direct / %d iface-dispatch / %d unresolved edges, took %s",
		firstReport.PackagesTotal, firstReport.PackagesLoaded, firstReport.PackagesCached, firstReport.PackagesFailed,
		firstReport.EdgesDirect, firstReport.EdgesIfaceDispatch, firstReport.EdgesUnresolved, firstElapsed)

	// Second (fully cached) run against the SAME store — nothing changed.
	secondStart := time.Now()
	secondReport, err := fusion.EnrichTypedEdges(ctx, typedStore, "fathom-selfhost", repoRoot)
	secondElapsed := time.Since(secondStart)
	if err != nil {
		t.Fatalf("second EnrichTypedEdges: %v", err)
	}
	t.Logf("CT-5 cached run: %d cached / %d total, took %s (%.1fx faster than first run)",
		secondReport.PackagesCached, secondReport.PackagesTotal, secondElapsed,
		float64(firstElapsed)/float64(max(secondElapsed, 1)))

	// The kill criterion (vision 10, ADR-021 §3): first-run typed load ≤ 2x
	// the baseline ingest wall-clock.
	limit := 2 * baselineElapsed
	if firstElapsed > limit {
		t.Errorf("CT-5 performance gate FAILED: first-run typed load %s > 2x baseline %s (limit %s)",
			firstElapsed, baselineElapsed, limit)
	} else {
		t.Logf("CT-5 performance gate PASSED: first-run typed load %s <= 2x baseline %s (limit %s, ratio %.2fx)",
			firstElapsed, baselineElapsed, limit, float64(firstElapsed)/float64(baselineElapsed))
	}

	if secondReport.PackagesLoaded != 0 {
		t.Errorf("cached run reloaded %d packages, want 0 (nothing changed between runs)", secondReport.PackagesLoaded)
	}
}

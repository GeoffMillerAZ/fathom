package indexbench

import (
	"context"
	"testing"
)

// growthBudget is how much a payload may grow before the gate calls it a
// regression. Ten percent is loose enough that a one-field addition to a
// response does not fail the build and tight enough that the O23 shape — a
// payload quietly doubling as a new layer blends in — cannot pass.
const growthBudget = 1.10

// TestPayloadReplayMatchesGolden is the L1 gate (bench-pyramid BP-16). Three
// things fail it, each naming the tool:
//
//   - a payload more than 10% larger than the golden: the discovery cost
//     fathom controls went up and nobody decided to spend it;
//   - a result count that moved: the tool is answering a different question
//     than the one the golden pinned;
//   - any `displaced` count on an obligations entry: the fixture layer is
//     deliberately under the serving cap, so a displaced claim means scoping
//     or ranking regressed — this is O23 (six C++ plan rows displacing
//     sixty-five others on a Go file) turned into a red number in CI.
//
// Regenerate after a deliberate change with:
//
//	go test ./internal/indexbench/... -update
//
// BP-21 will need exactly that: changing obligations ranking changes these
// bytes by design.
func TestPayloadReplayMatchesGolden(t *testing.T) {
	ctx := context.Background()

	var entries []ReplayEntry
	if err := readGolden(replayGoldenPath, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s has no entries", replayGoldenPath)
	}

	srv, err := NewFixtureServer(ctx)
	if err != nil {
		t.Fatalf("build fixture server: %v", err)
	}

	got, err := ReplayGolden(ctx, srv, entries)
	if err != nil {
		t.Fatalf("ReplayGolden: %v", err)
	}

	// Latency is checked BEFORE the timings are zeroed, and only against the
	// authored budget — never against the golden's own previous number,
	// which would make a fast machine's run the standard every slower
	// machine fails.
	for _, e := range got {
		if e.BudgetMS > 0 && e.ElapsedMS > e.BudgetMS {
			t.Errorf("%s (%s): %dms over the %dms budget", e.ID, e.Tool, e.ElapsedMS, e.BudgetMS)
		}
	}

	if *update {
		ZeroReplayTimings(got)
		if err := writeGolden(replayGoldenPath, got); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", replayGoldenPath)
		return
	}

	for i, g := range got {
		w := entries[i]
		if g.Displaced > 0 {
			t.Errorf("%s (%s): displaced %d — %d served-eligible claims were dropped by the cap. The fixture layer fits under the cap by construction, so this is a scoping or ranking regression, not fixture crowding.",
				g.ID, g.Tool, g.Displaced, g.Displaced)
		}
		if g.ResultCount != w.ResultCount {
			t.Errorf("%s (%s): result_count %d, golden %d — the tool is answering a different question. If that is deliberate: %s",
				g.ID, g.Tool, g.ResultCount, w.ResultCount, RegenerateCommand)
		}
		if w.Bytes > 0 && float64(g.Bytes) > float64(w.Bytes)*growthBudget {
			t.Errorf("%s (%s): payload grew to %d bytes from %d (+%.1f%%, budget +%.0f%%) — this is the discovery cost an agent pays on every call. If that is deliberate: %s",
				g.ID, g.Tool, g.Bytes, w.Bytes,
				100*(float64(g.Bytes)/float64(w.Bytes)-1), 100*(growthBudget-1), RegenerateCommand)
		}
		if g.Bytes < w.Bytes {
			t.Logf("%s (%s): payload SHRANK to %d bytes from %d. Bank it: %s", g.ID, g.Tool, g.Bytes, w.Bytes, RegenerateCommand)
		}
	}
}

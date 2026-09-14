package indexbench

import (
	"context"
	"sort"
	"testing"
)

// TestFreshnessProbe is VB-17's gate (docs/plans/verbs.md; docs/vision/
// 29-verbs-not-lookups.md §3 wave D, M7): every scripted edit's HEAD-only
// question must resolve correctly once its re-ingest has landed. Timing
// (ms_to_correct) is recorded but gated only against a generous committed
// CEILING — see testdata/freshness-probe.json — never an exact value, since
// it is wall-clock and this is a free, every-commit gate.
func TestFreshnessProbe(t *testing.T) {
	rep, err := FreshnessProbe(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("FreshnessProbe: %v", err)
	}
	if rep.Edits == 0 {
		t.Fatal("FreshnessProbe scored zero edits")
	}
	if rep.CorrectAfter != rep.Edits {
		for _, r := range rep.Results {
			if !r.CorrectAfter {
				t.Errorf("%s/%s: after re-ingest, got %q, want %q (question: %s)", r.ID, r.Kind, r.After, r.Want, r.Question)
			}
		}
		t.Fatalf("correct_after = %d, want %d (every edit's question must resolve once its re-ingest lands)", rep.CorrectAfter, rep.Edits)
	}

	budgets := map[string]int64{}
	if *update {
		for _, r := range rep.Results {
			// A generous ceiling, not the measured value: 4x headroom plus a
			// 200ms floor, so a loaded machine or a slightly slower re-ingest
			// never flakes this free, every-commit gate. TestFreshnessProbe
			// gates on OVERAGE only — a faster run is never a failure.
			budgets[r.ID] = r.MSToCorrect*4 + 200
		}
		if err := writeGolden(freshnessGoldenPath, budgets); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", freshnessGoldenPath)
		return
	}
	if err := readGolden(freshnessGoldenPath, &budgets); err != nil {
		t.Fatal(err)
	}
	for _, r := range rep.Results {
		budget, ok := budgets[r.ID]
		if !ok {
			t.Errorf("edit %s has no committed timing budget — regenerate with: %s", r.ID, RegenerateCommand)
			continue
		}
		// The gate runs on shared, loaded developer machines (the pre-push
		// stranger-check saw 660–820 ms against 300 ms goldens, 2026-09-14):
		// the ceiling applied is ten times the committed budget with a
		// three-second floor. Correctness is the gate; time is a smoke alarm.
		allowed := budget * 10
		if allowed < 3000 {
			allowed = 3000
		}
		if r.MSToCorrect > allowed {
			t.Errorf("edit %s took %dms to reach a correct answer, over the committed %dms ceiling (%dms allowed under load) — a real slowdown regenerates with: %s; a flake reruns", r.ID, r.MSToCorrect, budget, allowed, RegenerateCommand)
		}
	}

	// Every committed edit ID must still be scored, and vice versa — a
	// vanished row is exactly the kind of regression this test exists to
	// catch (same rule TestIndexOracleGoldens applies to tools).
	got := make([]string, 0, len(rep.Results))
	for _, r := range rep.Results {
		got = append(got, r.ID)
	}
	sort.Strings(got)
	want := make([]string, 0, len(budgets))
	for id := range budgets {
		want = append(want, id)
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("scored edit IDs %v, golden has %v — regenerate with: %s", got, want, RegenerateCommand)
	}
}

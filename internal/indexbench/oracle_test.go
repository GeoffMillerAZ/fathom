package indexbench

import (
	"flag"
	"fmt"
	"testing"
)

// update regenerates every committed golden in this package. It is shared by
// the L0 oracle and the L1 replay tests so one command covers both:
//
//	go test ./internal/indexbench/... -update
var update = flag.Bool("update", false, "rewrite this package's committed goldens ("+RegenerateCommand+")")

// TestIndexOracleMatchesGolden is the L0 gate's diagnostic-detail half
// (bench-pyramid BP-13): the full per-service, per-tool Report against
// testdata/fauxtown.golden.json, so a drop or a miss names the SERVICE.
// TestIndexOracleGoldens below is VB-6's own compact regression floor
// (per-tool, micro-averaged); both score the SAME IndexOracleGoldens run —
// this test just reads the richer shape out of it.
//
// It fails on any precision or recall DROP against the committed golden — a
// missed call edge is a failing test, not a benchmark number a reader might
// not notice.
//
// An improvement is not a failure: it is reported and the golden is
// regenerated with -update, which is the only supported way to move these
// numbers. The denominators are recorded but never gated — they move
// legitimately whenever the fixture fleet gains a file, and gating them
// would make every fixture edit look like a regression.
func TestIndexOracleMatchesGolden(t *testing.T) {
	got, err := IndexOracleGoldens(t.TempDir())
	if err != nil {
		t.Fatalf("IndexOracleGoldens: %v", err)
	}
	ZeroTimings(&got)

	if *update {
		if err := writeGolden(fauxtownGoldenPath, got); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", fauxtownGoldenPath)
		return
	}

	var want Report
	if err := readGolden(fauxtownGoldenPath, &want); err != nil {
		t.Fatal(err)
	}

	wantByService := map[string]ServiceScore{}
	for _, s := range want.Services {
		wantByService[s.Service] = s
	}
	if len(got.Services) != len(want.Services) {
		t.Errorf("scored %d services, golden has %d — regenerate with: %s",
			len(got.Services), len(want.Services), RegenerateCommand)
	}
	for _, gs := range got.Services {
		ws, ok := wantByService[gs.Service]
		if !ok {
			t.Errorf("service %s is not in the golden — regenerate with: %s", gs.Service, RegenerateCommand)
			continue
		}
		wantTools := map[string]ToolScore{}
		for _, ts := range ws.Tools {
			wantTools[ts.Tool] = ts
		}
		for _, gt := range gs.Tools {
			wt, ok := wantTools[gt.Tool]
			if !ok {
				t.Errorf("%s/%s is not in the golden — regenerate with: %s", gs.Service, gt.Tool, RegenerateCommand)
				continue
			}
			checkRate(t, gs.Service, gt.Tool, "precision", round4(gt.Precision), round4(wt.Precision), gt.Correct, gt.Predicted)
			checkRate(t, gs.Service, gt.Tool, "recall", round4(gt.Recall), round4(wt.Recall), gt.Correct, gt.Truth)
		}
	}
}

// checkRate fails on a drop and logs an improvement with the command that
// banks it.
func checkRate(t *testing.T, service, tool, metric string, got, want float64, num, den int) {
	t.Helper()
	switch {
	case got < want:
		t.Errorf("%s/%s %s dropped: %.4f < golden %.4f (%s). The index lost an answer it used to get right — fix the extractor or the analyzer, do not regenerate the golden to make this green.",
			service, tool, metric, got, want, fmt.Sprintf("%d/%d", num, den))
	case got > want:
		t.Logf("%s/%s %s IMPROVED: %.4f > golden %.4f (%d/%d). Bank it: %s",
			service, tool, metric, got, want, num, den, RegenerateCommand)
	}
}

// TestIndexOracleGoldens is VB-6's `task validate` gate (docs/plans/
// verbs.md, ADR-035 decision 5, docs/design/bench-pyramid.md §3.1): the
// compact, per-tool, micro-averaged precision/recall/n committed at
// internal/indexbench/testdata/l0-goldens.json for find_callers,
// references and the transitive walk blast_radius uses. It fails on any
// precision or recall DROP for a tool the golden already covers, or a tool
// present in one side and not the other — a regression this test is meant
// to catch is exactly as likely to show up as "the tool vanished" as "the
// number fell".
//
// This is the SAME scored run TestIndexOracleMatchesGolden reads above —
// aggregateL0Goldens just collapses IndexOracleGoldens' per-service Report
// into the compact shape this golden pins. An improvement is reported, not
// failed, and only -update moves the number.
func TestIndexOracleGoldens(t *testing.T) {
	rep, err := IndexOracleGoldens(t.TempDir())
	if err != nil {
		t.Fatalf("IndexOracleGoldens: %v", err)
	}
	got := aggregateL0Goldens(rep)

	if *update {
		if err := writeGolden(l0GoldensPath, got); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", l0GoldensPath)
		return
	}

	var want map[string]L0Golden
	if err := readGolden(l0GoldensPath, &want); err != nil {
		t.Fatal(err)
	}
	for tool, wt := range want {
		gt, ok := got[tool]
		if !ok {
			t.Errorf("tool %s is in the golden but the oracle run did not score it — regenerate with: %s", tool, RegenerateCommand)
			continue
		}
		checkL0Rate(t, tool, "precision", gt.Precision, wt.Precision, gt.N)
		checkL0Rate(t, tool, "recall", gt.Recall, wt.Recall, gt.N)
	}
	for tool := range got {
		if _, ok := want[tool]; !ok {
			t.Errorf("tool %s was scored but is not in the golden — regenerate with: %s", tool, RegenerateCommand)
		}
	}
}

// checkL0Rate is checkRate's l0-goldens.json shape: the compact golden
// carries only the micro-averaged rate and its query count n, never the
// raw correct/predicted/truth a per-service Report keeps, so the message
// names n rather than fabricating a numerator/denominator it does not have.
func checkL0Rate(t *testing.T, tool, metric string, got, want float64, n int) {
	t.Helper()
	switch {
	case got < want:
		t.Errorf("l0/%s %s dropped: %.4f < golden %.4f (n=%d). The index lost an answer it used to get right — fix the extractor or the analyzer, do not regenerate the golden to make this green.",
			tool, metric, got, want, n)
	case got > want:
		t.Logf("l0/%s %s IMPROVED: %.4f > golden %.4f (n=%d). Bank it: %s",
			tool, metric, got, want, n, RegenerateCommand)
	}
}

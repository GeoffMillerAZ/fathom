package indexbench

import (
	"context"
	"flag"
	"fmt"
	"math"
	"testing"
)

// update regenerates every committed golden in this package. It is shared by
// the L0 oracle and the L1 replay tests so one command covers both:
//
//	go test ./internal/indexbench/... -update
var update = flag.Bool("update", false, "rewrite this package's committed goldens ("+RegenerateCommand+")")

// round4 is the comparison precision. Four decimals is fine enough to catch
// a single lost edge in a corpus of a few hundred and coarse enough that
// float noise never fails the gate.
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

// TestIndexOracleMatchesGolden is the L0 gate (bench-pyramid BP-13). It
// fails on any precision or recall DROP against the committed golden — a
// missed call edge is a failing test, not a benchmark number a reader might
// not notice.
//
// An improvement is not a failure: it is reported and the golden is
// regenerated with -update, which is the only supported way to move these
// numbers. The denominators are recorded but never gated — they move
// legitimately whenever the fixture fleet gains a file, and gating them
// would make every fixture edit look like a regression.
func TestIndexOracleMatchesGolden(t *testing.T) {
	ctx := context.Background()

	subjects, err := FauxtownSubjects(t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture modules: %v", err)
	}
	c, _, err := NewFauxtownIndex(ctx)
	if err != nil {
		t.Fatalf("build fixture index: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	got, err := IndexOracleReport(ctx, c, subjects)
	if err != nil {
		t.Fatalf("IndexOracleReport: %v", err)
	}
	got.Corpus = "fauxtown"
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

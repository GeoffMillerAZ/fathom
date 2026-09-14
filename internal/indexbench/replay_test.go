package indexbench

import (
	"testing"
)

// byteTolerance is how far a payload may drift, either direction, before
// the gate calls it a regression (VB-7, docs/plans/verbs.md): "bytes within
// ±5%". Tight enough that the O23 shape — a payload quietly doubling as a
// new layer blends in — cannot pass, and symmetric on purpose: a payload
// that SHRANK by more than the tolerance moved for a reason just as much as
// one that grew, and a re-baseline (`-update`) needs the same conscious,
// commit-message-justified step either way.
const byteTolerance = 0.05

// TestPayloadReplay is the L1 gate (bench-pyramid BP-16, VB-7). Three
// things fail it, each naming the tool:
//
//   - a payload more than ±5% off the golden: the discovery cost fathom
//     controls moved and nobody decided to spend (or bank) it;
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
// A re-baseline must be justified in the commit message that carries it —
// this golden moving is itself the signal a reviewer is meant to see.
// BP-21 will need exactly that: changing obligations ranking changes these
// bytes by design.
func TestPayloadReplay(t *testing.T) {
	var entries []ReplayRow
	if err := readGolden(l1ReplayPath, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s has no entries", l1ReplayPath)
	}

	got, err := PayloadReplay(t.TempDir())
	if err != nil {
		t.Fatalf("PayloadReplay: %v", err)
	}

	// Latency is checked BEFORE the timings are zeroed, and only against the
	// authored budget — never against the golden's own previous number,
	// which would make a fast machine's run the standard every slower
	// machine fails. The gate runs `go test -race ./...` beside vet, the
	// stranger-check and whatever else the machine carries, so the ceiling
	// applied is the same one the freshness probe uses: ten times the
	// authored budget with a one-second floor (a 250ms budget tripped at
	// 383ms under four concurrent workers, 2026-09-13). Bytes, counts and
	// elisions below are the gate; time is a smoke alarm.
	for _, e := range got {
		if e.BudgetMS <= 0 {
			continue
		}
		allowed := e.BudgetMS * 10
		if allowed < 1000 {
			allowed = 1000
		}
		if e.ElapsedMS > allowed {
			t.Errorf("%s (%s): %dms over the %dms budget (%dms allowed under load)", e.ID, e.Tool, e.ElapsedMS, e.BudgetMS, allowed)
		} else if e.ElapsedMS > e.BudgetMS {
			t.Logf("%s (%s): %dms over the %dms budget, inside the loaded-machine ceiling", e.ID, e.Tool, e.ElapsedMS, e.BudgetMS)
		}
	}

	if *update {
		ZeroReplayTimings(got)
		if err := writeGolden(l1ReplayPath, got); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", l1ReplayPath)
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
		if w.Bytes > 0 {
			drift := float64(g.Bytes)/float64(w.Bytes) - 1
			if drift > byteTolerance || drift < -byteTolerance {
				t.Errorf("%s (%s): payload %d bytes vs golden %d (%+.1f%%, budget ±%.0f%%) — this is the discovery cost an agent pays on every call. If that is deliberate: %s",
					g.ID, g.Tool, g.Bytes, w.Bytes, 100*drift, 100*byteTolerance, RegenerateCommand)
			}
		}
	}
}

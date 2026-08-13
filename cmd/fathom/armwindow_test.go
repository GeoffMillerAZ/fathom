package main

// armwindow_test.go — the amended unit: an arm-covered WINDOW of a resumed
// session counts, and nothing from before the boundary leaks into it.
//
// Registered before any data existed under it (production-tokenomics.md
// §5, amendment 2026-08-12). The owner's practice is one session resumed
// across days, so the original session-level unit produced a control arm
// that could never admit anything — the instrument had to bend to the
// work, not the work to the instrument.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// windowTranscript writes lines straddling the boundary: 3 assistant turns
// with usage+tools BEFORE, 2 AFTER. The before-side figures are large so a
// leak is unmistakable.
func windowTranscript(t *testing.T, boundary time.Time) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "t.jsonl")
	mk := func(at time.Time, in, out int, tool string) string {
		m := map[string]any{
			"type": "assistant", "timestamp": at.Format(time.RFC3339),
			"message": map[string]any{
				"role":  "assistant",
				"usage": map[string]any{"input_tokens": in, "output_tokens": out},
				"content": []any{map[string]any{
					"type": "tool_use", "id": "u-" + at.Format("150405"), "name": tool,
					"input": map[string]any{"pattern": "x"},
				}},
			},
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	var lines []string
	for i := 1; i <= 3; i++ { // BEFORE: huge figures, raw tools
		lines = append(lines, mk(boundary.Add(-time.Duration(i)*time.Hour), 100000, 50000, "Grep"))
	}
	for i := 1; i <= 2; i++ { // AFTER: small figures
		lines = append(lines, mk(boundary.Add(time.Duration(i)*time.Minute), 100, 50, "Grep"))
	}
	// One line with NO timestamp — unattributable, must not leak in.
	lines = append(lines, `{"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":999999,"output_tokens":999999}}}`)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWindowExcludesPreArmActivity(t *testing.T) {
	boundary := time.Date(2026, 8, 12, 21, 25, 12, 0, time.UTC)
	p := windowTranscript(t, boundary)

	sum, truncated, err := windowSummaryFrom(p, boundary, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("a file under the cap must not report a truncated window")
	}
	if sum.TurnCount == nil || *sum.TurnCount != 2 {
		t.Fatalf("window turns = %v, want 2 — only post-boundary turns count", sum.TurnCount)
	}
	if sum.TokensIn == nil || *sum.TokensIn != 200 {
		t.Fatalf("window tokens_in = %v, want 200 — pre-arm activity (300,000) or the "+
			"untimestamped line (999,999) leaked into the control window", sum.TokensIn)
	}
	if sum.RawDiscoveryCalls != 2 {
		t.Errorf("window raw calls = %d, want 2 — the three pre-arm Greps must not count", sum.RawDiscoveryCalls)
	}
}

func TestWindowTailReadDetectsIncompleteness(t *testing.T) {
	boundary := time.Date(2026, 8, 12, 21, 0, 0, 0, time.UTC)
	p := windowTranscript(t, boundary)

	// A cap so small the tail cannot reach back to the boundary: the first
	// surviving line is AFTER it, so the window is missing its oldest span.
	st, _ := os.Stat(p)
	sum, truncated, err := windowSummaryFrom(p, boundary, st.Size()/3)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("a tail that cannot reach the boundary must report the window truncated — " +
			"a partial window admitted as a unit is the truncation bug one level down")
	}
	_ = sum
}

func TestArmUnitAdmission(t *testing.T) {
	turns := func(n int) *int { return &n }

	t.Run("a clean window is a unit at its per-turn rate", func(t *testing.T) {
		in, out := 1000, 500
		e := sessionLedgerEntry{
			Arm: "off", ArmCoverage: "partial",
			ArmWindow: &armWindowSummary{Turns: turns(15), TokensIn: &in, TokensOut: &out},
		}
		rate, ok := armUnitFor(e)
		if !ok || rate != 100 {
			t.Fatalf("got (%d,%v), want (100,true) — (1000+500)/15", rate, ok)
		}
	})
	t.Run("a truncated window is refused", func(t *testing.T) {
		in, out := 1000, 500
		e := sessionLedgerEntry{
			Arm: "off", ArmCoverage: "partial",
			ArmWindow: &armWindowSummary{Turns: turns(15), TokensIn: &in, TokensOut: &out, Truncated: true},
		}
		if _, ok := armUnitFor(e); ok {
			t.Fatal("a window missing its oldest span must not be a unit")
		}
	})
	t.Run("a window under the turns floor is refused", func(t *testing.T) {
		in, out := 100, 50
		e := sessionLedgerEntry{
			Arm: "off", ArmCoverage: "partial",
			ArmWindow: &armWindowSummary{Turns: turns(abMinTurnsPerUnit - 1), TokensIn: &in, TokensOut: &out},
		}
		if _, ok := armUnitFor(e); ok {
			t.Fatal("a unit below the registered turns floor is noise, not a sample")
		}
	})
	t.Run("an off row with no window and no full coverage is refused", func(t *testing.T) {
		in, out := 1000, 500
		e := sessionLedgerEntry{Arm: "off", ArmCoverage: "partial", TokensIn: &in, TokensOut: &out, TurnCount: turns(100)}
		if _, ok := armUnitFor(e); ok {
			t.Fatal("whole-session figures of a contaminated session must never be the unit")
		}
	})
	t.Run("an on row is its whole session, per turn", func(t *testing.T) {
		in, out := 4000, 2000
		e := sessionLedgerEntry{Arm: "on", TokensIn: &in, TokensOut: &out, TurnCount: turns(60)}
		rate, ok := armUnitFor(e)
		if !ok || rate != 100 {
			t.Fatalf("got (%d,%v), want (100,true)", rate, ok)
		}
	})
}

// TestImpactAdmitsWindowedUnits pins the fold end to end: a resumed OFF
// session with a clean window joins the comparison; its whole-session
// figures never do.
func TestImpactAdmitsWindowedUnits(t *testing.T) {
	dir := hermeticObsEnv(t)
	rows := []string{
		// ON: whole session, 6000/60 = 100/turn.
		`{"schema_version":2,"session_id":"on1","arm":"on","at":"2026-08-12T20:00:00Z","tokens_note":"","turn_count":60,"tokens_total":99,"tokens_in":4000,"tokens_out":2000}`,
		// OFF resumed: contaminated whole-session figures (HUGE), clean window 3000/20 = 150/turn.
		`{"schema_version":2,"session_id":"off1","arm":"off","arm_coverage":"partial","at":"2026-08-12T22:00:00Z","tokens_note":"","turn_count":2000,"tokens_total":99,"tokens_in":900000000,"tokens_out":900000,"arm_window":{"since":"2026-08-12T21:25:12Z","turns":20,"tokens_in":2000,"tokens_out":1000}}`,
		// OFF resumed with a TRUNCATED window: excluded, counted.
		`{"schema_version":2,"session_id":"off2","arm":"off","arm_coverage":"partial","at":"2026-08-12T22:30:00Z","tokens_note":"","arm_window":{"since":"2026-08-12T21:25:12Z","turns":50,"tokens_in":5000,"tokens_out":2500,"truncated":true}}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC))
	if imp.TokensOnSessions != 1 || imp.TokensOffSessions != 1 {
		t.Fatalf("units on/off = %d/%d, want 1/1", imp.TokensOnSessions, imp.TokensOffSessions)
	}
	if imp.TokensOffMedian == nil || *imp.TokensOffMedian != 150 {
		t.Fatalf("off median = %v, want 150/turn from the WINDOW — if this is enormous, the contaminated "+
			"whole-session figures leaked in", imp.TokensOffMedian)
	}
	if imp.TokensOnMedian == nil || *imp.TokensOnMedian != 100 {
		t.Errorf("on median = %v, want 100/turn", imp.TokensOnMedian)
	}
	if imp.UncleanArmRows != 1 {
		t.Errorf("unclean = %d, want 1 (the truncated-window row), counted and named", imp.UncleanArmRows)
	}
}

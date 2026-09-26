package intent_test

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

func betsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# Project intent — router\n\n## Docs map\n\n| Question | Read |\n|---|---|\n| Decisions | `../../ADR.md` |\n",
		"ADR.md":                "# Decisions\n\n## ADR-001 — a thing\n",
		"Taskfile.yml":          "version: '3'\ntasks:\n  bench:\n    cmds: [echo]\n",
		"scripts/measure.sh":    "#!/bin/sh\n",
		"docs/intent/scope.md": `# Scope

## Current goals

**G1. A bet.** Done when: the thing reaches 20 units.
{bet: arm-bench, instrument: "fathom flywheel", threshold: "≥20 units per arm, then a verdict", review: 2026-10-25}

**G2. Missing review.** Done when: x {instrument: "fathom flywheel", threshold: "20"}

**G3. Bad date.** Done when: y {instrument: "fathom flywheel", threshold: "20", review: "next month"}

**G4. No condition.** Just prose, with no done-when.

**G5. Plain.** Done when: all rows proven.

**G6. Unknown verb.** Done when: z {instrument: "fathom nosuchverb", threshold: "1", review: 2026-10-01}

**G7. Due, by a task target.** Done when: w {instrument: "task bench", threshold: "p50 < 1s", review: 2026-09-01}

**G8. A script.** Done when: v {instrument: "scripts/measure.sh --json", threshold: "0 misses", review: 2026-12-01}

**G9. Prose instrument.** Done when: u {instrument: "the continuity loop", threshold: "20 units", review: 2026-12-01}
`,
	})
	return root
}

func isFathomVerb(v string) bool { return v == "flywheel" || v == "doctor" }

// TestGoalBets_RefusedAtLoadWithoutInstrumentThresholdReview (DP-4): a goal
// is a pre-registered bet — an instrument a human can run, a threshold, and
// a review date — or it is refused at load, with the refusal naming exactly
// what is missing. The uncheckable-concern refusal, applied to ambition: no
// goal of adjectives becomes a fathom artifact.
func TestGoalBets_RefusedAtLoadWithoutInstrumentThresholdReview(t *testing.T) {
	root := betsFixture(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Goals) != 9 {
		t.Fatalf("parsed %d goals, want 9: %+v", len(c.Goals), c.Goals)
	}
	if c.Goals[0].ID != "G1" || c.Goals[0].Title != "A bet." || c.Goals[0].File != "docs/intent/scope.md" {
		t.Errorf("goal identity = %+v", c.Goals[0])
	}

	bets := intent.LoadBets(c, isFathomVerb)
	by := map[string]intent.GoalBet{}
	for _, b := range bets {
		by[b.Goal.ID] = b
	}
	if len(bets) != 9 {
		t.Fatalf("LoadBets must return every goal, bet or refused; got %d", len(bets))
	}

	g1 := by["G1"]
	if g1.Refused != "" || g1.Name != "arm-bench" || g1.Instrument != "fathom flywheel" ||
		!strings.HasPrefix(g1.Threshold, "≥20 units") || g1.ReviewRaw != "2026-10-25" || g1.InstrumentKind != "fathom verb" {
		t.Errorf("G1 must load as a bet, got %+v", g1)
	}
	wantRefused := map[string]string{
		"G2": "review",
		"G3": "next month",
		"G4": "Done when",
		"G5": "instrument, threshold, review",
		"G6": "nosuchverb",
		"G9": "the continuity loop",
	}
	for id, mention := range wantRefused {
		b := by[id]
		if b.Refused == "" {
			t.Errorf("%s must be refused at load (%s), got a bet: %+v", id, mention, b)
			continue
		}
		if !strings.Contains(b.Refused, mention) {
			t.Errorf("%s refusal %q must name %q", id, b.Refused, mention)
		}
	}
	if b := by["G7"]; b.Refused != "" || b.InstrumentKind != "task target" {
		t.Errorf("G7 (task bench, a Taskfile target) must load as a bet, got %+v", b)
	}
	if b := by["G8"]; b.Refused != "" || b.InstrumentKind != "repo script" {
		t.Errorf("G8 (a repo script) must load as a bet, got %+v", b)
	}

	// A bet block is a pre-registration, not an oracle: it must never count
	// toward K0's annotation demand signal, or the surface that enumerates
	// bets would fake the signal ADR-026 gates the oracle runtime on.
	for _, cond := range c.Conditions {
		if cond.Record == "G1" && cond.Annotated() {
			t.Errorf("a bet-only block counted as an oracle annotation: %+v", cond)
		}
	}

	// The findings: every refusal, and every bet whose review date passed.
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	fs := intent.BetFindings(bets, now)
	kinds := map[string][]string{}
	for _, f := range fs {
		kinds[f.Kind] = append(kinds[f.Kind], f.Ref)
		if f.Remedy == "" || f.Evidence == "" {
			t.Errorf("finding without remedy/evidence: %+v", f)
		}
	}
	if len(kinds["refused"]) != len(wantRefused) {
		t.Errorf("refused findings %v, want %d", kinds["refused"], len(wantRefused))
	}
	if strings.Join(kinds["review-due"], ",") != "G7" {
		t.Errorf("review-due = %v, want [G7] (review 2026-09-01 has passed; G1's has not)", kinds["review-due"])
	}

	// Without a verb resolver a fathom instrument is not refused — it is
	// named as unchecked, never guessed either way.
	for _, b := range intent.LoadBets(c, nil) {
		if b.Goal.ID == "G6" && b.Refused != "" {
			t.Errorf("with no verb resolver, G6 must not be refused on a guess: %+v", b)
		}
		if b.Goal.ID == "G6" && b.InstrumentKind != "fathom verb (unchecked)" {
			t.Errorf("G6 instrument kind = %q, want \"fathom verb (unchecked)\"", b.InstrumentKind)
		}
	}
}

// TestGoalBets_RealScopeEnumeratesArmBenchAndThreadBet (DP-4): this repo's
// two standing bets — the arm bench (G2) and the continuity-thread bet (G3)
// — load as bets through the same surface, instrument and all.
func TestGoalBets_RealScopeEnumeratesArmBenchAndThreadBet(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Goals) < 5 {
		t.Fatalf("parsed %d goals from scope.md, want ≥5", len(c.Goals))
	}
	bets := intent.LoadBets(c, isFathomVerb)
	names := map[string]intent.GoalBet{}
	for _, b := range bets {
		if b.Refused == "" {
			names[b.Goal.ID] = b
		}
	}
	for id, name := range map[string]string{"G2": "arm-bench", "G3": "thread-bet"} {
		b, ok := names[id]
		if !ok {
			t.Errorf("%s (%s) must load as a bet", id, name)
			continue
		}
		if b.Name != name || b.Instrument == "" || b.Threshold == "" || b.Review.IsZero() {
			t.Errorf("%s: %+v", id, b)
		}
	}
}

package evidenceladder_test

import (
	"context"
	"errors"
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	ladder "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-ladder"
	evidencesig "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
)

// ---------------------------------------------------------------------------
// Fixed clock for deterministic timestamps.
// ---------------------------------------------------------------------------

var fixedNow = time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }

// ---------------------------------------------------------------------------
// Test doubles.
// ---------------------------------------------------------------------------

type stubTier1 struct {
	signals []ladder.Signal
	err     error
}

func (s *stubTier1) Lookup(_ context.Context, concernID string) ([]ladder.Signal, error) {
	return s.signals, s.err
}

type stubTier2 struct {
	findings evidencesig.Finding
	manifest evidencesig.CoverageManifest
	items    []evidencesig.Finding
	err      error
}

func (s *stubTier2) Search(_ context.Context, _ evidencesig.ConcernSignature) (ladder.Findings, evidencesig.CoverageManifest, error) {
	return ladder.Findings{Items: s.items}, s.manifest, s.err
}

type stubTier3 struct {
	verdict ladder.HumanVerdict
	err     error
}

func (s *stubTier3) Escalate(_ context.Context, _ ladder.EscalationRecord) (ladder.HumanVerdict, error) {
	return s.verdict, s.err
}

// ---------------------------------------------------------------------------
// Helper: canonical existence signature.
// ---------------------------------------------------------------------------

func existenceSig(id string) evidencesig.ConcernSignature {
	return evidencesig.ConcernSignature{
		ID:        id,
		Archetype: evidencesig.ArchetypeExistence,
		Scope:     evidencesig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []evidencesig.Predicate{
			{Kind: "regexp", Arg: "TODO"},
		},
	}
}

func absenceSig(id string) evidencesig.ConcernSignature {
	return evidencesig.ConcernSignature{
		ID:        id,
		Archetype: evidencesig.ArchetypeAbsence,
		Scope:     evidencesig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []evidencesig.Predicate{
			{Kind: "regexp", Arg: "password="},
		},
		Canary: &evidencesig.Canary{Path: "testdata/canary.go", Pattern: "password=CANARY"},
	}
}

// ---------------------------------------------------------------------------
// Core progression tests.
// ---------------------------------------------------------------------------

func TestLadder_Tier1MissProceeds(t *testing.T) {
	t.Parallel()
	// Tier-1 returns nothing → must proceed to Tier-2, not conclude.
	tier1 := &stubTier1{signals: nil}
	tier2 := &stubTier2{
		items: []evidencesig.Finding{{Path: "main.go", LineStart: 5, LineEnd: 5, Decisive: true}},
		manifest: evidencesig.CoverageManifest{
			Files: []string{"main.go"}, Tool: "test", TreeHash: "abc",
		},
	}
	l := ladder.New(tier1, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c1"))
	if res.Verdict != ladder.VerdictFound {
		t.Errorf("expected found, got %s (%s)", res.Verdict, res.Reason)
	}
}

func TestLadder_EmptyTier1IsNeverConclusion(t *testing.T) {
	t.Parallel()
	// Even with a "cache hit" (non-nil but empty signals), Tier-2 must run.
	tier1 := &stubTier1{signals: []ladder.Signal{}}
	tier2 := &stubTier2{
		items: []evidencesig.Finding{{Path: "main.go", Decisive: true}},
		manifest: evidencesig.CoverageManifest{
			Files: []string{"main.go"}, Tool: "test",
		},
	}
	l := ladder.New(tier1, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c2"))
	if res.Verdict != ladder.VerdictFound {
		t.Errorf("got %s, want found", res.Verdict)
	}
}

func TestLadder_AbsentProven(t *testing.T) {
	t.Parallel()
	// Zero findings + complete manifest → ABSENT_PROVEN.
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files:    []string{"a.go", "b.go"},
			Tool:     "test",
			TreeHash: "tree001",
		},
	}
	l := ladder.New(nil, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), absenceSig("c3"))
	if res.Verdict != ladder.VerdictAbsentProven {
		t.Errorf("got %s, want absent_proven; reason: %s", res.Verdict, res.Reason)
	}
	if res.ScopeHash != "tree001" {
		t.Errorf("ScopeHash = %q, want tree001", res.ScopeHash)
	}
}

func TestLadder_AbsentUnverified_IncompleteManifest(t *testing.T) {
	t.Parallel()
	// Zero findings but manifest has binary skips → ABSENT_UNVERIFIED.
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files: []string{"a.go"},
			Tool:  "test",
			Skipped: evidencesig.SkippedFiles{
				Binary: []string{"image.png"},
			},
		},
	}
	l := ladder.New(nil, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), absenceSig("c4"))
	if res.Verdict != ladder.VerdictAbsentUnverified {
		t.Errorf("got %s, want absent_unverified; reason: %s", res.Verdict, res.Reason)
	}
}

func TestLadder_Tier3EscalatesOnUnsatisfied(t *testing.T) {
	t.Parallel()
	// Tier-2 finds nothing and manifest is complete, but it's an existence sig → unsatisfied → Tier-3.
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files: []string{"a.go"}, Tool: "test", TreeHash: "t2",
		},
	}
	validVerdict := ladder.HumanVerdict{
		Who:       "alice",
		When:      "2026-07-10T09:00:00Z",
		GradeTier: evidencegrade.TierDocumentary,
		Rationale: "Found it manually",
		Feedback:  "Add broader regexp to predicate",
	}
	tier3 := &stubTier3{verdict: validVerdict}
	l := ladder.New(nil, tier2, tier3, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c5"))
	if res.Verdict != ladder.VerdictFound {
		t.Errorf("got %s, want found (from Tier-3 human verdict); reason: %s", res.Verdict, res.Reason)
	}
}

func TestLadder_Tier3Timeout_FailClosed(t *testing.T) {
	t.Parallel()
	// Tier-2 unsatisfied; Tier-3 times out → UNRESOLVED (not absent).
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files: []string{"a.go"}, Tool: "test",
		},
	}
	tier3 := &stubTier3{err: errors.New("context deadline exceeded")}
	l := ladder.New(nil, tier2, tier3, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c6"))
	if res.Verdict != ladder.VerdictUnresolved {
		t.Errorf("Tier-3 timeout should give UNRESOLVED, got %s", res.Verdict)
	}
}

func TestLadder_Tier2Error_ReturnsUnresolved(t *testing.T) {
	t.Parallel()
	tier2 := &stubTier2{err: errors.New("disk error")}
	l := ladder.New(nil, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c7"))
	if res.Verdict != ladder.VerdictUnresolved {
		t.Errorf("Tier-2 error should give UNRESOLVED, got %s", res.Verdict)
	}
}

func TestLadder_InvalidSignature_ReturnsUnresolved(t *testing.T) {
	t.Parallel()
	badSig := evidencesig.ConcernSignature{} // invalid
	tier2 := &stubTier2{manifest: evidencesig.CoverageManifest{Tool: "test"}}
	l := ladder.New(nil, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), badSig)
	if res.Verdict != ladder.VerdictUnresolved {
		t.Errorf("invalid sig should give UNRESOLVED, got %s", res.Verdict)
	}
}

func TestLadder_GapRecordEmittedOnTier3Escalation(t *testing.T) {
	t.Parallel()
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files: []string{"a.go"}, Tool: "test",
		},
	}
	tier3 := &stubTier3{err: errors.New("timeout")} // will time out

	var gaps []ladder.GapRecord
	l := ladder.New(
		nil, tier2, tier3,
		ladder.WithClock(fixedClock),
		ladder.WithGapLogger(func(g ladder.GapRecord) {
			gaps = append(gaps, g)
		}),
	)
	l.Run(context.Background(), existenceSig("c8"))
	if len(gaps) != 1 {
		t.Errorf("expected 1 GapRecord, got %d", len(gaps))
	}
	if gaps[0].ConcernID != "c8" {
		t.Errorf("GapRecord.ConcernID = %q, want c8", gaps[0].ConcernID)
	}
	if !gaps[0].OccurredAt.Equal(fixedNow) {
		t.Errorf("GapRecord.OccurredAt = %v, want %v", gaps[0].OccurredAt, fixedNow)
	}
}

// ---------------------------------------------------------------------------
// HumanVerdict validation.
// ---------------------------------------------------------------------------

func TestHumanVerdict_FeedbackMandatory(t *testing.T) {
	t.Parallel()
	v := ladder.HumanVerdict{
		Who:       "alice",
		When:      "2026-07-10T09:00:00Z",
		GradeTier: evidencegrade.TierDocumentary,
		Rationale: "some rationale",
		Feedback:  "", // missing!
	}
	if err := v.Validate(); err == nil {
		t.Error("HumanVerdict with empty Feedback should be invalid")
	}
}

func TestHumanVerdict_ValidFull(t *testing.T) {
	t.Parallel()
	v := ladder.HumanVerdict{
		Who:       "bob",
		When:      "2026-07-10T10:00:00Z",
		GradeTier: evidencegrade.TierCircumstantial,
		Rationale: "manually inspected",
		Feedback:  "add broader glob",
	}
	if err := v.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// EscalationRecord validation.
// ---------------------------------------------------------------------------

func TestEscalationRecordValidate(t *testing.T) {
	t.Parallel()
	valid := ladder.EscalationRecord{
		ConcernID:        "c1",
		SignatureID:      "sig-1",
		ManifestTreeHash: "abc123",
		Question:         "Is this absent?",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	missing := ladder.EscalationRecord{} // all empty
	if err := missing.Validate(); err == nil {
		t.Error("empty EscalationRecord should be invalid")
	}
}

// ---------------------------------------------------------------------------
// Nil Tier3 → UNRESOLVED when Tier-2 doesn't satisfy.
// ---------------------------------------------------------------------------

func TestLadder_NoTier3_Unresolved(t *testing.T) {
	t.Parallel()
	tier2 := &stubTier2{
		items: nil,
		manifest: evidencesig.CoverageManifest{
			Files: []string{"a.go"}, Tool: "test",
		},
	}
	l := ladder.New(nil, tier2, nil, ladder.WithClock(fixedClock))
	res := l.Run(context.Background(), existenceSig("c9"))
	if res.Verdict != ladder.VerdictUnresolved {
		t.Errorf("no Tier-3 + unsatisfied should give UNRESOLVED, got %s", res.Verdict)
	}
}

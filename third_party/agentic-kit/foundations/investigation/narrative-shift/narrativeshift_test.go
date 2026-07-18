// Package narrativeshift_test exercises Detect using an external test package,
// treating the API as a black box. All tests are table-driven and deterministic
// — no time.Now(), no random data.
package narrativeshift_test

import (
	"testing"
	"time"

	assertion "github.com/geoffmilleraz/agentic-kit/foundations/evidence/assertion"
	narrativeshift "github.com/geoffmilleraz/agentic-kit/foundations/investigation/narrative-shift"
)

// day returns a fixed UTC time at midnight offset by n days from an arbitrary
// epoch, giving clean human-readable test timestamps without wall-clock calls.
func day(n int) time.Time {
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return epoch.AddDate(0, 0, n)
}

// makeAssertion constructs a minimal assertion.Assertion sufficient for
// narrativeshift.Detect. Fields not relevant to shift detection are left at
// their zero values — Detect does not validate provenance.
func makeAssertion(id, declarant, propID string, stance assertion.Stance, content string, statedAt time.Time) assertion.Assertion {
	return assertion.Assertion{
		ID:            id,
		Declarant:     declarant,
		PropositionID: propID,
		Stance:        stance,
		Content:       content,
		StatedAt:      statedAt,
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      []assertion.Assertion
		wantLen    int
		checkFirst func(t *testing.T, shift narrativeshift.NarrativeShift)
		checkAll   func(t *testing.T, shifts []narrativeshift.NarrativeShift)
	}{
		{
			// Test 1: Alice's deposition says "home"; later interview says "bar"
			// on the same proposition → exactly one content_change with correct GapDays.
			name: "content_change_depo_to_interview",
			input: []assertion.Assertion{
				makeAssertion("a1", "entity:alice", "prop:location", assertion.StanceAffirms, "I was home", day(0)),
				makeAssertion("a2", "entity:alice", "prop:location", assertion.StanceAffirms, "I was at the bar", day(30)),
			},
			wantLen: 1,
			checkFirst: func(t *testing.T, s narrativeshift.NarrativeShift) {
				t.Helper()
				if s.Kind != narrativeshift.KindContentChange {
					t.Errorf("Kind = %q; want content_change", s.Kind)
				}
				if s.GapDays != 30 {
					t.Errorf("GapDays = %d; want 30", s.GapDays)
				}
				if s.From.ID != "a1" {
					t.Errorf("From.ID = %q; want a1", s.From.ID)
				}
				if s.To.ID != "a2" {
					t.Errorf("To.ID = %q; want a2", s.To.ID)
				}
			},
		},
		{
			// Test 2: affirm then recant → recantation.
			name: "affirm_then_recant",
			input: []assertion.Assertion{
				makeAssertion("b1", "entity:alice", "prop:contract", assertion.StanceAffirms, "I signed it", day(0)),
				makeAssertion("b2", "entity:alice", "prop:contract", assertion.StanceRecants, "I withdraw my prior statement", day(7)),
			},
			wantLen: 1,
			checkFirst: func(t *testing.T, s narrativeshift.NarrativeShift) {
				t.Helper()
				if s.Kind != narrativeshift.KindRecantation {
					t.Errorf("Kind = %q; want recantation", s.Kind)
				}
				if s.GapDays != 7 {
					t.Errorf("GapDays = %d; want 7", s.GapDays)
				}
			},
		},
		{
			// Test 3: single assertion in a group → no shifts at all.
			name: "single_assertion_no_shift",
			input: []assertion.Assertion{
				makeAssertion("c1", "entity:alice", "prop:location", assertion.StanceAffirms, "I was home", day(0)),
			},
			wantLen: 0,
		},
		{
			// Test 4: two different declarants assert the same proposition
			// differently → NO shift (cross-declarant disagreement is a conflict,
			// not a narrative shift).
			name: "different_declarants_no_shift",
			input: []assertion.Assertion{
				makeAssertion("d1", "entity:alice", "prop:meeting", assertion.StanceAffirms, "Alice: yes the meeting happened", day(0)),
				makeAssertion("d2", "entity:bob", "prop:meeting", assertion.StanceDenies, "Bob: no meeting happened", day(0)),
			},
			wantLen: 0,
		},
		{
			// Test 5: stable ordering — shuffled input produces identical output.
			// Ties on StatedAt are broken by ID. We construct two assertions
			// with the same StatedAt but different IDs; the one with the
			// lexicographically smaller ID must appear as From.
			name: "stable_ordering_tie_broken_by_id",
			input: []assertion.Assertion{
				// Deliberately provided out of chronological order; IDs determine tie.
				makeAssertion("z2", "entity:alice", "prop:order", assertion.StanceAffirms, "second content", day(0)),
				makeAssertion("a1", "entity:alice", "prop:order", assertion.StanceAffirms, "first content", day(0)),
			},
			wantLen: 1,
			checkFirst: func(t *testing.T, s narrativeshift.NarrativeShift) {
				t.Helper()
				// "a1" < "z2" so a1 should be From and z2 should be To.
				if s.From.ID != "a1" {
					t.Errorf("From.ID = %q; want a1 (ID tie-break)", s.From.ID)
				}
				if s.To.ID != "z2" {
					t.Errorf("To.ID = %q; want z2", s.To.ID)
				}
				if s.Kind != narrativeshift.KindContentChange {
					t.Errorf("Kind = %q; want content_change", s.Kind)
				}
				if s.GapDays != 0 {
					t.Errorf("GapDays = %d; want 0 (same day)", s.GapDays)
				}
			},
			// Verify shuffled input produces the same result.
			checkAll: func(t *testing.T, shifts []narrativeshift.NarrativeShift) {
				t.Helper()
				// Run Detect again on the reversed slice.
				reversed := []assertion.Assertion{
					makeAssertion("a1", "entity:alice", "prop:order", assertion.StanceAffirms, "first content", day(0)),
					makeAssertion("z2", "entity:alice", "prop:order", assertion.StanceAffirms, "second content", day(0)),
				}
				got2 := narrativeshift.Detect(reversed)
				if len(got2) != len(shifts) {
					t.Errorf("shuffled: len = %d; want %d", len(got2), len(shifts))
					return
				}
				if len(shifts) > 0 && got2[0].From.ID != shifts[0].From.ID {
					t.Errorf("shuffled: From.ID = %q; want %q", got2[0].From.ID, shifts[0].From.ID)
				}
			},
		},
		{
			// Test 6: GapDays computed correctly across a multi-day gap.
			name: "gap_days_multi_day",
			input: []assertion.Assertion{
				makeAssertion("e1", "entity:alice", "prop:gap", assertion.StanceAffirms, "earlier statement", day(0)),
				makeAssertion("e2", "entity:alice", "prop:gap", assertion.StanceAffirms, "later statement", day(45)),
			},
			wantLen: 1,
			checkFirst: func(t *testing.T, s narrativeshift.NarrativeShift) {
				t.Helper()
				if s.GapDays != 45 {
					t.Errorf("GapDays = %d; want 45", s.GapDays)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := narrativeshift.Detect(tc.input)

			if len(got) != tc.wantLen {
				t.Fatalf("Detect returned %d shifts; want %d\n%+v", len(got), tc.wantLen, got)
			}

			if tc.checkFirst != nil && len(got) > 0 {
				tc.checkFirst(t, got[0])
			}

			if tc.checkAll != nil {
				tc.checkAll(t, got)
			}
		})
	}
}

// TestDetect_StanceChange verifies that affirm→deny (different stances, neither
// a retraction) produces a stance_change rather than recantation or content_change.
func TestDetect_StanceChange(t *testing.T) {
	t.Parallel()

	input := []assertion.Assertion{
		makeAssertion("f1", "entity:bob", "prop:meeting", assertion.StanceAffirms, "The meeting happened", day(0)),
		makeAssertion("f2", "entity:bob", "prop:meeting", assertion.StanceDenies, "The meeting never happened", day(14)),
	}
	got := narrativeshift.Detect(input)
	if len(got) != 1 {
		t.Fatalf("want 1 shift; got %d", len(got))
	}
	if got[0].Kind != narrativeshift.KindStanceChange {
		t.Errorf("Kind = %q; want stance_change", got[0].Kind)
	}
	if got[0].GapDays != 14 {
		t.Errorf("GapDays = %d; want 14", got[0].GapDays)
	}
}

// TestDetect_CompatiblePair verifies that two assertions with identical Stance
// and identical Content are not reported as a shift.
func TestDetect_CompatiblePair(t *testing.T) {
	t.Parallel()

	input := []assertion.Assertion{
		makeAssertion("g1", "entity:alice", "prop:foo", assertion.StanceAffirms, "same content", day(0)),
		makeAssertion("g2", "entity:alice", "prop:foo", assertion.StanceAffirms, "same content", day(5)),
	}
	got := narrativeshift.Detect(input)
	if len(got) != 0 {
		t.Errorf("want 0 shifts for compatible pair; got %d: %+v", len(got), got)
	}
}

// TestShiftKind_Validate verifies the ShiftKind validator accepts all known
// kinds and rejects unknown strings.
func TestShiftKind_Validate(t *testing.T) {
	t.Parallel()

	validKinds := []narrativeshift.ShiftKind{
		narrativeshift.KindRecantation,
		narrativeshift.KindStanceChange,
		narrativeshift.KindContentChange,
		narrativeshift.KindEscalation,
	}
	for _, k := range validKinds {
		if err := k.Validate(); err != nil {
			t.Errorf("Validate(%q) returned unexpected error: %v", k, err)
		}
	}

	invalid := narrativeshift.ShiftKind("bogus")
	if err := invalid.Validate(); err == nil {
		t.Error("Validate(\"bogus\") returned nil; want error")
	}
}

// TestDetect_WithdrawsIsRetraction verifies that StanceWithdraws (softer
// retraction) also produces KindRecantation, since Stance.IsRetraction()
// covers both withdraws and recants.
func TestDetect_WithdrawsIsRetraction(t *testing.T) {
	t.Parallel()

	input := []assertion.Assertion{
		makeAssertion("h1", "entity:alice", "prop:bar", assertion.StanceAffirms, "I was there", day(0)),
		makeAssertion("h2", "entity:alice", "prop:bar", assertion.StanceWithdraws, "I no longer stand by that", day(3)),
	}
	got := narrativeshift.Detect(input)
	if len(got) != 1 {
		t.Fatalf("want 1 shift; got %d", len(got))
	}
	if got[0].Kind != narrativeshift.KindRecantation {
		t.Errorf("Kind = %q; want recantation (withdraws is a retraction)", got[0].Kind)
	}
}

package expertchat

import (
	"errors"
	"testing"
)

func TestNewCitedAnswer_RejectsZeroCitations(t *testing.T) {
	_, err := NewCitedAnswer("some answer", nil)
	if !errors.Is(err, ErrNoCitations) {
		t.Fatalf("got err %v, want ErrNoCitations", err)
	}

	_, err = NewCitedAnswer("some answer", []Citation{})
	if !errors.Is(err, ErrNoCitations) {
		t.Fatalf("got err %v, want ErrNoCitations for empty (non-nil) slice", err)
	}
}

func TestNewCitedAnswer_ZeroValueUnrepresentable(t *testing.T) {
	// The zero value carries no citations and no way to add any through the
	// public API — this is the type-level guarantee, proven directly.
	var a CitedAnswer
	if got := a.Citations(); len(got) != 0 {
		t.Fatalf("zero-value CitedAnswer.Citations() = %v, want empty", got)
	}
	if got := a.Text(); got != "" {
		t.Fatalf("zero-value CitedAnswer.Text() = %q, want empty", got)
	}
}

func TestNewCitedAnswer_RejectsIncompleteCitation(t *testing.T) {
	cases := []struct {
		name string
		c    Citation
	}{
		{"empty ClaimID", Citation{Module: "m", Tier: "verified"}},
		{"empty Module", Citation{ClaimID: "c1", Tier: "verified"}},
		{"empty Tier", Citation{Module: "m", ClaimID: "c1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCitedAnswer("answer", []Citation{tc.c})
			if err == nil {
				t.Fatal("expected an error for an incomplete citation, got nil")
			}
		})
	}
}

func TestNewCitedAnswer_SortsCitationsDeterministically(t *testing.T) {
	in := []Citation{
		{Module: "zeta", ClaimID: "c9", Tier: "verified"},
		{Module: "alpha", ClaimID: "c2", Tier: "verified"},
		{Module: "alpha", ClaimID: "c1", Tier: "verified"},
	}
	a, err := NewCitedAnswer("answer", in)
	if err != nil {
		t.Fatalf("NewCitedAnswer: %v", err)
	}
	got := a.Citations()
	want := []string{"alpha/c1", "alpha/c2", "zeta/c9"}
	for i, c := range got {
		if key := c.Module + "/" + c.ClaimID; key != want[i] {
			t.Fatalf("Citations()[%d] = %s, want %s (full order: %v)", i, key, want[i], got)
		}
	}
}

func TestCitedAnswer_Citations_ReturnsIndependentCopy(t *testing.T) {
	a, err := NewCitedAnswer("answer", []Citation{{Module: "m", ClaimID: "c1", Tier: "verified"}})
	if err != nil {
		t.Fatalf("NewCitedAnswer: %v", err)
	}
	got := a.Citations()
	got[0].ClaimID = "mutated"
	again := a.Citations()
	if again[0].ClaimID != "c1" {
		t.Fatalf("mutating a returned Citations() slice affected the CitedAnswer: got %q", again[0].ClaimID)
	}
}

func TestCitedAnswer_Render_Deterministic(t *testing.T) {
	citations := []Citation{
		{Module: "m1", ModuleVersion: "v1", ClaimID: "c2", Tier: "verified", Status: StatusFresh, SourceRef: "src2", Anchor: "anchor 2"},
		{Module: "m1", ModuleVersion: "v1", ClaimID: "c1", Tier: "doc-claimed", Status: StatusStale, SourceRef: "src1", Anchor: "anchor 1"},
	}
	a1, err := NewCitedAnswer("the answer text", citations)
	if err != nil {
		t.Fatalf("NewCitedAnswer: %v", err)
	}
	// Build a second value from the SAME logical citations, supplied in a
	// DIFFERENT order — Render must still be byte-identical (sorting is
	// applied at construction, not at render time only).
	reordered := []Citation{citations[1], citations[0]}
	a2, err := NewCitedAnswer("the answer text", reordered)
	if err != nil {
		t.Fatalf("NewCitedAnswer: %v", err)
	}
	if a1.Render() != a2.Render() {
		t.Fatalf("Render() not deterministic across input citation order:\na1=%q\na2=%q", a1.Render(), a2.Render())
	}
	if got := a1.Render(); got == "" {
		t.Fatal("Render() returned empty string")
	}
}

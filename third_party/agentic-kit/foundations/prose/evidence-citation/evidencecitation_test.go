package evidencecitation_test

import (
	"testing"

	ec "github.com/geoffmilleraz/agentic-kit/foundations/prose/evidence-citation"
)

// TestCite_NilRefs ensures nil input returns an empty string.
func TestCite_NilRefs(t *testing.T) {
	if got := ec.Cite(nil); got != "" {
		t.Errorf("expected empty string for nil refs, got %q", got)
	}
}

// TestCite_EmptySlice ensures an empty slice returns an empty string.
func TestCite_EmptySlice(t *testing.T) {
	if got := ec.Cite([]string{}); got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}
}

// TestCite_SingleRef ensures a single ref is formatted correctly.
func TestCite_SingleRef(t *testing.T) {
	got := ec.Cite([]string{"span-001"})
	want := "[span-001]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCite_MultipleRefsAreSorted ensures output is lexicographically sorted.
func TestCite_MultipleRefsAreSorted(t *testing.T) {
	got := ec.Cite([]string{"span-c", "span-a", "span-b"})
	want := "[span-a; span-b; span-c]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCite_Deterministic ensures two calls with the same refs in different
// orders produce identical output.
func TestCite_Deterministic(t *testing.T) {
	refs1 := []string{"z-ref", "a-ref", "m-ref"}
	refs2 := []string{"m-ref", "z-ref", "a-ref"}
	if ec.Cite(refs1) != ec.Cite(refs2) {
		t.Errorf("Cite is not deterministic: %q != %q", ec.Cite(refs1), ec.Cite(refs2))
	}
}

// TestCite_DeduplicatesRefs ensures duplicate refs appear only once.
func TestCite_DeduplicatesRefs(t *testing.T) {
	got := ec.Cite([]string{"span-a", "span-a", "span-b"})
	want := "[span-a; span-b]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCite_IgnoresEmptyStrings ensures empty-string elements are filtered out.
func TestCite_IgnoresEmptyStrings(t *testing.T) {
	got := ec.Cite([]string{"", "span-001", ""})
	want := "[span-001]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCite_AllEmptyStrings ensures all-empty-element slices return "".
func TestCite_AllEmptyStrings(t *testing.T) {
	if got := ec.Cite([]string{"", ""}); got != "" {
		t.Errorf("expected empty string for all-empty refs, got %q", got)
	}
}

// TestCite_AlphanumericSorting ensures lexicographic ordering is stable across
// digit-prefixed identifiers.
func TestCite_AlphanumericSorting(t *testing.T) {
	got := ec.Cite([]string{"span-10", "span-2", "span-1"})
	// Lexicographic: "span-1" < "span-10" < "span-2"
	want := "[span-1; span-10; span-2]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

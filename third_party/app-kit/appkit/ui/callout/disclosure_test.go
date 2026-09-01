package callout

import (
	"strings"
	"testing"
)

// TestCalloutDisclosureIsOptional pins the contract that makes one callout
// family serve both jobs: a status notice that has nothing more to say stays
// exactly as flat as it was, and one that does say more discloses it.
//
// The backward-compatibility half matters as much as the new half — every
// existing caller passes no Details, and their markup must not shift.
func TestCalloutDisclosureIsOptional(t *testing.T) {
	flat := Render(Callout{Kind: Info, Title: "Information", Body: "Neutral context."})

	for _, forbidden := range []string{"<details", "<summary", "group-open:"} {
		if strings.Contains(flat, forbidden) {
			t.Errorf("a callout with no Details emitted %q — existing callers must be unchanged:\n%s", forbidden, flat)
		}
	}
	if !strings.Contains(flat, `role="note"`) {
		t.Errorf("flat callout lost its ARIA role:\n%s", flat)
	}

	rich := Render(Callout{
		Kind: Caution, Title: "Weigh this", Body: "A caveat.",
		Details: "<p>The elaboration.</p>", DetailsLabel: "What changed",
	})
	for _, want := range []string{"<details", "<summary", "group-open:rotate-90", "What changed", "The elaboration."} {
		if !strings.Contains(rich, want) {
			t.Errorf("expandable callout missing %q:\n%s", want, rich)
		}
	}
	// A <details> must NOT carry an ARIA role — it would override the native
	// disclosure semantics screen readers already handle.
	if strings.Contains(rich, `<details role=`) {
		t.Errorf("details element carries an ARIA role, overriding native semantics:\n%s", rich)
	}
	if strings.Contains(rich, " open") {
		t.Errorf("expandable callout defaulted to open without being asked:\n%s", rich)
	}

	// Open is for things a reader must not miss.
	if !strings.Contains(Render(Callout{Kind: Danger, Title: "Failed", Details: "why", Open: true}), "<details open") {
		t.Error("Open did not force the disclosure open")
	}

	// Details with no summary content is still legal, but the summary must not
	// be empty — the affordance has to say something.
	only := Render(Callout{Kind: Note, Details: "<p>x</p>"})
	if !strings.Contains(only, "<summary") {
		t.Errorf("Details without Title/Body produced no summary:\n%s", only)
	}
}

package proseplan_test

import (
	"testing"

	pp "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-plan"
)

// validSection returns a well-formed SectionPlan.
func validSection() pp.SectionPlan {
	return pp.SectionPlan{
		Title:  "Executive Summary",
		Intent: "Summarise key findings for a non-technical executive audience.",
	}
}

// validPlan returns a well-formed ProsePlan.
func validPlan() pp.ProsePlan {
	return pp.ProsePlan{
		Sections: []pp.SectionPlan{validSection()},
	}
}

// TestSectionPlan_ValidateHappyPath ensures a well-formed section passes.
func TestSectionPlan_ValidateHappyPath(t *testing.T) {
	if err := validSection().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestSectionPlan_RequiredFields ensures Title and Intent are required.
func TestSectionPlan_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*pp.SectionPlan)
	}{
		{"empty-Title", func(s *pp.SectionPlan) { s.Title = "" }},
		{"empty-Intent", func(s *pp.SectionPlan) { s.Intent = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSection()
			tc.mutFn(&s)
			if err := s.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// TestProsePlan_ValidateHappyPath ensures a well-formed plan passes.
func TestProsePlan_ValidateHappyPath(t *testing.T) {
	if err := validPlan().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestProsePlan_EmptySections ensures a plan with no sections is rejected.
func TestProsePlan_EmptySections(t *testing.T) {
	p := pp.ProsePlan{Sections: nil}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for empty Sections, got nil")
	}
}

// TestProsePlan_DuplicateTitles ensures duplicate section titles are rejected.
func TestProsePlan_DuplicateTitles(t *testing.T) {
	sec := validSection()
	p := pp.ProsePlan{Sections: []pp.SectionPlan{sec, sec}}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for duplicate section titles, got nil")
	}
}

// TestProsePlan_PropagatesSectionError ensures section errors bubble up.
func TestProsePlan_PropagatesSectionError(t *testing.T) {
	sec := validSection()
	sec.Intent = ""
	p := pp.ProsePlan{Sections: []pp.SectionPlan{sec}}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error from invalid section, got nil")
	}
}

// TestProsePlan_MultipleSections ensures a plan with distinct valid sections
// passes.
func TestProsePlan_MultipleSections(t *testing.T) {
	p := pp.ProsePlan{
		Sections: []pp.SectionPlan{
			{Title: "Context", Intent: "Background."},
			{Title: "Decision", Intent: "The chosen approach."},
			{Title: "Consequences", Intent: "Trade-offs."},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

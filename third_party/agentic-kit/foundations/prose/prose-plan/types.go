// Package proseplan defines the structured section plan used before prose
// generation begins. A ProsePlan governs what sections appear, in what order,
// and with what intent — so generation follows a governed structure rather than
// an ad-hoc shape.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency: values are immutable and safe for concurrent use.
package proseplan

import "fmt"

// SectionPlan describes a single section to be generated within a document.
// It captures the section's title and the authoring intent that should guide
// the generator's output for that section.
//
// CUE mirror: #SectionPlan in schema.cue.
type SectionPlan struct {
	// Title is the heading or label for the section. Must be non-empty.
	Title string

	// Intent describes the purpose of this section and the expected content
	// direction (e.g. "summarise key risks for a non-technical audience").
	// Must be non-empty.
	Intent string
}

// Validate returns an error if Title or Intent is empty.
func (sp SectionPlan) Validate() error {
	if sp.Title == "" {
		return fmt.Errorf("proseplan: SectionPlan.Title must not be empty")
	}
	if sp.Intent == "" {
		return fmt.Errorf("proseplan: SectionPlan.Intent must not be empty")
	}
	return nil
}

// ProsePlan is the ordered collection of SectionPlans that defines the full
// document structure before generation begins. It is a governed artifact:
// generation must follow the plan.
//
// CUE mirror: #ProsePlan in schema.cue.
type ProsePlan struct {
	// Sections is the ordered list of sections to generate.
	// Must contain at least one section.
	Sections []SectionPlan
}

// Validate returns an error if the plan or any of its sections is invalid.
// Rules:
//   - Sections must not be empty.
//   - Each SectionPlan must pass its own Validate.
//   - Section titles must be unique within the plan.
func (p ProsePlan) Validate() error {
	if len(p.Sections) == 0 {
		return fmt.Errorf("proseplan: ProsePlan.Sections must not be empty")
	}
	seen := make(map[string]struct{}, len(p.Sections))
	for i, sec := range p.Sections {
		if err := sec.Validate(); err != nil {
			return fmt.Errorf("proseplan: ProsePlan.Sections[%d]: %w", i, err)
		}
		if _, dup := seen[sec.Title]; dup {
			return fmt.Errorf("proseplan: ProsePlan.Sections[%d]: duplicate Title %q", i, sec.Title)
		}
		seen[sec.Title] = struct{}{}
	}
	return nil
}

// Package proseplan defines the CUE-first contract for the structured section
// plan used before prose generation begins. Go types in types.go mirror these
// definitions; any drift is a bug.
package proseplan

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #SectionPlan describes a single section to be generated within a document.
#SectionPlan: {
	// Title is the heading or label for the section.
	Title: _nonEmpty

	// Intent describes the purpose of this section and expected content
	// direction.
	Intent: _nonEmpty
}

// #ProsePlan is the ordered collection of SectionPlans that defines the full
// document structure before generation begins.
#ProsePlan: {
	// Sections must contain at least one SectionPlan.
	Sections: [#SectionPlan, ...#SectionPlan]
}

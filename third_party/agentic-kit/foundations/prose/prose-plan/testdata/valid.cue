// valid.cue — concrete values that satisfy the #SectionPlan and #ProsePlan
// constraints defined in schema.cue. Running `cue vet ./...` from the package
// root validates these against the schema.
package proseplan

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-plan:proseplan"

// A well-formed single-section plan.
_plan1: proseplan.#ProsePlan & {
	Sections: [{
		Title:  "Executive Summary"
		Intent: "Summarise key findings for a non-technical executive audience."
	}]
}

// A multi-section plan for an architecture decision record.
_plan2: proseplan.#ProsePlan & {
	Sections: [
		{
			Title:  "Context"
			Intent: "Describe the current state and the problem being addressed."
		},
		{
			Title:  "Decision"
			Intent: "State the selected approach and the rationale."
		},
		{
			Title:  "Consequences"
			Intent: "List the trade-offs and anticipated impacts."
		},
	]
}

// A minimal section plan on its own.
_sec1: proseplan.#SectionPlan & {
	Title:  "Risk Register"
	Intent: "Enumerate risks ordered by severity for the risk-management team."
}

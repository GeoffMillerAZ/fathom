// valid.cue — a generic agentic-practices checklist satisfying #Checklist.
// Used by `cue vet ./...` to confirm the schema accepts well-formed data.
package agenticpracticesevaluator

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/agentic-practices-evaluator:agenticpracticesevaluator"

// InvestigationChecklistV1 is a valid checklist that satisfies #Checklist.
InvestigationChecklistV1: agenticpracticesevaluator.#Checklist & {
	Name:    "investigation-v1"
	Version: "1.0.0"
	Practices: [
		{
			Name:        "uses-evidence-citations"
			Description: "The run must produce at least one evidence citation."
		},
		{
			Name:        "records-steps"
			Description: "The run must record at least one named step."
		},
		{
			Name:        "declares-confidence"
			Description: "The run metadata must include a confidence key."
		},
	]
}

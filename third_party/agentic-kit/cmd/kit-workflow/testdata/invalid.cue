// Negative fixtures are documentation: the task CUE gate vets only positive
// fixtures, while main_test.go independently proves JSON duplicate/unknown,
// trailing-value, and byte/nesting rejection before typed decoding.
package kitworkflow

// import workflow "github.com/geoffmilleraz/agentic-kit/cmd/kit-workflow:kitworkflow"

// Unknown fields are rejected by both CUE's closed #Input and Go's
// Decoder.DisallowUnknownFields:
// invalidUnknown: workflow.#Input & {version: "kit-workflow/prose/v1", unknown: true}

// An unversioned or changed envelope cannot select policy semantics:
// invalidVersion: workflow.#Input & {version: "kit-workflow/prose/v2"}

// Duplicate JSON keys and trailing JSON values are not representable in CUE;
// Go rejects them before decoding (see TestRunRejectsStrictJSONAndBounds).

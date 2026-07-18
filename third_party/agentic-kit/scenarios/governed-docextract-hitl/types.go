// Package governeddocextracthitl is an end-to-end scenario blueprint proving
// that docextract + agent-harness + hitl compose into a governed
// document-analysis workflow where the agent proposes a change, a human
// approves it, and the state mutates ONLY through the hitl engine.
//
// Entry point: [Run]. It returns a [Report] capturing evidence from all three
// kits. The scenario is seeded, deterministic, and passes -race.
//
// # Composition
//
//	  CSV / text attachment
//	        │
//	        ▼
//	kits/docextract.Extract          ← dep-free text extraction
//	        │  extracted text
//	        ▼
//	kits/agent-harness (MockClient) ← governed loop (budget + injection screen + journal)
//	        │  propose_change tool call
//	        ▼
//	kits/hitl Engine                ← preview → approve → apply
//	        │  applied state
//	        ▼
//	    Report (asserted in tests)
//
// Governance visible:
//   - budget: a budgetguard.Guard is wired; requests are counted.
//   - injection: InjectionBlock is on; the screener runs on every tool result.
//   - journal: every loop step is recorded; the report carries journal entries.
//   - run-report: result.FinalText is asserted to mention the run.
package governeddocextracthitl

import investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"

// Op is the typed operation the agent may propose.
type Op struct {
	// Kind is the operation type; must be one of the OpSet's allowed kinds.
	Kind string

	// Field is the document field to update.
	Field string

	// Value is the new value for the field.
	Value string
}

// State is the mutable document record the hitl engine operates on.
type State struct {
	// Fields holds the document key→value pairs.
	Fields map[string]string
}

// Report is the full outcome of a single [Run] invocation. It carries evidence
// from each kit so tests can assert that all three participated.
type Report struct {
	// ExtractedText is the text produced by kits/docextract from the CSV
	// attachment. Non-empty proves docextract ran.
	ExtractedText string

	// Journal carries all harness-recorded run-trace entries. Non-empty proves
	// the governed agent loop (kits/agent-harness) ran and journalled.
	Journal []investigationjournal.Entry

	// PreviewSummary is the hitl.Preview.Summary produced by the dry-run of
	// the agent's proposed operation. Non-empty proves hitl Preview ran.
	PreviewSummary string

	// AppliedState is the document state AFTER the approved op was applied.
	// Its Fields map proves the hitl Apply committed the expected mutation.
	AppliedState State

	// RunReport is result.FinalText from the harness run. The agent is
	// scripted to produce a run-report string that mentions "run"; the test
	// asserts it is present.
	RunReport string

	// Turns is the number of model calls the harness made.
	Turns int

	// BudgetUsed is the number of successful charges the budget guard permitted.
	// Equal to Turns when no charge was denied.
	BudgetUsed int
}

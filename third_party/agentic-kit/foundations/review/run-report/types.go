// Package runreport renders a deterministic Markdown run-report from the
// inputs captured during one agent-harness run: the investigation-journal
// entries (what happened, in words), telemetry events (per-turn timing,
// tool names, injection / compaction flags), and the cost-tracker report
// (aggregate token spend).
//
// Design rationale — why foundations/review/run-report:
//
// The report is a review artefact: it is what a human (or automated quality
// gate) reads after a run to decide whether the agent behaved correctly.  All
// the other foundations in foundations/review/ (claim-lifecycle,
// concern-flags, correction-ledger, …) produce or consume data that is
// reviewed; this package produces the human-readable document that makes a
// single run reviewable.  It composes three existing foundations (telemetry,
// investigation-journal, cost-tracker) and adds no new data; a kit (kits/)
// would add the wrong layering — kits compose foundations into workflows, not
// into review artefacts.  The alternative location foundations/prose/ is for
// general prose helpers (citations, rendering policies); run-report is
// specifically about run review.
//
// API: Render(Input) ([]byte, error) is the sole entry point.  It is a pure
// function: given the same Input it returns the same bytes every time.  Each
// section degrades gracefully when its input is absent (nil / empty) and
// emits a "not captured" note rather than fabricating data.
//
// Concurrency: Render is a pure function and safe for concurrent use.
// Input values are not mutated and are treated as read-only.
package runreport

import (
	"time"

	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// Input collects the optional run artefacts that Render turns into a Markdown
// document.  Every field is optional: nil / zero values cause the corresponding
// section to emit a "not captured" note.
type Input struct {
	// RunID is the trace run identifier (event.AttrRunID).  When empty the
	// header emits "(no trace ID)".
	RunID string

	// StartedAt is the wall-clock time the run began.  Zero prints as
	// "(unknown)".
	StartedAt time.Time

	// FinishedAt is the wall-clock time the run ended.  Zero prints as
	// "(unknown)".  When both StartedAt and FinishedAt are non-zero a
	// wall-clock duration is shown in the header.
	FinishedAt time.Time

	// Model is the LLM model name used in the run.  Empty prints as
	// "(not recorded)".
	Model string

	// StopCause describes why the run ended (e.g. "completed", "budget",
	// "max_turns").  Empty prints as "(not recorded)".
	StopCause string

	// Turns is the number of model calls completed during the run.
	Turns int

	// ToolCalls is the total number of tool invocations dispatched.
	ToolCalls int

	// Journal, when non-nil, provides the sequence of investigation-journal
	// entries recorded during the run.  These become the "Agent activity"
	// section.
	Journal *investigationjournal.Journal

	// Events is the slice of telemetry events emitted during the run (e.g.
	// from a telemetry/sink.MemorySink).  Turn events, injection events, and
	// compaction events are each rendered in their own sub-section.
	Events []event.Event

	// CostReport is the string returned by costtracker.Tracker.Report().
	// When empty the cost section emits "not captured".
	CostReport string
}

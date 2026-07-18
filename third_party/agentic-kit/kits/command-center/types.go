// Package commandcenter composes workflow-engine, review-queue, and telemetry
// foundations into a cross-workflow command surface.
//
// The kit wires together:
//   - foundations/orchestration/workflow-engine (FSM-based workflow execution)
//   - foundations/review/review-queue           (prioritised human review queue)
//   - foundations/telemetry/event               (canonical telemetry event shape)
//   - foundations/telemetry/sink                (telemetry emission interface)
//
// Concurrency: CommandCenter is NOT safe for concurrent use; callers that share
// one across goroutines must serialise access externally.
package commandcenter

import (
	workflowengine "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/workflow-engine"
	reviewqueue "github.com/geoffmilleraz/agentic-kit/foundations/review/review-queue"
)

// WorkflowSpec carries the definition and initial context for a workflow that
// is submitted to the CommandCenter.
type WorkflowSpec struct {
	// Definition is the declarative workflow to run. Must have a valid ID,
	// Version, InitialState, and at least one State.
	Definition workflowengine.WorkflowDefinition

	// Context is the initial key-value context for the workflow instance.
	// May be nil.
	Context map[string]string
}

// WorkflowRecord is the CommandCenter's view of a submitted workflow.
type WorkflowRecord struct {
	// InstanceID is the stable identifier assigned by the CommandCenter.
	InstanceID string

	// Instance is the live WorkflowInstance managed by the engine.
	Instance workflowengine.WorkflowInstance

	// Engine is the workflow engine executing this instance.
	Engine *workflowengine.WorkflowEngine
}

// PendingReview is a single item waiting for human review, as returned by
// CommandCenter.Pending.
type PendingReview struct {
	// Item is the underlying review queue entry.
	Item reviewqueue.ReviewItem

	// InstanceID is the workflow instance that produced this review item.
	InstanceID string
}

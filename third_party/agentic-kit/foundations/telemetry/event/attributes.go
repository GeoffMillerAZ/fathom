package event

// Workflow telemetry vocabulary shared by producers, renderers and exporters.
// Values are wire-compatible with the original agent-harness constants.
const (
	// AttrStep classifies a workflow telemetry event.
	AttrStep = "step"

	// AttrTurn is the zero-based model-turn index.
	AttrTurn = "turn"

	// AttrStopReason is the model response stop reason.
	AttrStopReason = "stop_reason"

	// AttrToolNames lists requested tools in call order.
	AttrToolNames = "tool_names"

	// AttrDurationMS is elapsed time in milliseconds.
	AttrDurationMS = "duration_ms"

	// AttrError identifies an error class.
	AttrError = "error"

	// AttrReason describes a control decision.
	AttrReason = "reason"

	// AttrAction identifies the action taken by a control.
	AttrAction = "action"

	// AttrRuleIDs lists matching rule identifiers.
	AttrRuleIDs = "rule_ids"

	// AttrSeverity is the maximum finding severity.
	AttrSeverity = "severity"

	// AttrToolUseID correlates a tool call with its result.
	AttrToolUseID = "tool_use_id"

	// AttrStopCause identifies why a workflow stopped.
	AttrStopCause = "stop_cause"

	// AttrTurns counts completed model calls.
	AttrTurns = "turns"

	// AttrLastTool names the last requested tool.
	AttrLastTool = "last_tool"

	// AttrTried lists distinct attempted tools in first-call order.
	AttrTried = "tried"

	// AttrCheckpointRef identifies the checkpoint content.
	AttrCheckpointRef = "checkpoint_ref"

	// StepTurn marks a completed or failed model call.
	StepTurn = "turn"

	// StepBudget marks a denied budget request.
	StepBudget = "budget"

	// StepInjection marks an injection finding.
	StepInjection = "injection"

	// StepHandoff marks a workflow handoff.
	StepHandoff = "handoff"
)

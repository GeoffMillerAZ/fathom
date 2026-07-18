package agentharness

import (
	"context"
	"encoding/json"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/agent/conversation"
	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
)

// InjectionAction selects what happens when the injection detector flags a
// tool result at or above Config.InjectionMinSeverity. It only applies when
// Deps.Detector is non-nil — a nil detector disables scanning regardless.
//
// CUE mirror: #InjectionAction in schema.cue.
type InjectionAction string

const (
	// InjectionOff disables scanning even when a Detector is injected.
	InjectionOff InjectionAction = "off"

	// InjectionAnnotate keeps the flagged tool result but appends a warning
	// trailer naming the fired rules, telling the model to treat the content
	// as untrusted data rather than instructions. The raw payload DOES enter
	// the conversation under this mode.
	InjectionAnnotate InjectionAction = "annotate"

	// InjectionBlock (the default) replaces the flagged tool result with an
	// is_error tool_result naming the detection. The model sees the refusal;
	// the raw payload never re-enters the conversation.
	InjectionBlock InjectionAction = "block"
)

// StopCause reports why a Run ended. Governance stops are causes, not Go
// errors — see the package doc, "Errors vs stops".
type StopCause string

const (
	// StopCompleted: the model finished (end_turn or stop_sequence; also
	// max_tokens when Config.TreatMaxTokensAsError is explicitly false).
	StopCompleted StopCause = "completed"

	// StopMaxTurns: Config.MaxTurns model calls completed without a terminal
	// stop reason.
	StopMaxTurns StopCause = "max_turns"

	// StopBudget: the budget guard denied the pre-call charge.
	StopBudget StopCause = "budget"

	// StopRefusal: the model declined for safety reasons. Never retried.
	StopRefusal StopCause = "refusal"

	// StopMaxTokens: generation hit the MaxTokens cap and
	// Config.TreatMaxTokensAsError is true (the default).
	StopMaxTokens StopCause = "max_tokens"

	// StopCanceled: ctx was cancelled (or the client returned a
	// Canceled/DeadlineExceeded error).
	StopCanceled StopCause = "canceled"

	// StopError: Run is returning a non-nil infrastructure error.
	StopError StopCause = "error"

	// StopHook: a hook returned a non-nil error, stopping the run.
	StopHook StopCause = "hook"
)

// Config holds the data-shaped settings of a Harness. All fields are plain
// data; behavioral collaborators live in Deps.
//
// CUE mirror: #Config in schema.cue.
type Config struct {
	// MaxTurns caps the number of model calls in one Run. Required, > 0.
	MaxTurns int

	// System is the optional system prompt, carried on Request.System for
	// every model call (system prompts are not conversation messages).
	System string

	// Model names the model for every call. Empty means the injected
	// client's configured default.
	Model string

	// MaxTokensPerCall caps each response's length (Request.MaxTokens).
	// Zero means the model default. Must not be negative.
	MaxTokensPerCall int

	// TreatMaxTokensAsError controls how a max_tokens stop is surfaced.
	// nil or true (the DEFAULT): the run ends with StopMaxTokens — a
	// distinct cause, because truncated output is usually not a usable
	// answer. Explicitly false: the truncated response is accepted as a
	// normal completion (StopCompleted). In neither case does Run return a
	// Go error — truncation is a model outcome, not an infrastructure
	// failure; RunResult.FinalText carries the truncated text either way.
	TreatMaxTokensAsError *bool

	// InjectionAction is the policy applied when Deps.Detector flags a tool
	// result at or above InjectionMinSeverity. Empty defaults to
	// InjectionBlock. Ignored when Deps.Detector is nil.
	InjectionAction InjectionAction

	// InjectionMinSeverity is the severity at or above which
	// InjectionAction applies; detections below it pass through untouched.
	// Empty defaults to injectiondetector.SeverityHigh.
	InjectionMinSeverity injectiondetector.Severity

	// EstimatedUSDPerCall is what Deps.Budget.Charge is fed before every
	// model call. It is an honest ESTIMATE, not an actual: at charge time
	// the call has not happened yet, so no usage-based cost exists — the
	// guard's USD/day dimension is only as accurate as this number. Set it
	// from your price table's worst typical call; for usage-based actuals,
	// additionally wrap the client with llminstrument.WithCost (the guard
	// then provides the hard pre-call cap while the tracker records what
	// really happened). Zero is valid — the guard's request-count
	// dimensions still govern. Must not be negative.
	EstimatedUSDPerCall float64

	// ContextBudget, when > 0, is the token budget passed to
	// conversation.WindowCtx before each model call. The model sees the
	// windowed view; the conversation retains the full ground truth.
	// A compaction error stops the run (StopError) after journaling.
	ContextBudget int

	// Conversation carries optional settings forwarded to conversation.New:
	// Estimator, PreserveHead, and Compactor.
	Conversation conversation.Options

	// Hooks, when non-nil, wires synchronous hook points into the loop.
	// Hooks are on the hot path; any error stops the run with StopHook.
	Hooks *HookConfig

	// ParallelTools: 0 or 1 = sequential (default). N > 1 = dispatch up to N
	// tool calls concurrently. Results preserved in tool_use order.
	ParallelTools int
}

// HookConfig wires synchronous hook points into the governed loop. Any hook
// returning a non-nil error stops the run with StopHook.
type HookConfig struct {
	PreModelCall   func(ctx context.Context, v PreModelCallView) error
	PreDispatch    func(ctx context.Context, v ToolCallView) (allow bool, reason string, err error)
	PostToolResult func(ctx context.Context, v ToolResultView, result llmclient.ContentBlock) (llmclient.ContentBlock, error)
}

// PreModelCallView is the hook view passed to HookConfig.PreModelCall.
type PreModelCallView struct {
	Turn     int
	Messages []llmclient.Message // windowed messages the model will see
}

// ToolCallView is the hook view passed to HookConfig.PreDispatch.
type ToolCallView struct {
	Turn      int
	ToolUseID string
	ToolName  string
	Input     json.RawMessage
}

// ToolResultView is the hook view passed to HookConfig.PostToolResult.
type ToolResultView struct {
	Turn       int
	ToolUseID  string
	ToolName   string
	WasBlocked bool // true if the injection detector blocked this result
}

// Deps holds the behavioral collaborators of a Harness. Client and Registry
// are required; every other dependency is optional, and nil means the
// corresponding feature is off (see the package doc, "Seams").
type Deps struct {
	// Client is the LLM seam. REQUIRED. Inject it already wrapped with
	// whatever retry/telemetry/cost middleware the caller wants — the
	// harness performs no retries of its own.
	Client llmclient.LLMClient

	// Registry supplies tool definitions (Defs) and execution (DispatchAll).
	// REQUIRED — registries with zero tools are fine for text-only agents.
	Registry *toolregistry.Registry

	// Budget, when non-nil, is charged Config.EstimatedUSDPerCall before
	// every model call; a denial stops the run with StopBudget. Sharing one
	// Guard across harnesses/runs shares the budget — that is the point.
	Budget *budgetguard.Guard

	// Detector, when non-nil, scans every tool result before it re-enters
	// the conversation, per Config.InjectionAction.
	Detector *injectiondetector.Detector

	// Sink, when non-nil, receives one telemetry event per loop step (plus
	// budget-denial and injection events). Emit failures are swallowed.
	Sink sink.Sink

	// Journal, when non-nil, records a best-effort run trace. NOT safe for
	// concurrent Runs — see the package doc, "Concurrency contract".
	Journal *investigationjournal.Journal

	// Now is the injected clock for event timestamps, durations, and journal
	// entries. Nil falls back to time.Now; always inject in tests.
	Now func() time.Time
}

// RunResult is the outcome of one Run or RunStream (after draining). It is
// always meaningful — including when Run returns a non-nil error (Stop is
// then StopError and Messages holds the transcript up to the failure).
type RunResult struct {
	// FinalText is the text of the LAST assistant message: the final answer
	// on StopCompleted, the refusal text on StopRefusal, the truncated text
	// on StopMaxTokens, and possibly empty on stops that interrupt the loop
	// between model calls (budget, max_turns, canceled).
	FinalText string

	// Stop is why the run ended.
	Stop StopCause

	// Turns is the number of model calls that completed successfully.
	Turns int

	// ToolCalls is the total number of tool invocations dispatched.
	ToolCalls int

	// Usage is the sum of every model call's token accounting.
	Usage llmclient.Usage

	// Messages is the final transcript (a fresh copy, oldest first) — ready
	// to persist, inspect, or replay.
	Messages []llmclient.Message

	// Final is the Checkpoint captured at the end of this run — useful for
	// multi-turn session stitching and crash/budget-stop resume. It is always
	// populated (non-nil), even on error returns (Stop == StopError).
	Final *Checkpoint
}

// checkpointVersion is the format version written into every Checkpoint's JSON.
// Restore rejects any version it does not recognise.
const checkpointVersion = 1

// Checkpoint is a serialisable snapshot of all loop state needed to resume a
// run. It is embedded in every RunResult.Final and may be fed back to
// RunConversation/RunStream to continue from where a previous run stopped.
type Checkpoint struct {
	// Version is the format version. Currently 1. Required.
	Version int `json:"version"`

	// ConvSnapshot is the JSON encoding of the conversation at the point the
	// run ended (foundations/agent/conversation.Conversation.Snapshot format,
	// version 1). Required.
	ConvSnapshot []byte `json:"conv_snapshot"`

	// BudgetSnapshot, when non-nil, is the serialised budgetguard.Snapshot
	// captured at the end of the run. Only present when Deps.Budget was wired.
	BudgetSnapshot *budgetguard.Snapshot `json:"budget_snapshot,omitempty"`

	// TurnCursor is the zero-based index of the NEXT turn that would have
	// been executed. Equal to RunResult.Turns on a normal run end.
	TurnCursor int `json:"turn_cursor"`
}

// HarnessEventKind discriminates the variants of HarnessEvent.
type HarnessEventKind string

const (
	// HarnessEventTextDelta is an incremental piece of assistant text. Only
	// emitted by RunStream; Run callers receive the full text in RunResult.FinalText.
	HarnessEventTextDelta HarnessEventKind = "text_delta"

	// HarnessEventTurnStart is emitted at the beginning of each model call,
	// before the call is made. Turn contains the zero-based turn index.
	HarnessEventTurnStart HarnessEventKind = "turn_start"

	// HarnessEventTurnComplete is emitted after each successful model call.
	// Turn, StopReason, ToolCalls (names dispatched), and DurationMS are set.
	HarnessEventTurnComplete HarnessEventKind = "turn_complete"

	// HarnessEventToolResult is emitted after each tool result is screened and
	// added to the conversation. ToolUseID and ToolResultContent are set.
	HarnessEventToolResult HarnessEventKind = "tool_result"

	// HarnessEventBudgetDenied is emitted when the budget guard denies a
	// pre-call charge. Reason is set. The terminal HarnessEventResult follows.
	HarnessEventBudgetDenied HarnessEventKind = "budget_denied"

	// HarnessEventInjection is emitted when the injection detector fires on a
	// tool result. Action, RuleIDs, Severity, and ToolUseID are set.
	HarnessEventInjection HarnessEventKind = "injection"

	// HarnessEventResult is the TERMINAL event. Result carries the full
	// RunResult. After this event the channel closes. Err is non-nil when the
	// run ended with an infrastructure error (RunResult.Stop == StopError).
	HarnessEventResult HarnessEventKind = "result"

	// HarnessEventCompaction is emitted when context windowing drops messages.
	// DroppedMessages is set to the count of messages removed.
	HarnessEventCompaction HarnessEventKind = "compaction"

	// HarnessEventHookDenied is emitted when a PreDispatch hook denies a tool
	// call (the run continues — a denial result is sent back to the model).
	// HookStep and HookReason are set.
	HarnessEventHookDenied HarnessEventKind = "hook_denied"
)

// HarnessEvent is one event on the channel returned by RunStream. See
// HarnessEventKind for the full set. Per event, the fields relevant to its
// Kind are populated and all others are zero.
type HarnessEvent struct {
	// Kind discriminates which fields are meaningful.
	Kind HarnessEventKind

	// TextDelta is the incremental text (HarnessEventTextDelta).
	TextDelta string

	// Turn is the zero-based turn index (TurnStart, TurnComplete).
	Turn int

	// StopReason is the model's stop reason (TurnComplete).
	StopReason llmclient.StopReason

	// ToolNames is the comma-joined list of tool names dispatched (TurnComplete,
	// when tools were called).
	ToolNames string

	// DurationMS is the model call's wall-clock duration in milliseconds
	// (TurnComplete).
	DurationMS int64

	// ToolUseID is the tool_use ID (ToolResult, Injection).
	ToolUseID string

	// ToolResultContent is the screened tool result content (ToolResult).
	ToolResultContent string

	// ToolResultIsError is true when the tool result is an is_error block.
	ToolResultIsError bool

	// Action is the injection action taken: "block" or "annotate" (Injection).
	Action string

	// RuleIDs is the comma-joined fired rule IDs (Injection).
	RuleIDs string

	// Severity is the detection's maximum severity (Injection).
	Severity string

	// Reason is the budget denial reason (BudgetDenied).
	Reason string

	// Result is the final RunResult (HarnessEventResult). Always non-nil on
	// the terminal event.
	Result *RunResult

	// Err is the infrastructure error on a StopError run (HarnessEventResult).
	// Nil on all governance stops (budget, max_turns, refusal, etc.).
	Err error

	// DroppedMessages is the count of messages dropped by context compaction
	// (HarnessEventCompaction).
	DroppedMessages int

	// HookStep names the hook that was triggered (HarnessEventHookDenied).
	HookStep string

	// HookReason is the hook's denial reason (HarnessEventHookDenied).
	HookReason string
}

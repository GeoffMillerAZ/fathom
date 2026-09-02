package agentharness

import (
	"context"
	"fmt"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

// EffectPolicy selects how the harness treats a tool's declared
// toolregistry.Effect at dispatch time. It is orthogonal to the registry's
// own Policy (name/argument allow-deny) and to the PreDispatch hook: those
// say WHETHER a tool may be called; the effect policy says whether an
// IRREVERSIBLE call needs a human token first.
//
// CUE mirror: #EffectPolicy in schema.cue.
type EffectPolicy string

const (
	// EffectPermissive (the default; also the empty string) dispatches every
	// call the registry and the hooks allow, whatever its effect. This is the
	// pre-Effect behavior, unchanged: effects are advisory.
	EffectPermissive EffectPolicy = "permissive"

	// EffectStrict routes every dispatch whose effect is UNDECLARED
	// (Tool.Effect nil) or DESTRUCTIVE (Effect.Destructive true) through
	// RequireApproval before it runs. A denied or absent approval returns an
	// is_error tool_result to the model — the loop continues — and records a
	// gate_denied event. A tool that declares a read-only or explicitly
	// non-destructive effect dispatches exactly as under EffectPermissive.
	// The gate is PER CALL by design: a tool approved once is not approved
	// for its next call.
	EffectStrict EffectPolicy = "strict"
)

// Attribute keys and values stamped on effect-gate telemetry events, in
// addition to AttrStep, AttrTurn, AttrToolUseID and AttrReason.
const (
	// AttrToolName is the tool the gate decided on (gate events).
	AttrToolName = "tool_name"

	// AttrEffect is why the gate applied: EffectUndeclared or
	// EffectDestructive (gate events).
	AttrEffect = "effect"

	// AttrApprovedBy is Approval.ApprovedBy on a gate_approved event; absent
	// when the approver left it empty.
	AttrApprovedBy = "approved_by"

	// EffectUndeclared is the AttrEffect value for a tool with no declared
	// Effect — the strictest reading, per docs/intent/decisions/0002.
	EffectUndeclared = "undeclared"

	// EffectDestructive is the AttrEffect value for a tool whose declared
	// Effect reads as destructive.
	EffectDestructive = "destructive"
)

// AttrStep values for the effect gate.
const (
	// StepGateDenied marks a strict effect gate denying a dispatch: no
	// approver was wired, the approver denied, or the approver failed. The
	// model receives an is_error tool_result and the run continues.
	StepGateDenied = "gate_denied"

	// StepGateApproved marks a strict effect gate granting a dispatch; the
	// approver's ApprovedBy is carried for audit.
	StepGateApproved = "gate_approved"
)

// ApprovalRequest is what the strict effect gate hands the Approver: the
// pending call, its declared effect (nil = undeclared), and a kits/hitl
// Preview — the approval card shape hitl consumers already render
// (Summary → title). The harness cannot dry-run a tool, so Preview.Changes
// is always empty and Preview.OK is true: the card describes what WOULD run,
// not what it would change. Preview.Summary names the tool, the effect label
// and a bounded rendering of the input.
type ApprovalRequest struct {
	// Call is the pending tool call as the PreDispatch hook sees it.
	Call ToolCallView

	// Effect is the tool's declared effect; nil means undeclared, which is
	// why the gate fired.
	Effect *toolregistry.Effect

	// Preview is the hitl approval card for this call.
	Preview hitl.Preview
}

// Approval is an Approver's decision on one ApprovalRequest. The zero value
// is a denial with no reason — the fail-closed reading.
type Approval struct {
	// Approved grants exactly this dispatch. It carries no authority over the
	// tool's next call.
	Approved bool

	// Reason is surfaced to the model on a denial ("tool call denied by
	// effect gate: <Reason>") and recorded on the gate event either way.
	Reason string

	// ApprovedBy names the human or system that granted the approval, for
	// the audit event. Empty is allowed but discouraged.
	ApprovedBy string
}

// Approver is the human-in-the-loop channel a strict effect gate composes:
// the harness proposes, the Approver decides, the harness dispatches only on
// Approved. Implementations own the transport (a CLI prompt, an approval
// endpoint, a queue); they must honour ctx and must be safe for concurrent
// use if the Harness is used for concurrent Runs. A non-nil error is treated
// as a DENIAL, never as an approval.
type Approver interface {
	Approve(ctx context.Context, req ApprovalRequest) (Approval, error)
}

// ApproverFunc adapts a plain function to the Approver interface.
type ApproverFunc func(ctx context.Context, req ApprovalRequest) (Approval, error)

// Approve calls f.
func (f ApproverFunc) Approve(ctx context.Context, req ApprovalRequest) (Approval, error) {
	return f(ctx, req)
}

// RequireApproval routes one tool call through Deps.Approver and returns
// the decision. It is the strict effect gate's action and is exported so a
// consumer running its own dispatch can gate the same way. It applies no
// policy of its own — it asks for EVERY call it is given; the policy that
// decides which calls need asking is EffectStrict in the loop.
//
// Fail-closed: with no Approver wired the result is a denial whose Reason
// says so; an Approver error is returned AND the Approval is a denial
// carrying the error text, so a caller that ignores the error still cannot
// dispatch. A denial with an empty Reason is given a generic one.
func (h *Harness) RequireApproval(ctx context.Context, call ToolCallView) (Approval, error) {
	var eff *toolregistry.Effect
	if h.deps.Registry != nil {
		eff, _ = h.deps.Registry.EffectOf(call.ToolName)
	}
	req := ApprovalRequest{Call: call, Effect: eff, Preview: previewFor(call, eff)}

	if h.deps.Approver == nil {
		return Approval{Reason: "no Approver wired (Deps.Approver is nil; a strict effect policy needs one to grant approvals)"}, nil
	}
	a, err := h.deps.Approver.Approve(ctx, req)
	if err != nil {
		return Approval{Reason: "approver failed: " + err.Error()}, err
	}
	if !a.Approved && a.Reason == "" {
		a.Reason = "approval denied"
	}
	return a, nil
}

// gateLabel reports whether the strict gate applies to the named tool and
// why. An unregistered tool is not gated: nothing can happen, and Dispatch
// reports "unknown tool" to the model as it always has.
func (h *Harness) gateLabel(name string) (label string, gated bool) {
	if _, exists := h.deps.Registry.Get(name); !exists {
		return "", false
	}
	eff, declared := h.deps.Registry.EffectOf(name)
	switch {
	case !declared:
		return EffectUndeclared, true
	case eff.Destructive():
		return EffectDestructive, true
	}
	return "", false
}

// effectLabelOf is gateLabel's vocabulary for an ApprovalRequest: the label
// the gate fired on, or "declared" for a call a consumer chose to route
// through RequireApproval itself.
func effectLabelOf(eff *toolregistry.Effect) string {
	switch {
	case eff == nil:
		return EffectUndeclared
	case eff.Destructive():
		return EffectDestructive
	}
	return "declared"
}

// previewFor builds the hitl approval card for one call.
func previewFor(call ToolCallView, eff *toolregistry.Effect) hitl.Preview {
	input := "{}"
	if len(call.Input) > 0 {
		input = truncateNote(string(call.Input))
	}
	return hitl.Preview{
		Summary: fmt.Sprintf("dispatch tool %q (effect: %s) with input %s", call.ToolName, effectLabelOf(eff), input),
		OK:      true,
	}
}

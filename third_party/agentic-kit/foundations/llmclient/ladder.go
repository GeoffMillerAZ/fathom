package llmclient

import "fmt"

// Rung represents one model tier in the Ladder.
type Rung struct {
	// Model is the identifier passed to the backend (e.g. "claude-haiku-4-5").
	// Apply injects it into requests whose Model is empty, which is how a
	// Ladder composes with any LLMClient.
	Model string

	// ContextLimit is the maximum number of tokens this rung's model accepts
	// (prompt + completion combined).
	ContextLimit int

	// CostWeight is a relative cost indicator. Lower is cheaper.
	// Used by callers to track expected spend; the Ladder itself does not
	// enforce budget limits — that responsibility belongs to the caller.
	CostWeight float64

	// QualityThreshold is the minimum quality score (0–1) a caller expects
	// at this rung before escalating. The Ladder itself does not evaluate
	// quality — it is up to the caller to measure quality and decide whether
	// to emit an Escalate signal.
	QualityThreshold float64
}

// Apply returns a copy of req with the rung's Model injected when req.Model is
// empty. A non-empty req.Model always wins — the caller explicitly chose a
// model, and the rung must not override it. This is how a Ladder composes with
// the LLMClient interface:
//
//	resp, err := client.Complete(ctx, ladder.Current().Apply(req))
func (r Rung) Apply(req Request) Request {
	if req.Model == "" {
		req.Model = r.Model
	}
	return req
}

// Signal carries the escalation decision from the caller back to the Ladder.
type Signal struct {
	// Escalate is true when the caller decides the current rung is insufficient
	// and the Ladder should advance to the next rung.
	Escalate bool

	// Reason is a human-readable description of why the escalation was triggered,
	// useful for logging and audit records.
	Reason string
}

// Ladder is a caller-ordered escalation sequence for a particular task and role.
// It starts at rung 0 and advances only when the caller emits an escalation Signal.
// The order is policy, not a universal model ranking: the caller must establish
// deployment eligibility and measured suitability before constructing the rungs.
//
// A Response with StopReason == StopToolUse is a successful completion — it is
// never an escalation signal. Escalate on measured quality, not on the model
// choosing to call tools.
//
// Ladder is not safe for concurrent use without external synchronisation.
type Ladder struct {
	rungs   []Rung
	current int
}

// NewLadder constructs a Ladder from rungs in the caller's preferred escalation
// order. At least one rung is required. It does not check availability or quality.
func NewLadder(rungs ...Rung) (*Ladder, error) {
	if len(rungs) == 0 {
		return nil, fmt.Errorf("llmclient: NewLadder requires at least one rung")
	}
	cp := make([]Rung, len(rungs))
	copy(cp, rungs)
	return &Ladder{rungs: cp, current: 0}, nil
}

// Current returns the Rung the Ladder is presently positioned at.
func (l *Ladder) Current() Rung {
	return l.rungs[l.current]
}

// CurrentIndex returns the zero-based index of the current rung.
func (l *Ladder) CurrentIndex() int {
	return l.current
}

// Len returns the total number of rungs in the Ladder.
func (l *Ladder) Len() int {
	return len(l.rungs)
}

// Climb evaluates the Signal and, if escalation is requested and a higher rung
// exists, advances the Ladder and returns (newRung, true).
//
// If Signal.Escalate is false, the current rung is returned unchanged and the
// second return value is false.
//
// If already at the top rung and Signal.Escalate is true, the top rung is
// returned and the second return value is false — the caller knows no further
// escalation is possible.
func (l *Ladder) Climb(sig Signal) (Rung, bool) {
	if !sig.Escalate {
		return l.rungs[l.current], false
	}
	// Already at the top.
	if l.current >= len(l.rungs)-1 {
		return l.rungs[l.current], false
	}
	l.current++
	return l.rungs[l.current], true
}

// Reset moves the Ladder back to rung 0.
func (l *Ladder) Reset() {
	l.current = 0
}

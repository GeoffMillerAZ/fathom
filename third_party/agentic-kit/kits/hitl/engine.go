// Package hitl is a generic human-in-the-loop engine for agents that CHANGE
// data. The model may only PROPOSE typed operations; the engine dry-runs each
// into a preview for a human, and commits it only after approval — the model
// never has a write path.
//
// The load-bearing invariant: preview and apply are driven by the SAME Applier,
// so the effect a human approves is byte-for-byte what commits. Mutation is
// bounded to a whitelisted OpSet whose kinds also emit the tool-schema enum
// (see OpSet), so the allowed set and the schema the model sees can never drift.
//
// The engine is transport- and storage-agnostic (loopback-safe): the app parses
// tool calls into Ops, owns the approval endpoint, and owns the lock/persistence
// around Apply. It pairs with app-kit appkit/ui/hitl to render the preview
// (Engine.Preview → hitl.ApprovalCard: Summary → Title, Changes → Metrics).
//
//	eng := hitl.New(applyToState, cloneState,
//	    hitl.WithGauges[State, Op](readMetrics),
//	    hitl.WithGuard[State, Op](hitl.Guard(opSet, Op.kindOf)))
//	pv := eng.Preview(store.Snapshot(), op) // dry-run for the approval card
//	// … human approves …
//	summary, err := eng.Apply(store.Live(), op) // caller holds lock + persists
//
// Concurrency: Engine itself is safe for concurrent use (it is immutable after
// construction); the caller must serialise Apply calls on any single live-state
// value, and the injected Applier, Cloner, and gauge functions must be safe for
// concurrent use.
package hitl

// Applier applies one op to a mutable state, returning a human-readable summary
// of what it did (shown on the approval card) or an error if the op is invalid.
// THE SAME Applier drives both Preview (on a clone) and Apply (on live state),
// which is what makes an approved preview identical to what commits. It must be
// deterministic and must not perform I/O.
type Applier[State any, Op any] func(st *State, op Op) (summary string, err error)

// Cloner returns a deep copy of state so a preview can mutate it freely without
// touching the caller's live state. For a value-type State with no reference
// fields, `func(s State) State { return s }` suffices; anything holding slices,
// maps, or pointers MUST copy them.
type Cloner[State any] func(State) State

// Gauge is one named metric read from a state snapshot for impact display, e.g.
// {Key: "weekly", Label: "Weekly tuition", Value: "$8,304"}.
type Gauge struct {
	Key   string
	Label string
	Value string
}

// Change is a before→after delta of a gauge, emitted only when the value moved.
// It maps directly onto app-kit appkit/ui/hitl.Metric{Label, From, To}.
type Change struct {
	Key   string
	Label string
	From  string
	To    string
}

// Preview is the result of dry-running an op: the summary the Applier produced,
// the before→after Changes, and — if the op was rejected or invalid — Err. OK is
// true only when the op previewed cleanly and is safe to offer for approval.
type Preview struct {
	Summary string
	Changes []Change
	Err     string
	OK      bool
}

// Engine dry-runs and applies typed operations against an app's State. Construct
// it with New; it is safe for concurrent use as long as the Applier/Cloner/gauge
// funcs are (they should be pure), and as long as the caller serializes Apply on
// the live state.
type Engine[State any, Op any] struct {
	apply  Applier[State, Op]
	clone  Cloner[State]
	gauges func(State) []Gauge
	allow  func(Op) error
}

// Option configures an Engine.
type Option[State any, Op any] func(*Engine[State, Op])

// WithGauges supplies a function that reads impact metrics from a state, used to
// compute the before→after Changes on a preview. Without it, previews carry a
// summary but no metric deltas.
func WithGauges[State any, Op any](f func(State) []Gauge) Option[State, Op] {
	return func(e *Engine[State, Op]) { e.gauges = f }
}

// WithGuard supplies a whitelist gate rejected ops fail on (see Guard). Both
// Preview and Apply consult it, so a disallowed op can never reach the Applier.
func WithGuard[State any, Op any](allow func(Op) error) Option[State, Op] {
	return func(e *Engine[State, Op]) { e.allow = allow }
}

// New builds an Engine from the single Applier (shared by preview and apply) and
// a Cloner used to protect live state during previews.
func New[State any, Op any](apply Applier[State, Op], clone Cloner[State], opts ...Option[State, Op]) *Engine[State, Op] {
	e := &Engine[State, Op]{apply: apply, clone: clone}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Preview dry-runs op against a CLONE of snapshot and returns the summary plus
// before→after Changes, without mutating snapshot. A guard rejection or Applier
// error is returned in Err (OK=false) so the UI can show why it can't be applied.
func (e *Engine[State, Op]) Preview(snapshot State, op Op) Preview {
	if e.allow != nil {
		if err := e.allow(op); err != nil {
			return Preview{Err: err.Error()}
		}
	}
	before := e.readGauges(snapshot)
	work := e.clone(snapshot)
	summary, err := e.apply(&work, op)
	if err != nil {
		return Preview{Err: err.Error()}
	}
	after := e.readGauges(work)
	return Preview{Summary: summary, Changes: diff(before, after), OK: true}
}

// PreviewAll dry-runs several ops independently against the same snapshot (each
// on its own clone), returning one Preview per op in order.
func (e *Engine[State, Op]) PreviewAll(snapshot State, ops []Op) []Preview {
	out := make([]Preview, len(ops))
	for i, op := range ops {
		out[i] = e.Preview(snapshot, op)
	}
	return out
}

// Apply commits op to *st IN PLACE using the same Applier that produced the
// preview, after re-checking the guard. The caller owns concurrency and
// persistence: hold the write lock across Apply and persist *st on success.
func (e *Engine[State, Op]) Apply(st *State, op Op) (string, error) {
	if e.allow != nil {
		if err := e.allow(op); err != nil {
			return "", err
		}
	}
	return e.apply(st, op)
}

func (e *Engine[State, Op]) readGauges(s State) []Gauge {
	if e.gauges == nil {
		return nil
	}
	return e.gauges(s)
}

// diff pairs before/after gauges by Key (in `after` order) and emits a Change
// for each whose Value moved.
func diff(before, after []Gauge) []Change {
	prev := make(map[string]string, len(before))
	for _, g := range before {
		prev[g.Key] = g.Value
	}
	var out []Change
	for _, g := range after {
		from, ok := prev[g.Key]
		if ok && from == g.Value {
			continue
		}
		out = append(out, Change{Key: g.Key, Label: g.Label, From: from, To: g.Value})
	}
	return out
}

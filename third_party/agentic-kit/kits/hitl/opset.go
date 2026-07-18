package hitl

import "fmt"

// OpSet is the whitelist of operation kinds an agent is allowed to propose. Its
// Kinds() feed BOTH the tool-schema enum the model sees (so it can only propose
// allowed ops) AND Allows() (so the engine rejects anything else) — one source
// of truth, so the schema and the enforced whitelist can never drift.
//
//	ops := hitl.NewOpSet("add_student", "set_last_day", "record_check")
//	// schema: the model only ever sees these kinds
//	schema := toolregistry.ObjectSchema(toolregistry.Prop{
//	    Name: "op", Type: "string", Required: true, Enum: ops.Kinds()})
//	// engine: reject anything not in the set
//	eng := hitl.New(apply, clone, hitl.WithGuard[S, Op](hitl.Guard(ops, kindOf)))
type OpSet struct {
	kinds []string
	set   map[string]bool
}

// NewOpSet builds a whitelist from the allowed kinds, preserving their order
// (so the emitted schema enum is deterministic). Duplicate or empty kinds panic
// — a malformed whitelist is a programming bug that should fail at startup.
func NewOpSet(kinds ...string) *OpSet {
	s := &OpSet{set: make(map[string]bool, len(kinds))}
	for _, k := range kinds {
		if k == "" {
			panic("hitl: NewOpSet: empty kind")
		}
		if s.set[k] {
			panic(fmt.Sprintf("hitl: NewOpSet: duplicate kind %q", k))
		}
		s.set[k] = true
		s.kinds = append(s.kinds, k)
	}
	return s
}

// Kinds returns the allowed kinds in declaration order — feed this to the tool
// schema's op enum so the model only ever sees allowed operations.
func (s *OpSet) Kinds() []string {
	out := make([]string, len(s.kinds))
	copy(out, s.kinds)
	return out
}

// Allows reports whether kind is in the whitelist.
func (s *OpSet) Allows(kind string) bool { return s.set[kind] }

// Guard adapts an OpSet + a kind extractor into the allow func WithGuard wants,
// rejecting any op whose kind is not whitelisted. (A free function, not a
// method, because Go methods cannot take type parameters.)
func Guard[Op any](s *OpSet, kindOf func(Op) string) func(Op) error {
	return func(op Op) error {
		k := kindOf(op)
		if !s.Allows(k) {
			return fmt.Errorf("hitl: operation %q is not allowed", k)
		}
		return nil
	}
}

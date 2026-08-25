package reachfix

// A and B share a method NAME (Extract) on different receivers — the VS-5
// ambiguity shape: a bare-name resolver must never let a test calling one
// flip reachability for the other.
type A struct{}

func (A) Extract() string { return "A" }

// NewA constructs an A.
func NewA() A { return A{} }

type B struct{}

func (B) Extract() string { return "B" }

// NewB constructs a B. Deliberately unused by any test in this fixture —
// TestAExtract must never reach it.
func NewB() B { return B{} }

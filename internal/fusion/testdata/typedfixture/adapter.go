package typedfixture

// Adapter mirrors the ADR-021 §3 adapter-style interface residue case (O9):
// a Complete method implemented by more than one concrete adapter,
// previously refused by the syntax-only resolver as an ambiguous method
// name (internal/analysis/callgraph/graph.go:218-240). Org-agnostic
// (ADR-011): this shape stands in for the real kit's LLM-adapter pattern.
type Adapter interface {
	Complete(prompt string) string
}

// AdapterA is the first concrete implementer.
type AdapterA struct{}

// Complete implements Adapter.
func (AdapterA) Complete(prompt string) string { return "A:" + prompt }

// AdapterB is the second concrete implementer.
type AdapterB struct{}

// Complete implements Adapter.
func (AdapterB) Complete(prompt string) string { return "B:" + prompt }

// RunAdapter calls Complete through the interface — an interface-dispatch
// call site with two in-scope implementers.
func RunAdapter(a Adapter) string {
	return a.Complete("x")
}

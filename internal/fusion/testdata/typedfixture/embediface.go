package typedfixture

// Wrapper embeds the Adapter INTERFACE (not a struct): its method set
// satisfies Adapter through the embedded interface VALUE, so its Complete
// is the interface's own method declaration, not a concrete one — the F6
// phantom-key shape. The typed pass must never emit an edge to a
// "Adapter.Complete"-style key (goextract creates no method node for an
// interface), and must never offer Wrapper as a dispatch implementer (the
// true runtime target — whatever the embedded field holds — is statically
// unknowable).
type Wrapper struct {
	Adapter
}

// UseWrapper calls the promoted Complete through the embedded interface —
// dynamic dispatch whose fan-out must go to the REAL implementers
// (AdapterA, AdapterB), never to a phantom interface-method key.
func UseWrapper(w Wrapper) string {
	return w.Complete("z")
}

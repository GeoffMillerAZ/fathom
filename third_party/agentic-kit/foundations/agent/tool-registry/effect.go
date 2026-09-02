package toolregistry

// Effect declares what a tool does to its environment, in the MCP
// tool-annotation vocabulary VERBATIM — readOnlyHint, destructiveHint,
// idempotentHint, openWorldHint (MCP ToolAnnotations, byte-identical across
// spec revisions 2025-03-26 through 2026-07-28) — so the kit invents no
// second taxonomy: a declared Effect round-trips out through an MCP server's
// tools/list annotations (kits/mcp-server) and back in from a client's
// tools/list (foundations/agent/mcp-client) without translation.
//
// Semantics follow the MCP schema comments:
//
//   - ReadOnlyHint: the tool does not modify its environment.
//   - DestructiveHint: the tool may perform destructive (non-additive)
//     updates; meaningful only when ReadOnlyHint is false.
//   - IdempotentHint: repeated calls with the same arguments have no
//     additional effect; meaningful only when ReadOnlyHint is false.
//   - OpenWorldHint: the tool interacts with an open world of external
//     entities (the web, other agents) rather than a closed domain.
//
// Nil versus zero. Tool.Effect == nil means UNDECLARED — the strictest
// reading: a strict effect policy (kits/agent-harness EffectStrict) treats it
// as destructive, mirroring CapabilitySet's nil-is-unknown rule. An explicit
// &Effect{} (every hint false) is a DECLARATION — "additive, not destructive,
// closed world" — and is taken at its word, as DeclareNone is. ReadOnlyHint
// and DestructiveHint both true is a pair the MCP schema leaves undefined;
// the kit reads it fail-closed as destructive (see Destructive).
//
// An Effect is self-description, exactly like Capabilities: a tool that lies
// about its effect defeats it. What a declaration buys is STRUCTURAL — a
// harness can refuse to dispatch an undeclared or destructive tool without a
// human token. It makes nothing reversible, sandboxes nothing, and cannot see
// an effect a tool did not declare.
type Effect struct {
	// ReadOnlyHint: the tool does not modify its environment.
	ReadOnlyHint bool

	// DestructiveHint: the tool may perform destructive (non-additive)
	// updates to its environment. Only meaningful when ReadOnlyHint is false.
	DestructiveHint bool

	// IdempotentHint: calling the tool repeatedly with the same arguments has
	// no additional effect. Only meaningful when ReadOnlyHint is false.
	IdempotentHint bool

	// OpenWorldHint: the tool may interact with an open world of external
	// entities; false means its domain of interaction is closed.
	OpenWorldHint bool
}

// Destructive reports whether a dispatch under this declared effect may be
// irreversible: DestructiveHint is set. ReadOnlyHint does not override it —
// the contradictory pair reads fail-closed. Destructive is defined on the
// value, not the pointer, so it can never answer for an UNDECLARED tool:
// a caller holding a nil *Effect must treat the absence as destructive
// itself.
func (e Effect) Destructive() bool { return e.DestructiveHint }

// EffectOf returns a copy of the Effect declared for the named tool. ok is
// false — and the pointer nil — when the tool is not registered OR is
// registered with no declaration (Tool.Effect nil). Callers that need to
// tell the two apart use Get. The copy is the caller's to mutate; the
// registered declaration is immutable after Register.
func (r *Registry) EffectOf(name string) (*Effect, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, exists := r.tools[name]
	if !exists || t.Effect == nil {
		return nil, false
	}
	e := *t.Effect
	return &e, true
}

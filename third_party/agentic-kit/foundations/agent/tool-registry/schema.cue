// Package toolregistry defines the CUE contract for the one DATA-shaped part
// of a tool registration: #Effect, the tool's declared effect on its
// environment. Everything else a Tool carries — the handler, the timeout,
// the policy hook — is behavior and lives only in Go (types in registry.go).
//
// #Effect's field names are the MCP ToolAnnotations vocabulary VERBATIM
// (readOnlyHint, destructiveHint, idempotentHint, openWorldHint), so a value
// that unifies here is the exact object kits/mcp-server emits as a tool's
// `annotations` and foundations/agent/mcp-client reads back. The Go mirror
// is the Effect struct in effect.go (Go-cased field names, same order).
package toolregistry

// #Effect declares what a tool does to its environment. Every hint is
// optional here, as on the wire; the MCP defaults for an absent hint are
// readOnlyHint=false, destructiveHint=true, idempotentHint=false,
// openWorldHint=true, and mcp-client applies exactly those when it reads
// annotations in. The Go type is a value struct where an absent hint is
// false, which is why kits/mcp-server always emits all four explicitly.
#Effect: {
	// readOnlyHint: the tool does not modify its environment.
	readOnlyHint?: bool

	// destructiveHint: the tool may perform destructive (non-additive)
	// updates. Meaningful only when readOnlyHint is false; a strict policy
	// reads readOnlyHint=true together with destructiveHint=true as
	// destructive (fail-closed).
	destructiveHint?: bool

	// idempotentHint: repeated calls with the same arguments have no
	// additional effect. Meaningful only when readOnlyHint is false.
	idempotentHint?: bool

	// openWorldHint: the tool may interact with an open world of external
	// entities; false means a closed domain.
	openWorldHint?: bool
}

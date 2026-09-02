package mcpclient

import (
	"encoding/json"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
)

// effectFromAnnotations maps one tool's MCP `annotations` object onto a
// toolregistry.Effect. It returns nil — UNDECLARED, which a strict harness
// reads as destructive — when annotations are absent, JSON null, or
// malformed: a parse failure never yields a permissive effect (the
// "parse-failure-means-allow" anti-pattern). Absent hints take the MCP
// schema defaults, which are themselves the strict reading: readOnlyHint
// false, destructiveHint TRUE, idempotentHint false, openWorldHint TRUE. So
// a server that sends `{}` has declared a destructive, open-world tool, and
// only an explicit `"destructiveHint": false` (or `"readOnlyHint": true`
// with it) reads as safe.
//
// Trust is the CALLER's decision, not this function's: MCP requires clients
// to treat annotations as untrusted unless the server is trusted, so
// ListTools marks the result with Options.TrustedServer and RegisterAll
// attaches it to the registered Tool only when that flag is set.
func effectFromAnnotations(raw json.RawMessage) *toolregistry.Effect {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var a struct {
		ReadOnlyHint    *bool `json:"readOnlyHint"`
		DestructiveHint *bool `json:"destructiveHint"`
		IdempotentHint  *bool `json:"idempotentHint"`
		OpenWorldHint   *bool `json:"openWorldHint"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil
	}
	eff := &toolregistry.Effect{
		ReadOnlyHint:    false,
		DestructiveHint: true,
		IdempotentHint:  false,
		OpenWorldHint:   true,
	}
	if a.ReadOnlyHint != nil {
		eff.ReadOnlyHint = *a.ReadOnlyHint
	}
	if a.DestructiveHint != nil {
		eff.DestructiveHint = *a.DestructiveHint
	}
	if a.IdempotentHint != nil {
		eff.IdempotentHint = *a.IdempotentHint
	}
	if a.OpenWorldHint != nil {
		eff.OpenWorldHint = *a.OpenWorldHint
	}
	return eff
}

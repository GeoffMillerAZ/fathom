// Package mcp — the served protocol contract (docs/plans/reach-parity.md
// RP-10..RP-12, docs/vision/25-harness-port.md §1): version negotiation,
// tool annotations, output schemas. A client that never reads CLAUDE.md
// must be able to learn from tools/list alone what each tool does, whether
// it writes, and what shape comes back.
package mcp

import "sort"

// supportedProtocolVersions is the table of MCP protocol revisions this
// server speaks, newest first. All four are "legacy era" revisions in the
// 2026-07-28 spec's terminology — the `initialize` handshake, one JSON
// response per POST — which is exactly what serve-http and serve-mcp
// implement: no per-request `_meta`, no `server/discover`, no SSE
// streaming. 2026-07-28 is deliberately absent — claiming it would promise
// the stateless-core RPCs this server does not serve; a client asking for
// it is answered with the newest supported version, per the legacy
// negotiation rule (the client then disconnects or falls back).
var supportedProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

// SupportedProtocolVersions returns the supported revisions, newest first
// (a copy; callers may not mutate the table).
func SupportedProtocolVersions() []string {
	return append([]string(nil), supportedProtocolVersions...)
}

// NegotiateProtocolVersion is negotiateProtocolVersion for the HTTP layer
// (cmd/fathom's serve-http echoes the MCP-Protocol-Version header with it).
func NegotiateProtocolVersion(requested string) string {
	return negotiateProtocolVersion(requested)
}

// negotiateProtocolVersion applies the 2025-11-25 lifecycle rule (RP-12):
// "If the server supports the requested protocol version, it MUST respond
// with the same version. Otherwise, the server MUST respond with another
// protocol version it supports. This SHOULD be the latest version
// supported by the server." An empty or unknown request therefore yields
// the newest supported revision; the MCP-Protocol-Version header on later
// HTTP requests is tolerated, never a rejection — the HTTP layer runs it
// through this same function to decide what to echo back.
func negotiateProtocolVersion(requested string) string {
	for _, v := range supportedProtocolVersions {
		if v == requested {
			return v
		}
	}
	return supportedProtocolVersions[0]
}

// toolAnnotations returns the MCP ToolAnnotations (readOnlyHint,
// destructiveHint, idempotentHint, openWorldHint — field names unchanged
// from 2025-03-26 through 2026-07-28) for a tool, or nil when the tool's
// behaviour has not been classified (a client MUST then assume the spec
// defaults: may write, may be destructive). Every agent-surface tool is
// classified (RP-10); the classification is per-tool truth read from what
// the handler does, not a blanket rule:
//
//   - ingest triggers a store write (re-index / onboard a service): not
//     read-only, idempotent (hash-gated: a clean repo is a no-op), never
//     destructive (it only adds or refreshes observations).
//   - obligations serves rules read-only BUT also carries the D2 claim
//     feedback write path (action:"refute"|"corroborate" flips a claim's
//     status): not read-only, idempotent (re-refuting is a no-op), not
//     destructive (a refuted claim stays listed, only its status changes).
//     The plan text's "16 read-only" counted it as read-only; the handler
//     says otherwise, and the served annotation follows the handler.
//   - everything else on the agent surface only reads the index; every
//     tool's domain is the local index (openWorldHint:false) — nothing on
//     this surface reaches the network.
func toolAnnotations(name string) map[string]any {
	readOnly := func() map[string]any {
		return map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
	}
	switch name {
	case "ingest", "obligations":
		return map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
	}
	if companionTools[name] {
		return readOnly()
	}
	return nil
}

// annotatedTools returns the sorted names of every tool toolAnnotations
// classifies — the agent-surface allowlist today.
func annotatedTools() []string {
	out := make([]string, 0, len(companionTools))
	for name := range companionTools {
		if toolAnnotations(name) != nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

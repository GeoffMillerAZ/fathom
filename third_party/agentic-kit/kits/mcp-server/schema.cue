// Package mcpserver defines the CUE contract for the mcp-server kit Config
// and HTTPOptions. These constraints mirror the data-shaped fields of the Go
// structs in types.go and http.go and are evaluated with `cue vet ./...`.
// CallLog (io.Writer) and RandHex (func) are behavioral seams wired in Go,
// not data — they have no CUE shape.
package mcpserver

import "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry:toolregistry"

// #ToolAnnotations is the `annotations` object a tools/list entry carries
// for a tool with a declared Effect. It IS toolregistry.#Effect — the kit
// invents no second taxonomy — restated here so the wire contract and the
// registry declaration are provably one shape. The Go side always emits all
// four hints explicitly (see types.go toolAnnotations).
#ToolAnnotations: toolregistry.#Effect

// #Config configures an MCP stdio server.
#Config: {
	// Name is the server name reported in the initialize serverInfo.
	Name: string & !=""

	// Version is the server version reported in the initialize serverInfo.
	Version: string & !=""

	// MaxMessageBytes bounds one newline-delimited JSON-RPC message.
	// Optional: omit it to take the Go default of 4 MiB (4194304). When
	// present it must be at least 1; an explicit 0 is a misconfiguration —
	// omit the field instead.
	MaxMessageBytes?: int & >=1

	// LogPayloads opts the call log into recording tools/call arguments and
	// results. SENSITIVE: payloads carry user data — enable only when the
	// log's storage and retention are scoped for it. Optional; the Go kit
	// defaults it to false (payloads are never logged by default).
	LogPayloads?: bool
}

// #HTTPOptions configures the Streamable HTTP handler (NewHTTPHandler).
// RandHex — the injectable randomness seam for server-side child spans — is
// wired in Go and has no CUE shape, the same precedent as #Config's CallLog.
#HTTPOptions: {
	// MaxBodyBytes bounds one POST body. Optional: omit it to inherit the
	// server's MaxMessageBytes (itself defaulting to 4 MiB). When present it
	// must be at least 1; an explicit 0 is a misconfiguration — omit the
	// field instead.
	MaxBodyBytes?: int & >=1

	// AllowedOrigins lists the exact Origin header values the handler
	// accepts (DNS-rebinding protection, a 2025-11-25 Streamable HTTP MUST).
	// Optional and FAIL-CLOSED: omitting it (or listing nothing) rejects
	// every Origin-bearing request with 403; requests without an Origin
	// header (non-browser clients) always pass. Entries are compared
	// verbatim and must be non-empty.
	AllowedOrigins?: [...string & !=""]
}

// _rfc3339 constrains a string to the RFC3339 timestamp format Go's
// time.Time marshals to (e.g. "2026-07-28T00:00:00Z").
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #ProtocolRevision is an MCP protocol revision id: the date of the last
// backwards-incompatible change, YYYY-MM-DD. The Go constant ProtocolRevision
// ("2025-11-25") is the revision this kit implements.
#ProtocolRevision: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"

// #Deprecation is one row of the retirement ledger Deprecations() returns —
// the same shape foundations/agent/mcp-client exports. RemovableAfter is
// optional: an absent floor means the MCP deprecated-features registry
// publishes no date, and DeprecationsDue treats an undated floor on a
// still-implemented feature as due immediately (fail closed).
#Deprecation: {
	// Feature is the retired feature, named as the MCP changelog names it.
	Feature: string & !=""

	// DeprecatedIn is the release date of the revision that retired the
	// feature (MCP revisions are named by date, so this is the revision).
	DeprecatedIn: _rfc3339

	// RemovableAfter is the earliest date on or after which a revision may
	// remove the feature: twelve months after DeprecatedIn under the MCP
	// feature-lifecycle policy. Omitted when the registry publishes no date.
	RemovableAfter?: _rfc3339

	// StillImplemented reports whether this package still speaks the feature.
	StillImplemented: bool
}

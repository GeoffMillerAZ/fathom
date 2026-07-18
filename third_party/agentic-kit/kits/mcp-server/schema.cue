// Package mcpserver defines the CUE contract for the mcp-server kit Config
// and HTTPOptions. These constraints mirror the data-shaped fields of the Go
// structs in types.go and http.go and are evaluated with `cue vet ./...`.
// CallLog (io.Writer) and RandHex (func) are behavioral seams wired in Go,
// not data — they have no CUE shape.
package mcpserver

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

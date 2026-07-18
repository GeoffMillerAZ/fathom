// Package mcpserver — valid test data that must unify with schema.cue.
package mcpserver

// The explicit ":mcpserver" qualifier is required: CUE cannot derive a
// package name from the hyphenated final path element.
import "github.com/geoffmilleraz/agentic-kit/kits/mcp-server:mcpserver"

// validConfig is a fully specified server configuration with an explicit
// message bound and payload logging left off (the safe default).
validConfig: mcpserver.#Config & {
	Name:            "demo-tools"
	Version:         "1.2.3"
	MaxMessageBytes: 4194304
	LogPayloads:     false
}

// validConfigMinimal omits every optional field: MaxMessageBytes defaults to
// 4 MiB in Go, LogPayloads to false, and the call log stays disabled (CallLog
// is a Go-side io.Writer, never present in data configs).
validConfigMinimal: mcpserver.#Config & {
	Name:    "minimal"
	Version: "0.1.0"
}

// validConfigPayloadLogging opts into payload logging — only for deployments
// whose log storage is scoped for SENSITIVE user data.
validConfigPayloadLogging: mcpserver.#Config & {
	Name:            "audited-tools"
	Version:         "2.0.0"
	MaxMessageBytes: 1048576
	LogPayloads:     true
}

// validHTTPOptions is a fully specified Streamable HTTP configuration: an
// explicit body bound and a browser-origin allowlist.
validHTTPOptions: mcpserver.#HTTPOptions & {
	MaxBodyBytes: 4194304
	AllowedOrigins: ["http://localhost:8080", "https://tools.example.test"]
}

// validHTTPOptionsMinimal omits every optional field: MaxBodyBytes inherits
// the server's MaxMessageBytes, and the absent AllowedOrigins list is the
// FAIL-CLOSED default — every Origin-bearing request is rejected with 403
// while header-less (non-browser) clients pass. RandHex is a Go-side func
// seam, never present in data configs.
validHTTPOptionsMinimal: mcpserver.#HTTPOptions & {}

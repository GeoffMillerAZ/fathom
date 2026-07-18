// http.go — the MCP Streamable HTTP transport: a single-POST, stateless
// handler over the same dispatch core as the stdio Serve loop (serveOne).
// Design record: docs/designs/mcp-streamable-http.md. See doc.go,
// "Streamable HTTP transport" for the full contract, including what is
// deliberately NOT provided (SSE, sessions, TLS/auth).

package mcpserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// mcpProtocolVersionHeader is the HTTP request header carrying the MCP
// protocol revision (2025-11-25 spec, Streamable HTTP §Protocol Version
// Header). Header names are matched case-insensitively by net/http.
const mcpProtocolVersionHeader = "MCP-Protocol-Version"

// HTTPOptions configures NewHTTPHandler. The CUE contract for the
// data-shaped fields (MaxBodyBytes, AllowedOrigins) lives in schema.cue;
// RandHex is a behavioral seam with no CUE shape, the same precedent as
// Config.CallLog.
type HTTPOptions struct {
	// MaxBodyBytes bounds one POST body. Zero or negative means the Server's
	// resolved MaxMessageBytes (which itself defaults to
	// DefaultMaxMessageBytes). An oversized body is rejected with HTTP 200 +
	// JSON-RPC -32700 carrying a best-effort id recovered from the retained
	// prefix — the same policy as the stdio transport, enforced via
	// http.MaxBytesReader instead of readLine.
	MaxBodyBytes int

	// AllowedOrigins lists the exact Origin header values this handler
	// accepts (scheme://host[:port], compared verbatim). Origin validation
	// is the DNS-rebinding protection the 2025-11-25 Streamable HTTP spec
	// makes a MUST: a request bearing an Origin header not in this list is
	// answered 403 Forbidden with an empty body, before any body read.
	//
	// FAIL-CLOSED DEFAULT: an empty list rejects ALL Origin-bearing
	// requests. Requests without an Origin header (non-browser clients such
	// as the kit's own HTTP transport) always pass.
	AllowedOrigins []string

	// RandHex supplies the randomness used to derive the server-side child
	// span from an inbound traceparent header (event.TraceContext.Child).
	// nil means crypto/rand, per the event package's documented default.
	// Inject a deterministic func(nBytes int) string in tests.
	RandHex func(nBytes int) string
}

// NewHTTPHandler returns the MCP Streamable HTTP endpoint for s: a single
// stateless POST handler with the same dispatch semantics as Serve (both
// call serveOne, so JSON-RPC response bytes are identical across the two
// transports).
//
// Per request it: (1) validates the Origin header per opts.AllowedOrigins
// (403 on mismatch, before any body read); (2) rejects non-POST methods with
// 405; (3) checks MCP-Protocol-Version — an absent header assumes 2025-03-26
// per the spec's backwards-compatibility rule (behavior is identical across
// all known revisions for this tools-only surface), an unknown or malformed
// value is answered 400 Bad Request; (4) reads the body under the resolved
// byte bound; (5) extracts an inbound traceparent header, if any, onto the
// request context; (6) dispatches via serveOne. A request yields HTTP 200
// with Content-Type: application/json and one JSON-RPC response object; a
// notification yields 202 Accepted with an empty body. The request Accept
// header is deliberately ignored: this JSON-only stateless server always
// answers application/json and never opens an SSE stream, which is this
// implementation's stateless-mode reading of the 2025-11-25 Accept
// requirement (the client-side MUST governs what clients advertise, and a
// server choosing the application/json response path satisfies it).
//
// Concurrency: the returned handler is safe for concurrent requests — the
// dispatch core is stateless, the registry is concurrency-safe, the call-log
// writer is mutex-guarded, and the tool_use id counter is atomic. Note the
// contrast with stdio Serve, which processes one request at a time.
//
// NewHTTPHandler PANICS when s is nil, in the spirit of New's nil-registry
// panic: a handler without a server is a programming bug that should fail at
// startup.
func NewHTTPHandler(s *Server, opts HTTPOptions) http.Handler {
	if s == nil {
		panic("mcpserver: NewHTTPHandler: server must be non-nil")
	}
	allowedOrigins := make(map[string]bool, len(opts.AllowedOrigins))
	for _, o := range opts.AllowedOrigins {
		allowedOrigins[o] = true
	}
	maxBody := opts.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = s.max
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Origin validation first, before any body read (DNS-rebinding
		// protection; 2025-11-25 Streamable HTTP security MUST). Empty
		// allowlist = reject every Origin-bearing request (fail closed).
		if origin := r.Header.Get("Origin"); origin != "" && !allowedOrigins[origin] {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if r.Method != http.MethodPost {
			// No GET SSE stream in stateless mode: 405 is the spec's
			// prescribed answer when the server offers no stream.
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// MCP-Protocol-Version (2025-11-25, §Protocol Version Header):
		// absent → assume 2025-03-26 per the spec's backwards-compatibility
		// rule (no behavioral difference for this tools-only surface, so the
		// assumption is a no-op); invalid or unsupported → 400 Bad Request,
		// a spec MUST.
		if v := r.Header.Get(mcpProtocolVersionHeader); v != "" && !knownProtocolVersions[v] {
			http.Error(w, fmt.Sprintf("unsupported MCP-Protocol-Version %q", v), http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(maxBody)))
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				// Oversized body: HTTP 200 + JSON-RPC -32700 with a
				// best-effort id from the retained prefix — the stdio
				// oversized-message policy on the HTTP mechanism. Resync is
				// free here: the connection's next request starts clean.
				writeJSONRPCResponse(w, rpcResponse{
					JSONRPC: "2.0",
					ID:      bestEffortID(body),
					Error: &rpcError{
						Code:    ErrCodeParse,
						Message: fmt.Sprintf("message exceeds maximum size of %d bytes", maxBody),
					},
				})
				return
			}
			// Transport read failure mid-body: there is no recoverable
			// message to answer; fail closed at the HTTP layer.
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Inbound trace (W3C traceparent): a valid header roots the
		// server-side work as a child span of the caller's context, so one
		// RunID spans the process boundary. Absent or malformed header →
		// no trace, never an error (fail-closed degradation to "untraced").
		ctx := r.Context()
		if tc, perr := event.ParseTraceparent(r.Header.Get(event.TraceparentHeader)); perr == nil {
			ctx = event.WithTrace(ctx, tc.Child(opts.RandHex))
		}

		resp := s.serveOne(ctx, body)
		if resp == nil {
			// Notification (or blank body): 202 Accepted, empty body, per
			// the Streamable HTTP spec. Stdio equivalently emits nothing.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSONRPCResponse(w, *resp)
	})
}

// writeJSONRPCResponse writes one JSON-RPC response object as an HTTP 200
// application/json body, encoded by the same encodeResponse the stdio
// transport uses (identical bytes, no trailing newline).
func writeJSONRPCResponse(w http.ResponseWriter, resp rpcResponse) {
	b := encodeResponse(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// readBufSize is the bufio.Reader buffer size. It bounds per-read memory, not
// message size — readLine accumulates fragments up to the configured maximum.
const readBufSize = 32 * 1024

// nullID is the JSON-RPC null id used when a request's id is unrecoverable.
var nullID = json.RawMessage("null")

// Server serves a tool registry over the MCP stdio transport. Construct with
// New; the zero value is not usable. See the package doc for the full
// protocol and concurrency contracts.
type Server struct {
	cfg     Config
	reg     *toolregistry.Registry
	now     func() time.Time
	callLog *jsonllog.Writer // nil when Config.CallLog is nil
	max     int              // resolved MaxMessageBytes
	callSeq atomic.Int64     // synthesized tool_use id counter
}

// New constructs a Server over reg. cfg.Name and cfg.Version populate the
// initialize serverInfo; cfg.MaxMessageBytes <= 0 means
// DefaultMaxMessageBytes. now is the injected clock used for call-log
// timestamps and durations; nil means time.Now.
//
// New PANICS when reg is nil — a server without a registry is a programming
// bug that should fail at startup, in the spirit of MustRegister.
func New(cfg Config, reg *toolregistry.Registry, now func() time.Time) *Server {
	if reg == nil {
		panic("mcpserver: New: registry must be non-nil")
	}
	if now == nil {
		now = time.Now
	}
	max := cfg.MaxMessageBytes
	if max <= 0 {
		max = DefaultMaxMessageBytes
	}
	s := &Server{cfg: cfg, reg: reg, now: now, max: max}
	if cfg.CallLog != nil {
		s.callLog = jsonllog.NewWriter(cfg.CallLog)
	}
	return s
}

// Serve reads newline-delimited JSON-RPC 2.0 messages from in, dispatches
// them one at a time in arrival order, and writes responses to out (one line
// per response, serialized by a mutex). It returns nil on EOF, ctx.Err() when
// ctx is cancelled, and a non-nil error on transport failure — including a
// failed resync after an oversized message (fail closed; see doc.go, "Input
// bounds and resync").
//
// Cancellation is observed between messages: a Read blocked on a quiet stream
// is not interrupted (stdio has no read deadline), which is the standard
// stdio-server caveat — clients end the session by closing the stream.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	var outMu sync.Mutex
	writeResp := func(resp rpcResponse) error {
		b := encodeResponse(resp)
		outMu.Lock()
		defer outMu.Unlock()
		if _, werr := out.Write(append(b, '\n')); werr != nil {
			return fmt.Errorf("mcpserver: write response: %w", werr)
		}
		return nil
	}

	br := bufio.NewReaderSize(in, readBufSize)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, tooLong, err := readLine(br, s.max)
		if tooLong {
			// Oversized message: reject before parsing, with a best-effort id
			// recovered from the retained prefix. readLine has already tried
			// to resync to the next newline; err reports how that went.
			werr := writeResp(rpcResponse{
				JSONRPC: "2.0",
				ID:      bestEffortID(line),
				Error: &rpcError{
					Code:    ErrCodeParse,
					Message: fmt.Sprintf("message exceeds maximum size of %d bytes", s.max),
				},
			})
			if werr != nil {
				return werr
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				// Clean resync impossible: fail closed.
				return fmt.Errorf("mcpserver: cannot resync after oversized message: %w", err)
			}
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("mcpserver: read: %w", err)
		}

		resp := s.serveOne(ctx, line)
		if resp == nil {
			// Blank line or notification: nothing to write.
			continue
		}
		if werr := writeResp(*resp); werr != nil {
			return werr
		}
	}
}

// serveOne processes one raw JSON-RPC message that the transport has already
// framed and size-bounded, and returns the response to send back — or nil
// when the message warrants no response (blank/whitespace-only input, or a
// notification). It is the transport-agnostic core shared by the stdio Serve
// loop and the Streamable HTTP handler (http.go): batch rejection, parse
// errors, notification detection, dispatch, and the response envelope all
// live here, so the two transports cannot drift.
func (s *Server) serveOne(ctx context.Context, msg []byte) *rpcResponse {
	msg = bytes.TrimSpace(msg)
	if len(msg) == 0 {
		return nil
	}
	if msg[0] == '[' {
		// JSON-RPC batch arrays are deliberately unsupported; see doc.go.
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      nullID,
			Error:   &rpcError{Code: ErrCodeInvalid, Message: "batch requests are not supported"},
		}
	}

	var req rpcRequest
	if uerr := json.Unmarshal(msg, &req); uerr != nil {
		code, errMsg := ErrCodeParse, "parse error: "+uerr.Error()
		if json.Valid(msg) {
			// Well-formed JSON that is not a request object (e.g. a bare
			// scalar) is an invalid request, not a parse error.
			code, errMsg = ErrCodeInvalid, "invalid request: not a JSON-RPC object"
		}
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      nullID,
			Error:   &rpcError{Code: code, Message: errMsg},
		}
	}

	// Notifications (absent or literal-null id) get NO response.
	if isNotification(req.ID) {
		return nil
	}

	result, rpcErr := s.serveReq(ctx, req)
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	return resp
}

// encodeResponse marshals resp to its wire JSON. When the result fails to
// marshal it falls back in-band to an internal-error envelope that contains
// only primitives and cannot itself fail. Both transports encode through
// here, so the bytes on a stdio line and in an HTTP body are identical.
func encodeResponse(resp rpcResponse) []byte {
	b, err := json.Marshal(resp)
	if err != nil {
		b, _ = json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      resp.ID,
			Error:   &rpcError{Code: ErrCodeInternal, Message: "marshal response: " + err.Error()},
		})
	}
	return b
}

// isNotification reports whether the raw id marks a notification: absent, or
// the literal null that JSON-RPC 2.0 discourages but some clients send.
func isNotification(id json.RawMessage) bool {
	return len(id) == 0 || string(id) == "null"
}

// serveReq dispatches one request and, when a call log is configured, records
// the outcome with timestamps from the injected clock.
func (s *Server) serveReq(ctx context.Context, req rpcRequest) (any, *rpcError) {
	if s.callLog == nil {
		result, rpcErr, _ := s.dispatch(ctx, req)
		return result, rpcErr
	}
	start := s.now()
	result, rpcErr, meta := s.dispatch(ctx, req)
	s.logCall(start, s.now().Sub(start), req.Method, meta, rpcErr)
	return result, rpcErr
}

// readLine returns the next newline-delimited line from br, without its line
// ending (a trailing "\r\n" or "\n" is trimmed; a final unterminated line at
// EOF is returned as-is, then the next call reports io.EOF).
//
// When the line exceeds max bytes, readLine returns (prefix, true, err) where
// prefix is the first max bytes (kept for best-effort id recovery) and the
// rest of the line has been discarded up to and including the next '\n' —
// the resync point. err is non-nil only when that discard failed: io.EOF when
// the stream ended mid-line (benign), or the transport's read error (the
// caller must fail closed — the stream position is no longer on a message
// boundary).
//
// Memory is bounded by max + the bufio buffer regardless of input.
func readLine(br *bufio.Reader, max int) (line []byte, tooLong bool, err error) {
	var buf []byte
	for {
		frag, ferr := br.ReadSlice('\n')
		buf = append(buf, frag...)
		switch {
		case ferr == nil:
			l := trimEOL(buf)
			if len(l) > max {
				// Delimiter already consumed — resync is complete.
				return l[:max], true, nil
			}
			return l, false, nil
		case errors.Is(ferr, bufio.ErrBufferFull):
			if len(buf) > max {
				return buf[:max], true, discardToNewline(br)
			}
			// Keep accumulating fragments.
		case errors.Is(ferr, io.EOF):
			if len(buf) == 0 {
				return nil, false, io.EOF
			}
			l := trimEOL(buf)
			if len(l) > max {
				// Stream ended mid-line; nothing left to resync past.
				return l[:max], true, nil
			}
			return l, false, nil
		default:
			return nil, false, ferr
		}
	}
}

// discardToNewline consumes input up to and including the next '\n', using
// only the bufio buffer. It returns nil once the delimiter is consumed,
// io.EOF when the stream ends first, or the transport's read error.
func discardToNewline(br *bufio.Reader) error {
	for {
		_, err := br.ReadSlice('\n')
		switch {
		case err == nil:
			return nil
		case errors.Is(err, bufio.ErrBufferFull):
			// Keep discarding.
		case errors.Is(err, io.EOF):
			return io.EOF
		default:
			return err
		}
	}
}

// trimEOL strips one trailing "\n" and then one trailing "\r" (CRLF
// tolerance for clients on Windows).
func trimEOL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	if n := len(b); n > 0 && b[n-1] == '\r' {
		b = b[:n-1]
	}
	return b
}

// bestEffortID attempts to recover the "id" of an oversized message from its
// retained prefix, so the -32700 error can carry it. It streams JSON tokens
// across the (truncated) prefix looking for a top-level "id" key with a
// string or number value; anything unrecoverable — id past the truncation
// point, non-object message, malformed prefix — falls back to null, which is
// what the JSON-RPC spec prescribes for undeterminable ids.
func bestEffortID(prefix []byte) json.RawMessage {
	dec := json.NewDecoder(bytes.NewReader(prefix))
	tok, err := dec.Token()
	if err != nil {
		return nullID
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nullID
	}
	for {
		keyTok, err := dec.Token()
		if err != nil {
			return nullID
		}
		key, ok := keyTok.(string)
		if !ok {
			return nullID // closing '}' — no id key present
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nullID // value truncated before completing
		}
		if key == "id" {
			if len(val) > 0 && (val[0] == '"' || val[0] == '-' || (val[0] >= '0' && val[0] <= '9')) {
				return val
			}
			return nullID // null/object/array id — not echoable
		}
	}
}

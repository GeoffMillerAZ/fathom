// httptransport.go — MCP Streamable HTTP client transport.
//
// Speaks the single-POST Streamable HTTP transport (2025-11-25 revision,
// stateless mode): every JSON-RPC message is POSTed to one endpoint; a
// request's response arrives as the HTTP response body; notifications are
// acknowledged with 202/204 and carry no body. Requests send
// "Accept: application/json, text/event-stream" per the 2025-11-25 client
// requirement (for interop with non-kit servers), but SSE response bodies
// are NOT consumed — a "text/event-stream" response is a typed failure,
// not a stream (see non-guarantees below).
//
// Trace propagation (roadmap #21): Send serialises the ambient
// event.TraceContext from ctx (event.TraceFrom) into a W3C traceparent
// header via event.FormatTraceparent. A missing or zero trace context means
// no header is sent — never an error, never an all-zero header.
//
// Non-guarantees (stated plainly): no SSE streaming responses, no
// mcp-session-id / resumability, no retries, and no TLS or auth beyond what
// the injected Doer provides. Server-initiated notifications are impossible
// over this stateless transport.
//
// Concurrency: httpTransport is NOT safe for concurrent use as a transport —
// the Send→Receive pairing must be serialized by the caller; Client does so
// via its internal mutex (the same contract as ioTransport). Individual
// methods are internally mutex-guarded only so that Close may race a
// straggling Send/Receive without a data race.

package mcpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// Doer is the minimal HTTP execution seam over which the HTTP transport
// issues requests. *http.Client satisfies it; tests inject fakes or
// httptest-backed clients.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// ErrResponseTooLarge is wrapped into the error returned by Send when the
// HTTP response body exceeds the transport's message-size bound.
var ErrResponseTooLarge = errors.New("mcpclient: response exceeds maximum message size")

// HTTPStatusError is returned by Send when the server answers with an HTTP
// status other than 200 OK, 202 Accepted, or 204 No Content.
type HTTPStatusError struct {
	StatusCode int
}

// Error implements the error interface, reporting the unexpected HTTP status
// code.
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("mcpclient: unexpected HTTP status %d", e.StatusCode)
}

// httpTransport implements Transport over MCP Streamable HTTP. See the file
// comment for the transport and concurrency contract.
type httpTransport struct {
	endpoint string
	doer     Doer
	max      int

	mu     sync.Mutex // guards queue and closed (see file comment)
	queue  [][]byte
	closed bool
}

// NewHTTPTransport constructs a Transport that POSTs each message to
// endpoint using doer and queues response bodies for Receive.
// maxMessageBytes bounds one received response body; zero means
// DefaultMaxMessageBytes (4 MiB, matching kits/mcp-server). A nil doer
// defaults to http.DefaultClient (documented nil-default; inject an
// httptest client in tests).
//
// Semantics per message: a 200 response's body is read (bounded) and queued
// for the next Receive; 202/204 (notification acknowledgements) queue
// nothing; any other status returns *HTTPStatusError from Send. Receive
// pops the oldest queued body or errors when the queue is empty. Close is
// idempotent; Send and Receive after Close return errors.
func NewHTTPTransport(endpoint string, doer Doer, maxMessageBytes int) Transport {
	if doer == nil {
		doer = http.DefaultClient
	}
	max := maxMessageBytes
	if max <= 0 {
		max = DefaultMaxMessageBytes
	}
	return &httpTransport{
		endpoint: endpoint,
		doer:     doer,
		max:      max,
	}
}

// Send POSTs msg to the endpoint with Content-Type application/json and
// Accept "application/json, text/event-stream". When ctx carries a non-zero
// event.TraceContext, its W3C traceparent serialisation is set on the
// request (header name event.TraceparentHeader); a zero or absent trace
// context sends no header. The response body (if any) is queued for
// Receive; see NewHTTPTransport for the status-code semantics.
func (t *httpTransport) Send(ctx context.Context, msg []byte) error {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return errors.New("mcpclient: transport closed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("mcpclient: send: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if tc, ok := event.TraceFrom(ctx); ok {
		if tp, ok := event.FormatTraceparent(tc); ok {
			req.Header.Set(event.TraceparentHeader, tp)
		}
	}

	resp, err := t.doer.Do(req)
	if err != nil {
		return fmt.Errorf("mcpclient: send: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Response body follows below.
	case http.StatusAccepted, http.StatusNoContent:
		// Notification acknowledgement: nothing to queue. Drain (bounded)
		// so the underlying connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, int64(t.max)))
		return nil
	default:
		return &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "text/event-stream") {
		return errors.New("mcpclient: server answered text/event-stream; SSE responses are not supported by this transport")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.max)+1))
	if err != nil {
		return fmt.Errorf("mcpclient: send: read response: %w", err)
	}
	if len(body) > t.max {
		return fmt.Errorf("mcpclient: response exceeds maximum size of %d bytes: %w", t.max, ErrResponseTooLarge)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("mcpclient: transport closed")
	}
	t.queue = append(t.queue, body)
	return nil
}

// Receive pops the oldest queued response body. It never blocks: over this
// transport a response is produced synchronously by the preceding Send, so
// an empty queue is a protocol-usage error, not a wait condition.
func (t *httpTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, errors.New("mcpclient: transport closed")
	}
	if len(t.queue) == 0 {
		return nil, errors.New("mcpclient: receive: no queued response (Receive without a preceding request Send)")
	}
	msg := t.queue[0]
	t.queue = t.queue[1:]
	return msg, nil
}

// Close marks the transport closed and drops any queued responses. It is
// idempotent and always returns nil; subsequent Send/Receive calls return
// errors. The injected Doer is not closed — the caller owns it.
func (t *httpTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.queue = nil
	return nil
}

package llmclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// CallRecord captures the details of a single call made to MockClient.
type CallRecord struct {
	Request Request
	// Attempt is the 1-based attempt number within a WithRetry chain.
	// When MockClient is used directly (not wrapped), Attempt is always 0.
	Attempt int
}

// MockEntry is one scripted response or error for MockClient.
type MockEntry struct {
	// Response is returned if Err is nil.
	Response Response

	// Err is returned instead of Response when non-nil. For Stream it is a
	// pre-stream error (returned directly, no channel is created).
	Err error

	// StreamErr scripts a MID-stream failure: Stream emits the entry's text
	// deltas and blocks, then a terminal chunk with Err = StreamErr, then
	// closes the channel. Complete treats it like Err (a mid-stream failure
	// means the full completion failed). Ignored when Err is set.
	StreamErr error
}

// MockClient is a deterministic, scriptable LLMClient for use in tests.
//
// Behaviour:
//   - Complete and Stream consume entries from the Entries slice in order.
//   - When Entries is exhausted, a fallback response with text
//     "mock: no more entries" is returned (no error).
//   - Scripted responses are normalised so they look like real provider
//     output: Content is derived from Text (and vice versa) when only one is
//     set, and an empty StopReason defaults to StopEndTurn — or StopToolUse
//     when the content contains a tool_use block.
//   - Stream emits text as two or more TextDelta chunks (one chunk when the
//     text is a single rune), non-text blocks as complete Block chunks, then
//     the terminal chunk, honouring ctx throughout. See the package doc,
//     "Streaming semantics".
//   - All calls are recorded in Calls for later assertion.
//   - Requests violating the contract's documented request limits are
//     rejected with a typed InvalidRequestError BEFORE any entry is consumed
//     or call recorded, mirroring the real provider adapters' pre-HTTP
//     validation, so tests cannot pass requests a live client would refuse:
//     more than MaxCacheBreakpoints total cache breakpoints (counted exactly
//     as the anthropic adapter counts them — SystemCacheHint only with a
//     non-empty System, CacheHint on BlockUnknown blocks ignored),
//     ToolChoiceTool with an empty Name, a non-object ProviderParams, and a
//     non-object OutputSchema.
//
// # OutputSchema and structured outputs
//
// MockClient validates that OutputSchema, when non-empty, is a JSON object
// (the same check the real adapters perform). Responses are ALWAYS
// caller-scripted via MockEntry — MockClient never generates output and
// therefore never constrains generation to the schema. Tests that exercise the
// full CUE→schema→constrained-generation loop must use a real adapter backed
// by an httptest server; the end-to-end proof lives in
// toolchains/jsonschema/jsonschemagen_test.go.
//
// Concurrency: MockClient is safe for concurrent use.
type MockClient struct {
	mu      sync.Mutex
	entries []MockEntry
	calls   []CallRecord
	cursor  int
}

// NewMockClient creates a MockClient pre-loaded with the given entries.
func NewMockClient(entries ...MockEntry) *MockClient {
	return &MockClient{entries: append([]MockEntry(nil), entries...)}
}

// AddEntry appends a scripted response or error.
func (m *MockClient) AddEntry(e MockEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
}

// Calls returns a snapshot of all recorded calls.
func (m *MockClient) Calls() []CallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]CallRecord, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// CallCount returns the number of calls made so far.
func (m *MockClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// Reset clears recorded calls and resets the cursor to the beginning of the
// entry list without discarding the entries themselves.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = m.calls[:0]
	m.cursor = 0
}

// validateRequest enforces the documented request limits the real provider
// adapters reject before any HTTP, so MockClient cannot accept a request the
// live client would refuse (the mock→real seam stays pinned). The breakpoint
// counting mirrors the anthropic adapter exactly: SystemCacheHint counts only
// when System is non-empty, and CacheHint on BlockUnknown blocks is ignored
// (verbatim passthrough wins, no cache_control can attach).
func validateRequest(req Request) error {
	n := 0
	if req.System != "" && req.SystemCacheHint {
		n++
	}
	for _, t := range req.Tools {
		if t.CacheHint {
			n++
		}
	}
	for _, m := range req.Messages {
		for _, b := range m.Content {
			if b.CacheHint && b.Type != BlockUnknown {
				n++
			}
		}
	}
	if n > MaxCacheBreakpoints {
		return &InvalidRequestError{Message: fmt.Sprintf(
			"mock: %d cache breakpoints requested across tools+system+blocks; the contract allows at most %d per request",
			n, MaxCacheBreakpoints,
		)}
	}
	if req.ToolChoice.Mode == ToolChoiceTool && req.ToolChoice.Name == "" {
		return &InvalidRequestError{Message: `mock: ToolChoice.Mode is "tool" but ToolChoice.Name is empty`}
	}
	if err := ValidateProviderParams(req.ProviderParams, "mock"); err != nil {
		return err
	}
	if err := ValidateOutputSchema(req.OutputSchema, "mock"); err != nil {
		return err
	}
	// Effort is recorded verbatim: a provider-neutral scripted mock does not
	// choose a provider's effort vocabulary or simulate reasoning quality.
	return nil
}

func (m *MockClient) next(req Request) MockEntry {
	m.calls = append(m.calls, CallRecord{Request: req})
	if m.cursor >= len(m.entries) {
		return MockEntry{Response: Response{Text: "mock: no more entries"}}
	}
	e := m.entries[m.cursor]
	m.cursor++
	return e
}

// normalize fills the derived fields of a scripted Response so entries behave
// like real provider output. Fields the script set explicitly are never
// overwritten — only gaps are filled.
func normalize(r Response) Response {
	if len(r.Content) == 0 && r.Text != "" {
		r.Content = []ContentBlock{TextBlock(r.Text)}
	}
	if r.Text == "" {
		r.Text = concatText(r.Content)
	}
	if r.StopReason == "" {
		r.StopReason = StopEndTurn
		for _, b := range r.Content {
			if b.Type == BlockToolUse {
				r.StopReason = StopToolUse
				break
			}
		}
	}
	// Default non-zero Usage so the mock mirrors real provider output: real
	// providers always report token counts; a wholly zero Usage is not valid
	// in a successful completion. The default applies only when Usage was not
	// scripted at all — an explicitly scripted partial Usage (e.g. only
	// InputTokens) is honored verbatim, since the test author's intent wins.
	if r.Usage == (Usage{}) {
		r.InputTokens = 1
		r.OutputTokens = 1
	}
	return r
}

// Complete returns the next scripted entry. It records every call, except
// requests rejected by the contract's documented limits (see the type doc),
// which fail with a typed InvalidRequestError before any entry is consumed.
func (m *MockClient) Complete(ctx context.Context, req Request) (Response, error) {
	if err := validateRequest(req); err != nil {
		return Response{}, err
	}

	m.mu.Lock()
	e := m.next(req)
	m.mu.Unlock()

	// Honour context cancellation so callers behave correctly under test.
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	default:
	}

	if e.Err != nil {
		return Response{}, e.Err
	}
	if e.StreamErr != nil {
		// A mid-stream failure means the full completion failed.
		return Response{}, e.StreamErr
	}
	return normalize(e.Response), nil
}

// splitDeltas splits s into two roughly equal halves by rune so the mock
// stream always emits at least two text deltas for multi-rune text.
func splitDeltas(s string) []string {
	if s == "" {
		return nil
	}
	r := []rune(s)
	if len(r) < 2 {
		return []string{s}
	}
	mid := len(r) / 2
	return []string{string(r[:mid]), string(r[mid:])}
}

// Stream returns the next scripted entry as a v2 chunk stream. A scripted Err
// is returned directly as a pre-stream error. Otherwise a producer goroutine
// emits the response's text as two or more TextDelta chunks, non-text blocks
// as complete Block chunks, then the terminal chunk — StopReason+Usage on
// success, Err on a scripted StreamErr — and closes the channel.
//
// The producer selects on ctx.Done() at every send: on cancellation it
// attempts a best-effort terminal chunk carrying ctx.Err(), closes the
// channel, and returns without leaking.
//
// Requests rejected by the contract's documented limits (see the type doc)
// fail PRE-stream with a typed InvalidRequestError — no channel, no entry
// consumed, no call recorded — mirroring the real adapters' pre-HTTP
// validation.
func (m *MockClient) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	m.mu.Lock()
	e := m.next(req)
	m.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if e.Err != nil {
		return nil, e.Err
	}

	resp := normalize(e.Response)
	ch := make(chan StreamChunk, 1)
	go func() {
		defer close(ch)

		// send delivers c, or — if ctx is cancelled first — attempts a
		// best-effort terminal error chunk and reports failure.
		send := func(c StreamChunk) bool {
			select {
			case ch <- c:
				return true
			case <-ctx.Done():
				select {
				case ch <- StreamChunk{Err: ctx.Err()}:
				default:
				}
				return false
			}
		}

		for _, b := range resp.Content {
			if b.Type == BlockText {
				for _, delta := range splitDeltas(b.Text) {
					if !send(StreamChunk{TextDelta: delta}) {
						return
					}
				}
				continue
			}
			block := b
			if !send(StreamChunk{Block: &block}) {
				return
			}
		}

		if e.StreamErr != nil {
			send(StreamChunk{Err: e.StreamErr})
			return
		}
		u := resp.Usage
		send(StreamChunk{StopReason: resp.StopReason, Usage: &u})
	}()
	return ch, nil
}

// MockEntryOK is a convenience constructor for a successful text entry.
func MockEntryOK(text string) MockEntry {
	return MockEntry{Response: Response{Text: text}}
}

// MockEntryErr is a convenience constructor for an error entry.
func MockEntryErr(err error) MockEntry {
	return MockEntry{Err: err}
}

// MockEntryRateLimit is a convenience constructor for a RateLimitError entry.
func MockEntryRateLimit(msg string) MockEntry {
	return MockEntry{Err: &RateLimitError{Message: msg}}
}

// MockEntryInvalid is a convenience constructor for a non-retryable error entry.
func MockEntryInvalid(msg string) MockEntry {
	return MockEntry{Err: &InvalidRequestError{Message: fmt.Sprintf("%s", msg)}}
}

// MockEntryToolUse is a convenience constructor for a tool_use entry: the
// response carries a single tool_use block and StopReason == StopToolUse.
// Chain entries to script multi-turn agentic sequences (tool_use, then a
// final text turn).
func MockEntryToolUse(id, name string, input json.RawMessage) MockEntry {
	return MockEntry{Response: Response{
		Content:    []ContentBlock{ToolUseBlock(id, name, input)},
		StopReason: StopToolUse,
	}}
}

// MockEntryWithResponse is a convenience constructor wrapping a fully
// hand-built Response. Gaps are still normalised (Text from Content and vice
// versa, StopReason defaulted) but explicitly set fields are returned as-is.
func MockEntryWithResponse(resp Response) MockEntry {
	return MockEntry{Response: resp}
}

// MockEntryStreamErr scripts a MID-stream failure: Stream emits partialText as
// deltas, then a terminal chunk with Err = err, then closes. Complete returns
// err directly.
func MockEntryStreamErr(partialText string, err error) MockEntry {
	return MockEntry{Response: Response{Text: partialText}, StreamErr: err}
}

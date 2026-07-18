package llmreplay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// ReplayClient serves a recorded cassette as an llmclient.LLMClient. Entries
// are consumed ORDINALLY: the n-th call gets the n-th entry, the entry's Kind
// must match the called method, and running past the end is a clear error.
// Opt-in MatchByRequestHash additionally verifies each call's request against
// the recorded one. A failed match does NOT consume the entry, so every
// subsequent call reports the same divergence instead of cascading.
//
// Concurrency: the cursor is mutex-guarded (data-race safe), but concurrent
// calls consume entries in scheduling order, which is not the recorded order
// — replay single-goroutine, mirroring how the cassette was recorded.
type ReplayClient struct {
	mu          sync.Mutex
	entries     []Entry
	cursor      int
	matchByHash bool
}

// ReplayOption configures a ReplayClient.
type ReplayOption func(*ReplayClient)

// MatchByRequestHash enables strict matching: each call's request must hash
// (RequestHash) to the same value as the recorded entry's request. Mismatches
// error with the entry index and both hashes. Requests must be deterministic
// for this to work — see RequestHash and the package doc, "Matching".
func MatchByRequestHash() ReplayOption {
	return func(rp *ReplayClient) { rp.matchByHash = true }
}

// NewReplayClient builds a ReplayClient over the given entries (typically
// from Load or LoadFile). The slice is copied; the entries' interior pointers
// and slices are shared.
func NewReplayClient(entries []Entry, opts ...ReplayOption) *ReplayClient {
	rp := &ReplayClient{entries: append([]Entry(nil), entries...)}
	for _, opt := range opts {
		opt(rp)
	}
	return rp
}

// Load reads a JSONL cassette from r and returns its entries in order. Errors
// carry 1-based line numbers (via jsonl-log); a torn trailing line surfaces
// as jsonllog.ErrPartialTrailingLine. Entries with an unknown kind, or
// complete entries with neither a response nor a recorded error, are
// rejected.
func Load(r io.Reader) ([]Entry, error) {
	var entries []Entry
	err := jsonllog.ReadAll(r, func(raw json.RawMessage) error {
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("decode cassette entry: %w", err)
		}
		switch e.Kind {
		case KindComplete:
			if e.Response == nil && e.Err == "" {
				return fmt.Errorf("complete entry has neither response nor err")
			}
		case KindStream:
			// A stream entry may legitimately have zero chunks (a stream
			// cancelled before anything arrived) — no shape check beyond
			// kind; Stream synthesizes the missing terminal chunk at replay
			// time.
		default:
			return fmt.Errorf("unknown entry kind %q", e.Kind)
		}
		normalizeEntry(&e)
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("llmreplay: load cassette: %w", err)
	}
	return entries, nil
}

// normalizeEntry restores nil-ness lost in the JSON round-trip: a nil
// json.RawMessage marshals as null, and unmarshalling null into a RawMessage
// stores the literal bytes "null" rather than nil. Replayed structs must
// DeepEqual what the live client returned, so null raw-JSON payloads collapse
// back to nil. The entry is freshly decoded and owned by Load, so in-place
// mutation is safe.
func normalizeEntry(e *Entry) {
	for i := range e.Request.Messages {
		normalizeBlocks(e.Request.Messages[i].Content)
	}
	for i := range e.Request.Tools {
		e.Request.Tools[i].InputSchema = nilIfNull(e.Request.Tools[i].InputSchema)
	}
	if e.Response != nil {
		normalizeBlocks(e.Response.Content)
	}
	for i := range e.Chunks {
		if b := e.Chunks[i].Block; b != nil {
			normalizeBlock(b)
		}
	}
}

// normalizeBlocks applies normalizeBlock to every block in place.
func normalizeBlocks(blocks []llmclient.ContentBlock) {
	for i := range blocks {
		normalizeBlock(&blocks[i])
	}
}

// normalizeBlock collapses null raw-JSON payloads back to nil.
func normalizeBlock(b *llmclient.ContentBlock) {
	b.Input = nilIfNull(b.Input)
	b.Raw = nilIfNull(b.Raw)
}

// nilIfNull returns nil for the literal JSON null payload, raw otherwise.
func nilIfNull(raw json.RawMessage) json.RawMessage {
	if string(raw) == "null" {
		return nil
	}
	return raw
}

// LoadFile reads the JSONL cassette at path. See Load.
func LoadFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("llmreplay: open cassette: %w", err)
	}
	defer f.Close()
	entries, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

// RequestHash returns the hex SHA-256 of req's v2 canonical JSON.
//
// V2 canonical form: marshal, decode into generic values, DROP zero-value
// fields (empty strings, zero numbers, false bools, nil/empty slices and
// maps), then re-encode with sorted keys. This makes the hash invariant to
// additive struct growth: a new zero-valued field added to llmclient.Request
// or ContentBlock does not change the hash of existing requests that don't
// use the field — the cassette survives schema evolution without re-recording.
//
// Callers using MatchByRequestHash must keep requests deterministic — no
// timestamps, random IDs, or other volatile content. Note also that recorded
// requests are scrubbed before being written, so a live request containing
// redactable content will not hash-match its recorded form — by design.
func RequestHash(req llmclient.Request) (string, error) {
	return requestHashV2(req)
}

// requestHashV2 hashes req using the v2 canonical form (sorted keys + zero
// dropping). Used for all newly recorded cassettes and for the live-request
// side of MatchByRequestHash regardless of cassette version.
func requestHashV2(req llmclient.Request) (string, error) {
	b, err := canonicalJSONv2(req)
	if err != nil {
		return "", fmt.Errorf("llmreplay: hash request v2: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalJSONv2 renders v as v2 canonical JSON: marshal, decode into
// generic values, drop zero-value fields recursively, then re-marshal with
// sorted keys. The drop step makes the encoding invariant to additive schema
// growth — new zero-valued fields do not change the hash.
func canonicalJSONv2(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, err
	}
	generic = dropZeroes(generic)
	return json.Marshal(generic)
}

// dropZeroes recursively removes zero-value entries from the generic JSON
// value tree produced by json.Unmarshal into any. Zero values are:
//   - map entries whose value is itself zero after recursion
//   - empty strings (""), zero numbers (0 as float64), false booleans
//   - nil values
//   - empty slices ([]any{})
//   - empty maps (map[string]any{})
//
// Slices of non-zero elements are preserved with their element order; only
// ALL-zero slices are dropped as a unit. Scalar types not listed above
// (non-empty string, non-zero number, true bool) are returned as-is.
func dropZeroes(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			child = dropZeroes(child)
			if !isZero(child) {
				out[k] = child
			}
		}
		if len(out) == 0 {
			return nil // the whole object was zero-valued
		}
		return out
	case []any:
		out := make([]any, 0, len(val))
		for _, elem := range val {
			out = append(out, dropZeroes(elem))
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return v
	}
}

// isZero reports whether v is a JSON zero value after dropZeroes processing.
func isZero(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case float64:
		return val == 0
	case bool:
		return !val
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	}
	return false
}

// next consumes the cursor entry for a call of the given kind, enforcing kind
// match (and request-hash match when enabled). On any error the entry is NOT
// consumed, so the divergence is reported identically on retry rather than
// shifting the cassette.
func (rp *ReplayClient) next(kind Kind, req llmclient.Request) (Entry, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	idx := rp.cursor
	if idx >= len(rp.entries) {
		return Entry{}, fmt.Errorf("llmreplay: cassette exhausted: %s call would be entry %d but the cassette has %d entries",
			kind, idx, len(rp.entries))
	}
	e := rp.entries[idx]
	if e.Kind != kind {
		return Entry{}, fmt.Errorf("llmreplay: entry %d: recorded kind %q does not match called method (%s)",
			idx, e.Kind, kind)
	}
	if rp.matchByHash {
		// Both the live request and the stored request are always hashed with
		// v2 (drop zero-value fields + sorted keys). This is correct for both
		// v2 cassettes and v1 cassettes:
		//
		//   - V2 cassettes: recorded with v2, so the stored request was already
		//     canonicalised without zero fields at write time. Re-hashing the
		//     stored request with v2 reproduces that hash exactly.
		//
		//   - V1 cassettes (HashVersion == 0): the stored request JSON may
		//     LACK fields that were added to the struct after the cassette was
		//     recorded (e.g. ContentBlock.MediaType / Data added in commit
		//     19a63b4). When that JSON is decoded into the current struct, those
		//     missing fields arrive as Go zero values. Hashing the stored
		//     request with v2 drops those zero fields — exactly what the v1
		//     algorithm would have done implicitly when they did not yet exist.
		//     The live request's same zero fields are also dropped by v2, so
		//     both hashes agree.
		//
		// The HashVersion field on Entry records what was used at write time for
		// auditability; the read path always uses v2 for both sides.
		got, err := requestHashV2(req)
		if err != nil {
			return Entry{}, fmt.Errorf("llmreplay: entry %d: %w", idx, err)
		}
		want, err := requestHashV2(e.Request)
		if err != nil {
			return Entry{}, fmt.Errorf("llmreplay: entry %d: recorded request: %w", idx, err)
		}
		if got != want {
			return Entry{}, fmt.Errorf("llmreplay: entry %d: request hash mismatch: got %s, want %s (the call has diverged from the recording; note recorded requests are scrubbed before hashing)",
				idx, got, want)
		}
	}
	rp.cursor++
	return e, nil
}

// Complete replays the next recorded entry, which must be KindComplete. A
// recorded failure is returned as a *ReplayedError carrying the original
// message.
func (rp *ReplayClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	e, err := rp.next(KindComplete, req)
	if err != nil {
		return llmclient.Response{}, err
	}

	select {
	case <-ctx.Done():
		return llmclient.Response{}, ctx.Err()
	default:
	}

	if e.Err != "" {
		return llmclient.Response{}, &ReplayedError{Message: e.Err}
	}
	return *e.Response, nil
}

// Stream replays the next recorded entry, which must be KindStream. A
// recorded pre-stream failure is returned directly (as *ReplayedError, no
// channel); otherwise a producer goroutine re-emits the recorded chunk
// sequence with its original segmentation — including a recorded terminal
// Err chunk, re-emitted as an equivalent *ReplayedError — then closes the
// channel exactly once. The producer selects on ctx.Done() at every send and
// terminates without leaking; on cancellation a terminal chunk carrying
// ctx.Err() is delivered best-effort.
//
// A stream entry recorded under cancellation may end without a terminal
// chunk: zero chunks, or a last chunk carrying no StopReason/Usage/Err.
// Replaying that verbatim would close the channel with NO terminal chunk
// while the replay ctx is live — violating the llmclient streaming contract —
// so after the recorded chunks a terminal chunk is synthesized whose Err is a
// *ReplayedError explaining that the cassette was recorded under
// cancellation. Re-record the cassette to replay a complete stream.
func (rp *ReplayClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	e, err := rp.next(KindStream, req)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if e.Err != "" {
		return nil, &ReplayedError{Message: e.Err}
	}

	needTerminal := len(e.Chunks) == 0 || !e.Chunks[len(e.Chunks)-1].isTerminal()

	ch := make(chan llmclient.StreamChunk, 1)
	go func() {
		defer close(ch)
		for _, rc := range e.Chunks {
			select {
			case ch <- rc.toStreamChunk():
			case <-ctx.Done():
				select {
				case ch <- llmclient.StreamChunk{Err: ctx.Err()}:
				default:
				}
				return
			}
		}
		if needTerminal {
			terminal := llmclient.StreamChunk{Err: &ReplayedError{
				Message: "llmreplay: recorded stream ends without a terminal chunk (the cassette was recorded under cancellation); re-record the cassette to replay a complete stream",
			}}
			select {
			case ch <- terminal:
			case <-ctx.Done():
				select {
				case ch <- llmclient.StreamChunk{Err: ctx.Err()}:
				default:
				}
			}
		}
	}()
	return ch, nil
}

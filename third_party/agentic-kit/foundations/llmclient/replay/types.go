package llmreplay

import (
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Kind discriminates the two cassette entry variants.
type Kind string

// Entry kinds. The kind must match the method that replays the entry:
// KindComplete entries serve Complete calls, KindStream entries serve Stream
// calls.
const (
	// KindComplete records one Complete call.
	KindComplete Kind = "complete"
	// KindStream records one Stream call, chunk by chunk.
	KindStream Kind = "stream"
)

// HashVersion2 identifies the v2 canonical-hash algorithm used to record a
// cassette entry's request hash: key-sorted PLUS zero-value fields dropped
// (empty strings, zero numbers, nil/empty slices and maps). It is the
// current default for newly recorded cassettes.
//
// Entry.HashVersion records which canonical-hash algorithm was used when an
// entry was recorded, so readers can re-canonicalize the stored request
// before hashing as the request schema evolves:
//
//   - 0 (absent): v1 — key-sorted, no field dropping (the original behaviour).
//   - 2: v2 — HashVersion2, described above.
//
// Note: there is no v1 constant; the zero value IS v1, preserved by JSON
// omitempty so existing cassettes without the field read as v1 naturally.
const HashVersion2 = 2

// Entry is one recorded interaction: a request paired with either a response
// (KindComplete), a chunk sequence (KindStream), or a recorded error. Entries
// hold domain structs only — never HTTP headers, never API keys.
type Entry struct {
	// Kind selects the variant: KindComplete or KindStream.
	Kind Kind `json:"kind"`

	// HashVersion identifies the canonical-hash algorithm used when this entry
	// was recorded. Zero (absent in old cassettes) means v1; 2 means v2.
	// See the HashVersion2 constant and the package doc, "Matching".
	HashVersion int `json:"hash_version,omitempty"`

	// Request is the request as recorded — AFTER scrubbing, when a Scrubber
	// is active.
	Request llmclient.Request `json:"request"`

	// Response is the successful result of a KindComplete entry; nil for
	// stream entries and for recorded failures.
	Response *llmclient.Response `json:"response,omitempty"`

	// Chunks is the recorded chunk sequence of a KindStream entry, preserving
	// the original segmentation (delta boundaries, block positions, terminal
	// chunk). Empty for complete entries and for recorded pre-stream failures.
	Chunks []Chunk `json:"chunks,omitempty"`

	// Err is the recorded error message of a failed Complete call or a
	// PRE-stream Stream failure. Mid-stream terminal failures live in the
	// final Chunk's Err instead. Replayed as *ReplayedError; the original
	// error type is not preserved.
	Err string `json:"err,omitempty"`
}

// Chunk is the recorded form of one llmclient.StreamChunk. Field semantics
// mirror StreamChunk exactly, except Err is a string (errors do not survive
// JSON round-trips as types).
type Chunk struct {
	// TextDelta is an incremental piece of response text.
	TextDelta string `json:"text_delta,omitempty"`

	// Block is a complete non-text content block.
	Block *llmclient.ContentBlock `json:"block,omitempty"`

	// StopReason is set on the terminal success chunk.
	StopReason llmclient.StopReason `json:"stop_reason,omitempty"`

	// Usage is set (non-nil) on the terminal success chunk.
	Usage *llmclient.Usage `json:"usage,omitempty"`

	// Err is the recorded message of a terminal failure chunk. Replayed as
	// *ReplayedError.
	Err string `json:"err,omitempty"`
}

// isTerminal reports whether c is a terminal chunk per the llmclient
// streaming contract: a success terminal (StopReason set, Usage non-nil) or a
// failure terminal (Err set). A recorded stream whose last chunk is not
// terminal was cut short — typically recorded under cancellation — and replay
// synthesizes a terminal error chunk for it (see ReplayClient.Stream).
func (c Chunk) isTerminal() bool {
	return c.StopReason != "" || c.Usage != nil || c.Err != ""
}

// chunkFromStream converts a live StreamChunk into its recorded form. Pointer
// payloads are copied so the recorded entry never aliases live client state.
func chunkFromStream(c llmclient.StreamChunk) Chunk {
	rc := Chunk{TextDelta: c.TextDelta, StopReason: c.StopReason}
	if c.Block != nil {
		b := *c.Block
		rc.Block = &b
	}
	if c.Usage != nil {
		u := *c.Usage
		rc.Usage = &u
	}
	if c.Err != nil {
		rc.Err = errString(c.Err)
	}
	return rc
}

// toStreamChunk converts a recorded chunk back into a live StreamChunk.
// Pointer payloads are copied so concurrent or repeated replays never share
// mutable state through the cassette.
func (c Chunk) toStreamChunk() llmclient.StreamChunk {
	sc := llmclient.StreamChunk{TextDelta: c.TextDelta, StopReason: c.StopReason}
	if c.Block != nil {
		b := *c.Block
		sc.Block = &b
	}
	if c.Usage != nil {
		u := *c.Usage
		sc.Usage = &u
	}
	if c.Err != "" {
		sc.Err = &ReplayedError{Message: c.Err}
	}
	return sc
}

// ReplayedError is the replayed form of a recorded error. Error() returns the
// recorded message verbatim, so message-based assertions written against the
// live client keep passing under replay. The ORIGINAL error type is not
// preserved (cassettes store messages, not Go types): detect a replayed
// failure with errors.As(*ReplayedError), and note that llmclient.WithRetry
// treats ReplayedError as non-retryable regardless of what the original was.
type ReplayedError struct {
	// Message is the recorded error message.
	Message string
}

// Error implements the error interface, returning the recorded message
// verbatim.
func (e *ReplayedError) Error() string { return e.Message }

// errString renders err for recording, substituting a placeholder for the
// pathological empty-message error so a recorded failure is never mistaken
// for "no error".
func errString(err error) string {
	s := err.Error()
	if s == "" {
		s = "llmreplay: recorded error with empty message"
	}
	return s
}

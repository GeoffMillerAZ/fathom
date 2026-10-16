package llmreplay

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// RecordingClient wraps an inner llmclient.LLMClient and appends every
// interaction to a JSONL cassette. Calls are forwarded unchanged — the caller
// sees exactly what the inner client produced (same responses, same chunk
// segmentation, same errors) — while the cassette accumulates Entry lines for
// later replay.
//
// Recording failures are surfaced loudly, never swallowed: a cassette that
// silently dropped an entry would replay a different session than the one
// observed. A failed append fails the Complete call (the response is
// discarded) or replaces the stream's terminal chunk with an Err chunk.
//
// Concurrency: see the package doc — data-race safe (the writer is
// mutex-guarded), but interleaved concurrent recording produces an entry
// order that ordinal replay cannot reproduce. Record single-goroutine, and
// drain each stream fully before the next call.
type RecordingClient struct {
	inner llmclient.LLMClient
	log   *jsonllog.Writer
	scrub Scrubber
}

// RecorderOption configures a RecordingClient.
type RecorderOption func(*RecordingClient)

// WithScrubber replaces DefaultScrubber with s. Passing nil panics: silently
// disabling scrubbing through a nil value is exactly the accident the
// explicit NoScrub option exists to prevent.
func WithScrubber(s Scrubber) RecorderOption {
	if s == nil {
		panic("llmreplay: WithScrubber(nil) — use NoScrub() to disable scrubbing explicitly")
	}
	return func(rc *RecordingClient) { rc.scrub = s }
}

// NoScrub disables scrubbing entirely. DANGEROUS: the cassette will contain
// every prompt, completion, and error verbatim — credentials included. Use
// only when the recorded session is known to contain no sensitive content,
// keep the cassette under workspace/ (gitignored), and review it before it
// goes anywhere else.
func NoScrub() RecorderOption {
	return func(rc *RecordingClient) { rc.scrub = nil }
}

// NewRecordingClient wraps inner so every call is appended to w as one JSONL
// Entry per interaction. DefaultScrubber is applied to each entry before the
// append unless overridden via WithScrubber or disabled via NoScrub.
//
// w receives one Write call per entry (whole lines, mutex-guarded). For a
// durable on-disk cassette pass an *os.File opened for append and sync/close
// it after recording; for in-memory cassettes pass a *bytes.Buffer.
func NewRecordingClient(inner llmclient.LLMClient, w io.Writer, opts ...RecorderOption) *RecordingClient {
	rc := &RecordingClient{
		inner: inner,
		log:   jsonllog.NewWriter(w),
		scrub: DefaultScrubber,
	}
	for _, opt := range opts {
		opt(rc)
	}
	return rc
}

// append scrubs (unless opted out) and persists one entry.
func (rc *RecordingClient) append(e Entry) error {
	if rc.scrub != nil {
		e = rc.scrub(e)
	}
	if err := rc.log.Append(e); err != nil {
		return fmt.Errorf("llmreplay: record %s entry: %w", e.Kind, err)
	}
	return nil
}

// Complete forwards to the inner client and records the outcome: a successful
// response as {kind:"complete", request, response}, a failure as
// {kind:"complete", request, err}. The inner result is returned unchanged —
// unless the append itself fails, in which case Complete fails with the
// recording error (joined with the inner error, if any).
func (rc *RecordingClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	resp, cerr := rc.inner.Complete(ctx, req)

	e := Entry{Kind: KindComplete, HashVersion: HashVersion2, Request: req}
	if cerr != nil {
		e.Err = errString(cerr)
	} else {
		r := resp
		e.Response = &r
	}
	if aerr := rc.append(e); aerr != nil {
		return llmclient.Response{}, errors.Join(aerr, cerr)
	}
	return resp, cerr
}

// Stream forwards to the inner client, relays every chunk to the caller
// unchanged, and records the chunk sequence with its original segmentation as
// {kind:"stream", request, chunks}. A pre-stream failure is recorded as
// {kind:"stream", request, err} and returned. The entry is appended when the
// terminal chunk arrives (or when the inner channel closes), so drain the
// stream fully before making the next recorded call.
//
// If the append fails, the recorded terminal chunk is replaced by a terminal
// Err chunk carrying the recording error — a session whose cassette is broken
// must not look successful.
func (rc *RecordingClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	in, err := rc.inner.Stream(ctx, req)
	if err != nil {
		if aerr := rc.append(Entry{Kind: KindStream, HashVersion: HashVersion2, Request: req, Err: errString(err)}); aerr != nil {
			return nil, errors.Join(aerr, err)
		}
		return nil, err
	}

	out := make(chan llmclient.StreamChunk, 1)
	go func() {
		defer close(out)

		var chunks []Chunk
		recorded := false
		record := func() error {
			if recorded {
				return nil
			}
			recorded = true
			return rc.append(Entry{Kind: KindStream, HashVersion: HashVersion2, Request: req, Chunks: chunks})
		}

		for c := range in {
			chunks = append(chunks, chunkFromStream(c))
			if c.StopReason != "" || c.Err != nil {
				// Terminal chunk: record BEFORE forwarding so an append
				// failure can replace it with a terminal Err chunk.
				if aerr := record(); aerr != nil {
					c = llmclient.StreamChunk{Err: aerr}
				}
			}
			select {
			case out <- c:
			case <-ctx.Done():
				// Caller cancelled and stopped reading. Drain the inner
				// producer (it honours ctx and closes promptly), persist what
				// was seen, and attempt a best-effort terminal chunk.
				for range in {
				}
				_ = record()
				select {
				case out <- llmclient.StreamChunk{Err: ctx.Err()}:
				default:
				}
				return
			}
		}

		// Inner channel closed. Normally the terminal chunk was already
		// recorded above; this persists streams that closed without one
		// (e.g. cancellation where the best-effort terminal was dropped).
		if aerr := record(); aerr != nil {
			select {
			case out <- llmclient.StreamChunk{Err: aerr}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

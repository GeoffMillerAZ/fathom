package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Stream sends a streaming Messages API call. The returned error covers
// pre-stream failures only (request mapping, transport, non-200 status);
// once a channel is returned, all outcomes travel on the channel and the
// producer goroutine closes it exactly once. See the llmclient package doc,
// "Streaming semantics".
func (c *Client) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	body, err := c.buildBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, c.streamClient, body, "text/event-stream")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		mapped := c.mapHTTPError(resp)
		resp.Body.Close()
		return nil, mapped
	}

	ch := make(chan llmclient.StreamChunk)
	go c.consumeSSE(ctx, resp.Body, ch)
	return ch, nil
}

// pendingBlock accumulates one streamed content block between its
// content_block_start and content_block_stop events.
type pendingBlock struct {
	blockType   string
	id          string            // tool_use
	name        string            // tool_use
	input       []byte            // tool_use: concatenated input_json_delta fragments
	raw         json.RawMessage   // unknown block types: the content_block_start JSON
	deltaFields map[string]string // unknown block types: field name → accumulated string value from deltas
}

// consumeSSE is the stream producer: it parses SSE events from body, emits
// StreamChunks, and closes ch exactly once. It selects on ctx.Done() for
// every send and terminates without goroutine leaks; body is always closed.
//
// Mapping: text_delta events become TextDelta chunks; tool_use blocks are
// accumulated from input_json_delta fragments and emitted COMPLETE on
// content_block_stop; unknown block types are emitted as BlockUnknown
// carrying the content_block_start JSON verbatim (their deltas are not
// modeled); message_delta carries stop_reason and usage for the terminal
// chunk; ping events and comment lines are ignored; an error event maps to a
// typed error in a terminal Err chunk.
func (c *Client) consumeSSE(ctx context.Context, body io.ReadCloser, ch chan<- llmclient.StreamChunk) {
	defer close(ch)
	defer body.Close()

	var (
		reader     = newSSEReader(body)
		usage      llmclient.Usage
		stopReason llmclient.StopReason
		pending    = make(map[int]*pendingBlock)
	)

	// fail emits a best-effort terminal Err chunk. Delivery is not
	// guaranteed when the caller is gone (ctx done and nobody reading).
	fail := func(err error) {
		emit(ctx, ch, llmclient.StreamChunk{Err: err})
	}
	malformed := func(what string, err error) {
		fail(&llmclient.ServerError{
			StatusCode: 500,
			Message:    c.redact(fmt.Sprintf("anthropic: malformed %s event: %v", what, err)),
		})
	}

	for {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}
		ev, err := reader.next()
		if err != nil {
			if ctx.Err() != nil {
				fail(ctx.Err())
				return
			}
			if errors.Is(err, errSSETooLarge) {
				// An oversize line/event is a malfunctioning (or malicious,
				// proxied) server, not a caller bug: typed and retryable.
				fail(&llmclient.ServerError{
					Message: c.redact(fmt.Sprintf("anthropic: %v", err)),
				})
				return
			}
			// Severed before message_stop: transient, hence retryable.
			fail(&llmclient.ServerError{
				Message: c.redact(fmt.Sprintf("anthropic: stream ended before message_stop: %v", err)),
			})
			return
		}

		switch ev.name {
		case "ping":
			// Heartbeat: ignore.

		case "message_start":
			var ms struct {
				Message struct {
					Usage wireUsage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(ev.data, &ms); err != nil {
				malformed("message_start", err)
				return
			}
			mergeUsage(&usage, ms.Message.Usage)

		case "content_block_start":
			var cbs struct {
				Index        int             `json:"index"`
				ContentBlock json.RawMessage `json:"content_block"`
			}
			if err := json.Unmarshal(ev.data, &cbs); err != nil {
				malformed("content_block_start", err)
				return
			}
			var head struct {
				Type string `json:"type"`
				Text string `json:"text"`
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(cbs.ContentBlock, &head); err != nil {
				malformed("content_block_start", err)
				return
			}
			switch head.Type {
			case "text":
				pending[cbs.Index] = &pendingBlock{blockType: "text"}
				if head.Text != "" {
					if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: head.Text}) {
						return
					}
				}
			case "tool_use":
				pending[cbs.Index] = &pendingBlock{blockType: "tool_use", id: head.ID, name: head.Name}
			default:
				// Unknown block type: store start JSON and an empty delta map.
				// Deltas are accumulated by field name; reconstruction patches
				// the start JSON in-place preserving key order.
				pending[cbs.Index] = &pendingBlock{
					blockType:   head.Type,
					raw:         cbs.ContentBlock,
					deltaFields: make(map[string]string),
				}
			}

		case "content_block_delta":
			var cbd struct {
				Index int             `json:"index"`
				Delta json.RawMessage `json:"delta"`
			}
			if err := json.Unmarshal(ev.data, &cbd); err != nil {
				malformed("content_block_delta", err)
				return
			}
			var d struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			}
			if err := json.Unmarshal(cbd.Delta, &d); err != nil {
				malformed("content_block_delta", err)
				return
			}
			switch d.Type {
			case "text_delta":
				if d.Text != "" {
					if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: d.Text}) {
						return
					}
				}
			case "input_json_delta":
				if pb := pending[cbd.Index]; pb != nil {
					pb.input = append(pb.input, d.PartialJSON...)
				}
			default:
				// Unmodeled delta type (e.g. thinking_delta, signature_delta):
				// accumulate into the unknown block's delta map so the final Raw
				// matches what Complete would produce for the same block.
				if pb := pending[cbd.Index]; pb != nil && pb.deltaFields != nil {
					applyUnknownDelta(pb.deltaFields, cbd.Delta)
				}
			}

		case "content_block_stop":
			var cbe struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(ev.data, &cbe); err != nil {
				malformed("content_block_stop", err)
				return
			}
			pb := pending[cbe.Index]
			delete(pending, cbe.Index)
			if pb == nil {
				break
			}
			switch pb.blockType {
			case "text":
				// Already emitted incrementally as TextDelta chunks.
			case "tool_use":
				input := pb.input
				if len(input) == 0 {
					input = []byte(`{}`)
				}
				blk := llmclient.ToolUseBlock(pb.id, pb.name, json.RawMessage(input))
				if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
					return
				}
			default:
				// Reconstruct the full Raw by patching start JSON with delta
				// values, preserving the original key order so the result is
				// byte-equivalent to what Complete would have produced.
				raw := reconstructUnknownRaw(pb.deltaFields, pb.raw)
				blk := llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: raw}
				if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
					return
				}
			}

		case "message_delta":
			var md struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage wireUsage `json:"usage"`
			}
			if err := json.Unmarshal(ev.data, &md); err != nil {
				malformed("message_delta", err)
				return
			}
			if md.Delta.StopReason != "" {
				stopReason = llmclient.StopReason(md.Delta.StopReason)
			}
			mergeUsage(&usage, md.Usage)

		case "message_stop":
			if stopReason == "" {
				stopReason = llmclient.StopEndTurn
			}
			u := usage
			emit(ctx, ch, llmclient.StreamChunk{StopReason: stopReason, Usage: &u})
			return

		case "error":
			var ee struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			// Best-effort parse: a garbled error event still terminates the
			// stream with a typed error.
			_ = json.Unmarshal(ev.data, &ee)
			fail(streamErrorToTyped(ee.Error.Type, c.redact(ee.Error.Message)))
			return

		default:
			// Unknown event types: ignored for forward compatibility.
		}
	}
}

// emit delivers chunk unless ctx is done first; it reports whether the chunk
// was delivered. Selecting on ctx.Done() here is what guarantees the
// producer never leaks when the caller cancels and stops reading.
func emit(ctx context.Context, ch chan<- llmclient.StreamChunk, chunk llmclient.StreamChunk) bool {
	select {
	case ch <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// applyUnknownDelta accumulates one delta event into deltaFields for an
// unknown block. The Anthropic streaming protocol names deltas by suffix
// convention: "thinking_delta" carries {"thinking":"..."}, "signature_delta"
// carries {"signature":"..."}. We strip the "_delta" suffix to derive the
// field name, then append the delta's string value to any prior accumulation.
//
// For delta types we don't recognise (no "_delta" suffix, non-string value),
// we leave deltaFields untouched — the start JSON value stands unchanged.
func applyUnknownDelta(deltaFields map[string]string, deltaRaw json.RawMessage) {
	var d map[string]json.RawMessage
	if err := json.Unmarshal(deltaRaw, &d); err != nil {
		return
	}
	var typ string
	if err := json.Unmarshal(d["type"], &typ); err != nil {
		return
	}
	const suffix = "_delta"
	if len(typ) <= len(suffix) || typ[len(typ)-len(suffix):] != suffix {
		return
	}
	fieldName := typ[:len(typ)-len(suffix)]
	deltaVal, ok := d[fieldName]
	if !ok {
		return
	}
	var deltaStr string
	if err := json.Unmarshal(deltaVal, &deltaStr); err != nil {
		return
	}
	deltaFields[fieldName] += deltaStr
}

// reconstructUnknownRaw patches startRaw by substituting each key in
// deltaFields with its accumulated string value, preserving the original
// JSON key order from the start block. This makes the streamed Raw
// byte-equivalent to what Complete would produce for the same block.
//
// The substitution walks the startRaw token stream: for each string-valued
// key whose name appears in deltaFields, the value is replaced with the
// accumulated string. All other tokens are copied verbatim. If any step
// fails, startRaw is returned unchanged as a safe fallback.
func reconstructUnknownRaw(deltaFields map[string]string, startRaw json.RawMessage) json.RawMessage {
	if len(deltaFields) == 0 {
		return startRaw
	}
	dec := json.NewDecoder(bytes.NewReader(startRaw))
	dec.UseNumber()

	// Read the opening '{'.
	tok, err := dec.Token()
	if err != nil {
		return startRaw
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return startRaw
	}

	var out []byte
	out = append(out, '{')
	first := true
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return startRaw
		}
		key, ok := keyTok.(string)
		if !ok {
			return startRaw
		}
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return startRaw
		}
		var valRaw json.RawMessage
		if err := dec.Decode(&valRaw); err != nil {
			return startRaw
		}
		if !first {
			out = append(out, ',')
		}
		first = false
		out = append(out, keyBytes...)
		out = append(out, ':')
		if accumulated, hit := deltaFields[key]; hit {
			b, err := json.Marshal(accumulated)
			if err != nil {
				return startRaw
			}
			out = append(out, b...)
		} else {
			out = append(out, valRaw...)
		}
	}
	if _, err := dec.Token(); err != nil { // consume closing '}'
		return startRaw
	}
	out = append(out, '}')
	return out
}

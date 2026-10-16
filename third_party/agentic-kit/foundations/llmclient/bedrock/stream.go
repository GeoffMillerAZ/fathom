package bedrock

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Stream sends an InvokeModelWithResponseStream call. The returned error covers
// pre-stream failures only (request mapping, signing, transport, non-200
// status); once a channel is returned, all outcomes travel on the channel and
// the producer goroutine closes it exactly once. See the llmclient package doc,
// "Streaming semantics".
func (c *Client) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	body, model, err := c.buildBody(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := c.newSignedRequest(ctx, invokeStreamPath(model), body, "application/vnd.amazon.eventstream")
	if err != nil {
		return nil, err
	}
	resp, err := c.streamClient.Do(httpReq)
	if err != nil {
		return nil, c.mapTransportErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		mapped := c.mapHTTPError(resp)
		resp.Body.Close()
		return nil, mapped
	}

	ch := make(chan llmclient.StreamChunk)
	go c.consume(ctx, resp.Body, ch)
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

// streamState carries the per-stream accumulation across events.
type streamState struct {
	usage      llmclient.Usage
	stopReason llmclient.StopReason
	pending    map[int]*pendingBlock
}

// consume is the stream producer: it decodes AWS event-stream frames, unwraps
// each "chunk" event to its inner Anthropic streaming event JSON, emits
// StreamChunks, and closes ch exactly once. It selects on ctx.Done() for every
// send and terminates without goroutine leaks; body is always closed.
func (c *Client) consume(ctx context.Context, body io.ReadCloser, ch chan<- llmclient.StreamChunk) {
	defer close(ch)
	defer body.Close()

	reader := newEventStreamReader(body)
	st := &streamState{pending: make(map[int]*pendingBlock)}

	fail := func(err error) { emit(ctx, ch, llmclient.StreamChunk{Err: err}) }
	malformed := func(what string, err error) {
		fail(&llmclient.ServerError{
			StatusCode: 500,
			Message:    c.redact(fmt.Sprintf("bedrock: malformed %s event: %v", what, err)),
		})
	}

	for {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}
		frame, err := reader.next()
		if err != nil {
			if ctx.Err() != nil {
				fail(ctx.Err())
				return
			}
			if errors.Is(err, io.EOF) {
				// Clean end of frames without a message_stop: the model turn
				// was cut short — transient, hence retryable.
				fail(&llmclient.ServerError{
					Message: c.redact("bedrock: stream ended before message_stop"),
				})
				return
			}
			if errors.Is(err, errFrameTooLarge) {
				fail(&llmclient.ServerError{Message: c.redact(fmt.Sprintf("bedrock: %v", err))})
				return
			}
			fail(&llmclient.ServerError{
				Message: c.redact(fmt.Sprintf("bedrock: stream ended before message_stop: %v", err)),
			})
			return
		}

		// An exception frame ends the stream with a typed error.
		if frame.messageType == "exception" || frame.exceptionType != "" {
			msg := exceptionMessage(frame.payload)
			fail(streamErrorToTyped(frame.exceptionType, c.redact(msg)))
			return
		}
		if frame.eventType != "chunk" {
			// :event-type other than "chunk" (none defined today) — ignore for
			// forward compatibility.
			continue
		}

		// The chunk payload wraps the inner Anthropic event as base64.
		inner, err := unwrapChunk(frame.payload)
		if err != nil {
			malformed("chunk envelope", err)
			return
		}

		done, ferr := c.handleAnthropicEvent(ctx, ch, st, inner)
		if ferr != nil {
			malformed(ferr.what, ferr.err)
			return
		}
		if done {
			return
		}
	}
}

// chunkEnvelope is the JSON wrapper Bedrock puts around each inner event.
type chunkEnvelope struct {
	Bytes []byte `json:"bytes"` // base64 in the JSON; decoded by encoding/json into []byte
}

// unwrapChunk decodes the {"bytes":"<base64>"} envelope to the inner Anthropic
// event JSON. encoding/json base64-decodes a []byte field, so the result is the
// raw inner bytes.
func unwrapChunk(payload []byte) ([]byte, error) {
	var env chunkEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	if len(env.Bytes) == 0 {
		// Tolerate a manually base64'd field shape just in case; otherwise the
		// inner event is empty and we skip it.
		return nil, nil
	}
	return env.Bytes, nil
}

// base64Std is exposed for the tests' fixture builders (encode an inner event
// the way Bedrock would).
var base64Std = base64.StdEncoding

// failPoint carries which event failed to parse, for malformed().
type failPoint struct {
	what string
	err  error
}

// handleAnthropicEvent dispatches one inner Anthropic streaming event (the same
// event shapes the public SSE API emits) against the accumulating stream state.
// It returns done==true after a message_stop (terminal success chunk already
// emitted), or a non-nil *failPoint on a parse failure. An empty inner event is
// a no-op.
func (c *Client) handleAnthropicEvent(ctx context.Context, ch chan<- llmclient.StreamChunk, st *streamState, inner []byte) (done bool, fp *failPoint) {
	if len(inner) == 0 {
		return false, nil
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(inner, &head); err != nil {
		return false, &failPoint{what: "stream", err: err}
	}

	switch head.Type {
	case "ping":
		// Heartbeat: ignore.

	case "message_start":
		var ms struct {
			Message struct {
				Usage wireUsage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(inner, &ms); err != nil {
			return false, &failPoint{"message_start", err}
		}
		mergeUsage(&st.usage, ms.Message.Usage)

	case "content_block_start":
		var cbs struct {
			Index        int             `json:"index"`
			ContentBlock json.RawMessage `json:"content_block"`
		}
		if err := json.Unmarshal(inner, &cbs); err != nil {
			return false, &failPoint{"content_block_start", err}
		}
		var hb struct {
			Type string `json:"type"`
			Text string `json:"text"`
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(cbs.ContentBlock, &hb); err != nil {
			return false, &failPoint{"content_block_start", err}
		}
		switch hb.Type {
		case "text":
			st.pending[cbs.Index] = &pendingBlock{blockType: "text"}
			if hb.Text != "" {
				if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: hb.Text}) {
					return true, nil
				}
			}
		case "tool_use":
			st.pending[cbs.Index] = &pendingBlock{blockType: "tool_use", id: hb.ID, name: hb.Name}
		default:
			// Unknown block type: store start JSON and an empty delta map.
			// Deltas are accumulated by field name; reconstruction patches
			// the start JSON in-place preserving key order.
			st.pending[cbs.Index] = &pendingBlock{
				blockType:   hb.Type,
				raw:         cbs.ContentBlock,
				deltaFields: make(map[string]string),
			}
		}

	case "content_block_delta":
		var cbd struct {
			Index int             `json:"index"`
			Delta json.RawMessage `json:"delta"`
		}
		if err := json.Unmarshal(inner, &cbd); err != nil {
			return false, &failPoint{"content_block_delta", err}
		}
		var d struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		}
		if err := json.Unmarshal(cbd.Delta, &d); err != nil {
			return false, &failPoint{"content_block_delta", err}
		}
		switch d.Type {
		case "text_delta":
			if d.Text != "" {
				if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: d.Text}) {
					return true, nil
				}
			}
		case "input_json_delta":
			if pb := st.pending[cbd.Index]; pb != nil {
				pb.input = append(pb.input, d.PartialJSON...)
			}
		default:
			// Unmodeled delta type (e.g. thinking_delta, signature_delta):
			// accumulate into the unknown block's delta map so the final Raw
			// matches what Complete would produce for the same block.
			if pb := st.pending[cbd.Index]; pb != nil && pb.deltaFields != nil {
				applyUnknownDelta(pb.deltaFields, cbd.Delta)
			}
		}

	case "content_block_stop":
		var cbe struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal(inner, &cbe); err != nil {
			return false, &failPoint{"content_block_stop", err}
		}
		pb := st.pending[cbe.Index]
		delete(st.pending, cbe.Index)
		if pb == nil {
			return false, nil
		}
		switch pb.blockType {
		case "text":
			// Already emitted incrementally.
		case "tool_use":
			input := pb.input
			if len(input) == 0 {
				input = []byte(`{}`)
			}
			blk := llmclient.ToolUseBlock(pb.id, pb.name, json.RawMessage(input))
			if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
				return true, nil
			}
		default:
			// Reconstruct the full Raw by patching start JSON with delta
			// values, preserving the original key order so the result is
			// byte-equivalent to what Complete would have produced.
			raw := reconstructUnknownRaw(pb.deltaFields, pb.raw)
			blk := llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: raw}
			if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
				return true, nil
			}
		}

	case "message_delta":
		var md struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage wireUsage `json:"usage"`
		}
		if err := json.Unmarshal(inner, &md); err != nil {
			return false, &failPoint{"message_delta", err}
		}
		if md.Delta.StopReason != "" {
			st.stopReason = llmclient.StopReason(md.Delta.StopReason)
		}
		mergeUsage(&st.usage, md.Usage)

	case "message_stop":
		if st.stopReason == "" {
			st.stopReason = llmclient.StopEndTurn
		}
		u := st.usage
		emit(ctx, ch, llmclient.StreamChunk{StopReason: st.stopReason, Usage: &u})
		return true, nil

	case "error":
		var ee struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(inner, &ee)
		emit(ctx, ch, llmclient.StreamChunk{Err: anthropicStreamError(ee.Error.Type, c.redact(ee.Error.Message))})
		return true, nil

	default:
		// Unknown event types: ignored for forward compatibility.
	}
	return false, nil
}

// anthropicStreamError maps an inner Anthropic "error" event (rare on Bedrock,
// which prefers exception frames) to a typed error.
func anthropicStreamError(errType, msg string) error {
	full := msg
	if errType != "" {
		full = errType + ": " + msg
	}
	switch errType {
	case "rate_limit_error":
		return &llmclient.RateLimitError{Message: full}
	case "authentication_error", "permission_error":
		return &llmclient.AuthError{Message: full}
	case "invalid_request_error", "not_found_error", "request_too_large":
		return &llmclient.InvalidRequestError{Message: full}
	default:
		return &llmclient.ServerError{StatusCode: 500, Message: full}
	}
}

// exceptionMessage best-effort extracts a human message from a Bedrock
// exception event payload {"message":"..."}.
func exceptionMessage(payload []byte) string {
	if m := parseAPIErrorMessage(payload); m != "" {
		return m
	}
	return string(payload)
}

// emit delivers chunk unless ctx is done first; it reports whether the chunk
// was delivered. Selecting on ctx.Done() here is what guarantees the producer
// never leaks when the caller cancels and stops reading.
func emit(ctx context.Context, ch chan<- llmclient.StreamChunk, chunk llmclient.StreamChunk) bool {
	select {
	case ch <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// applyUnknownDelta accumulates one delta event into deltaFields for an
// unknown block. See the identical function in the anthropic package for the
// full contract; bedrock uses the same Anthropic-shaped inner events.
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
// JSON key order from the start block. See the identical function in the
// anthropic package for the full contract.
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

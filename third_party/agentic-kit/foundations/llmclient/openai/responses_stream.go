package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func (c *Client) streamResponses(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	body, err := c.buildResponsesBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, c.streamClient, body, "text/event-stream")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err = c.mapHTTPError(resp)
		resp.Body.Close()
		return nil, err
	}
	ch := make(chan llmclient.StreamChunk)
	go c.consumeResponses(ctx, resp.Body, ch)
	return ch, nil
}

// consumeResponses emits text for the current output item and buffers later
// items until their predecessors finish. Function argument deltas and completed
// function items are never executable blocks until the terminal response has
// matched their canonical representation. Total wire bytes, event bytes
// and retained state are bounded by maxBodyBytes; EOF cannot masquerade as success.
func (c *Client) consumeResponses(ctx context.Context, body io.ReadCloser, ch chan<- llmclient.StreamChunk) {
	defer close(ch)
	defer body.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = body.Close() })
	defer stopClose()
	reader := newSSEReader(io.LimitReader(body, maxBodyBytes+1))
	total := 0
	next := 0
	var partial strings.Builder
	done := map[int]json.RawMessage{}
	var emitted []llmclient.ContentBlock
	fail := func(err error) { emit(ctx, ch, llmclient.StreamChunk{Err: err}) }
	flush := func(raw json.RawMessage) error {
		blocks, _, err := c.parseResponseItem(raw)
		if err != nil {
			return err
		}
		text := textOf(blocks)
		if !strings.HasPrefix(text, partial.String()) {
			return c.responsesError("text delta disagrees with completed item")
		}
		if suffix := strings.TrimPrefix(text, partial.String()); suffix != "" {
			if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: suffix}) {
				return ctx.Err()
			}
		}
		emitted = append(emitted, blocks...)
		partial.Reset()
		return nil
	}
	for {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}
		data, err := nextResponsesEvent(reader, &total)
		if err != nil {
			if ctx.Err() != nil {
				fail(ctx.Err())
			} else {
				fail(c.responsesError("stream ended before terminal response: " + err.Error()))
			}
			return
		}
		var event struct {
			Type     string          `json:"type"`
			Index    int             `json:"output_index"`
			Delta    string          `json:"delta"`
			Item     json.RawMessage `json:"item"`
			Response json.RawMessage `json:"response"`
			Code     string          `json:"code"`
			Message  string          `json:"message"`
		}
		if json.Unmarshal(data, &event) != nil || event.Type == "" || event.Index < 0 {
			fail(c.responsesError("malformed stream event"))
			return
		}
		switch event.Type {
		case "response.output_text.delta", "response.refusal.delta":
			if event.Index < next {
				fail(c.responsesError("delta after completed item"))
				return
			}
			if event.Index == next {
				partial.WriteString(event.Delta)
				if event.Delta != "" && !emit(ctx, ch, llmclient.StreamChunk{TextDelta: event.Delta}) {
					return
				}
			}
		case "response.output_item.done":
			if _, _, err := c.parseResponseItem(event.Item); err != nil {
				fail(err)
				return
			}
			if event.Index < next || done[event.Index] != nil {
				fail(c.responsesError("duplicate completed item"))
				return
			}
			done[event.Index] = event.Item
			for done[next] != nil {
				if err := flush(done[next]); err != nil {
					fail(err)
					return
				}
				delete(done, next)
				next++
			}
		case "response.completed", "response.incomplete":
			resp, err := c.parseResponse(event.Response)
			if err != nil {
				fail(err)
				return
			}
			var wire responsesWire
			_ = json.Unmarshal(event.Response, &wire)
			if (event.Type == "response.completed" && wire.Status != "completed") || (event.Type == "response.incomplete" && wire.Status != "incomplete") {
				fail(c.responsesError("terminal status mismatch"))
				return
			}
			if next > len(wire.Output) || len(emitted) > len(resp.Content) || !reflect.DeepEqual(emitted, resp.Content[:len(emitted)]) && len(emitted) > 0 {
				fail(c.responsesError("terminal output disagrees with completed items"))
				return
			}
			for index, item := range done {
				if index >= len(wire.Output) {
					fail(c.responsesError("unmatched output index"))
					return
				}
				blocks, _, err := c.parseResponseItem(item)
				final, _, finalErr := c.parseResponseItem(wire.Output[index])
				if err != nil || finalErr != nil || !reflect.DeepEqual(blocks, final) {
					fail(c.responsesError("pending item disagrees with terminal response"))
					return
				}
			}
			for i := next; i < len(wire.Output); i++ {
				if err := flush(wire.Output[i]); err != nil {
					fail(err)
					return
				}
			}
			if partial.Len() != 0 {
				fail(c.responsesError("unmatched output items"))
				return
			}
			// The terminal response is the only authority for executable blocks.
			// Emitting them after all reconciliation prevents a later mismatch from
			// becoming an already-dispatched tool call.
			for _, block := range resp.Content {
				if block.Type == llmclient.BlockText {
					continue
				}
				b := block
				if !emit(ctx, ch, llmclient.StreamChunk{Block: &b}) {
					return
				}
			}
			emit(ctx, ch, llmclient.StreamChunk{StopReason: resp.StopReason, Usage: &resp.Usage})
			return
		case "response.failed":
			_, err := c.parseResponse(event.Response)
			if err == nil {
				err = c.responsesError("response.failed")
			}
			fail(err)
			return
		case "error":
			fail(c.responsesError(event.Code + ": " + event.Message))
			return
		default:
			// Lifecycle, reasoning and argument deltas are bounded and decoded,
			// but their completed items are the lossless content representation.
		}
	}
}

// nextResponsesEvent follows SSE framing, including optional spaces, multiline
// data, comments and CRLF. The total includes ignored fields and keepalives.
func nextResponsesEvent(reader *sseReader, total *int) ([]byte, error) {
	var data []byte
	for {
		line, err := reader.readLine()
		if err != nil {
			return nil, err
		}
		*total += len(line) + 1
		if *total > maxBodyBytes {
			return nil, errSSETooLarge
		}
		if len(line) == 0 {
			if len(data) > 0 {
				return bytes.TrimSuffix(data, []byte{'\n'}), nil
			}
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			value := bytes.TrimPrefix(line, []byte("data:"))
			value = bytes.TrimPrefix(value, []byte(" "))
			data = append(data, value...)
			data = append(data, '\n')
		}
	}
}

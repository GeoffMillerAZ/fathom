package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// errSSETooLarge reports that the server sent a line or event exceeding the
// size cap. Mapped to a typed retryable ServerError.
var errSSETooLarge = fmt.Errorf("SSE line or event data exceeds the %d-byte cap", maxBodyBytes)

// Stream sends a streaming Chat Completions call. The returned error covers
// pre-stream failures only; once a channel is returned, all outcomes travel on
// the channel and the producer goroutine closes it exactly once.
func (c *Client) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	if c.useResponses {
		return c.streamResponses(ctx, req)
	}
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

// pendingToolCall accumulates one streamed tool_call by index.
type pendingToolCall struct {
	id        string
	name      string
	arguments []byte // concatenated partial argument fragments
}

// consumeSSE is the stream producer: it parses OpenAI SSE events from body,
// emits StreamChunks on ch, and closes ch exactly once. It selects on
// ctx.Done() for every send and terminates without goroutine leaks; body is
// always closed.
//
// OpenAI SSE delta events carry:
//
//	data: {"id":"...","choices":[{"delta":{"content":"...",...},"finish_reason":null}],...}
//
// The final data chunk before "data: [DONE]" carries usage when
// stream_options.include_usage=true was set.
func (c *Client) consumeSSE(ctx context.Context, body io.ReadCloser, ch chan<- llmclient.StreamChunk) {
	defer close(ch)
	defer body.Close()

	var (
		reader = newSSEReader(body)
		usage  llmclient.Usage
		stop   llmclient.StopReason
		// pendingTools maps tool_call index → accumulated state.
		pendingTools = make(map[int]*pendingToolCall)
	)

	fail := func(err error) {
		emit(ctx, ch, llmclient.StreamChunk{Err: err})
	}

	for {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}

		line, err := reader.nextDataLine()
		if err != nil {
			if ctx.Err() != nil {
				fail(ctx.Err())
				return
			}
			if errors.Is(err, errSSETooLarge) {
				fail(&llmclient.ServerError{
					Message: c.redact(fmt.Sprintf("openai: %v", err)),
				})
				return
			}
			if errors.Is(err, io.EOF) {
				// Stream ended before we received a finish_reason terminal.
				// Emit what we have accumulated.
				if stop == "" {
					stop = llmclient.StopEndTurn
				}
				u := usage
				emit(ctx, ch, llmclient.StreamChunk{StopReason: stop, Usage: &u})
				return
			}
			fail(&llmclient.ServerError{
				Message: c.redact(fmt.Sprintf("openai: stream ended unexpectedly: %v", err)),
			})
			return
		}

		// "[DONE]" signals the end of the stream.
		if string(line) == "[DONE]" {
			// Flush any accumulated tool_calls as complete BlockToolUse blocks.
			// OpenAI streams tool_calls delta-by-delta and only marks finish_reason
			// after the delta stream ends, so tool blocks are emitted here.
			for idx := 0; idx < len(pendingTools)+10; idx++ {
				tc, ok := pendingTools[idx]
				if !ok {
					continue
				}
				args := tc.arguments
				if len(args) == 0 {
					args = []byte(`{}`)
				}
				blk := llmclient.ToolUseBlock(tc.id, tc.name, json.RawMessage(args))
				if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
					return
				}
				delete(pendingTools, idx)
			}
			if stop == "" {
				stop = llmclient.StopEndTurn
			}
			u := usage
			emit(ctx, ch, llmclient.StreamChunk{StopReason: stop, Usage: &u})
			return
		}

		// Parse the SSE data payload.
		var event struct {
			Choices []struct {
				Delta struct {
					Content   *string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(line, &event); err != nil {
			// Malformed event; treat as a transient server error.
			fail(&llmclient.ServerError{
				Message: c.redact(fmt.Sprintf("openai: malformed SSE event: %v", err)),
			})
			return
		}

		// Capture usage from the final usage chunk (include_usage:true).
		if event.Usage != nil {
			usage.InputTokens = event.Usage.PromptTokens
			usage.OutputTokens = event.Usage.CompletionTokens
		}

		if len(event.Choices) == 0 {
			continue
		}

		choice := event.Choices[0]

		// Accumulate tool_call deltas by index.
		for _, tc := range choice.Delta.ToolCalls {
			pb, ok := pendingTools[tc.Index]
			if !ok {
				pb = &pendingToolCall{}
				pendingTools[tc.Index] = pb
			}
			if tc.ID != "" {
				pb.id = tc.ID
			}
			if tc.Function.Name != "" {
				pb.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				pb.arguments = append(pb.arguments, tc.Function.Arguments...)
			}
		}

		// Emit text deltas.
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			if !emit(ctx, ch, llmclient.StreamChunk{TextDelta: *choice.Delta.Content}) {
				return
			}
		}

		// Capture finish_reason when it arrives.
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			stop = mapFinishReason(*choice.FinishReason)

			// When finish_reason is tool_calls, emit the accumulated tool blocks.
			if stop == llmclient.StopToolUse {
				// Emit in index order.
				maxIdx := -1
				for idx := range pendingTools {
					if idx > maxIdx {
						maxIdx = idx
					}
				}
				for idx := 0; idx <= maxIdx; idx++ {
					tc, ok := pendingTools[idx]
					if !ok {
						continue
					}
					args := tc.arguments
					if len(args) == 0 {
						args = []byte(`{}`)
					}
					blk := llmclient.ToolUseBlock(tc.id, tc.name, json.RawMessage(args))
					if !emit(ctx, ch, llmclient.StreamChunk{Block: &blk}) {
						return
					}
					delete(pendingTools, idx)
				}
			}
		}
	}
}

// emit delivers chunk, selecting on ctx.Done() so the producer never leaks.
func emit(ctx context.Context, ch chan<- llmclient.StreamChunk, chunk llmclient.StreamChunk) bool {
	select {
	case ch <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// sseReader is a line-oriented reader for OpenAI SSE. OpenAI emits one
// "data: <json>" line per event (unlike Anthropic which uses named events).
type sseReader struct {
	r *bufio.Reader
}

func newSSEReader(r io.Reader) *sseReader {
	return &sseReader{r: bufio.NewReader(r)}
}

var dataPrefix = []byte("data: ")

// nextDataLine returns the JSON payload of the next "data: ..." SSE line.
// Comment lines (':') and non-data lines are skipped. Blank lines are skipped.
// Returns io.EOF when the stream ends cleanly.
func (s *sseReader) nextDataLine() ([]byte, error) {
	for {
		line, err := s.readLine()
		if err != nil {
			return nil, err
		}
		if len(line) == 0 {
			continue // blank line: SSE event boundary, skip
		}
		if line[0] == ':' {
			continue // comment
		}
		if bytes.HasPrefix(line, dataPrefix) {
			return line[len(dataPrefix):], nil
		}
		// Non-data fields (event:, id:, retry:): skip.
	}
}

// readLine reads one line, tolerating both \n and \r\n terminators. Lines
// exceeding maxBodyBytes fail with errSSETooLarge. The returned slice is
// freshly allocated and safe to retain.
func (s *sseReader) readLine() ([]byte, error) {
	var line []byte
	for {
		frag, err := s.r.ReadSlice('\n')
		if len(line)+len(frag) > maxBodyBytes {
			return nil, errSSETooLarge
		}
		line = append(line, frag...)
		if err == nil {
			break
		}
		if err != bufio.ErrBufferFull {
			if len(line) > 0 {
				// Partial line at EOF: strip newline and return it so the
				// caller can process the final "[DONE]" if it came without \n.
				line = stripNewline(line)
				return line, nil
			}
			return nil, err
		}
	}
	line = stripNewline(line)
	return line, nil
}

func stripNewline(line []byte) []byte {
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
	}
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line
}

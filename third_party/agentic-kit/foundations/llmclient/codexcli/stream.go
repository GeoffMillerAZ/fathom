package codexcli

import (
	"context"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Stream satisfies llmclient.LLMClient's Stream method. There is no
// sub-turn incremental streaming for this adapter: `codex exec --json`
// emits per-ITEM events (thread.started, turn.started, item.completed,
// turn.completed/turn.failed), not per-token deltas within an
// agent_message, and this adapter reduces the whole event stream to one
// Response before returning (see reduceEvents) — mirroring claudecli's
// precedent exactly. Stream runs Complete to completion and delivers the
// result as a single TextDelta chunk followed by the terminal chunk
// (StopReason + Usage). Callers that need real incremental delivery should
// use Complete and treat this Stream as a compatibility shim for code
// written against the Stream half of the interface. True incremental
// streaming from the JSONL event feed is deferred — see the package doc,
// "Deferred".
//
// The returned error is always nil; a Complete failure arrives as the
// terminal chunk's Err, per the llmclient streaming contract.
func (c *Client) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk, 2)
	go func() {
		defer close(ch)

		resp, err := c.Complete(ctx, req)
		if err != nil {
			send(ctx, ch, llmclient.StreamChunk{Err: err})
			return
		}

		if resp.Text != "" {
			if !send(ctx, ch, llmclient.StreamChunk{TextDelta: resp.Text}) {
				return
			}
		}
		u := resp.Usage
		send(ctx, ch, llmclient.StreamChunk{StopReason: resp.StopReason, Usage: &u})
	}()
	return ch, nil
}

// send delivers c on ch, or — if ctx is cancelled first — makes a
// best-effort terminal error chunk and reports failure so the caller stops
// sending. Mirrors claudecli's (and the noop/MockClient) shared
// send-with-cancellation shape.
func send(ctx context.Context, ch chan<- llmclient.StreamChunk, c llmclient.StreamChunk) bool {
	select {
	case ch <- c:
		return true
	case <-ctx.Done():
		select {
		case ch <- llmclient.StreamChunk{Err: ctx.Err()}:
		default:
		}
		return false
	}
}

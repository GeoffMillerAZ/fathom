package llminstrument

import (
	"context"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// watchStream wraps inner: every chunk is forwarded unchanged on the returned
// channel and, after inner closes, onDone is invoked exactly once with the
// terminal information observed — stop/usage from the terminal success chunk,
// streamErr from a terminal error chunk, zero values when inner closed
// without any terminal chunk. The returned channel closes only after onDone
// returns, so a caller that drained the wrapped stream to completion is
// guaranteed the terminal bookkeeping has already happened.
//
// Abandonment: forwarding never traps the goroutine. When ctx is cancelled
// (the contract-conformant way to abandon a stream), forwarding stops and the
// goroutine keeps draining inner until the producer closes it — which the
// llmclient streaming contract guarantees happens promptly on cancellation —
// then calls onDone and terminates. A caller that stops receiving WITHOUT
// cancelling ctx violates the llmclient contract ("callers MUST drain the
// channel until it closes") and blocks the wrapper exactly as it would block
// the un-wrapped producer.
func watchStream(ctx context.Context, inner <-chan llmclient.StreamChunk, onDone func(stop llmclient.StopReason, usage *llmclient.Usage, streamErr error)) <-chan llmclient.StreamChunk {
	out := make(chan llmclient.StreamChunk)
	go func() {
		defer close(out)
		var (
			stop       llmclient.StopReason
			usage      *llmclient.Usage
			streamErr  error
			forwarding = true
		)
		for chunk := range inner {
			if chunk.Err != nil {
				streamErr = chunk.Err
			}
			if chunk.StopReason != "" {
				stop = chunk.StopReason
				usage = chunk.Usage
			}
			if forwarding {
				select {
				case out <- chunk:
				case <-ctx.Done():
					forwarding = false
				}
			}
		}
		onDone(stop, usage, streamErr)
	}()
	return out
}

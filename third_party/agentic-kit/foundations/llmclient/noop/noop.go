// Package llmnoop provides a no-op implementation of llmclient.LLMClient that
// never reaches a model: every Complete and Stream call returns one canned
// refusal block, "LLM unavailable (no model configured)", with StopReason
// llmclient.StopRefusal.
//
// # Why
//
// It is the graceful air-gap fallback. A deterministic core — a Bayesian
// optimisation loop, a planner, any pipeline whose correctness does not depend
// on the model — must keep running when no real provider is reachable
// (disconnected edge deploy, missing credentials, a closed-network exercise).
// Wiring a Client here lets that core degrade gracefully: it still produces its
// deterministic output, just without the LLM-generated narrative around it,
// instead of crashing on a nil client or hanging on a dead endpoint.
//
// Client returns SUCCESS, not error, by design. The refusal is modelled as a
// normal completion whose StopReason is StopRefusal — exactly the shape a real
// provider uses when it declines — so callers that already branch on
// StopReason (per the open-set rule in the llmclient package doc) handle it
// with no special-casing, and llmclient.WithRetry never retries it (a nil error
// is always success). Code that needs a non-narrative path can detect it via
// StopReason == StopRefusal or by comparing Response.Text to Message.
//
// Composition: imports only the parent foundations/llmclient (the domain types
// and the LLMClient interface) and the standard library. No network, no
// credentials, no provider SDK — that is the entire point.
//
// Concurrency: Client is immutable and stateless; it is safe for concurrent use
// and every channel it returns is closed exactly once by its single producer
// goroutine, honouring ctx cancellation without leaking, per the llmclient
// streaming contract.
package llmnoop

import (
	"context"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Message is the single canned text block every call yields. It is the literal
// Response.Text and the Text of the one BlockText content block.
const Message = "LLM unavailable (no model configured)"

// Client is a no-op llmclient.LLMClient: it never contacts a model and always
// returns the canned refusal (see the package doc). The zero value is ready to
// use; New is provided for symmetry with the other clients in the kit.
//
// Client carries no configuration and no state, so all instances are
// interchangeable.
type Client struct{}

// compile-time assertion that Client satisfies the interface.
var _ llmclient.LLMClient = Client{}

// New returns a no-op LLMClient. Use it as the air-gap fallback when no real
// model is reachable, so a deterministic core keeps running without narrative
// (see the package doc for the degrade-gracefully intent).
func New() Client {
	return Client{}
}

// response builds the canned refusal Response. A fresh value is returned on
// every call so callers may mutate the result (e.g. append blocks) without
// affecting later calls — no shared backing array is handed out.
func response() llmclient.Response {
	return llmclient.Response{
		Text:       Message,
		Content:    []llmclient.ContentBlock{llmclient.TextBlock(Message)},
		StopReason: llmclient.StopRefusal,
	}
}

// Complete returns the canned refusal Response with StopReason StopRefusal and
// a nil error: a no-op completion is a successful, never-retried outcome, not a
// failure. The request is ignored entirely. ctx cancellation is honoured: a
// call on an already-cancelled context returns ctx.Err().
func (Client) Complete(ctx context.Context, _ llmclient.Request) (llmclient.Response, error) {
	if err := ctx.Err(); err != nil {
		return llmclient.Response{}, err
	}
	return response(), nil
}

// Stream returns a channel that yields the canned refusal as one text block,
// then a terminal success chunk carrying StopReason StopRefusal and a non-nil
// (zero) Usage, then closes — exactly once. The returned error is always nil
// (there is no pre-stream failure mode); all outcomes travel on the channel,
// per the llmclient streaming contract. The producer selects on ctx.Done() at
// every send and, on cancellation, makes a best-effort terminal chunk carrying
// ctx.Err() before closing, so it never leaks.
func (Client) Stream(ctx context.Context, _ llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk, 1)
	go func() {
		defer close(ch)

		// send delivers c, or — if ctx is cancelled first — attempts a
		// best-effort terminal error chunk and reports failure. This mirrors
		// MockClient.Stream so the two share streaming semantics.
		send := func(c llmclient.StreamChunk) bool {
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

		if !send(llmclient.StreamChunk{TextDelta: Message}) {
			return
		}
		var u llmclient.Usage
		send(llmclient.StreamChunk{StopReason: llmclient.StopRefusal, Usage: &u})
	}()
	return ch, nil
}

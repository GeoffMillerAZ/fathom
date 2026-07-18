package llminstrument

import (
	"context"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// costClient wraps an inner LLMClient and records usage and USD cost into a
// Tracker. See WithCost.
type costClient struct {
	inner   llmclient.LLMClient
	tracker *costtracker.Tracker
	table   costtracker.PriceTable
	label   string
}

var _ llmclient.LLMClient = (*costClient)(nil)

// WithCost wraps inner so that every successful Complete — and every stream
// that ends with a terminal success chunk — records its token usage and USD
// cost into t under label. t must be non-nil. The model used for pricing is
// Request.Model, also recorded as the Tracker's model key.
//
// Unknown models (absent from table, including the empty model name when the
// request relies on the client's configured default) record their tokens with
// ZERO cost — usage visibility is preserved, spend is understated. Watch for
// unpriced models via Tracker.Report's by-model section and keep the table
// covering every model you route to; pass explicit Request.Model values (or
// compose below a Ladder rung applier) so pricing sees the real model name.
//
// Failed calls record nothing: an error response carries no usage, so there
// is no ledger entry to write (call counts therefore mean BILLED calls, not
// attempted calls). Cost recording never alters the call path — responses,
// chunks, and errors pass through unchanged.
//
// See the package doc for the recommended composition order
// (WithCost(WithTelemetry(WithRetry(client)))) and its rationale.
func WithCost(inner llmclient.LLMClient, t *costtracker.Tracker, table costtracker.PriceTable, label string) llmclient.LLMClient {
	return &costClient{inner: inner, tracker: t, table: table, label: label}
}

// Complete delegates to the inner client and records usage and cost on
// success.
func (c *costClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err == nil {
		c.record(req.Model, resp.Usage)
	}
	return resp, err
}

// Stream delegates to the inner client and wraps the returned channel:
// chunks are forwarded unchanged, and when the stream ends with a terminal
// success chunk (non-nil Usage, no Err) the usage and cost are recorded.
// Pre-stream failures, mid-stream errors, and streams cancelled before the
// terminal chunk record nothing.
func (c *costClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	inner, err := c.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	return watchStream(ctx, inner, func(stop llmclient.StopReason, usage *llmclient.Usage, streamErr error) {
		if streamErr == nil && usage != nil {
			c.record(req.Model, *usage)
		}
	}), nil
}

// record prices the usage (zero cost for unknown models) and writes one
// Tracker entry.
func (c *costClient) record(model string, u llmclient.Usage) {
	cost, _ := c.table.Cost(model, u) // unknown model: cost = 0, tokens still recorded
	c.tracker.Record(c.label, model, u, cost)
}

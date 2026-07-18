package main

import (
	"context"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/metrics"
)

// Error counter family names exposed on GET /metrics alongside the kit's
// endpoint_guard_requests_total outcome counters.
const (
	// metricLLMErrors counts failed LLM calls, labeled role=answerer|judge.
	metricLLMErrors = "endpoint_guard_llm_errors_total"

	// metricEmbeddingErrors counts failed embedding calls.
	metricEmbeddingErrors = "endpoint_guard_embedding_errors_total"
)

// countingLLM wraps an llmclient.LLMClient and increments errs on every
// failed call. Responses and errors pass through unchanged. Safe for
// concurrent use (the inner client and Counter both are).
type countingLLM struct {
	inner llmclient.LLMClient
	errs  *metrics.Counter
}

var _ llmclient.LLMClient = (*countingLLM)(nil)

func (c *countingLLM) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err != nil {
		c.errs.Inc()
	}
	return resp, err
}

// Stream counts pre-stream failures directly and forwards chunks, counting a
// terminal Err chunk as one error. Callers keep the usual contract: drain the
// returned channel until it closes.
func (c *countingLLM) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch, err := c.inner.Stream(ctx, req)
	if err != nil {
		c.errs.Inc()
		return nil, err
	}
	out := make(chan llmclient.StreamChunk)
	go func() {
		defer close(out)
		for chunk := range ch {
			if chunk.Err != nil {
				c.errs.Inc()
			}
			out <- chunk
		}
	}()
	return out, nil
}

// countingEmbedder wraps an embeddingclient.Embedder and increments errs on
// every failed call. Safe for concurrent use.
type countingEmbedder struct {
	inner embeddingclient.Embedder
	errs  *metrics.Counter
}

var _ embeddingclient.Embedder = (*countingEmbedder)(nil)

func (c *countingEmbedder) Embed(ctx context.Context, text string) (embeddingclient.Vector, error) {
	v, err := c.inner.Embed(ctx, text)
	if err != nil {
		c.errs.Inc()
	}
	return v, err
}

func (c *countingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]embeddingclient.Vector, error) {
	vs, err := c.inner.EmbedBatch(ctx, texts)
	if err != nil {
		c.errs.Inc()
	}
	return vs, err
}

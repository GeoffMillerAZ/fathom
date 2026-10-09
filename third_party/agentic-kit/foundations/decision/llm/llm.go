package llm

import (
	"context"
	"fmt"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// DefaultMaxTokens is the MaxTokens LLM sends when WithMaxTokens is not
// used.
const DefaultMaxTokens = 2048

// LLM is a decision.Decider backed by one llmclient.LLMClient completion
// per Decide call: every Question in the Request is rendered into one prompt
// and one OutputSchema, so a batch of questions costs one round trip.
//
// Concurrency: LLM holds no mutable state after construction — client,
// model, MaxTokens and the clock are fixed at NewLLM time — so it is safe
// for concurrent use exactly when the wrapped llmclient.LLMClient is.
type LLM struct {
	client    llmclient.LLMClient
	model     string
	maxTokens int
	now       func() time.Time
}

// LLMOption configures an LLM at construction. See WithMaxTokens and
// WithNow.
type LLMOption func(*LLM)

// WithMaxTokens overrides the MaxTokens sent on every Decide call. The
// default is DefaultMaxTokens.
func WithMaxTokens(n int) LLMOption {
	return func(l *LLM) { l.maxTokens = n }
}

// WithNow overrides LLM's clock, used to compute Response.Latency. The
// default is time.Now. Tests use this to pin Latency deterministically.
func WithNow(now func() time.Time) LLMOption {
	return func(l *LLM) { l.now = now }
}

// NewLLM constructs an LLM Decider over client, using model for every
// Decide call's Request.Model. opts apply in order after the defaults
// (DefaultMaxTokens, time.Now).
func NewLLM(client llmclient.LLMClient, model string, opts ...LLMOption) *LLM {
	l := &LLM{
		client:    client,
		model:     model,
		maxTokens: DefaultMaxTokens,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Name returns "llm".
func (l *LLM) Name() string { return "llm" }

// Decide sends req.State and req.Questions to the wrapped LLMClient as ONE
// completion (see the package doc for the request shape: a System contract
// prompt, a deterministic JSON Prompt preserving Question order, and an
// OutputSchema built from req.Questions) and parses the reply.
//
// Per question, the reply is validated against the Question and classified:
// a valid answer lands in Response.Answers; a choice not among the
// Question's Options, an out-of-range or missing value, or a malformed
// per-question shape lands in Response.Invalid instead, naming the reason —
// Decide does not fail the whole batch for one bad question. Confidence is
// always computed by decision.Confidence (see the decision package doc),
// never taken from the model.
//
// LLM declares no Capabilities.Images: a Request carrying Images is
// answered with every question in Response.Invalid (the reason is
// decision.UnsupportedImages') and no completion, never without the image.
//
// Decide returns an error only when req fails Validate, the underlying
// Complete call errors, or the reply is not JSON at all.
func (l *LLM) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if err := decision.Validate(req); err != nil {
		return decision.Response{}, err
	}
	if reason, unsupported := decision.UnsupportedImages(l.Name(), l.Capabilities(), req); unsupported {
		invalid := make(map[string]string, len(req.Questions))
		for _, q := range req.Questions {
			invalid[q.ID] = reason
		}
		return decision.Response{Provider: l.Name(), Model: l.model, Answers: map[string]decision.Answer{}, Invalid: invalid}, nil
	}

	start := l.now()

	prompt, err := renderPrompt(req)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: rendering prompt: %w", err)
	}

	creq := llmclient.Request{
		Model:        l.model,
		System:       llmSystemPrompt,
		Prompt:       string(prompt),
		OutputSchema: buildOutputSchema(req.Questions),
		MaxTokens:    l.maxTokens,
	}

	cresp, err := l.client.Complete(ctx, creq)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: llm completion: %w", err)
	}

	answers, invalid, err := parseResponse(req, cresp.Text)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: %w", err)
	}

	return decision.Response{
		Provider: l.Name(),
		Model:    l.model,
		Answers:  answers,
		Invalid:  invalid,
		Usage: decision.Usage{
			InputTokens:  cresp.Usage.InputTokens,
			OutputTokens: cresp.Usage.OutputTokens,
		},
		Latency: l.now().Sub(start),
	}, nil
}

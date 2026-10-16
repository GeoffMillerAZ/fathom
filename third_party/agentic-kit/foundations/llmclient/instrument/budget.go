package llminstrument

import (
	"context"
	"errors"
	"fmt"

	costtracker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/cost/cost-tracker"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// ErrBudgetExceeded is the sentinel error that *BudgetExceededError wraps via
// Unwrap. Consumers should match budget denials with errors.Is rather than
// type-asserting, so that code written against this package continues to work
// if the concrete error gains additional fields in a future version:
//
//	if errors.Is(err, llminstrument.ErrBudgetExceeded) { ... }
//
// Both patterns are valid and interoperable:
//
//	errors.Is(err, llminstrument.ErrBudgetExceeded)  // sentinel check
//	errors.As(err, &be)                               // for accessing Reason
var ErrBudgetExceeded = errors.New("budget exceeded")

// BudgetExceededError is returned by WithBudget when any quota dimension is
// exceeded before making the call. It is non-retryable: llmclient.WithRetry
// never retries it (it matches neither RateLimitError nor ServerError).
//
// Use errors.Is(err, ErrBudgetExceeded) to test for budget denial in a
// sentinel-compatible way. Use errors.As(err, &be) to retrieve the Reason.
type BudgetExceededError struct {
	Reason string
}

// Error implements the error interface, reporting which budget dimension
// was exceeded.
func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("budget exceeded: %s", e.Reason)
}

// Unwrap returns ErrBudgetExceeded so that errors.Is(err, ErrBudgetExceeded)
// returns true for any *BudgetExceededError value, regardless of the Reason.
func (e *BudgetExceededError) Unwrap() error { return ErrBudgetExceeded }

// budgetClient wraps an inner LLMClient and gates every call through a
// budgetguard.Guard, using a two-phase ChargeTracked+Settle pattern so that
// the guard's accumulated USD spend reflects actual (not estimated) costs.
type budgetClient struct {
	inner        llmclient.LLMClient
	guard        *budgetguard.Guard
	table        costtracker.PriceTable
	est          costtracker.Estimator
	defaultModel string
}

var _ llmclient.LLMClient = (*budgetClient)(nil)

// WithBudget wraps inner so that every Complete and Stream call:
//  1. Estimates the pre-flight cost (input tokens from request text + output
//     cap from MaxTokens, defaulting to 4096 when zero).
//  2. Calls guard.ChargeTracked with the estimate. If denied, returns
//     *BudgetExceededError immediately — the inner client is never called.
//  3. On completion, calls guard.Settle to reconcile the estimate against the
//     actual cost from the response's Usage (zero for unknown models).
//     Failed inner calls settle to $0 so the pre-flight estimate is fully
//     reversed.
//
// Parameters:
//   - inner: the client to wrap (typically WithCost(WithTelemetry(WithRetry(provider)))).
//   - guard: the budget quota guard. Must be non-nil.
//   - table: the price table used for pre-flight estimation and actual pricing.
//   - est: optional token-count estimator; nil uses costtracker.EstimateTokens.
//   - defaultModel: model name used for pricing when req.Model is "".
//
// # Composition order
//
// WithBudget must sit OUTERMOST so that retries inside WithRetry are treated
// as a single logical call from the budget's perspective:
//
//	WithBudget(WithCost(WithTelemetry(WithRetry(client), ...), ...), guard, table, nil, "")
//
// Budget charges once per LOGICAL call; retries never trigger re-charging.
// This is the correct billing semantics: the budget enforces spend on logical
// calls, not on individual network attempts.
//
// # Concurrency
//
// budgetClient is safe for concurrent use, provided inner, guard, and table
// honour their own concurrency contracts (all three do).
func WithBudget(inner llmclient.LLMClient, guard *budgetguard.Guard, table costtracker.PriceTable, est costtracker.Estimator, defaultModel string) llmclient.LLMClient {
	return &budgetClient{
		inner:        inner,
		guard:        guard,
		table:        table,
		est:          costtracker.EstimatorOrDefault(est),
		defaultModel: defaultModel,
	}
}

// Complete gates the call through the guard, makes the inner call on success,
// then settles the charge to the actual cost.
func (b *budgetClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	model := b.resolveModel(req.Model)
	estIn, estOut := b.estimateTokens(req)
	estUSD := b.estimateCost(model, estIn, estOut)

	d, id := b.guard.ChargeTracked(estUSD)
	if !d.Allowed {
		return llmclient.Response{}, &BudgetExceededError{Reason: d.Reason}
	}

	resp, err := b.inner.Complete(ctx, req)
	if err != nil {
		b.guard.Settle(id, 0)
		return resp, err
	}

	actualUSD, _ := b.table.Cost(model, resp.Usage)
	b.guard.Settle(id, actualUSD)
	return resp, nil
}

// Stream gates the call through the guard, starts the inner stream on success,
// and settles the charge at stream termination (at the terminal chunk for
// success, or at $0 for pre-stream or mid-stream errors).
func (b *budgetClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	model := b.resolveModel(req.Model)
	estIn, estOut := b.estimateTokens(req)
	estUSD := b.estimateCost(model, estIn, estOut)

	d, id := b.guard.ChargeTracked(estUSD)
	if !d.Allowed {
		return nil, &BudgetExceededError{Reason: d.Reason}
	}

	inner, err := b.inner.Stream(ctx, req)
	if err != nil {
		// Pre-stream error: reverse the estimate.
		b.guard.Settle(id, 0)
		return nil, err
	}

	return watchStream(ctx, inner, func(stop llmclient.StopReason, usage *llmclient.Usage, streamErr error) {
		if streamErr == nil && usage != nil {
			// Terminal success: settle to actual cost.
			actualUSD, _ := b.table.Cost(model, *usage)
			b.guard.Settle(id, actualUSD)
		} else {
			// Mid-stream error or stream with no terminal chunk: reverse estimate.
			b.guard.Settle(id, 0)
		}
	}), nil
}

// resolveModel returns model when non-empty, otherwise b.defaultModel.
func (b *budgetClient) resolveModel(model string) string {
	if model != "" {
		return model
	}
	return b.defaultModel
}

// defaultMaxOutputTokens is the fallback output cap used when req.MaxTokens == 0.
const defaultMaxOutputTokens = 4096

// estimateTokens returns estimated (inputTokens, outputTokens) for req.
// Input tokens are estimated from the concatenated text of System, Prompt,
// and all message block Text/Content fields. Output tokens are capped by
// MaxTokens (defaulting to defaultMaxOutputTokens when zero).
func (b *budgetClient) estimateTokens(req llmclient.Request) (in, out int) {
	var text string
	text += req.System
	text += req.Prompt
	for _, msg := range req.Messages {
		for _, blk := range msg.Content {
			text += blk.Text
			text += blk.Content
		}
	}
	in = b.est(text)
	out = req.MaxTokens
	if out == 0 {
		out = defaultMaxOutputTokens
	}
	return in, out
}

// estimateCost computes the pre-flight USD estimate from token counts.
// Returns 0 for unknown models (the guard still charges 0, tracking the call).
func (b *budgetClient) estimateCost(model string, inTok, outTok int) float64 {
	u := llmclient.Usage{InputTokens: inTok, OutputTokens: outTok}
	cost, _ := b.table.Cost(model, u)
	return cost
}

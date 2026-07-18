package llmclient

import (
	"context"
	"math/rand"
	"time"
)

// RetryConfig controls the retry behaviour of WithRetry.
type RetryConfig struct {
	// MaxAttempts is the total number of tries including the first.
	// A value of 1 means no retries (one attempt only).
	MaxAttempts int

	// BaseDelay is the initial backoff duration before the second attempt.
	// Subsequent delays double (exponential backoff) before jitter is applied.
	BaseDelay time.Duration

	// Jitter adds a random fraction of the current delay to prevent thundering
	// herd on shared API endpoints. When Jitter is true the actual delay is
	//   delay + rand.Int63n(delay)
	// The random source is injected via JitterSource so tests can be
	// deterministic.
	Jitter bool

	// JitterSource is the random source used to compute jitter. If nil and
	// Jitter is true, a default source seeded with the current time is used.
	// Inject a seeded *rand.Rand in tests to make backoff deterministic.
	JitterSource *rand.Rand
}

// retryClient wraps an LLMClient with retry logic.
type retryClient struct {
	inner  LLMClient
	cfg    RetryConfig
	jitter *rand.Rand
}

// WithRetry wraps inner with exponential-backoff retry logic described by cfg.
// Retryable errors (RateLimitError, ServerError) are retried up to
// cfg.MaxAttempts times in total. Non-retryable errors (InvalidRequestError,
// AuthError, and any other error type) cause an immediate failure regardless
// of remaining attempts.
//
// A nil error is always success. In particular, a Response with
// StopReason == StopToolUse is a successful completion — it is returned
// immediately, never retried, and never treated as an escalation signal.
//
// When a RateLimitError carries a RetryAfter hint (> 0), the next sleep uses
// that hint verbatim instead of the computed exponential backoff (no jitter is
// added to a server-provided hint).
//
// The returned Response.Attempts field reflects the total number of attempts made.
func WithRetry(inner LLMClient, cfg RetryConfig) LLMClient {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}

	var src *rand.Rand
	if cfg.Jitter {
		if cfg.JitterSource != nil {
			src = cfg.JitterSource
		} else {
			//nolint:gosec // non-cryptographic jitter
			src = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
	}

	return &retryClient{inner: inner, cfg: cfg, jitter: src}
}

// sleepDuration computes how long to wait before the next attempt: the
// server's RetryAfter hint when err carries one, otherwise the current
// exponential delay plus optional jitter.
func (r *retryClient) sleepDuration(err error, delay time.Duration) time.Duration {
	if hint := retryAfterHint(err); hint > 0 {
		return hint
	}
	d := delay
	if r.cfg.Jitter && r.jitter != nil && delay > 0 {
		d += time.Duration(r.jitter.Int63n(int64(delay)))
	}
	return d
}

func (r *retryClient) Complete(ctx context.Context, req Request) (Response, error) {
	var (
		resp    Response
		err     error
		delay   = r.cfg.BaseDelay
		attempt int
	)

	for attempt = 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		resp, err = r.inner.Complete(ctx, req)
		resp.Attempts = attempt

		if err == nil {
			// Success — including StopReason == StopToolUse, which is a
			// successful completion, not a retry or escalation signal.
			return resp, nil
		}

		// Non-retryable: fail immediately.
		if !isRetryable(err) {
			return resp, err
		}

		// No more attempts left.
		if attempt >= r.cfg.MaxAttempts {
			break
		}

		// Respect context cancellation before sleeping.
		select {
		case <-ctx.Done():
			resp.Attempts = attempt
			return resp, ctx.Err()
		default:
		}

		// Sleep with context awareness.
		timer := time.NewTimer(r.sleepDuration(err, delay))
		select {
		case <-ctx.Done():
			timer.Stop()
			resp.Attempts = attempt
			return resp, ctx.Err()
		case <-timer.C:
		}

		delay *= 2 // exponential backoff
	}

	resp.Attempts = attempt
	return resp, err
}

// Stream delegates to the inner client — retry wrapping for streaming covers
// pre-stream failures only: if Stream returns a retryable error before a
// channel is handed out, the call is retried (up to MaxAttempts, honouring
// RetryAfter hints). Mid-stream errors (StreamChunk.Err) cannot be retried
// transparently because the caller may have already consumed part of the
// stream; they pass through untouched.
func (r *retryClient) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	var (
		ch    <-chan StreamChunk
		err   error
		delay = r.cfg.BaseDelay
	)

	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		ch, err = r.inner.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}

		if !isRetryable(err) {
			return nil, err
		}

		if attempt >= r.cfg.MaxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		timer := time.NewTimer(r.sleepDuration(err, delay))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		delay *= 2
	}

	return nil, err
}

package llmclient

import (
	"context"
	"errors"
	"sync"
	"time"
)

// FailoverReason codes why the failover wrapper chose a lane.
type FailoverReason string

const (
	// FailoverReasonNone means the primary lane served the request normally.
	FailoverReasonNone FailoverReason = ""
	// FailoverReasonAuthError means the primary returned an AuthError.
	FailoverReasonAuthError FailoverReason = "auth_error"
	// FailoverReasonRateLimit means the primary returned a RateLimitError.
	FailoverReasonRateLimit FailoverReason = "rate_limited"
	// FailoverReasonUnavailable means the primary returned an UnavailableError.
	FailoverReasonUnavailable FailoverReason = "unavailable"
)

// FailoverDecision describes which lane served the last request and why.
// It contains no credential or token material.
type FailoverDecision struct {
	// LaneIndex is the zero-based index of the lane that served the last request.
	// Zero before any request.
	LaneIndex int

	// FallbackActive is true when any lane other than lane 0 served the request.
	FallbackActive bool

	// Reason is the typed reason the primary lane was skipped, or
	// FailoverReasonNone when the primary served the request.
	Reason FailoverReason
}

// failoverClient wraps an ordered list of LLMClient lanes with transparent
// failover on eligible errors. It is the carrier for the two-lane
// subscription-first → API-key strategy.
//
// Eligibility: AuthError, RateLimitError, UnavailableError cause failover to
// the next lane. InvalidRequestError and context cancellation propagate
// immediately — retrying with a different lane cannot fix a bad request or an
// already-cancelled context.
//
// Sticky cooldown: after a lane fails over, it is skipped for cooldownWindow
// (default 5 minutes) so subsequent requests skip the failed-round-trip cost.
// A successful direct use of a lane (not via failover) resets its cooldown.
//
// Stream failover: only when the failing lane returned a pre-stream error with
// zero deltas received. Mid-stream errors propagate unchanged — splicing two
// partial answers is forbidden.
//
// Concurrency: safe for concurrent use.
type failoverClient struct {
	lanes          []LLMClient
	cooldownWindow time.Duration
	clock          func() time.Time

	mu        sync.Mutex
	cooldowns []time.Time // cooldowns[i] = time until lane i is cooled down; zero = ready
	last      FailoverDecision
}

// FailoverOption configures a failoverClient.
type FailoverOption func(*failoverClient)

// WithCooldown sets the sticky-cooldown window (default 5m).
// Pass an injectable clock for tests.
func WithCooldown(d time.Duration, clock func() time.Time) FailoverOption {
	return func(f *failoverClient) {
		if d > 0 {
			f.cooldownWindow = d
		}
		if clock != nil {
			f.clock = clock
		}
	}
}

// NewFailover returns an LLMClient that tries lanes in order on eligible
// errors. At least two lanes are required; fewer than two panics.
// Each lane is a fully configured LLMClient (typically WithRetry(Swappable)).
func NewFailover(lanes []LLMClient, opts ...FailoverOption) LLMClient {
	if len(lanes) < 2 {
		panic("llmclient.NewFailover: at least two lanes required")
	}
	f := &failoverClient{
		lanes:          lanes,
		cooldownWindow: 5 * time.Minute,
		clock:          func() time.Time { return time.Now() },
		cooldowns:      make([]time.Time, len(lanes)),
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// LastDecision returns the failover decision for the last request. Safe to
// call concurrently. Before any request it returns the zero value (lane 0, no
// fallback, no reason).
func (f *failoverClient) LastDecision() FailoverDecision {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last
}

// isEligibleForFailover returns true when err is a lane-level failure that
// warrants trying the next lane. Bad requests and context cancellation must
// not cross lanes.
func isEligibleForFailover(err error) (FailoverReason, bool) {
	if err == nil {
		return FailoverReasonNone, false
	}
	var ae *AuthError
	if errors.As(err, &ae) {
		return FailoverReasonAuthError, true
	}
	var rle *RateLimitError
	if errors.As(err, &rle) {
		return FailoverReasonRateLimit, true
	}
	var ue *UnavailableError
	if errors.As(err, &ue) {
		return FailoverReasonUnavailable, true
	}
	return FailoverReasonNone, false
}

// activeLane returns the first lane whose cooldown has expired, starting at
// lane 0. Must be called with f.mu held.
func (f *failoverClient) activeLane() int {
	now := f.clock()
	for i, cd := range f.cooldowns {
		if cd.IsZero() || now.After(cd) {
			return i
		}
	}
	// All lanes cooled down — try last lane anyway.
	return len(f.lanes) - 1
}

// setCooldown marks lane i as cooling down until now+window. Must be called
// with f.mu held.
func (f *failoverClient) setCooldown(i int) {
	f.cooldowns[i] = f.clock().Add(f.cooldownWindow)
}

// resetCooldown clears the cooldown for lane i (successful direct use).
// Must be called with f.mu held.
func (f *failoverClient) resetCooldown(i int) {
	f.cooldowns[i] = time.Time{}
}

// Complete tries lanes in order starting from the first non-cooled lane.
// On an eligible error it marks the lane on cooldown and tries the next.
// Ineligible errors propagate immediately.
func (f *failoverClient) Complete(ctx context.Context, req Request) (Response, error) {
	f.mu.Lock()
	start := f.activeLane()
	f.mu.Unlock()

	var (
		lastErr    error
		lastReason FailoverReason
		servedBy   int
	)

	for i := start; i < len(f.lanes); i++ {
		resp, err := f.lanes[i].Complete(ctx, req)
		if err == nil {
			f.mu.Lock()
			f.resetCooldown(i)
			f.last = FailoverDecision{
				LaneIndex:      i,
				FallbackActive: i > 0,
				Reason:         lastReason,
			}
			servedBy = i
			_ = servedBy
			f.mu.Unlock()
			return resp, nil
		}

		// Context cancellation: propagate immediately, never failover.
		if ctx.Err() != nil {
			return resp, err
		}

		reason, eligible := isEligibleForFailover(err)
		if !eligible {
			// Ineligible error (e.g. InvalidRequestError): propagate now.
			f.mu.Lock()
			f.last = FailoverDecision{LaneIndex: i, FallbackActive: i > 0, Reason: reason}
			f.mu.Unlock()
			return resp, err
		}

		// Eligible: mark cooldown and try next lane.
		lastErr = err
		lastReason = reason
		f.mu.Lock()
		f.setCooldown(i)
		f.mu.Unlock()
	}

	// All lanes exhausted.
	f.mu.Lock()
	f.last = FailoverDecision{
		LaneIndex:      len(f.lanes) - 1,
		FallbackActive: start < len(f.lanes)-1 || lastReason != FailoverReasonNone,
		Reason:         lastReason,
	}
	f.mu.Unlock()
	return Response{}, lastErr
}

// Stream tries lanes in order. Failover only on pre-stream errors (zero deltas
// received). Mid-stream errors propagate without lane switching.
func (f *failoverClient) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	f.mu.Lock()
	start := f.activeLane()
	f.mu.Unlock()

	var (
		lastErr    error
		lastReason FailoverReason
	)

	for i := start; i < len(f.lanes); i++ {
		ch, err := f.lanes[i].Stream(ctx, req)
		if err != nil {
			// Context cancellation: propagate immediately.
			if ctx.Err() != nil {
				return nil, err
			}
			reason, eligible := isEligibleForFailover(err)
			if !eligible {
				f.mu.Lock()
				f.last = FailoverDecision{LaneIndex: i, FallbackActive: i > 0, Reason: reason}
				f.mu.Unlock()
				return nil, err
			}
			// Pre-stream eligible error: try next lane.
			lastErr = err
			lastReason = reason
			f.mu.Lock()
			f.setCooldown(i)
			f.mu.Unlock()
			continue
		}

		// Stream opened. Read first chunk to detect zero-delta pre-stream
		// failures delivered via the channel. If no delta seen before error,
		// failover. If any delta seen, propagate — mid-stream splice forbidden.
		//
		// We wrap the channel in a lookahead: consume the first chunk.
		// If it is an error chunk with zero prior text, fail over.
		// Otherwise wrap ch back into a combined channel with the peeked chunk prepended.

		peeked, ok := <-ch
		if !ok {
			// Channel closed immediately (empty stream). Treat as success.
			f.mu.Lock()
			f.resetCooldown(i)
			f.last = FailoverDecision{LaneIndex: i, FallbackActive: i > 0, Reason: lastReason}
			f.mu.Unlock()
			empty := make(chan StreamChunk)
			close(empty)
			return empty, nil
		}

		if peeked.Err != nil && peeked.TextDelta == "" && peeked.StopReason == "" {
			// Zero-delta error chunk — eligible for stream failover.
			reason, eligible := isEligibleForFailover(peeked.Err)
			if eligible {
				if ctx.Err() != nil {
					return nil, peeked.Err
				}
				lastErr = peeked.Err
				lastReason = reason
				f.mu.Lock()
				f.setCooldown(i)
				f.mu.Unlock()
				continue
			}
		}

		// First chunk is usable (or is a non-eligible error): re-attach it.
		f.mu.Lock()
		f.resetCooldown(i)
		f.last = FailoverDecision{LaneIndex: i, FallbackActive: i > 0, Reason: lastReason}
		f.mu.Unlock()

		out := make(chan StreamChunk, 1)
		go func(first StreamChunk, rest <-chan StreamChunk) {
			defer close(out)
			out <- first
			if first.Err != nil || first.StopReason != "" {
				return
			}
			for c := range rest {
				out <- c
				if c.Err != nil || c.StopReason != "" {
					return
				}
			}
		}(peeked, ch)
		return out, nil
	}

	// All lanes exhausted.
	f.mu.Lock()
	f.last = FailoverDecision{
		LaneIndex:      len(f.lanes) - 1,
		FallbackActive: lastReason != FailoverReasonNone,
		Reason:         lastReason,
	}
	f.mu.Unlock()
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &UnavailableError{Message: "failover: all lanes exhausted"}
}

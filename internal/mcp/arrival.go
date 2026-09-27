package mcp

// arrival.go — RT-179 (the wave-B live reading's F2): a call's clock runs
// from the request's arrival at the daemon, not from the moment dispatch
// gets to it. The call log's latency_ms and queued_ms include any wait
// before admission, and the admission gate measures the class deadline
// from the same instant, so a read that waited past its deadline is
// refused by name instead of run for nobody and logged as fast.

import (
	"context"
	"time"
)

type arrivalKey struct{}

// WithArrival records when the request carrying ctx arrived (the HTTP
// layer sets it before reading the body).
func WithArrival(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, arrivalKey{}, t)
}

// arrivalFrom is ctx's arrival, or the zero time when none was recorded.
func arrivalFrom(ctx context.Context) time.Time {
	t, _ := ctx.Value(arrivalKey{}).(time.Time)
	return t
}

// sinceArrival is how long ago ctx's request arrived; 0 when unrecorded.
func sinceArrival(ctx context.Context) time.Duration {
	if t := arrivalFrom(ctx); !t.IsZero() {
		return max(time.Since(t), 0)
	}
	return 0
}

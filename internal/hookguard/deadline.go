package hookguard

import (
	"context"
	"strconv"
	"time"
)

// DeadlineHeader carries the milliseconds a hook RPC has left, so the
// daemon can bound (and drop) work whose answer nobody will read (§3.5).
const DeadlineHeader = "X-Fathom-Deadline"

// Deadline is one hook invocation's monotonic deadline. The zero value is
// "no deadline" (a CLI verb, not a hook).
type Deadline struct {
	start time.Time
	at    time.Time
}

// HookDeadline returns the deadline for a hook invocation that started at
// start under a harness descriptor timeout of timeoutSec:
// start + timeoutSec − max(1 s, 20%). Claude Code's 5 s hooks get 4 s,
// 10 s hooks 8 s, Codex's 3 s SessionEnd 2 s — the margin is the hook's
// own exit and the harness's bookkeeping, so the harness never has to kill
// the hook. start should come from time.Now(), whose monotonic reading
// makes the deadline immune to wall-clock steps. timeoutSec <= 0 (no
// descriptor) is the zero Deadline.
func HookDeadline(start time.Time, timeoutSec int) Deadline {
	if timeoutSec <= 0 {
		return Deadline{}
	}
	total := time.Duration(timeoutSec) * time.Second
	margin := total / 5
	if margin < time.Second {
		margin = time.Second
	}
	budget := total - margin
	if budget <= 0 {
		budget = total / 2
	}
	return Deadline{start: start, at: start.Add(budget)}
}

// IsZero reports "no deadline".
func (d Deadline) IsZero() bool { return d.at.IsZero() }

// At is the deadline instant.
func (d Deadline) At() time.Time { return d.at }

// Budget is the whole allowance from start to deadline.
func (d Deadline) Budget() time.Duration { return d.at.Sub(d.start) }

// Remaining is the time left, never negative; a very large value when
// there is no deadline.
func (d Deadline) Remaining() time.Duration {
	if d.IsZero() {
		return time.Duration(1<<62 - 1)
	}
	if r := time.Until(d.at); r > 0 {
		return r
	}
	return 0
}

// Spent reports a deadline that has passed.
func (d Deadline) Spent() bool { return !d.IsZero() && d.Remaining() == 0 }

// Context derives a context that ends at the deadline (or parent's end,
// whichever is first). With no deadline it is a plain cancelable child.
func (d Deadline) Context(parent context.Context) (context.Context, context.CancelFunc) {
	if d.IsZero() {
		return context.WithCancel(parent)
	}
	return context.WithDeadline(parent, d.at)
}

// HeaderValue is the X-Fathom-Deadline value: whole milliseconds left.
func (d Deadline) HeaderValue() string {
	if d.IsZero() {
		return ""
	}
	return strconv.FormatInt(d.Remaining().Milliseconds(), 10)
}

// Scale shrinks (or stretches) the budget by f, keeping the start. It
// exists so a test can drive every descriptor's real deadline through a
// hanging daemon in a fraction of wall time; f <= 0 or no deadline is a
// no-op.
func (d Deadline) Scale(f float64) Deadline {
	if d.IsZero() || f <= 0 {
		return d
	}
	return Deadline{start: d.start, at: d.start.Add(time.Duration(float64(d.Budget()) * f))}
}

package tenancy

import "context"

// RunUntilDone runs fn in a goroutine and returns its result, or ctx's error
// the moment ctx is done, whichever comes first. On ctx done the goroutine is
// ABANDONED, not cancelled: it keeps running until the process exits, so this
// is for one-shot hook processes only (the process exit is the cancellation)
// and never for the daemon, where it would leak the work. A ctx already done
// returns its error without starting fn.
func RunUntilDone[T any](ctx context.Context, fn func() T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	ch := make(chan T, 1)
	go func() { ch <- fn() }()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

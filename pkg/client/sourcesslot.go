package client

// sourcesslot.go — RT-146 (docs/design/responsible-tenant.md §4.6, the
// wave-A backstop): until whole-tree reads leave the request path,
// (*Client).Sources takes one of two process-wide slots. A caller whose
// ctx is marked hook-origin never waits for one — a hook has a deadline
// measured in seconds and its answer is optional — and gets ErrSourcesBusy,
// a named skip. Every other caller waits, bounded by its own ctx. This caps
// the remaining whole-tree readers (console project index, report
// evidence, review, spec coverage, discoveries, …) at two at a time.

import (
	"context"
	"errors"
)

// maxConcurrentSources is the process-wide whole-tree read bound.
const maxConcurrentSources = 2

var sourcesSlots = make(chan struct{}, maxConcurrentSources)

// ErrSourcesBusy is the named skip a hook-origin Sources call gets when both
// slots are held. Callers render it as a skip, never as an empty corpus.
var ErrSourcesBusy = errors.New("coverage skipped: whole-tree read in progress")

type hookOriginKey struct{}

// WithHookOrigin marks ctx as a hook's request: a whole-tree read under it
// never waits for a slot (RT-146).
func WithHookOrigin(ctx context.Context) context.Context {
	return context.WithValue(ctx, hookOriginKey{}, true)
}

// IsHookOrigin reports whether ctx was marked by WithHookOrigin.
func IsHookOrigin(ctx context.Context) bool {
	v, _ := ctx.Value(hookOriginKey{}).(bool)
	return v
}

// acquireSourcesSlot takes a whole-tree read slot, returning its release.
// A hook-origin ctx gets ErrSourcesBusy at once when none is free; any
// other ctx waits until a slot frees or ctx ends.
func acquireSourcesSlot(ctx context.Context) (release func(), err error) {
	release = func() { <-sourcesSlots }
	select {
	case sourcesSlots <- struct{}{}:
		return release, nil
	default:
	}
	if IsHookOrigin(ctx) {
		return nil, ErrSourcesBusy
	}
	select {
	case sourcesSlots <- struct{}{}:
		return release, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

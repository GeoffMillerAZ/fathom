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
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// maxConcurrentSources is the process-wide whole-tree read bound.
const maxConcurrentSources = 2

var sourcesSlots = make(chan struct{}, maxConcurrentSources)

// ErrSourcesBusy is the named skip a hook-origin Sources call gets when both
// slots are held. Callers render it as a skip, never as an empty corpus.
// It is core.ErrSourcesBusy, so internal/mcp names it without importing
// this package.
var ErrSourcesBusy = core.ErrSourcesBusy

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

type sourcesScopeKey struct{}

// sourcesScope is one request's hold on a whole-tree slot. The slot bounds
// whole trees in memory, not only the read phase: a request's Sources
// result lives on while the request renders, so inside a scope the slot is
// taken on the first Sources call and released only when the request ends
// (done). A second Sources in the same scope reuses it — never a
// self-deadlock.
type sourcesScope struct {
	mu      sync.Mutex
	held    bool
	ended   bool
	abandon <-chan struct{}
}

// WithSourcesScope marks ctx as one request's scope for whole-tree reads
// (RT-146): the first Sources under it takes a slot and keeps it until done
// is called; done is idempotent. abandon, when non-nil, ends a wait for a
// slot and cancels the read itself — the daemon passes the HTTP request's
// Done channel, since its /rpc ctx is deliberately detached from the
// connection (an ingest must survive a disconnect) and has no deadline.
func WithSourcesScope(ctx context.Context, abandon <-chan struct{}) (context.Context, func()) {
	sc := &sourcesScope{abandon: abandon}
	done := func() {
		sc.mu.Lock()
		defer sc.mu.Unlock()
		if sc.held {
			<-sourcesSlots
			sc.held = false
		}
		sc.ended = true
	}
	return context.WithValue(ctx, sourcesScopeKey{}, sc), done
}

func scopeFrom(ctx context.Context) *sourcesScope {
	sc, _ := ctx.Value(sourcesScopeKey{}).(*sourcesScope)
	return sc
}

// abandonOf is the scope's abandon channel (nil outside a scope; a nil
// channel never fires in a select).
func abandonOf(ctx context.Context) <-chan struct{} {
	if sc := scopeFrom(ctx); sc != nil {
		return sc.abandon
	}
	return nil
}

// cancelOnAbandon derives a ctx that ends when ctx's scope is abandoned, so
// a request abandoned mid-read stops the walk and the fetch loop too, not
// only the wait for a slot (the daemon's /rpc ctx is detached from the
// connection). Outside a scope it returns ctx and a no-op cancel.
func cancelOnAbandon(ctx context.Context) (context.Context, context.CancelFunc) {
	abandon := abandonOf(ctx)
	if abandon == nil {
		return ctx, func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	//fathom:tenancy-allow bare-go ties a derived context to the slot's release; exits on either the caller's context or the release
	go func() {
		select {
		case <-abandon:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// acquireSourcesSlot takes a whole-tree read slot, returning its release.
// A hook-origin ctx gets ErrSourcesBusy at once when none is free; any
// other ctx waits until a slot frees, ctx ends, or its scope is abandoned.
// Inside a live scope the slot is held until the scope's done, and the
// returned release is a no-op.
func acquireSourcesSlot(ctx context.Context) (release func(), err error) {
	if sc := scopeFrom(ctx); sc != nil {
		sc.mu.Lock()
		defer sc.mu.Unlock()
		if !sc.ended {
			if sc.held {
				return func() {}, nil
			}
			if err := takeSourcesSlot(ctx, sc.abandon); err != nil {
				return nil, err
			}
			sc.held = true
			return func() {}, nil
		}
	}
	if err := takeSourcesSlot(ctx, abandonOf(ctx)); err != nil {
		return nil, err
	}
	return func() { <-sourcesSlots }, nil
}

func takeSourcesSlot(ctx context.Context, abandon <-chan struct{}) error {
	select {
	case sourcesSlots <- struct{}{}:
		return nil
	default:
	}
	if IsHookOrigin(ctx) {
		return ErrSourcesBusy
	}
	select {
	case sourcesSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-abandon:
		return context.Canceled
	}
}

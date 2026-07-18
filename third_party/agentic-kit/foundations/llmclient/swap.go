package llmclient

import (
	"context"
	"sync/atomic"
)

// Swappable is an LLMClient whose underlying client can be replaced
// atomically at runtime. It is the late-bound credential pattern for
// local-first applications: construct the Swappable unconfigured (nil inner)
// at startup, wire it through the application as the one LLMClient, and Swap
// in a real provider client the moment credentials arrive (e.g. the user
// pastes a key into a Settings page). No plumbing downstream of the Swappable
// ever changes.
//
// While no inner client is configured, Available reports false and
// Complete/Stream fail with a typed *UnavailableError — non-retryable by
// WithRetry's classification. Callers surface a "configure your credentials"
// prompt; they do not retry.
//
// Concurrency: Swappable is safe for concurrent use. Swap is a single atomic
// pointer store; Complete/Stream load the pointer once per call, so a call
// already in flight on the previous inner client is never interrupted — it
// holds its own reference, and any stream it returned keeps its normal
// lifecycle. Swap provides no rendezvous: it returns immediately and does not
// wait for in-flight calls on the old client to finish.
type Swappable struct {
	inner atomic.Pointer[swapSlot]
}

// swapSlot boxes the inner client so an interface value can live behind an
// atomic.Pointer (which requires a concrete pointed-to type).
type swapSlot struct {
	client LLMClient
}

var _ LLMClient = (*Swappable)(nil)

// NewSwappable returns a Swappable wrapping inner. A nil inner is valid and
// means "not yet configured": Available reports false and every call fails
// with a typed *UnavailableError until Swap installs a client.
func NewSwappable(inner LLMClient) *Swappable {
	s := &Swappable{}
	s.Swap(inner)
	return s
}

// Swap atomically replaces the inner client. Passing nil returns the
// Swappable to the unconfigured state (e.g. the user cleared or revoked the
// credential). Calls already in flight on the previous client are not
// interrupted; calls that start after Swap returns see the new client.
func (s *Swappable) Swap(inner LLMClient) {
	if inner == nil {
		s.inner.Store(nil)
		return
	}
	s.inner.Store(&swapSlot{client: inner})
}

// Available reports whether an inner client is currently configured. False
// means Complete and Stream would fail with *UnavailableError — show a
// configuration prompt instead of calling. Note the answer is instantaneous:
// a concurrent Swap may change it immediately after Available returns, so
// treat it as a UI hint, not a lock.
func (s *Swappable) Available() bool {
	return s.inner.Load() != nil
}

// current returns the configured inner client, or nil when unconfigured.
func (s *Swappable) current() LLMClient {
	if slot := s.inner.Load(); slot != nil {
		return slot.client
	}
	return nil
}

// Complete forwards to the current inner client. When none is configured it
// fails with a typed *UnavailableError (non-retryable) and performs no work.
func (s *Swappable) Complete(ctx context.Context, req Request) (Response, error) {
	c := s.current()
	if c == nil {
		return Response{}, &UnavailableError{Message: "no client configured; Swap in a provider client first"}
	}
	return c.Complete(ctx, req)
}

// Stream forwards to the current inner client. When none is configured it
// fails PRE-stream with a typed *UnavailableError — no channel is created.
func (s *Swappable) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	c := s.current()
	if c == nil {
		return nil, &UnavailableError{Message: "no client configured; Swap in a provider client first"}
	}
	return c.Stream(ctx, req)
}

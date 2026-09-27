// Package tenancy holds fathom's responsible-tenant policy (ADR-047,
// docs/design/responsible-tenant.md): pure decisions over injected
// samples and clocks, applied by cmd/fathom.
package tenancy

import (
	"fmt"
	"time"
)

// UnmanagedBounds are the lifetime bounds of an unmanaged serve-http
// (§10.1): a scratch daemon a test, script, bench or agent started. A zero
// bound is disabled.
type UnmanagedBounds struct {
	IdleExit time.Duration // no request in flight and none for this long
	TTL      time.Duration // hard stop after this long, busy or not
}

// DefaultUnmanagedBounds are §10.1's defaults: idle 20 min, TTL 8 h.
func DefaultUnmanagedBounds() UnmanagedBounds {
	return UnmanagedBounds{IdleExit: 20 * time.Minute, TTL: 8 * time.Hour}
}

// UnmanagedState is one observation of a running unmanaged daemon.
type UnmanagedState struct {
	Now           time.Time
	StartedAt     time.Time
	LastActivity  time.Time // the last request's start or end
	InFlight      int       // requests being served; an /events stream counts
	ExeUnlinked   bool      // fstat of the executable: nlink == 0
	StoreUnlinked bool      // fstat of the store: nlink == 0
}

// EvaluateUnmanagedExit decides whether an unmanaged daemon exits now and
// names why. Order: an unlinked executable, an unlinked store (both free
// deleted files the process holds open), the TTL, then idle. Pure.
func EvaluateUnmanagedExit(b UnmanagedBounds, s UnmanagedState) (reason string, exit bool) {
	switch {
	case s.ExeUnlinked:
		return "executable unlinked", true
	case s.StoreUnlinked:
		return "store unlinked", true
	case b.TTL > 0 && s.Now.Sub(s.StartedAt) >= b.TTL:
		return fmt.Sprintf("ttl %s", b.TTL), true
	case b.IdleExit > 0 && s.InFlight == 0 && s.Now.Sub(s.LastActivity) >= b.IdleExit:
		return fmt.Sprintf("idle %s", b.IdleExit), true
	}
	return "", false
}

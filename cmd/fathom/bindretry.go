package main

// bindretry.go — DG-19 (diagnostics sweep, owner amendment (c), P2): a
// self-restart (selfrestart.go) or a `task deploy` exits the OLD daemon so
// launchd can start the new one; if the old process is still tearing down
// (closing its listener, flushing the store) when launchd races the new
// one up, ListenAndServe's bind can lose to "address already in use" even
// though nothing is permanently wrong — the previous instance is on its
// way out. Retrying the BIND (not the whole serve loop) with backoff for a
// bounded window absorbs exactly that race without masking a genuinely
// stuck port forever.

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

var (
	// bindRetryMaxWait bounds the total time spent retrying before giving
	// up and returning the bind error — "up to 10 s" per the owner's
	// amendment.
	bindRetryMaxWait = 10 * time.Second // a var so a test can shorten it
	// bindRetryInitialBackoff is the first retry's delay; it doubles (capped
	// at bindRetryMaxBackoff) on each subsequent attempt.
	bindRetryInitialBackoff = 250 * time.Millisecond
	bindRetryMaxBackoff     = 2 * time.Second
)

// isAddrInUse reports whether err is a bind failure because the address is
// already in use (EADDRINUSE) — the one class worth retrying. Any other
// failure (bad address, permission denied, ...) is not: retrying it would
// only delay an operator seeing a real misconfiguration.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// listenWithBackoff binds addr, retrying ONLY an EADDRINUSE failure with
// exponential backoff (capped) for up to maxWait total, logging each retry
// at warn — then returns the last bind error unchanged, exactly as an
// immediate, non-retried bind failure would have. listen and sleep are
// injected (production: net.Listen and time.Sleep) so tests exercise the
// retry/backoff/give-up logic without a real listening socket or real
// wall-clock delay.
func listenWithBackoff(addr string, maxWait time.Duration, listen func(network, address string) (net.Listener, error), sleep func(time.Duration), log *dlog.Logger) (net.Listener, error) {
	backoff := bindRetryInitialBackoff
	var elapsed time.Duration
	attempt := 0
	for {
		ln, err := listen("tcp", addr)
		if err == nil {
			if attempt > 0 {
				log.Info(fmt.Sprintf("serve-http: bound %s after %d retry attempt(s) — a previous instance was likely still exiting", addr, attempt))
			}
			return ln, nil
		}
		if !isAddrInUse(err) {
			return nil, err
		}
		if elapsed >= maxWait {
			return nil, err
		}

		attempt++
		wait := backoff
		if elapsed+wait > maxWait {
			wait = maxWait - elapsed
		}
		log.Warn(fmt.Sprintf("serve-http: %s is already in use (attempt %d, retrying in %s) — a previous instance may still be exiting", addr, attempt, wait))
		sleep(wait)
		elapsed += wait

		backoff *= 2
		if backoff > bindRetryMaxBackoff {
			backoff = bindRetryMaxBackoff
		}
	}
}

// listenTCPWithRetry is runServeHTTP's production entry point: real
// net.Listen, real time.Sleep, the standard 10s window.
func listenTCPWithRetry(addr string, log *dlog.Logger) (net.Listener, error) {
	return listenWithBackoff(addr, bindRetryMaxWait, net.Listen, time.Sleep, log)
}

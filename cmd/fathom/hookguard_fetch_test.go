package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// countingRT fails every attempt with err and counts them.
type countingRT struct {
	n   atomic.Int32
	err error
}

func (c *countingRT) RoundTrip(*http.Request) (*http.Response, error) {
	c.n.Add(1)
	return nil, c.err
}

// TestFetchServicesNeverRetriesAfterTimeout is RT-8 (§11.1): the old
// fetchServices answered ANY failure with a fresh 4 s retry client, so a
// hook facing a hung daemon spent its own timeout and then four more
// seconds — the pile-up that let hooks outlive their harness budget. Now
// only a refused or reset connection is retried, once, and only while the
// hook's deadline has time left.
func TestFetchServicesNeverRetriesAfterTimeout(t *testing.T) {
	t.Run("timeout is not retried", func(t *testing.T) {
		var hits atomic.Int32
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		defer srv.Close()
		defer close(release)
		// The hung daemon holds every request until the client leaves, so
		// "it retried" is exactly a second request: the count is the proof
		// (the client's 200 ms is the input, not a duration we assert on).
		_, err := fetchServices(&http.Client{Timeout: 200 * time.Millisecond}, srv.URL)
		if err == nil {
			t.Fatal("want an error from a hung daemon")
		}
		if got := hits.Load(); got != 1 {
			t.Fatalf("requests = %d, want 1 (no retry after a timeout)", got)
		}
	})
	t.Run("HTTP 500 is not retried", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		if _, err := fetchServices(&http.Client{Timeout: time.Second}, srv.URL); err == nil {
			t.Fatal("want an error")
		}
		if got := hits.Load(); got != 1 {
			t.Fatalf("requests = %d, want 1", got)
		}
	})
	for _, tc := range []struct {
		name  string
		errno syscall.Errno
	}{{"refused is retried once", syscall.ECONNREFUSED}, {"reset is retried once", syscall.ECONNRESET}} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &countingRT{err: &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: tc.errno}}}
			_, err := fetchServices(&http.Client{Transport: rt, Timeout: time.Second}, "http://127.0.0.1:1")
			if !errors.Is(err, tc.errno) {
				t.Fatalf("err = %v, want %v", err, tc.errno)
			}
			if got := rt.n.Load(); got != 2 {
				t.Fatalf("attempts = %d, want 2", got)
			}
		})
	}
	t.Run("refused is not retried past the hook deadline", func(t *testing.T) {
		g := &hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now().Add(-time.Hour), 5), HealthDir: t.TempDir(), DaemonDir: t.TempDir()}
		restore := activateHookGuard(g)
		defer restore()
		rt := &countingRT{err: &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}}}
		if _, err := fetchServices(&http.Client{Transport: rt, Timeout: time.Second}, "http://127.0.0.1:1"); err == nil {
			t.Fatal("want an error")
		}
		if got := rt.n.Load(); got != 1 {
			t.Fatalf("attempts = %d, want 1 (deadline spent)", got)
		}
	})
}

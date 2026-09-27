package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

// addrInUseErr builds an error shaped like a real bind failure — wrapping
// syscall.EADDRINUSE the way net.Listen's own *net.OpError does — so
// isAddrInUse's errors.Is check is exercised the same way it would be
// against a genuine bind failure.
func addrInUseErr(addr string) error {
	return fmt.Errorf("listen tcp %s: bind: %w", addr, syscall.EADDRINUSE)
}

func TestListenWithBackoff_SucceedsImmediately(t *testing.T) {
	calls := 0
	listen := func(network, address string) (net.Listener, error) {
		calls++
		return &net.TCPListener{}, nil
	}
	slept := 0
	sleep := func(time.Duration) { slept++ }
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)

	ln, err := listenWithBackoff("127.0.0.1:7581", 10*time.Second, listen, sleep, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ln == nil {
		t.Fatal("expected a non-nil listener on immediate success")
	}
	if calls != 1 {
		t.Errorf("listen called %d times, want exactly 1 (no retry needed)", calls)
	}
	if slept != 0 {
		t.Errorf("sleep called %d times, want 0 — nothing to back off from", slept)
	}
}

func TestListenWithBackoff_NonAddrInUseErrorFailsImmediately(t *testing.T) {
	permErr := errors.New("listen tcp 127.0.0.1:80: bind: permission denied")
	calls := 0
	listen := func(network, address string) (net.Listener, error) {
		calls++
		return nil, permErr
	}
	sleep := func(time.Duration) { t.Fatal("must not sleep/retry a non-EADDRINUSE error") }
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)

	_, err := listenWithBackoff("127.0.0.1:80", 10*time.Second, listen, sleep, log)
	if !errors.Is(err, permErr) {
		t.Fatalf("err = %v, want the original permission error unwrapped", err)
	}
	if calls != 1 {
		t.Errorf("listen called %d times, want exactly 1 (no retry for a non-addr-in-use failure)", calls)
	}
}

// TestListenWithBackoff_RetriesThenSucceeds pins DG-19's core contract: a
// transient EADDRINUSE (the previous instance still exiting) is retried
// with backoff, logged at warn each time, and a subsequent success is
// returned normally.
func TestListenWithBackoff_RetriesThenSucceeds(t *testing.T) {
	const failCount = 3
	calls := 0
	listen := func(network, address string) (net.Listener, error) {
		calls++
		if calls <= failCount {
			return nil, addrInUseErr(address)
		}
		return &net.TCPListener{}, nil
	}
	var slept []time.Duration
	sleep := func(d time.Duration) { slept = append(slept, d) }
	var buf strings.Builder
	log := dlog.New(&buf, dlog.LevelDebug, time.Now)

	ln, err := listenWithBackoff("127.0.0.1:7581", 10*time.Second, listen, sleep, log)
	if err != nil {
		t.Fatalf("unexpected error after eventual success: %v", err)
	}
	if ln == nil {
		t.Fatal("expected a non-nil listener")
	}
	if calls != failCount+1 {
		t.Errorf("listen called %d times, want %d (failCount retries + 1 success)", calls, failCount+1)
	}
	if len(slept) != failCount {
		t.Errorf("slept %d times, want %d (one retry backoff per failure)", len(slept), failCount)
	}
	// Backoff must increase: 250ms, 500ms, 1s.
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second}
	for i, w := range want {
		if i >= len(slept) {
			break
		}
		if slept[i] != w {
			t.Errorf("slept[%d] = %s, want %s", i, slept[i], w)
		}
	}
	if strings.Count(buf.String(), "already in use") != failCount {
		t.Errorf("expected %d \"already in use\" warn lines, got log:\n%s", failCount, buf.String())
	}
	if !strings.Contains(buf.String(), "bound") {
		t.Errorf("expected an eventual-success line naming the retry count:\n%s", buf.String())
	}
}

// TestListenWithBackoff_GivesUpAfterMaxWait pins the bounded-window half:
// a persistently occupied address is retried only until the cumulative
// backoff reaches maxWait, then the original bind error is returned
// unchanged — "then fail with the existing error" per the owner amendment.
func TestListenWithBackoff_GivesUpAfterMaxWait(t *testing.T) {
	calls := 0
	listen := func(network, address string) (net.Listener, error) {
		calls++
		return nil, addrInUseErr(address)
	}
	var totalSlept time.Duration
	sleep := func(d time.Duration) { totalSlept += d }
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)

	maxWait := 2 * time.Second
	_, err := listenWithBackoff("127.0.0.1:7581", maxWait, listen, sleep, log)
	if !isAddrInUse(err) {
		t.Fatalf("err = %v, want the original EADDRINUSE error", err)
	}
	if totalSlept > maxWait {
		t.Errorf("slept a total of %s, want capped at maxWait %s", totalSlept, maxWait)
	}
	if totalSlept < maxWait {
		t.Errorf("slept only %s before giving up, want the full %s window used", totalSlept, maxWait)
	}
	if calls < 2 {
		t.Errorf("listen called only %d time(s); want at least one retry before giving up", calls)
	}
}

func TestIsAddrInUse(t *testing.T) {
	if !isAddrInUse(addrInUseErr("127.0.0.1:7581")) {
		t.Error("isAddrInUse(EADDRINUSE-wrapped error) = false, want true")
	}
	if isAddrInUse(errors.New("some other failure")) {
		t.Error("isAddrInUse(unrelated error) = true, want false")
	}
	if isAddrInUse(nil) {
		t.Error("isAddrInUse(nil) = true, want false")
	}
}

package main

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestInstallKillSwitch_InvokesOnKillSignalOnSIGTERM pins O57 finding 1 at
// the driver level: installKillSwitch arms a handler that calls
// onKillSignal (whose production body is
// scenariobench.KillAllLiveProcessGroups — see
// internal/scenariobench.TestKillAllLiveProcessGroups, which pins THAT
// half with a `sleep` stand-in) the moment SIGTERM arrives. onKillSignal
// is substituted here so this test observes the signal plumbing without
// exercising the real os.Exit.
func TestInstallKillSwitch_InvokesOnKillSignalOnSIGTERM(t *testing.T) {
	old := onKillSignal
	var mu sync.Mutex
	called := false
	done := make(chan struct{})
	onKillSignal = func(sig os.Signal) {
		mu.Lock()
		called = true
		mu.Unlock()
		close(done)
	}
	t.Cleanup(func() { onKillSignal = old })

	stop := installKillSwitch()
	t.Cleanup(stop)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatal("onKillSignal was not called after SIGTERM")
	}

	mu.Lock()
	got := called
	mu.Unlock()
	if !got {
		t.Fatal("onKillSignal was not invoked")
	}
}

// TestInstallKillSwitch_StopDisarms: once stop() is called, that
// particular handler no longer reacts to a signal — proven by confirming a
// SECOND, still-armed kill switch keeps working after the first is
// stopped, which would not be meaningfully distinguishable from "stop is a
// no-op" if the first handler were still silently armed and double-firing
// (signal.Stop, not a shared flag, is what actually disarms it).
func TestInstallKillSwitch_StopDisarms(t *testing.T) {
	old := onKillSignal
	fires := make(chan struct{}, 4)
	onKillSignal = func(sig os.Signal) { fires <- struct{}{} }
	t.Cleanup(func() { onKillSignal = old })

	stop1 := installKillSwitch()
	stop1()

	stop2 := installKillSwitch()
	defer stop2()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fires:
	case <-time.After(waitSafety):
		t.Fatal("the second, still-armed kill switch should have fired")
	}
	// Exactly one firing: the stopped first switch must not have also fired.
	select {
	case <-fires:
		t.Fatal("the stopped kill switch fired too — stop() did not disarm it")
	case <-time.After(200 * time.Millisecond):
	}
}

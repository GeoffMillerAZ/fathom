// signals.go — O57 finding 1's driver-level half: "a driver kills what it
// started." installKillSwitch arms a SIGTERM/SIGINT handler that kills
// every live session process group and scratch daemon this process has
// registered (internal/scenariobench.KillAllLiveProcessGroups) before
// exiting, so a killed `scenariobench run` never leaves an orphan `claude`
// or scratch daemon running against a shared, reused worktree.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// killSwitchExitCode is the conventional 128+SIGTERM/SIGINT exit status a
// process killed by a signal reports.
const killSwitchExitCode = 130

// onKillSignal is the seam installKillSwitch's goroutine calls once a
// tracked signal arrives — a package var so a test can observe it without
// the process actually exiting (os.Exit cannot be recovered from in a
// test). The production default kills every live process group and exits.
var onKillSignal = func(sig os.Signal) {
	killed := scenariobench.KillAllLiveProcessGroups()
	fmt.Fprintf(os.Stderr, "scenariobench: %s received, killed %d live process group(s), exiting\n", sig, len(killed))
	os.Exit(killSwitchExitCode)
}

// installKillSwitch arms the SIGTERM/SIGINT handler for the lifetime of a
// `run` invocation and returns a func that disarms it (deferred by the
// caller) — every subcommand that can start sessions or a scratch daemon
// (currently only `run`) must call this before doing so.
func installKillSwitch() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan struct{})
	//fathom:tenancy-allow bare-go benchmark signal forwarder, one per bench run
	go func() {
		select {
		case sig := <-ch:
			onKillSignal(sig)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

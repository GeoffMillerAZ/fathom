package scenariobench

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestKillAllLiveProcessGroups pins O57 finding 1: a process started in its
// own process group (Setpgid, exactly as harness_claude.go's session
// spawn and subject.go's scratch daemon both do) and registered here is
// killed — process group and all — by KillAllLiveProcessGroups, and is
// unregistered afterward so a second call is a no-op.
func TestKillAllLiveProcessGroups(t *testing.T) {
	before := LiveProcessGroupCount()

	stand := exec.Command("sleep", "300") // the sleep stand-in for `claude`
	stand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stand.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stand.Process.Kill(); _, _ = stand.Process.Wait() })
	pgid := stand.Process.Pid
	registerLiveProcessGroup(pgid, "test-session")

	if got := LiveProcessGroupCount(); got != before+1 {
		t.Fatalf("LiveProcessGroupCount = %d, want %d", got, before+1)
	}

	killed := KillAllLiveProcessGroups()
	found := false
	for _, k := range killed {
		if k == pgid {
			found = true
		}
	}
	if !found {
		t.Fatalf("KillAllLiveProcessGroups() = %v, want it to include pgid %d", killed, pgid)
	}

	if got := LiveProcessGroupCount(); got != before {
		t.Errorf("LiveProcessGroupCount after kill = %d, want %d (the registry entry must be removed)", got, before)
	}

	// The stand-in is this test's own child, so it lingers as a zombie
	// until waited on; Wait returning is the proof it was actually killed.
	waited := make(chan error, 1)
	go func() { _, err := stand.Process.Wait(); waited <- err }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatalf("sleep stand-in pid %d is still alive after KillAllLiveProcessGroups", pgid)
	}

	// A second call finds nothing left to kill.
	if killed2 := KillAllLiveProcessGroups(); len(killed2) != 0 {
		t.Errorf("second KillAllLiveProcessGroups() = %v, want none (already reaped)", killed2)
	}
}

// TestRegisterLiveProcessGroup_IgnoresLowPGIDs: pgid <= 1 is never
// registered — SIGKILLing process group 1 would be catastrophic.
func TestRegisterLiveProcessGroup_IgnoresLowPGIDs(t *testing.T) {
	before := LiveProcessGroupCount()
	registerLiveProcessGroup(0, "bogus")
	registerLiveProcessGroup(1, "bogus")
	registerLiveProcessGroup(-5, "bogus")
	if got := LiveProcessGroupCount(); got != before {
		t.Errorf("LiveProcessGroupCount = %d, want %d (pgid <= 1 must never register)", got, before)
	}
}

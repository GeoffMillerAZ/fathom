package main

import (
	"net/http"
	"strings"
	"testing"
)

// TestCheckDaemonFresh pins PC-4: the row compares the DAEMON's stamp to
// THIS binary's — pass on the same revision, fail on another, skip with the
// reason for a v1 daemon (no field), an unstamped local binary, or a
// daemon that is down. HEAD is never consulted: no git runs here.
func TestCheckDaemonFresh(t *testing.T) {
	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	hc := &http.Client{}

	t.Run("same revision passes", func(t *testing.T) {
		ts := statuszServer(t, "6c6f4377b53aad47cf6ab86224721df8592cadf0", false, "w")
		defer ts.Close()
		doctorBinaryRevision = func() (string, bool, bool) { return "6c6f4377b53aad47cf6ab86224721df8592cadf0", false, true }
		c := checkDaemonFresh(hc, ts.URL, true)
		if c.Status != doctorPass {
			t.Fatalf("status = %s (%s), want pass", c.Status, c.Reason)
		}
	})
	t.Run("different revision fails and names both", func(t *testing.T) {
		ts := statuszServer(t, "6c6f4377b53aad47cf6ab86224721df8592cadf0", true, "w")
		defer ts.Close()
		doctorBinaryRevision = func() (string, bool, bool) { return "3f0c3b1deadbeef00000000000000000000000000", false, true }
		c := checkDaemonFresh(hc, ts.URL, true)
		if c.Status != doctorFail {
			t.Fatalf("status = %s (%s), want fail", c.Status, c.Reason)
		}
		if !strings.Contains(c.Reason, "6c6f4377b53a-dirty") || !strings.Contains(c.Reason, "3f0c3b1deadb") {
			t.Errorf("reason must name both stamps: %q", c.Reason)
		}
		if !strings.Contains(c.Remedy, "task deploy") || strings.Contains(c.Remedy, "cp bin/fathom") {
			t.Errorf("remedy must be task deploy, never a raw cp: %q", c.Remedy)
		}
	})
	t.Run("v1 daemon skips with the reason", func(t *testing.T) {
		ts := statuszServer(t, "", false, "w")
		defer ts.Close()
		doctorBinaryRevision = func() (string, bool, bool) { return "3f0c3b1deadbeef", false, true }
		c := checkDaemonFresh(hc, ts.URL, true)
		if c.Status != doctorSkipped {
			t.Fatalf("status = %s (%s), want skipped", c.Status, c.Reason)
		}
		if !strings.Contains(c.Reason, "binary_commit") {
			t.Errorf("skip reason must name the missing field: %q", c.Reason)
		}
	})
	t.Run("unstamped local binary skips", func(t *testing.T) {
		ts := statuszServer(t, "6c6f4377b53aad47cf6ab86224721df8592cadf0", false, "w")
		defer ts.Close()
		doctorBinaryRevision = func() (string, bool, bool) { return "", false, false }
		if c := checkDaemonFresh(hc, ts.URL, true); c.Status != doctorSkipped {
			t.Fatalf("status = %s (%s), want skipped", c.Status, c.Reason)
		}
	})
	t.Run("daemon down skips", func(t *testing.T) {
		if c := checkDaemonFresh(hc, "http://127.0.0.1:1", false); c.Status != doctorSkipped {
			t.Fatalf("status = %s (%s), want skipped", c.Status, c.Reason)
		}
	})
}

// TestDoctorRegistersDaemonFresh: the row is in the fixed check order,
// right after binary_fresh, so the two freshness answers read together.
func TestDoctorRegistersDaemonFresh(t *testing.T) {
	checks := runDoctorChecks(t.TempDir(), "", "claude", "http://127.0.0.1:1")
	prev := ""
	for _, c := range checks {
		if c.Name == "daemon_fresh" {
			if prev != "binary_fresh" {
				t.Errorf("daemon_fresh follows %q, want binary_fresh", prev)
			}
			return
		}
		prev = c.Name
	}
	t.Fatal("daemon_fresh row not registered")
}

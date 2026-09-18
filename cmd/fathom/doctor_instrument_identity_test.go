package main

// doctor_instrument_identity_test.go — J5's doctor-row proof (app-kit O25).

import (
	"strings"
	"testing"
)

func TestCheckInstrumentIdentity_NoServiceNamedSkips(t *testing.T) {
	daemon := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"app-kit"}})
	defer daemon.Close()
	got := checkInstrumentIdentity(daemon.Client(), daemon.URL, "")
	if got.Name != "instrument_identity" {
		t.Fatalf("row name = %q", got.Name)
	}
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want skipped (no --service named)", got.Status)
	}
}

func TestCheckInstrumentIdentity_ResolvedServicePasses(t *testing.T) {
	svcRoot := t.TempDir()
	daemon := fakeDoctorDaemon(t, fakeDaemonCfg{
		services:     []string{"app-kit"},
		servicePaths: map[string]string{"app-kit": svcRoot},
	})
	defer daemon.Close()
	got := checkInstrumentIdentity(daemon.Client(), daemon.URL, "app-kit")
	if got.Status != doctorPass {
		t.Fatalf("status = %v, want pass: %s", got.Status, got.Reason)
	}
	want := callLogPathFor(svcRoot)
	if !strings.Contains(got.Reason, want) || !strings.Contains(got.Reason, "app-kit") {
		t.Errorf("reason = %q, want it to name the resolved path %q and the service", got.Reason, want)
	}
}

func TestCheckInstrumentIdentity_UnknownServiceWarns(t *testing.T) {
	daemon := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"other-svc"}})
	defer daemon.Close()
	got := checkInstrumentIdentity(daemon.Client(), daemon.URL, "app-kit")
	if got.Status != doctorWarn {
		t.Fatalf("status = %v, want warn (the daemon doesn't know app-kit, only the CWD-relative fallback agrees): %s", got.Status, got.Reason)
	}
}

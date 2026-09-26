package tenancy

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 26, 21, 7, 0, 0, time.UTC)

func present(raw string) SwitchFile { return SwitchFile{Present: true, Raw: []byte(raw)} }

// RT-6 (§9): `daemon stop --for 2h` writes an `until`; while it is in the
// future a fresh process exits 75 (so a schema-2 plist retries it after the
// throttle), and once it passes the daemon starts normally. An empty
// disabled.json (the "no binary available" `touch`) disables with the reason
// unknown; paused.json moves the daemon to safe mode and its removal moves it
// back.
func TestTimedPauseExits75UntilExpiry(t *testing.T) {
	until := t0.Add(2 * time.Hour)
	timed := MarshalSwitch(SwitchRecord{By: "fathom daemon stop", Reason: "load test", Since: t0, Until: until})

	v := EvaluateKillSwitch(KillSwitchInput{Disabled: SwitchFile{Present: true, Raw: timed}, Now: t0.Add(time.Minute)})
	if v.Action != ActionExitRetry || v.ExitCode != ExitRetryLater {
		t.Fatalf("a timed stop before its expiry must exit %d (retry later), got %+v", ExitRetryLater, v)
	}
	if !strings.Contains(v.Message, "load test") || !strings.Contains(v.Message, "until") {
		t.Fatalf("the message must disclose the reason and the expiry: %q", v.Message)
	}

	v = EvaluateKillSwitch(KillSwitchInput{Disabled: SwitchFile{Present: true, Raw: timed}, Now: until.Add(time.Second)})
	if v.Action != ActionRun || !v.Expired {
		t.Fatalf("after the expiry the daemon must start normally and say the stop expired: %+v", v)
	}

	// An empty file (touch) disables — reason unknown, exit 0 so launchd
	// stays down under a schema-2 plist.
	v = EvaluateKillSwitch(KillSwitchInput{Disabled: present(""), Now: t0})
	if v.Action != ActionExitStay || v.ExitCode != ExitStayDown {
		t.Fatalf("an empty disabled.json must disable: %+v", v)
	}
	if !strings.Contains(v.Message, "reason unknown") {
		t.Fatalf("an empty switch reads 'disabled, reason unknown': %q", v.Message)
	}

	// An unparseable file still disables: a switch the operator wrote is
	// never ignored because it is malformed.
	v = EvaluateKillSwitch(KillSwitchInput{Disabled: present("{not json"), Now: t0})
	if v.Action != ActionExitStay {
		t.Fatalf("a malformed disabled.json must still disable: %+v", v)
	}

	// An untimed stop stays down.
	v = EvaluateKillSwitch(KillSwitchInput{Disabled: SwitchFile{Present: true, Raw: MarshalSwitch(SwitchRecord{By: "breaker", Reason: "breaker", Since: t0})}, Now: t0.Add(48 * time.Hour)})
	if v.Action != ActionExitStay || v.ExitCode != 0 {
		t.Fatalf("an untimed stop never expires: %+v", v)
	}

	// pause → safe mode; resume (file removed) → run.
	v = EvaluateKillSwitch(KillSwitchInput{Paused: SwitchFile{Present: true, Raw: MarshalSwitch(SwitchRecord{By: "fathom daemon pause", Since: t0})}, Now: t0})
	if v.Action != ActionSafeMode || v.ExitCode != 0 {
		t.Fatalf("paused.json must put the daemon in safe mode, not stop it: %+v", v)
	}
	v = EvaluateKillSwitch(KillSwitchInput{Now: t0})
	if v.Action != ActionRun || v.Expired {
		t.Fatalf("no switch files: run: %+v", v)
	}

	// disabled wins over paused.
	v = EvaluateKillSwitch(KillSwitchInput{Disabled: present(""), Paused: present(""), Now: t0})
	if v.Action != ActionExitStay {
		t.Fatalf("disabled.json outranks paused.json: %+v", v)
	}
}

// The switch round-trips, and a record without `until` does not serialize a
// zero time an older reader would misread as an expiry in year 1.
func TestSwitchRecordRoundTrip(t *testing.T) {
	raw := MarshalSwitch(SwitchRecord{By: "x", Reason: "y", Since: t0})
	if strings.Contains(string(raw), "until") {
		t.Fatalf("an untimed record must not carry until: %s", raw)
	}
	got, err := ParseSwitch(raw)
	if err != nil || got.By != "x" || !got.Since.Equal(t0) || !got.Until.IsZero() {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

package heartbeat_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

func fixedClock(t0 time.Time, step time.Duration) func() time.Time {
	i := 0
	return func() time.Time {
		t := t0.Add(time.Duration(i) * step)
		i++
		return t
	}
}

var t0 = time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)

func TestLog_RoundTripByteStable(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	for _, buf := range []*bytes.Buffer{&buf1, &buf2} {
		l, err := heartbeat.NewLog(buf, fixedClock(t0, time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Beat("edit-hook", "seen", map[string]string{"service": "fathom"}); err != nil {
			t.Fatal(err)
		}
		if err := l.Beat("edit-hook", "emitted", nil); err != nil {
			t.Fatal(err)
		}
	}
	if buf1.String() != buf2.String() {
		t.Fatalf("encoding is not byte-stable:\n%q\n%q", buf1.String(), buf2.String())
	}

	beats, err := heartbeat.Load(&buf1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(beats) != 2 || beats[0].Channel != "edit-hook" || beats[0].Kind != "seen" ||
		!beats[0].At.Equal(t0) || beats[0].Meta["service"] != "fathom" {
		t.Fatalf("round-trip loss: %+v", beats)
	}
}

func TestLog_RejectsNilClockAndEmptyChannel(t *testing.T) {
	if _, err := heartbeat.NewLog(&bytes.Buffer{}, nil); err == nil {
		t.Error("nil clock must be rejected — no wall-time defaults")
	}
	l, err := heartbeat.NewLog(&bytes.Buffer{}, fixedClock(t0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Beat("", "seen", nil); err == nil {
		t.Error("empty channel must be rejected")
	}
}

func TestLoad_AcceptPartial(t *testing.T) {
	good := `{"channel":"a","at":"2026-07-16T08:00:00Z"}`
	input := good + "\n{torn line\n" + good + "\n"
	if _, err := heartbeat.Load(strings.NewReader(input), false); err == nil {
		t.Error("strict load must fail on a torn line")
	}
	beats, err := heartbeat.Load(strings.NewReader(input), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(beats) != 2 {
		t.Fatalf("accept-partial kept %d beats, want 2", len(beats))
	}
}

func TestRollup_Deterministic(t *testing.T) {
	beats := []heartbeat.Beat{
		{Channel: "b", Kind: "x", At: t0},
		{Channel: "a", Kind: "y", At: t0.Add(time.Hour)},
		{Channel: "a", Kind: "y", At: t0.Add(2 * time.Hour)},
		{Channel: "a", Kind: "y", At: t0.Add(25 * time.Hour)}, // next day
	}
	got := heartbeat.Rollup(beats)
	want := []heartbeat.DayCount{
		{Day: "2026-07-16", Channel: "a", Kind: "y", N: 2},
		{Day: "2026-07-16", Channel: "b", Kind: "x", N: 1},
		{Day: "2026-07-17", Channel: "a", Kind: "y", N: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Rollup = %+v, want %+v", got, want)
	}
	if again := heartbeat.Rollup(beats); !reflect.DeepEqual(got, again) {
		t.Error("Rollup is not deterministic")
	}
}

func TestCheckLiveness(t *testing.T) {
	ref := t0.Add(24 * time.Hour)
	exps := []heartbeat.Expectation{
		{IfChannel: "edit-hook.seen", ThenChannel: "edit-hook.emitted", Within: 24 * time.Hour},
	}

	// Activity with silent effect channel → alarm (the four-dark-days case).
	beats := []heartbeat.Beat{{Channel: "edit-hook", Kind: "seen", At: t0.Add(time.Hour)}}
	alarms := heartbeat.CheckLiveness(beats, exps, ref)
	if len(alarms) != 1 {
		t.Fatalf("want 1 alarm, got %d", len(alarms))
	}
	if alarms[0].LastThen != (time.Time{}) || !strings.Contains(alarms[0].Detail, "edit-hook.emitted") {
		t.Errorf("alarm shape wrong: %+v", alarms[0])
	}

	// Both beat → no alarm.
	both := append(beats, heartbeat.Beat{Channel: "edit-hook", Kind: "emitted", At: t0.Add(2 * time.Hour)})
	if a := heartbeat.CheckLiveness(both, exps, ref); len(a) != 0 {
		t.Errorf("both-beat must not alarm: %+v", a)
	}

	// Neither beat (no activity) → nothing owed, no alarm.
	if a := heartbeat.CheckLiveness(nil, exps, ref); len(a) != 0 {
		t.Errorf("no-activity must not alarm: %+v", a)
	}

	// Activity outside the window → no alarm.
	old := []heartbeat.Beat{{Channel: "edit-hook", Kind: "seen", At: t0.Add(-48 * time.Hour)}}
	if a := heartbeat.CheckLiveness(old, exps, ref); len(a) != 0 {
		t.Errorf("stale activity must not alarm: %+v", a)
	}

	// Determinism.
	a1 := heartbeat.CheckLiveness(beats, exps, ref)
	a2 := heartbeat.CheckLiveness(beats, exps, ref)
	if !reflect.DeepEqual(a1, a2) {
		t.Error("CheckLiveness is not deterministic")
	}
}

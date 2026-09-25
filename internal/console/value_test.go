package console_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestValue_TruthStatesRoundTripJSON pins OC-1: every truth state survives a
// JSON round trip with its reason, source, denominator and time; absent and
// error never carry a v on the wire; state and reason are always present;
// and a writer can never smuggle a number into an absence.
func TestValue_TruthStatesRoundTripJSON(t *testing.T) {
	asOf := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		in     console.Value[int]
		wantV  bool
		reason bool
	}{
		{"present", console.Present(42, "call log").WithDenominator("of 3 repos").WithAsOf(asOf), true, false},
		{"zero", console.Measured(0, "call log"), true, false},
		{"absent", console.Absent[int]("no call log at /x — never zero").WithSource("call log"), false, true},
		{"stale", console.Stale(7, "older than 14 days", asOf), true, true},
		{"partial", console.Partial(114, "7 of 18 repos reporting; lower bound"), true, true},
		{"error", console.Errored[int]("open: operation not permitted"), false, true},
	}
	seen := map[console.State]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var generic map[string]any
			if err := json.Unmarshal(raw, &generic); err != nil {
				t.Fatal(err)
			}
			if _, ok := generic["state"]; !ok {
				t.Errorf("%s: JSON lacks state: %s", tc.name, raw)
			}
			if _, ok := generic["reason"]; !ok {
				t.Errorf("%s: JSON lacks reason (it is always carried): %s", tc.name, raw)
			}
			if _, has := generic["v"]; has != tc.wantV {
				t.Errorf("%s: v present = %v, want %v: %s", tc.name, has, tc.wantV, raw)
			}
			var back console.Value[int]
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatalf("unmarshal %s: %v", raw, err)
			}
			if back != tc.in {
				t.Errorf("%s: round trip = %+v, want %+v", tc.name, back, tc.in)
			}
			if tc.reason && back.Reason == "" {
				t.Errorf("%s: a %s value must keep its reason", tc.name, back.State)
			}
			seen[back.State] = true
		})
	}
	for _, s := range []console.State{console.StatePresent, console.StateZero, console.StateAbsent, console.StateStale, console.StatePartial, console.StateError} {
		if !seen[s] {
			t.Errorf("state %q not exercised", s)
		}
	}

	// A measured zero is a number (zero state), never an absence.
	if z := console.Measured(0, "ledger"); z.State != console.StateZero || !z.Known() {
		t.Errorf("Measured(0) = %+v, want a known zero", z)
	}
	if a := console.Absent[int]("no ledger"); a.Known() {
		t.Error("an absent value must not report Known")
	}

	// Refusals: a v on an absence, an unknown state, a reasonless absence.
	for _, bad := range []string{
		`{"v":0,"state":"absent","reason":"no log"}`,
		`{"v":3,"state":"error","reason":"unreadable"}`,
		`{"v":1,"state":"guessed","reason":""}`,
		`{"state":"absent","reason":""}`,
		`{"v":2,"state":"partial","reason":""}`,
	} {
		var v console.Value[int]
		if err := json.Unmarshal([]byte(bad), &v); err == nil {
			t.Errorf("unmarshal %s: want an error, got %+v", bad, v)
		}
	}
	if _, err := json.Marshal(console.Value[int]{State: console.StateAbsent}); err == nil {
		t.Error("marshal of a reasonless absence must fail")
	}

	// Non-int payloads carry the same contract.
	s := console.Present("fathom", "registry")
	raw, _ := json.Marshal(s)
	if !strings.Contains(string(raw), `"v":"fathom"`) {
		t.Errorf("string value lost its v: %s", raw)
	}
}

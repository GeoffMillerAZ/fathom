package mcp

import (
	"bytes"
	"testing"
	"time"
)

// TestCallLogger_SurfaceTraffic_CountsAndLastTS is A1-1's instrument-repair
// unit (docs/plans/activation.md): server_stats needs "is anything flowing
// on /rpc/agent?" answerable from the daemon without tailing the JSONL call
// log, so CallLogger itself accumulates process-lifetime per-surface call
// counts and last-call timestamps as it logs — the same site that already
// sees every request's resolved surface.
func TestCallLogger_SurfaceTraffic_CountsAndLastTS(t *testing.T) {
	var buf bytes.Buffer
	l := NewCallLogger(&buf)

	// Zero calls on a WIRED logger is a non-nil empty map — "instrument
	// live, zero traffic" — never nil, which is reserved for "no logger at
	// all" (bounce finding 3 / K3's existence check).
	if got := l.SurfaceTraffic(); got == nil || len(got) != 0 {
		t.Fatalf("SurfaceTraffic() before any calls = %v, want non-nil empty map", got)
	}

	l.log("find_symbol", "fathom", "shared", "agent", "", "", false, time.Millisecond, nil)
	l.log("outline", "fathom", "shared", "agent", "", "", false, time.Millisecond, nil)
	l.log("secret_scan", "fathom", "shared", "", "", "", false, time.Millisecond, nil)

	traffic := l.SurfaceTraffic()
	if len(traffic) != 2 {
		t.Fatalf("SurfaceTraffic() = %v, want 2 distinct surfaces", traffic)
	}
	agent, ok := traffic["agent"]
	if !ok {
		t.Fatalf("SurfaceTraffic() missing %q: %v", "agent", traffic)
	}
	if agent.Calls != 2 {
		t.Errorf("agent surface calls = %d, want 2", agent.Calls)
	}
	if agent.LastTS == "" {
		t.Error("agent surface last_ts is empty, want a timestamp")
	}
	bare, ok := traffic[""]
	if !ok {
		t.Fatalf("SurfaceTraffic() missing the bare/unscoped surface: %v", traffic)
	}
	if bare.Calls != 1 {
		t.Errorf("bare surface calls = %d, want 1", bare.Calls)
	}

	// A later call on the same surface bumps LastTS forward (never backward)
	// and increments Calls again.
	before := traffic["agent"].LastTS
	time.Sleep(2 * time.Millisecond)
	l.log("repo_map", "fathom", "shared", "agent", "", "", false, time.Millisecond, nil)
	after := l.SurfaceTraffic()["agent"]
	if after.Calls != 3 {
		t.Errorf("agent surface calls after 3rd call = %d, want 3", after.Calls)
	}
	if after.LastTS == before {
		t.Errorf("agent surface last_ts did not advance: still %q", after.LastTS)
	}
}

// TestCallLogger_SurfaceTraffic_NilLoggerIsEmpty: a nil *CallLogger (call
// logging disabled) must not panic and reports no traffic — mirrors log's
// existing nil-receiver no-op contract.
func TestCallLogger_SurfaceTraffic_NilLoggerIsEmpty(t *testing.T) {
	var l *CallLogger
	if got := l.SurfaceTraffic(); got != nil {
		t.Errorf("nil CallLogger SurfaceTraffic() = %v, want nil", got)
	}
}

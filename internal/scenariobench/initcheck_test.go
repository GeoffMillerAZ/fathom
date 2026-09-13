package scenariobench

import "testing"

func TestCheckInitJSON_CleanExitZero(t *testing.T) {
	out := `{"service":"svc","root":"/tmp/x","exit_code":0,"writes":[{"path":".claude/settings.json","action":"created"}],"blocked":[],"manual_steps":[]}`
	if err := checkInitJSON(out); err != nil {
		t.Fatalf("checkInitJSON: %v", err)
	}
}

func TestCheckInitJSON_DaemonUnreachableExitTwoAccepted(t *testing.T) {
	out := `{"exit_code":2,"writes":[],"blocked":[],"manual_steps":[` +
		`{"text":"fathom daemon at http://127.0.0.1:9 is unreachable — wiring below is complete, but the index is unconfirmed."},` +
		`{"text":"start the daemon: fathom serve-http"}` +
		`]}`
	if err := checkInitJSON(out); err != nil {
		t.Fatalf("checkInitJSON: %v (the daemon-unreachable operational exit must be accepted)", err)
	}
}

func TestCheckInitJSON_NoSymbolsExitTwoIsAFailure(t *testing.T) {
	// A DIFFERENT exit-2 shape ("reachable but no symbols yet") is not the
	// daemon-unreachable case wireScratchDaemon accepts, since it already
	// waited for the daemon to answer before running init — a real signal
	// here, not init's documented fail-open.
	out := `{"exit_code":2,"writes":[],"blocked":[],"manual_steps":[` +
		`{"text":"ingest ran but service \"svc\" has no symbols in the index yet — check the daemon logs"}` +
		`]}`
	if err := checkInitJSON(out); err == nil {
		t.Fatal("checkInitJSON: want an error for a non-daemon-unreachable exit 2, got nil")
	}
}

func TestCheckInitJSON_ExitOneIsAFailure(t *testing.T) {
	out := `{"exit_code":1,"writes":[],"blocked":[],"manual_steps":[]}`
	if err := checkInitJSON(out); err == nil {
		t.Fatal("checkInitJSON: want an error for exit_code 1, got nil")
	}
}

func TestCheckInitJSON_NonEmptyBlockedIsAFailureEvenAtExitZero(t *testing.T) {
	// Defensive: blocked should never be non-empty at exit 0 (cmdInit
	// returns 1 whenever it is), but checkInitJSON must not rely on that
	// invariant holding forever.
	out := `{"exit_code":0,"writes":[],"blocked":[{"path":".claude/settings.json","action":"blocked","detail":"drifted"}],"manual_steps":[]}`
	if err := checkInitJSON(out); err == nil {
		t.Fatal("checkInitJSON: want an error for a non-empty blocked list, got nil")
	}
}

func TestCheckInitJSON_WriteActionErrorIsAFailure(t *testing.T) {
	out := `{"exit_code":0,"writes":[{"path":".claude/settings.json","action":"error","detail":"boom"}],"blocked":[],"manual_steps":[]}`
	if err := checkInitJSON(out); err == nil {
		t.Fatal("checkInitJSON: want an error for a write with action \"error\", got nil")
	}
}

func TestCheckInitJSON_NoJSONObjectIsAFailure(t *testing.T) {
	if err := checkInitJSON("plain text, no JSON here"); err == nil {
		t.Fatal("checkInitJSON: want an error when no JSON object is present, got nil")
	}
}

func TestCheckInitJSON_TolerantOfTrailingStderrAfterTheJSONBlob(t *testing.T) {
	// renderInitJSON (cmd/fathom/init_json.go) writes the JSON envelope to
	// stdout, then re-emits cmdInit's captured stderr separately — a
	// CombinedOutput() capture of the real subprocess concatenates both,
	// so trailing bytes after the JSON object are the normal shape, not an
	// edge case.
	out := `{"exit_code":0,"writes":[],"blocked":[],"manual_steps":[]}` + "\nsome stray stderr line\n"
	if err := checkInitJSON(out); err != nil {
		t.Fatalf("checkInitJSON: %v", err)
	}
}

package main

// armmeasurement_test.go — the OFF arm silences ASSISTANCE, never
// MEASUREMENT (vision 14 §2).
//
// These tests exist because the two were conflated: every hook checked one
// merged gate before doing any work, so an active OFF arm silenced the
// session ledger and the arm's own session decrement along with the
// assistance. The arm burned its budget collecting nothing, and the
// counterfactual pilot could never leave "collecting" — the instrument was
// self-defeating in exactly the way it was built to prevent. Measured on
// 2026-08-08: an arm set at 07:11Z produced zero ledger rows across a full
// coding day, with sessions_remaining still reading 10 of 10.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// pipeStdin feeds a hook its JSON envelope on os.Stdin and returns the
// restore func. Hooks read os.Stdin directly by contract, so there is no
// injection seam to use instead.
func pipeStdin(t *testing.T, body string) func() {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(body))
		_ = w.Close()
	}()
	return func() { os.Stdin = old }
}

// armOffEnv points the arm and ledger at temp files and arms an OFF window
// with the given session budget.
func armOffEnv(t *testing.T, sessions int) (armPath, ledgerPath string) {
	t.Helper()
	dir := t.TempDir()
	armPath = filepath.Join(dir, "arm.json")
	ledgerPath = filepath.Join(dir, "ledger.jsonl")
	t.Setenv("FATHOM_ARM_STATE", armPath)
	t.Setenv("FATHOM_SESSION_LEDGER", ledgerPath)
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(dir, "narrate.json"))
	t.Setenv("FATHOM_CHANNELS", "")
	resetObsCache()

	st := map[string]any{
		"schema_version":     1,
		"fathom_version":     "test",
		"arm":                "off",
		"set_at":             time.Now().UTC().Format(time.RFC3339),
		"sessions_remaining": sessions,
	}
	raw, _ := json.Marshal(st)
	if err := os.WriteFile(armPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if !armOffActive() {
		t.Fatal("test setup failed: the OFF arm is not active")
	}
	return armPath, ledgerPath
}

// TestArmSilencesAssistanceNotMeasurement is the contract in one place.
func TestArmSilencesAssistanceNotMeasurement(t *testing.T) {
	t.Run("the two gates are not the same gate", func(t *testing.T) {
		armOffEnv(t, 3)
		if fathomChannelsDisabled() {
			t.Error("an OFF arm must NOT trip the total-silence gate — that gate is the env kill switch alone")
		}
		if !assistiveChannelsDisabled() {
			t.Error("an OFF arm must trip the assistive gate — the session has to be genuinely unaided")
		}
	})

	t.Run("the env kill switch still silences everything", func(t *testing.T) {
		armOffEnv(t, 3)
		t.Setenv("FATHOM_CHANNELS", "off")
		if !fathomChannelsDisabled() || !assistiveChannelsDisabled() {
			t.Error("FATHOM_CHANNELS=off keeps its historical total-silence semantics")
		}
	})
}

// TestSessionHookAnnouncesAndDecrementsUnderArm pins the two things the
// merged gate was skipping: the loud announcement (the O15 lesson — a
// silent disablement is the failure mode) and the budget decrement without
// which an OFF arm never ends.
func TestSessionHookAnnouncesAndDecrementsUnderArm(t *testing.T) {
	armPath, _ := armOffEnv(t, 3)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	var out, errb bytes.Buffer
	if code := cmdSessionHook([]string{"--url", srv.URL, "--dir", t.TempDir()}, strings.NewReader("{}"), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "OFF arm active") {
		t.Errorf("an active OFF arm must announce itself at session start:\n%s", got)
	}
	if !strings.Contains(got, "fathom arm on") {
		t.Errorf("the announcement must carry the undo:\n%s", got)
	}

	raw, err := os.ReadFile(armPath)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Remaining *int `json:"sessions_remaining"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.Remaining == nil || *st.Remaining != 2 {
		t.Fatalf("sessions_remaining = %v, want 2 — an arm that never decrements never ends", st.Remaining)
	}
}

// TestStopHookRecordsLedgerUnderArmAndSkipsVerify pins the other half: the
// row that IS the counterfactual data gets written, and the assistive
// verify pass — whose daemon calls would also pollute the call log with
// fathom activity during a supposedly unaided session — does not run.
func TestStopHookRecordsLedgerUnderArmAndSkipsVerify(t *testing.T) {
	_, ledgerPath := armOffEnv(t, 3)

	var daemonHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		daemonHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	repo := t.TempDir()
	hook := map[string]any{
		"stop_hook_active": false,
		"cwd":              repo,
		"session_id":       "arm-off-session",
		"transcript_path":  filepath.Join(t.TempDir(), "nope.jsonl"),
	}
	raw, _ := json.Marshal(hook)

	defer pipeStdin(t, string(raw))()

	var out, errb bytes.Buffer
	if code := cmdStopHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("a stop hook must never block session end: exit %d, %s", code, errb.String())
	}

	if n := daemonHits.Load(); n != 0 {
		t.Errorf("the stop hook made %d daemon call(s) under an OFF arm — an unaided session must not generate fathom traffic", n)
	}

	data, rerr := os.ReadFile(ledgerPath)
	if rerr != nil {
		t.Fatalf("the OFF arm must still write a ledger row — that row IS the counterfactual data: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("want exactly one ledger row, got %d:\n%s", len(lines), string(data))
	}
	var row struct {
		Arm           string `json:"arm"`
		SessionID     string `json:"session_id"`
		VerifyCatches *int   `json:"verify_catches"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("bad ledger row: %v\n%s", err, lines[0])
	}
	if row.Arm != "off" {
		t.Errorf("arm = %q, want %q — paired comparison depends on every row naming the arm it ran under", row.Arm, "off")
	}
	if row.VerifyCatches != nil {
		t.Errorf("verify_catches = %v, want nil — verify did not run, so its result is UNKNOWN, and a fabricated zero would read as \"the gate found nothing\" (law 3)", *row.VerifyCatches)
	}
}

// TestEditHookIsFullySilentUnderArm pins the assistive half: the treatment
// really is withheld, so the control group is a control group.
func TestEditHookIsFullySilentUnderArm(t *testing.T) {
	armOffEnv(t, 3)

	var daemonHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		daemonHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// cmdEditHook reads os.Stdin directly (hook contract), so feed it there.
	restore := pipeStdin(t, `{"tool_name":"Edit","tool_input":{"file_path":"main.go"},"cwd":"`+t.TempDir()+`"}`)
	defer restore()

	var out, errb bytes.Buffer
	if code := cmdEditHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("the edit hook must inject nothing under an OFF arm:\n%s", out.String())
	}
	if n := daemonHits.Load(); n != 0 {
		t.Errorf("the edit hook made %d daemon call(s) under an OFF arm", n)
	}
}

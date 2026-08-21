package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestOffArmStillCountsSeenBeats pins IT-3: the OFF arm must not delete the
// counting-only baseline. armmeasurement_test.go's own
// TestEditHookIsFullySilentUnderArm already pins the assistive half (zero
// daemon calls, zero stdout under the arm) — this test pins the other half,
// that the counting-only beat (seen / observer counts) the arm needs to
// prove it ran at all still fires, stamped "arm":"off", exactly like the
// session ledger row TestStopHookRecordsLedgerUnderArmAndSkipsVerify pins
// for the Stop hook.
func TestOffArmStillCountsSeenBeats(t *testing.T) {
	_, _ = armOffEnv(t, 3)
	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)

	var daemonHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		daemonHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	restore := pipeStdin(t, `{"tool_name":"Edit","tool_input":{"file_path":"main.go"},"cwd":"`+t.TempDir()+`"}`)
	defer restore()

	var out, errb bytes.Buffer
	if code := cmdEditHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("the edit hook must inject nothing under an OFF arm: %q", out.String())
	}
	if n := daemonHits.Load(); n != 0 {
		t.Errorf("the edit hook made %d daemon call(s) under an OFF arm", n)
	}

	beats, err := readRetroBeats(beatLog)
	if err != nil {
		t.Fatalf("reading beat log: %v", err)
	}
	if len(beats) == 0 {
		t.Fatal("the OFF arm deleted the counting-only baseline — no beat was recorded at all")
	}
	found := false
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == "seen" && b.Meta["arm"] == "off" {
			found = true
		}
	}
	if !found {
		t.Errorf("no edit-hook 'seen' beat carries arm:off; beats = %+v", beats)
	}
}

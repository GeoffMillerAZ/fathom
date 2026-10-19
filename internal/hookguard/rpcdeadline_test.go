package hookguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestDeadlineHeaderIsTheShorterOfHookAndCallCap is RT-190 (§11.1 amended
// 2026-10-19, I53): a hook RPC's X-Fathom-Deadline is the shorter of the
// hook's remaining deadline and the request context's own deadline — the
// one http.Client.Timeout sets — so the daemon sizes its answer to the
// call's cap (the pre-edit hook's 500 ms blast_radius), not the hook's
// remainder.
func TestDeadlineHeaderIsTheShorterOfHookAndCallCap(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get(DeadlineHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	header := func(t *testing.T, hook Deadline, clientCap time.Duration) int64 {
		t.Helper()
		g := &Guard{HealthDir: filepath.Join(t.TempDir(), "h"), DaemonDir: t.TempDir(), Deadline: hook}
		hc := &http.Client{Timeout: clientCap, Transport: &Transport{Base: http.DefaultTransport, Active: func() *Guard { return g }}}
		resp, err := hc.Get(srv.URL + "/rpc")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		v := <-got
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("%s = %q, want whole milliseconds", DeadlineHeader, v)
		}
		return ms
	}

	// A 4 s hook remainder and a 500 ms call cap: the cap.
	if ms := header(t, HookDeadline(time.Now(), 5), 500*time.Millisecond); ms <= 0 || ms > 500 {
		t.Errorf("4 s hook, 500 ms client timeout: header %d ms, want (0, 500]", ms)
	}
	// A 4 s hook remainder and a 5 s call cap: the hook's remainder.
	if ms := header(t, HookDeadline(time.Now(), 5), 5*time.Second); ms <= 3000 || ms > 4000 {
		t.Errorf("4 s hook, 5 s client timeout: header %d ms, want (3000, 4000]", ms)
	}
	// No call cap: the hook's remainder, as before.
	if ms := header(t, HookDeadline(time.Now(), 5), 0); ms <= 3000 || ms > 4000 {
		t.Errorf("4 s hook, no client timeout: header %d ms, want (3000, 4000]", ms)
	}

	// The rule itself: the shorter of the two, never negative; neither is
	// no header.
	hook := HookDeadline(time.Now(), 10) // 8 s
	short, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if v := rpcDeadlineHeader(short, hook); v == "" {
		t.Error("a hook deadline and a call cap: no header")
	} else if ms, _ := strconv.ParseInt(v, 10, 64); ms > 200 {
		t.Errorf("8 s hook, 200 ms cap: %s ms", v)
	}
	if v := rpcDeadlineHeader(context.Background(), Deadline{}); v != "" {
		t.Errorf("no hook deadline and no call cap: header %q, want none", v)
	}
	if v := rpcDeadlineHeader(short, Deadline{}); v == "" {
		t.Error("a call cap alone: no header")
	}
	spent, scancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer scancel()
	if v := rpcDeadlineHeader(spent, hook); v != "0" {
		t.Errorf("a spent call cap: header %q, want 0", v)
	}
}

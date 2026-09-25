package main

// console_truth_test.go — the console's truth states under failure: a
// source that exists and cannot be read is an error that names what failed,
// never an absence, a "no file" label or a smaller count (ADR-045's six
// states, design §2, invariant I38).

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// An unreadable call log reaches every page as state "error" naming the
// read failure — not "absent" ("not measured"), which is what a missing or
// unconfigured log is. The model's CallLogNote says the same words, but the
// facts' read error must win.
func TestConsoleMetrics_UnreadableCallLogIsError(t *testing.T) {
	resolved := console.ResolvedCallLog{Path: "/logs/mcp-calls.jsonl", Rule: console.RuleDaemon, Note: "the daemon's configured call log"}
	readErr := &fs.PathError{Op: "open", Path: resolved.Path, Err: errors.New("permission denied")}
	m := observatoryModel{
		callFacts:   callLogFactsFor(resolved, nil, readErr),
		CallLogNote: "call log unreadable: " + readErr.Error() + " — traffic figures are not measured, never zero",
	}
	v := consoleModelMetrics(m)[console.MetricCalls]
	if v.State != console.StateError || !strings.Contains(v.Reason, "permission denied") {
		t.Fatalf("calls.total = %+v, want state error naming the read failure", v)
	}
	if s := valueStat("tool calls (log)", v); s.Value != "unreadable" || s.ValueRole != "danger" {
		t.Errorf("the tile must draw the error distinctly, got %+v", s)
	}
	// A missing log and a log-off daemon stay absent: nothing to read.
	m.callFacts = callLogFactsFor(resolved, nil, fs.ErrNotExist)
	m.CallLogNote = "no call log at " + resolved.Path
	if v := consoleModelMetrics(m)[console.MetricCalls]; v.State != console.StateAbsent {
		t.Errorf("a missing log = %+v, want absent", v)
	}
	m.callFacts = callLogFactsFor(console.ResolvedCallLog{Rule: console.RuleNone, Note: "no call log is configured anywhere"}, nil, nil)
	m.CallLogNote = "no call log is configured anywhere"
	if v := consoleModelMetrics(m)[console.MetricCalls]; v.State != console.StateAbsent {
		t.Errorf("logging off = %+v, want absent", v)
	}
	// A snapshot model (no facts) with a named absence keeps it.
	snap := observatoryModel{CallLogNote: "synthetic call log unavailable"}
	if v := consoleModelMetrics(snap)[console.MetricCalls]; v.State != console.StateAbsent || v.Reason != "synthetic call log unavailable" {
		t.Errorf("snapshot absence = %+v", v)
	}
}

// A daemon started with `serve-http --call-log ""` (logging off) records
// call_log "". Its pages resolve to rule none; a verb talking to it must
// too (P10), instead of discarding the record and reading a stale legacy
// log the daemon never wrote. The record also still counts as a previous
// daemon, so the next start publishes "restart", not "start".
func TestResolveConsoleCallLog_LoggingOffRecordIsNone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Unsetenv("FATHOM_CALL_LOG")
	t.Setenv("FATHOM_HTTP", "http://127.0.0.1:7598")
	t.Chdir(t.TempDir()) // not a git repo: the legacy default is under HOME
	legacy := defaultCallLogPath()
	mustWrite(t, legacy, `{"ts":"`+time.Now().UTC().Format(time.RFC3339)+`","tool":"find_symbol","service":"x","surface":"agent"}`+"\n")
	recPath, _ := console.DaemonRecordPath(home, "127.0.0.1:7598")
	if err := console.WriteDaemonRecord(recPath, console.NewDaemonRecord("127.0.0.1:7598", 42, "", "", "c0ffee", time.Now())); err != nil {
		t.Fatal(err)
	}
	rec, ok := readDaemonRecordFor("http://127.0.0.1:7598")
	if !ok || rec.PID != 42 {
		t.Fatalf("a logging-off record is still a record: ok=%v rec=%+v", ok, rec)
	}
	got := resolveConsoleCallLog("", "")
	if got.Rule != console.RuleNone || got.Path != "" || !strings.Contains(got.Note, "logging off") {
		t.Fatalf("resolved = %+v, want rule none naming the daemon's logging-off start", got)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"metrics", "traffic", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("metrics traffic: %d %s", code, errb.String())
	}
	if strings.Contains(out.String(), `"total_calls": 1`) {
		t.Errorf("the verb read the legacy log %s the logging-off daemon never wrote:\n%s", filepath.Base(legacy), out.String())
	}
	// An explicit flag still wins over the record.
	if got := resolveConsoleCallLog(legacy, ""); got.Rule != console.RuleFlag {
		t.Errorf("--call-log must win: %+v", got)
	}
}

// A repo whose .fathom/observations.md exists and cannot be read reports
// the read error — on its tile ("unreadable", never "no observations
// file"), in the shared metric (state error) and in the fleet report's
// JSON (observations_error), which used to drop the repo's count silently.
func TestObservations_UnreadableFileNamedEverywhere(t *testing.T) {
	root := t.TempDir()
	obs := filepath.Join(root, ".fathom", "observations.md")
	mustWrite(t, obs, "## Open\n\n### O1 — x\n")
	if err := os.Chmod(obs, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(obs, 0o644) })
	if _, err := os.ReadFile(obs); err == nil {
		t.Skip("running with privileges that read a mode-000 file")
	}
	n, errNote := serviceOpenObservations(root)
	if n != nil || !strings.Contains(errNote, "permission denied") {
		t.Fatalf("serviceOpenObservations = %v, %q; want nil and the read error", n, errNote)
	}
	if n, errNote := serviceOpenObservations(t.TempDir()); n != nil || errNote != "" {
		t.Errorf("no file = %v, %q; want nil and no error (not reporting)", n, errNote)
	}

	two := 2
	m := observatoryModel{Services: []obsServiceRow{
		{fleetServiceRow: fleetServiceRow{Service: "alpha", Path: "/r/alpha", OpenObservations: &two, Freshness: "fresh"}},
		{fleetServiceRow: fleetServiceRow{Service: "beta", Path: root, ObservationsError: errNote, Freshness: "fresh"}},
	}}
	if v := consoleModelMetrics(m)[console.MetricObservations]; v.State != console.StateError || !strings.Contains(v.Reason, "beta") {
		t.Errorf("observations.open = %+v, want error naming beta", v)
	}
	for name, body := range map[string]string{"fleet": renderObsFleet(m), "project": func() string { b, _ := renderObsService(m, "beta"); return b }()} {
		if !strings.Contains(body, "unreadable") || !strings.Contains(body, "permission denied") {
			t.Errorf("%s tile must say the file is unreadable and why", name)
		}
	}

	// The fleet report carries the error beside the missing count.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/statusz" {
			w.Write([]byte(`{"services":[]}`))
			return
		}
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"beta","symbol_count":1,"languages":["go"],"path":"` + root + `"}]}}`))
	}))
	defer srv.Close()
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "none.jsonl"))
	rep, err := buildFleetReport(&http.Client{Timeout: 5 * time.Second}, srv.URL, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := rep.Services[0].ObservationsError; !strings.Contains(got, "permission denied") {
		t.Errorf("fleet report observations_error = %q, want the read error", got)
	}
}

// The store feeders publish what moved and nothing else: a hash-gated
// no-op re-ingest (a new indexed_at over unchanged content) is not an
// ingest event, a changed file is; and every compile run recorded between
// two polls gets its own event, so two layers compiled within the 10 s poll
// each name their layer (the real store shows runs 58 ms apart).
func TestConsoleEvents_StorePollPublishesContentAndEveryRun(t *testing.T) {
	ctx := t.Context()
	repo := t.TempDir()
	for _, name := range []string{"ledger.go", "errors.go"} {
		raw, err := os.ReadFile(filepath.Join(fauxtownPath("ledger-service"), name))
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(repo, name), string(raw))
	}
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ingest := func() {
		t.Helper()
		if _, err := c.Ingest(ctx, client.ServiceRef{ID: "ledger", Path: repo}); err != nil {
			t.Fatal(err)
		}
	}
	ingest()
	bus := console.NewEventBus(console.BusOptions{})
	src := newConsoleEventSources(bus, c, "", "")
	src.pollStore(ctx) // baseline
	cursor := bus.Cursor()
	since := func() []console.Event {
		sub, evs, _ := bus.SubscribeAfter(cursor)
		sub.Close()
		cursor = bus.Cursor()
		return evs
	}

	time.Sleep(2 * time.Millisecond) // a later indexed_at
	ingest()                         // hash-gated: nothing changed
	src.pollStore(ctx)
	if evs := since(); len(evs) != 0 {
		t.Errorf("a no-op re-ingest published %+v", evs)
	}
	f, _ := os.OpenFile(filepath.Join(repo, "ledger.go"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("\nfunc addedForTheTest() {}\n")
	f.Close()
	ingest()
	src.pollStore(ctx)
	if evs := since(); len(evs) != 1 || evs[0].Kind != console.EventIngest || evs[0].Service != "ledger" {
		t.Errorf("a content change = %+v, want one ingest event", evs)
	}

	at := time.Now().UTC()
	for i, layer := range []string{"adr-rules", "spec-bindings"} {
		if err := c.Store().RecordCompileRun(ctx, core.CompileRun{RunID: "run-" + layer, ServiceID: "ledger", Layer: layer, RanAt: at.Add(time.Duration(i) * 60 * time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
	}
	src.pollStore(ctx)
	evs := since()
	var layers []string
	for _, e := range evs {
		if e.Kind == console.EventCompile {
			layers = append(layers, e.Name)
		}
	}
	if strings.Join(layers, ",") != "adr-rules,spec-bindings" {
		t.Errorf("compile events = %v, want one per run, oldest first", layers)
	}
	src.pollStore(ctx)
	if evs := since(); len(evs) != 0 {
		t.Errorf("an unchanged store published %+v", evs)
	}
}

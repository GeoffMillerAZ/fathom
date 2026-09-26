package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestProjectActivity_FromResolvedLog pins OC-39: a project's tool calls
// come from the RESOLVED call log (the path ResolveCallLog chose, named in
// the model), scoped to the project and the window, with the console's own
// reads excluded and counted; its sessions are the shared project.sessions
// metric itself (OC-9: one projection, never a recount), and the sessions
// listed come from the same read of the ledger, one per session id by the
// one dedupe rule, through the label mapping the model uses. A day with no
// rows is a measured zero; a missing log is absent and an unreadable one an
// error — never zeros.
func TestProjectActivity_FromResolvedLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "daemon-calls.jsonl")
	ledger := filepath.Join(dir, "session-ledger.jsonl")
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(logPath, strings.Join([]string{
		`{"ts":"2026-09-10T10:00:00Z","tool":"find_symbol","service":"fathom","surface":"agent","latency_ms":3}`, // outside the window
		`{"ts":"2026-09-24T10:00:00Z","tool":"find_symbol","service":"fathom","surface":"agent","latency_ms":3}`,
		`{"ts":"2026-09-24T11:00:00Z","tool":"obligations","service":"fathom","surface":"","latency_ms":5,"error":"boom"}`,
		`{"ts":"2026-09-25T09:00:00.123Z","tool":"references","service":"Fathom","surface":"agent","latency_ms":4}`,
		`{"ts":"2026-09-25T09:30:00Z","tool":"list_services","service":"fathom","origin":"console","latency_ms":1}`, // the console reading itself
		`{"ts":"2026-09-25T09:31:00Z","tool":"find_symbol","service":"app-kit","surface":"agent","latency_ms":2}`,   // another project
		`{"ts":"2026-09-25T09:32:00Z","tool":"verify_plan","service":"","latency_ms":9}`,                            // unattributed
		`not json`,
	}, "\n")+"\n")
	ledgerRead := []byte(strings.Join([]string{
		`{"session_id":"s-1","arm":"on","repo":"sextant","at":"2026-09-24T10:00:00Z","turn_count":3}`,
		`{"session_id":"s-1","arm":"on","repo":"sextant","at":"2026-09-24T10:05:00Z","turn_count":7,"first_edit_turn":2}`,
		`{"session_id":"s-2","arm":"off","repo":"fathom","at":"2026-09-25T09:00:00Z","turn_count":4}`,
		`{"session_id":"s-3","arm":"on","repo":"app-kit","at":"2026-09-25T09:00:00Z"}`,
		// A count that does not decode still names its session: the shared
		// count counts it, so the list must too.
		`{"session_id":"s-4","arm":"on","repo":"fathom","at":"2026-09-20T09:00:00Z","turn_count":"many"}`,
	}, "\n") + "\n")
	// The ledger on disk has grown since the model's read (a session the
	// shared count never saw): the tab lists from the read, never the file.
	write(ledger, string(ledgerRead)+`{"session_id":"s-late","arm":"on","repo":"fathom","at":"2026-09-25T11:59:00Z"}`+"\n")
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	resolved := ResolveCallLog(CallLogInputs{Daemon: logPath, DaemonNote: "recorded by serve-http pid 1"})
	src := Sources{CallLog: resolved, SessionLedger: ledger, Now: func() time.Time { return now }}
	labelKey := func(l string) string {
		if strings.EqualFold(l, "sextant") {
			return "fathom" // the Stop hook labels this repo by its directory
		}
		return strings.ToLower(l)
	}
	n := DistinctSessionsBy(ledgerRead, labelKey)["fathom"]
	shared := ProjectSessions(WorkspaceFacts{Name: "fathom", Sessions: &n, SessionsState: SessionCount(ledger, ledgerRead, nil)})
	m := ProjectActivity(src, ActivityInput{Service: "fathom", LabelKey: labelKey, Sessions: shared, Ledger: ledgerRead})

	if m.CallLog.Path != logPath || m.CallLog.Rule != RuleDaemon {
		t.Fatalf("the model must name the resolved log and its rule: %+v", m.CallLog)
	}
	if m.WindowCalls.State != StatePresent || m.WindowCalls.V != 3 {
		t.Errorf("window calls = %+v, want the project's 3 (case-folded service, window-bounded)", m.WindowCalls)
	}
	if !strings.Contains(m.WindowCalls.Source, logPath) || !strings.Contains(m.WindowCalls.Source, "daemon") {
		t.Errorf("the figure names the log it was read from: %q", m.WindowCalls.Source)
	}
	if m.ConsoleReads != 1 || !strings.Contains(m.WindowCalls.Denominator, "1 console page reads excluded") {
		t.Errorf("console reads excluded and named: %d, %q", m.ConsoleReads, m.WindowCalls.Denominator)
	}
	if m.Errors.V != 1 {
		t.Errorf("errors = %+v", m.Errors)
	}
	if len(m.Days) != 14 || m.From != "2026-09-12" || m.To != "2026-09-25" {
		t.Fatalf("window = %s … %s over %d days", m.From, m.To, len(m.Days))
	}
	byDate := map[string]Value[int]{}
	for _, d := range m.Days {
		byDate[d.Date] = d.Calls
	}
	if byDate["2026-09-24"].V != 2 || byDate["2026-09-25"].V != 1 || byDate["2026-09-20"].State != StateZero {
		t.Errorf("days = %+v", m.Days)
	}
	if len(m.Recent) != 3 || m.Recent[0].Tool != "references" || m.Recent[1].Error != "boom" {
		t.Errorf("recent newest first = %+v", m.Recent)
	}
	if m.Sessions != shared || m.Sessions.V != 3 {
		t.Fatalf("sessions = %+v, want the shared metric %+v itself", m.Sessions, shared)
	}
	if len(m.SessionRows) != m.Sessions.V || m.SessionRows[0].ID != "s-2" || m.SessionRows[2].ID != "s-4" {
		t.Fatalf("the sessions listed are the ones counted, newest first: %+v", m.SessionRows)
	}
	s1 := m.SessionRows[1]
	if s1.Rows != 2 || s1.Turns == nil || *s1.Turns != 7 || s1.FirstEditTurn == nil || *s1.FirstEditTurn != 2 {
		t.Errorf("a session keeps its newest row and counts its rows: %+v", s1)
	}
	if s4 := m.SessionRows[2]; s4.Turns != nil || s4.Arm != "on" {
		t.Errorf("a count that does not decode is not recorded, the session still listed: %+v", s4)
	}
	// Not read for this snapshot: absent, and no rows.
	if unread := ProjectActivity(src, ActivityInput{Service: "fathom", LabelKey: labelKey, Ledger: ledgerRead}); unread.Sessions.State != StateAbsent || len(unread.SessionRows) != 0 {
		t.Errorf("no shared read = %+v %+v", unread.Sessions, unread.SessionRows)
	}

	// A missing log is absent — every day, never a row of zeros.
	noLedger := filepath.Join(dir, "none-ledger.jsonl")
	_, readErr := os.ReadFile(noLedger)
	gone := ProjectActivity(Sources{CallLog: ResolvedCallLog{Path: filepath.Join(dir, "none.jsonl"), Rule: RuleDaemon}, SessionLedger: noLedger, Now: src.Now},
		ActivityInput{Service: "fathom", Sessions: ProjectSessions(WorkspaceFacts{Name: "fathom", SessionsState: SessionCount(noLedger, nil, readErr)})})
	if gone.WindowCalls.State != StateAbsent || gone.Days[0].Calls.State != StateAbsent || gone.Sessions.State != StateAbsent {
		t.Errorf("missing sources = %+v %+v %+v", gone.WindowCalls, gone.Days[0].Calls, gone.Sessions)
	}
	// Logging off (rule none) is absent with the resolver's own words.
	off := ProjectActivity(Sources{CallLog: ResolveCallLog(CallLogInputs{DaemonOff: true}), Now: src.Now}, ActivityInput{Service: "fathom"})
	if off.WindowCalls.State != StateAbsent || !strings.Contains(off.WindowCalls.Reason, "logging off") {
		t.Errorf("logging off = %+v", off.WindowCalls)
	}
	// An unreadable log is an error naming the failure.
	if err := os.Chmod(logPath, 0); err == nil && os.Getuid() != 0 {
		bad := ProjectActivity(src, ActivityInput{Service: "fathom"})
		if bad.WindowCalls.State != StateError || !strings.Contains(bad.WindowCalls.Reason, "unreadable") || bad.Days[3].Calls.State != StateError {
			t.Errorf("unreadable log = %+v", bad.WindowCalls)
		}
		_ = os.Chmod(logPath, 0o644)
	}
}

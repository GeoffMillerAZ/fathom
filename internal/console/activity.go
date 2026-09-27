package console

// activity.go — OC-39 (vision 35 §3 "Activity: sessions and tool calls";
// design §1): one project's sessions and tool calls, read from the
// RESOLVED call log (Sources.CallLog — the log the daemon writes, never a
// path relative to a working directory) and the session ledger.
//
// The console's own page reads (origin "console") are logged but are not
// usage: they are left out and counted, and the count is named. A day with
// no rows in a log that WAS read is a measured zero; a log that could not
// be read makes every figure absent or an error — never a row of zeros.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"
)

// ActivityInput selects one project's activity.
type ActivityInput struct {
	Service string
	// LabelKey maps a session-ledger repo label to the service whose root it
	// names (the model's sessionLabelKey); nil compares the label to the
	// service lowered.
	LabelKey func(label string) string
	// Days is the window (default 14), ending at Sources.Now.
	Days int
	// Limit bounds the recent calls and sessions listed (default 20).
	Limit int
	// Sessions is the project's shared project.sessions metric
	// (ProjectSessions over the model's one read of the session ledger): the
	// tab shows that value, never a recount of its own (OC-9). A zero Value
	// is "not read".
	Sessions Value[int]
	// Ledger is the session ledger's bytes as that same read saw them; the
	// sessions listed are folded from them, by the one dedupe rule, so the
	// list and the count are one read.
	Ledger []byte
	// LedgerFold is that read already folded (a tail's); when set, Ledger
	// is not read.
	LedgerFold *LedgerFold
}

// ActivityCall is one logged tool call.
type ActivityCall struct {
	At        string  `json:"at"`
	Tool      string  `json:"tool"`
	Surface   string  `json:"surface"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

// ActivityDay is one day of the window.
type ActivityDay struct {
	Date  string     `json:"date"`
	Calls Value[int] `json:"calls"`
}

// ActivitySession is one distinct session in the project (its newest row).
type ActivitySession struct {
	ID            string `json:"id"`
	Arm           string `json:"arm,omitempty"`
	At            string `json:"at"`
	Turns         *int   `json:"turns,omitempty"`
	FirstEditTurn *int   `json:"first_edit_turn,omitempty"`
	FathomCalls   *int   `json:"fathom_calls,omitempty"`
	Rows          int    `json:"rows"`
}

// ProjectActivityModel is the activity tab's projection.
type ProjectActivityModel struct {
	Service string          `json:"service"`
	CallLog ResolvedCallLog `json:"call_log"`
	From    string          `json:"from"`
	To      string          `json:"to"`
	// WindowCalls is the project's calls in the window; Days splits them.
	WindowCalls Value[int]    `json:"window_calls"`
	Days        []ActivityDay `json:"days"`
	// Errors is the window's calls that returned an error.
	Errors Value[int] `json:"window_errors"`
	// Recent is the newest calls in the window, newest first.
	Recent []ActivityCall `json:"recent"`
	// ConsoleReads is how many of the project's rows were the console
	// reading itself — excluded, named.
	ConsoleReads int `json:"console_reads_excluded"`
	// Sessions is the distinct sessions in the ledger for the project (all
	// time) — the shared project.sessions metric itself; SessionRows the
	// newest of them, from the same read.
	Sessions      Value[int]        `json:"sessions"`
	SessionRows   []ActivitySession `json:"session_rows"`
	SessionLedger string            `json:"session_ledger"`
}

// callRow is the slice of a call-log row the projection reads.
type callRow struct {
	TS        string  `json:"ts"`
	Tool      string  `json:"tool"`
	Service   string  `json:"service"`
	Surface   string  `json:"surface"`
	Origin    string  `json:"origin"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error"`
}

// ProjectActivity reads the resolved call log and the session ledger for
// one project. It is deterministic given the files and src's clock.
func ProjectActivity(src Sources, in ActivityInput) ProjectActivityModel {
	days, limit := in.Days, in.Limit
	if days <= 0 {
		days = 14
	}
	if limit <= 0 {
		limit = 20
	}
	now := src.Clock().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -days)
	svc := strings.ToLower(in.Service)
	m := ProjectActivityModel{
		Service: in.Service, CallLog: src.CallLog, SessionLedger: src.SessionLedger,
		From: start.Format("2006-01-02"), To: end.AddDate(0, 0, -1).Format("2006-01-02"),
		Days: []ActivityDay{}, Recent: []ActivityCall{}, SessionRows: []ActivitySession{},
	}

	logSource := src.CallLog.Path + " (" + string(src.CallLog.Rule) + ": " + src.CallLog.Note + ")"
	// A row that does not mention the service anywhere cannot be its row:
	// skipping it before decoding keeps a 90,000-row log cheap to read.
	svcBytes := []byte(strings.ToLower(in.Service))
	calls, readErr := readCallRows(src.CallLog.Path, func(line []byte) bool { return bytes.Contains(bytes.ToLower(line), svcBytes) })
	var absent, failed string
	switch {
	case src.CallLog.Path == "":
		absent = src.CallLog.Note
		if absent == "" {
			absent = "no call log was resolved"
		}
	case readErr != nil && errors.Is(readErr, fs.ErrNotExist):
		absent = "no call log at " + src.CallLog.Path + " — nothing recorded yet; calls are not measured, never zero"
	case readErr != nil:
		failed = "call log unreadable: " + readErr.Error()
	}
	perDay := map[string]int{}
	total, errs := 0, 0
	var recent []ActivityCall
	if absent == "" && failed == "" {
		for _, c := range calls {
			if strings.ToLower(c.Service) != svc {
				continue
			}
			if c.Origin == "console" {
				m.ConsoleReads++
				continue
			}
			ts, ok := parseCallTS(c.TS)
			if !ok || ts.Before(start) || !ts.Before(end) {
				continue
			}
			total++
			perDay[ts.UTC().Format("2006-01-02")]++
			if c.Error != "" {
				errs++
			}
			recent = append(recent, ActivityCall{At: ts.UTC().Format(time.RFC3339), Tool: c.Tool, Surface: c.Surface, LatencyMS: c.LatencyMS, Error: c.Error})
		}
	}
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		day := ActivityDay{Date: key}
		switch {
		case failed != "":
			day.Calls = Errored[int](failed)
		case absent != "":
			day.Calls = Absent[int](absent)
		default:
			day.Calls = Measured(perDay[key], logSource)
		}
		m.Days = append(m.Days, day)
	}
	window := fmt.Sprintf("%s … %s (%d days, UTC)", m.From, m.To, days)
	switch {
	case failed != "":
		m.WindowCalls, m.Errors = Errored[int](failed).WithSource(logSource), Errored[int](failed).WithSource(logSource)
	case absent != "":
		m.WindowCalls, m.Errors = Absent[int](absent).WithSource(logSource), Absent[int](absent).WithSource(logSource)
	default:
		denom := "calls attributed to " + in.Service + " in " + window
		if m.ConsoleReads > 0 {
			denom += "; " + thousands(m.ConsoleReads) + " console page reads excluded (not usage)"
		}
		m.WindowCalls = Measured(total, logSource).WithDenominator(denom).WithAsOf(now)
		m.Errors = Measured(errs, logSource).WithDenominator("of " + thousands(total) + " calls in the window, the ones that returned an error").WithAsOf(now)
	}
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].At > recent[j].At })
	if len(recent) > limit {
		recent = recent[:limit]
	}
	m.Recent = append(m.Recent, recent...)

	m.Sessions = in.Sessions
	if m.Sessions.State == "" {
		m.Sessions = Absent[int]("the session ledger was not read for this snapshot")
	}
	if m.Sessions.Known() {
		m.SessionRows = append(m.SessionRows, sessionRowsFold(in.LedgerFold, in.Ledger, in.Service, in.LabelKey, limit)...)
	}
	return m
}

// sessionRows folds ledger bytes into the project's distinct sessions,
// newest first, at most limit of them — keyed by the one dedupe rule every
// count shares (LedgerFold: one per session id; a row with none counts
// once on its own), so the rows are exactly the sessions the shared count
// counted. A row whose optional fields do not decode still names its
// session; those fields are then not recorded.
func sessionRows(data []byte, service string, labelKey func(string) string, limit int) []ActivitySession {
	return sessionRowsFold(nil, data, service, labelKey, limit)
}

// sessionRowsFold is sessionRows over a folded ledger, or data folded now.
func sessionRowsFold(f *LedgerFold, data []byte, service string, labelKey func(string) string, limit int) []ActivitySession {
	if labelKey == nil {
		labelKey = func(l string) string {
			if strings.EqualFold(strings.TrimSpace(l), service) {
				return service
			}
			return strings.ToLower(l)
		}
	}
	if f == nil {
		f = FoldLedger(data)
	}
	return f.activitySessions(service, labelKey, limit)
}

// optionalCount decodes an optional count field: nil when it is missing or
// is not a number — not recorded, never a 0.
func optionalCount(raw json.RawMessage) *int {
	var n int
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil {
		return nil
	}
	return &n
}

// readCallRows reads the rows of a JSONL call log that keep passes;
// malformed lines are skipped (the adoption reader's rule).
func readCallRows(path string, keep func(line []byte) bool) ([]callRow, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var out []callRow
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || (keep != nil && !keep(line)) {
			continue
		}
		var r callRow
		if json.Unmarshal(line, &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// parseCallTS parses a call-log timestamp (RFC3339, with or without
// fractional seconds).
func parseCallTS(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

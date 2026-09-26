package main

// metrics_evidence.go — the Evidence verbs the console renders (vision 35
// §3 Evidence, P10): `fathom metrics impact`, `metrics catches` and
// `metrics sessions`. Each prints the internal/console projection its page
// shows (ImpactView, CatchTimeline, SessionTable / SessionWindow), built by
// the same adapters from the same instruments, so the page's JSON section
// and the verb's --json cannot disagree (TestConsolePagesRenderTheirVerbs).
//
// The adapters take their sources as values: a verb fills them from fresh
// reads, a page from the observatory model's one read (OC-9), so a page's
// table and its shared metrics never come from two different reads.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// consoleVerbNow is the clock of the verbs the console renders — the fleet
// report's (fleetReportNow), so the JSON-parity test pins the page and
// every verb it names to one instant.
func consoleVerbNow() time.Time { return fleetReportNow() }

// callLogState is a call-log read's state for a projection: "" when the log
// was read, else the reason, and whether it is an error (a log that exists
// and failed) rather than an absence. It is console.CallTotal's own
// classification, so every page agrees with the shared call metric.
func callLogState(f console.CallLogFacts) (reason string, isErr bool, source string) {
	v := console.CallTotal(f)
	if v.Known() {
		return "", false, v.Source
	}
	return v.Reason, v.State == console.StateError, v.Source
}

// fileState classifies a ledger read: "" when it read, else the reason and
// whether the file exists and failed (an error) rather than being absent.
func fileState(what, path string, err error) (reason string, isErr bool) {
	switch {
	case err == nil:
		return "", false
	case path == "":
		return "no " + what + " path is configured", false
	case os.IsNotExist(err):
		return "no " + what + " at " + path + " — not measured, never zero", false
	}
	return what + " unreadable: " + err.Error(), true
}

// toolClassesOf is each tool's dominant caller class over the usage rows.
func toolClassesOf(entries []adoptionCallEntry) map[string]console.CallerClass {
	out := map[string]console.CallerClass{}
	for _, tl := range console.ToolLoads(callRowsOf(entries)) {
		out[tl.Tool] = tl.Class
	}
	return out
}

// callRowsOf converts call-log entries to the console's row shape.
func callRowsOf(entries []adoptionCallEntry) []console.CallRow {
	rows := make([]console.CallRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, console.CallRow{TS: e.TS, Tool: e.Tool, Service: e.Service, Surface: e.Surface, Origin: e.Origin, LatencyMS: e.LatencyMS, Error: e.Error})
	}
	return rows
}

// reviewRunsOf keeps the `fathom review` rows that carry per-dimension
// finding counts (review.go's forced-channel row, T5-7).
func reviewRunsOf(entries []adoptionCallEntry) []console.ReviewRun {
	var out []console.ReviewRun
	for _, e := range entries {
		if e.Tool != "review" || e.VerdictOutcome == nil {
			continue
		}
		out = append(out, console.ReviewRun{At: e.TS, Service: e.Service, Outcome: e.VerdictOutcome})
	}
	return out
}

// ─── impact ──────────────────────────────────────────────────────────────

// impactSources is what ImpactView is built from.
type impactSources struct {
	imp        obsImpact
	misses     []obsMissRow
	classes    map[string]console.CallerClass
	callFacts  console.CallLogFacts
	ledgerPath string
	ledgerErr  error
	verify     []verifyLedgerRow
}

// impactInputsFrom adapts the sources to ImpactView's inputs.
func impactInputsFrom(s impactSources) console.ImpactInputs {
	imp := s.imp
	in := console.ImpactInputs{
		LedgerSource: "session ledger " + s.ledgerPath,
		Sessions:     imp.LedgerSessions,
		Arms: console.ArmInputs{On: imp.armOn, Off: imp.armOff, Excluded: imp.UncleanArmRows,
			Criterion: imp.CriterionNote, PilotState: imp.PilotState, Detail: imp.PilotDetail},
		ExpSessions: imp.ExpSessions, ExpRawCalls: imp.ExpRawCalls, ExpRawTokens: imp.ExpRawTokens,
		ExpIdxCalls: imp.ExpFathomCalls, ExpIdxTokens: imp.ExpFathomTokens, ExpRatioX10: imp.ExpRatioMedianX10, ExpNote: imp.ExpNote,
		StopCatches: imp.VerifyCatchesTotal, StopCatchesRan: imp.catchesRan,
		ObligationsServed: imp.ObligationCalls,
		FirstEditMedian:   imp.FirstEditMedian, FirstEditSessions: imp.firstEditN, FirstEditNote: imp.FirstEditNote,
	}
	in.LedgerReason, in.LedgerError = fileState("session ledger", s.ledgerPath, s.ledgerErr)
	in.CallLogReason, in.CallLogError, in.CallLogSource = callLogState(s.callFacts)
	in.GateRefusals, in.GateRuns, in.GateNote = gateCatchStatsFromLedger(s.verify)
	for _, r := range s.misses {
		in.Tools = append(in.Tools, console.ToolOutcomeInput{Tool: r.Tool, Class: s.classes[r.Tool], Calls: r.Calls,
			Hits: r.Hits, Misses: r.Misses, DidYouMean: r.DidYouMean, NotRecorded: r.NotRecorded})
	}
	return in
}

// readCallLogFacts reads the resolved call log once: its usage entries
// (console reads excluded) and the facts the shared call metric reads.
func readCallLogFacts(resolved console.ResolvedCallLog) ([]adoptionCallEntry, console.CallLogFacts) {
	if resolved.Path == "" {
		return nil, console.CallLogFacts{Log: resolved}
	}
	entries, _, consoleReads, err := callLogReadCounting(resolved.Path)
	f := callLogFactsFor(resolved, entries, err)
	if err == nil {
		f.ConsoleReads = consoleReads
	}
	return entries, f
}

// freshImpactSources is the verb's read of every impact source.
func freshImpactSources(resolved console.ResolvedCallLog, now time.Time) impactSources {
	entries, facts := readCallLogFacts(resolved)
	imp := buildImpactModel(now)
	for _, e := range entries {
		if e.Tool == "obligations" {
			imp.ObligationCalls++
		}
	}
	path := sessionLedgerPath()
	_, ledgerErr := os.Stat(path)
	return impactSources{imp: imp, misses: buildMissModel(entries), classes: toolClassesOf(entries), callFacts: facts,
		ledgerPath: path, ledgerErr: ledgerErr, verify: readVerifyLedger()}
}

func cmdMetricsImpact(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics impact", stderr)
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: the daemon's configured log (its record), then [daemon].call_log, then the legacy default")
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: *callLogFlag, BaseURL: *url})
	m := console.ImpactView(impactInputsFrom(freshImpactSources(resolved, consoleVerbNow())))
	if *jsonOut {
		jsonPrint(stdout, withCallLog(m, resolved))
		return 0
	}
	fmt.Fprintln(stdout, "fathom impact — the four questions, in order")
	fmt.Fprintln(stdout, callLogLine(resolved))
	for _, q := range m.Questions {
		fmt.Fprintf(stdout, "\n%s · %s\n  %s\n", q.ID, q.Question, q.Answer)
		for _, f := range q.Figures {
			fmt.Fprintf(stdout, "  %-48s %s\n", f.Label, valuePlain(f.Value))
		}
	}
	fmt.Fprintf(stdout, "\narms (%s):\n", m.ArmUnit)
	for _, a := range m.Arms {
		fmt.Fprintf(stdout, "  %-26s sessions %s · p25 %s · median %s · p75 %s\n", a.Label, valuePlain(a.Units), valuePlain(a.Low), valuePlain(a.Median), valuePlain(a.High))
	}
	return 0
}

// ─── catches ─────────────────────────────────────────────────────────────

// catchSources is what CatchTimeline is built from.
type catchSources struct {
	verify     []verifyLedgerRow
	verifyPath string
	verifyErr  error
	reviews    []console.ReviewRun
	callFacts  console.CallLogFacts
	guard      []console.GuardBeat
	guardPath  string
	guardErr   error
}

func catchInputsFrom(s catchSources) console.CatchInputs {
	in := console.CatchInputs{Reviews: s.reviews, GuardBeats: s.guard,
		VerifySource: "verify ledger " + s.verifyPath, GuardSource: "hooks log " + s.guardPath}
	in.VerifyReason, _ = fileState("verify ledger", s.verifyPath, s.verifyErr)
	for _, r := range s.verify {
		in.VerifyRuns = append(in.VerifyRuns, console.VerifyRun{At: r.At, Service: r.Service, Plan: r.Plan, Origin: r.Origin, Failing: r.Failing, Claimed: r.Claimed})
	}
	in.ReviewReason, _, in.ReviewSource = callLogState(s.callFacts)
	in.GuardReason, _ = fileState("hooks log", s.guardPath, s.guardErr)
	return in
}

// readGuardBeats reads the PreToolUse edit-hook beats (the guard hook's)
// from the hooks log, naming each beat's service through labelKey (the
// project directory's base name mapped to the service whose root it is).
// A line is decoded only when it carries the pre phase.
func readGuardBeats(path string, labelKey func(string) string) ([]console.GuardBeat, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []console.GuardBeat
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	needle := []byte(`"phase":"pre"`)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, needle) || !bytes.Contains(line, []byte(`"edit-hook"`)) {
			continue
		}
		var b struct {
			Channel string            `json:"channel"`
			Kind    string            `json:"kind"`
			At      time.Time         `json:"at"`
			Meta    map[string]string `json:"meta"`
		}
		if json.Unmarshal(line, &b) != nil || b.Channel != "edit-hook" || b.Meta["phase"] != "pre" {
			continue
		}
		svc := ""
		if dir := b.Meta["project_dir"]; dir != "" && labelKey != nil {
			svc = labelKey(filepath.Base(dir))
		}
		out = append(out, console.GuardBeat{At: b.At, Kind: b.Kind, Target: b.Meta["target"], Service: svc, Session: b.Meta["session"]})
	}
	return out, sc.Err()
}

// catchFilterFrom builds the timeline filter from query values: kinds
// (unknown ones ignored), a service, and a window of days back from now
// (0 = all time).
func catchFilterFrom(kinds []string, service string, days int, now time.Time) console.CatchFilter {
	f := console.CatchFilter{Service: service}
	for _, raw := range kinds {
		for _, k := range strings.Split(raw, ",") {
			if kind, ok := console.ParseCatchKind(strings.TrimSpace(k)); ok {
				f.Kinds = append(f.Kinds, kind)
			}
		}
	}
	if days > 0 {
		f.From = now.UTC().AddDate(0, 0, -days)
	}
	return f
}

// catchPageSize is the catches table's page: the newest twenty, the rest
// paged (the timeline above it draws every catch in the window).
const catchPageSize = 20

// stringsFlag is a repeatable string flag.
type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

func cmdMetricsCatches(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics catches", stderr)
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL): review runs' conservation and integrity findings")
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log, and whose registry names a guard beat's service")
	var kinds stringsFlag
	fs.Var(&kinds, "kind", "keep one kind (repeatable or comma-separated): verify-refusal, conserve-loss, integrity-catch, guard-refusal")
	service := fs.String("service", "", "keep one service's catches")
	days := fs.Int("days", 0, "keep the last N days (0 = all time)")
	page := fs.Int("page", 1, "page of rows (newest first)")
	perPage := fs.Int("per-page", catchPageSize, "rows per page")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	now := consoleVerbNow()
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: *callLogFlag, BaseURL: *url})
	entries, facts := readCallLogFacts(resolved)
	roots := map[string]string{}
	if svcs, err := fetchServices(&http.Client{Timeout: 5 * time.Second}, *url); err == nil {
		for name, info := range svcs {
			roots[name] = info.path
		}
	}
	src := freshCatchSources(reviewRunsOf(entries), facts, sessionLabelKey(roots))
	m := console.CatchTimeline(catchInputsFrom(src), catchFilterFrom(kinds, *service, *days, now), *page, *perPage)
	if *jsonOut {
		jsonPrint(stdout, withCallLog(m, resolved))
		return 0
	}
	fmt.Fprintln(stdout, "fathom catches — what the gates and guards caught")
	fmt.Fprintln(stdout, callLogLine(resolved))
	for _, k := range m.Kinds {
		fmt.Fprintf(stdout, "  %-18s %s\n", k.Label, valuePlain(k.Count))
	}
	for _, c := range m.Catches {
		fmt.Fprintf(stdout, "  %s  %-16s %-22s %s — %s\n", c.At.Format(time.RFC3339), c.Kind, c.Service, c.Subject, c.Detail)
	}
	fmt.Fprintf(stdout, "%d catch(es); page %d of %d\n", m.Total, m.Page, m.Pages)
	return 0
}

// freshCatchSources reads the verify ledger and the hooks log; the review
// rows and the call log's state come from the caller's one read of the log.
func freshCatchSources(reviews []console.ReviewRun, facts console.CallLogFacts, labelKey func(string) string) catchSources {
	vp := verifyLedgerPath()
	_, verr := os.Stat(vp)
	gp := beatLogPath()
	guard, gerr := readGuardBeats(gp, labelKey)
	return catchSources{verify: readVerifyLedger(), verifyPath: vp, verifyErr: verr, reviews: reviews, callFacts: facts,
		guard: guard, guardPath: gp, guardErr: gerr}
}

// ─── sessions ────────────────────────────────────────────────────────────

// sessionReplay is one session's replay window with its last ledger row.
type sessionReplay struct {
	Window     console.SessionWindowModel `json:"window"`
	Ledger     *sessionLedgerEntry        `json:"ledger,omitempty"`
	LedgerNote string                     `json:"ledger_note,omitempty"`
}

// replayHiddenMeta are beat labels the replay does not repeat on every
// event (the session itself, provenance stamps, the project directory).
var replayHiddenMeta = map[string]bool{"session": true, "fathom_version": true, "schema_version": true, "harness": true, "project_dir": true, "service": true}

// readSessionEvents reads one session's beats from the hooks log and its
// one rotated generation (hooks.jsonl.1, older), decoding only the lines
// that name the session — the survey's replay re-decoded the whole log.
func readSessionEvents(path, sessionID string) ([]console.ReplayEvent, error) {
	if path == "" || sessionID == "" {
		return nil, nil
	}
	needle := []byte(`"session":"` + sessionID + `"`)
	var out []console.ReplayEvent
	var firstErr error
	for i, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			if i == 1 && !os.IsNotExist(err) {
				firstErr = err
			}
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if !bytes.Contains(line, needle) {
				continue
			}
			var b struct {
				Channel string            `json:"channel"`
				Kind    string            `json:"kind"`
				At      time.Time         `json:"at"`
				Meta    map[string]string `json:"meta"`
			}
			if json.Unmarshal(line, &b) != nil || b.Meta["session"] != sessionID {
				continue
			}
			out = append(out, console.ReplayEvent{At: b.At.UTC(), Channel: b.Channel, Kind: b.Kind, Service: b.Meta["service"], Detail: replayDetail(b.Meta)})
		}
		if err := sc.Err(); err != nil && firstErr == nil {
			firstErr = err
		}
		f.Close()
	}
	return out, firstErr
}

// sessionEventsCache keeps the last few sessions' events for the model's
// TTL: paging and filtering a replay re-reads nothing for 30 s (the hooks
// log grows with every edit, so its size cannot key the cache).
var sessionEventsCache struct {
	sync.Mutex
	entries map[string]sessionEventsEntry
}

type sessionEventsEntry struct {
	at     time.Time
	events []console.ReplayEvent
}

// cachedSessionEvents is readSessionEvents behind sessionEventsCache (at
// most eight sessions are kept).
func cachedSessionEvents(path, sessionID string) []console.ReplayEvent {
	key := path + "\x00" + sessionID
	now := time.Now()
	sessionEventsCache.Lock()
	if e, ok := sessionEventsCache.entries[key]; ok && now.Sub(e.at) < obsCacheTTL && !now.Before(e.at) {
		sessionEventsCache.Unlock()
		return e.events
	}
	sessionEventsCache.Unlock()
	events, _ := readSessionEvents(path, sessionID)
	sessionEventsCache.Lock()
	defer sessionEventsCache.Unlock()
	if sessionEventsCache.entries == nil || len(sessionEventsCache.entries) >= 8 {
		sessionEventsCache.entries = map[string]sessionEventsEntry{}
	}
	sessionEventsCache.entries[key] = sessionEventsEntry{at: now, events: events}
	return events
}

// replayDetail renders a beat's remaining labels as "k=v" pairs, sorted; a
// file path is shown relative to the project directory.
func replayDetail(meta map[string]string) string {
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if !replayHiddenMeta[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := meta[k]
		if k == "file" {
			if dir := meta["project_dir"]; dir != "" {
				if rel, err := filepath.Rel(dir, v); err == nil && !strings.HasPrefix(rel, "..") {
					v = rel
				}
			}
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, " ")
}

// lastLedgerRow is a session's last ledger row in ledger bytes.
func lastLedgerRow(ledger []byte, sessionID string) *sessionLedgerEntry {
	var last *sessionLedgerEntry
	needle := []byte(`"` + sessionID + `"`)
	for _, line := range bytes.Split(ledger, []byte{'\n'}) {
		if !bytes.Contains(line, needle) {
			continue
		}
		var e sessionLedgerEntry
		if json.Unmarshal(line, &e) == nil && e.SessionID == sessionID {
			cp := e
			last = &cp
		}
	}
	return last
}

// buildSessionReplay is `fathom metrics sessions --id S --json`'s whole
// computation; the session page calls it with its own read of the ledger.
func buildSessionReplay(ledger []byte, sessionID string, channels []string, page, perPage int) sessionReplay {
	events := cachedSessionEvents(beatLogPath(), sessionID)
	r := sessionReplay{Window: console.SessionWindow(sessionID, events, channels, page, perPage)}
	if r.Ledger = lastLedgerRow(ledger, sessionID); r.Ledger == nil {
		r.LedgerNote = "no ledger row for this session (the stop hook writes one at each turn's end)"
	}
	return r
}

func cmdMetricsSessions(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics sessions", stderr)
	arm := fs.String("arm", "", "keep one arm (on | off)")
	repo := fs.String("repo", "", "keep one repo label")
	q := fs.String("q", "", "keep sessions whose id or repo contains this")
	sortKey := fs.String("sort", "at", "sort by: "+strings.Join(console.SessionSorts, " | "))
	asc := fs.Bool("asc", false, "ascending (default: descending)")
	id := fs.String("id", "", "replay one session: its events, windowed")
	var channels stringsFlag
	fs.Var(&channels, "channel", "with --id: keep one beat channel (repeatable or comma-separated)")
	page := fs.Int("page", 1, "page (1-based)")
	perPage := fs.Int("per-page", 0, "rows per page (default 50; with --id, events per window: default 500, at most 2000)")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ledgerPath := sessionLedgerPath()
	ledger, lerr := os.ReadFile(ledgerPath)
	if lerr != nil && !os.IsNotExist(lerr) {
		fmt.Fprintf(stderr, "warn: session ledger unreadable: %v\n", lerr)
	}
	if *id != "" {
		var chans []string
		for _, c := range channels {
			chans = append(chans, strings.Split(c, ",")...)
		}
		r := buildSessionReplay(ledger, *id, chans, *page, *perPage)
		if *jsonOut {
			jsonPrint(stdout, r)
			return 0
		}
		w := r.Window
		fmt.Fprintf(stdout, "session %s — %d event(s), window %d of %d (%d per window)\n", w.SessionID, w.Matching, w.Page, w.Pages, w.PerPage)
		for _, e := range w.Window {
			fmt.Fprintf(stdout, "  %s  %-14s %-12s %s\n", e.At.Format(time.RFC3339), e.Channel, e.Kind, e.Detail)
		}
		return 0
	}
	per := *perPage
	if per <= 0 {
		per = 50
	}
	m := console.SessionTable(ledger, console.SessionFilter{Arm: *arm, Repo: *repo, Query: *q, Sort: *sortKey, Asc: *asc}, *page, per)
	if *jsonOut {
		jsonPrint(stdout, m)
		return 0
	}
	fmt.Fprintf(stdout, "sessions — %d distinct from %d ledger rows; %d match; page %d of %d\n", m.Sessions, m.LedgerRows, m.Matching, m.Page, m.Pages)
	num := func(p *int) string {
		if p == nil {
			return "—"
		}
		return fmtThousands(*p)
	}
	for _, r := range m.Rows {
		fmt.Fprintf(stdout, "  %-36s %-20s %-4s turns %-6s first edit %-5s tokens %-10s fathom %-5s catches %s\n", r.ID, r.Repo, r.Arm, num(r.Turns), num(r.FirstEditTurn), num(r.Tokens), num(r.FathomCalls), num(r.VerifyCatches))
	}
	return 0
}

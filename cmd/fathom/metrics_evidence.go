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
	return foldCallLog(entries).toolClasses()
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
	return foldCallLog(entries).reviewRuns()
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

// ─── load ────────────────────────────────────────────────────────────────

// cmdMetricsLoad is `fathom metrics load`: the daemon's load from the
// resolved call log — calls, p50, p95 and error rate per UTC day per
// surface (14 days; a day with no row at all is absent, never zero), the
// caller classes and the tools by caller class (7 days). It prints
// console.LoadView, exactly what the Operate page's traffic heatmap and
// "What carries the load" panels render (P10).
func cmdMetricsLoad(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics load", stderr)
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: the daemon's configured log (its record), then [daemon].call_log, then the legacy default")
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: *callLogFlag, BaseURL: *url})
	entries, facts := readCallLogFacts(resolved)
	reason, isErr, _ := callLogState(facts)
	m := console.LoadView(operateLoadInputs(entries, resolved, reason, isErr, consoleVerbNow()))
	if *jsonOut {
		jsonPrint(stdout, m) // the model carries its call_log itself
		return 0
	}
	fmt.Fprintln(stdout, "fathom load — calls per day per surface, and who calls which tool")
	fmt.Fprintln(stdout, callLogLine(resolved))
	fmt.Fprintf(stdout, "calls, last 7 days: %s\n", valuePlain(m.WindowCalls))
	if !m.WindowCalls.Known() {
		return 0
	}
	for _, d := range m.DayTotals {
		fmt.Fprintf(stdout, "  %s  %s\n", d.Day, valuePlain(d.Calls))
	}
	for _, c := range m.Classes {
		fmt.Fprintf(stdout, "  class %-13s calls %-7d p50 %.0f ms  p95 %.0f ms  errors %.1f%%\n", c.Class, c.Calls, c.P50MS, c.P95MS, c.ErrorRate)
	}
	fmt.Fprintf(stdout, "tools called: %s\n", valuePlain(m.ToolsCalled))
	for _, t := range m.Tools {
		var split []string
		for _, c := range t.By {
			split = append(split, fmt.Sprintf("%s %d", c.Class, c.Calls))
		}
		fmt.Fprintf(stdout, "  %-24s %7d  %s\n", t.Tool, t.Calls, strings.Join(split, ", "))
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
	// guardPartial is a hooks-log generation that could not be read while
	// the other was (the guard count is then a lower bound).
	guardPartial error
	// registry decides which rows name a service it does not list (OC-H2).
	registry console.Registry
}

func catchInputsFrom(s catchSources) console.CatchInputs {
	in := console.CatchInputs{Reviews: s.reviews, GuardBeats: s.guard, Registry: s.registry,
		VerifySource: "verify ledger " + s.verifyPath, GuardSource: hooksLogSource(s.guardPath)}
	in.VerifyReason, in.VerifyError = fileState("verify ledger", s.verifyPath, s.verifyErr)
	for _, r := range s.verify {
		in.VerifyRuns = append(in.VerifyRuns, console.VerifyRun{At: r.At, Service: r.Service, Plan: r.Plan, Origin: r.Origin, Failing: r.Failing, Claimed: r.Claimed})
	}
	in.ReviewReason, in.ReviewError, in.ReviewSource = callLogState(s.callFacts)
	in.GuardReason, in.GuardError = fileState("hooks log", s.guardPath, s.guardErr)
	if s.guardPartial != nil {
		in.GuardPartial = "a lower bound — " + s.guardPartial.Error()
	}
	return in
}

// hooksLogSource names the hooks log and its rotated generation — the two
// files every hooks-log reader here reads (beats.go keeps one prior
// generation, hooks.jsonl.1, as the bounded backstop).
func hooksLogSource(path string) string {
	return "hooks log " + path + " and its rotated generation " + path + ".1"
}

// readGuardBeats reads the PreToolUse edit-hook beats (the guard hook's)
// from the hooks log and its rotated generation, naming each beat's service
// through labelKey (the project directory's base name mapped to the service
// whose root it is). It reads through the daemon's hooks-log index (RT-114:
// nothing reads hooks.jsonl whole — only what was appended since the last
// read is decoded). partial is a generation that could not be read while
// the other was.
func readGuardBeats(path string, labelKey func(string) string) (beats []console.GuardBeat, partial, err error) {
	return hooksBeatIndex.guardBeats(path, labelKey)
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
	svcs, svcErr := fetchServices(&http.Client{Timeout: 5 * time.Second}, *url)
	for name, info := range svcs {
		roots[name] = info.path
	}
	src := freshCatchSources(reviewRunsOf(entries), facts, sessionLabelKey(roots), registryOf(svcs, svcErr))
	in := catchInputsFrom(src)
	if *service != "" && !in.NamesService(*service) {
		if _, registered := roots[*service]; !registered {
			if in.ExcludesNamedService(*service) {
				fmt.Fprintf(stderr, "warn: the daemon's registry does not list %q — its rows are excluded from every lane (named under unregistered without --service)\n", *service)
			} else {
				fmt.Fprintf(stderr, "warn: no instrument names a service %q and the daemon registers none — every lane is not instrumented for it\n", *service)
			}
		}
	}
	m := console.CatchTimeline(in, catchFilterFrom(kinds, *service, *days, now), *page, *perPage)
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

// freshCatchSources reads the verify ledger and both hooks-log
// generations; the review rows and the call log's state come from the
// caller's one read of the log. A ledger that exists and fails to read is
// an error (the external volume's EPERM wedge: stat succeeds, the read
// does not), never an empty ledger.
func freshCatchSources(reviews []console.ReviewRun, facts console.CallLogFacts, labelKey func(string) string, registry console.Registry) catchSources {
	vp := verifyLedgerPath()
	verify, verr := readVerifyLedgerErr(vp)
	gp := beatLogPath()
	guard, partial, gerr := readGuardBeats(gp, labelKey)
	return catchSources{verify: verify, verifyPath: vp, verifyErr: verr, reviews: reviews, callFacts: facts,
		guard: guard, guardPath: gp, guardErr: gerr, guardPartial: partial, registry: registry}
}

// ─── sessions ────────────────────────────────────────────────────────────

// sessionReplay is one session's replay window with its last ledger row.
// LedgerState is the row's truth state: present, absent (no ledger, or no
// row for this session — LedgerNote says which) or error (the ledger exists
// and could not be read).
type sessionReplay struct {
	Window      console.SessionWindowModel `json:"window"`
	Ledger      *sessionLedgerEntry        `json:"ledger,omitempty"`
	LedgerState console.State              `json:"ledger_state"`
	LedgerNote  string                     `json:"ledger_note,omitempty"`
}

// replayHiddenMeta are beat labels the replay does not repeat on every
// event (the session itself, provenance stamps, the project directory).
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var replayHiddenMeta = map[string]bool{"session": true, "fathom_version": true, "schema_version": true, "harness": true, "project_dir": true, "service": true}

// readSessionEvents reads one session's beats from the hooks log and its
// one rotated generation (hooks.jsonl.1, older) through the per-session
// offset index (RT-114: hooksBeatIndex) — only this session's lines are
// read and decoded, and the log is never read whole per page. err and
// partial are the index's (beatOffsetIndex.refresh): nothing could be
// read, or one generation could not.
func readSessionEvents(path, sessionID string) (events []console.ReplayEvent, partial, err error) {
	if sessionID == "" {
		return nil, nil, nil
	}
	lines, partial, err := sessionBeatIndex(path, sessionID)
	if err != nil {
		return nil, nil, err
	}
	for _, line := range lines {
		var b struct {
			Channel string            `json:"channel"`
			Kind    string            `json:"kind"`
			At      time.Time         `json:"at"`
			Meta    map[string]string `json:"meta"`
		}
		if json.Unmarshal(line, &b) != nil || b.Meta["session"] != sessionID {
			continue
		}
		events = append(events, console.ReplayEvent{At: b.At.UTC(), Channel: b.Channel, Kind: b.Kind, Service: b.Meta["service"], Detail: replayDetail(b.Meta)})
	}
	return events, partial, nil
}

// sessionEventsCache keeps the last few sessions' reads for the model's
// TTL: paging and filtering a replay re-reads nothing for 30 s (the hooks
// log grows with every edit, so its size cannot key the cache). A failed
// read is cached with its reason, so a replay never turns an unreadable log
// into "no beats".
var sessionEventsCache struct {
	sync.Mutex
	entries map[string]sessionEventsEntry
}

type sessionEventsEntry struct {
	at  time.Time
	src console.ReplaySource
}

// resetSessionEventsCache drops the cached reads (tests that change the log).
func resetSessionEventsCache() {
	sessionEventsCache.Lock()
	sessionEventsCache.entries = nil
	sessionEventsCache.Unlock()
}

// cachedSessionEvents is readSessionEvents behind sessionEventsCache (at
// most eight sessions are kept), as the replay's source with its read state.
func cachedSessionEvents(path, sessionID string) console.ReplaySource {
	key := path + "\x00" + sessionID
	now := time.Now()
	sessionEventsCache.Lock()
	if e, ok := sessionEventsCache.entries[key]; ok && now.Sub(e.at) < obsCacheTTL && !now.Before(e.at) {
		sessionEventsCache.Unlock()
		return e.src
	}
	sessionEventsCache.Unlock()
	events, partial, err := readSessionEvents(path, sessionID)
	src := console.ReplaySource{Events: events, Source: hooksLogSource(path)}
	src.Reason, src.Error = fileState("hooks log", path, err)
	if partial != nil {
		src.Partial = "a lower bound — " + partial.Error()
	}
	sessionEventsCache.Lock()
	defer sessionEventsCache.Unlock()
	if sessionEventsCache.entries == nil || len(sessionEventsCache.entries) >= 8 {
		sessionEventsCache.entries = map[string]sessionEventsEntry{}
	}
	sessionEventsCache.entries[key] = sessionEventsEntry{at: now, src: src}
	return src
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

// buildSessionReplay is `fathom metrics sessions --id S --json`'s whole
// computation; the session page calls it with the model's read of the
// ledger (ledgerErr is that read's error: the ledger row is then absent or
// an error naming the ledger, never "no row for this session").
func buildSessionReplay(ledgerPath string, ledger *sessionLedgerSnap, ledgerErr error, sessionID string, channels []string, page, perPage int) sessionReplay {
	src := cachedSessionEvents(beatLogPath(), sessionID)
	r := sessionReplay{Window: console.SessionWindow(sessionID, src, channels, page, perPage), LedgerState: console.StatePresent}
	if reason, isErr := fileState("session ledger", ledgerPath, ledgerErr); reason != "" {
		r.LedgerState, r.LedgerNote = console.StateAbsent, reason
		if isErr {
			r.LedgerState = console.StateError
		}
		return r
	}
	if r.Ledger = ledger.lastRow(sessionID); r.Ledger == nil {
		r.LedgerState = console.StateAbsent
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
	if reason, _ := fileState("session ledger", ledgerPath, lerr); reason != "" {
		fmt.Fprintf(stderr, "warn: %s\n", reason)
	}
	if *id != "" {
		var chans []string
		for _, c := range channels {
			chans = append(chans, strings.Split(c, ",")...)
		}
		r := buildSessionReplay(ledgerPath, foldSessionLedger(ledger), lerr, *id, chans, *page, *perPage)
		if *jsonOut {
			jsonPrint(stdout, r)
			return 0
		}
		w := r.Window
		if !w.Events.Known() {
			fmt.Fprintf(stdout, "session %s — events %s\n", w.SessionID, valuePlain(w.Events))
			return 0
		}
		fmt.Fprintf(stdout, "session %s — %s event(s), window %d of %d (%d per window)\n", w.SessionID, valuePlain(w.Matching), w.Page, w.Pages, w.PerPage)
		for _, e := range w.Window {
			fmt.Fprintf(stdout, "  %s  %-14s %-12s %s\n", e.At.Format(time.RFC3339), e.Channel, e.Kind, e.Detail)
		}
		return 0
	}
	per := *perPage
	if per <= 0 {
		per = 50
	}
	m := console.SessionTable(ledgerPath, ledger, lerr, console.SessionFilter{Arm: *arm, Repo: *repo, Query: *q, Sort: *sortKey, Asc: *asc}, *page, per)
	if *jsonOut {
		jsonPrint(stdout, m)
		return 0
	}
	if !m.Sessions.Known() {
		fmt.Fprintf(stdout, "sessions — %s\n", valuePlain(m.Sessions))
		return 0
	}
	fmt.Fprintf(stdout, "sessions — %s distinct from %s ledger rows; %s match; page %d of %d\n", valuePlain(m.Sessions), valuePlain(m.LedgerRows), valuePlain(m.Matching), m.Page, m.Pages)
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

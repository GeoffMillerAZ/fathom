package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

// fathom fleet report (vision 14 §4, plan IO-16): the fleet audit that took
// an agent an afternoon, as a standing one-command product. Reads the
// daemon's service registry, the repo-scoped call log, each service's local
// .fathom state and hook wiring, and the user-central session ledger; writes
// ONE local artifact (~/.fathom/fleet/report-<date>.{json,md}) with a row
// per service. Collection stays sovereign per repo — this verb only READS;
// no daemon writes, no network beyond the loopback daemon, nothing shared.

type fleetServiceRow struct {
	Service       string `json:"service"`
	Path          string `json:"path,omitempty"`
	Symbols       int    `json:"symbols"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	RepoHead      string `json:"repo_head,omitempty"`
	Freshness     string `json:"freshness"` // fresh | stale | unknown: <reason>

	HooksNote        string `json:"hooks_note"`  // counted fathom hook registrations per scope, or the honest absence
	StanzaNote       string `json:"stanza_note"` // CLAUDE.md fathom stanza present?
	OpenObservations *int   `json:"open_observations,omitempty"`
	// ObservationsError is set when .fathom/observations.md exists and
	// could not be read: the count is then unknown, never "no file".
	ObservationsError string `json:"observations_error,omitempty"`

	// Unproven is the newest recorded `fathom intent audit` for this repo
	// (EP-23) — read from the LOCAL intent ledger, never computed here: a
	// fleet report that ran 40 audits would take minutes and would report
	// numbers nobody asked for. nil = no audit recorded, rendered
	// "n/a: no audit recorded", never 0.
	Unproven     *int   `json:"unproven,omitempty"`
	UnprovenAt   string `json:"unproven_at,omitempty"`
	CallsTotal   int    `json:"calls_total"`
	CallsAgent   int    `json:"calls_agent"`
	LastCallTS   string `json:"last_call_ts,omitempty"`
	SessionCount *int   `json:"session_count,omitempty"` // from the ledger, when readable
}

type fleetReport struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	GeneratedAt   string `json:"generated_at"`
	CallLogPath   string `json:"call_log_path"`
	CallLogNote   string `json:"call_log_note,omitempty"`
	// CallLog names the rule that chose CallLogPath (OC-2), additively.
	CallLog    console.ResolvedCallLog `json:"call_log"`
	LedgerNote string                  `json:"ledger_note,omitempty"`

	Services []fleetServiceRow `json:"services"`
}

// fleetServiceRowWire is a row as the report writes it: the traffic columns
// shadow the embedded ints so a call log that was not read omits them
// instead of writing a 0 beside a note saying "never zero" (doctrine rule
// 6). encoding/json prefers the shallower field of the same name.
type fleetServiceRowWire struct {
	fleetServiceRow
	CallsTotal *int `json:"calls_total,omitempty"`
	CallsAgent *int `json:"calls_agent,omitempty"`
}

// MarshalJSON writes the report with its traffic columns present only when
// the call log was read (CallLogNote empty).
func (rep fleetReport) MarshalJSON() ([]byte, error) {
	type plain fleetReport
	wire := struct {
		plain
		Services []fleetServiceRowWire `json:"services"`
	}{plain: plain(rep), Services: make([]fleetServiceRowWire, 0, len(rep.Services))}
	for _, r := range rep.Services {
		w := fleetServiceRowWire{fleetServiceRow: r}
		if rep.CallLogNote == "" {
			total, agent := r.CallsTotal, r.CallsAgent
			w.CallsTotal, w.CallsAgent = &total, &agent
		}
		wire.Services = append(wire.Services, w)
	}
	return json.Marshal(wire)
}

// Bumped to 2 for the unproven column (EP-23). A reader pinned to v1 sees a
// field it does not know; a reader that CHECKS the version knows why.
const fleetReportSchemaVersion = 2

func cmdFleetReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet report", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	outDir := fs.String("out", "", "output directory (default ~/.fathom/fleet)")
	jsonOnly := fs.Bool("json", false, "print the JSON report to stdout instead of writing files")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	hc := &http.Client{Timeout: 5 * time.Second}
	rep, err := buildFleetReport(hc, *url, fleetReportNow())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOnly {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}

	dir := *outDir
	if dir == "" {
		dir = fleetSnapshotDir()
		if dir == "" {
			fmt.Fprintf(stderr, "error: cannot resolve the fathom home directory (no $HOME, no %s)\n", fathomhome.Env)
			return 1
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	date := time.Now().UTC().Format("2006-01-02")
	jsonPath := filepath.Join(dir, "report-"+date+".json")
	mdPath := filepath.Join(dir, "report-"+date+".md")
	jb, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(jsonPath, append(jb, '\n'), 0o600); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(mdPath, []byte(renderFleetReportMD(rep)), 0o600); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "fleet report: %d service(s) → %s and %s\n", len(rep.Services), jsonPath, mdPath)
	return 0
}

// fleetReportNow is the fleet report's clock — a variable so the console's
// JSON-parity test (TestConsolePagesRenderTheirVerbs) can pin the page and
// the verb to one instant.
var fleetReportNow = time.Now

// buildFleetReport is `fathom fleet report --json`'s whole computation. The
// /fleet console page calls the same function for its "fathom fleet report
// --json" section, so the page and the verb cannot disagree (P10, OC-8).
func buildFleetReport(hc *http.Client, url string, now time.Time) (fleetReport, error) {
	svcs, err := fetchServices(hc, url)
	if err != nil {
		return fleetReport{}, fmt.Errorf("fleet report needs the daemon's service registry: %w", err)
	}

	rep := fleetReport{
		SchemaVersion: fleetReportSchemaVersion,
		FathomVersion: buildinfo.Version(),
		GeneratedAt:   now.UTC().Format(time.RFC3339),
	}

	// Call-log aggregation through the one resolution every verb and page
	// shares (OC-2, console_calllog.go): the daemon's configured log first.
	resolved := resolveConsoleCallLogFor(callLogQuery{BaseURL: url})
	callLogPath := resolved.Path
	rep.CallLogPath, rep.CallLog = callLogPath, resolved
	entries, _, callLogErr := classifyCallLogRead(callLogPath)
	switch {
	case callLogErr == nil:
	case callLogPath == "":
		rep.CallLogNote = resolved.Note + " — traffic columns are not measured, never zero"
	case os.IsNotExist(callLogErr):
		rep.CallLogNote = "no call log at " + callLogPath + " — traffic columns are not measured, never zero"
	default:
		rep.CallLogNote = "call log unreadable: " + callLogErr.Error() + " — traffic columns are not measured, never zero"
	}
	type traffic struct {
		total, agent int
		last         string
	}
	perSvc := map[string]*traffic{}
	for _, e := range entries {
		if e.Service == "" {
			continue
		}
		k := strings.ToLower(e.Service)
		tr := perSvc[k]
		if tr == nil {
			tr = &traffic{}
			perSvc[k] = tr
		}
		tr.total++
		if e.Surface == "agent" || e.Surface == "companion" {
			tr.agent++
		}
		if e.TS > tr.last {
			tr.last = e.TS
		}
	}

	// Ledger sessions per repo label: distinct sessions (the one dedupe rule,
	// console.DistinctSessionsByRepo — the Stop hook appends a row per
	// turn), from the ledger sessionLedgerPath names (FATHOM_SESSION_LEDGER
	// honored, as every other ledger reader does).
	// A row's repo label is mapped to the service whose root it names ("sextant"
	// → fathom), so a service is never a false 0 beside its own sessions.
	roots := make(map[string]string, len(svcs))
	for name, info := range svcs {
		roots[name] = info.path
	}
	var sessionsByRepo map[string]int
	ledgerReadable := false
	if data, rerr := os.ReadFile(sessionLedgerPath()); rerr == nil {
		ledgerReadable = true
		sessionsByRepo = console.DistinctSessionsBy(data, sessionLabelKey(roots))
	}
	if !ledgerReadable {
		rep.LedgerNote = "session ledger not readable — session counts are not measured, never zero"
	}

	// One ledger read for the whole fleet (EP-23) — read, never computed.
	intentLedger := intent.ReadLedger()

	names := make([]string, 0, len(svcs))
	for name := range svcs {
		names = append(names, name)
	}
	sort.Strings(names)
	st := daemonStatuszRead(hc, url) // one statusz read for the fleet (RT-115)
	for _, name := range names {
		info := svcs[name]
		row := fleetServiceRow{Service: name, Path: info.path, Symbols: info.symbols}
		row.Freshness, row.IndexedCommit, row.RepoHead = fleetFreshnessFrom(st, name, info.path)
		row.HooksNote, row.StanzaNote = fleetWiringNotes(info.path)
		row.OpenObservations, row.ObservationsError = serviceOpenObservations(info.path)
		if info.path != "" {
			if n, at, ok := fleetUnprovenColumn(intentLedger, info.path); ok {
				row.Unproven, row.UnprovenAt = &n, at
			}
		}
		if tr := perSvc[name]; tr != nil {
			row.CallsTotal, row.CallsAgent, row.LastCallTS = tr.total, tr.agent, tr.last
		}
		if ledgerReadable {
			n := sessionsByRepo[name]
			row.SessionCount = &n
		}
		rep.Services = append(rep.Services, row)
	}

	return rep, nil
}

// statuszRead is one read of the daemon's /statusz: each service's
// indexed commit and compile summary, or why it could not be read.
type statuszRead struct {
	indexed map[string]string // lower-cased service → indexed commit
	compile map[string]string // lower-cased service → compile summary
	err     string            // "" when read: "statusz unreachable" / "statusz unparsable"
}

// daemonStatuszRead reads the daemon's /statusz once: in process when
// baseURL is this daemon (RT-115: no loopback request), else over HTTP.
func daemonStatuszRead(hc *http.Client, baseURL string) statuszRead {
	var rep struct {
		Services []struct {
			Service             string            `json:"service"`
			IndexedCommit       string            `json:"indexed_commit"`
			LastCompilePerLayer map[string]string `json:"last_compile_per_layer"`
		} `json:"services"`
	}
	raw, err := daemonStatuszJSON(hc, baseURL)
	if err != nil {
		return statuszRead{err: "statusz unreachable"}
	}
	if json.Unmarshal(raw, &rep) != nil {
		return statuszRead{err: "statusz unparsable"}
	}
	out := statuszRead{indexed: map[string]string{}, compile: map[string]string{}}
	for _, s := range rep.Services {
		k := strings.ToLower(s.Service)
		out.indexed[k] = s.IndexedCommit
		if len(s.LastCompilePerLayer) == 0 {
			continue
		}
		parts := make([]string, 0, len(s.LastCompilePerLayer))
		for layer, sum := range s.LastCompilePerLayer {
			parts = append(parts, layer+": "+sum)
		}
		sort.Strings(parts)
		out.compile[k] = strings.Join(parts, " · ")
	}
	return out
}

// daemonStatuszJSON is the daemon's /statusz JSON: built in process when
// baseURL is this daemon, else fetched.
func daemonStatuszJSON(hc *http.Client, baseURL string) ([]byte, error) {
	if r := inProcessDaemon(baseURL); r != nil && r.statusz != nil {
		return json.Marshal(r.statusz(context.Background()))
	}
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := hc.Get(strings.TrimRight(baseURL, "/") + "/statusz?format=json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /statusz: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// fleetFreshness compares the daemon's indexed commit against the repo's
// HEAD when the corpus root is locally readable. Honest three-state. The
// HEAD is read from the repository's files (gitro.Head: no git process)
// and the indexed commit from the daemon's statusz — in process inside the
// daemon (RT-115). A caller walking many services reads statusz once
// (daemonStatuszRead) and calls fleetFreshnessFrom.
func fleetFreshness(hc *http.Client, baseURL, service, path string) (freshness, indexed, head string) {
	if path == "" {
		return "unknown: no recorded corpus path", "", ""
	}
	return fleetFreshnessFrom(daemonStatuszRead(hc, baseURL), service, path)
}

// fleetFreshnessFrom is fleetFreshness over one statusz read.
func fleetFreshnessFrom(st statuszRead, service, path string) (freshness, indexed, head string) {
	if path == "" {
		return "unknown: no recorded corpus path", "", ""
	}
	head, gerr := gitro.Head(context.Background(), path)
	if gerr != nil {
		return "unknown: repo HEAD unreadable (" + gerr.Error() + ")", "", ""
	}
	if st.err != "" {
		return "unknown: " + st.err, "", head
	}
	indexed, ok := st.indexed[strings.ToLower(service)]
	if !ok {
		return "unknown: service not in statusz", "", head
	}
	if indexed == "" {
		return "unknown: no indexed commit recorded", indexed, head
	}
	if strings.HasPrefix(head, indexed) || strings.HasPrefix(indexed, head) {
		return "fresh", indexed, head
	}
	return "stale: indexed " + shortSHA(indexed) + " vs HEAD " + shortSHA(head), indexed, head
}

// fleetWiringNotes reports hook + stanza state for a locally readable root.
func fleetWiringNotes(path string) (hooks, stanza string) {
	if path == "" {
		return "not checkable: no recorded corpus path", "not checkable"
	}
	settings := filepath.Join(path, ".claude", "settings.json")
	data, err := os.ReadFile(settings)
	switch {
	case err == nil:
		n := strings.Count(string(data), "fathom")
		if n == 0 {
			// O15/DG-14: a project settings.json with zero fathom
			// references does not mean fathom has no hooks here —
			// `fathom init`'s own user-scope dedup (O42) deliberately
			// wires hooks at ~/.claude/settings.json instead of
			// per-project whenever they are already present there. Check
			// that scope before reporting an absence this confidently;
			// the realization grader (realizationGrade) keys off the
			// "hook reference" substring, so a genuine user-scope find
			// grades as wired, not as a gap.
			if un, ok := userScopeFathomHookCount(); ok && un > 0 {
				hooks = fmt.Sprintf("wired at user scope (%d fathom hook reference(s) in ~/.claude/settings.json; none in project settings.json)", un)
			} else {
				hooks = "settings.json present, no fathom hooks"
			}
		} else {
			hooks = fmt.Sprintf("%d fathom hook reference(s) in project settings", n)
		}
	case os.IsNotExist(err):
		if un, ok := userScopeFathomHookCount(); ok && un > 0 {
			hooks = fmt.Sprintf("wired at user scope (%d fathom hook reference(s) in ~/.claude/settings.json)", un)
		} else {
			hooks = "no project settings.json (user-scope or plugin wiring not visible from here)"
		}
	default:
		hooks = "settings.json unreadable: " + err.Error()
	}
	cm, err := os.ReadFile(filepath.Join(path, "CLAUDE.md"))
	switch {
	case err == nil && strings.Contains(string(cm), "fathom"):
		stanza = "present"
	case err == nil:
		stanza = "CLAUDE.md present, no fathom stanza"
	case os.IsNotExist(err):
		stanza = "no CLAUDE.md"
	default:
		stanza = "unreadable: " + err.Error()
	}
	return hooks, stanza
}

// userScopeFathomHookCount counts "fathom" occurrences in the current
// user's ~/.claude/settings.json — the same file `fathom init`'s
// user-scope dedup (O42) and doctor's checkHooksInstalled already treat as
// a valid wiring scope. ok is false when $HOME cannot be resolved or the
// file is absent/unreadable, so callers fall back to reporting only what
// the project scope showed rather than claiming an absence they never
// checked.
func userScopeFathomHookCount() (int, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return 0, false
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return 0, false
	}
	// Finding [5]/DG-14: a raw strings.Count(data, "fathom") over the WHOLE
	// file matched a project's fathom MCP-server registration or
	// mcp__fathom__* permission entries just as readily as an actual hook
	// command -- neither of which wires a single hook. Parse the file and
	// count only genuine hook command entries under the "hooks" key, the
	// same scope doctor.go's checkHooksInstalled/hookMarkerPresent trusts.
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		// Malformed JSON: nothing here can be honestly confirmed as wired --
		// never fall back to the substring scan, which is exactly the
		// false-positive class this rewrite exists to close.
		return 0, true
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	return countFathomHookCommands(hooksObj), true
}

// countFathomHookCommands counts actual hook COMMAND entries mentioning
// "fathom" under a parsed settings.json "hooks" object. Deliberately
// looser than doctor.go's hookMarkerPresent (which requires an EXACT
// (event, matcher) match against fathomHookSpecs, one fathom's own
// installer writes) -- this only answers "is there ANY fathom hook entry
// here at all", the question fleetWiringNotes asks before it reports a
// user-scope find. hooksObj may be nil (no "hooks" key, or a settings.json
// that never parsed as an object) -- ranging over a nil map is a no-op.
func countFathomHookCommands(hooksObj map[string]any) int {
	n := 0
	for _, rawGroups := range hooksObj {
		groups, ok := rawGroups.([]any)
		if !ok {
			continue
		}
		for _, rawGroup := range groups {
			group, ok := rawGroup.(map[string]any)
			if !ok {
				continue
			}
			hooks, _ := group["hooks"].([]any)
			for _, rawHook := range hooks {
				hook, ok := rawHook.(map[string]any)
				if !ok {
					continue
				}
				cmd, _ := hook["command"].(string)
				if strings.Contains(cmd, "fathom") {
					n++
				}
			}
		}
	}
	return n
}

func renderFleetReportMD(rep fleetReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fathom fleet report — %s\n\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "> generated by fathom %s · schema v%d · %s\n", rep.FathomVersion, rep.SchemaVersion, callLogLine(rep.CallLog))
	if rep.CallLogNote != "" {
		fmt.Fprintf(&b, "> %s\n", rep.CallLogNote)
	}
	if rep.LedgerNote != "" {
		fmt.Fprintf(&b, "> %s\n", rep.LedgerNote)
	}
	b.WriteString("\n| service | freshness | symbols | calls (agent) | sessions | open obs | unproven | hooks | stanza |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rep.Services {
		obs := "—"
		if r.OpenObservations != nil {
			obs = fmt.Sprintf("%d", *r.OpenObservations)
		} else if r.ObservationsError != "" {
			obs = "unreadable: " + r.ObservationsError
		}
		sessions := "—"
		if r.SessionCount != nil {
			sessions = fmt.Sprintf("%d", *r.SessionCount)
		}
		unproven := "n/a: no audit recorded"
		if r.Unproven != nil {
			unproven = fmt.Sprintf("%d (%s)", *r.Unproven, r.UnprovenAt)
		}
		calls := "—"
		if rep.CallLogNote == "" {
			calls = fmt.Sprintf("%d (%d)", r.CallsTotal, r.CallsAgent)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s | %s | %s | %s |\n",
			r.Service, r.Freshness, r.Symbols, calls, sessions, obs, unproven, r.HooksNote, r.StanzaNote)
	}
	b.WriteString("\nEvery unmeasured cell says so — a dash is \"not measured\", never zero (law 3).\n")
	return b.String()
}

// fleetUnprovenColumn reads the newest recorded intent audit for repoPath
// and returns its total unproven count and when it was measured. Not
// recorded → (0, "", false), which the renderer prints as
// "n/a: no audit recorded" — never a 0 that reads as "clean" (law 3).
//
// Q16 is NOT answered by this: the number stays in a LOCAL artifact under
// ~/.fathom/fleet/. Nothing here reaches the research bundle, and nothing
// should until the owner decides whether honesty-debt counts may leave the
// machine at all (law 13, docs/intent/questions.md Q16).
func fleetUnprovenColumn(rows []intent.LedgerRow, repoPath string) (int, string, bool) {
	newest, ok := intent.NewestDebt(rows, repoPath)
	if !ok {
		return 0, "", false
	}
	total := 0
	for _, n := range newest.ByKind {
		total += n
	}
	return total, newest.At, true
}

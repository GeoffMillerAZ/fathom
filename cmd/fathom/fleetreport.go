package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
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
	LedgerNote    string `json:"ledger_note,omitempty"`

	Services []fleetServiceRow `json:"services"`
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
	svcs, err := fetchServices(hc, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: fleet report needs the daemon's service registry: %v\n", err)
		return 1
	}

	rep := fleetReport{
		SchemaVersion: fleetReportSchemaVersion,
		FathomVersion: buildinfo.Version(),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Call-log aggregation, via the same resolution the adoption gauge uses.
	callLogPath := resolveCallLogPath()
	if _, statErr := os.Stat(callLogPath); statErr != nil {
		for _, cand := range resolveCallLogCandidates(hc, *url, true, ".") {
			if _, e := os.Stat(cand); e == nil {
				callLogPath = cand
				break
			}
		}
	}
	rep.CallLogPath = callLogPath
	entries, _, read := readCallLog(callLogPath)
	if !read {
		rep.CallLogNote = "call log not readable from here — traffic columns are not measured, never zero"
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

	// Ledger sessions per repo label.
	sessionsByRepo := map[string]int{}
	ledgerReadable := false
	if home, herr := os.UserHomeDir(); herr == nil {
		if data, rerr := os.ReadFile(filepath.Join(home, ".fathom", "session-ledger.jsonl")); rerr == nil {
			ledgerReadable = true
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				var row struct {
					Repo string `json:"repo"`
				}
				if json.Unmarshal([]byte(line), &row) == nil && row.Repo != "" {
					sessionsByRepo[strings.ToLower(row.Repo)]++
				}
			}
		}
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
	for _, name := range names {
		info := svcs[name]
		row := fleetServiceRow{Service: name, Path: info.path, Symbols: info.symbols}
		row.Freshness, row.IndexedCommit, row.RepoHead = fleetFreshness(hc, *url, name, info.path)
		row.HooksNote, row.StanzaNote = fleetWiringNotes(info.path)
		if info.path != "" {
			if content, existed, oerr := readObservations(info.path); oerr == nil && existed {
				n := countOpenObservations(content)
				row.OpenObservations = &n
			}
		}
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

	if *jsonOnly {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}

	dir := *outDir
	if dir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			fmt.Fprintf(stderr, "error: %v\n", herr)
			return 1
		}
		dir = filepath.Join(home, ".fathom", "fleet")
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

// fleetFreshness compares the daemon's indexed commit against the repo's
// HEAD when the corpus root is locally readable. Honest three-state.
func fleetFreshness(hc *http.Client, baseURL, service, path string) (freshness, indexed, head string) {
	req, err := originHeader(baseURL, strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_stats","arguments":{}}}`)))
	_ = req
	_ = err
	// The recorded indexed commit comes from ServiceMeta via the daemon;
	// fetchServices doesn't carry it, so resolve locally: git HEAD vs the
	// daemon's own auto-refresh makes "fresh" the overwhelmingly common
	// state — report unknown honestly when either side is unreadable.
	if path == "" {
		return "unknown: no recorded corpus path", "", ""
	}
	out, gerr := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output()
	if gerr != nil {
		return "unknown: repo HEAD unreadable (" + gerr.Error() + ")", "", ""
	}
	head = strings.TrimSpace(string(out))
	// Daemon-side indexed commit via the statusz JSON (loopback).
	resp, herr := hc.Get(baseURL + "/statusz?format=json")
	if herr != nil {
		return "unknown: statusz unreachable", "", head
	}
	defer resp.Body.Close()
	var st struct {
		Services []struct {
			Service       string `json:"service"`
			IndexedCommit string `json:"indexed_commit"`
		} `json:"services"`
	}
	if jerr := json.NewDecoder(resp.Body).Decode(&st); jerr != nil {
		return "unknown: statusz unparsable", "", head
	}
	for _, s := range st.Services {
		if strings.EqualFold(s.Service, service) {
			indexed = s.IndexedCommit
			if indexed != "" && (strings.HasPrefix(head, indexed) || strings.HasPrefix(indexed, head)) {
				return "fresh", indexed, head
			}
			if indexed == "" {
				return "unknown: no indexed commit recorded", indexed, head
			}
			return "stale: indexed " + shortSHA(indexed) + " vs HEAD " + shortSHA(head), indexed, head
		}
	}
	return "unknown: service not in statusz", "", head
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
	return strings.Count(string(data), "fathom"), true
}

func renderFleetReportMD(rep fleetReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fathom fleet report — %s\n\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "> generated by fathom %s · schema v%d · call log: %s\n", rep.FathomVersion, rep.SchemaVersion, rep.CallLogPath)
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
		}
		sessions := "—"
		if r.SessionCount != nil {
			sessions = fmt.Sprintf("%d", *r.SessionCount)
		}
		unproven := "n/a: no audit recorded"
		if r.Unproven != nil {
			unproven = fmt.Sprintf("%d (%s)", *r.Unproven, r.UnprovenAt)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d (%d) | %s | %s | %s | %s | %s |\n",
			r.Service, r.Freshness, r.Symbols, r.CallsTotal, r.CallsAgent, sessions, obs, unproven, r.HooksNote, r.StanzaNote)
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

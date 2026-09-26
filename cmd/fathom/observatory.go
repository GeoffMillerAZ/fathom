package main

// observatory.go — vision 17 / docs/plans/observatory.md: the
// value-realization UI over the impact verbs. One model
// (buildObservatoryModel) renders to JSON and HTML from the same value —
// statusz's pattern at fleet scale. Every metric is value-or-named-reason
// (law 3), savings appear only as counterfactual arm data (law 12), story
// rows render under the lens banner (law 14), and nothing here leaves
// loopback (law 13 — `fathom brief` writes a LOCAL file a human sends).

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/console"
)

const observatorySchemaVersion = 1

// obsServiceRow is one service tile: the fleet-report row plus the remedy
// commands the UI attaches to every warning (the page is a to-do list).
type obsServiceRow struct {
	fleetServiceRow
	FreshnessRemedy string `json:"freshness_remedy,omitempty"`
	WiringRemedy    string `json:"wiring_remedy,omitempty"`
	// Grade / GradeRaise (IQ-15): the wiring grade and the one command
	// that raises it.
	Grade      string `json:"grade,omitempty"`
	GradeRaise string `json:"grade_raise,omitempty"`
	// TrafficDays (IQ-8): this service's last-14-day call counts, oldest
	// first, aligned with the model's TrafficDays dates.
	TrafficDays []int `json:"traffic_days,omitempty"`
	// Config (EX-7/FR-5): the .fathom configuration state, including the
	// obligations-compile condition.
	Config      *obsConfigPanel `json:"config,omitempty"`
	CompileNote string          `json:"compile_note,omitempty"`
	// Realization (phase Y): the five-rung ladder — wired, fresh, queried,
	// preventing, gating — that grades what fathom is DOING here rather
	// than how it is configured.
	Realization *realizationReport `json:"realization,omitempty"`
	// ladder is the rung-level projection the grade folds
	// (console.RealizationLadder): the project overview and `fathom fleet
	// realization` render it. Unexported, so observatoryModel v1's JSON is
	// unchanged.
	ladder *console.Ladder
	// Series (RG-6): the 30-day per-dimension activity series from the call
	// log. Nil when the log could not be read (the model's CallLogNote then
	// carries the reason) — a series of thirty "no rows" absences would
	// claim the log was read and found empty, which is a different claim.
	Series []dimSeries `json:"series,omitempty"`
	// SymbolsNote (RG-8) is non-empty when no store read reported a symbol
	// count for this row: the symbols cell then renders the note, never
	// the int's zero value. buildObservatoryModel leaves it empty because
	// a row only exists once the daemon's registry answered for it; a
	// model assembled from any other source must set it rather than let
	// Symbols default to a fabricated 0 (law 3).
	SymbolsNote string `json:"symbols_note,omitempty"`
	// languages are the registry's languages for the service (the fleet
	// table's column, console_projects.go). Unexported: observatoryModel
	// v1's JSON is a compatibility surface (OC-7).
	languages []string
}

// obsArmBanner / obsNarrateBanner surface the active dials loudly.
type obsArmBanner struct {
	Active bool   `json:"active"`
	Detail string `json:"detail"`
}

// obsImpact is the wins model: every count carries its denominator and its
// note; a nil pointer with a note is an absent instrument, never a zero.
type obsImpact struct {
	LedgerSessions int `json:"ledger_sessions"`
	// LedgerStops is the raw row count. It is reported alongside the
	// session count so the gap between them is visible rather than
	// silently corrected: 200 stops across 8 sessions is a fact about how
	// the harness fires its hook, and hiding it would make the correction
	// unauditable.
	LedgerStops int    `json:"ledger_stops,omitempty"`
	LedgerNote  string `json:"ledger_note,omitempty"`
	DateRange   string `json:"date_range,omitempty"`

	VerifyCatchesTotal *int   `json:"verify_catches_total,omitempty"`
	VerifyCatchesNote  string `json:"verify_catches_note"`

	DiscoveryTotal *int   `json:"discovery_actions_total,omitempty"`
	DiscoveryNote  string `json:"discovery_note"`

	TokensOnSessions  int `json:"tokens_on_sessions"`
	TokensOffSessions int `json:"tokens_off_sessions"`
	// RatedOn / RatedOff and their medians are the OUTCOME half of the
	// pre-registered criterion (production-tokenomics.md §5): "lower median
	// tokens AT EQUAL-OR-BETTER self-rated outcome". Without them a token
	// drop cannot be told apart from an unaided session giving up sooner.
	RatedOn     int  `json:"rated_on_sessions"`
	RatedOff    int  `json:"rated_off_sessions"`
	RatingOnMed *int `json:"rating_on_median,omitempty"`
	// DO-1/DO-2 (docs/design/continuity-threads.md §4.1): the derived
	// outcome — commits landed in each admissible unit's beat-window.
	DerivedOnUnits  int  `json:"derived_on_units"`
	DerivedOffUnits int  `json:"derived_off_units"`
	DerivedOnMed    *int `json:"derived_on_median,omitempty"`
	DerivedOffMed   *int `json:"derived_off_median,omitempty"`
	RatingOffMed    *int `json:"rating_off_median,omitempty"`
	// CriterionNote states, in words, whether the registered criterion can
	// be evaluated yet and what is missing — never a silent absence.
	CriterionNote string `json:"criterion_note"`
	// UncleanArmRows counts ledger rows excluded from the paired
	// comparison because their session predates the arm they wear.
	UncleanArmRows  int    `json:"unclean_arm_rows,omitempty"`
	UncleanArmNote  string `json:"unclean_arm_note,omitempty"`
	TokensOnMedian  *int   `json:"tokens_on_median,omitempty"`
	TokensOffMedian *int   `json:"tokens_off_median,omitempty"`
	// Spend-shaped medians are in+out only (cache reads excluded — the
	// 2026-08-08 inflation finding); these two carry the disclosure.
	TokensPreSplitRows *int `json:"tokens_pre_split_rows,omitempty"`
	CacheReadMedianCtx *int `json:"cache_read_median_ctx,omitempty"`
	// CacheReadFrozenExcluded (IT-12) counts ledger rows excluded from
	// CacheReadMedianCtx's sum because transcript_truncated was true: the
	// 16MB LimitReader (sessionledger.go, transcriptReadMaxBytes) freezes
	// EVERY harness-usage field it was still accumulating — including
	// tokens_cache_read — at whatever partial value had been summed when
	// the cap hit, not the session's true total. Summing that frozen
	// partial alongside real totals silently skews the average; excluding
	// it and naming the count (never a silent drop) is what a truncated
	// row's whole-session figure being unusable actually looks like in the
	// rendered output.
	CacheReadFrozenExcluded *int   `json:"cache_read_frozen_excluded,omitempty"`
	PilotState              string `json:"pilot_state"`
	PilotDetail             string `json:"pilot_detail,omitempty"`

	Stories     []string `json:"stories,omitempty"`
	StoriesNote string   `json:"stories_note"`

	// Compounding (Q4, IQ-7): warm-up length and rules-served proxy.
	FirstEditMedian *int   `json:"first_edit_median_turns,omitempty"`
	FirstEditNote   string `json:"first_edit_note"`
	ObligationCalls int    `json:"obligation_calls"`

	// The exploration story (EX-3): folded across sessions carrying the
	// exploration ledger. Per-question = tokens / calls, per mode; the
	// ratio is OBSERVATIONAL (labeled) — the arm pilot carries causality.
	ExpSessions       int  `json:"exp_sessions"`
	ExpRawCalls       int  `json:"exp_raw_calls"`
	ExpRawTokens      int  `json:"exp_raw_tokens_est"`
	ExpFathomCalls    int  `json:"exp_fathom_calls"`
	ExpFathomTokens   int  `json:"exp_fathom_tokens_est"`
	ExpRatioMedianX10 *int `json:"exp_ratio_median_x10,omitempty"` // per-question raw/indexed ×10
	ExpRepeatFiles    int  `json:"exp_repeat_files"`
	ExpRepeatTokens   int  `json:"exp_repeat_tokens_est"`
	ExpSidechainShare int  `json:"exp_sidechain_results"`
	// ExpToolSearchTurns (BP-23, ADR-032 §5) is the deferral tax summed
	// across the same sessions: turns spent loading deferred MCP schemas.
	// It belongs beside the two exploration modes because only ONE of them
	// pays it — Bash is never deferred (O24) — so a per-question comparison
	// that ignores it flatters the indexed mode.
	ExpToolSearchTurns int    `json:"exp_toolsearch_turns"`
	ExpNote            string `json:"exp_note"`
}

type observatoryModel struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	GeneratedAt   string `json:"generated_at"`

	DegradedNote string `json:"degraded_note,omitempty"`

	Arm     obsArmBanner `json:"arm"`
	Narrate obsArmBanner `json:"narrate"`

	Services []obsServiceRow `json:"services"`

	// TrafficDays is the last 14 days of call-log traffic (REAL rows from
	// the same log the adoption gauge reads), oldest first. Empty when the
	// log is unreadable — never fabricated.
	TrafficDays []obsTrafficDay `json:"traffic_days,omitempty"`

	// CallLogNote (RG-8) is readCallLog's read bool surfaced as a named
	// absence: non-empty when the call log could not be read, and then
	// every call-log-derived figure — per-service traffic, the fleet's
	// tool-call total, the agent-surface share, rules served, the 30-day
	// series — is absent WITH this reason rather than a zero. Empty means
	// the log was read, and a zero downstream is a measured zero.
	CallLogNote string `json:"call_log_note,omitempty"`

	// Misses (IQ-3): per-tool outcome profile from the call log.
	Misses []obsMissRow `json:"misses,omitempty"`

	// SurfaceTax (TD-1): the standing context cost of the agent surface —
	// schema bytes every MCP-connected session carries before any work.
	AgentSurfaceTools int `json:"agent_surface_tools"`
	AgentSchemaBytes  int `json:"agent_schema_bytes"`

	Impact obsImpact `json:"impact"`

	// Cost (BA-8, docs/plans/ambient-fathom.md): what fathom itself put
	// into agent contexts, by tool — the denominator half of the impact
	// story, folded from the same call-log entries the Q3 misses table
	// already reads.
	Cost costPanel `json:"cost"`

	// The console's shared-metric facts (OC-9, internal/console/metrics.go).
	// Unexported: observatoryModel v1's JSON is a compatibility surface and
	// stays byte-for-byte what it was (OC-7).
	callFacts console.CallLogFacts
	sessions  console.Value[int]
	// sessionLedger is the session ledger's bytes as this build read them —
	// the read every sessions figure came from — so a page that lists
	// sessions (the activity tab) lists the ones that were counted.
	sessionLedger []byte
}

// obsTrafficDay is one day of call-log traffic.
type obsTrafficDay struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Calls int    `json:"calls"`
}

// buildObservatoryModel folds the registry, per-service wiring/freshness,
// the call log, the session ledger and the dial states into one value. An
// unreachable registry degrades to a named note with zero service rows —
// the impact section still renders from user-central state (OB-1).
func buildObservatoryModel(baseURL string, now time.Time) observatoryModel {
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: buildinfo.Version(),
		GeneratedAt:   now.UTC().Format(time.RFC3339),
	}

	if st, active := loadArmState(now); active {
		m.Arm = obsArmBanner{Active: true, Detail: armBannerDetail(st)}
	} else {
		m.Arm = obsArmBanner{Detail: "no measurement arm active — channels live (fathom arm off --sessions N starts one)"}
	}
	if st, active := loadNarrateState(now); active {
		m.Narrate = obsArmBanner{Active: true, Detail: narrateBannerDetail(st)}
	} else {
		m.Narrate = obsArmBanner{Detail: "narrate off (default) — story rows not being captured (fathom narrate on --for 7d to pioneer)"}
	}

	// OC-9: the shared sessions metric, from the one dedupe rule every page
	// uses (console.SessionCount) — read once per model build.
	ledgerPath := sessionLedgerPath()
	ledgerData, ledgerErr := os.ReadFile(ledgerPath)
	m.sessions = console.SessionCount(ledgerPath, ledgerData, ledgerErr).WithAsOf(now)
	if ledgerErr == nil {
		m.sessionLedger = ledgerData
	}

	// The model's reads of the daemon (list_services) carry the console's
	// origin: the console reading itself is never usage (mcp.OriginConsole).
	hc := consoleRPCClient(5 * time.Second)
	svcs, err := fetchServices(hc, baseURL)
	if err != nil {
		m.DegradedNote = "service registry unreachable (" + err.Error() + ") — fleet tiles are not measured, never empty-as-healthy"
		m.callFacts = console.CallLogFacts{Log: console.ResolvedCallLog{Rule: console.RuleNone, Note: "not read"}, AbsentReason: "call log not read: " + m.DegradedNote}
	} else {
		// Traffic fold, same resolution the adoption gauge uses.
		perSvc := map[string]*struct {
			total, agent int
			last         string
		}{}
		// OC-2: the one call-log resolution (console_calllog.go). Inside
		// the daemon this is the daemon's own --call-log — the survey found
		// the pages rendering from a different log than the daemon writes.
		resolvedLog := resolveConsoleCallLogFor(callLogQuery{BaseURL: baseURL})
		callLogPath := resolvedLog.Path
		// classifyCallLogRead (instrument_adoption.go) tells "no log yet"
		// (ENOENT) apart from a log that is right there and unreadable (a
		// wedged TCC verdict, a permission defect); both are absences the
		// tiles must name, but only the second is a defect worth a remedy.
		entries, _, consoleReads, callLogErr := callLogReadCounting(callLogPath)
		callLogRead := callLogErr == nil
		m.callFacts = callLogFactsFor(resolvedLog, entries, callLogErr)
		if callLogRead {
			m.callFacts.ConsoleReads = consoleReads
		}
		switch {
		case callLogRead:
		case callLogPath == "":
			m.CallLogNote = resolvedLog.Note
		case os.IsNotExist(callLogErr):
			m.CallLogNote = "no call log at " + callLogPath + " — nothing recorded yet; traffic figures are not measured, never zero"
		default:
			m.CallLogNote = "call log unreadable: " + callLogErr.Error() + " — traffic figures are not measured, never zero"
		}
		m.Misses = buildMissModel(entries)

		// Ledger sessions per repo (mirrors fleetreport.go's own fold): the
		// index-only distinction needs to know whether ANYONE runs coding
		// sessions in a service's repo, separately from whether it is
		// queried (CallsTotal) — a remote corpus can be queried with zero
		// sessions of its own.
		// Sessions are distinct sessions (console.DistinctSessionsByRepo,
		// the rule every page shares), from the ledger bytes this build
		// already read for the shared metric.
		// A row's repo label maps to the service whose root it names.
		roots := make(map[string]string, len(svcs))
		for name, info := range svcs {
			roots[name] = info.path
		}
		labelKey := sessionLabelKey(roots)
		sessionsByRepo := map[string]int{}
		ledgerReadable := ledgerErr == nil
		// The realization ladder's window (docs/design/realization-grade.md
		// §2): the same fourteen UTC days the traffic sparkline draws, for
		// every rung — calls, obligations injections, sessions and gate
		// runs in the window, never ever.
		winFrom, winTo := realizationWindowBounds(now)
		winFromDay, winToDay := winFrom.Format("2006-01-02"), winTo.Format("2006-01-02")
		windowSessions := map[string]int{}
		if ledgerReadable {
			sessionsByRepo = console.DistinctSessionsBy(ledgerData, labelKey)
			windowSessions = console.DistinctSessionsIn(ledgerData, labelKey, winFrom, winTo)
		}
		// The gating rung reads the verify ledger (SD-1): every `fathom
		// verify` run appends to it, a pre-push hook's included — the
		// session ledger's verify_catches only ever saw runs inside an agent
		// session, which told repositories that gate on every push to add
		// the gate (O30's failure, again).
		verifyRuns, verifyLedgerNote := windowVerifyRuns(winFrom, winTo)

		obligationsBySvc := map[string]int{}
		windowObligations := map[string]int{}
		compileBySvc := fetchCompileStates(hc, baseURL)
		obligationCalls := 0
		byDay := map[string]int{}
		svcByDay := map[string]map[string]int{}
		for _, e := range entries {
			if e.Tool == "obligations" {
				obligationCalls++
				if e.Service != "" {
					obligationsBySvc[strings.ToLower(e.Service)]++
					if inRealizationWindow(e.TS, winFromDay, winToDay) {
						windowObligations[strings.ToLower(e.Service)]++
					}
				}
			}
			if len(e.TS) >= 10 {
				byDay[e.TS[:10]]++
				if e.Service != "" {
					k := strings.ToLower(e.Service)
					if svcByDay[k] == nil {
						svcByDay[k] = map[string]int{}
					}
					svcByDay[k][e.TS[:10]]++
				}
			}
			if e.Service == "" {
				continue
			}
			k := strings.ToLower(e.Service)
			tr := perSvc[k]
			if tr == nil {
				tr = &struct {
					total, agent int
					last         string
				}{}
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

		names := make([]string, 0, len(svcs))
		for name := range svcs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			info := svcs[name]
			row := obsServiceRow{fleetServiceRow: fleetServiceRow{Service: name, Path: info.path, Symbols: info.symbols}, languages: splitLangs(info.langs)}
			row.Freshness, row.IndexedCommit, row.RepoHead = fleetFreshness(hc, baseURL, name, info.path)
			if strings.HasPrefix(row.Freshness, "stale") {
				row.FreshnessRemedy = daemonRefreshRemedy(info.path, name)
			}
			row.HooksNote, row.StanzaNote = fleetWiringNotes(info.path)
			if strings.Contains(row.HooksNote, "no fathom hooks") || strings.Contains(row.HooksNote, "absent") {
				row.WiringRemedy = "fathom fleet reconcile --apply (or: cd " + info.path + " && fathom init)"
			}
			row.OpenObservations, row.ObservationsError = serviceOpenObservations(info.path)
			if tr := perSvc[name]; tr != nil {
				row.CallsTotal, row.CallsAgent, row.LastCallTS = tr.total, tr.agent, tr.last
			}
			if ledgerReadable {
				n := sessionsByRepo[name]
				row.SessionCount = &n
			}
			if days := svcByDay[name]; days != nil {
				for i := 13; i >= 0; i-- {
					d := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
					row.TrafficDays = append(row.TrafficDays, days[d])
				}
			}
			row.Grade, row.GradeRaise = wiringGrade(row)
			// Phase Y: the REALIZATION ladder alongside the wiring grade.
			// The window is the same 14 days the traffic sparkline uses,
			// so the letter and the chart can never disagree.
			win := realizationWindow{
				Calls:            sumInts(row.TrafficDays),
				ObligationServes: windowObligations[name],
				Sessions:         windowSessions[name],
				VerifyRuns:       verifyRuns[name],
				WindowDays:       realizationWindowDays,
				CallLogNote:      m.CallLogNote,
				VerifyLedgerNote: verifyLedgerNote,
			}
			win.HasPlans, win.PlansNote = repoPlansRead(info.path)
			if !ledgerReadable {
				win.SessionsNote = "the session ledger was not read (" + ledgerPath + ")"
			}
			ladder := console.RealizationLadder(realizationLadderFacts(row, win))
			rr := realizationReportFrom(ladder)
			row.Realization, row.ladder = &rr, &ladder
			if callLogRead {
				row.Series = dimensionSeries(entries, name, now, seriesDays)
			}
			cp := buildConfigPanel(info.path, obligationsBySvc[name])
			row.Config = &cp
			if note, ok := compileBySvc[name]; ok && note != "" {
				row.CompileNote = note
			} else {
				row.CompileNote = "no compile recorded for this service — if every service shows this, the daemon has no LLM key configured (obligations then serve staleness warnings, not compiled rules)"
			}
			m.Services = append(m.Services, row)
		}
		if callLogRead {
			for i := 13; i >= 0; i-- {
				d := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
				m.TrafficDays = append(m.TrafficDays, obsTrafficDay{Date: d, Calls: byDay[d]})
			}
		}
		// RG-4: ordered by UNREALIZED VALUE, not by traffic. Sorting by
		// call count answers "who is busy", which nobody needs to be told;
		// this answers "where should I spend the next ten minutes". Ties
		// break on sessions then name, so the order is stable (house
		// contract). A row whose ladder could not be built falls back to
		// the traffic key rather than being dropped.
		sort.SliceStable(m.Services, func(i, j int) bool {
			a, b := m.Services[i], m.Services[j]
			ua, ub := unrealizedScore(a), unrealizedScore(b)
			if ua != ub {
				return ua > ub
			}
			if a.CallsTotal != b.CallsTotal {
				return a.CallsTotal > b.CallsTotal
			}
			return a.Service < b.Service
		})
		m.AgentSurfaceTools, m.AgentSchemaBytes = surfaceSchemaTax(hc, baseURL)
		m.Impact = buildImpactModel(now)
		m.Impact.ObligationCalls = obligationCalls
		m.Cost = observatoryCostPanel(entries, m.AgentSurfaceTools, m.AgentSchemaBytes, m.Impact.DiscoveryTotal)
		return m
	}

	m.Impact = buildImpactModel(now)
	return m
}

// buildImpactModel folds the session ledger into the wins model. Nil-aware
// throughout: a count sums only rows where the instrument ran, and its note
// always states the denominator (OB-6, law 3/12).
func buildImpactModel(now time.Time) obsImpact {
	imp := obsImpact{}
	path := sessionLedgerPath()
	data, err := os.ReadFile(path)
	if err != nil {
		imp.LedgerNote = "session ledger not readable at " + path + " — impact figures are not measured, never zero"
		imp.VerifyCatchesNote = "no ledger — verify catches unknown"
		imp.DiscoveryNote = "no ledger — discovery actions unknown"
		imp.PilotState = "idle"
		imp.PilotDetail = "no ledger rows; the pilot starts with fathom arm off --sessions N after pre-registering criteria (law 12)"
		imp.StoriesNote = "no story rows read"
		return imp
	}

	var (
		catchesSum, catchesRan      int
		discoverySum, discoveryRan  int
		onTokens, offTokens         []int
		preSplitRows                int
		cacheReadSum, cacheReadRows int
		cacheReadTruncatedExcluded  int
		onSessions, offSessions     int
		uncleanArmRows              int
		firstAt, lastAt             string
		firstEditTurns              []int
		expRatios                   []int
	)
	// ONE ROW PER SESSION, not one per stop (2026-08-09 finding).
	//
	// Claude Code's Stop event fires at every TURN boundary, not at session
	// close, so a single long session appends a row each time the assistant
	// finishes speaking. Measured on the live ledger: 200 rows, 8 distinct
	// sessions — one resume_as_code session alone wrote 74 of them, several
	// with identical turn counts. Counting rows inflated every denominator
	// roughly 25x ("of 188 sessions" was of 8), and would have let the
	// pre-registered "≥20 sessions per arm" criterion be satisfied by about
	// one real session.
	//
	// Dedupe at READ time, keeping the LAST row per session id: the final
	// stop has seen the most of the transcript, so its turn/token figures
	// are the complete ones. Read-time rather than a rewrite, because the
	// ledger is append-only evidence and the superseded rows are history,
	// not garbage.
	latest := map[string]sessionLedgerEntry{}
	var order []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e sessionLedgerEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		key := e.SessionID
		if key == "" {
			// A row with no session id cannot be deduped, so it counts once
			// on its own terms rather than being dropped or merged with
			// every other anonymous row.
			key = "\x00anon:" + e.At + ":" + e.Repo
		}
		if _, seen := latest[key]; !seen {
			order = append(order, key)
		}
		latest[key] = e
	}
	imp.LedgerStops = len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))

	for _, key := range order {
		e := latest[key]
		imp.LedgerSessions++
		if e.At != "" {
			if firstAt == "" || e.At < firstAt {
				firstAt = e.At
			}
			if e.At > lastAt {
				lastAt = e.At
			}
		}
		if e.VerifyCatches != nil {
			catchesRan++
			catchesSum += *e.VerifyCatches
		}
		if e.DiscoveryActionsTotal != nil {
			discoveryRan++
			discoverySum += *e.DiscoveryActionsTotal
		}
		if e.FirstEditTurn != nil {
			firstEditTurns = append(firstEditTurns, *e.FirstEditTurn)
		}
		if e.RawDiscoveryTokensEst != nil || e.FathomTokensEst != nil {
			imp.ExpSessions++
			imp.ExpRawCalls += e.RawDiscoveryCalls
			imp.ExpFathomCalls += e.FathomCalls
			if e.RawDiscoveryTokensEst != nil {
				imp.ExpRawTokens += *e.RawDiscoveryTokensEst
			}
			if e.FathomTokensEst != nil {
				imp.ExpFathomTokens += *e.FathomTokensEst
			}
			imp.ExpRepeatFiles += e.RepeatedReadFiles
			if e.RepeatedReadTokensEst != nil {
				imp.ExpRepeatTokens += *e.RepeatedReadTokensEst
			}
			imp.ExpSidechainShare += e.SidechainToolResults
			imp.ExpToolSearchTurns += e.ToolSearchTurns
			if e.RawDiscoveryCalls > 0 && e.FathomCalls > 0 && e.RawDiscoveryTokensEst != nil && e.FathomTokensEst != nil && *e.FathomTokensEst > 0 {
				rawPerQ := float64(*e.RawDiscoveryTokensEst) / float64(e.RawDiscoveryCalls)
				idxPerQ := float64(*e.FathomTokensEst) / float64(e.FathomCalls)
				if idxPerQ > 0 {
					expRatios = append(expRatios, int(rawPerQ/idxPerQ*10))
				}
			}
		}
		// Spend-shaped tokens only (2026-08-08 finding): TokensTotal sums
		// ALL flow including cache reads, which dominate cached sessions by
		// orders of magnitude — a 596M-token "median session" was cache
		// reads. Arms compare in+out; rows predating the split are counted
		// as pre-split (unknown), never fed to the median as their inflated
		// totals.
		//
		// IT-12: a row whose transcript_truncated is true had its
		// tokens_cache_read frozen mid-parse by the 16MB LimitReader
		// (sessionledger.go) — a partial figure, not the session's true
		// cache-read total. Summing it alongside real totals would skew
		// this average silently; exclude it and count the exclusion.
		switch {
		case e.TokensCacheRead == nil:
			// no figure at all — neither summed nor excluded
		case e.TranscriptTruncated:
			cacheReadTruncatedExcluded++
		default:
			cacheReadSum += *e.TokensCacheRead
			cacheReadRows++
		}
		// Arm coverage gate (2026-08-08 corruption finding) + the amended
		// unit (production-tokenomics.md §5, 2026-08-12). A row whose
		// transcript began before the arm carries assisted activity under
		// an "off" label; admitting its WHOLE-SESSION figures manufactures
		// a false null. But such a session's ARM-COVERED WINDOW is clean
		// by construction, so a partial row with an untruncated window of
		// ≥ abMinTurnsPerUnit turns is admitted AS ITS WINDOW, at a
		// per-turn rate. Everything else stays excluded and NAMED.
		unit, ok := armUnitFor(e)
		switch {
		case ok && e.Arm == "off":
			offSessions++
			offTokens = append(offTokens, unit)
		case ok:
			onSessions++
			onTokens = append(onTokens, unit)
		case e.Arm == "off" || e.ArmCoverage == "partial" || e.ArmCoverage == "unknown":
			// An off-row (or contaminated row) that yielded no unit:
			// counted, never silently dropped and never silently used.
			uncleanArmRows++
		default:
			// An on-row with no usable token figures.
			if e.TokensTotal != nil && (e.TokensIn == nil || e.TokensOut == nil) {
				preSplitRows++
			}
		}
	}
	if firstAt != "" {
		imp.DateRange = firstAt[:min(10, len(firstAt))] + " … " + lastAt[:min(10, len(lastAt))]
	}
	if catchesRan > 0 {
		imp.VerifyCatchesTotal = &catchesSum
		imp.VerifyCatchesNote = "verify ran in " + itoa(catchesRan) + " of " + itoa(imp.LedgerSessions) + " ledger sessions"
	} else {
		imp.VerifyCatchesNote = "verify never ran in any ledger session — unknown, not zero"
	}
	if discoveryRan > 0 {
		imp.DiscoveryTotal = &discoverySum
		imp.DiscoveryNote = "instrumented in " + itoa(discoveryRan) + " of " + itoa(imp.LedgerSessions) + " ledger sessions"
	} else {
		imp.DiscoveryNote = "discovery instrument absent from every ledger session — unknown, not zero"
	}
	// Join the self-ratings by session id (rate.go). Join-at-read, so a
	// rating given after a session ended still counts.
	ratings := loadRatings(ratingsPath())
	var onR, offR []int
	for _, key := range order {
		e := latest[key]
		r, ok := ratings[e.SessionID]
		if !ok || e.SessionID == "" {
			continue
		}
		if e.Arm == "off" {
			offR = append(offR, r.Rating)
		} else {
			onR = append(onR, r.Rating)
		}
	}
	imp.RatedOn, imp.RatedOff = len(onR), len(offR)
	if v, ok := medianInt(onR); ok {
		imp.RatingOnMed = &v
	}
	if v, ok := medianInt(offR); ok {
		imp.RatingOffMed = &v
	}
	imp.TokensOnSessions, imp.TokensOffSessions = onSessions, offSessions
	imp.UncleanArmRows = uncleanArmRows
	if uncleanArmRows > 0 {
		imp.UncleanArmNote = itoa(uncleanArmRows) + " OFF-arm session(s) excluded from the comparison: each either began " +
			"before the arm was set (so its totals include assisted activity) or predates coverage stamping (so its " +
			"coverage is unknown, which is not the same as clean). Counted here, never silently dropped."
	}
	// Since the 2026-08-12 unit amendment these medians are PER-TURN real
	// tokens (in+out over assistant turns of the same parsed span), not
	// per-session totals — per-session is length-confounded once units can
	// be windows of very different sizes.
	if v, ok := medianInt(onTokens); ok {
		imp.TokensOnMedian = &v
	}
	if v, ok := medianInt(offTokens); ok {
		imp.TokensOffMedian = &v
	}
	// DO-1/DO-2 (§4.1): the outcome half derives from disk — commits
	// landed in each unit's own beat-window. Same admissibility rule as
	// the token medians; ratings, when present, remain the human override
	// (DO-4).
	doBeats, _ := readRetroBeats(beatLogPath())
	var onC, offC []int
	for _, key := range order {
		e := latest[key]
		if e.SessionID == "" || e.Repo == "" {
			continue
		}
		if _, ok := armUnitFor(e); !ok {
			continue
		}
		c, ok := derivedCommitsForSession(doBeats, e.SessionID, e.Repo)
		if !ok {
			continue
		}
		if e.Arm == "off" {
			offC = append(offC, c)
		} else {
			onC = append(onC, c)
		}
	}
	imp.DerivedOnUnits, imp.DerivedOffUnits = len(onC), len(offC)
	if v, ok := medianInt(onC); ok {
		imp.DerivedOnMed = &v
	}
	if v, ok := medianInt(offC); ok {
		imp.DerivedOffMed = &v
	}

	// LAST: the criterion reads the session counts and both medians, so it
	// must run after every one of them is assigned.
	imp.CriterionNote = evaluateABCriterion(imp)
	if preSplitRows > 0 {
		imp.TokensPreSplitRows = &preSplitRows
	}
	if cacheReadRows > 0 {
		avg := cacheReadSum / cacheReadRows
		imp.CacheReadMedianCtx = &avg
	}
	if cacheReadTruncatedExcluded > 0 {
		imp.CacheReadFrozenExcluded = &cacheReadTruncatedExcluded
	}
	switch {
	case offSessions == 0:
		imp.PilotState = "idle"
		imp.PilotDetail = "no OFF-arm sessions recorded — a savings figure would violate law 12; start with fathom arm off --sessions N after pre-registering criteria"
	case len(onTokens) == 0 || len(offTokens) == 0:
		imp.PilotState = "collecting"
		imp.PilotDetail = "arms exist but token totals are missing on one side — the range publishes only when both arms carry real tokens"
	default:
		imp.PilotState = "collecting"
		imp.PilotDetail = "medians shown per arm with session counts; publish a RANGE with denominators once the pre-registered N per arm is met"
	}
	if v, ok := medianInt(expRatios); ok {
		imp.ExpRatioMedianX10 = &v
		imp.ExpNote = "OBSERVATIONAL: per-question token cost, raw exploration vs indexed discovery, same estimator, " + itoa(len(expRatios)) + " sessions with both modes; the counterfactual arm carries causality (law 12)"
	} else if imp.ExpSessions > 0 {
		imp.ExpNote = "exploration measured in " + itoa(imp.ExpSessions) + " sessions, but no session used both modes yet - the ratio needs both"
	} else {
		imp.ExpNote = "no sessions carry the exploration ledger yet (rows predate EX-1) - unknown, not zero"
	}
	if v, ok := medianInt(firstEditTurns); ok {
		imp.FirstEditMedian = &v
		imp.FirstEditNote = "median across " + itoa(len(firstEditTurns)) + " sessions with a recorded first edit"
	} else {
		imp.FirstEditNote = "no sessions carry first_edit_turn yet (rows predate IQ-2) — unknown, not zero"
	}
	imp.StoriesNote = "no story rows quoted here; narrate captures them per-repo while the dial is on"
	return imp
}

func medianInt(vals []int) (int, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	s := append([]int{}, vals...)
	sort.Ints(s)
	return s[len(s)/2], true
}

func itoa(n int) string { return strconv.Itoa(n) }

func armBannerDetail(st armState) string {
	d := "measurement arm OFF is ACTIVE — channels disabled for the counterfactual"
	if st.SessionsRemaining != nil {
		d += " (" + strconv.Itoa(*st.SessionsRemaining) + " sessions remaining)"
	}
	if st.Until != "" {
		d += " (until " + st.Until + ")"
	}
	if st.Note != "" {
		d += " — " + st.Note
	}
	return d
}

func narrateBannerDetail(st narrateState) string {
	d := "narrate is ON (story rows captured)"
	if st.SessionsRemaining != nil {
		d += " (" + strconv.Itoa(*st.SessionsRemaining) + " sessions remaining)"
	}
	if st.Until != "" {
		d += " (expires " + st.Until + ")"
	}
	return d
}

// ─── cache ────────────────────────────────────────────────────────────────

// obsCache keeps one built model for a short TTL: the fleet fold walks
// every service's repo, and a browser refresh should not re-walk the fleet.
var obsCache struct {
	sync.Mutex
	model observatoryModel
	at    time.Time
}

const obsCacheTTL = 30 * time.Second

func cachedObservatoryModel(baseURL string, now time.Time) observatoryModel {
	obsCache.Lock()
	defer obsCache.Unlock()
	if !obsCache.at.IsZero() && now.Sub(obsCache.at) < obsCacheTTL {
		return obsCache.model
	}
	m := buildObservatoryModel(baseURL, now)
	obsCache.model, obsCache.at = m, now
	return m
}

// ─── IQ-3: the miss fold ─────────────────────────────────────────────────

// obsMissRow is one tool's outcome profile from the call log.
type obsMissRow struct {
	Tool        string `json:"tool"`
	Calls       int    `json:"calls"`
	Hits        int    `json:"hits"`
	Misses      int    `json:"misses"`
	DidYouMean  int    `json:"did_you_mean"`
	NotRecorded int    `json:"not_recorded"`
}

// buildMissModel folds call outcomes per tool, most-called first. Rows
// logged before IQ-1 count as NotRecorded — never as hits (law 3).
func buildMissModel(entries []adoptionCallEntry) []obsMissRow {
	byTool := map[string]*obsMissRow{}
	for _, e := range entries {
		if e.Tool == "" {
			continue
		}
		r := byTool[e.Tool]
		if r == nil {
			r = &obsMissRow{Tool: e.Tool}
			byTool[e.Tool] = r
		}
		r.Calls++
		switch {
		case e.ResultCount == nil:
			r.NotRecorded++
		case e.MatchedVia == "did_you_mean":
			r.DidYouMean++
		case *e.ResultCount == 0:
			r.Misses++
		default:
			r.Hits++
		}
	}
	rows := make([]obsMissRow, 0, len(byTool))
	for _, r := range byTool {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Calls != rows[j].Calls {
			return rows[i].Calls > rows[j].Calls
		}
		return rows[i].Tool < rows[j].Tool
	})
	return rows
}

// ─── IQ-15: the wiring grade ─────────────────────────────────────────────

// wiringGrade folds a service row's doctor-class signals to one letter and
// the ONE command that raises it — the continuous form of the rewire audit.
// Signals: hooks referenced in project settings, a CLAUDE.md stanza, a
// fresh index, and any recorded traffic.
func wiringGrade(s obsServiceRow) (grade, raise string) {
	score, visible := 0, 0
	hooksVisible := !strings.Contains(s.HooksNote, "user-scope or plugin wiring not visible") && !strings.Contains(s.HooksNote, "not checkable")
	hooksOK := strings.Contains(s.HooksNote, "hook reference")
	if hooksVisible {
		visible++
		if hooksOK {
			score++
		}
	}
	stanzaOK := s.StanzaNote == "present"
	visible++
	if stanzaOK {
		score++
	}
	freshOK := s.Freshness == "fresh"
	visible++
	if freshOK {
		score++
	}
	trafficOK := s.CallsTotal > 0
	visible++
	if trafficOK {
		score++
	}
	frac := float64(score) / float64(visible)
	switch {
	case frac >= 1:
		grade = "A"
	case frac >= 0.75:
		grade = "B"
	case frac >= 0.5:
		grade = "C"
	case frac >= 0.25:
		grade = "D"
	default:
		grade = "F"
	}
	switch {
	case (hooksVisible && !hooksOK) || !stanzaOK:
		raise = "fathom fleet reconcile --apply (or: cd " + s.Path + " && fathom init)"
	case !freshOK:
		raise = daemonRefreshRemedy(s.Path, s.Service)
	case !trafficOK:
		raise = "wired but unused — open a session in " + s.Path + " and ask a where/who-calls question"
	}
	return grade, raise
}

// surfaceSchemaTax measures the agent surface's standing context cost
// (TD-1, docs/plans/exploration-ledger.md): tool count and serialized
// schema bytes from POST /rpc/agent tools/list — the bytes every
// MCP-connected session carries before any work happens. Zeros on any
// failure; the renderer names the absence (law 3), never charts a zero.
func surfaceSchemaTax(hc *http.Client, baseURL string) (int, int) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/rpc/agent", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if err != nil {
		return 0, 0
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, 0
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if err != nil {
		return 0, 0
	}
	var env struct {
		Result struct {
			Tools json.RawMessage `json:"tools"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &env) != nil || len(env.Result.Tools) == 0 {
		return 0, 0
	}
	var tools []json.RawMessage
	if json.Unmarshal(env.Result.Tools, &tools) != nil {
		return 0, 0
	}
	return len(tools), len(env.Result.Tools)
}

// fetchCompileStates maps service → a one-line summary of its most recent
// compile per layer, from the daemon's own statusz JSON. Empty map when
// unreachable — the caller renders the honest default.
func fetchCompileStates(hc *http.Client, baseURL string) map[string]string {
	out := map[string]string{}
	resp, err := hc.Get(baseURL + "/statusz?format=json")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var rep struct {
		Services []struct {
			Service             string            `json:"service"`
			LastCompilePerLayer map[string]string `json:"last_compile_per_layer"`
		} `json:"services"`
	}
	if json.NewDecoder(resp.Body).Decode(&rep) != nil {
		return out
	}
	for _, s := range rep.Services {
		if len(s.LastCompilePerLayer) == 0 {
			continue
		}
		parts := make([]string, 0, len(s.LastCompilePerLayer))
		for layer, sum := range s.LastCompilePerLayer {
			parts = append(parts, layer+": "+sum)
		}
		sort.Strings(parts)
		out[strings.ToLower(s.Service)] = strings.Join(parts, " · ")
	}
	return out
}

// abMinTurnsPerUnit is the amended admissibility floor (production-
// tokenomics.md §5, 2026-08-12): an arm-homogeneous unit below this many
// assistant turns is noise, not a sample.
const abMinTurnsPerUnit = 10

// armUnitFor extracts one arm-homogeneous unit from a ledger row as a
// PER-TURN real-token rate, or reports that the row yields none.
//
//   - full coverage (or an on-row, which has no window to fall outside):
//     the whole session is the unit — (in+out)/turns over the same parsed
//     span, so head-truncation yields a coherent rate rather than an
//     undercounted total;
//   - partial coverage with an untruncated arm window: the WINDOW is the
//     unit, same rate over the window's own span;
//   - anything else: no unit. The caller counts it, named, per law 3.
func armUnitFor(e sessionLedgerEntry) (int, bool) {
	rate := func(in, out, turns *int) (int, bool) {
		if in == nil || out == nil || turns == nil || *turns < abMinTurnsPerUnit {
			return 0, false
		}
		return (*in + *out) / *turns, true
	}
	if e.Arm != "off" || e.ArmCoverage == "full" {
		if e.Arm == "off" && e.ArmCoverage != "full" {
			return 0, false
		}
		return rate(e.TokensIn, e.TokensOut, e.TurnCount)
	}
	// off + not-full: only a clean window can speak for this row.
	w := e.ArmWindow
	if w == nil || w.Truncated {
		return 0, false
	}
	return rate(w.TokensIn, w.TokensOut, w.Turns)
}

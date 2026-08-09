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
	RatedOn      int  `json:"rated_on_sessions"`
	RatedOff     int  `json:"rated_off_sessions"`
	RatingOnMed  *int `json:"rating_on_median,omitempty"`
	RatingOffMed *int `json:"rating_off_median,omitempty"`
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
	TokensPreSplitRows *int   `json:"tokens_pre_split_rows,omitempty"`
	CacheReadMedianCtx *int   `json:"cache_read_median_ctx,omitempty"`
	PilotState         string `json:"pilot_state"`
	PilotDetail        string `json:"pilot_detail,omitempty"`

	Stories     []string `json:"stories,omitempty"`
	StoriesNote string   `json:"stories_note"`

	// Compounding (Q4, IQ-7): warm-up length and rules-served proxy.
	FirstEditMedian *int   `json:"first_edit_median_turns,omitempty"`
	FirstEditNote   string `json:"first_edit_note"`
	ObligationCalls int    `json:"obligation_calls"`

	// The exploration story (EX-3): folded across sessions carrying the
	// exploration ledger. Per-question = tokens / calls, per mode; the
	// ratio is OBSERVATIONAL (labeled) — the arm pilot carries causality.
	ExpSessions       int    `json:"exp_sessions"`
	ExpRawCalls       int    `json:"exp_raw_calls"`
	ExpRawTokens      int    `json:"exp_raw_tokens_est"`
	ExpFathomCalls    int    `json:"exp_fathom_calls"`
	ExpFathomTokens   int    `json:"exp_fathom_tokens_est"`
	ExpRatioMedianX10 *int   `json:"exp_ratio_median_x10,omitempty"` // per-question raw/indexed ×10
	ExpRepeatFiles    int    `json:"exp_repeat_files"`
	ExpRepeatTokens   int    `json:"exp_repeat_tokens_est"`
	ExpSidechainShare int    `json:"exp_sidechain_results"`
	ExpNote           string `json:"exp_note"`
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

	// Misses (IQ-3): per-tool outcome profile from the call log.
	Misses []obsMissRow `json:"misses,omitempty"`

	// SurfaceTax (TD-1): the standing context cost of the agent surface —
	// schema bytes every MCP-connected session carries before any work.
	AgentSurfaceTools int `json:"agent_surface_tools"`
	AgentSchemaBytes  int `json:"agent_schema_bytes"`

	Impact obsImpact `json:"impact"`
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

	hc := &http.Client{Timeout: 5 * time.Second}
	svcs, err := fetchServices(hc, baseURL)
	if err != nil {
		m.DegradedNote = "service registry unreachable (" + err.Error() + ") — fleet tiles are not measured, never empty-as-healthy"
	} else {
		// Traffic fold, same resolution the adoption gauge uses.
		perSvc := map[string]*struct {
			total, agent int
			last         string
		}{}
		callLogPath := resolveCallLogPath()
		if _, statErr := os.Stat(callLogPath); statErr != nil {
			for _, cand := range resolveCallLogCandidates(hc, baseURL, true, ".") {
				if _, e := os.Stat(cand); e == nil {
					callLogPath = cand
					break
				}
			}
		}
		entries, _, _ := readCallLog(callLogPath)
		m.Misses = buildMissModel(entries)

		// Ledger sessions per repo (mirrors fleetreport.go's own fold): the
		// index-only distinction needs to know whether ANYONE runs coding
		// sessions in a service's repo, separately from whether it is
		// queried (CallsTotal) — a remote corpus can be queried with zero
		// sessions of its own.
		sessionsByRepo := map[string]int{}
		verifyRunsByRepo := map[string]int{}
		ledgerReadable := false
		if data, lerr := os.ReadFile(sessionLedgerPath()); lerr == nil {
			ledgerReadable = true
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				var row struct {
					Repo          string `json:"repo"`
					VerifyCatches *int   `json:"verify_catches"`
				}
				if json.Unmarshal([]byte(line), &row) == nil && row.Repo != "" {
					sessionsByRepo[strings.ToLower(row.Repo)]++
					// verify_catches is nil when the gate never RAN, so a
					// non-nil value (including 0) is a run. Counting a nil
					// as a run would report a gate that never fired as
					// gating (law 3).
					if row.VerifyCatches != nil {
						verifyRunsByRepo[strings.ToLower(row.Repo)]++
					}
				}
			}
		}

		obligationsBySvc := map[string]int{}
		compileBySvc := fetchCompileStates(hc, baseURL)
		obligationCalls := 0
		byDay := map[string]int{}
		svcByDay := map[string]map[string]int{}
		for _, e := range entries {
			if e.Tool == "obligations" {
				obligationCalls++
				if e.Service != "" {
					obligationsBySvc[strings.ToLower(e.Service)]++
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
			row := obsServiceRow{fleetServiceRow: fleetServiceRow{Service: name, Path: info.path, Symbols: info.symbols}}
			row.Freshness, row.IndexedCommit, row.RepoHead = fleetFreshness(hc, baseURL, name, info.path)
			if strings.HasPrefix(row.Freshness, "stale") {
				row.FreshnessRemedy = "cd " + info.path + " && fathom ingest --service " + name
			}
			row.HooksNote, row.StanzaNote = fleetWiringNotes(info.path)
			if strings.Contains(row.HooksNote, "no fathom hooks") || strings.Contains(row.HooksNote, "absent") {
				row.WiringRemedy = "fathom fleet reconcile --apply (or: cd " + info.path + " && fathom init)"
			}
			if info.path != "" {
				if content, existed, oerr := readObservations(info.path); oerr == nil && existed {
					n := countOpenObservations(content)
					row.OpenObservations = &n
				}
			}
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
				ObligationServes: obligationsBySvc[name],
				VerifyRuns:       verifyRunsByRepo[name],
				HasPlans:         repoHasPlans(info.path),
			}
			if row.SessionCount != nil {
				win.Sessions = *row.SessionCount
			}
			rr := realizationGrade(row, win)
			row.Realization = &rr
			cp := buildConfigPanel(info.path, obligationsBySvc[name])
			row.Config = &cp
			if note, ok := compileBySvc[name]; ok && note != "" {
				row.CompileNote = note
			} else {
				row.CompileNote = "no compile recorded for this service — if every service shows this, the daemon has no LLM key configured (obligations then serve staleness warnings, not compiled rules)"
			}
			m.Services = append(m.Services, row)
		}
		for i := 13; i >= 0; i-- {
			d := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
			m.TrafficDays = append(m.TrafficDays, obsTrafficDay{Date: d, Calls: byDay[d]})
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
		spend := -1
		if e.TokensIn != nil && e.TokensOut != nil {
			spend = *e.TokensIn + *e.TokensOut
		}
		if e.TokensCacheRead != nil {
			cacheReadSum += *e.TokensCacheRead
			cacheReadRows++
		}
		// Arm coverage gate (2026-08-08 corruption finding): a row whose
		// transcript BEGAN before the arm was set carries mostly assisted
		// activity while wearing an "off" label. Admitting it to the
		// paired comparison drags the OFF arm toward the ON arm and
		// manufactures a false null result — the one conclusion that would
		// end the program, on mislabelled evidence. Such rows are counted
		// and NAMED, never silently dropped and never silently used.
		if e.ArmCoverage == "partial" || e.ArmCoverage == "unknown" {
			uncleanArmRows++
			continue
		}
		// A row stamped "off" with NO coverage field was written by a
		// binary that never checked — so its coverage is unknown, and
		// unknown is not clean (law 3). This is asymmetric on purpose:
		// "off" names a bounded window a session can fall outside of,
		// while "on" is ordinary operation with no window to be inside,
		// so an on-row needs no stamp.
		if e.Arm == "off" && e.ArmCoverage == "" {
			uncleanArmRows++
			continue
		}
		switch e.Arm {
		case "off":
			offSessions++
			if spend >= 0 {
				offTokens = append(offTokens, spend)
			} else if e.TokensTotal != nil {
				preSplitRows++
			}
		default:
			onSessions++
			if spend >= 0 {
				onTokens = append(onTokens, spend)
			} else if e.TokensTotal != nil {
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
	if v, ok := medianInt(onTokens); ok {
		imp.TokensOnMedian = &v
	}
	if v, ok := medianInt(offTokens); ok {
		imp.TokensOffMedian = &v
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
		raise = "cd " + s.Path + " && fathom ingest --service " + s.Service
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

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
}

// obsArmBanner / obsNarrateBanner surface the active dials loudly.
type obsArmBanner struct {
	Active bool   `json:"active"`
	Detail string `json:"detail"`
}

// obsImpact is the wins model: every count carries its denominator and its
// note; a nil pointer with a note is an absent instrument, never a zero.
type obsImpact struct {
	LedgerSessions int    `json:"ledger_sessions"`
	LedgerNote     string `json:"ledger_note,omitempty"`
	DateRange      string `json:"date_range,omitempty"`

	VerifyCatchesTotal *int   `json:"verify_catches_total,omitempty"`
	VerifyCatchesNote  string `json:"verify_catches_note"`

	DiscoveryTotal *int   `json:"discovery_actions_total,omitempty"`
	DiscoveryNote  string `json:"discovery_note"`

	TokensOnSessions  int    `json:"tokens_on_sessions"`
	TokensOffSessions int    `json:"tokens_off_sessions"`
	TokensOnMedian    *int   `json:"tokens_on_median,omitempty"`
	TokensOffMedian   *int   `json:"tokens_off_median,omitempty"`
	PilotState        string `json:"pilot_state"`
	PilotDetail       string `json:"pilot_detail,omitempty"`

	Stories     []string `json:"stories,omitempty"`
	StoriesNote string   `json:"stories_note"`

	// Compounding (Q4, IQ-7): warm-up length and rules-served proxy.
	FirstEditMedian *int   `json:"first_edit_median_turns,omitempty"`
	FirstEditNote   string `json:"first_edit_note"`
	ObligationCalls int    `json:"obligation_calls"`
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
		obligationCalls := 0
		byDay := map[string]int{}
		svcByDay := map[string]map[string]int{}
		for _, e := range entries {
			if e.Tool == "obligations" {
				obligationCalls++
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
			if days := svcByDay[name]; days != nil {
				for i := 13; i >= 0; i-- {
					d := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
					row.TrafficDays = append(row.TrafficDays, days[d])
				}
			}
			row.Grade, row.GradeRaise = wiringGrade(row)
			m.Services = append(m.Services, row)
		}
		for i := 13; i >= 0; i-- {
			d := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
			m.TrafficDays = append(m.TrafficDays, obsTrafficDay{Date: d, Calls: byDay[d]})
		}
		// IQ-8: most active first; name breaks ties so the order is stable.
		sort.SliceStable(m.Services, func(i, j int) bool {
			if m.Services[i].CallsTotal != m.Services[j].CallsTotal {
				return m.Services[i].CallsTotal > m.Services[j].CallsTotal
			}
			return m.Services[i].Service < m.Services[j].Service
		})
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
		catchesSum, catchesRan     int
		discoverySum, discoveryRan int
		onTokens, offTokens        []int
		onSessions, offSessions    int
		firstAt, lastAt            string
		firstEditTurns             []int
	)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e sessionLedgerEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
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
		switch e.Arm {
		case "off":
			offSessions++
			if e.TokensTotal != nil {
				offTokens = append(offTokens, *e.TokensTotal)
			}
		default:
			onSessions++
			if e.TokensTotal != nil {
				onTokens = append(onTokens, *e.TokensTotal)
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
	imp.TokensOnSessions, imp.TokensOffSessions = onSessions, offSessions
	if v, ok := medianInt(onTokens); ok {
		imp.TokensOnMedian = &v
	}
	if v, ok := medianInt(offTokens); ok {
		imp.TokensOffMedian = &v
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
	score := 0
	hooksOK := strings.Contains(s.HooksNote, "hook reference")
	if hooksOK {
		score++
	}
	stanzaOK := s.StanzaNote == "present"
	if stanzaOK {
		score++
	}
	freshOK := s.Freshness == "fresh"
	if freshOK {
		score++
	}
	trafficOK := s.CallsTotal > 0
	if trafficOK {
		score++
	}
	grade = [5]string{"F", "D", "C", "B", "A"}[score]
	switch {
	case !hooksOK || !stanzaOK:
		raise = "fathom fleet reconcile --apply (or: cd " + s.Path + " && fathom init)"
	case !freshOK:
		raise = "cd " + s.Path + " && fathom ingest --service " + s.Service
	case !trafficOK:
		raise = "wired but unused — open a session in " + s.Path + " and ask a where/who-calls question"
	}
	return grade, raise
}

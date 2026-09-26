package console

// fleet.go — OC-19 and OC-20 (vision 35 §3 Fleet, design §3): the fleet
// as one sortable, filterable table of every project, rolled up by health,
// with calls-per-day sparklines on one shared scale.
//
// Two honesty rules shape it:
//
//   - Unknown is never green. A project whose index freshness nobody can
//     determine, or whose plans have no recorded verify run, is "unknown" —
//     never "healthy". Known problems outrank unknowns: a project failing
//     its gate is "failed" whatever else is unknown about it.
//   - A sparkline never draws across an absent day. A day the call log holds
//     no row at all is absent for every project (the daemon may have been
//     down; nobody can tell), while a day with rows elsewhere and none here
//     is a measured zero. Every row shares one scale — the fleet's busiest
//     measured day — so a quiet project looks quiet.
//
// Sort and filter state are URL parameters (FleetQuery), so every view is a
// real route and back returns to it.

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Health is a project's rollup state.
type Health string

const (
	HealthFailed   Health = "failed"
	HealthDegraded Health = "degraded"
	HealthUnknown  Health = "unknown"
	HealthHealthy  Health = "healthy"
)

// HealthOrder lists the states worst first.
var HealthOrder = []Health{HealthFailed, HealthDegraded, HealthUnknown, HealthHealthy}

// Rank is the state's position in HealthOrder (0 is worst).
func (h Health) Rank() int {
	for i, x := range HealthOrder {
		if x == h {
			return i
		}
	}
	return len(HealthOrder)
}

// SparkPoint is one window day of one project's traffic. Calls is nil on
// an absent day, and Absent then says why.
type SparkPoint struct {
	Date   string `json:"date"`
	Calls  *int   `json:"calls,omitempty"`
	Absent string `json:"absent,omitempty"`
}

// Spark is one project's calls per day on the fleet's shared scale.
type Spark struct {
	Points []SparkPoint `json:"points"`
	// Max is the shared scale's top: the fleet's busiest measured day, the
	// same for every row.
	Max int `json:"max"`
	// Runs are the index ranges [from, to] of consecutive measured days. A
	// line is drawn within a run and never between two.
	Runs     [][2]int `json:"runs"`
	Measured int      `json:"measured_days"`
	Absent   int      `json:"absent_days"`
}

// FleetRow is one project in the table.
type FleetRow struct {
	Service   string `json:"service"`
	Href      string `json:"href"`
	Path      string `json:"path,omitempty"`
	Health    Health `json:"health"`
	HealthWhy string `json:"health_why"`
	// Symbols is the index's symbol count, absent with the reason when no
	// store read reported one.
	Symbols   Value[int] `json:"symbols"`
	Languages []string   `json:"languages,omitempty"`
	// Freshness is fresh, stale or unknown; FreshnessNote carries the full
	// reading ("stale: indexed … vs HEAD …", "unknown: …").
	Freshness     string `json:"freshness"`
	FreshnessNote string `json:"freshness_note,omitempty"`
	Ladder        []Rung `json:"ladder"`
	LadderMet     int    `json:"ladder_met"`
	LadderOf      int    `json:"ladder_applicable"`
	Grade         string `json:"grade,omitempty"`
	// PlansVerified is the claimed rows the newest recorded runs proved;
	// Plans the full totals behind it.
	PlansVerified Value[int] `json:"plans_verified"`
	Plans         *PlanProof `json:"plans,omitempty"`
	ClaimsFresh   Value[int] `json:"claims_fresh"`
	ClaimsTotal   int        `json:"claims_total"`
	CertsLive     Value[int] `json:"certificates_live"`
	CertsDeclared int        `json:"certificates_declared"`
	Calls         Value[int] `json:"calls_window"`
	Series        Spark      `json:"series"`
	Observations  Value[int] `json:"open_observations"`
}

// HealthGroup is one state of the rollup.
type HealthGroup struct {
	Health   Health   `json:"health"`
	Count    int      `json:"count"`
	Services []string `json:"services"`
}

// FleetQuery is the table's URL state.
type FleetQuery struct {
	Sort   string `json:"sort"`
	Dir    string `json:"dir"`
	Health string `json:"health,omitempty"`
	Q      string `json:"q,omitempty"`
	Lang   string `json:"lang,omitempty"`
	Page   int    `json:"page"`
	// PerPage bounds one page (default 50); Limit, when set, keeps only
	// the first rows after sorting (the Now page's pulse).
	PerPage int `json:"per_page"`
	Limit   int `json:"limit,omitempty"`
}

// FleetSort is one sortable column.
type FleetSort struct {
	Key, Label string
	// Desc is the key's natural first direction.
	Desc bool
}

// FleetSorts are the sortable columns in table order.
var FleetSorts = []FleetSort{
	{"service", "Project", false},
	{"health", "Health", false},
	{"symbols", "Symbols", true},
	{"freshness", "Index", false},
	{"realization", "Realization", true},
	{"plans", "Plans proven", true},
	{"claims", "Claims fresh", true},
	{"certs", "Certificates", true},
	{"calls", "Calls, 14 days", true},
	{"observations", "Observations", true},
}

func fleetSort(key string) (FleetSort, bool) {
	for _, s := range FleetSorts {
		if s.Key == key {
			return s, true
		}
	}
	return FleetSort{}, false
}

// ParseFleetQuery reads a table's URL state, replacing any unknown value
// with the default rather than guessing.
func ParseFleetQuery(v url.Values) FleetQuery {
	q := FleetQuery{Sort: v.Get("sort"), Dir: v.Get("dir"), Health: v.Get("health"), Q: strings.TrimSpace(v.Get("q")), Lang: strings.ToLower(strings.TrimSpace(v.Get("lang")))}
	if n, err := strconv.Atoi(v.Get("page")); err == nil {
		q.Page = n
	}
	return q.Normalize()
}

// Normalize fills defaults and drops values no column or state names.
func (q FleetQuery) Normalize() FleetQuery {
	s, ok := fleetSort(q.Sort)
	if !ok {
		s, _ = fleetSort("health")
	}
	q.Sort = s.Key
	if q.Dir != "asc" && q.Dir != "desc" {
		q.Dir = "asc"
		if s.Desc {
			q.Dir = "desc"
		}
	}
	valid := false
	for _, h := range HealthOrder {
		if string(h) == q.Health {
			valid = true
		}
	}
	if !valid {
		q.Health = ""
	}
	if len(q.Q) > 200 {
		q.Q = q.Q[:200]
	}
	if q.PerPage <= 0 {
		q.PerPage = 50
	}
	if q.Page < 1 {
		q.Page = 1
	}
	return q
}

// Values is the query as URL parameters, defaults omitted.
func (q FleetQuery) Values() url.Values {
	v := url.Values{}
	def := FleetQuery{}.Normalize()
	if q.Sort != def.Sort || q.Dir != def.Dir {
		v.Set("sort", q.Sort)
		v.Set("dir", q.Dir)
	}
	if q.Health != "" {
		v.Set("health", q.Health)
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Lang != "" {
		v.Set("lang", q.Lang)
	}
	if q.Page > 1 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	return v
}

// Filtered reports whether the query narrows the rows.
func (q FleetQuery) Filtered() bool { return q.Health != "" || q.Q != "" || q.Lang != "" }

// FleetTableModel is the fleet page's projection.
type FleetTableModel struct {
	Rows []FleetRow `json:"rows"`
	// Total is every project; Matched those the filters keep.
	Total   int           `json:"total"`
	Matched int           `json:"matched"`
	Pages   int           `json:"pages"`
	Query   FleetQuery    `json:"query"`
	Rollup  []HealthGroup `json:"rollup"`
	// Days is the traffic window; AbsentDays the days the call log holds
	// no row at all; ScaleMax the shared sparkline scale's top.
	Days        []string `json:"days"`
	AbsentDays  []string `json:"absent_days"`
	ScaleMax    int      `json:"scale_max"`
	CallLogNote string   `json:"call_log_note,omitempty"`
	Languages   []string `json:"languages"`
}

// FleetTable projects every project into the table: health, the shared
// sparkline scale, the rollup over the whole fleet, then the query's
// filter, sort and page.
func FleetTable(p Projects, q FleetQuery) FleetTableModel {
	q = q.Normalize()
	m := FleetTableModel{Query: q, Total: len(p.Rows), Days: append([]string{}, p.Days...), CallLogNote: p.CallLogNote}
	if m.Days == nil {
		m.Days = []string{}
	}
	logged := func(i int) bool { return i < len(p.Logged) && p.Logged[i] }
	for i, d := range p.Days {
		if !logged(i) {
			m.AbsentDays = append(m.AbsentDays, d)
		}
	}
	if m.AbsentDays == nil {
		m.AbsentDays = []string{}
	}
	// The shared scale: the busiest measured day of any project.
	if p.CallLogNote == "" {
		for _, r := range p.Rows {
			for i, n := range r.Traffic {
				if logged(i) && n > m.ScaleMax {
					m.ScaleMax = n
				}
			}
		}
	}
	langs := map[string]bool{}
	all := make([]FleetRow, 0, len(p.Rows))
	for _, r := range p.Rows {
		row := fleetRow(r, p, m.ScaleMax, logged)
		all = append(all, row)
		for _, l := range row.Languages {
			langs[strings.ToLower(l)] = true
		}
	}
	for l := range langs {
		m.Languages = append(m.Languages, l)
	}
	sort.Strings(m.Languages)
	if m.Languages == nil {
		m.Languages = []string{}
	}
	m.Rollup = rollup(all)

	var kept []FleetRow
	needle := strings.ToLower(q.Q)
	for _, r := range all {
		if q.Health != "" && string(r.Health) != q.Health {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(r.Service), needle) && !strings.Contains(strings.ToLower(r.Path), needle) {
			continue
		}
		if q.Lang != "" && !containsFold(r.Languages, q.Lang) {
			continue
		}
		kept = append(kept, r)
	}
	sortFleetRows(kept, q)
	m.Matched = len(kept)
	if q.Limit > 0 && len(kept) > q.Limit {
		kept = kept[:q.Limit]
	}
	m.Pages = (len(kept) + q.PerPage - 1) / q.PerPage
	if m.Pages == 0 {
		m.Pages = 1
	}
	if q.Page > m.Pages {
		q.Page = m.Pages
		m.Query.Page = q.Page
	}
	from := (q.Page - 1) * q.PerPage
	to := from + q.PerPage
	if to > len(kept) {
		to = len(kept)
	}
	m.Rows = append([]FleetRow{}, kept[from:to]...)
	return m
}

// fleetRow projects one project.
func fleetRow(r ProjectFacts, p Projects, scaleMax int, logged func(int) bool) FleetRow {
	row := FleetRow{Service: r.Service, Href: ProjectHref(r.Service, ""), Path: r.Path, Languages: r.Languages, Grade: r.Grade, Ladder: r.Rungs}
	if row.Ladder == nil {
		row.Ladder = []Rung{}
	}
	const registry = "the daemon's service registry (list_services)"
	if r.Symbols != nil {
		row.Symbols = Measured(*r.Symbols, registry)
	} else {
		row.Symbols = Absent[int](nonEmpty(r.SymbolsNote, "no store read reported a symbol count")).WithSource(registry)
	}
	row.Freshness = freshnessClass(r.Freshness)
	if r.Freshness != row.Freshness {
		row.FreshnessNote = r.Freshness
	}
	for _, g := range r.Rungs {
		switch g.State {
		case RungMet:
			row.LadderMet++
			row.LadderOf++
		case RungUnmet, RungUnknown:
			row.LadderOf++
		}
	}
	// Plans: the newest recorded run of each plan.
	switch {
	case p.Ledger.State == StateError:
		row.PlansVerified = Errored[int]("verify ledger unreadable: " + p.Ledger.Reason)
	case len(r.Runs) > 0:
		proof := SumPlanProof(r.Runs)
		row.Plans = &proof
		row.PlansVerified = Measured(proof.Verified, verifyLedgerSource).WithDenominator(fmt.Sprintf("of %d claimed in %s", proof.Claimed, countNoun(proof.Plans, "plan")))
	case r.HasPlans:
		row.PlansVerified = Absent[int]("plans here, but no verify run is recorded at " + nonEmpty(r.Path, "this root"))
	default:
		row.PlansVerified = Absent[int]("no plans here for the gate to check")
	}
	switch r.Claim.State {
	case StatePresent, StateZero:
		row.ClaimsFresh = Measured(r.Claim.V.Fresh, storeClaimsSource).WithDenominator(fmt.Sprintf("of %d claims", r.Claim.V.Total))
		row.ClaimsTotal = r.Claim.V.Total
	case StateError:
		row.ClaimsFresh = Errored[int](r.Claim.Reason)
	default:
		row.ClaimsFresh = Absent[int](nonEmpty(r.Claim.Reason, "claims not read"))
	}
	switch {
	case r.Certs.State == StateError:
		row.CertsLive = Errored[int](r.Certs.Reason)
	case r.Certs.State == StatePresent && r.Certs.V.Declared > 0:
		row.CertsLive = Measured(r.Certs.V.Live, storeCertsSource).WithDenominator(fmt.Sprintf("of %d declared targets", r.Certs.V.Declared))
		row.CertsDeclared = r.Certs.V.Declared
	case r.Certs.State == StatePresent:
		row.CertsLive = Absent[int]("no certification target declared here")
	default:
		row.CertsLive = Absent[int](nonEmpty(r.Certs.Reason, "certificates not read"))
	}
	switch {
	case r.ObservationsErr != "":
		row.Observations = Errored[int]("observations file unreadable: " + r.ObservationsErr)
	case r.OpenObservations != nil:
		row.Observations = Measured(*r.OpenObservations, "the repo's .fathom/observations.md")
	default:
		row.Observations = Absent[int]("no .fathom/observations.md here")
	}
	row.Series, row.Calls = sparkFor(r.Traffic, p, scaleMax, logged)
	row.Health, row.HealthWhy = projectHealth(r, p, row)
	return row
}

// sparkFor puts one project's traffic on the shared scale, absent days
// named and never joined across.
func sparkFor(traffic []int, p Projects, scaleMax int, logged func(int) bool) (Spark, Value[int]) {
	s := Spark{Max: scaleMax, Points: make([]SparkPoint, 0, len(p.Days)), Runs: [][2]int{}}
	const source = "the resolved call log, calls per day this project answered"
	if p.CallLogNote != "" {
		for _, d := range p.Days {
			s.Points = append(s.Points, SparkPoint{Date: d, Absent: p.CallLogNote})
			s.Absent++
		}
		return s, Absent[int](p.CallLogNote).WithSource(source)
	}
	total := 0
	for i, d := range p.Days {
		if !logged(i) {
			s.Points = append(s.Points, SparkPoint{Date: d, Absent: "no call-log rows for " + d})
			s.Absent++
			continue
		}
		n := 0
		if i < len(traffic) {
			n = traffic[i]
		}
		s.Points = append(s.Points, SparkPoint{Date: d, Calls: &n})
		s.Measured++
		total += n
		if k := len(s.Runs); k > 0 && s.Runs[k-1][1] == i-1 {
			s.Runs[k-1][1] = i
		} else {
			s.Runs = append(s.Runs, [2]int{i, i})
		}
	}
	if s.Measured == 0 {
		return s, Absent[int]("no call-log rows on any day of the window").WithSource(source)
	}
	denom := fmt.Sprintf("%d measured of %d days", s.Measured, len(p.Days))
	if s.Absent > 0 {
		return s, Partial(total, fmt.Sprintf("%d of %d days have no call-log rows at all; lower bound", s.Absent, len(p.Days))).WithSource(source).WithDenominator(denom)
	}
	return s, Measured(total, source).WithDenominator(denom)
}

// projectHealth folds a project's known problems and unknowns into one
// state: failed over degraded over unknown over healthy — a known problem
// outranks an unknown, and an unknown is never healthy.
func projectHealth(r ProjectFacts, p Projects, row FleetRow) (Health, string) {
	var failed, degraded, unknown []string
	if row.Plans != nil && row.Plans.Failing > 0 {
		failed = append(failed, fmt.Sprintf("%s the gate", countNoun(row.Plans.Failing, "claimed plan row")+" "+pluralVerb(row.Plans.Failing, "fails", "fail")))
	}
	if r.TypedEdgesFailed != "" {
		failed = append(failed, "typed call-edge pass failed")
	}
	switch row.Freshness {
	case "stale":
		degraded = append(degraded, "index behind HEAD")
	case "unknown":
		unknown = append(unknown, "freshness unknown ("+strings.TrimPrefix(nonEmpty(r.Freshness, "not reported"), "unknown: ")+")")
	}
	switch rungState(r.Rungs, "wired") {
	case RungUnmet:
		degraded = append(degraded, "not wired")
	case RungUnknown:
		unknown = append(unknown, "wiring unknown")
	}
	if row.Plans != nil && row.Plans.Unproven > 0 {
		degraded = append(degraded, countNoun(row.Plans.Unproven, "claimed row")+" the gate cannot prove")
	}
	if r.ObservationsErr != "" {
		degraded = append(degraded, "observations file unreadable")
	}
	if row.Plans == nil && r.HasPlans {
		if p.Ledger.State == StateError {
			unknown = append(unknown, "verify ledger unreadable")
		} else {
			unknown = append(unknown, "no verify run recorded for its plans")
		}
	}
	if p.RegistryNote != "" {
		unknown = append(unknown, "registry read incompletely")
	}
	why := strings.Join(append(append(append([]string{}, failed...), degraded...), unknown...), "; ")
	switch {
	case len(failed) > 0:
		return HealthFailed, why
	case len(degraded) > 0:
		return HealthDegraded, why
	case len(unknown) > 0:
		return HealthUnknown, why
	}
	if r.HasPlans {
		return HealthHealthy, "fresh, wired, and every claimed plan row proven"
	}
	return HealthHealthy, "fresh and wired; no plans here to gate"
}

// rollup groups every project by health, worst first.
func rollup(rows []FleetRow) []HealthGroup {
	out := make([]HealthGroup, 0, len(HealthOrder))
	for _, h := range HealthOrder {
		g := HealthGroup{Health: h, Services: []string{}}
		for _, r := range rows {
			if r.Health == h {
				g.Count++
				g.Services = append(g.Services, r.Service)
			}
		}
		sort.Strings(g.Services)
		out = append(out, g)
	}
	return out
}

// sortFleetRows orders rows by the query's column. A value that is not
// measured sorts after every measured one in both directions, by name.
func sortFleetRows(rows []FleetRow, q FleetQuery) {
	key := fleetSortKey(q.Sort)
	desc := q.Dir == "desc"
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		ka, oka := key(a)
		kb, okb := key(b)
		if oka != okb {
			return oka
		}
		if oka && ka != kb {
			if desc {
				return ka > kb
			}
			return ka < kb
		}
		// Ties: the busier project first, then by name.
		if q.Sort != "calls" {
			ca, cb := valueKey(a.Calls), valueKey(b.Calls)
			if ca != cb {
				return ca > cb
			}
		}
		return a.Service < b.Service
	})
}

func fleetSortKey(key string) func(FleetRow) (float64, bool) {
	switch key {
	case "service":
		return func(FleetRow) (float64, bool) { return 0, true } // ties fall to the name
	case "symbols":
		return func(r FleetRow) (float64, bool) { return valueOK(r.Symbols) }
	case "freshness":
		return func(r FleetRow) (float64, bool) {
			return map[string]float64{"fresh": 0, "stale": 1, "unknown": 2}[r.Freshness], true
		}
	case "realization":
		return func(r FleetRow) (float64, bool) {
			if r.LadderOf == 0 {
				return 0, false
			}
			return float64(r.LadderMet) / float64(r.LadderOf), true
		}
	case "plans":
		return func(r FleetRow) (float64, bool) {
			if r.Plans == nil || r.Plans.Claimed == 0 {
				return 0, false
			}
			return float64(r.Plans.Verified) / float64(r.Plans.Claimed), true
		}
	case "claims":
		return func(r FleetRow) (float64, bool) { return valueOK(r.ClaimsFresh) }
	case "certs":
		return func(r FleetRow) (float64, bool) { return valueOK(r.CertsLive) }
	case "calls":
		return func(r FleetRow) (float64, bool) { return valueOK(r.Calls) }
	case "observations":
		return func(r FleetRow) (float64, bool) { return valueOK(r.Observations) }
	}
	return func(r FleetRow) (float64, bool) { return float64(r.Health.Rank()), true }
}

func valueOK(v Value[int]) (float64, bool) {
	if !v.Known() {
		return 0, false
	}
	return float64(v.V), true
}

func valueKey(v Value[int]) int {
	if !v.Known() {
		return -1
	}
	return v.V
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

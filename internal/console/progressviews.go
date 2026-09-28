package console

// progressviews.go — PE-6, PE-8..PE-12 (docs/design/progress-and-economics.md
// §4): the Progress views, each a pure projection of the proof history
// (proofhistory.go) with its truth state. Read-only by construction: no
// function here writes a plan or a status (ADR-049 decision 4).

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// ─── burn-up (PE-6) ─────────────────────────────────────────────────────────

// BurnPoint is one day's weighted state at the day's end: proven weight
// over the weight of every row still in its plan and not dispositioned.
type BurnPoint struct {
	Day    string `json:"day"`
	Proven int    `json:"proven_weight"`
	Total  int    `json:"total_weight"`
}

// Projection is the burn-up's projected finish: a least-squares line over
// the fit window's proven weight, the day it reaches the newest total, and
// the interval from the slope's standard error (±2 SE). High is "" when the
// slope's lower bound is not positive: the interval has no late end.
type Projection struct {
	Finish string  `json:"finish"`
	Low    string  `json:"low"`
	High   string  `json:"high,omitempty"`
	Slope  float64 `json:"slope_per_day"`
	SE     float64 `json:"slope_se"`
	Days   int     `json:"fit_days"`
	Total  int     `json:"total_weight"`
}

// BurnUpModel is the burn-up view.
type BurnUpModel struct {
	Points     []BurnPoint       `json:"points"`
	Projection Value[Projection] `json:"projection"`
}

// Burn-up projection bounds (design §4, §6: never with fewer than 7 days).
const (
	BurnUpFitDays = 14
	BurnUpMinDays = 7
)

// BurnUp is the per-day proven weight from the first reading to now's day
// (or the filter's To), and its projected finish.
func BurnUp(h History, f HistoryFilter, now time.Time) BurnUpModel {
	rows := f.rows(h)
	var m BurnUpModel
	first := ""
	for _, r := range rows {
		if a := r.Readings[0].At; first == "" || a < first {
			first = a
		}
	}
	if first == "" {
		m.Projection = Absent[Projection]("no schema-3 reading in the proof history for this selection — the burn-up draws once `fathom verify` records per-row verdicts")
		return m
	}
	start, _ := time.Parse("2006-01-02", dayOf(first))
	end := now.UTC()
	if f.To != "" {
		if t, err := time.Parse("2006-01-02", f.To); err == nil && t.Before(end) {
			end = t
		}
	}
	if f.From != "" {
		if t, err := time.Parse("2006-01-02", f.From); err == nil && t.After(start) {
			start = t
		}
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		day := d.Format("2006-01-02")
		cut := day + "T23:59:59Z"
		p := BurnPoint{Day: day}
		for _, r := range rows {
			rd, ok := r.StateAt(cut)
			if !ok || rd.State == HistAbsent || rd.State == HistDispositioned {
				continue
			}
			p.Total += rd.Weight
			if rd.State == HistProven {
				p.Proven += rd.Weight
			}
		}
		m.Points = append(m.Points, p)
	}
	m.Projection = project(m.Points)
	return m
}

// project fits the last BurnUpFitDays points.
func project(pts []BurnPoint) Value[Projection] {
	if len(pts) < BurnUpMinDays {
		return Absent[Projection](fmt.Sprintf("%d days of proof history — a projected finish needs at least %d days of data", len(pts), BurnUpMinDays))
	}
	win := pts
	if len(win) > BurnUpFitDays {
		win = win[len(win)-BurnUpFitDays:]
	}
	n := float64(len(win))
	var sx, sy float64
	for i, p := range win {
		sx += float64(i)
		sy += float64(p.Proven)
	}
	mx, my := sx/n, sy/n
	var sxx, sxy float64
	for i, p := range win {
		dx := float64(i) - mx
		sxx += dx * dx
		sxy += dx * (float64(p.Proven) - my)
	}
	slope := sxy / sxx
	icpt := my - slope*mx
	var sse float64
	for i, p := range win {
		e := float64(p.Proven) - (icpt + slope*float64(i))
		sse += e * e
	}
	se := 0.0
	if n > 2 {
		se = math.Sqrt(sse / (n - 2) / sxx)
	}
	last := win[len(win)-1]
	day0, _ := time.Parse("2006-01-02", win[0].Day)
	pr := Projection{Slope: round2(slope), SE: round2(se), Days: len(win), Total: last.Total}
	if last.Proven >= last.Total {
		pr.Finish, pr.Low, pr.High = last.Day, last.Day, last.Day
		return Present(pr, "the proof history")
	}
	if slope <= 0 {
		return Absent[Projection](fmt.Sprintf("no proven weight gained over the last %d days — there is no finish to project", len(win)))
	}
	at := func(s float64) string {
		x := math.Ceil((float64(last.Total)-icpt)/s - 1e-9)
		return day0.AddDate(0, 0, int(x)).Format("2006-01-02")
	}
	pr.Finish = at(slope)
	pr.Low = at(slope + 2*se)
	if lo := slope - 2*se; lo > 0 {
		pr.High = at(lo)
	}
	return Present(pr, "the proof history").WithDenominator(fmt.Sprintf("a linear fit over %d days", len(win)))
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ─── proof regressions (PE-8) ───────────────────────────────────────────────

// Regression is a row once claimed-proven that now reads otherwise: the
// reading where it last became proven, and the first reading that changed.
type Regression struct {
	Service       string `json:"service"`
	Plan          string `json:"plan"`
	ID            string `json:"id"`
	ProvenAt      string `json:"proven_at"`
	ProvenCommit  string `json:"proven_commit,omitempty"`
	ChangedAt     string `json:"changed_at"`
	ChangedCommit string `json:"changed_commit,omitempty"`
	State         string `json:"state"`
	Criterion     string `json:"criterion,omitempty"`
}

// ProofRegressions lists every regression as of the filter's To whose
// change falls in its date range, newest change first. A row that left its
// plan (absent) is not a regression.
func ProofRegressions(h History, f HistoryFilter) []Regression {
	var out []Regression
	for _, r := range f.rows(h) {
		last := r.Latest()
		if last.State == HistProven || last.State == HistAbsent {
			continue
		}
		// The newest proven reading, and the reading right after it.
		pi := -1
		for i, rd := range r.Readings {
			if rd.State == HistProven {
				pi = i
			}
		}
		if pi < 0 || pi+1 >= len(r.Readings) {
			continue
		}
		p, c := r.Readings[pi], r.Readings[pi+1]
		if !f.inRange(c.At) {
			continue
		}
		out = append(out, Regression{Service: r.Service, Plan: r.Plan, ID: r.ID,
			ProvenAt: p.At, ProvenCommit: p.Commit, ChangedAt: c.At, ChangedCommit: c.Commit,
			State: last.State, Criterion: last.Criterion})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ChangedAt != out[j].ChangedAt {
			return out[i].ChangedAt > out[j].ChangedAt
		}
		return RowKey(out[i].Service, out[i].Plan, out[i].ID) < RowKey(out[j].Service, out[j].Plan, out[j].ID)
	})
	return out
}

// ─── time-to-proof (PE-9) ───────────────────────────────────────────────────

// RowTimeToProof is one row's first appearance and first proven reading.
// LowerBound marks a row present at its plan's first reading in the
// history: it existed before the history began, so its time is at least
// Hours, never exactly.
type RowTimeToProof struct {
	Service     string `json:"service"`
	Plan        string `json:"plan"`
	ID          string `json:"id"`
	FirstSeen   string `json:"first_seen"`
	FirstProven string `json:"first_proven,omitempty"`
	Hours       int    `json:"hours,omitempty"`
	LowerBound  bool   `json:"lower_bound,omitempty"`
}

// TimeToProofModel is the view: every row, the proven count and the median
// hours over rows whose time is exact.
type TimeToProofModel struct {
	Rows        []RowTimeToProof `json:"rows"`
	Proven      Value[int]       `json:"proven"`
	MedianHours Value[int]       `json:"median_hours"`
}

// TimeToProof is first appearance of a row id to its first claimed-proven
// reading, per row.
func TimeToProof(h History, f HistoryFilter) TimeToProofModel {
	var m TimeToProofModel
	var exact []int
	proven := 0
	for _, r := range f.rows(h) {
		first := r.Readings[0]
		row := RowTimeToProof{Service: r.Service, Plan: r.Plan, ID: r.ID, FirstSeen: first.At, LowerBound: first.First}
		for _, rd := range r.Readings {
			if rd.State != HistProven {
				continue
			}
			row.FirstProven = rd.At
			if a, ok := parseAt(first.At); ok {
				if b, ok := parseAt(rd.At); ok {
					row.Hours = int(b.Sub(a).Hours())
				}
			}
			break
		}
		if row.FirstProven != "" && f.inRange(row.FirstProven) {
			proven++
			if !row.LowerBound {
				exact = append(exact, row.Hours)
			}
		}
		if row.FirstProven == "" {
			row.LowerBound = false
		}
		m.Rows = append(m.Rows, row)
	}
	m.Proven = Measured(proven, "the proof history")
	if len(m.Rows) == 0 {
		m.Proven = Absent[int]("no schema-3 reading in the proof history for this selection")
	}
	if len(exact) == 0 {
		m.MedianHours = Absent[int]("no row both appeared and was proven inside the history — every proven row predates it (a lower bound only)")
	} else {
		sort.Ints(exact)
		m.MedianHours = Measured(exact[len(exact)/2], "the proof history").WithDenominator(fmt.Sprintf("%d %s", len(exact), plural(len(exact), "row", "rows")) + " first seen inside the history")
	}
	return m
}

// ─── claim honesty (PE-10) ──────────────────────────────────────────────────

// ModelNotRecorded is the model of a session whose ledger named none.
const ModelNotRecorded = "model not recorded"

// AgentSession is one agent session's window in one repository, from the
// session ledger. ID joins; it never reaches an output row (I58).
type AgentSession struct {
	ID      string
	Harness string
	Model   string
	Repo    string // a repository root, or its base name
	Start   string // RFC 3339
	End     string
}

// HonestyRow is one agent identity's claims across the readings its
// sessions produced: claimed rows the gate did not prove over claimed rows.
type HonestyRow struct {
	Harness  string         `json:"harness"`
	Model    string         `json:"model"`
	Readings int            `json:"readings"`
	Claimed  int            `json:"claimed"`
	Unproven int            `json:"unproven"`
	Rate     Value[float64] `json:"unproven_rate"`
}

// ClaimHonestyModel is the view. Unattributed counts readings no session
// window covers (a human's CLI run, a CI gate): named, never dropped.
type ClaimHonestyModel struct {
	Rows         []HonestyRow `json:"rows"`
	Readings     int          `json:"readings"`
	Unattributed int          `json:"unattributed_readings"`
}

// sessionGrace is how long after a session's last ledger row a reading in
// its repository is still its: the Stop hook's verify lands after it.
const sessionGrace = 10 * time.Minute

// ClaimHonesty joins each reading to the session whose window covers it in
// the same repository and sums claimed-unproven per (harness, model). No
// output row names a session or a person (I58).
func ClaimHonesty(h History, sessions []AgentSession, f HistoryFilter) ClaimHonestyModel {
	type ident struct{ harness, model string }
	agg := map[ident]*HonestyRow{}
	var m ClaimHonestyModel
	for _, r := range h.Readings {
		if !f.keepPlan(r.Service, r.Plan) || !f.inRange(r.At) {
			continue
		}
		s, ok := sessionFor(r, sessions)
		if !ok {
			if f.Agent == "" {
				m.Readings++
				m.Unattributed++
			}
			continue
		}
		id := ident{s.Harness, s.Model}
		if id.harness == "" {
			id.harness = "harness not recorded"
		}
		if id.model == "" {
			id.model = ModelNotRecorded
		}
		if f.Agent != "" && f.Agent != id.harness && f.Agent != id.harness+"/"+id.model {
			continue
		}
		m.Readings++
		row := agg[id]
		if row == nil {
			row = &HonestyRow{Harness: id.harness, Model: id.model}
			agg[id] = row
		}
		row.Readings++
		row.Claimed += r.Claimed
		row.Unproven += r.Unproven()
	}
	for _, row := range agg {
		if row.Claimed == 0 {
			row.Rate = Absent[float64]("no claimed row in this identity's readings")
		} else {
			row.Rate = Measured(round2(float64(row.Unproven)/float64(row.Claimed)), "the proof history joined to the session ledger").
				WithDenominator(fmt.Sprintf("%d claimed rows across %s", row.Claimed, fmt.Sprintf("%d %s", row.Readings, plural(row.Readings, "reading", "readings"))))
		}
		m.Rows = append(m.Rows, *row)
	}
	sort.Slice(m.Rows, func(i, j int) bool {
		if m.Rows[i].Harness != m.Rows[j].Harness {
			return m.Rows[i].Harness < m.Rows[j].Harness
		}
		return m.Rows[i].Model < m.Rows[j].Model
	})
	return m
}

func sessionFor(r PlanReading, sessions []AgentSession) (AgentSession, bool) {
	at, ok := parseAt(r.At)
	if !ok {
		return AgentSession{}, false
	}
	var best AgentSession
	found := false
	for _, s := range sessions {
		if s.Repo == "" || (s.Repo != r.Root && s.Repo != baseName(r.Root)) {
			continue
		}
		st, ok1 := parseAt(s.Start)
		en, ok2 := parseAt(s.End)
		if !ok1 || !ok2 || at.Before(st) || at.After(en.Add(sessionGrace)) {
			continue
		}
		if !found || s.Start > best.Start {
			best, found = s, true
		}
	}
	return best, found
}

func baseName(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ─── heatmap and streaks (PE-11) ────────────────────────────────────────────

// HeatCell is one plan's weight newly proven in one week (Monday start).
type HeatCell struct {
	Plan   string `json:"plan"`
	Week   string `json:"week"`
	Weight int    `json:"proven_weight"`
}

// HeatmapModel is the heatmap plus the "fun, true" counts: the current
// streak of days with a newly proven row and how many rows were proven
// today — each a count, never a score.
type HeatmapModel struct {
	Plans       []string   `json:"plans"`
	Weeks       []string   `json:"weeks"`
	Cells       []HeatCell `json:"cells"`
	Streak      Value[int] `json:"streak_days"`
	ProvedToday Value[int] `json:"proved_today"`
}

// ProofHeatmap folds every transition into claimed-proven by plan and week.
func ProofHeatmap(h History, f HistoryFilter, now time.Time) HeatmapModel {
	var m HeatmapModel
	cells := map[[2]string]int{}
	plans, weeks := map[string]bool{}, map[string]bool{}
	days := map[string]bool{}
	today := now.UTC().Format("2006-01-02")
	rows := f.rows(h)
	proved := 0
	for _, r := range rows {
		prev := ""
		for _, rd := range r.Readings {
			if rd.State == HistProven && prev != HistProven && !rd.First && f.inRange(rd.At) {
				wk := weekOf(rd.At)
				cells[[2]string{r.Plan, wk}] += rd.Weight
				plans[r.Plan], weeks[wk], days[dayOf(rd.At)] = true, true, true
				if dayOf(rd.At) == today {
					proved++
				}
			}
			prev = rd.State
		}
	}
	for k, w := range cells {
		m.Cells = append(m.Cells, HeatCell{Plan: k[0], Week: k[1], Weight: w})
	}
	sort.Slice(m.Cells, func(i, j int) bool {
		if m.Cells[i].Plan != m.Cells[j].Plan {
			return m.Cells[i].Plan < m.Cells[j].Plan
		}
		return m.Cells[i].Week < m.Cells[j].Week
	})
	m.Plans, m.Weeks = sortedKeys(plans), sortedKeys(weeks)
	if len(rows) == 0 {
		m.Streak = Absent[int]("no schema-3 reading in the proof history for this selection")
		m.ProvedToday = m.Streak
		return m
	}
	// The streak counts back from today (or yesterday, when today has no
	// proof yet — the day is not over).
	d := now.UTC()
	if !days[today] {
		d = d.AddDate(0, 0, -1)
	}
	streak := 0
	for days[d.Format("2006-01-02")] {
		streak++
		d = d.AddDate(0, 0, -1)
	}
	m.Streak = Measured(streak, "the proof history").WithDenominator("consecutive days with a newly proven row")
	m.ProvedToday = Measured(proved, "the proof history").WithDenominator("rows newly proven on " + today + " (UTC)")
	return m
}

// weekOf is the Monday (UTC) of the week instant at falls in.
func weekOf(at string) string {
	t, ok := parseAt(at)
	if !ok {
		return dayOf(at)
	}
	t = t.UTC()
	off := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -off).Format("2006-01-02")
}

// ─── derived kanban (PE-5's projection) ─────────────────────────────────────

// The derived board's columns (design §4), in order.
const (
	BoardOpen     = "open"
	BoardClaimed  = "claimed-unproven"
	BoardProven   = "proven"
	BoardDisposed = "refused-or-deferred"
)

// ProgressCard is one row on the derived board; Criterion is the failing (or
// undecidable) criterion a claimed-unproven card links to.
type ProgressCard struct {
	Service   string `json:"service"`
	Plan      string `json:"plan"`
	ID        string `json:"id"`
	Weight    int    `json:"weight"`
	Criterion string `json:"criterion,omitempty"`
	Since     string `json:"since"`
	Commit    string `json:"commit,omitempty"`
}

// ProgressColumn is one column of the derived board.
type ProgressColumn struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Cards []ProgressCard `json:"cards"`
}

// DerivedBoard sorts every row's newest state into the four columns. It is
// a projection: nothing moves a card but a verify reading.
func DerivedBoard(h History, f HistoryFilter) []ProgressColumn {
	cols := []ProgressColumn{
		{ID: BoardOpen, Label: "Open"},
		{ID: BoardClaimed, Label: "Claimed, unproven"},
		{ID: BoardProven, Label: "Proven"},
		{ID: BoardDisposed, Label: "Refused or deferred"},
	}
	for _, r := range f.rows(h) {
		last := r.Latest()
		c := ProgressCard{Service: r.Service, Plan: r.Plan, ID: r.ID, Weight: last.Weight, Criterion: last.Criterion, Since: last.At, Commit: last.Commit}
		switch last.State {
		case HistOpen:
			cols[0].Cards = append(cols[0].Cards, c)
		case HistUnproven, HistUnknown:
			cols[1].Cards = append(cols[1].Cards, c)
		case HistProven:
			cols[2].Cards = append(cols[2].Cards, c)
		case HistDispositioned:
			cols[3].Cards = append(cols[3].Cards, c)
		}
	}
	return cols
}

// ─── goal gauges and the goal filter (PE-12) ────────────────────────────────

// GoalGauge is one scope-bet goal: its measurement rows proven over their
// total, and its review date. Proven is partial when a named measurement
// row has no reading, absent when the goal names none.
type GoalGauge struct {
	Goal    string     `json:"goal"`
	Title   string     `json:"title"`
	Bet     string     `json:"bet,omitempty"`
	Review  string     `json:"review,omitempty"`
	Rows    []string   `json:"measurement_rows"`
	Total   int        `json:"total"`
	Proven  Value[int] `json:"proven"`
	Refused string     `json:"refused,omitempty"`
}

var (
	measurementRowRe = regexp.MustCompile(`\b[A-Z][A-Z0-9]*-M[0-9]+\b`)
	goalPlanRe       = regexp.MustCompile(`docs/plans/[A-Za-z0-9._/-]+\.md`)
)

func goalText(b intent.GoalBet, conds []intent.Condition) string {
	for _, c := range conds {
		if c.ID == b.Condition {
			return c.Text
		}
	}
	return ""
}

// goalGauges is a gauge per goal bet, from the newest reading of each
// measurement row its condition names (`PE-M1`, …).
func goalGauges(bets []intent.GoalBet, conds []intent.Condition, h History) []GoalGauge {
	latest := map[string]string{}
	for _, r := range h.Rows {
		latest[r.ID] = r.Latest().State
	}
	out := make([]GoalGauge, 0, len(bets))
	for _, b := range bets {
		g := GoalGauge{Goal: b.Goal.ID, Title: b.Goal.Title, Bet: b.Name, Review: b.ReviewRaw, Refused: b.Refused}
		seen := map[string]bool{}
		for _, id := range measurementRowRe.FindAllString(goalText(b, conds), -1) {
			if !seen[id] {
				seen[id] = true
				g.Rows = append(g.Rows, id)
			}
		}
		g.Total = len(g.Rows)
		proven, unread := 0, []string{}
		for _, id := range g.Rows {
			st, ok := latest[id]
			switch {
			case !ok:
				unread = append(unread, id)
			case st == HistProven:
				proven++
			}
		}
		switch {
		case g.Total == 0:
			g.Proven = Absent[int]("the goal's condition names no measurement row")
		case len(unread) == g.Total:
			g.Proven = Absent[int]("no reading of " + strings.Join(unread, ", ") + " in the proof history")
		case len(unread) > 0:
			g.Proven = Partial(proven, "no reading of "+strings.Join(unread, ", ")+" in the proof history").WithSource("the proof history")
		default:
			g.Proven = Measured(proven, "the proof history")
		}
		g.Proven = g.Proven.WithDenominator(fmt.Sprintf("%d %s", g.Total, plural(g.Total, "measurement row", "measurement rows")))
		out = append(out, g)
	}
	return out
}

// GoalGauges is goalGauges for the renderer.
func GoalGauges(bets []intent.GoalBet, conds []intent.Condition, h History) []GoalGauge {
	return goalGauges(bets, conds, h)
}

// GoalScope is what a goal filter selects: the plans its condition names.
type GoalScope struct {
	Goal  string   `json:"goal"`
	Plans []string `json:"plans"`
}

// GoalFilter resolves a goal id into the plans its condition names.
func GoalFilter(bets []intent.GoalBet, conds []intent.Condition, goal string) GoalScope {
	gs := GoalScope{Goal: goal}
	for _, b := range bets {
		if b.Goal.ID != goal {
			continue
		}
		seen := map[string]bool{}
		for _, p := range goalPlanRe.FindAllString(goalText(b, conds), -1) {
			if !seen[p] {
				seen[p] = true
				gs.Plans = append(gs.Plans, p)
			}
		}
	}
	sort.Strings(gs.Plans)
	return gs
}

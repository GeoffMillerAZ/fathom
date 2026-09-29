package console

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func fleetFixture() Projects {
	days := []string{"2026-09-19", "2026-09-20", "2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24", "2026-09-25"}
	met := func(names ...string) []Rung {
		var out []Rung
		for _, n := range []string{"wired", "fresh", "queried", "preventing", "gating"} {
			st := RungUnmet
			for _, m := range names {
				if m == n {
					st = RungMet
				}
			}
			out = append(out, Rung{Name: n, State: st, Evidence: n})
		}
		return out
	}
	store := func(fresh int) Value[ClaimCounts] {
		return Present(ClaimCounts{Total: fresh + 1, Fresh: fresh, Stale: 1}, "store")
	}
	return Projects{
		Days:   days,
		Logged: []bool{true, true, false, true, true, true, true}, // 09-21: the log holds no row at all
		Ledger: SourceRead{Name: SourceVerifyLedger, State: StatePresent},
		Store:  SourceRead{Name: SourceStore, State: StatePresent},
		Rows: []ProjectFacts{
			{Service: "busy", Path: "/src/busy", Symbols: intp(15000), Realized: 5, Available: 5, Sessions: 10, Languages: []string{"go"}, Freshness: "fresh", Rungs: met("wired", "fresh", "queried", "preventing", "gating"),
				HasPlans: true, Runs: []VerifyRun{{Plan: "p.md", Total: 10, Claimed: 10, Verified: 10}},
				Claim: store(300), Certs: Present(CertCounts{Declared: 4, Live: 1, Gaps: 3}, "s"), OpenObservations: intp(2),
				Traffic: []int{100, 400, 0, 250, 90, 0, 60}},
			{Service: "quiet", Path: "/src/quiet", Symbols: intp(900), Realized: 3, Available: 4, Sessions: 1, Languages: []string{"python"}, Freshness: "fresh", Rungs: met("wired", "fresh", "queried"),
				Claim: store(5), Certs: Present(CertCounts{}, "s"),
				Traffic: []int{1, 0, 0, 2, 0, 0, 1}},
			{Service: "failing", Path: "/src/failing", Symbols: intp(3000), Realized: 2, Available: 5, Sessions: 4, Languages: []string{"go", "typescript"}, Freshness: "fresh", Rungs: met("wired", "fresh"),
				HasPlans: true, Runs: []VerifyRun{{Plan: "p.md", Total: 10, Claimed: 9, Verified: 6, Failing: 3}},
				Claim: store(10), Certs: Present(CertCounts{}, "s"), Traffic: []int{5, 5, 0, 5, 5, 5, 5}},
			{Service: "stale", Path: "/src/stale", Symbols: intp(2000), Realized: 1, Available: 5, Sessions: 3, Languages: []string{"go"}, Freshness: "stale: indexed a vs HEAD b", Rungs: met("wired"),
				Claim: store(1), Certs: Present(CertCounts{}, "s"), Traffic: []int{0, 0, 0, 0, 0, 0, 0}},
			{Service: "remote", Symbols: intp(500), Freshness: "unknown: no recorded corpus path", Rungs: met("wired"),
				Claim: store(0), Certs: Present(CertCounts{}, "s"), Traffic: []int{0, 3, 0, 0, 0, 0, 0}},
			{Service: "unrun", Path: "/src/unrun", Symbols: intp(700), Freshness: "fresh", Rungs: met("wired", "fresh"), HasPlans: true,
				Claim: store(2), Certs: Present(CertCounts{}, "s"), Traffic: []int{0, 0, 0, 0, 0, 0, 0}},
		},
	}
}

func rowByName(rows []FleetRow, name string) FleetRow {
	for _, r := range rows {
		if r.Service == name {
			return r
		}
	}
	return FleetRow{}
}

// TestFleetTable_SortFilterAndUnknownNotGreen pins OC-19: the fleet is one
// table sortable by any column and filterable by health, name and language
// through URL state; the rollup counts every project by health; and a
// project nobody can vouch for is unknown — never healthy — while a known
// problem outranks an unknown.
func TestFleetTable_SortFilterAndUnknownNotGreen(t *testing.T) {
	p := fleetFixture()
	// The default order is RG-4's: unrealized value (unmet rungs × sessions
	// in the window), the busier project first on a tie.
	def := FleetTable(p, ParseFleetQuery(url.Values{}))
	if def.Query.Sort != "unrealized" || def.Query.Dir != "desc" {
		t.Fatalf("default query = %+v", def.Query)
	}
	var byValue []string
	for _, r := range def.Rows {
		byValue = append(byValue, r.Service)
	}
	if got := strings.Join(byValue, ","); got != "failing,stale,quiet,busy,remote,unrun" {
		t.Errorf("default (unrealized value) order = %s", got)
	}
	m := FleetTable(p, ParseFleetQuery(url.Values{"sort": {"health"}}))
	if m.Total != 6 || m.Matched != 6 || len(m.Rows) != 6 || m.Query.Sort != "health" || m.Query.Dir != "asc" {
		t.Fatalf("health table = total %d matched %d rows %d query %+v", m.Total, m.Matched, len(m.Rows), m.Query)
	}
	health := map[string]Health{}
	for _, r := range m.Rows {
		health[r.Service] = r.Health
		if r.HealthWhy == "" {
			t.Errorf("%s: health without its reason", r.Service)
		}
	}
	want := map[string]Health{"failing": HealthFailed, "stale": HealthDegraded, "remote": HealthUnknown, "unrun": HealthUnknown, "busy": HealthHealthy, "quiet": HealthHealthy}
	for svc, h := range want {
		if health[svc] != h {
			t.Errorf("%s health = %s, want %s (why: %s)", svc, health[svc], h, rowByName(m.Rows, svc).HealthWhy)
		}
	}
	if w := rowByName(m.Rows, "remote").HealthWhy; !strings.Contains(w, "freshness unknown") {
		t.Errorf("an unknown project names why: %q", w)
	}
	if w := rowByName(m.Rows, "unrun").HealthWhy; !strings.Contains(w, "no verify run recorded") {
		t.Errorf("plans with no recorded run are unknown, never proven: %q", w)
	}
	// Default order: worst first, the busier project first within a state.
	var order []string
	for _, r := range m.Rows {
		order = append(order, r.Service)
	}
	if got := strings.Join(order, ","); got != "failing,stale,remote,unrun,busy,quiet" {
		t.Errorf("health order = %s", got)
	}
	// The rollup covers the whole fleet whatever the filter.
	f := FleetTable(p, ParseFleetQuery(url.Values{"health": {"unknown"}}))
	if f.Matched != 2 || f.Total != 6 || len(f.Rollup) != 4 {
		t.Fatalf("health filter = matched %d total %d rollup %v", f.Matched, f.Total, f.Rollup)
	}
	for _, g := range f.Rollup {
		wantN := map[Health]int{HealthFailed: 1, HealthDegraded: 1, HealthUnknown: 2, HealthHealthy: 2}[g.Health]
		if g.Count != wantN {
			t.Errorf("rollup %s = %d, want %d", g.Health, g.Count, wantN)
		}
		if g.Health == HealthHealthy && (strings.Contains(strings.Join(g.Services, ","), "remote") || strings.Contains(strings.Join(g.Services, ","), "unrun")) {
			t.Error("unknown projects must never roll up as healthy")
		}
	}
	for _, r := range f.Rows {
		if r.Health != HealthUnknown {
			t.Errorf("filter health=unknown kept %s (%s)", r.Service, r.Health)
		}
	}
	// Sort by symbols, both directions.
	s := FleetTable(p, ParseFleetQuery(url.Values{"sort": {"symbols"}, "dir": {"asc"}}))
	if s.Rows[0].Service != "remote" || s.Rows[len(s.Rows)-1].Service != "busy" {
		t.Errorf("symbols asc = %s … %s", s.Rows[0].Service, s.Rows[len(s.Rows)-1].Service)
	}
	s = FleetTable(p, ParseFleetQuery(url.Values{"sort": {"symbols"}}))
	if s.Query.Dir != "desc" || s.Rows[0].Service != "busy" {
		t.Errorf("symbols default = %s first (dir %s)", s.Rows[0].Service, s.Query.Dir)
	}
	// Sort by name, both directions — by the name, never by calls. Make the
	// last name the busiest so calls order and name order differ.
	named := fleetFixture()
	named.Rows[len(named.Rows)-1].Traffic = []int{9000, 0, 0, 0, 0, 0, 0}
	for dir, want := range map[string]string{"asc": "busy,failing,quiet,remote,stale,unrun", "desc": "unrun,stale,remote,quiet,failing,busy"} {
		n := FleetTable(named, ParseFleetQuery(url.Values{"sort": {"service"}, "dir": {dir}}))
		var got []string
		for _, r := range n.Rows {
			got = append(got, r.Service)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("sort=service dir=%s = %s, want %s", dir, strings.Join(got, ","), want)
		}
	}
	// Certificates: a project with none declared is not measured, and
	// sorts after every measured one in both directions.
	for _, dir := range []string{"asc", "desc"} {
		c := FleetTable(p, ParseFleetQuery(url.Values{"sort": {"certs"}, "dir": {dir}}))
		if c.Rows[0].Service != "busy" || c.Rows[1].CertsLive.Known() {
			t.Errorf("certs %s: %s first, then a measured %v", dir, c.Rows[0].Service, c.Rows[1].CertsLive)
		}
	}
	// Name and language filters; unknown parameters fall back to defaults.
	if n := FleetTable(p, ParseFleetQuery(url.Values{"q": {"STA"}})); n.Matched != 1 || n.Rows[0].Service != "stale" {
		t.Errorf("q filter = %d", n.Matched)
	}
	if l := FleetTable(p, ParseFleetQuery(url.Values{"lang": {"TypeScript"}})); l.Matched != 1 || l.Rows[0].Service != "failing" {
		t.Errorf("lang filter = %d", l.Matched)
	}
	if b := ParseFleetQuery(url.Values{"sort": {"nope"}, "dir": {"sideways"}, "health": {"green"}, "page": {"-3"}}); b.Sort != "unrealized" || b.Dir != "desc" || b.Health != "" || b.Page != 1 {
		t.Errorf("bad query normalized to %+v", b)
	}
	if v := ParseFleetQuery(url.Values{"sort": {"calls"}, "health": {"failed"}}).Values(); v.Get("sort") != "calls" || v.Get("dir") != "desc" || v.Get("health") != "failed" {
		t.Errorf("query values = %v", v)
	}
	// A pulse is the top rows by a column.
	top := FleetTable(p, FleetQuery{Sort: "calls", Dir: "desc", Limit: 2})
	if len(top.Rows) != 2 || top.Rows[0].Service != "busy" || top.Rows[1].Service != "failing" || top.Matched != 6 {
		t.Errorf("pulse = %v (matched %d)", []string{top.Rows[0].Service, top.Rows[1].Service}, top.Matched)
	}
	// Paging past 50.
	many := Projects{Days: p.Days, Logged: p.Logged}
	for i := 0; i < 120; i++ {
		many.Rows = append(many.Rows, ProjectFacts{Service: "svc-" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Freshness: "fresh"})
	}
	pg := FleetTable(many, ParseFleetQuery(url.Values{"page": {"3"}}))
	if pg.Pages != 3 || len(pg.Rows) != 20 || pg.Query.Page != 3 || pg.ShownFrom != 101 || pg.ShownTo != 120 || pg.Matched != 120 {
		t.Errorf("paging = %d pages, %d rows (%d–%d of %d) on page %d", pg.Pages, len(pg.Rows), pg.ShownFrom, pg.ShownTo, pg.Matched, pg.Query.Page)
	}
	if first := FleetTable(many, ParseFleetQuery(url.Values{})); first.ShownFrom != 1 || first.ShownTo != 50 {
		t.Errorf("page 1 shows rows %d–%d", first.ShownFrom, first.ShownTo)
	}
	if none := FleetTable(many, ParseFleetQuery(url.Values{"q": {"no-such-project"}})); none.ShownFrom != 0 || none.ShownTo != 0 {
		t.Errorf("no match shows rows %d–%d", none.ShownFrom, none.ShownTo)
	}
	// The shared metrics over the same facts.
	if v := WiredAndFresh(p); v.State != StatePartial || v.V != 4 || !strings.Contains(v.Reason, "1 of 6") {
		t.Errorf("wired and fresh = %+v", v)
	}
	pm := ProjectMetrics(p)
	if v := pm[MetricClaims]; v.State != StatePresent || v.V != 324 || !strings.Contains(v.Denominator, "318 fresh · 6 stale") {
		t.Errorf("claims current = %+v", v)
	}
	if v := pm[MetricCertLive]; v.State != StatePresent || v.V != 1 || !strings.Contains(v.Denominator, "of 4 targets in 1 repo") {
		t.Errorf("certify live = %+v", v)
	}
	q := p
	q.Store = SourceRead{Name: SourceStore, State: StateAbsent, Reason: "no store recorded for this daemon"}
	if v := ProjectMetrics(q)[MetricClaims]; v.State != StateAbsent || v.Reason != "no store recorded for this daemon" {
		t.Errorf("claims with no store = %+v", v)
	}
}

// TestFleetTable_StaleProofIsNeverFailedOrHealthy: a gate failure recorded
// at a commit the project has moved past is history. The project is not
// "failed" on it (the gate has not spoken of HEAD) and not healthy either
// (unknown is never green); its plans value is stale, carrying the commit
// and age; the shared metric over it is stale too; and a failure at HEAD
// still fails.
func TestFleetTable_StaleProofIsNeverFailedOrHealthy(t *testing.T) {
	p := fleetFixture()
	p.Now = time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC)
	for i := range p.Rows {
		switch p.Rows[i].Service {
		case "failing":
			p.Rows[i].RepoHead = "3d58a0db490552d76bd15876f6b12cbbacd492c2"
			p.Rows[i].Runs[0].At, p.Rows[i].Runs[0].Commit = "2026-09-19T21:00:33Z", "d8770983"
			p.Rows[i].Runs[0].StaleReason = "recorded at d8770983; HEAD is 3d58a0db"
		case "busy":
			p.Rows[i].PlanFiles = []string{"p.md", "q.md"} // q.md: no run ever
		}
	}
	m := FleetTable(p, FleetQuery{})
	f := rowByName(m.Rows, "failing")
	if f.Health != HealthUnknown || !strings.Contains(f.HealthWhy, "before HEAD") || !strings.Contains(f.HealthWhy, "3 failing then") {
		t.Errorf("stale failure: health %s (%s); want unknown, naming the older run", f.Health, f.HealthWhy)
	}
	if f.PlansVerified.State != StateStale || f.PlansVerified.V != 6 || !strings.Contains(f.PlansVerified.Reason, "d8770983") ||
		!strings.Contains(f.PlansVerified.Reason, "6 d old") || f.PlansVerified.AsOf.IsZero() {
		t.Errorf("stale plans value = %+v", f.PlansVerified)
	}
	if f.Plans == nil || f.Plans.Failing != 0 || f.Plans.StaleFailing != 3 {
		t.Errorf("stale proof = %+v", f.Plans)
	}
	for _, g := range m.Rollup {
		if g.Health == HealthFailed && g.Count != 0 {
			t.Errorf("nothing fails at HEAD, yet the rollup lists failed: %v", g.Services)
		}
	}
	b := rowByName(m.Rows, "busy")
	if b.Health != HealthUnknown || !strings.Contains(b.HealthWhy, "1 plan with no recorded verify run") || b.PlansVerified.State != StatePartial {
		t.Errorf("a plan no run checked: health %s (%s), plans %+v", b.Health, b.HealthWhy, b.PlansVerified)
	}
	// The same run at HEAD fails.
	p.Rows[2].Runs[0].StaleReason = ""
	if f := rowByName(FleetTable(p, FleetQuery{}).Rows, "failing"); f.Health != HealthFailed || f.PlansVerified.State != StatePresent {
		t.Errorf("a failure at HEAD: %s %+v", f.Health, f.PlansVerified)
	}
}

// TestFleetTable_SharedScaleAndAbsentDays pins OC-20: every project's
// sparkline shares one scale — the fleet's busiest measured day — and a day
// the call log holds no row at all is absent for every project, splitting
// the line there rather than drawing across it; a quiet day elsewhere is a
// measured zero.
func TestFleetTable_SharedScaleAndAbsentDays(t *testing.T) {
	p := fleetFixture()
	m := FleetTable(p, FleetQuery{})
	if m.ScaleMax != 400 {
		t.Fatalf("shared scale = %d, want the fleet's busiest measured day (400)", m.ScaleMax)
	}
	if strings.Join(m.AbsentDays, ",") != "2026-09-21" {
		t.Errorf("absent days = %v", m.AbsentDays)
	}
	for _, r := range m.Rows {
		s := r.Series
		if s.Max != m.ScaleMax {
			t.Errorf("%s draws on scale %d, not the shared %d", r.Service, s.Max, m.ScaleMax)
		}
		if len(s.Points) != 7 || s.Measured != 6 || s.Absent != 1 {
			t.Errorf("%s: %d points, %d measured, %d absent", r.Service, len(s.Points), s.Measured, s.Absent)
		}
		abs := s.Points[2]
		if abs.Calls != nil || !strings.Contains(abs.Absent, "no call-log rows for 2026-09-21") {
			t.Errorf("%s: the absent day is %+v", r.Service, abs)
		}
		// Two runs, split at the absent day; no run spans it.
		if len(s.Runs) != 2 || s.Runs[0] != [2]int{0, 1} || s.Runs[1] != [2]int{3, 6} {
			t.Errorf("%s: runs = %v, want [[0 1] [3 6]]", r.Service, s.Runs)
		}
		for _, run := range s.Runs {
			for i := run[0]; i <= run[1]; i++ {
				if s.Points[i].Calls == nil {
					t.Errorf("%s: run %v crosses an absent day", r.Service, run)
				}
			}
		}
		// A lower bound: one day in the window was never observed.
		if r.Calls.State != StatePartial || !strings.Contains(r.Calls.Reason, "1 of 7 days") {
			t.Errorf("%s: 14-day calls = %+v", r.Service, r.Calls)
		}
	}
	busy := rowByName(m.Rows, "busy")
	if busy.Calls.V != 900 || *busy.Series.Points[5].Calls != 0 {
		t.Errorf("busy: %d calls, day 6 = %v (a measured zero)", busy.Calls.V, busy.Series.Points[5].Calls)
	}
	quiet := rowByName(m.Rows, "quiet")
	if quiet.Series.Max != 400 || *quiet.Series.Points[3].Calls != 2 {
		t.Error("the quiet project draws 2 calls on the fleet's 0–400 scale, so it looks quiet")
	}

	// No call log read: every day is absent with the reason, and no figure
	// is a zero.
	p.CallLogNote = "call log unreadable: operation not permitted"
	n := FleetTable(p, FleetQuery{})
	for _, r := range n.Rows {
		if r.Calls.State != StateAbsent || r.Series.Measured != 0 || len(r.Series.Runs) != 0 || r.Series.Points[0].Absent != p.CallLogNote {
			t.Errorf("%s with no log: calls %+v series %+v", r.Service, r.Calls, r.Series)
		}
	}
	if n.ScaleMax != 0 {
		t.Errorf("no log: scale %d", n.ScaleMax)
	}
	raw, _ := json.Marshal(n.Rows[0].Calls)
	if strings.Contains(string(raw), `"v"`) {
		t.Errorf("an absent figure carries no number: %s", raw)
	}
}

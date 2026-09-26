package main

// console_now_test.go — wave B's pages (plan section B) on the fixture
// fleet, through the real handler: the Now page's attention queue, "since
// you last looked" and fleet pulse, and the Fleet page's sortable,
// filterable table and health rollup — each with its verb chips, its truth
// states, role utilities only, and links that resolve to registered routes.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// withWaveBLedgers writes a verify ledger with one failing and unproven
// plan run at payment-service's registered root, and a page-view log whose
// last view of Now was two hours before the fixture's clock.
func withWaveBLedgers(t *testing.T, f *consoleFixture) string {
	t.Helper()
	prev := consolePlanExists
	consolePlanExists = func(string, string) bool { return true } // the fauxtown repos carry no plan files
	t.Cleanup(func() { consolePlanExists = prev })
	// The root the fixture store recorded for the project (the registry's
	// answer — the verify ledger is matched against it).
	root, _, found, err := f.client.ServiceMeta(t.Context(), "payment-service")
	if err != nil || !found || root == "" {
		t.Fatalf("payment-service has no recorded root: %v", err)
	}
	rows := []map[string]any{
		{"schema_version": 2, "at": "2026-09-20T10:00:00Z", "repo_root": root, "service": "payment-service", "plan": "docs/plans/pay.md", "commit": "abc1234", "total": 8, "claimed": 6, "verified": 3, "failing": 1, "origin": "cli"},
		{"schema_version": 2, "at": "2026-09-25T10:00:00Z", "repo_root": "/tmp/a-scratch-clone", "service": "payment-service", "plan": "docs/plans/pay.md", "total": 8, "claimed": 6, "verified": 6, "origin": "cli"},
	}
	var ledger bytes.Buffer
	for _, r := range rows {
		b, _ := json.Marshal(r)
		ledger.Write(append(b, '\n'))
	}
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), ledger.String())
	mustWrite(t, f.viewsLog, `{"schema_version":1,"at":"`+f.now.Add(-2*time.Hour).Format(time.RFC3339)+`","route":"/{$}","page":"home"}`+"\n")
	consoleProjectsCache.Lock()
	consoleProjectsCache.key = ""
	consoleProjectsCache.Unlock()
	return root
}

// assertLinksResolve holds every internal link in a rendered page to a
// route the console (or the daemon beside it) serves.
func assertLinksResolve(t *testing.T, page, body string) {
	t.Helper()
	mux := http.NewServeMux()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, p := range defaultConsoleRegistry().pages {
		mux.Handle(p.Route, ok)
	}
	for _, r := range []string{"/metrics", "/healthz", "/events", "/assets/", "/observatory/"} {
		mux.Handle(r, ok)
	}
	for _, m := range regexp.MustCompile(`href="(/[^"]*)"`).FindAllStringSubmatch(body, -1) {
		href := strings.ReplaceAll(m[1], "&amp;", "&")
		u, err := url.Parse(href)
		if err != nil {
			t.Errorf("%s: unparsable link %q", page, href)
			continue
		}
		if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, u.RequestURI(), nil)); pattern == "" {
			t.Errorf("%s links %q, which no route serves", page, href)
		}
	}
}

// TestConsoleNow_QueueSinceAndPulse renders the Now page on the fixture
// fleet: the attention queue in proof-blocking order with evidence and
// remedies, "since you last looked" marking what is new, the pulse's
// shared-scale sparklines, the verbs it renders and the metrics it shows.
func TestConsoleNow_QueueSinceAndPulse(t *testing.T) {
	f := newConsoleFixture(t)
	root := withWaveBLedgers(t, f)

	env := f.pageJSON(t, "/")
	if strings.Join(env.Verbs, "|") != "fathom brief --json|fathom fleet report --json" {
		t.Fatalf("Now renders %v", env.Verbs)
	}
	var rep nowReport
	if err := json.Unmarshal(env.Model, &rep); err != nil {
		t.Fatal(err)
	}
	var section nowReport
	if err := json.Unmarshal(env.Sections["fathom brief --json"], &section); err != nil {
		t.Fatal(err)
	}
	if rep.Schema != nowReportSchema || rep.GeneratedAt != section.GeneratedAt || len(rep.Attention.Items) != len(section.Attention.Items) {
		t.Errorf("the page's model and its brief section differ: %+v / %+v", rep.Summary, section.Summary)
	}
	a := rep.Attention
	if len(a.Items) < 2 || a.Items[0].Kind != console.AttentionGate || a.Items[1].Kind != console.AttentionUnproven {
		t.Fatalf("attention = %+v", a.Items)
	}
	gate := a.Items[0]
	if gate.Service != "payment-service" || gate.Subject != "docs/plans/pay.md" || gate.Count != 1 || !strings.Contains(gate.Remedy, "fathom verify --service payment-service --plan docs/plans/pay.md") || gate.Evidence.Href != "/projects/payment-service?tab=plans" {
		t.Errorf("gate item = %+v (the registered root's run, never the scratch clone's)", gate)
	}
	if a.Items[1].Count != 2 {
		t.Errorf("unproven = %+v", a.Items[1])
	}
	for i := 1; i < len(a.Items); i++ {
		if a.Items[i].Kind.Rank() < a.Items[i-1].Kind.Rank() {
			t.Errorf("item %d (%s) ranks after a less proof-blocking one", i, a.Items[i].Kind)
		}
	}
	// The fixture fleet's store: claims and certificates read (no targets
	// declared: absent, never 0), the ledger and the store named as read.
	if rep.Summary.CertificatesLive.State != console.StateAbsent || rep.Summary.PlanRowsVerified.V != 3 || rep.Summary.GateRefusals.V != 1 || rep.Summary.GateRefusals.State != console.StatePresent {
		t.Errorf("summary = %+v", rep.Summary)
	}
	// Since you last looked: the previous view two hours ago; the daemon's
	// start (its record) is new since.
	s := rep.Since
	if s.LastLook != f.now.Add(-2*time.Hour).UTC().Format(time.RFC3339) || s.New < 1 || s.Items[0].Kind != console.ChangeRestart || !s.Items[0].New {
		t.Errorf("since = last look %q, %d new, items %+v", s.LastLook, s.New, s.Items)
	}
	// The pulse: every row on one scale.
	if len(rep.Pulse.Rows) == 0 || rep.Pulse.Query.Sort != "calls" {
		t.Fatalf("pulse = %+v", rep.Pulse)
	}
	for _, r := range rep.Pulse.Rows {
		if r.Series.Max != rep.Pulse.ScaleMax {
			t.Errorf("%s on its own scale %d, not the shared %d", r.Service, r.Series.Max, rep.Pulse.ScaleMax)
		}
	}

	code, _, body := f.get(t, "/")
	if code != http.StatusOK || strings.Count(body, "<h1") != 1 {
		t.Fatalf("GET / = %d, %d h1", code, strings.Count(body, "<h1"))
	}
	for _, want := range []string{
		`data-console-copy="fathom brief --json"`, `data-console-copy="fathom fleet report --json"`,
		`id="attention"`, `id="since"`, `id="pulse"`, `data-attention-item data-kind="gate"`,
		`data-console-copy="cd ` + root + ` &amp;&amp; fathom verify --service payment-service --plan docs/plans/pay.md"`,
		`href="/projects/payment-service?tab=plans"`, `data-change="restart"`, `data-shared-spark`,
		"Calls per day as a table", `data-last-look>Last look ` + f.now.Add(-2*time.Hour).UTC().Format("01-02 15:04") + ` UTC (2 h ago).`,
		"Machine surfaces and the four questions",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Now is missing %q", want)
		}
	}
	// The gate item precedes the unproven one in the rendered list too.
	if strings.Index(body, `data-kind="gate"`) > strings.Index(body, `data-kind="unproven"`) {
		t.Error("the rendered queue is not in proof-blocking order")
	}
	assertRoleUtilitiesOnly(t, body)
	assertLinksResolve(t, "/", body)

	// The verb prints the page's section (P10): the CLI reads the store the
	// daemon recorded, read-only.
	var out, errb bytes.Buffer
	if code := run([]string{"brief", "--json", "--url", f.ts.URL}, &out, &errb); code != 0 {
		t.Fatalf("brief --json = %d: %s", code, errb.String())
	}
	var fromVerb, fromPage any
	_ = json.Unmarshal(out.Bytes(), &fromVerb)
	_ = json.Unmarshal(env.Sections["fathom brief --json"], &fromPage)
	vb, _ := json.Marshal(fromVerb)
	pb, _ := json.Marshal(fromPage)
	if !bytes.Equal(vb, pb) {
		t.Errorf("brief --json differs from the page:\nverb %s\npage %s", vb, pb)
	}
}

// TestConsoleFleet_TableSortFilterAndRollup renders the Fleet page: the
// table sorts and filters by URL, the rollup counts every project by
// health (unknown never healthy), the table's shared columns agree with
// the fleet report the page carries, and every chart has its table.
func TestConsoleFleet_TableSortFilterAndRollup(t *testing.T) {
	f := newConsoleFixture(t)
	withWaveBLedgers(t, f)

	env := f.pageJSON(t, "/fleet?sort=symbols&dir=asc")
	var m console.FleetTableModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if m.Total != len(consoleFixtureServices) || len(m.Rows) != m.Total || m.Query.Sort != "symbols" || m.Query.Dir != "asc" {
		t.Fatalf("fleet model = total %d rows %d query %+v", m.Total, len(m.Rows), m.Query)
	}
	for i := 1; i < len(m.Rows); i++ {
		if m.Rows[i].Symbols.V < m.Rows[i-1].Symbols.V {
			t.Errorf("symbols ascending broken at %s", m.Rows[i].Service)
		}
	}
	// payment-service fails its gate (the registered root's newest run).
	var failed int
	for _, r := range m.Rows {
		if r.Service == "payment-service" && r.Health != console.HealthFailed {
			t.Errorf("payment-service health = %s (%s)", r.Health, r.HealthWhy)
		}
		if r.Health == console.HealthFailed {
			failed++
		}
		if r.Health == console.HealthHealthy && (r.Freshness != "fresh" || r.HealthWhy == "") {
			t.Errorf("%s reads healthy with freshness %s", r.Service, r.Freshness)
		}
	}
	for _, g := range m.Rollup {
		if g.Health == console.HealthFailed && g.Count != failed {
			t.Errorf("rollup failed = %d, table %d", g.Count, failed)
		}
	}
	// The table's shared columns are the fleet report's (P10).
	var rep fleetReport
	if err := json.Unmarshal(env.Sections["fathom fleet report --json"], &rep); err != nil {
		t.Fatal(err)
	}
	for _, sr := range rep.Services {
		for _, r := range m.Rows {
			if r.Service != sr.Service {
				continue
			}
			if r.Symbols.V != sr.Symbols || r.Freshness != console.FreshnessClass(sr.Freshness) {
				t.Errorf("%s: table says %d symbols, %s; the fleet report %d, %s", r.Service, r.Symbols.V, r.Freshness, sr.Symbols, sr.Freshness)
			}
		}
	}
	// Filter by health.
	var filtered console.FleetTableModel
	if err := json.Unmarshal(f.pageJSON(t, "/fleet?health=failed").Model, &filtered); err != nil {
		t.Fatal(err)
	}
	if filtered.Matched != failed || filtered.Total != len(consoleFixtureServices) {
		t.Errorf("health=failed kept %d of %d", filtered.Matched, filtered.Total)
	}

	_, _, body := f.get(t, "/fleet?sort=symbols&dir=asc")
	for _, want := range []string{
		`data-console-copy="fathom fleet report --json"`, `role="tree" aria-label="projects by health"`,
		`href="/fleet?health=failed"`, `data-fleet-filters`, `name="health"`, `name="q"`, `name="lang"`,
		`<input type="hidden" name="sort" value="symbols">`,
		`data-sort="symbols"`, `sorted ascending; activate to sort descending`, `href="/fleet?dir=desc&amp;sort=symbols"`,
		`data-table="fleet-projects"`, `data-table="fleet-days"`, `data-shared-spark`, `data-health="failed"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/fleet is missing %q", want)
		}
	}
	if got := tileValue(t, body, "live certificates"); got != "—" {
		t.Errorf("no certification target anywhere: the tile is absent, never 0 (got %q)", got)
	}
	assertRoleUtilitiesOnly(t, body)
	assertLinksResolve(t, "/fleet", body)

	// An unknown filter value is ignored, never an empty table.
	var odd console.FleetTableModel
	if err := json.Unmarshal(f.pageJSON(t, "/fleet?health=green&sort=nope").Model, &odd); err != nil {
		t.Fatal(err)
	}
	if odd.Matched != len(consoleFixtureServices) || odd.Query.Sort != "unrealized" {
		t.Errorf("bad query = %+v", odd.Query)
	}
}

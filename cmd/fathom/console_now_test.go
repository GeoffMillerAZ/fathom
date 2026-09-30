package main

// console_now_test.go — wave B's pages (plan section B) on the fixture
// fleet, through the real handler: the Now page's attention queue, "since
// you last looked" and fleet pulse, and the Fleet page's sortable,
// filterable table and health rollup — each with its verb chips, its truth
// states, role utilities only, and links that resolve to registered routes.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// withWaveBLedgers writes a verify ledger with one failing and unproven
// plan run at payment-service's registered root at its HEAD, and a
// page-view log whose last view of Now was two hours before the fixture's
// clock. The fixture must be Rooted (a project with a readable HEAD).
func withWaveBLedgers(t *testing.T, f *consoleFixture) string {
	t.Helper()
	root, head := rootedProject(t, f, "payment-service")
	rows := []map[string]any{
		{"schema_version": 2, "at": "2026-09-20T10:00:00Z", "repo_root": root, "service": "payment-service", "plan": "docs/plans/pay.md", "commit": head, "total": 8, "claimed": 6, "verified": 3, "failing": 1, "origin": "cli"},
		{"schema_version": 2, "at": "2026-09-25T10:00:00Z", "repo_root": "/tmp/a-scratch-clone", "service": "payment-service", "plan": "docs/plans/pay.md", "total": 8, "claimed": 6, "verified": 6, "origin": "cli"},
	}
	writeVerifyLedger(t, rows)
	mustWrite(t, f.viewsLog, `{"schema_version":1,"at":"`+f.now.Add(-2*time.Hour).Format(time.RFC3339)+`","route":"/{$}","page":"home"}`+"\n")
	resetObsCache()
	return root
}

// rootedProject is a Rooted fixture project's registered root and its
// HEAD as `fathom verify` records it; plan files the fauxtown repos lack
// are taken as present.
func rootedProject(t *testing.T, f *consoleFixture, svc string) (root, head string) {
	t.Helper()
	prev := consolePlanExists
	consolePlanExists = func(string, string) bool { return true } // the fauxtown repos carry no plan files
	t.Cleanup(func() { consolePlanExists = prev })
	// The root the fixture store recorded for the project (the registry's
	// answer — the verify ledger is matched against it).
	root, _, found, err := f.client.ServiceMeta(t.Context(), svc)
	if err != nil || !found || root == "" {
		t.Fatalf("%s has no recorded root: %v", svc, err)
	}
	head = gitHeadShort(root)
	if head == "" {
		t.Skipf("%s's root %s has no readable HEAD (not a git checkout?)", svc, root)
	}
	return root, head
}

// writeVerifyLedger replaces the fixture's verify ledger with rows.
func writeVerifyLedger(t *testing.T, rows []map[string]any) {
	t.Helper()
	var ledger bytes.Buffer
	for _, r := range rows {
		b, _ := json.Marshal(r)
		ledger.Write(append(b, '\n'))
	}
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), ledger.String())
	resetObsCache()
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
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{Rooted: true})
	root := withWaveBLedgers(t, f)

	env := f.pageJSON(t, "/")
	if strings.Join(env.Verbs, "|") != "fathom fleet report --json" {
		t.Fatalf("Now renders %v", env.Verbs)
	}
	if strings.Join(env.ProjectVerbs, "|") != verbPlansStatusFor+"|"+verbCertifyStatusFor {
		t.Fatalf("Now renders per project %v", env.ProjectVerbs)
	}
	var rep nowReport
	if err := json.Unmarshal(env.Model, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Schema != nowReportSchema {
		t.Errorf("the page's model schema = %q, want %q", rep.Schema, nowReportSchema)
	}
	a := rep.Attention
	if len(a.Items) < 2 || a.Items[0].Kind != console.AttentionGate || a.Items[1].Kind != console.AttentionUnproven {
		t.Fatalf("attention = %+v", a.Items)
	}
	gate := a.Items[0]
	if gate.Service != "payment-service" || gate.Subject != "docs/plans/pay.md" || gate.Count != 1 || !strings.Contains(gate.Remedy, "fathom verify --service payment-service --plan docs/plans/pay.md") || gate.Evidence.Href != "/plans?svc=payment-service" {
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
	if rep.Summary.CertifyLive.State != console.StateAbsent || rep.Summary.GateRefusals.V != 1 || rep.Summary.GateRefusals.State != console.StatePresent {
		t.Errorf("summary = %+v", rep.Summary)
	}
	// One metric per question (OC-H3): the summary's claims and
	// certificates are the page's shared metrics themselves.
	for id, v := range map[console.MetricID]console.Value[int]{console.MetricClaims: rep.Summary.ClaimsCurrent, console.MetricCertLive: rep.Summary.CertifyLive} {
		var shared console.Value[int]
		if err := json.Unmarshal(env.Metrics[string(id)], &shared); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v, shared) {
			t.Errorf("summary %s = %+v, the shared metric %+v", id, v, shared)
		}
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
		`data-console-copy="fathom fleet report --json"`,
		`id="attention"`, `id="since"`, `id="pulse"`, `data-attention-item data-kind="gate"`,
		`data-console-copy="cd ` + root + ` &amp;&amp; fathom verify --service payment-service --plan docs/plans/pay.md"`,
		`href="/plans?svc=payment-service"`, `data-change="restart"`, `data-shared-spark`,
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
}

// TestConsoleFleet_TableSortFilterAndRollup renders the Fleet page: the
// table sorts and filters by URL, the rollup counts every project by
// health (unknown never healthy), the table's shared columns agree with
// the fleet report the page carries, and every chart has its table.
func TestConsoleFleet_TableSortFilterAndRollup(t *testing.T) {
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{Rooted: true})
	withWaveBLedgers(t, f)

	env := f.pageJSON(t, "/fleet?sort=symbols&dir=asc")
	var m console.FleetTableModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if m.Total != len(consoleRootedFixtureServices()) || len(m.Rows) != m.Total || m.Query.Sort != "symbols" || m.Query.Dir != "asc" {
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
	if filtered.Matched != failed || filtered.Total != len(consoleRootedFixtureServices()) {
		t.Errorf("health=failed kept %d of %d", filtered.Matched, filtered.Total)
	}

	_, _, body := f.get(t, "/fleet?sort=symbols&dir=asc")
	for _, want := range []string{
		`data-console-copy="fathom fleet report --json"`, `role="tree" aria-label="projects by health"`,
		`href="/fleet?health=failed"`, `data-fleet-filters`, `name="health"`, `name="q"`, `name="lang"`,
		`<select class="`, `id="fleet-sort" name="sort"`, `<option value="symbols" selected>Symbols</option>`, `<option value="asc" selected>Ascending</option>`,
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
	if odd.Matched != len(consoleRootedFixtureServices()) || odd.Query.Sort != "unrealized" {
		t.Errorf("bad query = %+v", odd.Query)
	}
}

// TestConsoleNow_GateStateIsTheRunAtHEAD is the survey's defect through
// the real handler: the registered root's newest run failed at an older
// commit, while a newer run passed at the commit the root is on. Now and
// Fleet read the run at HEAD: no gate failure, no "failed" health. A plan
// whose only run is the old one is history — stale, in the past tense with
// its commit, never "fails". PE-24: a detached clone's run is not the
// project's (its root is not registered), so its failure at HEAD is
// ignored, never a gate item.
func TestConsoleNow_GateStateIsTheRunAtHEAD(t *testing.T) {
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{Rooted: true})
	root, head := rootedProject(t, f, "payment-service")
	writeVerifyLedger(t, []map[string]any{
		{"schema_version": 2, "at": "2026-09-19T21:00:33Z", "repo_root": root, "service": "payment-service", "plan": "docs/plans/receipts.md", "commit": "d8770983", "total": 45, "claimed": 45, "verified": 42, "failing": 3, "origin": "cli"},
		{"schema_version": 2, "at": "2026-09-19T21:00:33Z", "repo_root": root, "service": "payment-service", "plan": "docs/plans/toolident.md", "commit": "d8770983", "total": 10, "claimed": 8, "verified": 7, "failing": 1, "origin": "cli"},
		{"schema_version": 2, "at": "2026-09-25T02:58:00Z", "repo_root": root, "service": "payment-service", "plan": "docs/plans/receipts.md", "commit": head, "total": 45, "claimed": 45, "verified": 45, "origin": "cli"},
		// A push clone's later run at HEAD that failed: an unregistered root, ignored.
		{"schema_version": 2, "at": "2026-09-25T03:30:00Z", "repo_root": "/tmp/scratchpad/pushclone", "service": "payment-service", "plan": "docs/plans/receipts.md", "commit": head, "total": 45, "claimed": 45, "verified": 40, "failing": 5, "origin": "cli"},
		// Another service at the same commit is another index: not this project's proof.
		{"schema_version": 2, "at": "2026-09-25T03:00:00Z", "repo_root": "/tmp/bench", "service": "payment-bench", "plan": "docs/plans/toolident.md", "commit": head, "total": 10, "claimed": 8, "verified": 8, "origin": "cli"},
	})

	var rep nowReport
	if err := json.Unmarshal(f.pageJSON(t, "/").Model, &rep); err != nil {
		t.Fatal(err)
	}
	var stale *console.AttentionItem
	for i, it := range rep.Attention.Items {
		if it.Kind == console.AttentionGate {
			t.Errorf("a failure the gate disproved at HEAD is on the queue: %+v", it)
		}
		if it.Service == "payment-service" && it.State == console.StateStale {
			stale = &rep.Attention.Items[i]
		}
	}
	if stale == nil || stale.Subject != "docs/plans/toolident.md" || !strings.Contains(stale.Title, "was last verified before HEAD") ||
		!strings.Contains(stale.Detail, "d8770983") || !strings.Contains(stale.Detail, "failed 1") || strings.Contains(stale.Title, "fails") {
		t.Errorf("stale item = %+v", stale)
	}

	var fleet console.FleetTableModel
	if err := json.Unmarshal(f.pageJSON(t, "/fleet").Model, &fleet); err != nil {
		t.Fatal(err)
	}
	for _, r := range fleet.Rows {
		if r.Service != "payment-service" {
			continue
		}
		if r.Health == console.HealthFailed || strings.Contains(r.HealthWhy, "fail the gate") {
			t.Errorf("payment-service health = %s (%s): nothing fails at HEAD", r.Health, r.HealthWhy)
		}
		if r.PlansVerified.State != console.StateStale || r.Plans == nil || r.Plans.Failing != 0 || r.Plans.StaleFailing != 1 {
			t.Errorf("payment-service plans = %+v / %+v", r.PlansVerified, r.Plans)
		}
	}
	for _, g := range fleet.Rollup {
		if g.Health == console.HealthFailed && g.Count > 0 {
			t.Errorf("rollup failed = %v", g.Services)
		}
	}
	_, _, body := f.get(t, "/fleet")
	if !strings.Contains(body, `data-truth-state="stale"`) || !strings.Contains(body, "verified before HEAD") {
		t.Error("the fleet table does not draw the stale proof as stale")
	}
}

// TestConsoleNow_SincePagesEveryChange renders "since you last looked"
// over 136 changes: every one is new (no earlier look) and the line says
// 136, the page shows the first 50 with the range named, and the pager
// links to the pages holding the rest — nothing past 50 is dropped.
func TestConsoleNow_SincePagesEveryChange(t *testing.T) {
	now := time.Date(2026, 9, 25, 20, 0, 0, 0, time.UTC)
	var changes []console.Change
	for i := 0; i < 136; i++ {
		changes = append(changes, console.Change{Kind: console.ChangePush, At: now.Add(-time.Duration(i+1) * time.Hour).Format(time.RFC3339), Service: "svc", Title: "Pushed", Detail: "svc origin/main", Ref: strconv.Itoa(i)})
	}
	src := []console.SourceRead{{Name: "git reflogs", State: console.StatePresent}}
	s := console.SinceLastLook(console.SinceInputs{Now: now, Changes: changes, Sources: src, Limit: 50})
	body := renderSincePanel(s, now)
	for _, want := range []string{
		"No earlier look is recorded. 136 changes new since.", "Changes 1–50 of 136, newest first.",
		`href="/?since_page=2#since"`, `href="/?since_page=3#since"`, "Show 42 changes earlier on this page",
		"denominator: 136 changes since", "page 1 of 3",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("since panel is missing %q", want)
		}
	}
	if n := strings.Count(body, "data-change="); n != 50 {
		t.Errorf("page 1 lists %d changes, want 50", n)
	}
	last := renderSincePanel(console.SinceLastLook(console.SinceInputs{Now: now, Changes: changes, Sources: src, Limit: 50, Page: 3}), now)
	if !strings.Contains(last, "Changes 101–136 of 136") || strings.Count(last, "data-change=") != 36 {
		t.Error("page 3 does not list changes 101–136")
	}
	assertLinksResolve(t, "since panel", body)
}

// TestConsoleSince_UnreadableReposAreOneBoundedWrappingLine: under the
// volume's EPERM wedge every project's reflog read fails; the panel's
// reason names three projects and how many more, with each distinct
// failure once and its path relative to the repository — and the truth
// line wraps anywhere, so a long path never runs off the card.
func TestConsoleSince_UnreadableReposAreOneBoundedWrappingLine(t *testing.T) {
	var u unreadRepos
	for _, svc := range []string{"fathom", "platform-engineering", "resume_as_code", "app-kit", "conjure", "imbue"} {
		root := "/Volumes/m2ext/code/geoffmilleraz/" + svc
		u.add(svc, &fs.PathError{Op: "open", Path: root + "/.git/logs/refs/remotes/origin/main", Err: syscall.EPERM}, root)
	}
	r := u.reason()
	if len(r) > 200 || !strings.Contains(r, "fathom, platform-engineering, resume_as_code and 3 more") ||
		strings.Count(r, "operation not permitted") != 1 || strings.Contains(r, "/Volumes/") {
		t.Errorf("reason (%d chars) = %q", len(r), r)
	}
	s := console.SinceLastLook(console.SinceInputs{Now: time.Now(), Sources: []console.SourceRead{{Name: "git reflogs", State: console.StateError, Reason: r}, {Name: "call log", State: console.StatePresent}}})
	panel := renderSincePanel(s, time.Now())
	if !regexp.MustCompile(`<p class="[^"]*\[overflow-wrap:anywhere\][^"]*" data-truth-line="partial">`).MatchString(panel) {
		t.Errorf("the truth line does not wrap: %s", panel)
	}
	for _, st := range []console.State{console.StateAbsent, console.StateError, console.StateStale, console.StatePartial} {
		if line := truthStateLine(st, "x"); !strings.Contains(line, "[overflow-wrap:anywhere]") {
			t.Errorf("%s truth line does not wrap: %s", st, line)
		}
	}
}

// TestConsoleFleet_ShowingNamesThePagesRows: once the table pages, the
// line above it names the rows on this page — never every match above 50
// rows.
func TestConsoleFleet_ShowingNamesThePagesRows(t *testing.T) {
	var p console.Projects
	for i := 0; i < 120; i++ {
		p.Rows = append(p.Rows, console.ProjectFacts{Service: fmt.Sprintf("svc-%03d", i), Freshness: "fresh"})
	}
	for page, want := range map[string]string{"1": "Showing 1–50 of 120 projects", "3": "Showing 101–120 of 120 projects"} {
		tbl := console.FleetTable(p, console.ParseFleetQuery(url.Values{"page": {page}}))
		body := renderFleetTablePanel(tbl, observatoryModel{})
		if !strings.Contains(body, want) {
			t.Errorf("page %s: missing %q", page, want)
		}
		if strings.Contains(body, "Showing 120 of 120") {
			t.Errorf("page %s claims every match is shown", page)
		}
	}
	filtered := console.FleetTable(p, console.ParseFleetQuery(url.Values{"q": {"svc-0"}, "page": {"2"}}))
	if body := renderFleetTablePanel(filtered, observatoryModel{}); !strings.Contains(body, "Showing 51–100 of 100 matching, of 120 projects") {
		t.Errorf("filtered and paged: %s", regexp.MustCompile(`data-fleet-shown>[^<]*`).FindString(body))
	}
	if body := renderFleetTablePanel(console.FleetTable(p, console.ParseFleetQuery(url.Values{"q": {"svc-11"}})), observatoryModel{}); !strings.Contains(body, "Showing 10 of 120 projects") {
		t.Errorf("one page: %s", regexp.MustCompile(`data-fleet-shown>[^<]*`).FindString(body))
	}
}

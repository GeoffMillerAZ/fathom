package main

// console_wavef_truth_test.go — wave F's fix round on the fixture fleet:
// every Evidence and System figure keeps its truth state when its
// instrument is missing, unreadable, rotated or filtered out (cmd/fathom/
// CLAUDE.md rule 6: absent is never 0), every panel names the verb that
// prints its numbers (P10), a copied verb chip runs what the page ran, and a
// phone keeps each panel's answer on screen.

import (
	"bytes"
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// guardBeatLine is one PreToolUse edit-hook beat in a project the fixture
// registers (payment-service's root ends in its name).
func guardBeatLine(kind, at, session string) string {
	return `{"channel":"edit-hook","kind":"` + kind + `","at":"` + at + `","meta":{"phase":"pre","target":"file","session":"` + session + `","project_dir":"/r/payment-service"}}`
}

func catchLane(t *testing.T, m console.CatchTimelineModel, k console.CatchKind) console.Value[int] {
	t.Helper()
	for _, s := range m.Kinds {
		if s.Kind == k {
			return s.Count
		}
	}
	t.Fatalf("no lane %s in %+v", k, m.Kinds)
	return console.Value[int]{}
}

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file; the unreadable case cannot be staged")
	}
}

// unreadable makes path unreadable for the rest of the test.
func unreadable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

// The guard lane reads both hooks-log generations: a rotation never drops
// the refusals older than it and leaves a measured zero behind.
func TestConsoleWaveF_GuardRefusalsCountTheRotatedHooksLog(t *testing.T) {
	f := newConsoleFixture(t)
	resetImpactReads()
	t.Cleanup(resetImpactReads)
	hooks := os.Getenv("FATHOM_HOOKS_LOG")
	mustWrite(t, hooks+".1", guardBeatLine("denied", "2026-09-21T10:00:00Z", "s-1")+"\n"+guardBeatLine("denied", "2026-09-21T10:05:00Z", "s-1")+"\n")
	mustWrite(t, hooks, guardBeatLine("seen", "2026-09-24T10:00:00Z", "s-2")+"\n")

	var im impactPageModel
	if err := json.Unmarshal(f.pageJSON(t, "/impact").Model, &im); err != nil {
		t.Fatal(err)
	}
	lane := catchLane(t, im.Catches, console.CatchGuardRefusal)
	if lane.State != console.StatePresent || lane.V != 2 || !strings.Contains(lane.Source, hooks+".1") {
		t.Errorf("guard lane = %+v, want the 2 refusals in the rotated generation, its source naming it", lane)
	}
	verb := runVerbJSON(t, f, "fathom metrics catches --json").(map[string]any)
	for _, k := range verb["kinds"].([]any) {
		kind := k.(map[string]any)
		if kind["kind"] == string(console.CatchGuardRefusal) {
			if c := kind["count"].(map[string]any); c["v"] != float64(2) {
				t.Errorf("`metrics catches` guard count = %v, want 2", c)
			}
		}
	}
}

// A service or window filter decides, per lane, whether the instrument
// recorded anything there; an unknown project is the shell's 404.
func TestConsoleWaveF_CatchFilterScopesEveryLane(t *testing.T) {
	f := newConsoleFixture(t)
	resetImpactReads()
	t.Cleanup(resetImpactReads)
	// PE-24: every row sits at payment-service's registered root (svc's too:
	// a stale service id run in the same checkout), so the registry rule
	// below, not the root rule, is what leaves svc's run out.
	payRoot := strconv.Quote(fixtureServiceRoot(t, f, "payment-service"))
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), `{"schema_version":2,"at":"2026-09-24T10:00:00Z","repo_root":`+payRoot+`,"service":"payment-service","plan":"docs/plans/p.md","claimed":3,"failing":1,"origin":"cli"}
{"schema_version":2,"at":"2026-09-24T11:00:00Z","repo_root":`+payRoot+`,"service":"payment-service","plan":"docs/plans/p.md","claimed":3,"failing":0,"origin":"cli"}
{"schema_version":2,"at":"2026-09-24T12:00:00Z","repo_root":`+payRoot+`,"service":"svc","plan":"docs/plans/t.md","claimed":1,"failing":1,"origin":"cli"}
`)
	mustWrite(t, os.Getenv("FATHOM_HOOKS_LOG"), guardBeatLine("seen", "2026-09-24T10:00:00Z", "s-1")+"\n")
	model := func(path string) impactPageModel {
		var im impactPageModel
		if err := json.Unmarshal(f.pageJSON(t, path).Model, &im); err != nil {
			t.Fatal(err)
		}
		return im
	}
	pay := catchLane(t, model("/impact?svc=payment-service").Catches, console.CatchVerifyRefusal)
	if pay.V != 1 || !strings.Contains(pay.Denominator, "of 2 recorded verify runs") {
		t.Errorf("payment-service verify lane = %+v, want 1 of its own 2 runs", pay)
	}
	checkout := model("/impact?svc=checkout-service").Catches
	for _, s := range checkout.Kinds {
		if s.Count.State != console.StateAbsent || !strings.Contains(s.Count.Reason, "for checkout-service") {
			t.Errorf("checkout-service lane %s = %+v: no instrument recorded anything there — absent, never a zero", s.Kind, s.Count)
		}
	}
	// A service the instruments name but the registry does not list is not
	// the fleet's (OC-H2): its rows are left out and named on the timeline,
	// and a filter naming it has nothing to show.
	all := model("/impact").Catches
	if v := catchLane(t, all, console.CatchVerifyRefusal); v.V != 1 || !strings.Contains(v.Denominator, "1 verify run naming a service the registry does not list (svc) excluded") {
		t.Errorf("the verify lane counts svc's run: %+v", v)
	}
	if code, _, _ := f.get(t, "/impact?svc=svc", "Accept", "application/json"); code != http.StatusNotFound {
		t.Errorf("/impact?svc=svc = %d, want 404: every row naming svc is excluded", code)
	}
	code, _, page := f.get(t, "/impact?svc=no-such-project")
	if code != http.StatusNotFound || !strings.Contains(page, "Unknown project") || !strings.Contains(page, "no-such-project") || !strings.Contains(page, `href="/impact"`) {
		t.Errorf("an unknown project filter = %d, want the shell's 404 with the way back:\n%.600s", code, page)
	}
	if code, _, _ := f.get(t, "/impact?svc=no-such-project", "Accept", "application/json"); code != http.StatusNotFound {
		t.Errorf("an unknown project filter's JSON = %d, want 404", code)
	}
	// The panel's freshness says what the page reuses, never "live".
	_, _, impact := f.get(t, "/impact")
	if strings.Contains(impact, "live — read for this request") || !strings.Contains(impact, "reused for 30 s") {
		t.Error("/impact: the catches panel reuses its ledger reads for 30 s and must say so")
	}
}

// No session ledger (or an unreadable one): the table, its tiles, its
// denominator and the verb's JSON are not measured — never "0 sessions".
func TestConsoleWaveF_SessionsWithoutALedgerAreNotMeasured(t *testing.T) {
	f := newConsoleFixture(t)
	if err := os.Remove(f.ledger); err != nil {
		t.Fatal(err)
	}
	resetObsCache()
	env := f.pageJSON(t, "/sessions")
	section, ok := env.Sections["fathom metrics sessions --json"]
	if !ok {
		t.Fatalf("no section: %v", keysOf(env.Sections))
	}
	for _, raw := range []string{string(env.Model), string(section)} {
		for _, key := range []string{`"sessions":{"v"`, `"matching":{"v"`, `"ledger_rows":{"v"`, `"anonymous_rows":{"v"`, `"sessions":0`} {
			if strings.Contains(raw, key) {
				t.Errorf("no ledger, but the JSON carries %s: %.300s", key, raw)
			}
		}
	}
	var sm sessionsPageModel
	if err := json.Unmarshal(env.Model, &sm); err != nil {
		t.Fatal(err)
	}
	if sm.Table.Sessions.State != console.StateAbsent || !strings.Contains(sm.Table.Sessions.Reason, "no session ledger") {
		t.Errorf("table sessions = %+v", sm.Table.Sessions)
	}
	_, _, page := f.get(t, "/sessions")
	for _, bad := range []string{"No session matches", "0 sessions from 0 ledger rows", "0 matching sessions", "of 0 sessions"} {
		if strings.Contains(page, bad) {
			t.Errorf("/sessions without a ledger says %q", bad)
		}
	}
	if got := tileValue(t, page, "matching this filter"); got != "—" {
		t.Errorf("matching tile = %q, want not measured", got)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"metrics", "sessions", "--json"}, &out, &errb); code != 0 || !strings.Contains(errb.String(), "no session ledger") {
		t.Errorf("metrics sessions without a ledger = %d, stderr %q", code, errb.String())
	}
	if strings.Contains(out.String(), `"sessions": 0`) || !strings.Contains(out.String(), `"state": "absent"`) {
		t.Errorf("metrics sessions --json without a ledger:\n%s", out.String())
	}
	// An unreadable ledger is an error, drawn as one.
	if err := os.Mkdir(f.ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	resetObsCache()
	if err := json.Unmarshal(f.pageJSON(t, "/sessions").Model, &sm); err != nil {
		t.Fatal(err)
	}
	if sm.Table.Sessions.State != console.StateError || sm.Table.Matching.State != console.StateError {
		t.Errorf("an unreadable ledger = %+v, want errors", sm.Table)
	}
	// The replay's ledger-row panel names the missing ledger, not a missing row.
	_, _, replay := f.get(t, "/sessions/s-1")
	if strings.Contains(replay, "no ledger row for this session") {
		t.Error("/sessions/s-1 without a readable ledger must name the ledger, not a missing row")
	}
}

// A hooks log that exists and cannot be read is an error on the replay and
// the guard lane — never "no beats recorded", never an absence.
func TestConsoleWaveF_UnreadableHooksLogIsAnError(t *testing.T) {
	skipIfRoot(t)
	f := newConsoleFixture(t)
	resetImpactReads()
	t.Cleanup(resetImpactReads)
	resetSessionEventsCache()
	t.Cleanup(resetSessionEventsCache)
	hooks := os.Getenv("FATHOM_HOOKS_LOG")
	mustWrite(t, hooks, guardBeatLine("seen", "2026-09-24T10:00:00Z", "s-1")+"\n")
	unreadable(t, hooks)

	var rp sessionReplay
	if err := json.Unmarshal(f.pageJSON(t, "/sessions/s-1").Model, &rp); err != nil {
		t.Fatal(err)
	}
	if rp.Window.Events.State != console.StateError || !strings.Contains(rp.Window.Events.Reason, "permission denied") {
		t.Errorf("replay over an unreadable hooks log = %+v", rp.Window.Events)
	}
	_, _, page := f.get(t, "/sessions/s-1")
	if !strings.Contains(page, `data-truth-line="error"`) || strings.Contains(page, "no beat") {
		t.Error("/sessions/s-1: an unreadable hooks log must be drawn as an error naming the failure")
	}
	var im impactPageModel
	if err := json.Unmarshal(f.pageJSON(t, "/impact").Model, &im); err != nil {
		t.Fatal(err)
	}
	if lane := catchLane(t, im.Catches, console.CatchGuardRefusal); lane.State != console.StateError {
		t.Errorf("guard lane over an unreadable hooks log = %+v, want an error", lane)
	}
}

// An unreadable call log is an error on every /operate panel and in every
// traffic verb's JSON — never an idle daemon.
func TestConsoleWaveF_OperateNeverDrawsAnUnreadableLogAsIdle(t *testing.T) {
	skipIfRoot(t)
	f := newConsoleFixture(t)
	unreadable(t, f.callLog)
	resetObsCache()
	resetOperateCache()
	t.Cleanup(resetOperateCache)
	_, _, page := f.get(t, "/operate")
	for _, bad := range []string{"no calls in window", "no usage call in the last 7 days", "no call on this surface in the last 7 days", "0 of 0 tools"} {
		if strings.Contains(page, htmlEscapeTest(bad)) {
			t.Errorf("/operate over an unreadable log says %q", bad)
		}
	}
	for _, id := range []string{"traffic", "surfaces", "callers", "tools"} {
		if st := panelTruthState(t, page, id); st != "error" {
			t.Errorf("/operate panel %s truth state = %q, want error", id, st)
		}
	}
	env := f.pageJSON(t, "/operate")
	for _, verb := range []string{operateTrafficVerb, operateCallerVerb} {
		var sec map[string]any
		if err := json.Unmarshal(env.Sections[verb], &sec); err != nil {
			t.Fatal(err)
		}
		if sec["state"] != "error" || sec["total_calls"] != nil {
			t.Errorf("%s over an unreadable log = %v, want state error and no counts", verb, sec)
		}
		if fromVerb := runVerbJSON(t, f, verb); !reflect.DeepEqual(fromVerb, any(sec)) {
			t.Errorf("%s disagrees with its section", verb)
		}
	}
}

// panelTruthState is the data-truth-state of the panel whose anchor is id.
func panelTruthState(t *testing.T, page, id string) string {
	t.Helper()
	re := regexp.MustCompile(`<div class="[^"]*" data-console-panel data-truth-state="([a-z]+)"[^>]*\bid="` + regexp.QuoteMeta(id) + `"`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no panel %s", id)
	}
	return m[1]
}

// Time to remedy keeps a stale median's lower bound, a closed gap that
// opened on or before the first snapshot reads "≥", the grade column names
// its snapshot, and with no snapshot nothing is a zero.
func TestConsoleWaveF_TimeToRemedyKeepsItsLowerBoundAndItsAbsence(t *testing.T) {
	f := newConsoleFixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetOperateCache()
	t.Cleanup(resetOperateCache)

	_, _, page := f.get(t, "/operate")
	for _, label := range []string{"gaps closed", "gaps still open", "below grade A, latest snapshot"} {
		if got := tileValue(t, page, label); got != "—" {
			t.Errorf("no snapshot: tile %q = %q, want not measured", label, got)
		}
	}
	env := f.pageJSON(t, "/operate")
	for _, key := range []string{`"closed_gaps"`, `"open_gaps"`, `"below_a_latest"`, `"graded_latest"`} {
		if strings.Contains(string(env.Sections[operateRemedyVerb]), key) {
			t.Errorf("no snapshot: `fleet remedy --json` carries %s", key)
		}
	}

	dir := filepath.Join(home, ".fathom", "fleet")
	row := func(svc, stanza string) string {
		return `{"service":"` + svc + `","path":"/r/` + svc + `","calls_total":4,"hooks_note":"3 fathom hook reference(s) in project settings","stanza_note":"` + stanza + `","freshness":"fresh"}`
	}
	for date, stanza := range map[string]string{"2026-08-08": "CLAUDE.md present, no fathom stanza", "2026-08-12": "present", "2026-08-14": "present"} {
		mustWrite(t, filepath.Join(dir, "report-"+date+".json"), `{"schema_version":2,"generated_at":"`+date+`T12:00:00Z","services":[`+row("beta", stanza)+`,`+row("alpha", "present")+`]}`)
	}
	resetOperateCache()
	var om operatePageModel
	if err := json.Unmarshal(f.pageJSON(t, "/operate").Model, &om); err != nil {
		t.Fatal(err)
	}
	med := om.RemedyNow.MedianHours
	if med.State != console.StateStale || med.V != 96 || !om.RemedyNow.LowerBound || !strings.Contains(med.Reason, "lower bound") {
		t.Errorf("remedy headline = %+v lower bound %v, want a stale 96 h that is also a lower bound", med, om.RemedyNow.LowerBound)
	}
	_, _, page = f.get(t, "/operate")
	if got := tileValue(t, page, "median time to remedy (hours)"); !strings.HasPrefix(got, "≥") {
		t.Errorf("the median tile = %q, want a ≥ lower bound", got)
	}
	for _, want := range []string{"≥ 96.0 h", "Grade at the latest snapshot", `data-stack="operate-remedy"`} {
		if !strings.Contains(page, htmlEscapeTest(want)) && !strings.Contains(page, want) {
			t.Errorf("/operate remedy missing %q", want)
		}
	}
	if strings.Contains(page, ">Grade now<") {
		t.Error("the grade column is the latest snapshot's, never 'now'")
	}
	if strings.Contains(string(f.pageJSON(t, "/operate").Sections[operateRemedyVerb]), `"episodes": null`) ||
		strings.Contains(string(f.pageJSON(t, "/operate").Sections[operateRemedyVerb]), `"episodes":null`) {
		t.Error("a repository with no gap lists no episodes, never null")
	}
}

// The decision port's per-site counts come from its log: an unreadable log
// is an error in every count, never 0 beside "unreadable".
func TestConsoleWaveF_DecideSitesNeverPrintZerosForAnUnreadableLog(t *testing.T) {
	skipIfRoot(t)
	f := newConsoleFixture(t)
	log := filepath.Join(filepath.Dir(f.callLog), "decisions.jsonl")
	mustWrite(t, log, `{"ts":"2026-09-24T10:00:00Z","site":"route","provider":"typesafe","kind":"choice","choice":"find_callers","confidence":0.4,"latency_ms":400,"applied":true}`+"\n")
	unreadable(t, log)
	env := f.pageJSON(t, "/decide")
	var dm console.DecideModel
	if err := json.Unmarshal(env.Model, &dm); err != nil {
		t.Fatal(err)
	}
	for _, s := range dm.Sites {
		if s.Applied.State != console.StateError || s.Errors.State != console.StateError {
			t.Errorf("site %s over an unreadable log = applied %+v errors %+v", s.Site, s.Applied, s.Errors)
		}
	}
	if strings.Contains(string(env.Model), `"applied":0`) || strings.Contains(string(env.Model), `"errors":0`) {
		t.Error("/decide JSON carries a zero for an unreadable log")
	}
	_, _, page := f.get(t, "/decide")
	sites := page[strings.Index(page, `data-table="decide-sites"`):]
	sites = sites[:strings.Index(sites, "</table>")]
	if n := strings.Count(sites, "unreadable"); n < 3*len(dm.Sites) {
		t.Errorf("the sites table names the failure in %d cells, want every rulings, applied and failed cell (%d)", n, 3*len(dm.Sites))
	}
}

// A verb chip quotes a value a shell would split: pasted, it runs exactly
// what the page ran and prints the page's section.
func TestConsoleWaveF_VerbChipsQuoteTheirValues(t *testing.T) {
	if got := evCommand("fathom metrics sessions", "--q", "it's a b", "--repo", "fathom"); got != `fathom metrics sessions --q 'it'\''s a b' --repo fathom --json` {
		t.Errorf("evCommand = %s", got)
	}
	f := newConsoleFixture(t)
	env := f.pageJSON(t, "/sessions?q=s-1%20x")
	const chip = "fathom metrics sessions --q 's-1 x' --json"
	section, ok := env.Sections[chip]
	if !ok {
		t.Fatalf("the filtered section must be keyed by the quoted chip %q: %v", chip, keysOf(env.Sections))
	}
	var fromPage any
	_ = json.Unmarshal(section, &fromPage)
	if fromVerb := runVerbJSON(t, f, chip); !reflect.DeepEqual(fromVerb, fromPage) {
		t.Errorf("the pasted chip disagrees with the page:\nverb %v\npage %v", fromVerb, fromPage)
	}
	_, _, page := f.get(t, "/sessions?q=s-1%20x")
	if !strings.Contains(page, html.EscapeString(chip)) {
		t.Error("/sessions must show the quoted chip")
	}
}

// Operate: every panel's chip is the verb that prints its numbers — the
// load (calls per day per surface, tools by caller class), the zero-call
// list and the harness matrix — and the chart's scroll floor is a class the
// stylesheet carries.
func TestConsoleWaveF_OperatePanelsNameTheVerbsThatPrintThem(t *testing.T) {
	f := newConsoleFixture(t)
	resetOperateCache()
	t.Cleanup(resetOperateCache)
	_, _, page := f.get(t, "/operate")
	chip := func(id string) string {
		re := regexp.MustCompile(`data-console-panel data-truth-state="[a-z]+"[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*data-console-verb="([^"]*)"`)
		m := re.FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no panel %s with a verb", id)
		}
		return m[1]
	}
	// The harness chip names this daemon's URL (the `mcp add` lines are
	// rendered with it); the verb's default is the live daemon's.
	harnessChip := "fathom harness matrix --url " + f.ts.URL + " --json"
	for id, want := range map[string]string{"traffic": operateLoadVerb, "surfaces": operateTrafficVerb, "callers": operateCallerVerb, "tools": operateLoadVerb, "zero-calls": operateAdoptVerb, "harnesses": harnessChip} {
		if got := chip(id); got != htmlEscapeTest(want) && got != want {
			t.Errorf("panel %s names %q, want %q", id, got, want)
		}
	}
	env := f.pageJSON(t, "/operate")
	var om operatePageModel
	if err := json.Unmarshal(env.Model, &om); err != nil {
		t.Fatal(err)
	}
	var load console.LoadModel
	if err := json.Unmarshal(env.Sections[operateLoadVerb], &load); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(load, om.Operate.LoadModel) {
		t.Error("the load section must be exactly what the traffic and tool panels render")
	}
	var fromChip, section any
	_ = json.Unmarshal(env.Sections[operateHarnessVerb], &section)
	if fromChip = runVerbJSON(t, f, harnessChip); !reflect.DeepEqual(fromChip, section) {
		t.Error("the harness chip, pasted, must print the panel's section")
	}
	for _, want := range []string{`data-table="operate-harnesses"`, "min-w-[560px]", `data-stack="operate-surfaces"`, `data-stack="operate-callers"`} {
		if !strings.Contains(page, want) {
			t.Errorf("/operate missing %s", want)
		}
	}
	css, err := os.ReadFile(filepath.Join("ui", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(css), "%dpx") || !strings.Contains(string(css), `.min-w-\[560px\]`) {
		t.Error("app.css must carry the heatmap's min-w-[560px] rule and no runtime-built class")
	}
}

// Below md each table that carries a panel's answer is restated as a
// stacked list, and the sessions table's sort is reachable there.
func TestConsoleWaveF_PhoneKeepsEachAnswer(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, impact := f.get(t, "/impact")
	if !strings.Contains(impact, `data-stack="impact-tools"`) || !strings.Contains(impact, "not instrumented") {
		t.Error("/impact: the per-tool outcomes need a stacked list below md")
	}
	mustWrite(t, filepath.Join(filepath.Dir(f.callLog), "decisions.jsonl"), `{"ts":"2026-09-24T10:00:00Z","site":"route","provider":"typesafe","kind":"choice","choice":"find_callers","confidence":0.4,"latency_ms":400,"applied":false}`+"\n")
	_, _, decide := f.get(t, "/decide")
	if !strings.Contains(decide, `data-stack="decide-rulings"`) {
		t.Error("/decide: the ruling log needs a stacked list below md")
	}
	_, _, sessions := f.get(t, "/sessions")
	i := strings.Index(sessions, "data-sessions-sort")
	if i < 0 {
		t.Fatal("/sessions: the sort needs a control outside the md-only table")
	}
	sortBlock := sessions[i:]
	sortBlock = sortBlock[:strings.Index(sortBlock, "</div>")]
	if strings.Count(sortBlock, "sort=") < len(console.SessionSorts) {
		t.Errorf("/sessions: the phone sort control must offer every sort key:\n%s", sortBlock)
	}
}

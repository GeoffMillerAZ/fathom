package main

// console_wavef_test.go — wave F's pages on the fixture fleet (vision 35 §3
// Evidence and System; OC-41..OC-45): /impact, /sessions, /sessions/{id},
// /operate and /decide render their verbs, draw every truth state in words,
// and keep their bounds. JSON parity with the declared verbs is
// TestConsolePagesRenderTheirVerbs; the shared metrics are
// TestConsoleMetricsAgreeAcrossPages; the budgets are TestConsoleBudgets.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func TestConsoleWaveF_PagesRenderTheirStates(t *testing.T) {
	f := newConsoleFixture(t)
	dir := filepath.Dir(f.callLog)
	resetImpactReads()
	t.Cleanup(resetImpactReads)

	for _, path := range []string{"/impact", "/sessions", "/sessions/s-1", "/operate", "/decide"} {
		code, _, body := f.get(t, path)
		if code != 200 {
			t.Fatalf("GET %s = %d", path, code)
		}
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s: want exactly one h1", path)
		}
		if strings.Count(body, "data-console-verb=") < 1 {
			t.Errorf("%s: its panels must name their verbs", path)
		}
		assertRoleUtilitiesOnly(t, body)
	}

	// Impact: the questions in order; an arm with no admitted session is
	// named, never a zero bar; a tool with no outcome says so in words.
	_, _, impact := f.get(t, "/impact")
	last := -1
	for _, q := range []string{"Q1 · ", "Q2 · ", "Q3 · ", "Q4 · "} {
		i := strings.Index(impact, q)
		if i < 0 || i < last {
			t.Errorf("/impact: %q missing or out of order", q)
		}
		last = i
	}
	if !strings.Contains(impact, "data-arm-absent") || !strings.Contains(impact, "not measured: no clean on-arm session yet") {
		t.Error("/impact: an arm with no admitted session must be named as not measured")
	}
	if strings.Contains(impact, `aria-label="counterfactual arms`) {
		t.Error("/impact: no range bar may be drawn for an arm with no admitted session")
	}
	var im impactPageModel
	if err := json.Unmarshal(f.pageJSON(t, "/impact").Model, &im); err != nil {
		t.Fatal(err)
	}
	if len(im.Impact.Tools) == 0 {
		t.Fatal("/impact: the fixture's tools must be listed")
	}
	for _, tool := range im.Impact.Tools {
		if tool.Hits.State != console.StateAbsent || !strings.Contains(tool.Hits.Reason, "not instrumented") {
			t.Errorf("tool %s hits = %+v: the fixture's rows carry no outcome field", tool.Tool, tool.Hits)
		}
	}
	if !strings.Contains(impact, `data-truth-state="absent" title="not instrumented`) {
		t.Error("/impact: the tools table must draw 'not instrumented' in words")
	}

	// The catches timeline: a failing verify-ledger row is a verify
	// refusal; a kind filter keeps every lane's count; a kind with no
	// instrument says why.
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), `{"schema_version":2,"at":"2026-09-24T10:00:00Z","repo_root":"/r","service":"payment-service","plan":"docs/plans/p.md","total":4,"claimed":3,"verified":2,"failing":1,"origin":"cli"}
{"schema_version":2,"at":"2026-09-24T11:00:00Z","repo_root":"/r","service":"payment-service","plan":"docs/plans/p.md","total":4,"claimed":3,"verified":3,"failing":0,"origin":"cli"}
`)
	resetImpactReads()
	_, _, impact = f.get(t, "/impact")
	if !strings.Contains(impact, "docs/plans/p.md") || !strings.Contains(impact, "1 failing claimed row of 3 claimed rows (cli)") {
		t.Error("/impact: the verify refusal must be on the catches table")
	}
	for _, reason := range []string{"no `fathom review` run in the call log checked its conservation dimension", "no hooks log at"} {
		if !strings.Contains(impact, htmlEscapeTest(reason)) {
			t.Errorf("/impact: a lane with no instrument must say %q", reason)
		}
	}
	var only impactPageModel
	if err := json.Unmarshal(f.pageJSON(t, "/impact?kind=guard-refusal").Model, &only); err != nil {
		t.Fatal(err)
	}
	if only.Catches.Total != 0 || only.Catches.Kinds[0].Count.V != 1 || len(only.Catches.Filter.Kinds) != 1 {
		t.Errorf("a kind filter narrows the rows and keeps every lane's count: %+v", only.Catches)
	}
	env := f.pageJSON(t, "/impact?kind=guard-refusal")
	if _, ok := env.Sections["fathom metrics catches --kind guard-refusal --json"]; !ok {
		t.Errorf("a filtered page's section is keyed by the exact command it shows: %v", keysOf(env.Sections))
	}

	// Sessions: one row per session, filterable; an empty filter says so.
	var sm sessionsPageModel
	if err := json.Unmarshal(f.pageJSON(t, "/sessions").Model, &sm); err != nil {
		t.Fatal(err)
	}
	if sm.Table.Sessions.V != 2 || len(sm.Table.Rows) != 2 || sm.Table.LedgerRows.V != 3 {
		t.Errorf("sessions table = %+v, want 2 sessions from 3 rows", sm.Table)
	}
	_, _, none := f.get(t, "/sessions?arm=off")
	if !strings.Contains(none, "No session matches") {
		t.Error("/sessions: a filter matching nothing must say so")
	}
	_, _, sorted := f.get(t, "/sessions?sort=turns&dir=desc")
	if !strings.Contains(sorted, "(sorted descending)") {
		t.Error("/sessions: the sorted column must say how it is sorted")
	}

	// Decide: no decisions log is absent, never zero rulings; a log beside
	// the call log is read; no calibration reading is named, never assumed.
	_, _, decide := f.get(t, "/decide")
	if !strings.Contains(decide, "no decisions log at") || !strings.Contains(decide, "no decision-port calibration reading") {
		t.Error("/decide: a missing log and a missing reading must each be named")
	}
	mustWrite(t, filepath.Join(dir, "decisions.jsonl"), `{"ts":"2026-09-24T10:00:00Z","site":"route","provider":"typesafe","kind":"choice","choice":"find_callers","confidence":0.4,"latency_ms":400,"applied":false}
{"ts":"2026-09-24T10:01:00Z","site":"route","provider":"typesafe","questions":1,"latency_ms":3000,"error":"timeout"}
`)
	var dm console.DecideModel
	if err := json.Unmarshal(f.pageJSON(t, "/decide").Model, &dm); err != nil {
		t.Fatal(err)
	}
	if dm.Rulings.V != 2 || dm.Errors.V != 1 || dm.ProviderOn || dm.Calibration != nil {
		t.Errorf("/decide = rulings %+v errors %+v provider-on %v", dm.Rulings, dm.Errors, dm.ProviderOn)
	}
	_, _, decide = f.get(t, "/decide")
	if !strings.Contains(decide, "failed: timeout") || !strings.Contains(decide, "(provider none: no call)") {
		t.Error("/decide: a failed call and an off provider must be said in words")
	}

	// Operate: the traffic heatmap draws a day with no rows as missing, the
	// tools carry their caller class, and time-to-remedy with no snapshot
	// is not measured.
	_, _, operate := f.get(t, "/operate")
	for _, want := range []string{
		`aria-label="usage calls per day per surface; hatched: no call-log rows that day"`,
		`data-table="operate-tools"`, `data-loop-status="`, `data-table="operate-capabilities"`,
	} {
		if !strings.Contains(operate, want) {
			t.Errorf("/operate missing %s", want)
		}
	}
	for _, want := range []string{
		"missing: no call-log rows that day", "every day and surface as a table", "edit hook", "unattributed",
		"registered tools with no call at all", "no fleet report snapshot — time-to-remedy is not measured", "[daemon] call_log",
	} {
		if !strings.Contains(operate, htmlEscapeTest(want)) {
			t.Errorf("/operate missing %q", want)
		}
	}
	var om operatePageModel
	if err := json.Unmarshal(f.pageJSON(t, "/operate").Model, &om); err != nil {
		t.Fatal(err)
	}
	if om.Operate.WindowCalls.V != f.callRows || len(om.Flywheel) == 0 || om.Capabilities == nil || om.RemedyNow.MedianHours.State != console.StateAbsent {
		t.Errorf("/operate model = window %+v, %d loops, caps %v, remedy %+v", om.Operate.WindowCalls, len(om.Flywheel), om.Capabilities != nil, om.RemedyNow)
	}
}

// The replay is windowed: a long session's page and JSON carry one window
// of events (the page's 200, never more than the 2,000 bound), paged in time
// order and filterable by channel, and the page's section is exactly what
// `fathom metrics sessions --id` prints.
func TestConsoleWaveF_ReplayWindowedAndEqualToItsVerb(t *testing.T) {
	f := newConsoleFixture(t)
	var beats strings.Builder
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 2600; i++ {
		ch := "edit-hook"
		if i%4 == 0 {
			ch = "discovery"
		}
		fmt.Fprintf(&beats, `{"channel":"%s","kind":"seen","at":"%s","meta":{"session":"s-1","phase":"post","file":"/r/a.go","project_dir":"/r"}}`+"\n", ch, base.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano))
	}
	beats.WriteString(`{"channel":"edit-hook","kind":"seen","at":"2026-09-24T09:00:00Z","meta":{"session":"s-2"}}` + "\n")
	mustWrite(t, os.Getenv("FATHOM_HOOKS_LOG"), beats.String())

	code, _, page := f.get(t, "/sessions/s-1")
	if code != 200 || len(page) > 250*1024 {
		t.Fatalf("/sessions/s-1 = %d, %d bytes", code, len(page))
	}
	if !strings.Contains(page, "window 1 of 13") || !strings.Contains(page, "a.go") {
		t.Errorf("the replay must say which window it shows:\n%.400s", page[strings.Index(page, "data-replay-window"):])
	}
	env := f.pageJSON(t, "/sessions/s-1")
	var rp sessionReplay
	if err := json.Unmarshal(env.Model, &rp); err != nil {
		t.Fatal(err)
	}
	if rp.Window.Events.V != 2600 || len(rp.Window.Window) != sessionPageWindow || rp.Ledger == nil || rp.Ledger.SessionID != "s-1" {
		t.Errorf("replay = %+v events, window %d, ledger %v", rp.Window.Events, len(rp.Window.Window), rp.Ledger)
	}
	if first := rp.Window.Window[0].Detail; first != "file=a.go phase=post" {
		t.Errorf("an event's labels are its meta minus the session and provenance, the file relative to the project: %q", first)
	}
	var big sessionReplay
	if err := json.Unmarshal(f.pageJSON(t, "/sessions/s-1?per=100000&page=2").Model, &big); err != nil {
		t.Fatal(err)
	}
	if big.Window.PerPage != console.MaxWindowEvents || len(big.Window.Window) != 600 || big.Window.Page != 2 {
		t.Errorf("an over-large window is clamped to %d: per %d, %d events on page %d", console.MaxWindowEvents, big.Window.PerPage, len(big.Window.Window), big.Window.Page)
	}
	var disc sessionReplay
	if err := json.Unmarshal(f.pageJSON(t, "/sessions/s-1?channel=discovery").Model, &disc); err != nil {
		t.Fatal(err)
	}
	if disc.Window.Matching.V != 650 {
		t.Errorf("channel filter = %+v matching, want 650", disc.Window.Matching)
	}

	// Parity: the section is the verb's JSON, and the page names it — in
	// its heading and on both panels (the ledger row is in the same JSON).
	verb := "fathom metrics sessions --id s-1 --per-page 200 --json"
	section, ok := env.Sections[verb]
	if !ok {
		t.Fatalf("the replay's section must be keyed by %q: %v", verb, keysOf(env.Sections))
	}
	if !reflect.DeepEqual(env.Verbs, []string{verb}) {
		t.Errorf("the replay page's verbs = %v, want [%s]", env.Verbs, verb)
	}
	heading := page[strings.Index(page, "data-console-heading"):]
	heading = heading[:strings.Index(heading, "data-console-panel")]
	if !strings.Contains(heading, `data-console-copy="`+verb+`"`) {
		t.Error("the replay page's heading must carry its verb chip")
	}
	if n := strings.Count(page, `data-console-verb="`+verb+`"`); n != 2 {
		t.Errorf("the replay and ledger-row panels must both name %q (%d do)", verb, n)
	}
	// The HTML page keeps its budget whatever ?per= asks; JSON may window
	// up to the 2,000-event bound.
	if code, _, wide := f.get(t, "/sessions/s-1?per=2000"); code != 200 || len(wide) > 250*1024 || !strings.Contains(wide, "window 1 of 13") {
		t.Errorf("/sessions/s-1?per=2000 = %d, %d bytes: the HTML window stays %d events", code, len(wide), sessionPageWindow)
	}
	var out, errb bytes.Buffer
	if code := run(strings.Fields(verb)[1:], &out, &errb); code != 0 {
		t.Fatalf("%s: %d %s", verb, code, errb.String())
	}
	var fromVerb, fromPage any
	_ = json.Unmarshal(out.Bytes(), &fromVerb)
	_ = json.Unmarshal(section, &fromPage)
	if !reflect.DeepEqual(fromVerb, fromPage) {
		t.Errorf("the replay page disagrees with `%s`", verb)
	}
}

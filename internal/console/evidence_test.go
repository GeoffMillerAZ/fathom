package console

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

// TestImpactView_NotInstrumentedNeverZero pins OC-41: the four questions
// come in order Q1..Q4; each arm is a range (p25..p75 with the median) over
// its admitted sessions, and an arm with none is absent — never a zero bar;
// a tool whose calls carry no outcome has absent hits, misses and
// did-you-mean values that say "not instrumented" — never a colored zero —
// while an instrumented tool's measured zero stays a zero.
func TestImpactView_NotInstrumentedNeverZero(t *testing.T) {
	in := ImpactInputs{
		LedgerSource: "ledger", CallLogSource: "log", Sessions: 12,
		Arms: ArmInputs{On: []int{900, 100, 500, 300, 700}, Criterion: "collecting"},
		Tools: []ToolOutcomeInput{
			{Tool: "find_symbol", Class: ClassAgent, Calls: 10, Hits: 7, Misses: 2, DidYouMean: 1},
			{Tool: "obligations", Class: ClassEditHook, Calls: 40, NotRecorded: 40},
			{Tool: "references", Class: ClassAgent, Calls: 4, Hits: 0, Misses: 0, NotRecorded: 1},
		},
		GateRefusals: intp(3), GateRuns: 30, ObligationsServed: 40,
	}
	m := ImpactView(in)
	var ids []string
	for _, q := range m.Questions {
		ids = append(ids, q.ID)
	}
	if strings.Join(ids, ",") != "Q1,Q2,Q3,Q4" {
		t.Fatalf("questions out of order: %v", ids)
	}
	on, off := m.Arms[0], m.Arms[1]
	if on.Arm != "on" || on.Units.V != 5 || on.Low.V != 300 || on.Median.V != 500 || on.High.V != 700 {
		t.Errorf("ON arm = %+v, want p25 300, median 500, p75 700 over 5 sessions", on)
	}
	if off.Median.State != StateAbsent || off.Low.State != StateAbsent || off.High.State != StateAbsent || !strings.Contains(off.Median.Reason, "law 12") {
		t.Errorf("an arm with no admitted session must be absent, got %+v", off)
	}
	if off.Units.State != StateZero {
		t.Errorf("the OFF arm's admitted-session count is a measured zero, got %+v", off.Units)
	}
	byTool := map[string]ToolOutcome{}
	for _, o := range m.Tools {
		byTool[o.Tool] = o
	}
	ob := byTool["obligations"]
	for name, v := range map[string]Value[int]{"hits": ob.Hits, "misses": ob.Misses, "did_you_mean": ob.DidYouMean} {
		if v.State != StateAbsent || !strings.Contains(v.Reason, "not instrumented") {
			t.Errorf("obligations %s = %+v, want absent: not instrumented", name, v)
		}
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), `"v"`) {
			t.Errorf("obligations %s carries a number in JSON: %s", name, raw)
		}
	}
	if ob.Instrumented.State != StateZero {
		t.Errorf("obligations instrumented calls = %+v, want a measured zero", ob.Instrumented)
	}
	if r := byTool["references"]; r.Hits.State != StateZero || r.Misses.State != StateZero || !strings.Contains(r.Hits.Denominator, "3 instrumented calls") {
		t.Errorf("an instrumented tool's zero stays a measured zero over its instrumented calls: %+v", r)
	}
	if fs := byTool["find_symbol"]; fs.Hits.V != 7 || fs.Hits.State != StatePresent || fs.Class != ClassAgent {
		t.Errorf("find_symbol = %+v", fs)
	}
	if m.Uninstrumented != 1 {
		t.Errorf("uninstrumented tools = %d, want 1", m.Uninstrumented)
	}
	// Q2's gate figure is the verify ledger's refusal count.
	if f := m.Questions[1].Figures[0]; f.ID != "gate.refusals" || f.Value.V != 3 {
		t.Errorf("gate figure = %+v", f)
	}
	// A ledger that could not be read makes every ledger figure absent, and
	// an unreadable call log makes every tool figure an error.
	bad := ImpactView(ImpactInputs{LedgerReason: "no session ledger", CallLogReason: "EPERM", CallLogError: true,
		Tools: []ToolOutcomeInput{{Tool: "find_symbol", Calls: 2, Hits: 2}}})
	if bad.Arms[0].Units.State != StateAbsent || bad.Tools[0].Hits.State != StateError {
		t.Errorf("unreadable sources: arms %+v tools %+v", bad.Arms[0], bad.Tools[0])
	}
	for _, q := range append(m.Questions, bad.Questions...) {
		for _, f := range q.Figures {
			if err := f.Value.Validate(); err != nil {
				t.Errorf("%s %s: %v", q.ID, f.ID, err)
			}
		}
	}
}

// TestCatchTimeline_KindsAndFilters pins OC-42: verify refusals, conserve
// losses, integrity catches and guard refusals land on one timeline, newest
// first; a kind filter keeps the other lanes' counts; a service and window
// filter narrow every lane; a kind whose instrument recorded nothing it can
// count from is absent with its reason, never an empty lane.
func TestCatchTimeline_KindsAndFilters(t *testing.T) {
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	in := CatchInputs{
		VerifyRuns: []VerifyRun{
			{At: "2026-09-20T10:00:00Z", Service: "fathom", Plan: "docs/plans/a.md", Failing: 2, Claimed: 9, Origin: "cli"},
			{At: "2026-09-21T10:00:00Z", Service: "fathom", Plan: "docs/plans/a.md", Failing: 0, Claimed: 9},
			{At: "2026-09-22T10:00:00Z", Service: "app-kit", Plan: "docs/plans/b.md", Failing: 1, Claimed: 3, Origin: "stop"},
		},
		Reviews: []ReviewRun{
			{At: "2026-09-23T10:00:00Z", Service: "fathom", Outcome: map[string]int{"conservation": 1, "integrity": 0}},
			{At: "2026-09-24T10:00:00Z", Service: "fathom", Outcome: map[string]int{"conservation": 0, "integrity": 2}},
		},
		GuardBeats: []GuardBeat{
			{At: at("2026-09-24T11:00:00Z"), Kind: "seen", Target: "file"},
			{At: at("2026-09-24T11:00:01Z"), Kind: "denied", Target: "file", Service: "fathom"},
		},
	}
	m := CatchTimeline(in, CatchFilter{}, 1, 50)
	if m.Total != 5 {
		t.Fatalf("total = %d, want 5 (2 verify, 1 conserve, 1 integrity, 1 guard)", m.Total)
	}
	var kinds []string
	for _, c := range m.Catches {
		kinds = append(kinds, string(c.Kind))
	}
	if strings.Join(kinds, ",") != "guard-refusal,integrity-catch,conserve-loss,verify-refusal,verify-refusal" {
		t.Errorf("newest first: %v", kinds)
	}
	count := func(m CatchTimelineModel, k CatchKind) Value[int] {
		for _, s := range m.Kinds {
			if s.Kind == k {
				return s.Count
			}
		}
		t.Fatalf("no lane %s", k)
		return Value[int]{}
	}
	if v := count(m, CatchVerifyRefusal); v.V != 2 || !strings.Contains(v.Denominator, "3 recorded verify runs") {
		t.Errorf("verify lane = %+v", v)
	}
	// A kind filter keeps the rows to that kind but every lane's count.
	only := CatchTimeline(in, CatchFilter{Kinds: []CatchKind{CatchIntegrity}}, 1, 50)
	if only.Total != 1 || only.Catches[0].Kind != CatchIntegrity || count(only, CatchVerifyRefusal).V != 2 {
		t.Errorf("kind filter = %+v", only)
	}
	for _, s := range only.Kinds {
		if s.Selected != (s.Kind == CatchIntegrity) {
			t.Errorf("lane %s selected = %v", s.Kind, s.Selected)
		}
	}
	// Service and window filters narrow every lane — the counts, the
	// denominators, and whether the lane's instrument recorded anything in
	// that scope at all: a service no `fathom review` run names has no
	// conservation instrument there, so its lane is absent, never a zero.
	svc := CatchTimeline(in, CatchFilter{Service: "app-kit"}, 1, 50)
	if svc.Total != 1 {
		t.Errorf("service filter = total %d, want 1", svc.Total)
	}
	if v := count(svc, CatchVerifyRefusal); v.V != 1 || v.State != StatePresent || !strings.Contains(v.Denominator, "of 1 recorded verify run") {
		t.Errorf("a service's verify lane counts over that service's runs: %+v", v)
	}
	for _, k := range []CatchKind{CatchConserveLoss, CatchIntegrity, CatchGuardRefusal} {
		if v := count(svc, k); v.State != StateAbsent || !strings.Contains(v.Reason, "for app-kit") {
			t.Errorf("lane %s for a service with no instrument there = %+v, want absent naming the service", k, v)
		}
	}
	if fs := CatchTimeline(in, CatchFilter{Service: "fathom"}, 1, 50); count(fs, CatchConserveLoss).State != StatePresent || count(fs, CatchGuardRefusal).V != 1 {
		t.Errorf("a service the instruments name keeps its measured lanes: %+v", fs.Kinds)
	}
	unknown := CatchTimeline(in, CatchFilter{Service: "no-such-project"}, 1, 50)
	for _, s := range unknown.Kinds {
		if s.Count.State != StateAbsent || s.Count.Denominator != "" {
			t.Errorf("an unknown service's lane %s = %+v, want absent (never a measured zero over the fleet's denominator)", s.Kind, s.Count)
		}
	}
	if in.NamesService("no-such-project") || !in.NamesService("APP-KIT") || !in.NamesService("fathom") {
		t.Error("NamesService: a service is named when any instrument's row names it (case-insensitive)")
	}
	win := CatchTimeline(in, CatchFilter{From: at("2026-09-23T00:00:00Z")}, 1, 50)
	if win.Total != 3 || win.Filter.FromText != "2026-09-23T00:00:00Z" {
		t.Errorf("window filter = %d %q", win.Total, win.Filter.FromText)
	}
	if v := count(win, CatchVerifyRefusal); v.State != StateAbsent || !strings.Contains(v.Reason, "in this window") {
		t.Errorf("a window with no verify run has no verify instrument reading: %+v", v)
	}
	both := CatchTimeline(in, CatchFilter{Service: "fathom", From: at("2026-09-21T00:00:00Z")}, 1, 50)
	if v := count(both, CatchVerifyRefusal); v.State != StateZero || !strings.Contains(v.Denominator, "of 1 recorded verify run") {
		t.Errorf("a scope with runs but no refusal is a measured zero over the scope's runs: %+v", v)
	}
	// Paging.
	p2 := CatchTimeline(in, CatchFilter{}, 2, 2)
	if p2.Pages != 3 || p2.Page != 2 || len(p2.Catches) != 2 || len(p2.Window) != 5 {
		t.Errorf("paging = page %d of %d, %d rows, %d points", p2.Page, p2.Pages, len(p2.Catches), len(p2.Window))
	}
	// No instrument: absent with a reason, never a zero lane.
	empty := CatchTimeline(CatchInputs{VerifyReason: "no verify ledger at /x"}, CatchFilter{}, 1, 50)
	for _, s := range empty.Kinds {
		if s.Count.State != StateAbsent || s.Count.Reason == "" {
			t.Errorf("lane %s with no instrument = %+v, want absent with a reason", s.Kind, s.Count)
		}
	}
	if !strings.Contains(count(empty, CatchConserveLoss).Reason, "conservation dimension") || !strings.Contains(count(empty, CatchGuardRefusal).Reason, "guard-hook beat") {
		t.Errorf("reasons must name the missing instrument: %+v", empty.Kinds)
	}
	// An instrument that exists and could not be read is an error naming
	// the failure — drawn differently from an absence.
	failed := CatchTimeline(CatchInputs{VerifyReason: "verify ledger unreadable: EPERM", VerifyError: true,
		ReviewReason: "call log unreadable: EPERM", ReviewError: true,
		GuardReason: "hooks log unreadable: EPERM", GuardError: true}, CatchFilter{}, 1, 50)
	for _, s := range failed.Kinds {
		if s.Count.State != StateError || !strings.Contains(s.Count.Reason, "EPERM") {
			t.Errorf("lane %s over an unreadable instrument = %+v, want an error", s.Kind, s.Count)
		}
	}
	// A hooks-log generation that could not be read makes the guard count a
	// lower bound, never a complete count.
	part := in
	part.GuardPartial = "the rotated hooks log could not be read"
	if v := count(CatchTimeline(part, CatchFilter{}, 1, 50), CatchGuardRefusal); v.State != StatePartial || v.V != 1 || v.Reason == "" {
		t.Errorf("guard lane with an unread generation = %+v, want a partial lower bound", v)
	}
}

// TestSessionWindow_Bounded pins OC-43: the table is one row per session
// (the last ledger row wins, never one per row), filtered, sorted and paged;
// a replay window never carries more than MaxWindowEvents events whatever
// is asked for, pages the rest in time order, and filters by channel while
// still counting every channel.
func TestSessionWindow_Bounded(t *testing.T) {
	ledger := `{"session_id":"s-1","arm":"on","repo":"fathom","at":"2026-09-24T10:00:00Z","turn_count":3}
{"session_id":"s-1","arm":"on","repo":"fathom","at":"2026-09-24T10:05:00Z","turn_count":7,"tokens_in":10,"tokens_out":5,"raw_discovery_tokens_est":100}
{"session_id":"s-2","arm":"off","repo":"app-kit","at":"2026-09-25T09:00:00Z","turn_count":4,"fathom_calls":6}
{"session_id":"","repo":"fathom","at":"2026-09-25T11:00:00Z"}
not json
`
	tbl := SessionTable("/l.jsonl", []byte(ledger), nil, SessionFilter{}, 1, 50)
	if tbl.Sessions.V != 3 || tbl.LedgerRows.V != 4 || tbl.Anonymous.V != 1 || len(tbl.Rows) != 3 {
		t.Fatalf("table = %+v, want 3 sessions from 4 rows", tbl)
	}
	// The table's session count is the shared sessions.total projection.
	if shared := SessionCount("/l.jsonl", []byte(ledger), nil); tbl.Sessions.V != shared.V || tbl.Sessions.Denominator != shared.Denominator || tbl.Sessions.Source != shared.Source {
		t.Errorf("table sessions %+v, shared metric %+v", tbl.Sessions, shared)
	}
	if tbl.Rows[0].At != "2026-09-25T11:00:00Z" || tbl.Rows[1].ID != "s-2" || tbl.Rows[2].ID != "s-1" {
		t.Errorf("newest first: %+v", tbl.Rows)
	}
	s1 := tbl.Rows[2]
	if s1.Rows != 2 || *s1.Turns != 7 || *s1.Tokens != 15 || s1.FathomCalls == nil || *s1.FathomCalls != 0 {
		t.Errorf("s-1 = %+v: the last row wins, tokens are in+out, a measured row's omitted fathom_calls is zero", s1)
	}
	if tbl.Rows[0].Turns != nil || tbl.Rows[0].FathomCalls != nil {
		t.Errorf("an unrecorded figure stays nil, never zero: %+v", tbl.Rows[0])
	}
	if off := SessionTable("/l.jsonl", []byte(ledger), nil, SessionFilter{Arm: "off"}, 1, 50); off.Matching.V != 1 || off.Rows[0].ID != "s-2" {
		t.Errorf("arm filter = %+v", off)
	}
	if none := SessionTable("/l.jsonl", []byte(ledger), nil, SessionFilter{Arm: "nope"}, 1, 50); none.Matching.State != StateZero || none.Sessions.V != 3 {
		t.Errorf("a filter matching nothing is a measured zero: %+v", none.Matching)
	}
	// No ledger, or an unreadable one: every count is absent or an error
	// with its reason — the JSON carries no zero for any of them.
	for _, c := range []struct {
		err  error
		want State
	}{{os.ErrNotExist, StateAbsent}, {os.ErrPermission, StateError}} {
		m := SessionTable("/missing.jsonl", nil, c.err, SessionFilter{}, 1, 50)
		for name, v := range map[string]Value[int]{"sessions": m.Sessions, "matching": m.Matching, "ledger_rows": m.LedgerRows, "anonymous_rows": m.Anonymous} {
			if v.State != c.want || v.Reason == "" {
				t.Errorf("%v: %s = %+v, want %s with a reason", c.err, name, v, c.want)
			}
		}
		raw, _ := json.Marshal(m)
		for _, key := range []string{`"sessions":{"v"`, `"matching":{"v"`, `"ledger_rows":{"v"`, `"anonymous_rows":{"v"`} {
			if strings.Contains(string(raw), key) {
				t.Errorf("%v: the table's JSON carries a number for an unmeasured count: %s", c.err, raw)
			}
		}
		if len(m.Rows) != 0 {
			t.Errorf("%v: no rows without a ledger", c.err)
		}
	}
	if byTurns := SessionTable("/l.jsonl", []byte(ledger), nil, SessionFilter{Sort: "turns"}, 1, 50); byTurns.Rows[0].ID != "s-1" || byTurns.Rows[2].Turns != nil {
		t.Errorf("sort by turns (unrecorded last) = %+v", byTurns.Rows)
	}
	if paged := SessionTable("/l.jsonl", []byte(ledger), nil, SessionFilter{}, 9, 2); paged.Page != 2 || paged.Pages != 2 || len(paged.Rows) != 1 {
		t.Errorf("paging clamps = page %d of %d, %d rows", paged.Page, paged.Pages, len(paged.Rows))
	}

	base := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	var events []ReplayEvent
	for i := 0; i < 5000; i++ {
		ch := "edit-hook"
		if i%5 == 0 {
			ch = "discovery"
		}
		// Out of order on purpose: the window sorts by time.
		events = append(events, ReplayEvent{At: base.Add(time.Duration(5000-i) * time.Second), Channel: ch, Kind: "seen"})
	}
	w := SessionWindow("s-1", ReplaySource{Events: events}, nil, 1, 1_000_000)
	if len(w.Window) != MaxWindowEvents || w.PerPage != MaxWindowEvents || w.Pages != 3 || w.Events.V != 5000 {
		t.Fatalf("window = %d events, per page %d, %d pages; want at most %d", len(w.Window), w.PerPage, w.Pages, MaxWindowEvents)
	}
	if !w.Window[0].At.Before(w.Window[1].At) {
		t.Error("the window runs oldest first")
	}
	def := SessionWindow("s-1", ReplaySource{Events: events}, nil, 3, 0)
	if def.PerPage != DefaultWindowEvents || def.Page != 3 || len(def.Window) != DefaultWindowEvents || def.From == "" {
		t.Errorf("default window = %+v", def.Page)
	}
	last := SessionWindow("s-1", ReplaySource{Events: events}, nil, 99, 0)
	if last.Page != last.Pages || len(last.Window) != 5000-(last.Pages-1)*DefaultWindowEvents {
		t.Errorf("the last page is clamped: page %d of %d, %d events", last.Page, last.Pages, len(last.Window))
	}
	disc := SessionWindow("s-1", ReplaySource{Events: events}, []string{"discovery"}, 1, 0)
	if disc.Matching.V != 1000 || disc.Events.V != 5000 || len(disc.Channels) != 2 {
		t.Errorf("channel filter = matching %+v of %+v, channels %+v", disc.Matching, disc.Events, disc.Channels)
	}
	for _, c := range disc.Channels {
		if c.Selected != (c.Channel == "discovery") {
			t.Errorf("channel %s selected = %v", c.Channel, c.Selected)
		}
	}
	raw, _ := json.Marshal(w)
	if len(raw) > 400*1024 {
		t.Errorf("one window's JSON is %d bytes", len(raw))
	}
	if none := SessionWindow("nope", ReplaySource{}, nil, 1, 0); none.Events.State != StateAbsent || none.Events.Reason == "" || len(none.Window) != 0 {
		t.Errorf("an unknown session names the absence: %+v", none)
	}
	// A hooks log that exists and could not be read is an error naming the
	// failure, never "no beats recorded"; a generation that could not be
	// read makes the counts lower bounds.
	bad := SessionWindow("s-1", ReplaySource{Reason: "hooks log unreadable: permission denied", Error: true}, nil, 1, 0)
	if bad.Events.State != StateError || !strings.Contains(bad.Events.Reason, "permission denied") || bad.Matching.State != StateError {
		t.Errorf("unreadable hooks log = %+v", bad)
	}
	if raw, _ := json.Marshal(bad); strings.Contains(string(raw), `"events":{"v"`) {
		t.Errorf("an unreadable log's replay carries a number: %s", raw)
	}
	part := SessionWindow("s-1", ReplaySource{Events: events[:10], Partial: "the rotated hooks log could not be read"}, nil, 1, 0)
	if part.Events.State != StatePartial || part.Events.V != 10 {
		t.Errorf("a partly read log = %+v, want a partial lower bound", part.Events)
	}
}

// TestOperateView_SurfacesAndCallerClasses pins OC-44: every surface's
// calls, p50, p95 and error rate per day from the resolved call log; a day
// with no rows at all is absent, a surface silent on a busy day is a
// measured zero; the tool table labels each tool by its caller classes, and
// the console's own reads are never counted.
func TestOperateView_SurfacesAndCallerClasses(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	rows := []CallRow{
		{TS: "2026-09-24T10:00:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 100},
		{TS: "2026-09-24T10:01:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 300, Error: "boom"},
		{TS: "2026-09-24T10:02:00Z", Tool: "obligations", Origin: "edit-hook", LatencyMS: 20},
		{TS: "2026-09-25T09:00:00Z", Tool: "list_services", Origin: "verify-cli", LatencyMS: 5},
		{TS: "2026-09-25T09:01:00Z", Tool: "verify_plan", Origin: "verify-cli", LatencyMS: 900},
		{TS: "2026-09-25T09:02:00Z", Tool: "ingest", Surface: "agent", LatencyMS: 50},
		{TS: "2026-09-25T09:03:00Z", Tool: "obligations", LatencyMS: 7},
		{TS: "2026-09-25T09:04:00Z", Tool: "list_services", Origin: "console", LatencyMS: 1},
	}
	m := OperateView(OperateInputs{
		Daemon:  DaemonFacts{Version: "v1", Commit: "abc", StartedAt: now.Add(-time.Hour), RSSBytes: 300 << 20, HeapBytes: 50 << 20, Goroutines: 42},
		Calls:   rows,
		CallLog: ResolvedCallLog{Path: "/logs/mcp-calls.jsonl", Rule: RuleDaemon}, Now: now, Days: 3,
	})
	if strings.Join(m.Days, ",") != "2026-09-23,2026-09-24,2026-09-25" || strings.Join(m.Surfaces, ",") != "(unscoped),agent" {
		t.Fatalf("axes = %v × %v", m.Days, m.Surfaces)
	}
	cell := func(day, surface string) SurfaceDay {
		for _, c := range m.Traffic {
			if c.Day == day && c.Surface == surface {
				return c
			}
		}
		t.Fatalf("no cell %s %s", day, surface)
		return SurfaceDay{}
	}
	if c := cell("2026-09-23", "agent"); c.Calls.State != StateAbsent || !strings.Contains(c.Calls.Reason, "absent, not zero") {
		t.Errorf("a day with no rows must be absent: %+v", c)
	}
	if c := cell("2026-09-24", "agent"); c.Calls.V != 2 || *c.P50MS != 100 || *c.P95MS != 300 || *c.ErrorRate != 50 {
		t.Errorf("agent 09-24 = %+v", c)
	}
	if c := cell("2026-09-25", "agent"); c.Calls.V != 1 {
		t.Errorf("agent 09-25 = %+v", c)
	}
	if c := cell("2026-09-25", "(unscoped)"); c.Calls.V != 3 {
		t.Errorf("unscoped 09-25 = %+v, want 3 (the console read excluded)", c)
	}
	if d := m.DayTotals[0]; d.Calls.State != StateAbsent {
		t.Errorf("an empty day's total is absent: %+v", d)
	}
	if m.WindowCalls.V != 7 {
		t.Errorf("window calls = %+v, want 7 usage calls (console read excluded)", m.WindowCalls)
	}
	classes := map[CallerClass]ClassTraffic{}
	for _, c := range m.Classes {
		classes[c.Class] = c
	}
	for class, want := range map[CallerClass]int{ClassAgent: 2, ClassEditHook: 1, ClassGate: 2, ClassRefresh: 1, ClassUnattributed: 1} {
		if classes[class].Calls != want {
			t.Errorf("class %s = %d calls, want %d", class, classes[class].Calls, want)
		}
	}
	if _, ok := classes[ClassConsole]; ok {
		t.Error("console reads are not a usage class")
	}
	tools := map[string]ToolLoad{}
	for _, tl := range m.Tools {
		tools[tl.Tool] = tl
	}
	if ob := tools["obligations"]; ob.Calls != 2 || len(ob.By) != 2 || ob.Class != ClassEditHook {
		t.Errorf("obligations load = %+v (edit hook 1, unattributed 1; the tie goes to the earlier class)", ob)
	}
	if ls := tools["list_services"]; ls.Calls != 1 || ls.Class != ClassGate {
		t.Errorf("list_services = %+v: origin verify-cli is the gate; the console's read is not counted", ls)
	}
	if m.Daemon.MemoryMB.V != 300 || m.Daemon.UptimeSec.V != 3600 || m.Daemon.Goroutines.V != 42 {
		t.Errorf("daemon = %+v", m.Daemon)
	}
	// The load a CLI verb prints (`fathom metrics load`) is OperateView's
	// traffic part exactly — the page's panels and the verb share it.
	load := LoadView(OperateInputs{Calls: rows, CallLog: ResolvedCallLog{Path: "/logs/mcp-calls.jsonl", Rule: RuleDaemon}, Now: now, Days: 3})
	lj, _ := json.Marshal(load)
	mj, _ := json.Marshal(m.LoadModel)
	if string(lj) != string(mj) {
		t.Errorf("LoadView disagrees with OperateView:\nload %s\nview %s", lj, mj)
	}
	// Unreadable log: an error, every day.
	bad := OperateView(OperateInputs{CallLogReason: "EPERM", CallLogError: true, Now: now, Days: 2})
	if bad.WindowCalls.State != StateError || bad.DayTotals[1].Calls.State != StateError || bad.Daemon.MemoryMB.State != StateAbsent {
		t.Errorf("unreadable log = %+v", bad)
	}
	if bad.ToolsCalled.State != StateError || len(bad.Tools) != 0 {
		t.Errorf("unreadable log: tools called = %+v, want an error, never 0 tools", bad.ToolsCalled)
	}
	if raw, _ := json.Marshal(bad); strings.Contains(string(raw), `"tools_called":{"v"`) {
		t.Errorf("unreadable log carries a tool count: %s", raw)
	}
	if m.ToolsCalled.V != 5 || m.ToolsCalled.State != StatePresent {
		t.Errorf("tools called = %+v, want 5 measured", m.ToolsCalled)
	}
}

// TestDecideView_LogAndCalibration pins OC-45: the provider and its sites
// (on or off per [decide] sites), the ruling log newest first with applied,
// error and latency figures, and the calibration table parsed from the
// newest reading; a missing log is absent, never zero rulings.
func TestDecideView_LogAndCalibration(t *testing.T) {
	log := `{"ts":"2026-09-22T10:00:00Z","site":"certify_suggest","provider":"typesafe","kind":"choice","choice":"asserts","confidence":0.95,"latency_ms":800,"applied":true}
{"ts":"2026-09-23T10:00:00Z","site":"route","provider":"typesafe","kind":"choice","choice":"find_callers","confidence":0.4,"latency_ms":400,"applied":false}
{"ts":"2026-09-23T11:00:00Z","site":"route","provider":"typesafe","questions":1,"latency_ms":30000,"error":"timeout"}
{"ts":"2026-09-21T10:00:00Z","site":"obligations","provider":"llm","kind":"noul","confidence":0.7,"latency_ms":600,"applied":true}
`
	reading, err := os.ReadFile("../../docs/bench/readings/decision-port-calibration-2026-09-24.md")
	if err != nil {
		t.Fatal(err)
	}
	cal, ok := ParseCalibrationReading("docs/bench/readings/decision-port-calibration-2026-09-24.md", reading)
	if !ok {
		t.Fatal("the reading's bucket table did not parse")
	}
	if cal.Date != "2026-09-24" || !strings.Contains(cal.Title, "CV-M11") || !strings.Contains(cal.Verdict, "fails") {
		t.Errorf("reading header = %+v", cal)
	}
	if len(cal.Buckets) != 3 || cal.Buckets[0].Hits != 16 || cal.Buckets[0].N != 35 || cal.Buckets[0].Fraction != 0.457 || !strings.HasPrefix(cal.Buckets[0].Bucket, "high") {
		t.Errorf("buckets = %+v, want high 16/35 0.457 first", cal.Buckets)
	}
	if len(cal.Sweep) != 8 || cal.Sweep[0].Bucket != "1.000" || cal.Sweep[0].Hits != 4 || cal.Sweep[0].N != 14 {
		t.Errorf("sweep = %+v", cal.Sweep)
	}
	m := DecideView(DecideInputs{
		Config:   DecideConfig{Provider: "typesafe", Sites: []string{"route", "certify_suggest"}, RouteConfidence: 0.9},
		AllSites: []string{"route", "compile_prefilter", "obligations_rerank", "certify_suggest"},
		LogPath:  "/logs/decisions.jsonl", Log: []byte(log), Calibration: &cal,
	}, DecideFilter{}, 1, 50)
	if !m.ProviderOn || m.Rulings.V != 4 || m.Applied.V != 2 || m.Errors.V != 1 || m.MedianLatencyMS.V != 600 {
		t.Errorf("figures = on %v rulings %+v applied %+v errors %+v median %+v", m.ProviderOn, m.Rulings, m.Applied, m.Errors, m.MedianLatencyMS)
	}
	sites := map[string]SiteState{}
	var order []string
	for _, s := range m.Sites {
		sites[s.Site] = s
		order = append(order, s.Site)
	}
	if strings.Join(order, ",") != "route,compile_prefilter,obligations_rerank,certify_suggest,obligations" {
		t.Errorf("site order = %v (the vocabulary, then logged sites outside it)", order)
	}
	if !sites["route"].On || sites["compile_prefilter"].On || sites["route"].Rulings.V != 2 || sites["route"].Errors.V != 1 || sites["route"].Applied.State != StateZero || sites["compile_prefilter"].Rulings.State != StateZero || sites["compile_prefilter"].Applied.State != StateZero {
		t.Errorf("sites = %+v", m.Sites)
	}
	if m.Rows[0].TS != "2026-09-23T11:00:00Z" || m.Rows[3].TS != "2026-09-21T10:00:00Z" {
		t.Errorf("rows newest first: %+v", m.Rows)
	}
	if r := DecideView(DecideInputs{Log: []byte(log)}, DecideFilter{Site: "route"}, 1, 1); r.Matching != 2 || r.Pages != 2 || len(r.Rows) != 1 {
		t.Errorf("site filter and paging = %+v", r)
	}
	none := DecideView(DecideInputs{LogPath: "/x/decisions.jsonl", LogReason: "no decisions log at /x/decisions.jsonl", AllSites: []string{"route"}, ReadingReason: "no calibration reading"}, DecideFilter{}, 1, 50)
	if none.Rulings.State != StateAbsent || none.Sites[0].Rulings.State != StateAbsent || none.Calibration != nil || none.CalibrationNote == "" || none.ProviderOn {
		t.Errorf("missing log = %+v", none)
	}
	raw, _ := json.Marshal(none)
	if strings.Contains(string(raw), `"rulings":{"v"`) {
		t.Errorf("absent rulings carry a number: %s", raw)
	}
	// A site's applied and failed counts come from the same log: with no
	// log they are absent, and with an unreadable one an error — never 0.
	if s := none.Sites[0]; s.Applied.State != StateAbsent || s.Errors.State != StateAbsent {
		t.Errorf("missing log: site %s applied %+v errors %+v, want absent", s.Site, s.Applied, s.Errors)
	}
	for _, key := range []string{`"applied":{"v"`, `"errors":{"v"`, `"applied":0`, `"errors":0`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("missing log: the JSON carries %s: %s", key, raw)
		}
	}
	unreadable := DecideView(DecideInputs{LogPath: "/x/decisions.jsonl", LogReason: "decisions log unreadable: permission denied", LogError: true, AllSites: []string{"route"}}, DecideFilter{}, 1, 50)
	if s := unreadable.Sites[0]; s.Applied.State != StateError || s.Errors.State != StateError || s.Rulings.State != StateError {
		t.Errorf("unreadable log: site %+v, want every count an error", s)
	}
	if _, ok := ParseCalibrationReading("x.md", []byte("# nothing here\n")); ok {
		t.Error("a reading with no bucket table must not parse")
	}
}

package console

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestAskGuide_QuestionShapesToTools pins OC-47's projection: every question
// shape maps to exactly one tool, in routing order, and carries whether the
// tool is on the agent surface, how often it was asked on the project (absent
// — never 0 — when the call log was not read), and the project's own example
// input; a try-it sends exactly the tool's arguments and its answer is
// projected into a summary and rows, a bounded input refused before any call.
func TestAskGuide_QuestionShapesToTools(t *testing.T) {
	surface := []string{"find_symbol", "find_callers", "references", "blast_radius", "obligations", "staleness_report", "verify_plan", "search"}
	f := AskFacts{
		Service: "payments", AgentSurface: surface,
		AgentTools: Measured(25, "tools/list"),
		Calls:      Measured(40, "call log").WithDenominator("calls"),
		ToolCalls:  map[string]int{"find_symbol": 30, "obligations": 10},
		Window:     "the last 30 days",
		Samples: map[string]AskSample{
			"symbol": {Value: "validateCharge", Note: "most called"},
			"file":   {Value: "payment.go", Note: "its file"},
		},
	}
	v := AskGuide(f)

	var ids, tools []string
	for _, s := range v.Shapes {
		ids = append(ids, s.ID)
		tools = append(tools, s.Tool)
		if s.Question == "" || s.Gets == "" {
			t.Errorf("shape %s lacks its question or what it answers", s.ID)
		}
	}
	wantIDs := []string{"where", "who-calls", "who-uses", "what-breaks", "what-governs", "what-was-known", "is-it-done", "literal", "issue"}
	wantTools := []string{"find_symbol", "find_callers", "references", "blast_radius", "obligations", "staleness_report", "verify_plan", "search", "localize"}
	if !reflect.DeepEqual(ids, wantIDs) || !reflect.DeepEqual(tools, wantTools) {
		t.Fatalf("shapes = %v → %v", ids, tools)
	}
	if v.ToolsTaught != 9 {
		t.Errorf("tools taught = %d, want 9", v.ToolsTaught)
	}
	byID := map[string]AskShape{}
	for _, s := range v.Shapes {
		byID[s.ID] = s
	}
	// Surface membership is read from the allowlist, never assumed.
	if !byID["where"].OnAgentSurface || byID["issue"].OnAgentSurface {
		t.Errorf("surface membership: where %v, issue %v", byID["where"].OnAgentSurface, byID["issue"].OnAgentSurface)
	}
	// Asked: measured per tool; a tool nobody called is a measured zero.
	if a := byID["where"].Asked; a.State != StatePresent || a.V != 30 {
		t.Errorf("where asked = %+v", a)
	}
	if a := byID["literal"].Asked; a.State != StateZero {
		t.Errorf("an uncalled tool on a read log is a measured zero, got %+v", a)
	}
	if v.NeverAsked.V != 7 || v.NeverAsked.State != StatePresent {
		t.Errorf("never asked = %+v, want 7 of the 9 tools", v.NeverAsked)
	}
	// Samples: the project's own, by kind; a kind with none says why.
	if byID["who-calls"].Sample != "validateCharge" || byID["what-governs"].Sample != "payment.go" || byID["what-was-known"].Sample != "" {
		t.Errorf("samples: %+v", byID)
	}
	if s := byID["is-it-done"]; s.Sample != "" || s.SampleNote == "" {
		t.Errorf("a shape with no sample must say why: %+v", s)
	}

	// An unread log: every asked count is absent with the reason, never 0.
	f.Calls = Absent[int]("no call log at /tmp/x")
	v = AskGuide(f)
	for _, s := range v.Shapes {
		if s.Asked.State != StateAbsent || s.Asked.Reason != "no call log at /tmp/x" {
			t.Errorf("%s asked with no log = %+v", s.ID, s.Asked)
		}
		raw, _ := json.Marshal(s.Asked)
		if strings.Contains(string(raw), `"v"`) {
			t.Errorf("%s: an absent count must carry no v: %s", s.ID, raw)
		}
	}
	if v.NeverAsked.State != StateAbsent {
		t.Errorf("never asked with no log = %+v", v.NeverAsked)
	}
	// An unreadable log: every count is an error naming what failed.
	f.Calls = Errored[int]("call log unreadable: open x: permission denied")
	v = AskGuide(f)
	for _, s := range v.Shapes {
		if s.Asked.State != StateError || !strings.Contains(s.Asked.Reason, "permission denied") {
			t.Errorf("%s asked over an unreadable log = %+v", s.ID, s.Asked)
		}
	}
	if v.NeverAsked.State != StateError {
		t.Errorf("never asked over an unreadable log = %+v", v.NeverAsked)
	}

	// The try-it sends exactly the shape's argument.
	where, _ := AskShapeByID("where")
	breaks, _ := AskShapeByID("what-breaks")
	known, _ := AskShapeByID("what-was-known")
	if got := AskCall(where.Tool, AskArgs(where, "payments", " validateCharge ")); got != `find_symbol {"name":"validateCharge","service":"payments"}` {
		t.Errorf("where call = %s", got)
	}
	if got := AskCall(breaks.Tool, AskArgs(breaks, "payments", "payment.go")); got != `blast_radius {"changed_files":["payment.go"],"service":"payments"}` {
		t.Errorf("what-breaks call = %s", got)
	}
	if got := AskCall(known.Tool, AskArgs(known, "payments", "ignored")); got != `staleness_report {"service":"payments"}` {
		t.Errorf("a no-input shape sends only the project: %s", got)
	}
	// Inputs are bounded and checked before any call.
	literal, _ := AskShapeByID("literal")
	for input, want := range map[string]string{
		"":                       "type a symbol name",
		strings.Repeat("x", 301): "at most 300",
		"a\x00b":                 "control character",
	} {
		if got := AskInputProblem(where, input); !strings.Contains(got, want) {
			t.Errorf("input %q: problem %q, want %q", input, got, want)
		}
	}
	if got := AskInputProblem(literal, "("); !strings.Contains(got, "not a regular expression") {
		t.Errorf("a bad pattern must be refused: %q", got)
	}
	if AskInputProblem(known, "") != "" || AskInputProblem(where, "validateCharge") != "" {
		t.Error("valid inputs must pass")
	}
	if a := AskNotAsked(where, "payments", "", "type a symbol name"); a.State != StateAbsent || a.Reason == "" || a.Call == "" {
		t.Errorf("not asked = %+v", a)
	}

	// Answers are projected per tool, the tool's own summary kept.
	callers, _ := AskShapeByID("who-calls")
	a := AskAnswerFrom(callers, "payments", "validateCharge", []byte(`{"summary":"1 direct caller of validateCharge: Charge (payment.go:26)","callers":[{"loc":"payment.go:26 Charge","c":"h"}]}`), nil)
	if a.State != StatePresent || a.Total != 1 || a.Summary != "1 direct caller of validateCharge: Charge (payment.go:26)" ||
		!reflect.DeepEqual(a.Rows, [][]string{{"payment.go:26 Charge", "heuristic"}}) || !reflect.DeepEqual(a.Columns, []string{"Caller", "Edge"}) {
		t.Errorf("find_callers answer = %+v", a)
	}
	a = AskAnswerFrom(where, "payments", "validateCharge", []byte(`{"matched_via":"exact","symbols":[{"name":"validateCharge","kind":"GoFunc","file_path":"payment.go","line":34,"tier":"verified"}]}`), nil)
	if a.Summary != "1 symbol matched (exact match)" || !reflect.DeepEqual(a.Rows, [][]string{{"validateCharge", "GoFunc", "payment.go:34", "verified"}}) {
		t.Errorf("find_symbol answer = %+v", a)
	}
	a = AskAnswerFrom(known, "payments", "", []byte(`{"indexed_commit":"abc","current_commit":"def","reingest_recommended":true,"source_count":2,"typed_edges":{"known":false},"layers":{"obligations":{"fresh":3,"stale":1},"_last_compile":{}}}`), nil)
	if a.State != StatePresent || !strings.Contains(a.Summary, "behind HEAD") || len(a.Rows) != 6 || a.Rows[5][0] != "layer obligations" || a.Rows[5][1] != "3 fresh · 1 stale" {
		t.Errorf("staleness_report answer = %+v", a)
	}
	search, _ := AskShapeByID("literal")
	if a := AskAnswerFrom(search, "payments", "x", []byte(`{"summary":"no hits","hits":[]}`), nil); a.State != StateZero || a.Summary != "no hits" {
		t.Errorf("a tool that found nothing is a measured zero: %+v", a)
	}
	if a := AskAnswerFrom(search, "payments", "x", nil, errors.New("connection refused")); a.State != StateError || !strings.Contains(a.Reason, "connection refused") {
		t.Errorf("a failed call is an error: %+v", a)
	}
	if a := AskAnswerFrom(search, "payments", "x", []byte("tool error: pattern too long"), nil); a.State != StateError || !strings.Contains(a.Reason, "pattern too long") {
		t.Errorf("a non-JSON answer is an error naming what the tool said: %+v", a)
	}
	var many []string
	for i := 0; i < 40; i++ {
		many = append(many, `{"loc":"f.go:1","sym":"s","text":"t"}`)
	}
	if a := AskAnswerFrom(search, "payments", "x", []byte(`{"hits":[`+strings.Join(many, ",")+`]}`), nil); a.Total != 40 || len(a.Rows) != AskAnswerRows {
		t.Errorf("rows are bounded with the total stated: total %d rows %d", a.Total, len(a.Rows))
	}
}

// TestAskAnswerFrom_KeepsTheToolsElision: a tool that cut its answer to fit
// its budget says so (BA-1's elided block, RC-19's elision line, truncated,
// blast_radius' depth and symbol caps, localize's candidates_total), and the
// try-it keeps every word of it — the tool's own total, the rows it listed,
// how to get the rest — as a partial answer, never a complete one with the
// listed count passed off as the total.
func TestAskAnswerFrom_KeepsTheToolsElision(t *testing.T) {
	rowsOf := func(n int, row string) string {
		out := make([]string, n)
		for i := range out {
			out[i] = row
		}
		return "[" + strings.Join(out, ",") + "]"
	}
	shape := func(id string) AskShape {
		s, ok := AskShapeByID(id)
		if !ok {
			t.Fatalf("no shape %s", id)
		}
		return s
	}
	check := func(name string, a AskAnswer, state State, listed, total int, lowerBound bool, reasonHas, moreHas string) {
		t.Helper()
		if a.State != state || a.Listed != listed || a.Total != total || a.TotalLowerBound != lowerBound {
			t.Errorf("%s: state %s listed %d total %d lower bound %v; want %s %d %d %v (%s)", name, a.State, a.Listed, a.Total, a.TotalLowerBound, state, listed, total, lowerBound, a.Reason)
		}
		if reasonHas != "" && !strings.Contains(a.Reason, reasonHas) {
			t.Errorf("%s: reason %q lacks %q", name, a.Reason, reasonHas)
		}
		if moreHas != "" && !strings.Contains(a.HowToGetMore, moreHas) {
			t.Errorf("%s: how to get more %q lacks %q", name, a.HowToGetMore, moreHas)
		}
		if len(a.Rows) > AskAnswerRows {
			t.Errorf("%s: %d rows shown, the page shows at most %d", name, len(a.Rows), AskAnswerRows)
		}
		if a.State == StatePartial && a.Reason == "" {
			t.Errorf("%s: a partial answer must name what the tool left out", name)
		}
	}

	// find_callers (the real fleet, NewMemoryStore): the callers list was cut
	// by the tool's own budget line (116 more), and the dispatcher's budget
	// trimmed ANOTHER list (typed_callers: 160 kept of 277) — that block is
	// not about the rows shown, so it neither inflates nor deflates them.
	raw := `{"summary":"277 direct callers of NewMemoryStore: …","callers":` + rowsOf(161, `{"loc":"a.go:1 F","c":"r"}`) +
		`,"typed_callers":` + rowsOf(160, `{"key":"k","kind":"direct"}`) +
		`,"elision":"elided: 116 more — refine with a package-qualified symbol (pkg.Name)","elided":{"count":117,"of":277,"ordering":"as returned","how_to_get_more":"narrow the query"}}`
	a := AskAnswerFrom(shape("who-calls"), "fathom", "NewMemoryStore", []byte(raw), nil)
	check("find_callers", a, StatePartial, 161, 277, false, "161 of 277", "a package-qualified symbol (pkg.Name)")

	// references: the occurrences were cut (25 kept of 294); the two
	// definitions are listed in full, so the tool reported 296 rows.
	raw = `{"definitions":` + rowsOf(2, `{"file":"a.go","line":1,"symbol":"X"}`) + `,"occurrences":` + rowsOf(25, `{"loc":"b.go:2","text":"X()"}`) +
		`,"truncated":true,"elided":{"count":269,"of":294,"ordering":"service order, then source-scan order","how_to_get_more":"re-call with a larger \"max\""}}`
	a = AskAnswerFrom(shape("who-uses"), "fathom", "X", []byte(raw), nil)
	check("references", a, StatePartial, 27, 296, false, "269", `re-call with a larger "max"`)

	// search: 50 hits kept of 301.
	raw = `{"hits":` + rowsOf(50, `{"loc":"a.go:1","sym":"F","text":"x"}`) + `,"elided":{"count":251,"of":301,"ordering":"path, then line","how_to_get_more":"narrow path or pattern"}}`
	a = AskAnswerFrom(shape("literal"), "fathom", "x", []byte(raw), nil)
	check("search", a, StatePartial, 50, 301, false, "50 of 301", "narrow path or pattern")

	// blast_radius: the walk stopped at its symbol and depth caps, and the
	// dispatcher kept one depth bucket of two — the rows are a lower bound.
	raw = `{"by_depth":[{"depth":0,"count":3,"symbols":[{"key":"a"}]}],"tests":[],"symbols":` + rowsOf(500, `"s"`) +
		`,"total_symbols":1070,"truncated":true,"depth_capped":true,"max_depth":2,"elided":{"count":1,"of":2,"ordering":"as returned","how_to_get_more":"narrow the query"}}`
	a = AskAnswerFrom(shape("what-breaks"), "fathom", "internal/store/memory.go", []byte(raw), nil)
	check("blast_radius", a, StatePartial, 1, 2, true, "500 of 1,070 symbols", "narrow the query")
	if !strings.Contains(a.Reason, "depth 2") {
		t.Errorf("blast_radius: the depth cap must be named: %q", a.Reason)
	}

	// A bare truncated flag with no count: partial, its total a lower bound.
	a = AskAnswerFrom(shape("literal"), "fathom", "x", []byte(`{"hits":`+rowsOf(3, `{"loc":"a.go:1"}`)+`,"truncated":true}`), nil)
	check("truncated without a count", a, StatePartial, 3, 3, true, "truncated", "")

	// localize: candidates_total and the elided block state one cut, never
	// counted twice.
	raw = `{"candidates":` + rowsOf(20, `{"file":"a.go","symbol":"F","score":0.5}`) + `,"candidates_total":80,"elided":{"count":60,"of":80,"ordering":"score desc","how_to_get_more":"budget"}}`
	a = AskAnswerFrom(shape("issue"), "fathom", "x", []byte(raw), nil)
	check("localize", a, StatePartial, 20, 80, false, "20 of 80", "budget")

	// staleness_report's rows are facts, not a list: a cut of its sources
	// list (source_count stays the true total) leaves the facts complete.
	raw = `{"indexed_commit":"abc","current_commit":"abc","source_count":2000,"sources":["a.go"],"elision":"elided: 1999 more — refine with read_source","elided":{"count":1999,"of":2000,"ordering":"as returned","how_to_get_more":"narrow"}}`
	a = AskAnswerFrom(shape("what-was-known"), "fathom", "", []byte(raw), nil)
	if a.State != StatePresent || a.HowToGetMore != "" {
		t.Errorf("staleness_report with its sources list cut = %s %q", a.State, a.HowToGetMore)
	}

	// A complete answer states its total exactly and carries no elision.
	a = AskAnswerFrom(shape("literal"), "fathom", "x", []byte(`{"hits":`+rowsOf(40, `{"loc":"a.go:1"}`)+`}`), nil)
	check("complete", a, StatePresent, 40, 40, false, "", "")
	if a.HowToGetMore != "" || a.Reason != "" {
		t.Errorf("a complete answer carries no elision: %+v", a)
	}
}

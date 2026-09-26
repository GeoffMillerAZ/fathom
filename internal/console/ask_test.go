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

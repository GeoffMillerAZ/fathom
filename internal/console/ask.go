package console

// ask.go — OC-47 (vision 35 §3 Learn, design §3 Learn): "Ask the right
// way". Question shapes mapped to the agent-surface tool that answers
// them — the routing fathom's operating stanza gives every agent (SB-7)
// and the decision port's route site judges prompts against (DP-9) — each
// with a try-it: one real, read-only call of that tool on the chosen
// project, its answer projected here into a summary and rows. Nothing new
// is exposed: the try-it calls the same tool an agent calls, on the same
// surface, and the projection only reshapes what the tool returned.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// AskShape is one question shape and the tool that answers it.
type AskShape struct {
	// ID is the shape; where the decision port's route site names one
	// (DP-9: where, who-calls, what-governs, what-was-known, literal,
	// issue) it is that label.
	ID       string `json:"id"`
	Question string `json:"question"`
	Tool     string `json:"tool"`
	Gets     string `json:"gets"`
	// Arg is the tool argument the question fills ("" when the question
	// needs no input); ArgLabel says what to type.
	Arg      string `json:"arg,omitempty"`
	ArgLabel string `json:"arg_label,omitempty"`
	// Sample is this project's example input and SampleNote where it came
	// from — or, with Sample empty, why there is none. SampleError is set
	// when the example's source could not be read (an instrument failure,
	// never "none found").
	Sample      string `json:"sample,omitempty"`
	SampleNote  string `json:"sample_note,omitempty"`
	SampleError string `json:"sample_error,omitempty"`
	// OnAgentSurface: the tool is on /rpc/agent, the surface every
	// connected agent session sees.
	OnAgentSurface bool `json:"on_agent_surface"`
	// Asked is how often the tool was called for this project in the
	// traffic window — absent when the call log was not read, never 0.
	Asked Value[int] `json:"asked"`
	// sampleKind picks the sample: symbol, file, row, pattern.
	sampleKind string
}

// askShapes is the routing, in the order an agent should reach for it: the
// stanza's where / who-calls / what-breaks / what-governs / what-was-known,
// the gate, then the literal and bug-location shapes the route site knows.
var askShapes = []AskShape{
	{ID: "where", Question: "Where is X defined?", Tool: "find_symbol",
		Gets: "the definition — exact match first, then case-insensitive, then substring — with its file, line, signature and evidence tier",
		Arg:  "name", ArgLabel: "Symbol name", sampleKind: "symbol"},
	{ID: "who-calls", Question: "Who calls X?", Tool: "find_callers",
		Gets: "the direct callers, each edge labelled resolved (type-checked) or heuristic (matched by name)",
		Arg:  "symbol", ArgLabel: "Symbol name", sampleKind: "symbol"},
	{ID: "who-uses", Question: "Where is X used — every mention?", Tool: "references",
		Gets: "the definitions and every occurrence, typed hits first; the call sites a rename must touch",
		Arg:  "symbol", ArgLabel: "Symbol name", sampleKind: "symbol"},
	{ID: "what-breaks", Question: "What breaks if I change this file?", Tool: "blast_radius",
		Gets: "the file's symbols, their transitive callers by depth and the tests that reach them",
		Arg:  "changed_files", ArgLabel: "Repository-relative file path", sampleKind: "file"},
	{ID: "what-governs", Question: "What governs this file?", Tool: "obligations",
		Gets: "the rules served before an edit to the file, each with its citation and tier",
		Arg:  "file", ArgLabel: "Repository-relative file path", sampleKind: "file"},
	{ID: "what-was-known", Question: "Is the index current — what does fathom hold for this project?", Tool: "staleness_report",
		Gets: "the indexed commit against HEAD, the files indexed, the typed-edge pass and each layer's claims"},
	{ID: "is-it-done", Question: "Is this plan row really done?", Tool: "verify_plan",
		Gets: "the row checked against the index — pass, fail or unknown — with the criterion that decided it",
		Arg:  "plan_markdown", ArgLabel: "One plan row (Markdown checklist line)", sampleKind: "row"},
	{ID: "literal", Question: "Where does this exact string appear?", Tool: "search",
		Gets: "grep with a receipt: each hit with the symbol it sits in",
		Arg:  "pattern", ArgLabel: "Regular expression", sampleKind: "pattern"},
	{ID: "issue", Question: "Where does this failure live?", Tool: "localize",
		Gets: "a bounded walk from an error, a stack trace or a failing test to the files it most likely lives in, each with why",
		Arg:  "concern", ArgLabel: "What is failing (an error, a test, an area)", sampleKind: "symbol"},
}

// AskShapes returns the question shapes in order (a copy).
func AskShapes() []AskShape { return append([]AskShape(nil), askShapes...) }

// AskShapeByID finds one shape.
func AskShapeByID(id string) (AskShape, bool) {
	for _, s := range askShapes {
		if s.ID == id {
			return s, true
		}
	}
	return AskShape{}, false
}

// AskSample is one example input read from the project, and where it came
// from (or, when Value is empty, why there is none). Err is set when the
// source could not be read: the note then says what failed, and the
// example is an error, not an absence.
type AskSample struct {
	Value string
	Note  string
	Err   string
}

// AskFacts is what the caller read for one project.
type AskFacts struct {
	Service string
	// AgentSurface is /rpc/agent's allowlist; AgentTools the shared
	// surface.agent_tools metric (what tools/list returned).
	AgentSurface []string
	AgentTools   Value[int]
	// Calls is every call for the project in Window, carrying the call
	// log's read state; ToolCalls splits it by tool.
	Calls     Value[int]
	ToolCalls map[string]int
	Window    string
	// Samples are keyed by kind: symbol, file, row, pattern.
	Samples map[string]AskSample
}

// AskView is the page's projection.
type AskView struct {
	Service    string     `json:"service"`
	Window     string     `json:"window"`
	Shapes     []AskShape `json:"shapes"`
	AgentTools Value[int] `json:"agent_tools"`
	// ToolsTaught is how many distinct agent-surface tools the guide
	// teaches; NeverAsked how many of those were never called here in the
	// window (absent when the call log was not read).
	ToolsTaught int        `json:"tools_taught"`
	NeverAsked  Value[int] `json:"never_asked"`
	Answer      *AskAnswer `json:"answer,omitempty"`
}

// AskGuide maps each question shape to its tool for one project: whether
// the tool is on the agent surface, how often it was asked here, and the
// project's own example input for the try-it.
func AskGuide(f AskFacts) AskView {
	v := AskView{Service: f.Service, Window: f.Window, AgentTools: f.AgentTools}
	onSurface := map[string]bool{}
	for _, t := range f.AgentSurface {
		onSurface[t] = true
	}
	taught := map[string]bool{}
	never := 0
	for _, s := range askShapes {
		s.OnAgentSurface = onSurface[s.Tool]
		switch f.Calls.State {
		case StateAbsent, StateError, "":
			reason := f.Calls.Reason
			if reason == "" {
				reason = "the call log was not read"
			}
			if f.Calls.State == StateError {
				s.Asked = Errored[int](reason)
			} else {
				s.Asked = Absent[int](reason)
			}
		default:
			s.Asked = Measured(f.ToolCalls[s.Tool], f.Calls.Source).WithDenominator("calls to " + s.Tool + " for " + f.Service + " in " + f.Window)
		}
		if !taught[s.Tool] {
			taught[s.Tool] = true
			if s.Asked.Known() && s.Asked.V == 0 {
				never++
			}
		}
		if s.sampleKind != "" {
			if smp, ok := f.Samples[s.sampleKind]; ok {
				s.Sample, s.SampleNote, s.SampleError = smp.Value, smp.Note, smp.Err
			} else {
				s.SampleNote = "no example could be read from this project"
			}
		}
		v.Shapes = append(v.Shapes, s)
	}
	v.ToolsTaught = len(taught)
	reason := f.Calls.Reason
	if reason == "" {
		reason = "the call log was not read"
	}
	switch {
	case f.Calls.Known():
		v.NeverAsked = Measured(never, f.Calls.Source).WithDenominator(fmt.Sprintf("of the %d tools this guide teaches, never called for %s in %s", len(taught), f.Service, f.Window))
	case f.Calls.State == StateError:
		v.NeverAsked = Errored[int](reason).WithSource(f.Calls.Source)
	default:
		v.NeverAsked = Absent[int](reason)
	}
	return v
}

// askInputMax bounds a try-it input: a plan row may be a long line; every
// other input is a name, a path or a pattern.
func askInputMax(s AskShape) int {
	if s.Arg == "plan_markdown" {
		return 2000
	}
	return 300
}

// AskInputProblem says why an input cannot be sent ("" when it can): a
// shape with an argument needs one, bounded, without control characters.
func AskInputProblem(s AskShape, input string) string {
	if s.Arg == "" {
		return ""
	}
	if strings.TrimSpace(input) == "" {
		return "type a " + strings.ToLower(s.ArgLabel) + " to ask " + s.Tool
	}
	if len(input) > askInputMax(s) {
		return fmt.Sprintf("the input is %d bytes; %s takes at most %d here", len(input), s.Tool, askInputMax(s))
	}
	for _, r := range input {
		if unicode.IsControl(r) && r != '\t' {
			return "the input holds a control character"
		}
	}
	if s.Arg == "pattern" {
		if _, err := regexp.Compile(input); err != nil {
			return "not a regular expression: " + err.Error()
		}
	}
	return ""
}

// AskArgs is the exact argument object a try-it sends: the project, and
// the input in the shape's argument.
func AskArgs(s AskShape, service, input string) map[string]any {
	args := map[string]any{"service": service}
	switch s.Arg {
	case "":
	case "changed_files":
		args[s.Arg] = []string{strings.TrimSpace(input)}
	case "plan_markdown", "pattern":
		args[s.Arg] = input
	default:
		args[s.Arg] = strings.TrimSpace(input)
	}
	return args
}

// AskCall renders a call as the tool name and its JSON arguments (keys
// sorted) — the exact call an agent makes on the agent surface.
func AskCall(tool string, args map[string]any) string {
	raw, err := json.Marshal(args)
	if err != nil {
		return tool
	}
	return tool + " " + string(raw)
}

// AskAnswerRows bounds the rows an answer shows; the tool's own total is
// always stated.
const AskAnswerRows = 25

// AskAnswer is one try-it: the call made and the tool's answer, projected
// into a summary and rows.
type AskAnswer struct {
	Shape string         `json:"shape"`
	Tool  string         `json:"tool"`
	Input string         `json:"input,omitempty"`
	Args  map[string]any `json:"args,omitempty"`
	Call  string         `json:"call"`
	// State is present (rows found), zero (the tool answered and found
	// nothing), partial (the tool cut its answer to fit its budget —
	// Reason says what it left out, in its own words), absent (the call
	// was not made — Reason says why) or error.
	State   State      `json:"state"`
	Reason  string     `json:"reason,omitempty"`
	Summary string     `json:"summary,omitempty"`
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
	// Listed is how many rows the tool listed; Rows shows at most
	// AskAnswerRows of them.
	Listed int `json:"listed"`
	// Total is how many rows the tool reported: the rows it listed plus
	// the ones it said it left out. TotalLowerBound is set when the tool
	// said it stopped early without saying by how much (a bare
	// truncated flag, a depth cap): Total is then "at least".
	Total           int  `json:"total"`
	TotalLowerBound bool `json:"total_lower_bound,omitempty"`
	// HowToGetMore is the tool's own instruction for the rest of a cut
	// answer (the elided block's how_to_get_more, the elision line's
	// refinement); "" for a complete answer.
	HowToGetMore string `json:"how_to_get_more,omitempty"`
}

// askListField is the list each tool's rows are drawn from — the list a
// budget cut (the elided block, the elision line) is about when it names
// that list's size — and what its entries are called. staleness_report's
// rows are facts, not a list.
var askListField = map[string][2]string{
	"find_symbol":  {"symbols", "symbols"},
	"find_callers": {"callers", "callers"},
	"references":   {"occurrences", "occurrences"},
	"blast_radius": {"by_depth", "depth rows"},
	"obligations":  {"rules", "rules"},
	"verify_plan":  {"tasks", "plan rows"},
	"search":       {"hits", "hits"},
	"localize":     {"candidates", "candidates"},
}

// elisionLineRE is RC-19's one-line disclosure (internal/mcp.elisionLine).
var elisionLineRE = regexp.MustCompile(`^elided: (\d+) more — refine with (.+)$`)

// askCut is what a tool said it left out of the list the rows come from.
type askCut struct {
	dropped    int      // rows of that list the tool dropped, by its own count
	stated     int      // the tool's own total for that list (0: none stated)
	lowerBound bool     // the tool stopped early without saying by how much
	notes      []string // the tool's disclosure, in words
	more       string   // how to get the rest, in the tool's words
}

// askCutOf reads a tool's budget disclosure (docs/design/budgeted-answers.md
// §2: silent truncation is a defect) against the rows listed from its list
// field. An elided block counts toward these rows only when its kept count
// (of − count) is exactly that list's length: the dispatcher's budget trims
// whichever array is largest, and a cut of another list (find_callers'
// typed_callers) says nothing about the rows shown.
func askCutOf(tool string, obj map[string]any, listed int) askCut {
	var c askCut
	field, noun := askListField[tool][0], askListField[tool][1]
	if field == "" {
		return c
	}
	inList := len(asSlice(obj[field]))
	if tool == "find_callers" {
		if m := elisionLineRE.FindStringSubmatch(str(obj["elision"])); m != nil {
			n, _ := strconv.Atoi(m[1])
			c.dropped += n
			c.more = "refine with " + m[2]
		}
	}
	if el, ok := obj["elided"].(map[string]any); ok {
		count, of := atoi(el["count"]), atoi(el["of"])
		if count > 0 && of-count == inList {
			c.dropped += count
			if c.more == "" {
				c.more = str(el["how_to_get_more"])
			}
			if ord := str(el["ordering"]); ord != "" && ord != "as returned" {
				c.notes = append(c.notes, "the "+noun+" it kept are the first by "+ord)
			}
		}
	}
	if tool == "localize" {
		c.stated = atoi(obj["candidates_total"])
	}
	if tool == "blast_radius" {
		if obj["truncated"] == true {
			total, symbols := atoi(obj["total_symbols"]), len(asSlice(obj["symbols"]))
			c.notes = append(c.notes, fmt.Sprintf("the walk stopped at its symbol cap: %s of %s symbols listed, and the depth rows count only those", thousands(symbols), thousands(total)))
		}
		if obj["depth_capped"] == true {
			c.lowerBound = true
			c.notes = append(c.notes, "the walk stopped at its depth cap (depth "+str(obj["max_depth"])+"); callers beyond it were not followed")
		}
	} else if obj["truncated"] == true && c.dropped == 0 && c.stated <= listed {
		c.lowerBound = true
		c.notes = append(c.notes, tool+" says its answer is truncated without saying by how much")
	}
	return c
}

// AskNotAsked is a try-it that was not sent, with the reason.
func AskNotAsked(s AskShape, service, input, reason string) AskAnswer {
	args := AskArgs(s, service, input)
	return AskAnswer{Shape: s.ID, Tool: s.Tool, Input: input, Args: args, Call: AskCall(s.Tool, args), State: StateAbsent, Reason: reason}
}

// AskUnreadable is a try-it that could not be sent because what it needed
// could not be read (its example's source) — an error naming what failed.
func AskUnreadable(s AskShape, service, input, reason string) AskAnswer {
	a := AskNotAsked(s, service, input, reason)
	a.State = StateError
	return a
}

// AskAnswerFrom projects one tool's JSON answer (raw, the tool's text
// content) or its failure into an AskAnswer.
func AskAnswerFrom(s AskShape, service, input string, raw []byte, callErr error) AskAnswer {
	args := AskArgs(s, service, input)
	a := AskAnswer{Shape: s.ID, Tool: s.Tool, Input: input, Args: args, Call: AskCall(s.Tool, args)}
	if callErr != nil {
		a.State, a.Reason = StateError, s.Tool+" did not answer: "+callErr.Error()
		return a
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		said := strings.TrimSpace(string(raw))
		if len(said) > 300 {
			said = said[:300] + "…"
		}
		a.State, a.Reason = StateError, s.Tool+" did not answer with a JSON object: "+said
		return a
	}
	if sum, ok := obj["summary"].(string); ok {
		a.Summary = sum
	}
	var rows [][]string
	switch s.Tool {
	case "find_symbol":
		a.Columns = []string{"Symbol", "Kind", "Location", "Tier"}
		for _, m := range objects(obj["symbols"]) {
			rows = append(rows, []string{str(m["name"]), str(m["kind"]), loc(m["file_path"], m["line"]), str(m["tier"])})
		}
		if a.Summary == "" {
			a.Summary = fmt.Sprintf("%s matched", countNoun(len(rows), "symbol"))
			if via := str(obj["matched_via"]); via != "" {
				a.Summary += " (" + via + " match)"
			}
		}
	case "find_callers":
		a.Columns = []string{"Caller", "Edge"}
		for _, m := range objects(obj["callers"]) {
			rows = append(rows, []string{str(m["loc"]), edgeWord(str(m["c"]))})
		}
	case "references":
		a.Columns = []string{"Kind", "Location", "Symbol or text"}
		for _, m := range objects(obj["definitions"]) {
			rows = append(rows, []string{"definition", loc(m["file"], m["line"]), str(m["symbol"])})
		}
		for _, m := range objects(obj["occurrences"]) {
			where := str(m["loc"])
			if where == "" {
				where = loc(m["file"], m["line"])
			}
			text := str(m["text"])
			if text == "" {
				text = str(m["symbol"])
			}
			rows = append(rows, []string{"occurrence", where, text})
		}
	case "blast_radius":
		a.Columns = []string{"Depth", "Symbols", "First of them"}
		for _, m := range objects(obj["by_depth"]) {
			var names []string
			for _, sym := range objects(m["symbols"]) {
				if k := str(sym["key"]); k != "" && len(names) < 3 {
					names = append(names, k)
				}
			}
			rows = append(rows, []string{str(m["depth"]), str(m["count"]), strings.Join(names, ", ")})
		}
		if tests := len(asSlice(obj["tests"])); tests > 0 {
			rows = append(rows, []string{"tests", strconv.Itoa(tests), strings.Join(firstStrings(obj["tests"], 3), ", ")})
		}
	case "obligations":
		a.Columns = []string{"Rule", "Citation", "Tier"}
		for _, m := range objects(obj["rules"]) {
			tier := str(m["tier"])
			if st := str(m["status"]); st != "" {
				tier += " · " + st
			}
			rows = append(rows, []string{str(m["rule"]), str(m["src"]), tier})
		}
	case "staleness_report":
		a.Columns = []string{"Fact", "Value"}
		ic, cc := str(obj["indexed_commit"]), str(obj["current_commit"])
		rows = append(rows,
			[]string{"indexed commit", orWords(ic, "not recorded")},
			[]string{"current HEAD", orWords(cc, "not a git checkout, or not readable from the daemon")},
			[]string{"re-ingest recommended", yesNo(obj["reingest_recommended"])},
			[]string{"source files", str(obj["source_count"])},
		)
		if te, ok := obj["typed_edges"].(map[string]any); ok {
			known, _ := te["known"].(bool)
			switch {
			case !known:
				rows = append(rows, []string{"typed-edge pass", "unknown — no pass has run in this daemon process"})
			case te["failed"] == true:
				rows = append(rows, []string{"typed-edge pass", "failed — " + str(te["reason"])})
			default:
				rows = append(rows, []string{"typed-edge pass", str(te["packages"]) + " packages"})
			}
		}
		if layers, ok := obj["layers"].(map[string]any); ok {
			names := make([]string, 0, len(layers))
			for name := range layers {
				if !strings.HasPrefix(name, "_") {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			for _, name := range names {
				rows = append(rows, []string{"layer " + name, statusCounts(layers[name])})
			}
		}
		if a.Summary == "" {
			switch {
			case ic != "" && cc != "" && ic != cc:
				a.Summary = "the index is behind HEAD: indexed " + shortRev(ic) + ", HEAD " + shortRev(cc)
			case ic != "" && cc != "":
				a.Summary = "the index is current at " + shortRev(ic)
			case ic != "":
				a.Summary = "indexed at " + shortRev(ic) + "; HEAD could not be read to compare"
			default:
				a.Summary = "no indexed commit is recorded for " + service
			}
		}
	case "verify_plan":
		a.Columns = []string{"Row", "Title", "Verdict", "Why"}
		for _, m := range objects(obj["tasks"]) {
			why := str(m["label"])
			for _, c := range objects(m["criteria"]) {
				if str(c["status"]) == "fail" {
					why = str(c["name"]) + ": " + str(c["detail"])
					break
				}
			}
			rows = append(rows, []string{str(m["id"]), str(m["title"]), str(m["status"]), why})
		}
		if sum, ok := obj["summary"].(map[string]any); ok {
			a.Summary = fmt.Sprintf("%s pass, %s fail, %s unknown of %s", str(sum["pass"]), str(sum["fail"]), str(sum["unknown"]), countNoun(atoi(sum["total"]), "row"))
		}
	case "search":
		a.Columns = []string{"Location", "In symbol", "Line"}
		for _, m := range objects(obj["hits"]) {
			rows = append(rows, []string{str(m["loc"]), str(m["sym"]), str(m["text"])})
		}
	case "localize":
		a.Columns = []string{"File", "Symbol", "Score", "Why"}
		for _, m := range objects(obj["candidates"]) {
			rows = append(rows, []string{str(m["file"]), str(m["symbol"]), str(m["score"]), strings.Join(firstStrings(m["why"], 2), "; ")})
		}
	}
	a.Listed = len(rows)
	cut := askCutOf(s.Tool, obj, a.Listed)
	a.Total = max(a.Listed+cut.dropped, cut.stated)
	a.TotalLowerBound = cut.lowerBound
	if len(rows) > AskAnswerRows {
		rows = rows[:AskAnswerRows]
	}
	a.Rows = rows
	if a.Summary == "" {
		a.Summary = s.Tool + " answered"
	}
	switch {
	case a.Total > a.Listed || len(cut.notes) > 0 || cut.lowerBound:
		a.State = StatePartial
		reason := s.Tool + " listed " + thousands(a.Listed)
		if left := a.Total - a.Listed; left > 0 {
			total := thousands(a.Total)
			if a.TotalLowerBound {
				total = "at least " + total
			}
			reason += fmt.Sprintf(" of %s — it left out %s to fit its budget", total, thousands(left))
		} else if a.TotalLowerBound {
			reason += " and stopped early, so there may be more"
		}
		a.Reason = strings.Join(append([]string{reason}, cut.notes...), "; ")
		a.HowToGetMore = cut.more
	case a.Total == 0 && s.Tool != "staleness_report":
		a.State = StateZero
	default:
		a.State = StatePresent
	}
	return a
}

func objects(v any) []map[string]any {
	var out []map[string]any
	for _, e := range asSlice(v) {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func firstStrings(v any, n int) []string {
	var out []string
	for _, e := range asSlice(v) {
		if len(out) == n {
			break
		}
		if s := str(e); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// str renders a JSON scalar as text: a whole number without a decimal
// point, a bool as yes/no, nothing for null.
func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', 2, 64)
	case bool:
		if t {
			return "yes"
		}
		return "no"
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

func atoi(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func loc(file, line any) string {
	f := str(file)
	if l := str(line); l != "" && l != "0" {
		return f + ":" + l
	}
	return f
}

func edgeWord(c string) string {
	switch c {
	case "r":
		return "resolved"
	case "h":
		return "heuristic"
	}
	return c
}

func yesNo(v any) string {
	if b, ok := v.(bool); ok && b {
		return "yes"
	}
	return "no"
}

func orWords(s, words string) string {
	if s == "" {
		return words
	}
	return s
}

// statusCounts renders a layer's {status: n} map as "12 fresh · 2 stale".
func statusCounts(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return str(v)
	}
	var parts []string
	for _, st := range []string{"fresh", "stale", "contradicted", "superseded", "refuted"} {
		if n := atoi(m[st]); n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+st)
		}
	}
	if len(parts) == 0 {
		return "no claims"
	}
	return strings.Join(parts, " · ")
}

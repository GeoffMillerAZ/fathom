//go:build decideexp

package decide_test

// exp_e3_test.go — E3's table arm, the judge step (EXP-M3;
// docs/bench/readings/decide/exp-preregistration.md section 9, the shape of
// docs/design/decision-models.md section 6). The agent writes a typed spec
// and selects rows with an existing fathom verb's JSON; this step resolves
// each row to a Go span through the subject's store, computes first with
// internal/ask's facts where the spec says a fact answers its question,
// asks the provider only for the residue, in one batched request, and
// prints a receipted table, computed and judged rows apart.
//
//	FATHOM_DECIDE_EXP=1 FATHOM_DECIDE_EXP_OUT=<scratch>/e3-<q>.json \
//	FATHOM_DECIDE_EXP_E3_SPEC=<scratch>/e3-<q>-spec.json \
//	FATHOM_DECIDE_EXP_E3_ROWS=<scratch>/e3-<q>-rows.json \
//	FATHOM_DECIDE_EXP_SUBJECTS='fathom|<checkout at fd091c4f>|<store>' \
//	FATHOM_DECIDE_EXP_CAP_USD=<$2 less the spend E1 and E2 recorded> \
//	FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE=<absolute path, chmod 600> \
//	  go test -tags decideexp -run 'TestDecideExpE3$' -count=1 -timeout 20m -v ./internal/decide
//
// The spec (JSON, unknown fields refused):
//
//	{"id": "q3", "kind": "predicate" | "choice",
//	 "question": "<the statement or question each row is asked>",
//	 "options": [{"name": "...", "criteria": "..."}, ..., {"name": "none", ...}],  // choice only; a none exit is required
//	 "computed": [{"fact": "executes_only", "holds": true, "answer": "false"}],     // optional, first match wins
//	 "max_rows": 60, "max_questions": 64}
//
// A computed rule answers a row outright when its fact (one of
// ask.FactDefinitions) applies to the row's span and holds (or not) as
// the rule says; every other row is the residue. max_rows defaults to 60
// and is at most 255; max_questions defaults to and is at most 64, one
// request's bound, which 96 KiB of state and questions bounds too. A
// residue row past either bound, or whose question would pass 16 KiB, is
// listed as not consulted with its reason, never dropped.
//
// The rows file is one verb's JSON output as it emits it: a `fathom
// serve-mcp` tools/call response line (or its structuredContent) from
// find_callers (callers[].loc, then transitive_callers[].key), blast_radius
// (by_depth[].symbols[].key) or test_impact (tests[]); `fathom tests select
// --json` (ranked_tests[].test); or a plain JSON list of keys or of {key}
// objects. scratch/e3-tooling.md has the exact commands. Each row is
// resolved through the subject's store to its span (file, start and end
// line); a row that names no Go function or method in the index, repeats
// an earlier row, passes max_rows or whose span the checkout does not hold
// is pruned, and the receipt names it and why.
//
// FATHOM_DECIDE_EXP_DRY=1 ($0) answers every residue row through the kit's
// validating mock (a predicate false at 0.05, a choice none at 0.95), and
// FATHOM_DECIDE=off or a decision.None from decide.Open still computes and
// prints the table with the residue not consulted. The receipt (JSON) goes
// to FATHOM_DECIDE_EXP_OUT and the table to stdout; neither carries a row's
// source text.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// expE3State is every E3 judged request's State, fixed before any E3 call.
const expE3State = "Each question below is about one row of a table of Go code: a function, method or test, shown in the question. Answer each independently."

// The spec's caps (design section 6) and one request's bound.
const (
	expE3DefaultRows = 60
	expE3MaxRows     = 255
)

type expE3Option struct {
	Name     string `json:"name"`
	Criteria string `json:"criteria,omitempty"`
}

type expE3Rule struct {
	Fact   string `json:"fact"`
	Holds  bool   `json:"holds"`
	Answer string `json:"answer"`
}

type expE3Spec struct {
	ID           string        `json:"id"`
	Kind         decision.Kind `json:"kind"`
	Question     string        `json:"question"`
	Options      []expE3Option `json:"options,omitempty"`
	Computed     []expE3Rule   `json:"computed,omitempty"`
	MaxRows      int           `json:"max_rows,omitempty"`
	MaxQuestions int           `json:"max_questions,omitempty"`
}

// expE3ParseSpec decodes and checks a spec, filling the caps' defaults.
func expE3ParseSpec(data []byte) (expE3Spec, error) {
	var s expE3Spec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, fmt.Errorf("spec: %w", err)
	}
	if strings.TrimSpace(s.ID) == "" || strings.ContainsAny(s.ID, " \t\n/@") {
		return s, fmt.Errorf("spec: id %q must be a short name with no space, slash or @", s.ID)
	}
	if strings.TrimSpace(s.Question) == "" {
		return s, fmt.Errorf("spec %s: question is empty", s.ID)
	}
	var answers []string
	switch s.Kind {
	case decision.KindPredicate:
		if len(s.Options) > 0 {
			return s, fmt.Errorf("spec %s: a predicate takes no options (its answers are true and false)", s.ID)
		}
		answers = []string{"true", "false"}
	case decision.KindChoice:
		if len(s.Options) < 2 || len(s.Options) > 255 {
			return s, fmt.Errorf("spec %s: a choice takes 2 to 255 options, got %d", s.ID, len(s.Options))
		}
		seen := map[string]bool{}
		for _, o := range s.Options {
			if strings.TrimSpace(o.Name) == "" || seen[o.Name] {
				return s, fmt.Errorf("spec %s: option names must be non-empty and unique (%q)", s.ID, o.Name)
			}
			seen[o.Name] = true
			answers = append(answers, o.Name)
		}
		if !seen["none"] {
			return s, fmt.Errorf("spec %s: a choice needs the none exit among its options", s.ID)
		}
	default:
		return s, fmt.Errorf("spec %s: kind %q; the E3 prototype takes predicate or choice", s.ID, s.Kind)
	}
	for _, r := range s.Computed {
		known := false
		for _, d := range ask.FactDefinitions {
			known = known || d.Fact == r.Fact
		}
		if !known {
			return s, fmt.Errorf("spec %s: computed fact %q is none of internal/ask's five", s.ID, r.Fact)
		}
		ok := false
		for _, a := range answers {
			ok = ok || a == r.Answer
		}
		if !ok {
			return s, fmt.Errorf("spec %s: computed answer %q is not one of %s", s.ID, r.Answer, strings.Join(answers, ", "))
		}
	}
	switch {
	case s.MaxRows == 0:
		s.MaxRows = expE3DefaultRows
	case s.MaxRows < 0 || s.MaxRows > expE3MaxRows:
		return s, fmt.Errorf("spec %s: max_rows %d, want 1 to %d", s.ID, s.MaxRows, expE3MaxRows)
	}
	switch {
	case s.MaxQuestions == 0:
		s.MaxQuestions = ask.ChunkQuestions
	case s.MaxQuestions < 0 || s.MaxQuestions > ask.ChunkQuestions:
		return s, fmt.Errorf("spec %s: max_questions %d, want 1 to %d (one request)", s.ID, s.MaxQuestions, ask.ChunkQuestions)
	}
	return s, nil
}

// expE3Ref is one row as the verb named it: a key, or a file:line loc with
// the symbol's name.
type expE3Ref struct {
	Key   string
	File  string
	Line  int
	Name  string
	Depth int
}

func (r expE3Ref) String() string {
	if r.Key != "" {
		return r.Key
	}
	return fmt.Sprintf("%s:%d %s", r.File, r.Line, r.Name)
}

// expE3ParseLoc reads find_callers' "file:line Name".
func expE3ParseLoc(loc string) (expE3Ref, bool) {
	at, name, _ := strings.Cut(loc, " ")
	i := strings.LastIndex(at, ":")
	if i <= 0 {
		return expE3Ref{}, false
	}
	line, err := strconv.Atoi(at[i+1:])
	if err != nil || line <= 0 {
		return expE3Ref{}, false
	}
	return expE3Ref{File: at[:i], Line: line, Name: strings.TrimSpace(name)}, true
}

// expE3Refs reads the rows file: which verb's JSON it is and its rows in
// the order the verb emitted them.
func expE3Refs(data []byte) (string, []expE3Ref, error) {
	var doc any
	if err := json.Unmarshal(bytes.TrimSpace(data), &doc); err != nil {
		return "", nil, fmt.Errorf("rows: %w", err)
	}
	if m, ok := doc.(map[string]any); ok {
		if e, ok := m["error"]; ok {
			return "", nil, fmt.Errorf("rows: the verb answered an error: %v", e)
		}
		if res, ok := m["result"].(map[string]any); ok {
			switch {
			case res["structuredContent"] != nil:
				doc = res["structuredContent"]
			default:
				content, _ := res["content"].([]any)
				first, _ := func() (map[string]any, bool) {
					if len(content) == 0 {
						return nil, false
					}
					c, ok := content[0].(map[string]any)
					return c, ok
				}()
				text, _ := first["text"].(string)
				if err := json.Unmarshal([]byte(text), &doc); err != nil {
					return "", nil, fmt.Errorf("rows: a tools/call result with no JSON content: %w", err)
				}
			}
		}
	}
	str := func(v any) string { s, _ := v.(string); return s }
	num := func(v any) int { f, _ := v.(float64); return int(f) }
	objs := func(v any) []map[string]any {
		var out []map[string]any
		list, _ := v.([]any)
		for _, x := range list {
			if o, ok := x.(map[string]any); ok {
				out = append(out, o)
			}
		}
		return out
	}
	var refs []expE3Ref
	switch d := doc.(type) {
	case map[string]any:
		switch {
		case d["callers"] != nil:
			for _, c := range objs(d["callers"]) {
				r, ok := expE3ParseLoc(str(c["loc"]))
				if !ok {
					return "", nil, fmt.Errorf("rows: find_callers loc %q is not file:line Name", str(c["loc"]))
				}
				r.Depth = 1
				refs = append(refs, r)
			}
			for _, c := range objs(d["transitive_callers"]) {
				refs = append(refs, expE3Ref{Key: str(c["key"]), Depth: num(c["depth"])})
			}
			return "find_callers", refs, nil
		case d["by_depth"] != nil:
			for _, b := range objs(d["by_depth"]) {
				for _, s := range objs(b["symbols"]) {
					refs = append(refs, expE3Ref{Key: str(s["key"]), Depth: num(s["depth"])})
				}
			}
			return "blast_radius", refs, nil
		case d["ranked_tests"] != nil:
			for _, rt := range objs(d["ranked_tests"]) {
				refs = append(refs, expE3Ref{Key: str(rt["test"]), File: str(rt["file"]), Line: num(rt["line"])})
			}
			return "tests select", refs, nil
		case d["tests"] != nil:
			list, _ := d["tests"].([]any)
			for _, k := range list {
				refs = append(refs, expE3Ref{Key: str(k)})
			}
			return "test_impact", refs, nil
		}
	case []any:
		for _, x := range d {
			switch v := x.(type) {
			case string:
				refs = append(refs, expE3Ref{Key: v})
			case map[string]any:
				refs = append(refs, expE3Ref{Key: str(v["key"])})
			}
		}
		return "keys", refs, nil
	}
	return "", nil, fmt.Errorf("rows: not a shape this step reads (find_callers, blast_radius, test_impact, tests select --json, or a list of keys)")
}

// expE3Span is a resolved row.
type expE3Span struct {
	Ref   expE3Ref
	Node  core.SymbolNode
	Start int
	End   int
}

func (s expE3Span) span() string { return fmt.Sprintf("%s:%d-%d", s.Node.FilePath, s.Start, s.End) }

type expE3Pruned struct {
	Row    string `json:"row"`
	Reason string `json:"reason"`
}

func expIsGoFunc(n core.SymbolNode) bool {
	return (n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod) && strings.HasSuffix(n.FilePath, ".go")
}

// expE3Resolve resolves refs through nodes (the subject's symbols) and the
// checkout at root, in order, pruning what it cannot keep.
func expE3Resolve(refs []expE3Ref, nodes []core.SymbolNode, root string, maxRows int) ([]expE3Span, []expE3Pruned) {
	byKey := map[string]core.SymbolNode{}
	byLoc := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		if _, dup := byKey[n.Key()]; !dup {
			byKey[n.Key()] = n
		}
		loc := fmt.Sprintf("%s:%d", n.FilePath, n.Line)
		byLoc[loc] = append(byLoc[loc], n)
	}
	lineCounts := map[string]int{}
	lines := func(file string) int {
		if n, ok := lineCounts[file]; ok {
			return n
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		n := -1
		if err == nil {
			n = expLineCount(string(data))
		}
		lineCounts[file] = n
		return n
	}
	var rows []expE3Span
	var pruned []expE3Pruned
	seen := map[string]bool{}
	for _, r := range refs {
		var n core.SymbolNode
		found := false
		if r.Key != "" {
			n, found = byKey[r.Key]
		} else {
			var funcs []core.SymbolNode
			for _, c := range byLoc[fmt.Sprintf("%s:%d", r.File, r.Line)] {
				if !expIsGoFunc(c) {
					continue
				}
				if c.Name == r.Name || strings.HasSuffix(c.Name, "."+r.Name) {
					n, found = c, true
					break
				}
				funcs = append(funcs, c)
			}
			if !found && len(funcs) == 1 {
				n, found = funcs[0], true
			}
		}
		switch {
		case !found:
			pruned = append(pruned, expE3Pruned{Row: r.String(), Reason: "not in the index"})
			continue
		case !expIsGoFunc(n):
			pruned = append(pruned, expE3Pruned{Row: r.String(), Reason: "not a Go function or method"})
			continue
		case seen[n.Key()]:
			pruned = append(pruned, expE3Pruned{Row: r.String(), Reason: "repeats an earlier row"})
			continue
		}
		seen[n.Key()] = true
		if len(rows) >= maxRows {
			pruned = append(pruned, expE3Pruned{Row: n.Key(), Reason: fmt.Sprintf("past max_rows (%d)", maxRows)})
			continue
		}
		end := n.EndLine
		if end < n.Line {
			end = n.Line
		}
		if total := lines(n.FilePath); n.Line < 1 || total < end {
			pruned = append(pruned, expE3Pruned{Row: n.Key(), Reason: "span not resolvable in the checkout"})
			continue
		}
		rows = append(rows, expE3Span{Ref: r, Node: n, Start: n.Line, End: end})
	}
	return rows, pruned
}

// expE3Fact is the named fact out of f.
func expE3Fact(f ask.Facts, name string) ask.Fact {
	switch name {
	case "unchecked_error":
		return f.UncheckedError
	case "wrapped":
		return f.Wrapped
	case "executes_only":
		return f.ExecutesOnly
	case "locked_write":
		return f.LockedWrite
	case "threaded_context":
		return f.ThreadedContext
	}
	return ask.Fact{}
}

func expBandName(p float64) string {
	for i := len(ask.Bands) - 1; i > 0; i-- {
		if p >= ask.Bands[i].Lo {
			return ask.Bands[i].Name
		}
	}
	return ask.Bands[0].Name
}

// expE3Row is one receipted row (design section 6's columns); it never
// carries the row's source text, only its hash.
type expE3Row struct {
	Key         string  `json:"key"`
	Span        string  `json:"span"`
	UnitHash    string  `json:"unit_hash"`
	Tier        string  `json:"tier"`
	Answer      string  `json:"answer,omitempty"`
	P           float64 `json:"p"`
	Band        string  `json:"band,omitempty"`
	Template    string  `json:"template"`
	Calibration string  `json:"calibration"`
	Cache       string  `json:"cache"`
	DecidedBy   string  `json:"decided_by"`
	Reason      string  `json:"reason,omitempty"`
	Depth       int     `json:"depth,omitempty"`
}

type expE3Cost struct {
	Calls        int     `json:"calls"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	USD          float64 `json:"usd"`
	WouldCostUSD float64 `json:"would_cost_usd,omitempty"`
	Source       string  `json:"source"`
}

type expE3Receipt struct {
	Experiment      string         `json:"experiment"`
	Preregistration string         `json:"preregistration"`
	Template        string         `json:"template"`
	Kind            decision.Kind  `json:"kind"`
	Question        string         `json:"question"`
	RowsFrom        string         `json:"rows_from"`
	Dry             bool           `json:"dry"`
	Decider         string         `json:"decider"`
	Killed          string         `json:"killed,omitempty"`
	RowsIn          int            `json:"rows_in"`
	Selected        int            `json:"rows_selected"`
	ComputedN       int            `json:"computed"`
	ResidueN        int            `json:"residue"`
	JudgedN         int            `json:"judged"`
	NotConsultedN   int            `json:"not_consulted"`
	ComputedShare   float64        `json:"computed_share"`
	ResidueShare    float64        `json:"residue_share"`
	FactsExcluded   map[string]int `json:"facts_excluded,omitempty"`
	Answers         map[string]int `json:"answers"`
	Computed        []expE3Row     `json:"computed_rows"`
	Judged          []expE3Row     `json:"judged_rows"`
	Pruned          []expE3Pruned  `json:"pruned"`
	Cost            expE3Cost      `json:"cost"`
	Budget          ask.Budget     `json:"budget"`
	StopReason      string         `json:"stop_reason,omitempty"`
	LatencyMS       float64        `json:"latency_ms"`
	TableBytes      int            `json:"table_bytes"`
	WallSeconds     float64        `json:"wall_seconds"`
}

// expE3Render is the receipt as the table printed to stdout.
func expE3Render(w io.Writer, r expE3Receipt) {
	fmt.Fprintf(w, "E3 table arm: %s (%s, rows from %s)\n  %s\n", r.Template, r.Kind, r.RowsFrom, r.Question)
	fmt.Fprintf(w, "rows: %d in, %d selected, %d pruned; %d computed (%.3f), %d residue (%.3f): %d judged, %d not consulted\n",
		r.RowsIn, r.Selected, len(r.Pruned), r.ComputedN, r.ComputedShare, r.ResidueN, r.ResidueShare, r.JudgedN, r.NotConsultedN)
	if r.Killed != "" {
		fmt.Fprintf(w, "judged: not consulted (%s)\n", r.Killed)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\ncomputed\tkey\tspan\tanswer\tp\tdecided_by")
	for _, row := range r.Computed {
		fmt.Fprintf(tw, "\t%s\t%s\t%s\t%.2f\t%s\n", row.Key, row.Span, row.Answer, row.P, row.DecidedBy)
	}
	fmt.Fprintln(tw, "\njudged\tkey\tspan\tanswer\tp\tband\tdecided_by\tcalibration")
	for _, row := range r.Judged {
		if row.Tier != "judged" {
			continue
		}
		fmt.Fprintf(tw, "\t%s\t%s\t%s\t%.3f\t%s\t%s\t%s\n", row.Key, row.Span, row.Answer, row.P, row.Band, row.DecidedBy, row.Calibration)
	}
	fmt.Fprintln(tw, "\nnot consulted\tkey\tspan\treason")
	for _, row := range r.Judged {
		if row.Tier == "not_consulted" {
			fmt.Fprintf(tw, "\t%s\t%s\t%s\n", row.Key, row.Span, row.Reason)
		}
	}
	fmt.Fprintln(tw, "\npruned\trow\treason")
	for _, p := range r.Pruned {
		fmt.Fprintf(tw, "\t%s\t%s\n", p.Row, p.Reason)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\ncost: %d call, %d input tokens, $%.6f (%s)", r.Cost.Calls, r.Cost.InputTokens, r.Cost.USD, r.Cost.Source)
	if r.Cost.WouldCostUSD > 0 {
		fmt.Fprintf(w, "; would have cost $%.6f", r.Cost.WouldCostUSD)
	}
	fmt.Fprintln(w)
}

func expShortHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

// expE3Source is the text a row is asked about: Compute's source when it
// computed the span, else the declaration parsed alone, else the span's
// lines.
func expE3Source(root string, row expE3Span, c ask.Computed) string {
	if c.Excluded == "" && c.Source != "" {
		return c.Source
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Node.FilePath)))
	if err != nil {
		return ""
	}
	if _, text, ok := ask.FuncSource(row.Node.FilePath, data, c.Span); ok {
		return text
	}
	lines := strings.Split(string(data), "\n")
	return strings.Join(lines[row.Start-1:min(row.End, len(lines))], "\n")
}

func TestDecideExpE3(t *testing.T) {
	if os.Getenv("FATHOM_DECIDE_EXP") != "1" {
		t.Skip("E3 judge step: set FATHOM_DECIDE_EXP=1 (stage 0, the cap less E1's and E2's spend; FATHOM_DECIDE_EXP_DRY=1 for the $0 dry run)")
	}
	out := os.Getenv("FATHOM_DECIDE_EXP_OUT")
	specPath, rowsPath := os.Getenv("FATHOM_DECIDE_EXP_E3_SPEC"), os.Getenv("FATHOM_DECIDE_EXP_E3_ROWS")
	if out == "" || specPath == "" || rowsPath == "" {
		t.Fatal("FATHOM_DECIDE_EXP_OUT, FATHOM_DECIDE_EXP_E3_SPEC and FATHOM_DECIDE_EXP_E3_ROWS are all required")
	}
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := expE3ParseSpec(specBytes)
	if err != nil {
		t.Fatal(err)
	}
	rowsBytes, err := os.ReadFile(rowsPath)
	if err != nil {
		t.Fatal(err)
	}
	verb, refs, err := expE3Refs(rowsBytes)
	if err != nil {
		t.Fatal(err)
	}
	var sub *expSubject
	for _, s := range expSubjects(t) {
		if s.Name == "fathom" {
			sub = &s
			break
		}
	}
	if sub == nil {
		t.Fatal("FATHOM_DECIDE_EXP_SUBJECTS names no fathom subject")
	}
	dry := os.Getenv("FATHOM_DECIDE_EXP_DRY") == "1"
	start := time.Now()
	ctx := context.Background()
	template := spec.ID + "@" + expShortHash(specBytes)
	rec := expE3Receipt{
		Experiment: "E3 table arm, judge step (EXP-M3)", Preregistration: "docs/bench/readings/decide/exp-preregistration.md",
		Template: template, Kind: spec.Kind, Question: spec.Question, RowsFrom: verb, Dry: dry, RowsIn: len(refs),
		Answers: map[string]int{}, Computed: []expE3Row{}, Judged: []expE3Row{}, Pruned: []expE3Pruned{},
	}

	c, err := client.NewConstantClient(client.ModeLocal, sub.Store)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := c.ListAllSymbols(ctx, sub.Name)
	_ = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	rows, pruned := expE3Resolve(refs, nodes, sub.Root, spec.MaxRows)
	rec.Selected, rec.Pruned = len(rows), append(rec.Pruned, pruned...)
	spans := make([]ask.Span, len(rows))
	for i, r := range rows {
		spans[i] = ask.Span{Repo: sub.Name, File: r.Node.FilePath, Name: r.Node.Name, Start: r.Start, End: r.End}
	}
	computed, err := ask.Compute(ctx, sub.Root, spans, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Compute first: a row a rule's fact answers is computed; the rest is
	// the residue.
	type residue struct {
		row  expE3Row
		text string
	}
	var rest []residue
	for i, r := range rows {
		cs := computed[i]
		src := expE3Source(sub.Root, r, cs)
		row := expE3Row{Key: r.Node.Key(), Span: r.span(), UnitHash: expShortHash([]byte(src)), Template: template, Cache: "none", Depth: r.Ref.Depth}
		if cs.Excluded != "" {
			if rec.FactsExcluded == nil {
				rec.FactsExcluded = map[string]int{}
			}
			rec.FactsExcluded[cs.Excluded]++
		} else {
			for _, rule := range spec.Computed {
				if f := expE3Fact(cs.Facts, rule.Fact); f.Applies && f.Holds == rule.Holds {
					row.Tier, row.Answer, row.P, row.Band = "computed", rule.Answer, 1, "computed"
					row.Calibration, row.DecidedBy = "exact (computed)", "fact:"+rule.Fact
					break
				}
			}
		}
		if row.Tier == "computed" {
			rec.Computed = append(rec.Computed, row)
			rec.Answers[row.Answer]++
			continue
		}
		text := spec.Question + fmt.Sprintf("\n\nRow %s, %s:\n```go\n%s\n```", row.Key, row.Span, strings.TrimSuffix(src, "\n"))
		rest = append(rest, residue{row: row, text: text})
	}
	rec.ComputedN, rec.ResidueN = len(rec.Computed), len(rest)
	if rec.Selected > 0 {
		rec.ComputedShare = float64(rec.ComputedN) / float64(rec.Selected)
		rec.ResidueShare = float64(rec.ResidueN) / float64(rec.Selected)
	}

	notConsulted := func(r residue, reason string) {
		r.row.Tier, r.row.Reason, r.row.DecidedBy, r.row.Calibration = "not_consulted", reason, "none", "n/a"
		rec.Judged = append(rec.Judged, r.row)
		rec.NotConsultedN++
	}
	// One request: the residue rows that fit one request's bounds, in order.
	var ask1, over []residue
	var sizes []int
	for _, r := range rest {
		if len(r.text) > ask.MaxQuestionBytes {
			over = append(over, r)
			continue
		}
		ask1 = append(ask1, r)
		sizes = append(sizes, len(r.text))
	}
	if chunks := ask.Chunks(sizes, len(expE3State), spec.MaxQuestions, ask.ChunkBytes); len(chunks) > 1 {
		n := len(chunks[0])
		over = append(over, ask1[n:]...)
		ask1 = ask1[:n]
	}
	var options []decision.Option
	for _, o := range spec.Options {
		options = append(options, decision.Option{Name: o.Name, Criteria: o.Criteria})
	}

	rec.Decider, rec.Cost.Source = "none", "none"
	var a *expAsker
	if reason := expKillReason(); reason != "" {
		rec.Killed = reason
	} else if len(ask1) > 0 {
		a = newExpAsker(t, dry)
		rec.Killed = a.killed
		rec.Decider = a.name()
	}
	if rec.Killed != "" || len(ask1) == 0 {
		for _, r := range ask1 {
			notConsulted(r, rec.Killed)
		}
	} else {
		qs := make([]decision.Question, len(ask1))
		for i, r := range ask1 {
			qs[i] = decision.Question{ID: fmt.Sprintf("q%03d", i), Kind: spec.Kind, Instructions: r.text, Options: options}
		}
		dryAnswers := func() decision.Response {
			answers := make(map[string]decision.Answer, len(qs))
			for _, q := range qs {
				if spec.Kind == decision.KindChoice {
					answers[q.ID] = expMockChoice(q.ID, options, "none")
				} else {
					answers[q.ID] = expMockPredicate(q.ID, 0.05)
				}
			}
			return decision.Response{Provider: "mock", Model: "mock", Answers: answers, Latency: time.Millisecond}
		}
		sent := a.send(ctx, expE3State, qs, dryAnswers)
		rec.StopReason = a.stop
		decidedBy := sent.Resp.Provider
		if sent.Resp.Model != "" {
			decidedBy += "/" + sent.Resp.Model
		}
		if sent.Sent {
			rec.Cost.Calls = 1
			rec.Cost.InputTokens, rec.Cost.OutputTokens = sent.Resp.Usage.InputTokens, sent.Resp.Usage.OutputTokens
			rec.Cost.Source = "reported"
			if sent.Resp.Usage.InputTokens == 0 && sent.Resp.Usage.OutputTokens == 0 {
				rec.Cost.Source = "estimated"
				rec.Cost.InputTokens = ask.EstimateTokens(expRequestBytes(expE3State, qs))
				rec.Cost.OutputTokens = ask.OutputTokensPerQuestion * len(qs)
			}
			rec.Cost.USD = sent.USD
			if dry {
				rec.Cost.USD, rec.Cost.WouldCostUSD, rec.Cost.Source = 0, sent.USD, "dry (the mock answered; nothing was sent)"
			}
			rec.LatencyMS = float64(sent.Latency.Milliseconds())
		}
		for i, r := range ask1 {
			switch {
			case !sent.Sent:
				notConsulted(r, a.stop)
				continue
			case sent.Err != nil:
				notConsulted(r, "the request failed")
				continue
			}
			got, miss := expE2Answer(sent.Resp, qs[i].ID, spec.Kind == decision.KindChoice)
			if miss != "" {
				notConsulted(r, "asked; the answer was "+miss)
				continue
			}
			r.row.Tier, r.row.Answer, r.row.P, r.row.Band = "judged", got.answer, got.p, expBandName(got.p)
			r.row.Calibration, r.row.DecidedBy = "uncalibrated", decidedBy
			rec.Judged = append(rec.Judged, r.row)
			rec.JudgedN++
			rec.Answers[got.answer]++
		}
		if sent.Err != nil {
			t.Logf("the request failed: %v", sent.Err)
		}
	}
	for _, r := range over {
		notConsulted(r, fmt.Sprintf("past one request's bound (%d questions, %d KiB, %d KiB a question)", spec.MaxQuestions, ask.ChunkBytes/1024, ask.MaxQuestionBytes/1024))
	}
	if a != nil {
		rec.Budget = a.budget
	}
	var table bytes.Buffer
	expE3Render(&table, rec)
	rec.TableBytes = table.Len()
	rec.WallSeconds = time.Since(start).Seconds()
	expWriteJSON(t, out, rec)
	fmt.Print(table.String())
	t.Logf("%s: %d rows in, %d selected, %d computed, %d residue (%d judged, %d not consulted); $%.6f; receipt %s",
		template, rec.RowsIn, rec.Selected, rec.ComputedN, rec.ResidueN, rec.JudgedN, rec.NotConsultedN, rec.Cost.USD, out)
	if rec.Killed != "" {
		t.Skipf("%s: the computed rows are printed; the residue was not consulted", rec.Killed)
	}
}

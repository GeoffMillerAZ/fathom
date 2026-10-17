//go:build decideexp

package decide_test

// exp_helpers_test.go — pure tests of the E2 and E3 harnesses' helpers:
// no FATHOM_DECIDE_EXP, no store, no provider, so `go test -tags
// decideexp -run TestExp ./internal/decide` runs them anywhere.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestExpE2WordsAreThePreRegistrationsVerbatim: the State, both
// questions and every option's name and criteria appear in section 8 word
// for word (line wrapping aside).
func TestExpE2WordsAreThePreRegistrationsVerbatim(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "bench", "readings", "decide", "exp-preregistration.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := strings.Join(strings.Fields(string(data)), " ")
	want := []string{expE2State, expE2aStatement, expE2bStatement}
	for _, o := range expE2bOptions {
		want = append(want, "`"+o.Name+"` ("+o.Criteria+")")
	}
	for _, w := range want {
		if !strings.Contains(page, w) {
			t.Errorf("the pre-registration does not hold %q", w)
		}
	}
	var order []string
	for _, o := range expE2bOptions {
		order = append(order, o.Name)
	}
	if strings.Join(order, ",") != "code_bug,environment,timing,outdated_expectation,other" {
		t.Errorf("options out of the page's order: %v", order)
	}
}

func TestExpFuncnameRegexPinsTheDeclaration(t *testing.T) {
	cases := []struct{ decl, name, want string }{
		{"func Open(cfg config.Decide) (*Ports, error) {", "Open", `^func Open[[(]`},
		{"func (c *Client) TypedEdges(ctx context.Context) {", "Client.TypedEdges", `^func [(]c [*]Client[)] TypedEdges[[(]`},
		{"func (s *Set[K, V]) Add(k K) {", "Set.Add", `^func [(]s [*]Set[[]K[,] V[]][)] Add[[(]`},
		{"func Map[T any](xs []T) []T {", "Map", `^func Map[[(]`},
		{"func (Open) Open() {", "Open.Open", `^func [(]Open[)] Open[[(]`},
	}
	for _, c := range cases {
		got, ok := expFuncnameRegex(c.decl+"\n\treturn\n}", c.name)
		if !ok || got != c.want {
			t.Errorf("expFuncnameRegex(%q, %q) = %q, %v; want %q", c.decl, c.name, got, ok, c.want)
			continue
		}
		// The bracketed form reads the same in Go's syntax: it matches the
		// declaration's line and nothing else here.
		re := regexp.MustCompile(got)
		if !re.MatchString(c.decl) || re.MatchString("// "+c.decl) || re.MatchString("func Other() {") {
			t.Errorf("%q does not pin %q", got, c.decl)
		}
	}
	if _, ok := expFuncnameRegex("type T struct{}", "T"); ok {
		t.Error("a non-func line must not give a regex")
	}
	if _, ok := expFuncnameRegex("func (c *Client)\n\tTypedEdges() {", "Client.TypedEdges"); ok {
		t.Error("a name not on the first line must not give a regex")
	}
}

func TestExpDiffChangesCodeSkipsCommentsAndHeaders(t *testing.T) {
	head := "diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -1,3 +1,3 @@ func F() {\n"
	if expDiffChangesCode(head + " func F() {\n-\t// old\n+\t// new\n+\n \treturn\n") {
		t.Error("a comment-only change must not count")
	}
	if expDiffChangesCode(head + "-\t/* old */\n+\t/* new */\n") {
		t.Error("a block-comment change must not count")
	}
	if !expDiffChangesCode(head + "-\treturn 1\n+\treturn 2 // two\n") {
		t.Error("a changed statement must count")
	}
	if expDiffChangesCode("diff --git a/x.go b/x.go\n--- /dev/null\n+++ b/x.go\n") {
		t.Error("headers alone change no line")
	}
}

func TestExpCapLinesKeepsEightyAndSaysHowManyMore(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "l")
	}
	got := expCapLines(strings.Join(lines, "\n")+"\n", 80)
	if n := expLineCount(got); n != 80 || !strings.HasSuffix(got, "[... 21 more lines of this diff not shown]") {
		t.Errorf("capped to %d lines ending %q", n, got[len(got)-50:])
	}
	if got := expCapLines("a\nb\n", 80); got != "a\nb" {
		t.Errorf("a short diff changed: %q", got)
	}
	if h, d := expSplitLogL("abc123\n\ndiff --git a/x b/x\n+y\n\n"); h != "abc123" || d != "diff --git a/x b/x\n+y" {
		t.Errorf("split %q %q", h, d)
	}
}

func writeLines(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestExpE2LoadRefusesUntilEveryItemHasAFrontierLabel is the "labels
// before any call" rule: expE2Load runs before anything is opened, and any
// gap or bad label is a refusal.
func TestExpE2LoadRefusesUntilEveryItemHasAFrontierLabel(t *testing.T) {
	dir := t.TempDir()
	items := writeLines(t, dir, "items.jsonl",
		`{"id":"e2a-001","function":"F","file":"a.go","caller":"G","caller_file":"b.go","diff":"@@\n-x\n+y","caller_src":"func G() { F() }"}`,
		`{"id":"e2b-001","test":"TestX","file":"x_test.go","output":"--- FAIL: TestX","test_src":"func TestX(t *testing.T) {}"}`)
	good := writeLines(t, dir, "good.jsonl",
		`{"id":"e2a-001","label":"true","provenance":"frontier","note":"reads F's result"}`,
		`{"id":"e2b-001","label":"timing","provenance":"frontier"}`,
		`{"id":"e2b-001","label":"code_bug","provenance":"human"}`)
	in, err := expE2Load([]string{items}, good)
	if err != nil {
		t.Fatalf("a complete label set refused: %v", err)
	}
	if len(in.Items) != 2 || in.Frontier["e2b-001"] != "timing" || in.Human["e2b-001"] != "code_bug" || len(in.Hashes) != 2 {
		t.Errorf("loaded %+v", in)
	}
	bad := map[string][]string{
		"no frontier label":  {`{"id":"e2a-001","label":"true","provenance":"frontier"}`, `{"id":"e2b-001","label":"timing","provenance":"human"}`},
		"unknown item":       {`{"id":"e2a-001","label":"true","provenance":"frontier"}`, `{"id":"e2b-001","label":"timing","provenance":"frontier"}`, `{"id":"e2a-009","label":"true","provenance":"frontier"}`},
		"bad predicate":      {`{"id":"e2a-001","label":"yes","provenance":"frontier"}`, `{"id":"e2b-001","label":"timing","provenance":"frontier"}`},
		"bad option":         {`{"id":"e2a-001","label":"true","provenance":"frontier"}`, `{"id":"e2b-001","label":"flaky","provenance":"frontier"}`},
		"bad provenance":     {`{"id":"e2a-001","label":"true","provenance":"model"}`, `{"id":"e2b-001","label":"timing","provenance":"frontier"}`},
		"two frontier":       {`{"id":"e2a-001","label":"true","provenance":"frontier"}`, `{"id":"e2a-001","label":"false","provenance":"frontier"}`, `{"id":"e2b-001","label":"timing","provenance":"frontier"}`},
		"unknown field":      {`{"id":"e2a-001","label":"true","provenance":"frontier","p":0.9}`, `{"id":"e2b-001","label":"timing","provenance":"frontier"}`},
		"no labels at all":   {""},
		"label before items": {`{"id":"e2b-001","label":"timing","provenance":"frontier"}`},
	}
	for name, lines := range bad {
		if _, err := expE2Load([]string{items}, writeLines(t, dir, "bad.jsonl", lines...)); err == nil {
			t.Errorf("%s: not refused", name)
		}
	}
	long := strings.Repeat("+x\n", 81)
	tooLong := writeLines(t, dir, "long.jsonl", `{"id":"e2a-001","function":"F","file":"a.go","caller":"G","caller_file":"b.go","diff":"`+strings.ReplaceAll(long, "\n", `\n`)+`","caller_src":"func G() {}"}`)
	if _, err := expE2Load([]string{tooLong}, good); err == nil {
		t.Error("an 81-line diff must be refused")
	}
	mixed := writeLines(t, dir, "mixed.jsonl", `{"id":"e2a-001","function":"F","file":"a.go","caller":"G","caller_file":"b.go","diff":"+x","caller_src":"func G() {}","output":"x"}`)
	if _, err := expE2Load([]string{mixed}, good); err == nil {
		t.Error("an e2a item with e2b fields must be refused")
	}
}

func TestExpE2TextFramesTheItem(t *testing.T) {
	it := expE2Item{ID: "e2a-001", Function: "F", File: "a.go", Caller: "G", CallerFile: "b.go", Diff: "+x\n", CallerSrc: "func G() {}"}
	got := expE2Text(it)
	if !strings.HasPrefix(got, expE2aStatement+"\n\nChanged function F in a.go") || !strings.Contains(got, "```diff\n+x\n```") || !strings.HasSuffix(got, "```go\nfunc G() {}\n```") {
		t.Errorf("e2a text: %q", got)
	}
	b := expE2Text(expE2Item{ID: "e2b-001", Test: "TestX", File: "x_test.go", Output: "FAIL", TestSrc: "func TestX(t *testing.T) {}"})
	if !strings.HasPrefix(b, expE2bStatement+"\n\nTest TestX in x_test.go, its output:\n```text\nFAIL\n```") {
		t.Errorf("e2b text: %q", b)
	}
}

func TestExpE3SpecRefusesWhatTheShapeRefuses(t *testing.T) {
	s, err := expE3ParseSpec([]byte(`{"id":"q3","kind":"predicate","question":"This test fails if a stale rule is served.","computed":[{"fact":"executes_only","holds":true,"answer":"false"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.MaxRows != 60 || s.MaxQuestions != 64 {
		t.Errorf("defaults: rows %d questions %d", s.MaxRows, s.MaxQuestions)
	}
	if _, err := expE3ParseSpec([]byte(`{"id":"q4","kind":"choice","question":"Which?","options":[{"name":"ts"},{"name":"none"}],"max_rows":255}`)); err != nil {
		t.Errorf("a choice with a none exit refused: %v", err)
	}
	bad := map[string]string{
		"choice without none":    `{"id":"q","kind":"choice","question":"Which?","options":[{"name":"a"},{"name":"b"}]}`,
		"predicate with options": `{"id":"q","kind":"predicate","question":"Is it?","options":[{"name":"a"},{"name":"none"}]}`,
		"score":                  `{"id":"q","kind":"score","question":"How much?"}`,
		"unknown fact":           `{"id":"q","kind":"predicate","question":"Is it?","computed":[{"fact":"reachable","holds":true,"answer":"true"}]}`,
		"answer not an option":   `{"id":"q","kind":"choice","question":"Which?","options":[{"name":"a"},{"name":"none"}],"computed":[{"fact":"wrapped","holds":true,"answer":"b"}]}`,
		"rows past 255":          `{"id":"q","kind":"predicate","question":"Is it?","max_rows":256}`,
		"questions past 64":      `{"id":"q","kind":"predicate","question":"Is it?","max_questions":65}`,
		"unknown field":          `{"id":"q","kind":"predicate","question":"Is it?","provider":"x"}`,
		"empty question":         `{"id":"q","kind":"predicate","question":" "}`,
		"duplicate option":       `{"id":"q","kind":"choice","question":"Which?","options":[{"name":"none"},{"name":"none"}]}`,
	}
	for name, spec := range bad {
		if _, err := expE3ParseSpec([]byte(spec)); err == nil {
			t.Errorf("%s: not refused", name)
		}
	}
}

func TestExpE3RefsReadsEachVerbsShape(t *testing.T) {
	cases := []struct {
		name, doc, verb string
		want            []string
	}{
		{"find_callers over serve-mcp", `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{}"}],"structuredContent":{"callers":[{"loc":"cmd/x/a.go:55 openPorts","c":"r"}],"transitive_callers":[{"key":"svc::cmd/x/main.run","depth":2}],"typed_callers":[{"key":"svc::docs/plan.X"}]}}}`,
			"find_callers", []string{"cmd/x/a.go:55 openPorts", "svc::cmd/x/main.run"}},
		{"find_callers as text content", `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"callers\":[{\"loc\":\"a.go:3 F\"}]}"}]}}`,
			"find_callers", []string{"a.go:3 F"}},
		{"blast_radius", `{"by_depth":[{"depth":0,"symbols":[{"key":"svc::p.A","depth":0}]},{"depth":1,"symbols":[{"key":"svc::p.B","depth":1}]}],"symbols":["svc::p.A","svc::p.B"]}`,
			"blast_radius", []string{"svc::p.A", "svc::p.B"}},
		{"test_impact", `{"tests":["svc::p.TestA","svc::p.TestB"],"elided":{"count":0}}`, "test_impact", []string{"svc::p.TestA", "svc::p.TestB"}},
		{"tests select", `{"invocations":[],"ranked_tests":[{"test":"svc::p.TestA","file":"p/a_test.go","line":9}]}`, "tests select", []string{"svc::p.TestA"}},
		{"keys", `["svc::p.A",{"key":"svc::p.B"}]`, "keys", []string{"svc::p.A", "svc::p.B"}},
	}
	for _, c := range cases {
		verb, refs, err := expE3Refs([]byte(c.doc))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		var got []string
		for _, r := range refs {
			got = append(got, r.String())
		}
		if verb != c.verb || strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %s %v, want %s %v", c.name, verb, got, c.verb, c.want)
		}
	}
	for _, doc := range []string{`{"summary":"x"}`, `{"jsonrpc":"2.0","error":{"code":-32602}}`, `not json`} {
		if _, _, err := expE3Refs([]byte(doc)); err == nil {
			t.Errorf("%s: not refused", doc)
		}
	}
}

func TestExpMockChoiceIsAValidDistribution(t *testing.T) {
	a := expMockChoice("q", expE2bOptions, "timing")
	var sum float64
	for _, p := range a.Probabilities {
		sum += p
	}
	if a.Choice != "timing" || a.Probabilities["timing"] != 0.95 || len(a.Probabilities) != 5 || sum < 0.999999 || sum > 1.000001 {
		t.Errorf("mock choice %+v sums to %v", a, sum)
	}
	if a.Kind != decision.KindChoice {
		t.Errorf("kind %q", a.Kind)
	}
}

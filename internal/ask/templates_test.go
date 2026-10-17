package ask

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBaselinesAreTheGrepsTheyClaim(t *testing.T) {
	cases := []struct {
		template, src string
		want          bool
	}{
		{"errcheck", "func f() {\n\t_ = g()\n}", false},
		{"errcheck", "func f() {\n\tn, _ := strconv.Atoi(s)\n}", false},
		{"errcheck", "func f() {\n\tfmt.Println(x)\n}", false},
		{"errcheck", "func f() {\n\tw.Close()\n}", false},
		{"errcheck", "func f() {\n\tdefer w.Close()\n}", true},
		{"errcheck", "func f() error {\n\treturn g()\n}", true},
		{"errcheck", "func f() {\n\tfor i, _ := range xs {\n\t}\n}", true},
		{"wrap", "return fmt.Errorf(\"open: %v\", err)", false},
		{"wrap", "return fmt.Errorf(\"open: %w\", err)", true},
		{"wrap", "return fmt.Errorf(\"100%% %d: %v\", n, errClosed)", false},
		{"wrap", "return fmt.Errorf(\"%w and %s\", a, b)", true},
		{"assert", "t.Errorf(\"x\")", true},
		{"assert", "check(t, ok)", true},
		{"assert", "require.NoError(t, err)", true},
		{"assert", "t.Log(\"x\")", false},
		{"assert", "t.Run(\"a\", func(t *testing.T) {})", false},
		{"lock", "c.mu.Lock()\n\tc.n++", true},
		{"lock", "c.n++\n\tc.mu.Lock()", false},
		{"lock", "c.mu.RLock()\n\tc.rows = x", false},
		{"lock", "c.mu.Lock()\n\treturn c.n", true},
	}
	for _, c := range cases {
		tmpl, ok := TemplateByID(c.template)
		if !ok {
			t.Fatalf("no template %q", c.template)
		}
		if got := tmpl.BaselineAnswer(c.src); got != c.want {
			t.Errorf("%s baseline over %q = %v, want %v", c.template, c.src, got, c.want)
		}
	}
}

func TestQuestionTextFramesTheSpan(t *testing.T) {
	lock, _ := TemplateByID("lock")
	c := Computed{Span: Span{Repo: "fathom", File: "a/b.go"}, Line: 7, Source: "func (c *T) F() {}", Guard: "type T struct{ mu sync.Mutex; n int }"}
	got := QuestionText(lock, c)
	want := lock.Question + "\n\nReceiver type:\n```go\ntype T struct{ mu sync.Mutex; n int }\n```\n\nFunction fathom/a/b.go, line 7:\n```go\nfunc (c *T) F() {}\n```"
	if got != want {
		t.Errorf("QuestionText =\n%s\nwant\n%s", got, want)
	}
	wrap, _ := TemplateByID("wrap")
	if strings.Contains(QuestionText(wrap, c), "Receiver type") {
		t.Error("only the lock template carries the receiver type")
	}
}

// TestPreRegistrationPinsTheHarnessVerbatim holds EXP-1's document and
// this package identical: its first content line, every template's
// question and baseline, every fact definition, the request state, the
// seed, the quotas, the bands, the cap and the price basis appear in it
// word for word.
func TestPreRegistrationPinsTheHarnessVerbatim(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "bench", "readings", "decide", "exp-preregistration.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	sc := bufio.NewScanner(strings.NewReader(doc))
	first := ""
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" && !strings.HasPrefix(line, "#") {
			first = line
			break
		}
	}
	if !regexp.MustCompile(`^EXP-1 pre-registered`).MatchString(first) || !strings.Contains(first, "fd091c4f") {
		t.Errorf("first content line %q must match ^EXP-1 pre-registered and name commit fd091c4f", first)
	}
	need := []string{RequestState, fmt.Sprint(SampleSeed), fmt.Sprintf("%d KiB", MaxQuestionBytes/1024), fmt.Sprintf("$%g", DefaultCapUSD)}
	for _, tmpl := range Templates {
		need = append(need, tmpl.Question, tmpl.BaselineRule)
		need = append(need, tmpl.Baseline...)
	}
	for _, d := range FactDefinitions {
		need = append(need, d.Definition)
	}
	for _, q := range E1Quotas {
		need = append(need, fmt.Sprintf("%s: %d", q.Repo, q.N))
	}
	for _, b := range Bands {
		need = append(need, b.Name)
	}
	need = append(need, fmt.Sprintf("US$%g per million input tokens", ListedPrice.InPerMTok))
	// Every bar, copied verbatim from the plan's EXP-M1 to EXP-M4 rows.
	plan, err := os.ReadFile(filepath.Join("..", "..", "docs", "plans", "agentic-decisions.md"))
	if err != nil {
		t.Fatal(err)
	}
	bar := regexp.MustCompile(`(?m)^- \[[ x]\] \(op\) (EXP-M[1-4], .*?) \{id: EXP-M[1-4],`)
	bars := bar.FindAllStringSubmatch(string(plan), -1)
	if len(bars) != 4 {
		t.Fatalf("found %d of the plan's four EXP-M bars", len(bars))
	}
	for _, m := range bars {
		need = append(need, m[1])
	}
	for _, s := range need {
		if !strings.Contains(doc, s) {
			t.Errorf("the pre-registration does not carry, verbatim: %s", s)
		}
	}
}

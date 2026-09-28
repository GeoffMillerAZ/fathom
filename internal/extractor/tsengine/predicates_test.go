package tsengine

import (
	"context"
	"sort"
	"strings"
	"testing"
)

const predSrc = `def foo():
    pass

def bar():
    pass

def baz_test():
    pass

x = 1
y = y
`

// capturedTexts runs query over predSrc and returns the sorted text of every
// capture named capture.
func capturedTexts(t *testing.T, p *Parser, query, capture string) []string {
	t.Helper()
	q, err := p.CompileQuery(query)
	if err != nil {
		t.Fatalf("CompileQuery(%s): %v", query, err)
	}
	defer q.Close()
	tree, err := p.Parse([]byte(predSrc))
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	ms, err := q.Run(tree)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range ms {
		for _, c := range m.Captures {
			if c.Name == capture {
				out = append(out, c.Node.Text([]byte(predSrc)))
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestQueryTextPredicates (LB-F2, design §3.2): the six text predicates are
// evaluated host-side and drop failing matches; documentation directives
// are accepted and ignored; any other predicate fails compilation by name.
func TestQueryTextPredicates(t *testing.T) {
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	def := func(pred string) string {
		return `((function_definition name: (identifier) @n) ` + pred + `)`
	}
	cases := []struct {
		name, query, capture, want string
	}{
		{"eq", def(`(#eq? @n "foo")`), "n", "foo"},
		{"not-eq", def(`(#not-eq? @n "foo")`), "n", "bar,baz_test"},
		{"match", def(`(#match? @n "^ba")`), "n", "bar,baz_test"},
		{"not-match", def(`(#not-match? @n "_test$")`), "n", "bar,foo"},
		{"any-of", def(`(#any-of? @n "foo" "baz_test")`), "n", "baz_test,foo"},
		{"not-any-of", def(`(#not-any-of? @n "foo" "baz_test")`), "n", "bar"},
		{"regex with parens and brackets", def(`(#match? @n "^[a-z)(]+$")`), "n", "bar,foo"},
		{"capture eq capture", `((assignment left: (identifier) @l right: (identifier) @r) (#eq? @l @r))`, "l", "y"},
		{"two predicates both hold", def(`(#match? @n "^b") (#not-eq? @n "bar")`), "n", "baz_test"},
		{"predicate in a later pattern only",
			"; a comment with (#bogus? @n) inside\n[(pass_statement) (expression_statement)] @stmt\n" + def(`(#eq? @n "bar")`), "n", "bar"},
		{"earlier pattern unaffected",
			def(`(#eq? @n "bar")`) + "\n(function_definition name: (identifier) @all)", "all", "bar,baz_test,foo"},
		{"doc directives ignored",
			def(`(#set! doc "x") (#strip! @n "^f") (#select-adjacent! @n @n) (#offset! @n 0 1 0 -1)`), "n", "bar,baz_test,foo"},
		{"quantified capture: every node must pass",
			`((module (function_definition name: (identifier) @n)+) (#match? @n "^(foo|bar)$"))`, "n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(capturedTexts(t, p, c.query, c.capture), ",")
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}

	t.Run("set! is readable per pattern", func(t *testing.T) {
		q, err := p.CompileQuery("(identifier) @x\n" + def(`(#set! injection.language "python")`))
		if err != nil {
			t.Fatal(err)
		}
		defer q.Close()
		if got := q.PatternSetting(1, "injection.language"); got != "python" {
			t.Errorf("PatternSetting(1) = %q, want python", got)
		}
		if got := q.PatternSetting(0, "injection.language"); got != "" {
			t.Errorf("PatternSetting(0) = %q, want empty", got)
		}
	})

	for name, c := range map[string]struct{ query, want string }{
		"unknown predicate":    {def(`(#is? @n local)`), "#is?"},
		"unknown directive":    {def(`(#gsub! @n "a" "b")`), "#gsub!"},
		"bad regex":            {def(`(#match? @n "([")`), "#match?"},
		"eq arity":             {def(`(#eq? @n)`), "#eq?"},
		"any-of needs capture": {def(`(#any-of? "a" "b")`), "#any-of?"},
	} {
		t.Run("refused/"+name, func(t *testing.T) {
			_, err := p.CompileQuery(c.query)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one naming %q", err, c.want)
			}
		})
	}
}

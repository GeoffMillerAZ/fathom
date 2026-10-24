package extractor

import (
	"strings"
	"testing"
)

// The jinja2 dialect reads a template whose pinned grammar cannot parse as written
// by what the pre-parse step rewrites (tsPreprocessors["jinja-templates"], LB-Q-jinja2):
// a `# comment` line of the surrounding text (the grammar reads it as a line
// statement), and the `in` / `not in` operators of an expression (the grammar knows
// `in` only in a for loop's header). Definitions and calls are the same either way.
func TestJinjaTextsMendWhatTheGrammarCannotParse(t *testing.T) {
	e, err := NewTSExtractor("jinja2")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	cases := []struct {
		name, src string
		wantDefs  []string
	}{
		{"hash comment in the text", "# NOTE(x): shell comment\n{% block a %}{% endblock %}\n", []string{"JinjaBlock a"}},
		{"in operator in a ternary", "{% set cmd = 'a' if d in ['x', 'y'] else 'b' %}\n{% block b %}{% endblock %}\n", []string{"JinjaVariable cmd", "JinjaBlock b"}},
		{"not in operator", "{% if 'a' not in groups %}{% macro m() %}{% endmacro %}{% endif %}\n", []string{"JinjaMacro m"}},
		{"for header keeps its own in", "{% for h in groups['a'] if h in other %}{{ f(h) }}{% endfor %}\n{% block c %}{% endblock %}\n", []string{"JinjaBlock c"}},
	}
	for _, c := range cases {
		nodes, err := e.Extract("svc", "t.j2", []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []string
		for _, s := range nodes {
			if s.Kind != "JinjaTemplate" {
				got = append(got, string(s.Kind)+" "+s.Name)
			}
		}
		if strings.Join(got, ",") != strings.Join(c.wantDefs, ",") {
			t.Errorf("%s: defs %v, want %v", c.name, got, c.wantDefs)
		}
	}
}

// A file that parses as written is never rewritten, and the rewrite keeps every
// byte offset (a blanked byte is a space), so a definition's lines are the source's.
func TestJinjaTextsKeepLength(t *testing.T) {
	src := []byte("# NOTE: x\n{% set a = 'k' if x not in y else z %}\n{% for i in l if i in m %}{% endfor %}\n")
	out := jinjaTexts(src)
	if len(out) != 1 || len(out[0]) != len(src) {
		t.Fatalf("texts %d, want one of the source's length", len(out))
	}
	if strings.Contains(string(out[0]), "not in") || strings.HasPrefix(string(out[0]), "#") {
		t.Errorf("not rewritten: %q", out[0])
	}
	if !strings.Contains(string(out[0]), "for i in l") {
		t.Errorf("a for header's own in was rewritten: %q", out[0])
	}
	if jinjaTexts([]byte("{% block a %}{% endblock %}\n")) != nil {
		t.Errorf("a clean template gave a rewrite")
	}
}

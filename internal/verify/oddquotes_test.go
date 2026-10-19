package verify_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestPlanRowWithOddEscapedQuotesParses: an escaped quote inside a quoted
// value is data, whatever the count. The brace scanner used to pair every
// `"`, so a pattern carrying an odd count of `\"` left the scanner inside a
// quote at the closing brace: no block, a positional id, and nothing said.
// The four spellings are the record's: five and three escaped quotes (the
// two that did not parse), four (the one that did), and one, followed by a
// comma and a brace that the old pairing read as delimiters.
func TestPlanRowWithOddEscapedQuotesParses(t *testing.T) {
	cases := []struct {
		id, pattern string
	}{
		{"T-1", `\"L3\": \"excellent\"|\"L3 ceiling: `},
		{"T-2", `\"L3\": \"(excellent|ceiling)`},
		{"T-3", `\"L3\": \"excellent\"|L3 ceiling:`},
		{"T-4", `say \" then, {x}`},
	}
	var b strings.Builder
	b.WriteString("# p\n\n")
	for _, c := range cases {
		fmt.Fprintf(&b, "- [ ] a row {id: %s, oracle: lexical, path: r.json, pattern: \"%s\"}\n", c.id, c.pattern)
	}
	p := verify.ParseMarkdownPlan("svc", []byte(b.String()))
	if len(p.Tasks) != len(cases) {
		t.Fatalf("got %d tasks, want %d", len(p.Tasks), len(cases))
	}
	for i, c := range cases {
		got := p.Tasks[i]
		if got.ID != c.id {
			t.Errorf("row %d parsed as %q, want %q (title %q)", i+1, got.ID, c.id, got.Title)
			continue
		}
		if got.OraclePattern != c.pattern {
			t.Errorf("%s: pattern = %q, want %q as written", c.id, got.OraclePattern, c.pattern)
		}
		if got.OraclePath != "r.json" || got.Oracle != "lexical" {
			t.Errorf("%s: oracle %q path %q, want lexical r.json", c.id, got.Oracle, got.OraclePath)
		}
		if got.Title != "a row" {
			t.Errorf("%s: title %q, want the block stripped", c.id, got.Title)
		}
	}
	for _, w := range verify.LintPlan(p) {
		t.Errorf("a row that parses drew a lint line: %s", w)
	}
	// Unquoted, as internal/specdialect writes a value with no comma or
	// brace: an escaped quote opens no span there either.
	u := verify.ParseMarkdownPlan("svc", []byte("- [ ] u {id: T-5, oracle: lexical, path: r.json, pattern: \\\"L3\\\": \\\"x}\n"))
	if len(u.Tasks) != 1 || u.Tasks[0].ID != "T-5" || u.Tasks[0].OraclePattern != `\"L3\": \"x` {
		t.Errorf("unquoted value with escaped quotes: %+v", u.Tasks)
	}
}

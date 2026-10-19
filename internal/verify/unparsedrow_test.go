package verify_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestPlanLintNamesAnUnparsedRowBlock: a line carrying `{id:` that still
// does not parse as a row is an error the lint names, with its line and the
// reason, never a silent positional row.
func TestPlanLintNamesAnUnparsedRowBlock(t *testing.T) {
	// Lines 4, 5 and 7 carry a {id: block read as no row; line 3 parses,
	// line 9 names the form in a code span, line 11 is a fenced example.
	md := strings.Join([]string{
		"# p",
		"",
		"- [ ] fine {id: A-1, symbol: Foo}",
		`- [ ] open quote {id: A-2, oracle: lexical, path: x.md, pattern: "a}`,
		"- [ ] no close {id: A-3, symbol: Foo",
		"",
		"-[x] no space {id: A-4, symbol: Foo}",
		"",
		"Prose names the `{id: X}` form in a code span.",
		"```",
		"- {id: EX-1, symbol: Example}",
		"```",
		"",
	}, "\n")
	p := verify.ParseMarkdownPlan("svc", []byte(md))
	errs := map[int]string{}
	for _, u := range p.UnparsedRows {
		errs[u.Line] = u.Reason
	}
	for _, line := range []int{4, 5, 7} {
		if errs[line] == "" {
			t.Errorf("line %d: no unparsed-row record (got %v)", line, p.UnparsedRows)
		}
	}
	for _, line := range []int{3, 9, 11} {
		if r, ok := errs[line]; ok {
			t.Errorf("line %d recorded as unparsed (%s): a parsed row, a code span and a fenced example are not rows", line, r)
		}
	}
	if !strings.Contains(errs[4], "quote") {
		t.Errorf("line 4 reason %q does not name the open quote", errs[4])
	}
	if !strings.Contains(errs[5], "}") {
		t.Errorf("line 5 reason %q does not name the missing close brace", errs[5])
	}
	if !strings.Contains(errs[7], "checkbox") {
		t.Errorf("line 7 reason %q does not name the checkbox", errs[7])
	}

	lint := verify.LintPlan(p)
	for _, line := range []int{4, 5, 7} {
		want := fmt.Sprintf("line %d: error:", line)
		found := false
		for _, w := range lint {
			if strings.Contains(w, want) && strings.Contains(w, errs[line]) {
				found = true
			}
		}
		if !found {
			t.Errorf("no lint error names line %d with its reason; lint: %v", line, lint)
		}
	}
	if again := verify.LintPlan(verify.ParseMarkdownPlan("svc", []byte(md))); strings.Join(again, "\n") != strings.Join(lint, "\n") {
		t.Errorf("lint not deterministic:\n%v\n%v", lint, again)
	}
}

// TestRepoPlans_EveryIDBlockParsesAsARow holds the lint error at the gate:
// no top-level plan (the pre-push gate's scope, README.md excluded) carries
// a `{id:` line that does not parse as a row. Under `task validate`, this
// test is the part that fails; the verify CLI prints the same line.
func TestRepoPlans_EveryIDBlockParsesAsARow(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("docs/plans unreadable (%v) — an unread plan must not pass as a clean one", err)
	}
	var bad []string
	read := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			continue
		}
		md, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("docs/plans/%s unreadable (%v)", name, err)
		}
		read++
		for _, u := range verify.ParseMarkdownPlan("fathom", md).UnparsedRows {
			bad = append(bad, fmt.Sprintf("docs/plans/%s:%d %s", name, u.Line, u.Reason))
		}
	}
	if read == 0 {
		t.Fatal("no top-level plan was read — the check would pass vacuously")
	}
	if len(bad) > 0 {
		t.Errorf("%d {id: line(s) do not parse as a row:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestQuotedBracesDoNotTerminateAnnotation pins VS-1 (O4): a `{`/`}` inside a
// double-quoted annotation value must not open/terminate the `{…}` block,
// exactly as quoted commas already do not split values. This is O4's own
// two-line repro — a control row with no braces in its quoted value, and a
// test row whose skip: value contains a literal "{symbol, package}" — and
// both rows must keep their author-owned id and full key set (id, skip_test,
// skip), not degrade to a positional md-NNN id with a polluted title.
func TestQuotedBracesDoNotTerminateAnnotation(t *testing.T) {
	md := []byte(`- [x] control row no braces {id: R-1, skip_test: true, skip: "alpha, beta, gamma"}
- [x] test row with braces in quoted value {id: R-2, skip_test: true, skip: "alpha, {symbol, package} beta"}
`)
	p := verify.ParseMarkdownPlan("svc", md)
	if len(p.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2: %+v", len(p.Tasks), p.Tasks)
	}
	r1, r2 := p.Tasks[0], p.Tasks[1]

	if r1.ID != "R-1" {
		t.Errorf("control row lost its author-owned id: got %q, want R-1", r1.ID)
	}
	if !r1.Expect.SkipTest || r1.Expect.SkipReason != "alpha, beta, gamma" {
		t.Errorf("control row keys mangled: skip_test=%v skip=%q", r1.Expect.SkipTest, r1.Expect.SkipReason)
	}
	if strings.Contains(r1.Title, "{") || strings.Contains(r1.Title, "skip_test") {
		t.Errorf("control row annotation leaked into title: %q", r1.Title)
	}

	if r2.ID != "R-2" {
		t.Fatalf("test row lost its author-owned id (the O4 md-NNN downgrade): got %q, want R-2", r2.ID)
	}
	if !r2.Expect.SkipTest {
		t.Errorf("test row lost skip_test: %+v", r2.Expect)
	}
	if r2.Expect.SkipReason != `alpha, {symbol, package} beta` {
		t.Errorf("test row skip: value mangled, got %q", r2.Expect.SkipReason)
	}
	if strings.Contains(r2.Title, "{id:") || strings.Contains(r2.Title, "skip_test") {
		t.Errorf("test row annotation leaked into title: %q", r2.Title)
	}
	if r2.AnnotationSuspect != "" {
		t.Errorf("test row wrongly flagged AnnotationSuspect: %q", r2.AnnotationSuspect)
	}
}

// TestMalformedAnnotationLintsLoudly pins VS-2 (O4): a checkbox line whose
// `{…}` block the parser cannot fully consume becomes a LintPlan warning
// naming the row and the unparsed fragment, rather than silently degrading
// to an indistinguishable no-contract row. Parsing itself still degrades
// exactly as before (no behavior change in verify) — only the silence is
// fixed.
func TestMalformedAnnotationLintsLoudly(t *testing.T) {
	md := []byte("- [ ] Broken block {id T-9 symbol Foo}\n")
	p := verify.ParseMarkdownPlan("svc", md)
	if len(p.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %+v", len(p.Tasks), p.Tasks)
	}
	row := p.Tasks[0]
	if row.Expect.Symbol != "" || row.Expect.SkipTest {
		t.Fatalf("malformed block should not have parsed a contract: %+v", row.Expect)
	}

	warnings := verify.LintPlan(p)
	var hit string
	for _, w := range warnings {
		if strings.Contains(w, row.ID) && strings.Contains(w, "id T-9 symbol Foo") {
			hit = w
			break
		}
	}
	if hit == "" {
		t.Fatalf("no LintPlan warning names row %q and the unparsed fragment; warnings=%v", row.ID, warnings)
	}
}

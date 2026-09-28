package specdialect

import (
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// nativeDialect is plan grammar v2 itself: docs/plans/*.md checklists
// parsed by verify.ParseMarkdownPlan, unchanged (SI-2). It is the dispatch
// fallback for any Markdown file no foreign dialect claims.
type nativeDialect struct{}

// Native returns the canonical dialect.
func Native() SpecDialect { return nativeDialect{} }

func (nativeDialect) ID() string { return NativeID }

// Detect claims docs/plans/*.md (not README.md, not archive/) — the plans
// directory `fathom plans` already reads.
func (nativeDialect) Detect(root string, files []string) []string {
	return detectBy(root, files, func(p string) bool {
		segs := pathSegments(p)
		return len(segs) == 3 && segs[0] == "docs" && segs[1] == "plans" &&
			strings.HasSuffix(segs[2], ".md") && segs[2] != "README.md"
	}, nil)
}

// Claims holds for any .md file: the native grammar reads every Markdown
// checklist, which is exactly why it is consulted last.
func (nativeDialect) Claims(path string, _ []byte) bool {
	return strings.HasSuffix(strings.ToLower(path), ".md")
}

func (nativeDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	p, spans := nativeDialectParse(path, content)
	return p, spans, nil
}

// checkboxColRe finds a checklist row's opening bracket (group 1 is the
// indentation and bullet before it).
var checkboxColRe = regexp.MustCompile(`^(\s*[-*]\s+)\[`)

// nativeDialectParse is verify.ParseMarkdownPlan with zero output change —
// the plan is returned exactly as the native parser produced it, Service
// left empty for the caller to set — plus each task's checkbox position.
func nativeDialectParse(_ string, content []byte) (verify.Plan, []Span) {
	plan := verify.ParseMarkdownPlan("", content)
	lines := strings.Split(string(content), "\n")
	spans := make([]Span, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		col := 0
		if t.Line >= 1 && t.Line <= len(lines) {
			if m := checkboxColRe.FindStringSubmatch(lines[t.Line-1]); m != nil {
				col = len(m[1]) + 1
			}
		}
		spans = append(spans, Span{TaskID: t.ID, Line: t.Line, Col: col})
	}
	return plan, spans
}

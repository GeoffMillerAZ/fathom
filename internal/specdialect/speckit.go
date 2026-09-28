package specdialect

import (
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// specKitDialect reads GitHub Spec Kit's specs/<feature>/tasks.md in place
// (design §3): rows "- [ ] T001 [P] [US1] Create the model in
// src/models/user.py", done as [x] or [X]. The T### id is the task id;
// the [P] parallel marker and [USn]/[Story] labels are ignored; anchors
// come from an inline fathom annotation, else from the paths the task
// names (foreignTask).
type specKitDialect struct{}

// SpecKit returns the Spec Kit adapter.
func SpecKit() SpecDialect { return specKitDialect{} }

func (specKitDialect) ID() string { return "speckit" }

// specKitIDRe is a Spec Kit task id at the start of a row's text.
var specKitIDRe = regexp.MustCompile(`^(T\d{3,})\b[:.]?\s*`)

// specKitMarkerRe is a Spec Kit bracket label: [P] (parallel), [US1] /
// [US-1] (user story), [Story].
var specKitMarkerRe = regexp.MustCompile(`^\[(?:P|US-?\d+|Story)\]\s*`)

func specKitPath(p string) bool { return hasSegmentsSuffix(p, "specs", "*", "tasks.md") }

// specKitShape reports whether content carries at least one T### row.
func specKitShape(content []byte) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if m := checkboxLine.FindStringSubmatch(line); m != nil && specKitIDRe.MatchString(m[3]) {
			return true
		}
	}
	return false
}

func (specKitDialect) Detect(root string, files []string) []string {
	return detectBy(root, files, specKitPath, specKitShape)
}

func (specKitDialect) Claims(path string, content []byte) bool {
	return specKitPath(path) && specKitShape(content)
}

func (specKitDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	p, spans := specKitParse(path, content)
	return p, spans, nil
}

// specKitParse maps a Spec Kit tasks.md into the canonical model. Only
// rows carrying a T### id are tasks; any other checkbox (a checklist in a
// preamble) is not the scaffold's task unit and is left out. Pure.
func specKitParse(_ string, content []byte) (verify.Plan, []Span) {
	var (
		plan  verify.Plan
		spans []Span
	)
	for i, line := range strings.Split(string(content), "\n") {
		m := checkboxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		idm := specKitIDRe.FindStringSubmatch(m[3])
		if idm == nil {
			continue
		}
		body := m[3][len(idm[0]):]
		for {
			mk := specKitMarkerRe.FindString(body)
			if mk == "" {
				break
			}
			body = body[len(mk):]
		}
		t := foreignTask(idm[1], body, m[2] == "x" || m[2] == "X", i+1)
		plan.Tasks = append(plan.Tasks, t)
		spans = append(spans, Span{TaskID: t.ID, Line: i + 1, Col: len(m[1]) + 1})
	}
	return plan, spans
}

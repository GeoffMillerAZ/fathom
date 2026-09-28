package specdialect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// kiroDialect reads Kiro's .kiro/specs/<name>/tasks.md in place (design
// §3): numbered checkbox rows ("- [ ] 1. Set up the project", "- [x] 2.
// Add retry"), done as [x]. Each task's own requirement links sit on the
// next line(s) up to the following checkbox row, in Kiro's own
// "_Requirements: 1.1, 2.3_" shape; the referenced ids ("<requirement
// number>.<acceptance criterion number>") resolve against the spec's own
// requirements.md, written in EARS ("## Requirement N" sections, a
// numbered "Acceptance Criteria" list under each). Anchors come from an
// inline fathom annotation, else from the paths the task names, exactly
// like Spec Kit and OpenSpec; a resolved requirement's WHEN/THEN/SHALL
// text becomes accept: when the task carries no accept: of its own — the
// same non-gating pattern OpenSpec's spec-delta linking uses.
type kiroDialect struct{}

// Kiro returns the Kiro adapter.
func Kiro() SpecDialect { return kiroDialect{} }

func (kiroDialect) ID() string { return "kiro" }

func kiroPath(p string) bool { return hasSegmentsSuffix(p, ".kiro", "specs", "*", "tasks.md") }

// kiroShape reports whether content carries at least one checkbox row —
// the path alone (".kiro/specs/<name>/tasks.md") is already distinctive
// enough that, like OpenSpec, no stronger content shape is required.
func kiroShape(content []byte) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if checkboxLine.MatchString(line) {
			return true
		}
	}
	return false
}

func (kiroDialect) Detect(root string, files []string) []string {
	return detectBy(root, files, kiroPath, kiroShape)
}

func (kiroDialect) Claims(path string, content []byte) bool {
	return kiroPath(path) && kiroShape(content)
}

func (kiroDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	p, spans := kiroParse(path, content, nil)
	return p, spans, nil
}

// SiblingPaths names the spec's own requirements.md, when present — the
// one file a Kiro tasks.md resolves _Requirements:_ links against.
// Bounded to the one spec directory (design §7).
func (kiroDialect) SiblingPaths(file string) []string {
	req := filepath.Join(filepath.Dir(file), "requirements.md")
	if _, err := os.Stat(req); err != nil {
		return nil
	}
	return []string{req}
}

func (kiroDialect) ParseWithSiblings(path string, content []byte, siblings map[string][]byte) (verify.Plan, []Span, error) {
	var reqBody []byte
	for _, b := range siblings { // at most one: requirements.md
		reqBody = b
	}
	p, spans := kiroParse(path, content, reqBody)
	return p, spans, nil
}

// kiroTaskNumRe is a Kiro task's leading number at the start of a row's
// text: "1", "1.1", "2.3.1", optionally followed by a dot — the same
// dotted-numeric shape OpenSpec's own tasks use.
var kiroTaskNumRe = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s+`)

// kiroReqsLineRe matches a task's "_Requirements: 1.1, 2.3_" line: an
// optional bullet marker, optional surrounding underscores (Kiro's own
// italic convention), the label, then a comma-separated id list. Group 1
// is the id list, case-insensitive on the label.
var kiroReqsLineRe = regexp.MustCompile(`(?i)^\s*(?:[-*]\s+)?_?Requirements?:\s*([0-9][0-9.,\s]*)_?\s*$`)

// kiroReqHeadingRe is a requirements.md "## Requirement N" heading (any
// heading depth). Group 1 is the requirement number.
var kiroReqHeadingRe = regexp.MustCompile(`(?i)^#{1,6}\s+Requirement\s+(\d+)\b`)

// kiroCriterionRe is one numbered acceptance-criterion line under a
// requirement heading ("1. WHEN ... THE SYSTEM SHALL ..."). Group 1 is
// the criterion number, group 2 its text.
var kiroCriterionRe = regexp.MustCompile(`^\s*(\d+)\.\s+(.*\S)\s*$`)

// kiroRequirements reads a requirements.md's EARS acceptance criteria, in
// document order: each numbered item under a "## Requirement N" heading
// becomes a Requirement whose ID is "N.M" (requirement number, criterion
// number) and whose Text is the criterion's own line. A criterion line
// before any requirement heading is ignored (there is no N to key it
// under). Pure.
func kiroRequirements(body []byte) []Requirement {
	var (
		out    []Requirement
		reqNum string
	)
	for _, line := range strings.Split(string(body), "\n") {
		if m := kiroReqHeadingRe.FindStringSubmatch(line); m != nil {
			reqNum = m[1]
			continue
		}
		if reqNum == "" {
			continue
		}
		if m := kiroCriterionRe.FindStringSubmatch(line); m != nil {
			out = append(out, Requirement{ID: reqNum + "." + m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return out
}

// kiroTaskRequirementRefs finds the requirement ids the task starting at
// checkbox line i (0-based into lines) names on its own "_Requirements:_"
// line — the first such line found before the next checkbox row or EOF.
func kiroTaskRequirementRefs(lines []string, i int) []string {
	for j := i + 1; j < len(lines); j++ {
		if checkboxLine.MatchString(lines[j]) {
			return nil
		}
		m := kiroReqsLineRe.FindStringSubmatch(lines[j])
		if m == nil {
			continue
		}
		var refs []string
		for _, ref := range strings.Split(m[1], ",") {
			if ref = strings.TrimSpace(ref); ref != "" {
				refs = append(refs, ref)
			}
		}
		return refs
	}
	return nil
}

// kiroParse maps a Kiro tasks.md into the canonical model. reqBody is the
// spec's requirements.md content (nil: no requirement linking, exactly
// the pure Parse path). A task whose _Requirements:_ line names one or
// more ids resolved in reqBody, and that carries no accept: of its own,
// gets every resolved requirement's "<id>: <criterion text>" joined with
// "; " as accept:; an id that does not resolve is kept verbatim, so a
// stale reference stays visible rather than silently vanishing.
// Unnumbered checkbox rows get a positional id ("t-001"). Pure.
func kiroParse(_ string, content []byte, reqBody []byte) (verify.Plan, []Span) {
	var reqs []Requirement
	if reqBody != nil {
		reqs = kiroRequirements(reqBody)
	}
	reqByID := map[string]Requirement{}
	for _, r := range reqs {
		reqByID[r.ID] = r
	}

	lines := strings.Split(string(content), "\n")
	var (
		plan  verify.Plan
		spans []Span
		n     int
	)
	for i, line := range lines {
		m := checkboxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		id, body := fmt.Sprintf("t-%03d", n), m[3]
		if nm := kiroTaskNumRe.FindStringSubmatch(body); nm != nil {
			id, body = nm[1], body[len(nm[0]):]
		}
		t := foreignTask(id, body, m[2] == "x" || m[2] == "X", i+1)
		// Linking requires an actual requirements.md to resolve against
		// (reqBody != nil): the pure Parse path (no siblings read) links
		// nothing, exactly like OpenSpec's own spec-delta linking.
		if t.Accept == "" && reqBody != nil {
			var parts []string
			for _, ref := range kiroTaskRequirementRefs(lines, i) {
				if r, ok := reqByID[ref]; ok {
					parts = append(parts, r.ID+": "+r.Text)
				} else {
					parts = append(parts, ref)
				}
			}
			t.Accept = strings.Join(parts, "; ")
		}
		plan.Tasks = append(plan.Tasks, t)
		spans = append(spans, Span{TaskID: t.ID, Line: i + 1, Col: len(m[1]) + 1})
	}
	return plan, spans
}

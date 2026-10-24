package specdialect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// openSpecDialect reads OpenSpec's openspec/changes/<id>/tasks.md in place
// (design §3): numbered checkbox rows ("- [ ] 1.1 Add the retry policy"),
// done as [x]. Anchors come from an inline fathom annotation, else from
// the paths the task names; requirement ids come from the change's spec
// deltas (openspec/changes/<id>/specs/**/spec.md), and a task that names a
// requirement carries its SHALL text as accept: — shown, never gating.
// Archived changes (openspec/changes/archive/…) are not detected.
type openSpecDialect struct{}

// OpenSpec returns the OpenSpec adapter.
func OpenSpec() SpecDialect { return openSpecDialect{} }

func (openSpecDialect) ID() string { return "openspec" }

// openSpecNumRe is an OpenSpec task number at the start of a row's text:
// "1", "1.1", "2.3.1", optionally followed by a dot.
var openSpecNumRe = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s+`)

func openSpecPath(p string) bool {
	return hasSegmentsSuffix(p, "openspec", "changes", "*", "tasks.md") &&
		pathSegments(p)[len(pathSegments(p))-2] != "archive"
}

// openSpecShape reports whether content carries at least one checkbox row.
func openSpecShape(content []byte) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if checkboxLine.MatchString(line) {
			return true
		}
	}
	return false
}

func (openSpecDialect) Detect(root string, files []string) []string {
	return detectBy(root, files, openSpecPath, openSpecShape)
}

func (openSpecDialect) Claims(path string, content []byte) bool {
	return openSpecPath(path) && openSpecShape(content)
}

func (openSpecDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	p, spans := openSpecParse(path, content, nil)
	return p, spans, nil
}

// SiblingPaths lists the change's spec deltas — every spec.md under
// <change>/specs, file being the resolved tasks.md path — for ParseFile to
// read. Bounded to the one change directory (design §7: detection and
// parsing never walk the repository).
func (openSpecDialect) SiblingPaths(file string) []string {
	specs := filepath.Join(filepath.Dir(file), "specs")
	var out []string
	_ = filepath.WalkDir(specs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == "spec.md" {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (openSpecDialect) ParseWithSiblings(path string, content []byte, siblings map[string][]byte) (verify.Plan, []Span, error) {
	p, spans := openSpecParse(path, content, siblings)
	return p, spans, nil
}

// Requirement is one requirement declared in an OpenSpec spec delta.
type Requirement struct {
	// ID is "<capability>/<name>": the capability directory the delta sits
	// in, then the requirement's own heading text.
	ID string
	// Op is the delta section it appears under: ADDED, MODIFIED, REMOVED
	// or RENAMED ("" outside any delta section).
	Op string
	// Name is the "### Requirement: <name>" heading text.
	Name string
	// Text is the requirement's first SHALL/MUST sentence line.
	Text string
}

var (
	openSpecDeltaRe = regexp.MustCompile(`^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\s+Requirements\b`)
	openSpecReqRe   = regexp.MustCompile(`^###\s+Requirement:\s*(.+?)\s*$`)
)

// openSpecRequirements reads one spec delta's requirements, in document
// order. capability names the delta (its specs/<capability>/ directory).
func openSpecRequirements(capability string, delta []byte) []Requirement {
	var (
		out []Requirement
		op  string
		cur *Requirement
	)
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(string(delta), "\n") {
		if m := openSpecDeltaRe.FindStringSubmatch(line); m != nil {
			flush()
			op = m[1]
			continue
		}
		if m := openSpecReqRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Requirement{ID: capability + "/" + m[1], Op: op, Name: m[1]}
			continue
		}
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "####") {
			flush()
			continue
		}
		if cur != nil && cur.Text == "" {
			if s := strings.TrimSpace(line); s != "" && (strings.Contains(s, "SHALL") || strings.Contains(s, "MUST")) {
				cur.Text = s
			}
		}
	}
	flush()
	return out
}

// deltaCapability is the capability directory a spec delta path sits in
// ("…/specs/<capability>/spec.md"), or the file's parent directory name.
func deltaCapability(p string) string {
	segs := pathSegments(p)
	if len(segs) >= 2 {
		return segs[len(segs)-2]
	}
	return p
}

// openSpecParse maps an OpenSpec tasks.md into the canonical model. deltas
// maps each spec-delta path to its content (nil: no requirement linking).
// A task whose text names a requirement (case-insensitively) and carries
// no accept: of its own gets "<id>: <SHALL text>" as accept:. Unnumbered
// checkbox rows get a positional id ("t-001"). Pure.
func openSpecParse(_ string, content []byte, deltas map[string][]byte) (verify.Plan, []Span) {
	var reqs []Requirement
	paths := make([]string, 0, len(deltas))
	for p := range deltas {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		reqs = append(reqs, openSpecRequirements(deltaCapability(p), deltas[p])...)
	}

	var (
		plan  verify.Plan
		spans []Span
		n     int
	)
	for i, line := range strings.Split(string(content), "\n") {
		m := checkboxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		id, body := fmt.Sprintf("t-%03d", n), m[3]
		if nm := openSpecNumRe.FindStringSubmatch(body); nm != nil {
			id, body = nm[1], body[len(nm[0]):]
		}
		t := foreignTask(id, body, m[2] == "x" || m[2] == "X", i+1)
		if t.Accept == "" {
			lower := strings.ToLower(t.Title)
			for _, r := range reqs {
				if r.Op != "REMOVED" && strings.Contains(lower, strings.ToLower(r.Name)) {
					t.Accept = r.ID
					if r.Text != "" {
						t.Accept += ": " + r.Text
					}
					break
				}
			}
		}
		plan.Tasks = append(plan.Tasks, t)
		spans = append(spans, Span{TaskID: t.ID, Line: i + 1, Col: len(m[1]) + 1})
	}
	return plan, spans
}

// siblingParser is implemented by a dialect whose tasks resolve against
// other files in the same spec directory (OpenSpec's spec deltas).
type siblingParser interface {
	SiblingPaths(file string) []string
	ParseWithSiblings(path string, content []byte, siblings map[string][]byte) (verify.Plan, []Span, error)
}

// ParseFile reads path (relative to root, or absolute) and parses it with
// d (ParseContent). Returns the plan, spans and the file's bytes as read —
// the caller compares them after a run to prove the file was not touched.
func ParseFile(d SpecDialect, root, path string) (verify.Plan, []Span, []byte, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, path)
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return verify.Plan{}, nil, nil, err
	}
	plan, spans, err := parseContent(d, root, abs, content)
	return plan, spans, content, err
}

// ParseContent parses content (the bytes of the file at file, a path the
// process can open) with d, handing a sibling-aware dialect the sibling
// files it names — and only those. For Spec Kit it also reads the `_test.go`
// files a task names, under the repository root.
func ParseContent(d SpecDialect, file string, content []byte) (verify.Plan, []Span, error) {
	return parseContent(d, "", file, content)
}

func parseContent(d SpecDialect, root, file string, content []byte) (verify.Plan, []Span, error) {
	if sp, ok := d.(siblingParser); ok {
		sib := map[string][]byte{}
		for _, p := range sp.SiblingPaths(file) {
			if b, rerr := os.ReadFile(p); rerr == nil {
				sib[p] = b
			}
		}
		plan, spans, err := sp.ParseWithSiblings(file, content, sib)
		if err == nil {
			plan = foreignPlanTestRefs(d, root, file, content, plan)
		}
		return plan, spans, err
	}
	plan, spans, err := d.Parse(file, content)
	if err == nil {
		plan = foreignPlanTestRefs(d, root, file, content, plan)
	}
	return plan, spans, err
}

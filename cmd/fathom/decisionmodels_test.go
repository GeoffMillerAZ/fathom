package main

// decisionmodels_test.go — DP-41 (ADR-073 decision 12): every design weighs
// the decision port. Every record numbered ADR-071 or higher in ADR.md
// carries a `**Decision models.**` paragraph (or a `### Decision models`
// subsection under its own `## ADR-NNN` heading), and every
// docs/design/*.md not on the committed grandfather list carries a
// `## Decision models` heading (a numbered `## 21. Decision models` counts).
// The section names the typed questions the design could ask through the
// hexagonal port, the deterministic path with provider none, the opt-out,
// and the measurement that proves or kills each, or says in one line why
// none applies; this test checks only that the section is there.
//
// The grandfather list holds the design docs that predate the rule, sorted,
// generated once on 2026-10-16. It only shrinks: a listed file that is gone
// or that now carries the heading is reported, so the list cannot hide a
// regression or go stale. A heading inside a fenced code block never counts.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/adr"
)

// decisionModelsFirstADR is the first record the rule binds: ADR-073 made
// it, and ADR-071 and ADR-072, written the same day, were brought under it.
const decisionModelsFirstADR = 71

// decisionModelsGrandfathered is the committed list, relative to the repo root.
const decisionModelsGrandfathered = "cmd/fathom/testdata/decisionmodels/grandfathered.txt"

var (
	designDecisionModelsRe = regexp.MustCompile(`^##\s+(?:\d+(?:\.\d+)*\.?\s+)?Decision models\b`)
	adrDecisionModelsRe    = regexp.MustCompile(`^(?:\*\*Decision models[.:]?\*\*|###\s+(?:\d+(?:\.\d+)*\.?\s+)?Decision models\b)`)
	adrNumberRe            = regexp.MustCompile(`^ADR-(\d+)$`)
)

// hasLineOutsideFences reports whether any line outside a fenced code block
// matches re.
func hasLineOutsideFences(body string, re *regexp.Regexp) bool {
	inFence := false
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		ln := strings.TrimRight(sc.Text(), " \t\r")
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && re.MatchString(ln) {
			return true
		}
	}
	return false
}

// readGrandfathered reads the list: one base name per line, `#` comments and
// blank lines ignored, order kept so the sort check can see it.
func readGrandfathered(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		out = append(out, ln)
	}
	return out, nil
}

// decisionModelsGaps returns every place the rule is broken, sorted. design
// maps a docs/design base name to its content; designDir and listPath only
// label the messages.
func decisionModelsGaps(adrLog []byte, design map[string][]byte, grandfathered []string, designDir, listPath string) []string {
	var gaps []string
	for _, sec := range adr.ParseSections(adrLog) {
		m := adrNumberRe.FindStringSubmatch(sec.ID)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n < decisionModelsFirstADR {
			continue
		}
		if !hasLineOutsideFences(sec.Body, adrDecisionModelsRe) {
			gaps = append(gaps, fmt.Sprintf("ADR.md:%d: %s has no **Decision models.** paragraph (or ### Decision models subsection)", sec.Line, sec.ID))
		}
	}
	listed := map[string]bool{}
	if !sort.StringsAreSorted(grandfathered) {
		gaps = append(gaps, listPath+": not sorted")
	}
	for _, name := range grandfathered {
		if listed[name] {
			gaps = append(gaps, fmt.Sprintf("%s: %s listed twice", listPath, name))
		}
		listed[name] = true
		body, ok := design[name]
		switch {
		case !ok:
			gaps = append(gaps, fmt.Sprintf("%s: %s is not in %s; remove it from the list", listPath, name, designDir))
		case hasLineOutsideFences(string(body), designDecisionModelsRe):
			gaps = append(gaps, fmt.Sprintf("%s: %s now carries ## Decision models; remove it from the list", listPath, name))
		}
	}
	for name, body := range design {
		if listed[name] {
			continue
		}
		if !hasLineOutsideFences(string(body), designDecisionModelsRe) {
			gaps = append(gaps, fmt.Sprintf("%s/%s has no ## Decision models section (ADR-073 decision 12; docs/AGENTS.md)", designDir, name))
		}
	}
	sort.Strings(gaps)
	return gaps
}

// readDesignDocs reads every top-level *.md in dir, keyed by base name.
func readDesignDocs(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = b
	}
	return out
}

// TestEveryNewRecordHasADecisionModelsSection is DP-41 on the real tree.
func TestEveryNewRecordHasADecisionModelsSection(t *testing.T) {
	root := repoRootForDocs(t)
	adrLog, err := os.ReadFile(filepath.Join(root, "ADR.md"))
	if err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(root, filepath.FromSlash(decisionModelsGrandfathered))
	grandfathered, err := readGrandfathered(listPath)
	if err != nil {
		t.Fatal(err)
	}
	design := readDesignDocs(t, filepath.Join(root, "docs", "design"))
	if len(design) == 0 {
		t.Fatal("no docs/design/*.md read: the check would pass vacuously")
	}
	gaps := decisionModelsGaps(adrLog, design, grandfathered, "docs/design", decisionModelsGrandfathered)
	for _, g := range gaps {
		t.Error(g)
	}
	if len(gaps) > 0 {
		t.Log("a new ADR or design doc carries a decision-models section: the typed questions it could ask through the port, the deterministic path with provider none, the opt-out, and the measurement that proves or kills each, or one line saying why none applies (ADR-073 decision 12)")
	}
}

// TestDecisionModelsCheckFlagsTheFixture proves the check is red where it
// must be: a record past the threshold that names the section only inline or
// in a fence, a design doc without the heading or with it only in a fence,
// and a grandfather list holding a gone file and a file that now has it.
func TestDecisionModelsCheckFlagsTheFixture(t *testing.T) {
	dir := filepath.Join("testdata", "decisionmodels", "fixture")
	adrLog, err := os.ReadFile(filepath.Join(dir, "ADR.md"))
	if err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(dir, "grandfathered.txt")
	grandfathered, err := readGrandfathered(listPath)
	if err != nil {
		t.Fatal(err)
	}
	design := readDesignDocs(t, filepath.Join(dir, "design"))
	got := decisionModelsGaps(adrLog, design, grandfathered, "design", "grandfathered.txt")
	want := []string{
		"ADR.md:21: ADR-073 has no **Decision models.** paragraph (or ### Decision models subsection)",
		"design/fenced.md has no ## Decision models section (ADR-073 decision 12; docs/AGENTS.md)",
		"design/without.md has no ## Decision models section (ADR-073 decision 12; docs/AGENTS.md)",
		"grandfathered.txt: gone.md is not in design; remove it from the list",
		"grandfathered.txt: with.md now carries ## Decision models; remove it from the list",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("gaps =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// An unsorted or duplicated list is reported too.
	bad := decisionModelsGaps(nil, map[string][]byte{"a.md": nil, "b.md": nil}, []string{"b.md", "a.md", "a.md"}, "design", "list")
	if !containsLine(bad, "list: not sorted") || !containsLine(bad, "list: a.md listed twice") {
		t.Errorf("unsorted, duplicated list not reported: %q", bad)
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

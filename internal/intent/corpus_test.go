package intent_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found")
	return ""
}

// TestLoadCorpusGoldenThreeRepos (EP-4) parses the real corpus. A parser
// that works only on a fixture is the healthy-empty failure this repository
// has recorded four times, so the assertions are about THIS repo's actual
// docs — and every count is a floor, not an equality, so ordinary editing
// does not break the test while a shape change still does.
func TestLoadCorpusGoldenThreeRepos(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Tier != intent.TierRouter {
		t.Fatalf("tier = %q, want router (docs/intent/README.md exists here)", c.Tier)
	}
	if c.RouterPath != "docs/intent/README.md" {
		t.Errorf("router = %q", c.RouterPath)
	}
	if c.DecisionsHome == "" {
		t.Error("decisions home unresolved — the router names ADR.md")
	}

	if len(c.Invariants) < 30 {
		t.Errorf("parsed %d invariant rows, want ≥30 — the table shape changed", len(c.Invariants))
	}
	if len(c.Surfaces) < 10 {
		t.Errorf("parsed %d sensitive surfaces, want ≥10", len(c.Surfaces))
	}
	if len(c.Conditions) < 40 {
		t.Errorf("parsed %d conditions, want ≥40", len(c.Conditions))
	}

	// Every invariant classifies onto the ladder, and the ladder is used:
	// a corpus where everything lands on one rung means the classifier is
	// not reading the cells.
	rungs := map[intent.Rung]int{}
	for _, inv := range c.Invariants {
		if inv.ID == "" || inv.Statement == "" {
			t.Errorf("invariant at %s:%d has no id or statement", inv.File, inv.Line)
		}
		rungs[inv.Rung]++
	}
	if len(rungs) < 3 {
		t.Errorf("all invariants landed on %d rung(s): %v — the classifier is not reading the cells", len(rungs), rungs)
	}
	if rungs[intent.RungGate] == 0 || rungs[intent.RungTest] == 0 {
		t.Errorf("expected both gate- and test-rung invariants here, got %v", rungs)
	}

	// Ids are record-scoped and unique.
	seen := map[string]string{}
	for _, cond := range c.Conditions {
		if cond.ID == "" || !strings.Contains(cond.ID, "#") {
			t.Errorf("condition at %s:%d has no record-scoped id (%q)", cond.File, cond.Line, cond.ID)
		}
		if prev, dup := seen[cond.ID]; dup {
			t.Errorf("duplicate condition id %q (%s:%d and %s)", cond.ID, cond.File, cond.Line, prev)
		}
		seen[cond.ID] = cond.File
		if cond.Label == "" {
			t.Errorf("condition %s has no label", cond.ID)
		}
	}

	// Nothing is annotated yet — K0's demand signal has not formed, and the
	// audit must say so rather than inventing verdicts.
	for _, cond := range c.Conditions {
		if cond.Annotated() {
			t.Logf("annotated condition present: %s (%s:%d)", cond.ID, cond.File, cond.Line)
		}
	}

	// Terminal conditions are recognised, so "Revisit when: never" cannot
	// inflate a fired/undecided denominator.
	terminals := 0
	for _, cond := range c.Conditions {
		if cond.Terminal {
			terminals++
		}
	}
	if terminals == 0 {
		t.Error(`no terminal conditions found — this repo writes "Revisit when: never" and "none identified"`)
	}
	t.Logf("corpus: %d conditions (%d terminal), %d invariants %v, %d surfaces, %d unparsed, decisions=%s",
		len(c.Conditions), terminals, len(c.Invariants), rungs, len(c.Surfaces), len(c.Unparsed), c.DecisionsHome)
	for _, u := range c.Unparsed {
		t.Logf("unparsed: %s:%d %s", u.File, u.Line, u.Reason)
	}
}

// TestConditionsMatchRouterGrep (EP-5): the router documents ONE way to
// enumerate every condition — a grep. If the parser and that grep disagree,
// the parser is wrong (the practice's registry is the grep), so the gap must
// be named rather than silently absorbed.
func TestConditionsMatchRouterGrep(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available")
	}
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cmd := exec.Command("grep", "-rn", "-E",
		"(Revisit when|Promote when|Drop if|Done when)", "docs/intent", "ADR.md")
	cmd.Dir = root // the router's grep is documented relative to the repo root
	out, _ := cmd.Output()
	grepLines := map[string]bool{}
	lineRe := regexp.MustCompile(`^([^:]+):(\d+):`)
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if m := lineRe.FindStringSubmatch(ln); m != nil {
			// The router's own grep line documents the labels; it is not a
			// condition.
			if strings.Contains(ln, "grep -rn") {
				continue
			}
			grepLines[m[1]+":"+m[2]] = true
		}
	}
	if len(grepLines) < 40 {
		t.Fatalf("the router's grep found only %d condition lines — it must be run from the repo root, or the corpus moved", len(grepLines))
	}

	parsed := map[string]bool{}
	for _, cond := range c.Conditions {
		parsed[cond.File+":"+itoa(cond.Line)] = true
	}

	// A grep cannot tell a condition from prose ABOUT conditions: the
	// router's own instructions name all four labels, the decisions index
	// has a "Revisit when" column header, and a question asks about a
	// `Promote when`. Every such line must be EXPLAINED — parsed as a
	// condition, or recorded with the reason it is not one. An unexplained
	// difference is a parser bug; a silent one is the failure mode this
	// package exists to prevent.
	explained := map[string]bool{}
	for _, ex := range c.Excluded {
		explained[ex.File+":"+itoa(ex.Line)] = true
	}
	for _, u := range c.Unparsed {
		explained[u.File+":"+itoa(u.Line)] = true
	}

	var unexplained []string
	for loc := range grepLines {
		if !parsed[loc] && !explained[loc] {
			unexplained = append(unexplained, loc)
		}
	}
	sort.Strings(unexplained)
	if len(unexplained) > 0 {
		t.Errorf("the router's grep finds %d line(s) the parser neither parsed nor explained: %v",
			len(unexplained), unexplained)
	}
	t.Logf("grep %d lines; parser %d conditions, %d explained exclusions",
		len(grepLines), len(c.Conditions), len(c.Excluded))
}

func itoa(i int) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.Replace(" "+itoaRaw(i), " ", "", 1)), ""))
}

func itoaRaw(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// TestDecisionsOnlyTierListsTrailers (EP-36): a repo with decision records
// and NO router still gets something back — its `Revisit when` trailers are
// exactly the conditions worth listing. Nine repos in this fleet are in that
// tier, and vision 23 promised them an `adr audit` with no router
// precondition.
func TestDecisionsOnlyTierListsTrailers(t *testing.T) {
	root := t.TempDir()
	body := `# Decisions

## ADR-001 — Pick a thing

**Decision.** Picked.

**Revisit when.** A second consumer appears.

## ADR-002 — Pick another

**Revisit when.** The first one is dropped.
`
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := intent.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Tier != intent.TierDecisionsOnly {
		t.Fatalf("tier = %q, want decisions-only (records but no router)", c.Tier)
	}
	if c.RouterPath != "" {
		t.Errorf("no router exists; RouterPath = %q", c.RouterPath)
	}
	if c.DecisionsHome != "ADR.md" {
		t.Errorf("decisions home = %q, want ADR.md", c.DecisionsHome)
	}
	if len(c.Conditions) != 2 {
		t.Fatalf("expected both trailers listed, got %d: %+v", len(c.Conditions), c.Conditions)
	}
	for _, cond := range c.Conditions {
		if cond.Record == "" || !strings.HasPrefix(cond.Record, "ADR-") {
			t.Errorf("a trailer must be attributed to its record, got %q", cond.Record)
		}
		if cond.Label != "Revisit when" {
			t.Errorf("label = %q", cond.Label)
		}
	}

	// A repo with neither is tier none, and says so rather than erroring.
	empty, err := intent.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on an empty repo must not error: %v", err)
	}
	if empty.Tier != intent.TierNone {
		t.Errorf("tier = %q, want none", empty.Tier)
	}
}

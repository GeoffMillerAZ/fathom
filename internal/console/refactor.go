package console

// refactor.go — the refactor page's projection (OC-26, vision 35 §4.7,
// design §3 "Refactor"): the conservation planes of `fathom conserve
// --since` as verdict chips, and the reading set — the changed functions
// only a human can vouch for — with the lines of each partial item that no
// test ran.
//
// ADR-044's rule carries over unchanged: a plane that did not run for the
// range reads "not run", never "conserved"; a plane with a missing input
// reads "unproven" and names it (I44).

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ADR44Planes is ADR-044 §0's plane set: what a refactor must conserve.
// The page shows every one of them, run or not.
var ADR44Planes = []string{"absence", "certificates", "generated", "governance", "integrity", "resurrection", "surface"}

// ConserveFinding is one finding a plane reported.
type ConserveFinding struct {
	Plane    string `json:"plane"`
	Kind     string `json:"kind"`
	Key      string `json:"key"`
	Detail   string `json:"detail"`
	Declared string `json:"declared"`
	Remedy   string `json:"remedy,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	// Informational findings are shown for visibility and never make
	// their plane a loss (conserve's own rule: an edited test, say).
	Informational bool `json:"informational,omitempty"`
}

// ConservePlane is one plane's report.
type ConservePlane struct {
	Plane    string            `json:"plane"`
	Verdict  string            `json:"verdict"`
	Missing  []string          `json:"missing"`
	Findings []ConserveFinding `json:"findings"`
}

// ResidueItem is one changed function in the reading set.
type ResidueItem struct {
	Symbol          string   `json:"symbol"`
	File            string   `json:"file"`
	Line            int      `json:"line"`
	Change          string   `json:"change"`
	Class           string   `json:"class"`
	Tests           []string `json:"tests"`
	Callers         int      `json:"callers"`
	Evidence        string   `json:"evidence"`
	UnexecutedLines []string `json:"unexecuted_lines,omitempty"`
}

// Residue is the reading set's counts and items.
type Residue struct {
	Changed             int `json:"changed"`
	Attested            int `json:"attested"`
	Unattested          int `json:"unattested"`
	Unreached           int `json:"unreached"`
	Partial             int `json:"partial"`
	Unexecuted          int `json:"unexecuted"`
	UnanalyzedFunctions int `json:"unanalyzed_functions"`
	// ClassedByExecution counts the changed functions an execution-evidence
	// profile classed; Partial and Unexecuted exist only among them.
	ClassedByExecution int           `json:"classed_by_execution"`
	Items              []ResidueItem `json:"items"`
	Truncated          int           `json:"truncated"`
	Unanalyzed         []string      `json:"unanalyzed"`
	Missing            []string      `json:"missing"`
	CoverageUnusable   string        `json:"coverage_unusable,omitempty"`
}

// RefactorInput is what the refactor page reads for one service and range.
type RefactorInput struct {
	Service string
	Since   string
	// Ran is true when a conservation report exists for the range; Err
	// names why it could not be made; Pending, that it is being made.
	Ran     bool
	Err     string
	Pending bool
	Base    string
	Head    string
	// Registered are the planes this build runs.
	Registered []string
	Planes     []ConservePlane
	Residue    *Residue
	// Sources holds the head text of each partial item's file, for the
	// excerpt of its unexecuted lines; a file missing here is named.
	Sources map[string]string
	// ExcerptContext is how many lines around the unexecuted ranges show.
	ExcerptContext int
}

// PlaneChip is one plane's verdict chip.
type PlaneChip struct {
	Plane    string   `json:"plane"`
	Verdict  string   `json:"verdict"` // conserved | losses | unproven | not run
	Role     string   `json:"role"`
	Findings int      `json:"findings"`
	Losses   int      `json:"losses"`
	Missing  []string `json:"missing,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// ResidueItemView is a reading-set item, with its excerpt when partial.
type ResidueItemView struct {
	ResidueItem
	Excerpt       []ExcerptLine `json:"excerpt,omitempty"`
	ExcerptReason string        `json:"excerpt_reason,omitempty"`
}

// ResidueView is the reading set as the page shows it.
type ResidueView struct {
	State      State             `json:"state"`
	Reason     string            `json:"reason,omitempty"`
	Changed    Value[int]        `json:"changed"`
	Attested   Value[int]        `json:"attested"`
	Partial    Value[int]        `json:"partial"`
	Unexecuted Value[int]        `json:"unexecuted"`
	Unattested Value[int]        `json:"unattested"`
	Unreached  Value[int]        `json:"unreached"`
	Items      []ResidueItemView `json:"items"`
	Truncated  int               `json:"truncated"`
	Unanalyzed []string          `json:"unanalyzed"`
	Missing    []string          `json:"missing"`
	Coverage   string            `json:"coverage_unusable,omitempty"`
}

// RefactorPage is the refactor page.
type RefactorPage struct {
	Service  string            `json:"service"`
	Since    string            `json:"since"`
	Base     string            `json:"base,omitempty"`
	Head     string            `json:"head,omitempty"`
	State    State             `json:"state"`
	Reason   string            `json:"reason,omitempty"`
	Planes   []PlaneChip       `json:"planes"`
	Findings []ConserveFinding `json:"findings"`
	Residue  ResidueView       `json:"residue"`
}

// RefactorView projects a conservation report for one range (OC-26). Every
// ADR-044 plane and every registered plane gets a chip; one the report
// does not carry reads "not run" with the reason, never "conserved".
func RefactorView(in RefactorInput) RefactorPage {
	p := RefactorPage{Service: in.Service, Since: in.Since, Base: shortCommit(in.Base), Head: in.Head, Findings: []ConserveFinding{}}
	names := map[string]bool{}
	for _, n := range ADR44Planes {
		names[n] = true
	}
	registered := map[string]bool{}
	for _, n := range in.Registered {
		names[n], registered[n] = true, true
	}
	reported := map[string]ConservePlane{}
	for _, pl := range in.Planes {
		names[pl.Plane] = true
		reported[pl.Plane] = pl
	}
	order := make([]string, 0, len(names))
	for n := range names {
		order = append(order, n)
	}
	sort.Strings(order)

	notRun := ""
	switch {
	case in.Err != "":
		p.State, p.Reason = StateError, "the conservation report could not be made: "+in.Err
		notRun = "the report could not be made for this range"
	case in.Pending:
		p.State, p.Reason = StateAbsent, "the conservation report for this range is still being computed — reload in a few seconds"
		notRun = "still being computed for this range"
	case !in.Ran:
		p.State, p.Reason = StateAbsent, "fathom conserve has not run for this range"
		notRun = "not run for this range"
	default:
		p.State = StatePresent
	}
	for _, n := range order {
		chip := PlaneChip{Plane: n}
		pl, ok := reported[n]
		switch {
		case !ok && in.Ran && !registered[n]:
			chip.Verdict, chip.Role = "not run", "ink-muted"
			chip.Reason = "no plane by this name is registered in this build, so nothing checked it"
		case !ok && in.Ran:
			chip.Verdict, chip.Role = "not run", "ink-muted"
			chip.Reason = "the report for this range does not carry it"
		case !ok:
			chip.Verdict, chip.Role, chip.Reason = "not run", "ink-muted", notRun
		default:
			chip.Verdict = pl.Verdict
			chip.Findings = len(pl.Findings)
			chip.Missing = pl.Missing
			for _, f := range pl.Findings {
				if f.Declared != "prior" && !f.Informational {
					chip.Losses++
				}
				p.Findings = append(p.Findings, f)
			}
			switch pl.Verdict {
			case "conserved":
				chip.Role = "positive"
				if chip.Findings > 0 {
					chip.Reason = fmt.Sprintf("nothing lost; %s shown for visibility", countNoun(chip.Findings, "finding"))
				}
			case "losses":
				chip.Role = "danger"
				chip.Reason = fmt.Sprintf("%s not excused by a prior declaration", countNoun(chip.Losses, "finding"))
			case "unproven":
				chip.Role = "caution"
				chip.Reason = "missing: " + strings.Join(pl.Missing, "; ")
			default:
				chip.Role = "ink-muted"
			}
		}
		p.Planes = append(p.Planes, chip)
	}
	p.Residue = residueView(in, notRun)
	return p
}

func residueView(in RefactorInput, notRun string) ResidueView {
	v := ResidueView{Items: []ResidueItemView{}, Unanalyzed: []string{}, Missing: []string{}}
	absent := func(reason string) {
		v.State, v.Reason = StateAbsent, reason
		for _, x := range []*Value[int]{&v.Changed, &v.Attested, &v.Partial, &v.Unexecuted, &v.Unattested, &v.Unreached} {
			*x = Absent[int](reason)
		}
	}
	switch {
	case in.Err != "" || !in.Ran || in.Pending:
		absent("the reading set is " + notRun)
		return v
	case in.Residue == nil:
		absent("the report carries no reading set")
		return v
	}
	r := in.Residue
	const source = "fathom conserve --json · residue"
	if len(r.Missing) > 0 {
		// I44: with a missing input the reading set classes nothing; the
		// changed functions are still counted.
		reason := "the reading set classes nothing while an input is missing (I44): " + strings.Join(r.Missing, "; ")
		v.State, v.Reason = StateAbsent, reason
		v.Changed = Measured(r.Changed, source).WithDenominator("changed functions in the range")
		for _, x := range []*Value[int]{&v.Attested, &v.Partial, &v.Unexecuted, &v.Unattested, &v.Unreached} {
			*x = Absent[int](reason).WithSource(source)
		}
	} else {
		v.State = StatePresent
		of := fmt.Sprintf("of %d changed functions", r.Changed)
		v.Changed = Measured(r.Changed, source).WithDenominator("changed functions in the range")
		v.Attested = Measured(r.Attested, source).WithDenominator(of)
		v.Unattested = Measured(r.Unattested, source).WithDenominator(of)
		v.Unreached = Measured(r.Unreached, source).WithDenominator(of)
		v.Partial, v.Unexecuted = executionClasses(r, source, of)
	}
	v.Truncated, v.Coverage = r.Truncated, r.CoverageUnusable
	v.Unanalyzed = append(v.Unanalyzed, r.Unanalyzed...)
	v.Missing = append(v.Missing, r.Missing...)
	ctx := in.ExcerptContext
	if ctx <= 0 {
		ctx = 2
	}
	for _, it := range r.Items {
		iv := ResidueItemView{ResidueItem: it}
		if it.Class == "partial" {
			iv.Excerpt, iv.ExcerptReason = unexecutedExcerpt(it, in.Sources, ctx)
		}
		v.Items = append(v.Items, iv)
	}
	return v
}

// executionClasses are the partial and unexecuted counts with their truth:
// both exist only where execution evidence classed a function (design
// §3.5), so with none they are not measured — a 0 there is conserve's
// "nothing classed", never a count — and with some functions classed by
// static reach instead they are a lower bound naming the split.
func executionClasses(r *Residue, source, of string) (partial, unexecuted Value[int]) {
	analyzed := r.Changed - r.UnanalyzedFunctions
	switch {
	case analyzed <= 0:
		// No changed function to class: zero is a count, not an absence.
	case r.ClassedByExecution == 0:
		why := "no coverage profile was loaded for this range"
		if r.CoverageUnusable != "" {
			why = "the coverage profile could not be used: " + r.CoverageUnusable
		}
		reason := "no execution evidence classed any changed function (" + why + "), so which changed lines ran is not measured; the classes stand on static reach"
		return Absent[int](reason).WithSource(source), Absent[int](reason).WithSource(source)
	case r.ClassedByExecution < analyzed:
		reason := fmt.Sprintf("%d of %d changed functions were classed by execution evidence, the rest by static reach — a lower bound", r.ClassedByExecution, analyzed)
		return Partial(r.Partial, reason).WithSource(source).WithDenominator(of), Partial(r.Unexecuted, reason).WithSource(source).WithDenominator(of)
	}
	return Measured(r.Partial, source).WithDenominator(of), Measured(r.Unexecuted, source).WithDenominator(of)
}

// maxExcerptLines bounds one excerpt: every line number is a keyboard stop.
const maxExcerptLines = 60

// unexecutedExcerpt is the partial item's function from its first line to
// its last unexecuted line (plus context), each unexecuted line marked. A
// function longer than the bound shows the unexecuted ranges with context.
func unexecutedExcerpt(it ResidueItem, sources map[string]string, ctx int) ([]ExcerptLine, string) {
	text, ok := sources[it.File]
	if !ok {
		return nil, it.File + " was not read at head, so its lines cannot be shown"
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	unexec := map[int]bool{}
	last := 0
	for _, rng := range it.UnexecutedLines {
		lo, hi, ok := parseLineRange(rng)
		if !ok {
			continue
		}
		for n := lo; n <= hi; n++ {
			unexec[n] = true
		}
		if hi > last {
			last = hi
		}
	}
	if len(unexec) == 0 {
		return nil, "the item names no unexecuted line range"
	}
	if last > len(lines) {
		return nil, fmt.Sprintf("%s has %d lines at head, but the report names line %d — the file changed since the report", it.File, len(lines), last)
	}
	start := it.Line
	if start < 1 || start > last {
		start = 1
	}
	end := last + ctx
	if end > len(lines) {
		end = len(lines)
	}
	keep := func(n int) bool { return true }
	if end-start+1 > maxExcerptLines {
		keep = func(n int) bool {
			for d := -ctx; d <= ctx; d++ {
				if unexec[n+d] {
					return true
				}
			}
			return n == start
		}
	}
	var out []ExcerptLine
	for n := start; n <= end && len(out) < maxExcerptLines; n++ {
		if keep(n) {
			out = append(out, ExcerptLine{Number: n, Text: lines[n-1], Unexecuted: unexec[n]})
		}
	}
	return out, ""
}

// parseLineRange reads "start-end" (or a single "n").
func parseLineRange(s string) (int, int, bool) {
	a, b, found := strings.Cut(strings.TrimSpace(s), "-")
	lo, err := strconv.Atoi(a)
	if err != nil || lo < 1 {
		return 0, 0, false
	}
	if !found {
		return lo, lo, true
	}
	hi, err := strconv.Atoi(b)
	if err != nil || hi < lo {
		return 0, 0, false
	}
	return lo, hi, true
}

package readout

// testdelta.go — RO-23 (docs/plans/readouts.md phase 2, design
// docs/design/readouts.md): rich mode's test delta and flake flag. A failing
// run's line compares it with the session's previous test-run rows (TR-7
// with RO-35's `head`, `tree` and `scope` keys): tests fixed, still failing
// (since which HEAD), new, and `not run` for a previous failure this run
// never ran. A test that failed and passed at the same HEAD and tree is a
// possible flake. Every doubt reads `not compared` or stays silent, never a
// wrong label: a test is `fixed` only when this run is known to have run
// it, and `new` only when the compared row ran the same command shape and
// logged every failure. Pure and deterministic.

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// TestRunRow is one previous test-run row of the session, as the hook log
// carries it, in log order.
type TestRunRow struct {
	Invocation string
	Result     string   // pass | fail | unknown
	Failing    []string // the logged failing ids
	// Capped is true when the failing list hit the row's id cap, so a
	// failure past it is not in Failing.
	Capped bool
	// Partial is true when piping or truncation may have hidden failures.
	Partial bool
	// Head, Tree and Scope are RO-35's keys; Head and Tree are "" when the
	// row carries none. Scope is "" for a command with no package pattern
	// or filter (RO-35 writes no key for an empty scope).
	Head, Tree, Scope string
}

// TestRunPackage is one package's final result in this run.
type TestRunPackage struct {
	Name     string
	Status   string // the testrun status: ok, fail, build-failed, no-tests, skip
	Panicked bool   // a test in it panicked, so later tests may never have run
}

// TestRunNow is the failing run the line is for.
type TestRunNow struct {
	Invocation string
	Failing    []string         // every failing leaf test id, in output order
	Packages   []TestRunPackage // the packages the output shows a final result for
	// Complete is true when the output shows every package the run reached
	// (parsed complete, neither piped nor truncated), so a package it does
	// not show was outside the run's scope.
	Complete bool
	// FiltersKnown is true when Scope's run= and skip= are every test
	// filter the command carries (a readable command passing no arguments
	// on to the test binary).
	FiltersKnown bool
	// FailFast is true when a failure stops the rest of its package.
	FailFast          bool
	Head, Tree, Scope string
}

// The delta's outcomes, as the run row logs them.
const (
	DeltaNone        = "none"         // legacy rows with no previous run
	DeltaNotCompared = "not-compared" // no eligible baseline or missing tree keys
	DeltaCompared    = "compared"     // a clause naming at least one test
	DeltaSilent      = "silent"       // nothing could be named
)

// testDeltaShown caps the names one bucket shows before `+N more`.
const testDeltaShown = 3

// testDeltaNameCap bounds one shown name in bytes.
const testDeltaNameCap = 80

// RunDelta returns the delta clause for a failing run and its outcome
// (renderTestDelta). prev is the session's previous rows in log order.
func RunDelta(prev []TestRunRow, now TestRunNow) (clause, outcome string) {
	return renderTestDelta(prev, now)
}

// ranVerdict is whether this run ran a previous failure.
type ranVerdict int

const (
	ranUnsure ranVerdict = iota // the output cannot say
	ranYes                      // in scope, not filtered out, its package ran clean of build errors
	ranNo                       // outside this run's scope, filtered out, or its package never built
)

// renderTestDelta compares now with the session's last decided, non-partial
// row of the same command shape (invocation and scope). Without one, only
// the logged outcome names the missing comparison.
func renderTestDelta(prev []TestRunRow, now TestRunNow) (string, string) {
	bi := -1
	for i := len(prev) - 1; i >= 0; i-- {
		r := prev[i]
		if sameShape(r, now.Invocation, now.Scope) && !r.Partial && (r.Result == "pass" || r.Result == "fail") {
			bi = i
			break
		}
	}
	if bi < 0 {
		return "", DeltaNotCompared
	}
	base := prev[bi]
	lead := "delta vs the last run"
	if base.Capped {
		lead += " (of the first 10 it logged)"
	}
	if now.Head == "" || now.Tree == "" || base.Head == "" || base.Tree == "" {
		return "", DeltaNotCompared
	}
	sameTree := base.Head == now.Head && base.Tree == now.Tree
	failingNow := setOf(now.Failing)
	failedBefore := setOf(base.Failing)
	// a pass of this command shape at this head and tree: a test failing
	// now passed with nothing changed
	passedHere := false
	for _, r := range prev {
		if r.Result == "pass" && !r.Partial && sameShape(r, now.Invocation, now.Scope) && r.Head == now.Head && r.Tree == now.Tree {
			passedHere = true
			break
		}
	}
	var fixed, still, fresh, notRun []string
	unsure := 0
	for _, id := range base.Failing {
		if failingNow[id] {
			since, earlier := sinceHead(prev, bi, id)
			note := "since " + shortSHA(since)
			if earlier {
				note += " or earlier"
			}
			if passedHere {
				note += ", possible flake"
			}
			still = append(still, deltaName(id, now.Packages)+" ("+note+")")
			continue
		}
		switch ranInThisRun(id, now) {
		case ranYes:
			name := deltaName(id, now.Packages)
			if sameTree {
				name += " (possible flake)"
			}
			fixed = append(fixed, name)
		case ranNo:
			notRun = append(notRun, deltaName(id, now.Packages))
		default:
			unsure++
		}
	}
	for _, id := range now.Failing {
		if failedBefore[id] {
			continue
		}
		if base.Capped {
			unsure++ // the compared row may have omitted it past the cap
			continue
		}
		name := deltaName(id, now.Packages)
		if passedHere {
			name += " (possible flake)"
		}
		fresh = append(fresh, name)
	}
	if len(fixed)+len(still)+len(fresh)+len(notRun) == 0 {
		return "", DeltaSilent
	}
	var parts []string
	for _, b := range []struct {
		label string
		names []string
	}{{"fixed", fixed}, {"still failing", still}, {"new", fresh}, {"not run", notRun}} {
		if len(b.names) > 0 {
			parts = append(parts, b.label+" "+shownNames(b.names))
		}
	}
	if unsure > 0 {
		parts = append(parts, strconv.Itoa(unsure)+" not compared")
	}
	return lead + ": " + strings.Join(parts, "; "), DeltaCompared
}

// sameShape reports whether r ran the same command shape: the same
// declared invocation and the same lexed scope.
func sameShape(r TestRunRow, invocation, scope string) bool {
	return r.Invocation == invocation && r.Scope == scope
}

// sinceHead walks back from prev[bi] through the rows of its command shape
// while each lists id failing, and returns the head the streak began at.
// earlier is true when the row that ends the walk cannot say the test
// passed (its list was capped, its result unknown, or it carries no head).
func sinceHead(prev []TestRunRow, bi int, id string) (since string, earlier bool) {
	base := prev[bi]
	since = base.Head
	for i := bi - 1; i >= 0; i-- {
		r := prev[i]
		if !sameShape(r, base.Invocation, base.Scope) {
			continue
		}
		if setOf(r.Failing)[id] {
			if r.Head == "" {
				return since, true
			}
			since = r.Head
			continue
		}
		return since, r.Capped || r.Partial || (r.Result != "pass" && r.Result != "fail")
	}
	return since, false
}

// ranInThisRun decides whether now ran the previous failure id: its
// package shows a final result, no filter excludes it, it built, and no
// panic or -failfast may have stopped the package before it.
func ranInThisRun(id string, now TestRunNow) ranVerdict {
	pkg, ok := packageOf(id, now.Packages)
	if !ok {
		if now.Complete {
			return ranNo
		}
		return ranUnsure
	}
	switch pkg.Status {
	case "ok", "fail":
	case "build-failed", "no-tests", "skip":
		return ranNo
	default:
		return ranUnsure
	}
	if !now.FiltersKnown {
		return ranUnsure
	}
	name := strings.TrimPrefix(id, pkg.Name+".")
	run, skip := scopeFilters(now.Scope)
	if run != "" {
		m, ok := goTestFilterMatch(run, name, false)
		if !ok {
			return ranUnsure
		}
		if !m {
			return ranNo
		}
	}
	if skip != "" {
		m, ok := goTestFilterMatch(skip, name, true)
		if !ok {
			return ranUnsure
		}
		if m {
			return ranNo
		}
	}
	if pkg.Status == "fail" && (pkg.Panicked || now.FailFast) {
		return ranUnsure
	}
	return ranYes
}

// packageOf finds the package id belongs to: the longest package name
// that prefixes it as "<package>.".
func packageOf(id string, pkgs []TestRunPackage) (TestRunPackage, bool) {
	var best TestRunPackage
	found := false
	for _, p := range pkgs {
		if p.Name != "" && strings.HasPrefix(id, p.Name+".") && len(p.Name) > len(best.Name) {
			best, found = p, true
		}
	}
	return best, found
}

// scopeFilters reads RO-35's scope text (`<patterns> run=<re> skip=<re>`)
// back into its -run and -skip values.
func scopeFilters(scope string) (run, skip string) {
	var cur *string
	for _, f := range strings.Split(scope, " ") {
		switch {
		case strings.HasPrefix(f, "run="):
			run, cur = strings.TrimPrefix(f, "run="), &run
		case strings.HasPrefix(f, "skip="):
			skip, cur = strings.TrimPrefix(f, "skip="), &skip
		case cur != nil:
			*cur += " " + f // a filter value with a space in it
		}
	}
	return run, skip
}

// goTestFilterMatch applies Go's -run (skip false) or -skip (skip true)
// rule to a test name: the pattern splits into alternatives at top-level
// `|` and each into levels at top-level `/`, matched unanchored against
// the name's `/` levels. A -run alternative matches when every level both
// carry matches; a -skip one only when the name has at least its levels.
// ok is false when a level does not compile.
func goTestFilterMatch(pattern, name string, skip bool) (match, ok bool) {
	levels := strings.Split(name, "/")
	for _, alt := range splitTopLevel(pattern, '|') {
		pats := splitTopLevel(alt, '/')
		all := true
		for i, p := range pats {
			if i >= len(levels) {
				break
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return false, false
			}
			if !re.MatchString(levels[i]) {
				all = false
				break
			}
		}
		if all && (!skip || len(levels) >= len(pats)) {
			return true, true
		}
	}
	return false, true
}

// splitTopLevel splits s at sep outside brackets and parentheses, as Go's
// testing package splits a -run pattern.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	cs, cp, start := 0, 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			cs++
		case ']':
			if cs > 0 {
				cs--
			}
		case '(':
			if cs == 0 {
				cp++
			}
		case ')':
			if cs == 0 && cp > 0 {
				cp--
			}
		case '\\':
			i++
		default:
			if s[i] == sep && cs == 0 && cp == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// deltaName shows a test id as its package's last element and the test
// (money.TestRound), the package found among this run's packages, else at
// the id's first `.Test`, `.Example`, `.Benchmark` or `.Fuzz`, else the id.
func deltaName(id string, pkgs []TestRunPackage) string {
	pkg, test := "", id
	if p, ok := packageOf(id, pkgs); ok {
		pkg, test = p.Name, strings.TrimPrefix(id, p.Name+".")
	} else {
		for _, kind := range []string{".Test", ".Example", ".Benchmark", ".Fuzz"} {
			if i := strings.Index(id, kind); i > 0 {
				pkg, test = id[:i], id[i+1:]
				break
			}
		}
	}
	name := test
	if pkg != "" {
		name = path.Base(pkg) + "." + test
	}
	if len(name) > testDeltaNameCap {
		n := testDeltaNameCap
		for n > 0 && name[n]&0xC0 == 0x80 {
			n-- // never split a UTF-8 sequence
		}
		name = name[:n] + "…"
	}
	return name
}

// shownNames joins at most testDeltaShown names, then `+N more`.
func shownNames(names []string) string {
	if len(names) <= testDeltaShown {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:testDeltaShown], ", ") + fmt.Sprintf(", +%d more", len(names)-testDeltaShown)
}

// shortSHA is a HEAD's first seven characters.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func setOf(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

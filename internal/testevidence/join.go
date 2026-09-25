package testevidence

// join.go — JoinCasesToSymbols (TC-3): the case-to-symbol join (design doc
// §2). A case's name is joined to a Test* symbol in the same service by
// EXACT bare-function-name match; "TestX/sub" joins to the enclosing
// "TestX". A case that joins nowhere — no Test* symbol has that name — is
// KEPT with an empty TestSymbolKey and counted in JoinStats.Total but not
// JoinStats.Joined: the join rate is a reported fact, never a pass/fail,
// and an ambiguous name is never guessed at.
//
// CV-72 (docs/design/refactor-conservation.md §3.5, CV-M13's own control
// measurement: "34 of 566 cases" joined to neither of two same-named
// packages): a bare name shared by Test* funcs in more than one package
// used to stay unjoined outright, even when the CASE ITSELF names which
// package it ran in. A go-test-json-derived case id carries its package's
// full Go import path before the bare test name — surveyor's own
// convention, e.g. "github.com/geoffmilleraz/conjure/internal/canon.
// TestDeleteJSONC" — so an otherwise-ambiguous bare name is disambiguated
// by checking which candidate's own directory the case's import path ends
// in. Still never guessed: a case id that names no matching candidate
// directory (or matches more than one — two candidates sharing a
// directory, from a duplicate/stale index row) stays unjoined exactly as
// before.
import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// JoinedCase pairs one ingested case with the Test* symbol key it joined
// to — "" when unjoined.
type JoinedCase struct {
	Case
	SuiteName     string
	TestSymbolKey string
}

// JoinStats is the reported join fact `fathom tests status` renders as
// "joined N of M cases".
type JoinStats struct {
	Joined int
	Total  int
}

// symbolCandidate is one Test* symbol sharing a bare name with at least
// one other — CV-72's own disambiguation record: the symbol's key plus
// the repo-relative directory its file lives in (path.Dir(FilePath)),
// the same directory a case's own Go import path is checked against.
type symbolCandidate struct {
	key string
	dir string
}

// JoinCasesToSymbols joins cases (from one suite, named suiteName) against
// symbols — every service's indexed Test* function. A bare name matching
// exactly one Test* symbol joins to it outright. A bare name shared by two
// or more (e.g. two packages' own TestMain) is disambiguated by the case's
// OWN id (CV-72, disambiguateByPackage) when exactly one candidate's
// directory matches; otherwise it is treated as unjoined rather than
// arbitrarily picking one: exactness is the whole contract (design doc
// §2's "exact name").
func JoinCasesToSymbols(cases []Case, suiteName string, symbols []core.SymbolNode) ([]JoinedCase, JoinStats) {
	byName := make(map[string][]symbolCandidate, len(symbols))
	for _, s := range symbols {
		if s.Kind != core.KindGoFunc || !strings.HasPrefix(s.Name, "Test") {
			continue
		}
		byName[s.Name] = append(byName[s.Name], symbolCandidate{key: s.Key(), dir: dirOf(s.FilePath)})
	}

	out := make([]JoinedCase, 0, len(cases))
	stats := JoinStats{Total: len(cases)}
	for _, c := range cases {
		candidates := byName[topLevelTestName(c.Name)]
		key := ""
		switch len(candidates) {
		case 0:
			key = ""
		case 1:
			key = candidates[0].key
		default:
			key = disambiguateByPackage(c.ID, candidates)
		}
		if key != "" {
			stats.Joined++
		}
		out = append(out, JoinedCase{Case: c, SuiteName: suiteName, TestSymbolKey: key})
	}
	return out, stats
}

// topLevelTestName returns the enclosing Test* function name a case name
// joins to: "TestX/sub/sub2" -> "TestX"; "TestX" -> "TestX".
func topLevelTestName(caseName string) string {
	if i := strings.IndexByte(caseName, '/'); i >= 0 {
		return caseName[:i]
	}
	return caseName
}

// disambiguateByPackage picks the one candidate (of several sharing a bare
// Test* name) whose own directory the case id's package portion — its Go
// import path, everything before the LAST "." (a bare id with no "."
// names no package at all and never matches) — ends in: an exact
// directory match, or a "/"+dir suffix match against the import path's
// trailing segments (a directory "internal/canon" matches an import path
// "github.com/geoffmilleraz/conjure/internal/canon" without ever needing
// to resolve the case's own module path). Never guesses: zero or more
// than one candidate matching returns "" (still unjoined), exactly like
// an ordinary unresolved ambiguity.
func disambiguateByPackage(caseID string, candidates []symbolCandidate) string {
	pkgPath := packageFromCaseID(caseID)
	if pkgPath == "" {
		return ""
	}
	match, matches := "", 0
	for _, cand := range candidates {
		if cand.dir == "" {
			continue
		}
		if pkgPath == cand.dir || strings.HasSuffix(pkgPath, "/"+cand.dir) {
			matches++
			match = cand.key
		}
	}
	if matches == 1 {
		return match
	}
	return ""
}

// packageFromCaseID returns the Go import path a case id names before its
// trailing ".<TestFunc>" — surveyor's go-test-json id convention (CV-M13's
// own control data, control/r1/envelope.json: "github.com/geoffmilleraz/
// conjure/internal/canon.TestDeleteJSONC"). "" when id carries no "." at
// all (an id shaped some other way — never guessed at).
func packageFromCaseID(id string) string {
	if i := strings.LastIndexByte(id, '.'); i >= 0 {
		return id[:i]
	}
	return ""
}

// dirOf returns filePath's directory in forward-slash, repo-relative
// form ("internal/canon/x.go" -> "internal/canon"), "" for a bare
// filename with no directory component. Deliberately re-derived here
// (rather than importing path.Dir directly at each call site) so a
// future change to "what a candidate's own directory means" has one
// place to land.
func dirOf(filePath string) string {
	if i := strings.LastIndexByte(filePath, '/'); i >= 0 {
		return filePath[:i]
	}
	return ""
}

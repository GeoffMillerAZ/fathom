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
			key = disambiguateByPackage(c.ID, c.Name, candidates)
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
// import path, everything the id carries BEFORE its own case name (never
// just "the last dot": a subtest name commonly contains one itself, e.g.
// "TestX/v1.2" or a table test keyed by a file path, CV-72/defect
// evidence) — ends in: an exact directory match, or, when no root-
// package candidate (dir == "") shares this bare name, a "/"+dir suffix
// match against the import path's trailing segments (a directory
// "internal/canon" matches an import path "github.com/geoffmilleraz/
// conjure/internal/canon" without ever needing to resolve the case's own
// module path). A root candidate makes the suffix form irreducibly
// ambiguous: this file has no go.mod, so it cannot tell "example.com/
// tools" (a ROOT package's own import path, which IS its module path)
// ending in "/tools" from a genuine "tools/" subdirectory candidate's
// import path doing the same — both readings are equally consistent
// with the bare id string alone. Rather than pick one (a real risk: the
// pre-fix behaviour always silently picked the subdirectory, sending a
// root-package case to the WRONG symbol whenever a subdirectory
// happened to share the module's own last path segment), a non-root
// candidate matches only EXACTLY when a root candidate is present in the
// same set — never by suffix — and an unresolved root ambiguity stays
// unjoined, exactly like an ordinary unresolved ambiguity: exactness is
// the whole contract (design doc §2's "exact name"), and "never guessed"
// means never guessed in either direction.
func disambiguateByPackage(caseID, caseName string, candidates []symbolCandidate) string {
	pkgPath := packageFromCaseID(caseID, caseName)
	if pkgPath == "" {
		return ""
	}
	hasRoot := false
	for _, cand := range candidates {
		if cand.dir == "" {
			hasRoot = true
			break
		}
	}
	// The root candidate's own import path IS the module path itself,
	// which this file has no go.mod to resolve, so it is never matched
	// here at all — only a non-root candidate is, and only by an EXACT
	// directory match once hasRoot makes the suffix form ambiguous.
	match, matches := "", 0
	for _, cand := range candidates {
		if cand.dir == "" {
			continue
		}
		exact := pkgPath == cand.dir
		suffix := !hasRoot && strings.HasSuffix(pkgPath, "/"+cand.dir)
		if exact || suffix {
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
// own trailing ".<c.Name>" — surveyor's go-test-json id convention
// (CV-M13's own control data, control/r1/envelope.json:
// "github.com/geoffmilleraz/conjure/internal/canon.TestDeleteJSONC"), cut
// by the case's OWN name rather than at id's last "." — a subtest name
// routinely contains a "." of its own (a version string "v1.2", a table
// test keyed by a file path "internal/b.go", both real shapes in this
// repository's own suite), and cutting at id's LAST "." then lands
// inside the subtest name, either matching no candidate at all or
// suffix-matching the WRONG one. Falls back to name's own top-level
// Test* function when id does not end in "."+name (a caller-supplied id
// shaped some other way); "" when neither suffix is found at all —
// never guessed at.
func packageFromCaseID(id, name string) string {
	if pkg, ok := strings.CutSuffix(id, "."+name); ok {
		return pkg
	}
	if top := topLevelTestName(name); top != name {
		if pkg, ok := strings.CutSuffix(id, "."+top); ok {
			return pkg
		}
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

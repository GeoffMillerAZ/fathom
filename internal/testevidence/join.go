package testevidence

// join.go — JoinCasesToSymbols (TC-3): the case-to-symbol join (design doc
// §2). A case's name is joined to a Test* symbol in the same service by
// EXACT bare-function-name match; "TestX/sub" joins to the enclosing
// "TestX". A case that joins nowhere — no Test* symbol has that name, or
// (a bare name shared by two same-named Test funcs in different packages)
// more than one does — is KEPT with an empty TestSymbolKey and counted in
// JoinStats.Total but not JoinStats.Joined: the join rate is a reported
// fact, never a pass/fail, and an ambiguous name is never guessed at.

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

// JoinCasesToSymbols joins cases (from one suite, named suiteName) against
// symbols — every service's indexed Test* function. Ambiguous names (two
// or more Test* funcs sharing a bare name, e.g. two packages' own
// TestMain) are treated as unjoined rather than arbitrarily picking one:
// exactness is the whole contract (design doc §2's "exact name").
func JoinCasesToSymbols(cases []Case, suiteName string, symbols []core.SymbolNode) ([]JoinedCase, JoinStats) {
	byName := make(map[string]string, len(symbols)) // bare Test* func name -> symbol key, "" once ambiguous
	seen := make(map[string]bool, len(symbols))
	for _, s := range symbols {
		if s.Kind != core.KindGoFunc || !strings.HasPrefix(s.Name, "Test") {
			continue
		}
		if seen[s.Name] {
			byName[s.Name] = "" // ambiguous: two Test* funcs share this bare name
			continue
		}
		seen[s.Name] = true
		byName[s.Name] = s.Key()
	}

	out := make([]JoinedCase, 0, len(cases))
	stats := JoinStats{Total: len(cases)}
	for _, c := range cases {
		key := byName[topLevelTestName(c.Name)]
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

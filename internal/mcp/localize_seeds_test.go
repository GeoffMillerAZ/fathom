package mcp

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestClampLocalizeBudget(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, localizeDefaultBudget},
		{-5, localizeDefaultBudget},
		{1, 1},
		{12, 12},
		{40, 40},
		{41, localizeMaxBudget},
		{1000, localizeMaxBudget},
	}
	for _, c := range cases {
		if got := clampLocalizeBudget(c.in); got != c.want {
			t.Errorf("clampLocalizeBudget(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLocalizeSeedsFor_StackTrace(t *testing.T) {
	text := "panic: nil pointer\n\tat internal/ledger/ledger.go:42\n\tBalance(ctx, id)\n\tat pkg.Handler(handler.go:10)"
	got := localizeSeedsFor("stack_trace", text)

	var fileLines, funcs []string
	for _, s := range got {
		switch s.Kind {
		case localizeSeedFileLine:
			fileLines = append(fileLines, s.File)
		case localizeSeedFunction:
			funcs = append(funcs, s.Value)
		default:
			t.Errorf("stack_trace seed of unexpected kind %q: %+v", s.Kind, s)
		}
	}
	if !reflect.DeepEqual(fileLines, []string{"internal/ledger/ledger.go", "handler.go"}) {
		t.Errorf("file_line seeds = %v, want [internal/ledger/ledger.go handler.go]", fileLines)
	}
	if !reflect.DeepEqual(funcs, []string{"Balance", "Handler"}) {
		t.Errorf("function seeds = %v, want [Balance Handler] (dotted pkg.Handler -> Handler)", funcs)
	}
}

func TestLocalizeSeedsFor_FailingTest(t *testing.T) {
	got := localizeSeedsFor("failing_test", "TestBalance/empty_account")
	want := []localizeSeed{
		{Kind: localizeSeedFunction, Value: "TestBalance"},
		{Kind: localizeSeedLiteral, Value: "TestBalance/empty_account"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("localizeSeedsFor(failing_test) = %+v, want %+v", got, want)
	}
}

// TestLocalizeSeedsFor_FailingTestStripsRunnerFormatting is the review's
// fix-first item: go test runner formatting around a failing test name
// must reduce to the same base function seed as the bare name.
func TestLocalizeSeedsFor_FailingTestStripsRunnerFormatting(t *testing.T) {
	cases := []struct {
		text     string
		wantBase string
	}{
		{"--- FAIL: TestBalance (0.00s)", "TestBalance"},
		{"=== RUN   TestBalance/sub", "TestBalance"},
		{"FAIL: TestBalance", "TestBalance"},
		{"TestBalance/subcase", "TestBalance"},
		{"TestBalance", "TestBalance"},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			got := localizeSeedsFor("failing_test", c.text)
			if len(got) == 0 || got[0].Kind != localizeSeedFunction || got[0].Value != c.wantBase {
				t.Errorf("localizeSeedsFor(failing_test, %q) = %+v, want a function seed %q first", c.text, got, c.wantBase)
			}
		})
	}
}

func TestLocalizeSeedsFor_Error(t *testing.T) {
	text := `ledger: validation failed at ledger.go:5: "errMissingAccount"`
	got := localizeSeedsFor("error", text)

	var fileLines []string
	var literals []string
	for _, s := range got {
		switch s.Kind {
		case localizeSeedFileLine:
			fileLines = append(fileLines, s.File)
		case localizeSeedLiteral:
			literals = append(literals, s.Value)
		default:
			t.Errorf("error seed of unexpected kind %q: %+v", s.Kind, s)
		}
	}
	if !reflect.DeepEqual(fileLines, []string{"ledger.go"}) {
		t.Errorf("file_line seeds = %v, want [ledger.go]", fileLines)
	}
	if !reflect.DeepEqual(literals, []string{"errMissingAccount"}) {
		t.Errorf("literal seeds = %v, want [errMissingAccount]", literals)
	}
}

func TestLocalizeSeedsFor_Concern(t *testing.T) {
	got := localizeSeedsFor("concern", "How does the Balance validation reject an empty account?")
	var terms []string
	for _, s := range got {
		if s.Kind != localizeSeedTerm {
			t.Errorf("concern seed of unexpected kind %q: %+v", s.Kind, s)
			continue
		}
		terms = append(terms, s.Value)
	}
	// "How"/"the"/"an" are below the 4-char floor (localizeWordRE), "does"
	// is a stopword; every other significant word survives, in order.
	want := []string{"Balance", "validation", "reject", "empty", "account"}
	if !reflect.DeepEqual(terms, want) {
		t.Errorf("concern terms = %v, want %v", terms, want)
	}
}

func TestLocalizeSeedsFor_DedupPreservesFirstOccurrenceOrder(t *testing.T) {
	got := localizeSeedsFor("stack_trace", "Balance(x); later Balance(y) again")
	if len(got) != 1 || got[0].Value != "Balance" {
		t.Errorf("expected exactly one deduplicated Balance seed, got %+v", got)
	}
}

func TestLocalizeSeedsFor_Deterministic(t *testing.T) {
	text := "at ledger.go:5 in Balance, also see Handler(x) and \"errMissingAccount\""
	a := localizeSeedsFor("error", text)
	b := localizeSeedsFor("error", text)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("localizeSeedsFor is not deterministic: %+v vs %+v", a, b)
	}
}

func TestLocalizePathsMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"ledger.go", "ledger.go", true},
		{"internal/ledger/ledger.go", "ledger/ledger.go", true},
		{"/abs/repo/internal/ledger/ledger.go", "internal/ledger/ledger.go", true},
		{"ledger.go", "handler.go", false},
		{"other/ledger.go", "ledger.go", true},
		{"notledger.go", "ledger.go", false},
	}
	for _, c := range cases {
		if got := localizePathsMatch(c.a, c.b); got != c.want {
			t.Errorf("localizePathsMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// ─── expandSeedsByGraph (ADR-041 D4) ───────────────────────────────────────

func TestExpandSeedsByGraph_ChainPreferstypedOverSeedsOwnHeuristicEdge(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Root", Kind: core.KindGoFunc, Calls: []string{"svc::pkg.Extra"}},
		{Service: "svc", Package: "pkg", Name: "Mid", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Leaf", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Extra", Kind: core.KindGoFunc},
	}
	g := callgraph.Build(nodes) // Root -[heuristic, from Calls]-> Extra
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.Root", ToSymbol: "svc::pkg.Mid", Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: "svc::pkg.Mid", ToSymbol: "svc::pkg.Leaf", Kind: core.EdgeDirect},
	})

	got := expandSeedsByGraph([]string{"svc::pkg.Root"}, g, 2)
	byKey := map[string]graphExpansion{}
	for _, e := range got {
		byKey[e.Key] = e
	}

	mid, ok := byKey["svc::pkg.Mid"]
	if !ok || mid.Distance != 1 || mid.Tier != "typed" {
		t.Errorf("Mid = %+v (ok=%v), want distance 1, tier typed", mid, ok)
	}
	leaf, ok := byKey["svc::pkg.Leaf"]
	if !ok || leaf.Distance != 2 || leaf.Tier != "typed" {
		t.Errorf("Leaf = %+v (ok=%v), want distance 2, tier typed", leaf, ok)
	}
	if !reflect.DeepEqual(leaf.Via, []string{"svc::pkg.Root", "svc::pkg.Mid", "svc::pkg.Leaf"}) {
		t.Errorf("Leaf.Via = %v, want the full seed-to-key chain", leaf.Via)
	}
	// Root already has a typed edge (to Mid) — its heuristic-only Calls
	// edge to Extra must be skipped entirely ("resolved edges first;
	// heuristic only when a seed has NO resolved edges").
	if _, ok := byKey["svc::pkg.Extra"]; ok {
		t.Errorf("Extra must not appear: Root has a typed edge, so its heuristic-only edge is skipped, got %+v", got)
	}
}

func TestExpandSeedsByGraph_HeuristicFallbackWhenSeedHasNoTypedEdges(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Solo", Kind: core.KindGoFunc, Calls: []string{"svc::pkg.Neighbor"}},
		{Service: "svc", Package: "pkg", Name: "Neighbor", Kind: core.KindGoFunc},
	}
	g := callgraph.Build(nodes) // Solo -[heuristic only]-> Neighbor; no typed edges anywhere

	got := expandSeedsByGraph([]string{"svc::pkg.Solo"}, g, 2)
	if len(got) != 1 || got[0].Key != "svc::pkg.Neighbor" || got[0].Distance != 1 || got[0].Tier != "heuristic" {
		t.Errorf("expandSeedsByGraph(Solo) = %+v, want exactly one heuristic distance-1 Neighbor (a seed with no resolved edges falls back to heuristic)", got)
	}
}

func TestExpandSeedsByGraph_ConvergenceRecordsEverySeed(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "SeedA", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "SeedB", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Shared", Kind: core.KindGoFunc},
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.SeedA", ToSymbol: "svc::pkg.Shared", Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: "svc::pkg.SeedB", ToSymbol: "svc::pkg.Shared", Kind: core.EdgeDirect},
	})

	got := expandSeedsByGraph([]string{"svc::pkg.SeedA", "svc::pkg.SeedB"}, g, 2)
	var shared *graphExpansion
	for i := range got {
		if got[i].Key == "svc::pkg.Shared" {
			shared = &got[i]
		}
	}
	if shared == nil {
		t.Fatalf("expected a Shared candidate, got %+v", got)
	}
	want := []string{"svc::pkg.SeedA", "svc::pkg.SeedB"}
	sort.Strings(want)
	if !reflect.DeepEqual(shared.FromSeeds, want) {
		t.Errorf("Shared.FromSeeds = %v, want %v (both seeds converge on it)", shared.FromSeeds, want)
	}
}

func TestExpandSeedsByGraph_NilGraphOrZeroDepth(t *testing.T) {
	if got := expandSeedsByGraph([]string{"x"}, nil, 2); got != nil {
		t.Errorf("expandSeedsByGraph(nil graph) = %v, want nil", got)
	}
	nodes := []core.SymbolNode{{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc}}
	g := callgraph.Build(nodes)
	if got := expandSeedsByGraph([]string{"svc::pkg.A"}, g, 0); got != nil {
		t.Errorf("expandSeedsByGraph(depth 0) = %v, want nil", got)
	}
}

func TestExpandSeedsByGraph_Deterministic(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Root", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Mid", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Leaf", Kind: core.KindGoFunc},
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.Root", ToSymbol: "svc::pkg.Mid", Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: "svc::pkg.Mid", ToSymbol: "svc::pkg.Leaf", Kind: core.EdgeDirect},
	})
	a := expandSeedsByGraph([]string{"svc::pkg.Root"}, g, 2)
	b := expandSeedsByGraph([]string{"svc::pkg.Root"}, g, 2)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("expandSeedsByGraph is not deterministic: %+v vs %+v", a, b)
	}
}

// ─── reformulateSeeds (ADR-041 D4) ─────────────────────────────────────────

func TestSplitIdentifier(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"getUserByID", []string{"get", "User", "By", "ID"}},
		{"get_user_by_id", []string{"get", "user", "by", "id"}},
		{"Balance", []string{"Balance"}}, // single word, no internal case transition
		{"balance", []string{"balance"}},
	}
	for _, c := range cases {
		if got := splitIdentifier(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitIdentifier(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestReformulateSeeds_IdentifierSplitting(t *testing.T) {
	seeds, terms := reformulateSeeds("something broke inside getBalanceForUser but not there")
	wantTerms := []string{"getBalanceForUser", "get", "Balance", "User"}
	if !reflect.DeepEqual(terms, wantTerms) {
		t.Errorf("reformulateSeeds terms = %v, want %v (get/Balance/User pass the >=3-char, non-stopword filter; For/for is a stopword)", terms, wantTerms)
	}
	var funcs, termSeeds []string
	for _, s := range seeds {
		switch s.Kind {
		case localizeSeedFunction:
			funcs = append(funcs, s.Value)
		case localizeSeedTerm:
			termSeeds = append(termSeeds, s.Value)
		default:
			t.Errorf("unexpected seed kind %q in %+v", s.Kind, s)
		}
	}
	if !reflect.DeepEqual(funcs, []string{"getBalanceForUser"}) {
		t.Errorf("function seeds = %v, want [getBalanceForUser]", funcs)
	}
	if !reflect.DeepEqual(termSeeds, []string{"get", "Balance", "User"}) {
		t.Errorf("term seeds = %v, want [get Balance User]", termSeeds)
	}
}

func TestReformulateSeeds_QuotedLiteralAndBasename(t *testing.T) {
	seeds, _ := reformulateSeeds(`saw "disk full" near internal/ledger/ledger_test.go:42`)
	var literals, terms []string
	for _, s := range seeds {
		switch s.Kind {
		case localizeSeedLiteral:
			literals = append(literals, s.Value)
		case localizeSeedTerm:
			terms = append(terms, s.Value)
		}
	}
	if !reflect.DeepEqual(literals, []string{"disk full"}) {
		t.Errorf("literal seeds = %v, want [disk full]", literals)
	}
	if !containsStrSeed(terms, "ledger_test") {
		t.Errorf("term seeds = %v, want to include the file basename ledger_test", terms)
	}
}

func TestReformulateSeeds_NothingToDerive(t *testing.T) {
	seeds, terms := reformulateSeeds("the a to is")
	if len(seeds) != 0 || len(terms) != 0 {
		t.Errorf("reformulateSeeds(no identifiers/literals/frames) = %+v/%v, want both empty", seeds, terms)
	}
}

func TestReformulateSeeds_Deterministic(t *testing.T) {
	text := "crash inside getBalanceForUser near internal/ledger/ledger_test.go:42 with \"disk full\""
	s1, t1 := reformulateSeeds(text)
	s2, t2 := reformulateSeeds(text)
	if !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(t1, t2) {
		t.Errorf("reformulateSeeds is not deterministic: (%+v,%v) vs (%+v,%v)", s1, t1, s2, t2)
	}
}

func containsStrSeed(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestFindFileByPathSuffix_DeterministicOnAmbiguity(t *testing.T) {
	byFile := map[string][]core.SymbolNode{
		"a/ledger.go": {{Name: "A", FilePath: "a/ledger.go", Line: 1}},
		"b/ledger.go": {{Name: "B", FilePath: "b/ledger.go", Line: 1}},
	}
	// Both keys suffix-match "ledger.go"; the choice must be the same
	// every time (sorted key order), never map-iteration-order dependent.
	path1, _, ok1 := findFileByPathSuffix(byFile, "ledger.go")
	path2, _, ok2 := findFileByPathSuffix(byFile, "ledger.go")
	if !ok1 || !ok2 || path1 != path2 {
		t.Errorf("findFileByPathSuffix not deterministic: (%q,%v) vs (%q,%v)", path1, ok1, path2, ok2)
	}
	if path1 != "a/ledger.go" {
		t.Errorf("findFileByPathSuffix = %q, want the sorted-first match a/ledger.go", path1)
	}
}

// TestFindFileForFrame_HintFirstThenOnlyAUniqueBasename (TL-1): a JVM frame's
// package names its file; when the package names no indexed directory (a
// library frame), a basename two files share resolves nothing rather than
// the first sorted guess.
func TestFindFileForFrame_HintFirstThenOnlyAUniqueBasename(t *testing.T) {
	byFile := map[string][]core.SymbolNode{
		"src/main/java/com/a/Util.java":   {{Name: "round", Line: 1}},
		"src/main/java/com/b/Util.java":   {{Name: "round", Line: 1}},
		"src/main/java/com/b/Single.java": {{Name: "only", Line: 1}},
	}
	if got, _, ok := findFileForFrame(byFile, "Util.java", "com/b/Util.java"); !ok || got != "src/main/java/com/b/Util.java" {
		t.Errorf("hinted = %q %v, want com/b/Util.java", got, ok)
	}
	if got, _, ok := findFileForFrame(byFile, "Util.java", "java/util/Util.java"); ok {
		t.Errorf("an unresolved hint over a shared basename resolved %q, want nothing", got)
	}
	if got, _, ok := findFileForFrame(byFile, "Single.java", "org/x/Single.java"); !ok || got != "src/main/java/com/b/Single.java" {
		t.Errorf("unique basename = %q %v, want com/b/Single.java", got, ok)
	}
	if got, _, ok := findFileForFrame(byFile, "Util.java", ""); !ok || got != "src/main/java/com/a/Util.java" {
		t.Errorf("unhinted = %q %v, want the sorted-first match as before", got, ok)
	}
}

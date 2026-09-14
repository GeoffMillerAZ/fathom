package mcp

import (
	"reflect"
	"testing"

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

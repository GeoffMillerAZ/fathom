package lookup

import (
	"sort"
	"sync"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sortedEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Add + Query
// ---------------------------------------------------------------------------

func TestAdd_SingleTerm(t *testing.T) {
	idx := New()
	idx.Add("id1", "validate")
	got := idx.Query("validate")
	if !sortedEq(got, []string{"id1"}) {
		t.Fatalf("want [id1] got %v", got)
	}
}

func TestAdd_CaseInsensitive(t *testing.T) {
	idx := New()
	idx.Add("id1", "ValidateToken")
	got := idx.Query("validatetoken")
	if !sortedEq(got, []string{"id1"}) {
		t.Fatalf("want [id1] got %v", got)
	}
}

func TestAdd_MultipleTerms(t *testing.T) {
	idx := New()
	idx.Add("id1", "auth", "token", "validate")
	if !sortedEq(idx.Query("auth"), []string{"id1"}) {
		t.Fatal("missing id1 under auth")
	}
	if !sortedEq(idx.Query("token"), []string{"id1"}) {
		t.Fatal("missing id1 under token")
	}
}

func TestAdd_MultipleIDs_SameTerm(t *testing.T) {
	idx := New()
	idx.Add("id2", "auth")
	idx.Add("id1", "auth")
	idx.Add("id3", "auth")
	got := idx.Query("auth")
	want := []string{"id1", "id2", "id3"}
	if !sortedEq(got, want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

func TestAdd_Deduplication(t *testing.T) {
	idx := New()
	idx.Add("id1", "alpha")
	idx.Add("id1", "alpha")
	idx.Add("id1", "alpha")
	got := idx.Query("alpha")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %v", got)
	}
}

func TestQuery_Miss(t *testing.T) {
	idx := New()
	got := idx.Query("unknown")
	if got != nil {
		t.Fatalf("expected nil on miss, got %v", got)
	}
}

func TestQuery_ReturnsCopy(t *testing.T) {
	idx := New()
	idx.Add("id1", "term")
	got := idx.Query("term")
	got[0] = "mutated"
	// Original should be unaffected.
	got2 := idx.Query("term")
	if got2[0] != "id1" {
		t.Fatalf("query result was not a copy; mutation leaked: %v", got2)
	}
}

// ---------------------------------------------------------------------------
// Build from SymbolNodes
// ---------------------------------------------------------------------------

func TestBuild_IndexesByNameTokens(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "auth", Name: "ValidateToken", Kind: core.KindGoFunc},
		{Service: "svc", Package: "auth", Name: "ParseClaims", Kind: core.KindGoFunc},
		{Service: "svc", Package: "store", Name: "validateSchema", Kind: core.KindGoFunc},
	}
	idx := New()
	idx.Build(symbols)

	// "validatetoken" is produced by tokenise("ValidateToken") → ["validatetoken"]
	got := idx.Query("validatetoken")
	if len(got) != 1 {
		t.Fatalf("want 1 result for validatetoken, got %v", got)
	}
	if got[0] != "svc/auth.ValidateToken" {
		t.Fatalf("unexpected id: %v", got[0])
	}

	// "parseclaims" → ParseClaims
	got2 := idx.Query("parseclaims")
	if len(got2) != 1 || got2[0] != "svc/auth.ParseClaims" {
		t.Fatalf("unexpected parseclaims result: %v", got2)
	}
}

func TestBuild_MultiTokenName(t *testing.T) {
	// CamelCase is a single token from FieldsFunc perspective;
	// add an explicit underscore-separated name to exercise multi-token.
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "get_user_by_id"},
	}
	idx := New()
	idx.Build(symbols)

	for _, tok := range []string{"get", "user", "by", "id"} {
		got := idx.Query(tok)
		if len(got) == 0 {
			t.Errorf("token %q not indexed", tok)
		}
	}
}

func TestBuild_Deterministic(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Name: "Foo"},
		{Service: "svc", Name: "Bar"},
		{Service: "svc", Name: "Baz"},
	}
	idx1, idx2 := New(), New()
	idx1.Build(symbols)
	idx2.Build(symbols)

	for _, tok := range []string{"foo", "bar", "baz"} {
		r1 := idx1.Query(tok)
		r2 := idx2.Query(tok)
		if !sortedEq(r1, r2) {
			t.Fatalf("non-deterministic result for %q: %v vs %v", tok, r1, r2)
		}
		if !sort.StringsAreSorted(r1) {
			t.Fatalf("result for %q is not sorted: %v", tok, r1)
		}
	}
}

func TestBuild_ReplacesExistingIndex(t *testing.T) {
	idx := New()
	idx.Add("old-id", "oldterm")

	symbols := []core.SymbolNode{
		{Service: "svc", Name: "NewFunc"},
	}
	idx.Build(symbols)

	if got := idx.Query("oldterm"); got != nil {
		t.Fatalf("Build should have replaced the old index; old term still present: %v", got)
	}
	if got := idx.Query("newfunc"); len(got) == 0 {
		t.Fatal("new term missing after Build")
	}
}

func TestBuild_NoPackage(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "", Name: "Standalone"},
	}
	idx := New()
	idx.Build(symbols)
	got := idx.Query("standalone")
	if len(got) == 0 {
		t.Fatal("no result for standalone")
	}
	if got[0] != "svc/Standalone" {
		t.Fatalf("unexpected id: %q", got[0])
	}
}

// ---------------------------------------------------------------------------
// Concurrent access under -race
// ---------------------------------------------------------------------------

func TestConcurrent_AddQuery(t *testing.T) {
	idx := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			idx.Add("id", "term")
		}(i)
		go func(n int) {
			defer wg.Done()
			idx.Query("term")
		}(i)
	}
	wg.Wait()
}

func TestConcurrent_BuildQuery(t *testing.T) {
	idx := New()
	symbols := []core.SymbolNode{
		{Service: "svc", Name: "Alpha"},
		{Service: "svc", Name: "Beta"},
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.Build(symbols)
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.Query("alpha")
		}()
	}
	wg.Wait()
}

func TestConcurrent_AddBuildQuery(t *testing.T) {
	idx := New()
	symbols := []core.SymbolNode{{Service: "svc", Name: "Gamma"}}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); idx.Add("id", "gamma") }()
		go func() { defer wg.Done(); idx.Build(symbols) }()
		go func() { defer wg.Done(); idx.Query("gamma") }()
	}
	wg.Wait()
}

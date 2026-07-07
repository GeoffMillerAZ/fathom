package packer_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/context/packer"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ---------------------------------------------------------------------------
// Pack tests
// ---------------------------------------------------------------------------

func TestPack_Empty(t *testing.T) {
	got := packer.Pack(nil, 100)
	if len(got.Included) != 0 {
		t.Errorf("expected no included items, got %d", len(got.Included))
	}
	if got.Tokens != 0 {
		t.Errorf("expected 0 tokens, got %d", got.Tokens)
	}
	if got.Dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", got.Dropped)
	}
}

func TestPack_ZeroBudget(t *testing.T) {
	items := []packer.Evidence{
		{ID: "a", Text: "hello", Score: 1.0},
	}
	got := packer.Pack(items, 0)
	if len(got.Included) != 0 {
		t.Errorf("expected no included items with zero budget, got %d", len(got.Included))
	}
	if got.Dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", got.Dropped)
	}
}

func TestPack_AllFit(t *testing.T) {
	items := []packer.Evidence{
		{ID: "a", Text: "abc", Score: 0.5},
		{ID: "b", Text: "def", Score: 0.9},
	}
	got := packer.Pack(items, 1000)
	if len(got.Included) != 2 {
		t.Fatalf("expected 2 included, got %d", len(got.Included))
	}
	if got.Dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", got.Dropped)
	}
}

func TestPack_ScoreOrdering(t *testing.T) {
	items := []packer.Evidence{
		{ID: "low", Text: strings.Repeat("x", 40), Score: 0.1},
		{ID: "high", Text: strings.Repeat("y", 40), Score: 0.9},
		{ID: "mid", Text: strings.Repeat("z", 40), Score: 0.5},
	}
	// Budget fits exactly 2 items (each ~10 tokens, budget=20).
	got := packer.Pack(items, 20)
	if len(got.Included) != 2 {
		t.Fatalf("expected 2 included, got %d", len(got.Included))
	}
	// High score first.
	if got.Included[0].ID != "high" {
		t.Errorf("expected first item to be 'high', got %q", got.Included[0].ID)
	}
	if got.Included[1].ID != "mid" {
		t.Errorf("expected second item to be 'mid', got %q", got.Included[1].ID)
	}
	if got.Dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", got.Dropped)
	}
}

func TestPack_TieBrokenByID(t *testing.T) {
	items := []packer.Evidence{
		{ID: "z", Text: "ztext", Score: 1.0},
		{ID: "a", Text: "atext", Score: 1.0},
		{ID: "m", Text: "mtext", Score: 1.0},
	}
	got := packer.Pack(items, 1000)
	if len(got.Included) != 3 {
		t.Fatalf("expected 3 included, got %d", len(got.Included))
	}
	// Tie on score → sorted by ID ascending.
	want := []string{"a", "m", "z"}
	for i, ev := range got.Included {
		if ev.ID != want[i] {
			t.Errorf("included[%d]: want ID %q, got %q", i, want[i], ev.ID)
		}
	}
}

func TestPack_TokenAccounting(t *testing.T) {
	// Text of 8 bytes → tokenEstimate = ceil(8/4) = 2 tokens.
	item := packer.Evidence{ID: "x", Text: "abcdefgh", Score: 1.0}
	got := packer.Pack([]packer.Evidence{item}, 2)
	if len(got.Included) != 1 {
		t.Fatalf("expected 1 included, got %d", len(got.Included))
	}
	if got.Tokens != 2 {
		t.Errorf("expected 2 tokens, got %d", got.Tokens)
	}
}

func TestPack_Deterministic(t *testing.T) {
	items := []packer.Evidence{
		{ID: "c", Text: strings.Repeat("c", 20), Score: 0.8},
		{ID: "a", Text: strings.Repeat("a", 20), Score: 0.8},
		{ID: "b", Text: strings.Repeat("b", 20), Score: 0.8},
	}
	first := packer.Pack(items, 15)
	second := packer.Pack(items, 15)
	if len(first.Included) != len(second.Included) {
		t.Fatalf("non-deterministic: different lengths %d vs %d", len(first.Included), len(second.Included))
	}
	for i := range first.Included {
		if first.Included[i].ID != second.Included[i].ID {
			t.Errorf("non-deterministic at index %d: %q vs %q", i, first.Included[i].ID, second.Included[i].ID)
		}
	}
}

// ---------------------------------------------------------------------------
// RepoMap tests
// ---------------------------------------------------------------------------

func TestRepoMap_Empty(t *testing.T) {
	if got := packer.RepoMap(nil, 100); got != "" {
		t.Errorf("expected empty string for nil symbols, got %q", got)
	}
}

func TestRepoMap_ZeroBudget(t *testing.T) {
	syms := []core.SymbolNode{{Package: "pkg", Name: "Foo"}}
	if got := packer.RepoMap(syms, 0); got != "" {
		t.Errorf("expected empty string for zero budget, got %q", got)
	}
}

func TestRepoMap_RankedByEdgeCount(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "pkg", Name: "A", Calls: []string{"x"}},
		{Package: "pkg", Name: "B", Calls: []string{"x", "y", "z"}},
		{Package: "pkg", Name: "C", Calls: nil},
	}
	got := packer.RepoMap(syms, 1000)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	// B has most edges → first line.
	if !strings.Contains(lines[0], "B") {
		t.Errorf("expected B first (most edges), got %q", lines[0])
	}
	if !strings.Contains(lines[1], "A") {
		t.Errorf("expected A second, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "C") {
		t.Errorf("expected C last (no edges), got %q", lines[2])
	}
}

func TestRepoMap_BudgetEnforced(t *testing.T) {
	// Each symbol line is ~10+ tokens; budget of 5 should admit very few.
	syms := []core.SymbolNode{
		{Package: "pkg", Name: "Alpha", Signature: "func Alpha()"},
		{Package: "pkg", Name: "Beta", Signature: "func Beta()"},
		{Package: "pkg", Name: "Gamma", Signature: "func Gamma()"},
	}
	got := packer.RepoMap(syms, 5)
	// With budget=5, at most a very short line fits. Result may be empty or 1.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if got == "" {
		lines = nil
	}
	if len(lines) >= 3 {
		t.Errorf("expected fewer than 3 lines with tight budget, got %d", len(lines))
	}
}

func TestRepoMap_Deterministic(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "pkg", Name: "X", Calls: []string{"a", "b"}},
		{Package: "pkg", Name: "Y", Calls: []string{"a", "b"}},
		{Package: "pkg", Name: "Z", Calls: []string{"a"}},
	}
	a := packer.RepoMap(syms, 200)
	b := packer.RepoMap(syms, 200)
	if a != b {
		t.Errorf("RepoMap is non-deterministic:\n%q\nvs\n%q", a, b)
	}
}

func TestRepoMap_TieBrokenByPackageName(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "pkg", Name: "Z", Calls: []string{"a"}},
		{Package: "pkg", Name: "A", Calls: []string{"b"}},
	}
	got := packer.RepoMap(syms, 1000)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "A") {
		t.Errorf("expected A first (tie broken by name), got %q", lines[0])
	}
}

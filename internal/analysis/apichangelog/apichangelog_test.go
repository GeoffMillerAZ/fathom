package apichangelog_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/apichangelog"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// helpers to build SymbolNode fixtures quickly.
func goFunc(pkg, name, sig string) core.SymbolNode {
	return core.SymbolNode{Package: pkg, Name: name, Kind: core.KindGoFunc, Signature: sig}
}

func goType(pkg, name, sig string) core.SymbolNode {
	return core.SymbolNode{Package: pkg, Name: name, Kind: core.KindGoType, Signature: sig}
}

// privateFunc is unexported — should be ignored by Changelog.
func privateFunc(pkg, name string) core.SymbolNode {
	return core.SymbolNode{Package: pkg, Name: name, Kind: core.KindGoFunc}
}

var (
	oldAPI = []core.SymbolNode{
		goFunc("auth", "Login", "func Login(ctx context.Context, user string) error"),
		goFunc("auth", "Logout", "func Logout(ctx context.Context) error"),
		goType("auth", "Token", "type Token struct { ... }"),
		privateFunc("auth", "internalHelper"), // unexported – ignored
	}

	newAPI = []core.SymbolNode{
		// Login signature changed (added password param)
		goFunc("auth", "Login", "func Login(ctx context.Context, user, password string) error"),
		// Logout unchanged
		goFunc("auth", "Logout", "func Logout(ctx context.Context) error"),
		// Token removed
		// Register added
		goFunc("auth", "Register", "func Register(ctx context.Context, user string) error"),
		privateFunc("auth", "internalHelper"), // unexported – ignored
	}
)

func TestChangelog_Basic(t *testing.T) {
	entries := apichangelog.Changelog(oldAPI, newAPI)

	// Expected: added=Register, removed=Token, changed=Login
	// Sorted: added < changed < removed (by kind priority), then alpha within kind.
	want := []apichangelog.Entry{
		{Kind: "added", Symbol: "auth.Register"},
		{Kind: "changed", Symbol: "auth.Login", Detail: "func Login(ctx context.Context, user, password string) error"},
		{Kind: "removed", Symbol: "auth.Token"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("Changelog() =\n%#v\nwant\n%#v", entries, want)
	}
}

func TestChangelog_NoChanges(t *testing.T) {
	entries := apichangelog.Changelog(oldAPI, oldAPI)
	if len(entries) != 0 {
		t.Errorf("expected no changes, got %v", entries)
	}
}

func TestChangelog_AllAdded(t *testing.T) {
	entries := apichangelog.Changelog(nil, newAPI)
	for _, e := range entries {
		if e.Kind != "added" {
			t.Errorf("expected all 'added', got Kind=%q for %q", e.Kind, e.Symbol)
		}
	}
	// 3 exported symbols in newAPI (Register, Login, Logout)
	if len(entries) != 3 {
		t.Errorf("expected 3 added entries, got %d: %v", len(entries), entries)
	}
}

func TestChangelog_AllRemoved(t *testing.T) {
	entries := apichangelog.Changelog(oldAPI, nil)
	for _, e := range entries {
		if e.Kind != "removed" {
			t.Errorf("expected all 'removed', got Kind=%q for %q", e.Kind, e.Symbol)
		}
	}
	// 3 exported symbols in oldAPI (Login, Logout, Token)
	if len(entries) != 3 {
		t.Errorf("expected 3 removed entries, got %d: %v", len(entries), entries)
	}
}

func TestChangelog_SortOrder(t *testing.T) {
	// Build a scenario with multiple entries of each kind to verify sort.
	old := []core.SymbolNode{
		goFunc("pkg", "Zoo", "func Zoo()"),
		goFunc("pkg", "Alpha", "func Alpha()"),
		goFunc("pkg", "Beta", "func Beta() int"),
	}
	nw := []core.SymbolNode{
		// Alpha removed, Zoo changed, Gamma added, Delta added
		goFunc("pkg", "Zoo", "func Zoo() error"),
		goFunc("pkg", "Beta", "func Beta() int"),
		goFunc("pkg", "Gamma", "func Gamma()"),
		goFunc("pkg", "Delta", "func Delta()"),
	}
	entries := apichangelog.Changelog(old, nw)

	// added: Delta, Gamma; changed: Zoo; removed: Alpha
	wantSymbols := []string{"pkg.Delta", "pkg.Gamma", "pkg.Zoo", "pkg.Alpha"}
	wantKinds := []string{"added", "added", "changed", "removed"}
	if len(entries) != len(wantSymbols) {
		t.Fatalf("expected %d entries, got %d: %v", len(wantSymbols), len(entries), entries)
	}
	for i, e := range entries {
		if e.Symbol != wantSymbols[i] || e.Kind != wantKinds[i] {
			t.Errorf("entry[%d] = {%s %s}; want {%s %s}", i, e.Kind, e.Symbol, wantKinds[i], wantSymbols[i])
		}
	}
}

func TestChangelog_Determinism(t *testing.T) {
	e1 := apichangelog.Changelog(oldAPI, newAPI)
	e2 := apichangelog.Changelog(oldAPI, newAPI)
	if !reflect.DeepEqual(e1, e2) {
		t.Errorf("non-deterministic: %v vs %v", e1, e2)
	}
}

func TestFormat_ContainsExpected(t *testing.T) {
	entries := apichangelog.Changelog(oldAPI, newAPI)
	md := apichangelog.Format(entries)

	if !strings.HasPrefix(md, "## API Changes\n") {
		t.Errorf("Format() should start with '## API Changes\\n', got: %q", md[:min(50, len(md))])
	}
	if !strings.Contains(md, "**added**") {
		t.Error("Format() missing **added** marker")
	}
	if !strings.Contains(md, "**removed**") {
		t.Error("Format() missing **removed** marker")
	}
	if !strings.Contains(md, "**changed**") {
		t.Error("Format() missing **changed** marker")
	}
	if !strings.Contains(md, "new signature:") {
		t.Error("Format() missing 'new signature:' for changed entry")
	}
}

func TestFormat_EmptyEntries(t *testing.T) {
	md := apichangelog.Format(nil)
	if md != "## API Changes\n" {
		t.Errorf("Format(nil) = %q; want %q", md, "## API Changes\n")
	}
}

func TestFormat_Determinism(t *testing.T) {
	entries := apichangelog.Changelog(oldAPI, newAPI)
	m1 := apichangelog.Format(entries)
	m2 := apichangelog.Format(entries)
	if m1 != m2 {
		t.Errorf("Format non-deterministic")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func writeFlagFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseRipgrepOutputNoContext(t *testing.T) {
	raw := "pkg/a.go:3:func A() {\npkg/a.go:7:func B() {\n"
	hits := parseRipgrepOutput(raw, 0)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
	if hits[0].File != "pkg/a.go" || hits[0].Line != 3 || hits[0].Text != "func A() {" {
		t.Errorf("hits[0] = %+v", hits[0])
	}
	if hits[1].Line != 7 {
		t.Errorf("hits[1] = %+v", hits[1])
	}
}

func TestParseRipgrepOutputWithContext(t *testing.T) {
	// rg -C1 shape: one context line before, the match, one after, "--" between groups.
	raw := "pkg/a.go-2-func A() {\npkg/a.go:3:\tB()\npkg/a.go-4-}\n--\npkg/a.go-6-func B() {\npkg/a.go:7:\treturn\npkg/a.go-8-}\n"
	hits := parseRipgrepOutput(raw, 1)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
	if len(hits[0].Before) != 1 || hits[0].Before[0] != "func A() {" {
		t.Errorf("hits[0].Before = %v", hits[0].Before)
	}
	if len(hits[0].After) != 1 || hits[0].After[0] != "}" {
		t.Errorf("hits[0].After = %v", hits[0].After)
	}
	if len(hits[1].Before) != 1 || hits[1].Before[0] != "func B() {" {
		t.Errorf("hits[1].Before = %v", hits[1].Before)
	}
}

func TestEnclosingSymbol(t *testing.T) {
	syms := []core.SymbolNode{
		{Name: "A", Kind: core.KindGoFunc, Line: 3, EndLine: 5},
		{Name: "B", Kind: core.KindGoFunc, Line: 7, EndLine: 0}, // open-ended, last in file
	}
	cases := []struct {
		line     int
		wantName string
		wantOK   bool
	}{
		{1, "", false},   // before any symbol
		{3, "A", true},   // A's opening line
		{5, "A", true},   // A's closing line (EndLine)
		{6, "", false},   // between A and B, A already closed
		{7, "B", true},   // B's opening line
		{100, "B", true}, // deep inside B (open-ended: last symbol in file)
	}
	for _, c := range cases {
		got, ok := enclosingSymbol(syms, c.line)
		if ok != c.wantOK || (ok && got.Name != c.wantName) {
			t.Errorf("enclosingSymbol(line=%d) = (%q, %v), want (%q, %v)", c.line, got.Name, ok, c.wantName, c.wantOK)
		}
	}
}

func TestSymbolKindLabel(t *testing.T) {
	cases := []struct {
		kind     core.SourceKind
		wantOK   bool
		wantName string
	}{
		{core.KindGoFunc, true, "func"},
		{core.KindGoMethod, true, "method"},
		{core.KindGoType, true, "type"},
		{core.KindGoInterface, true, "type"},
		{core.KindGoConst, true, "const"},
		{core.KindGoVar, true, "var"},
		{core.KindRustFn, true, "func"},
		{core.KindTerraformResource, false, ""},
		{core.KindK8sWorkload, false, ""},
	}
	for _, c := range cases {
		got, ok := symbolKindLabel(c.kind)
		if ok != c.wantOK || got != c.wantName {
			t.Errorf("symbolKindLabel(%v) = (%q, %v), want (%q, %v)", c.kind, got, ok, c.wantName, c.wantOK)
		}
	}
}

func TestIsTestPath(t *testing.T) {
	cases := map[string]bool{
		"pkg/a_test.go":          true,
		"pkg/a.go":               false,
		"a/test_thing.py":        true,
		"a/thing.py":             false,
		"a/thing.test.js":        true,
		"a/thing_spec.rb":        true,
		"internal/testdata/x.go": true,
		"internal/x.go":          false,
	}
	for rel, want := range cases {
		if got := isTestPath(rel); got != want {
			t.Errorf("isTestPath(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestClassifyHit(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		line      string
		pattern   string
		wantClass string
		wantOK    bool
	}{
		{"go ident", "a.go", `func A() { B() }`, `B`, "ident", true},
		{"go line comment", "a.go", `// call B() here`, `B`, "comment", true},
		{"go string literal", "a.go", `s := "call B here"`, `B`, "string", true},
		{"go token before comment stays ident", "a.go", `A() // then B()`, `A`, "ident", true},
		{"unknown extension skipped", "a.unknownext", `func A() { B() }`, `B`, "", false},
		{"no extension skipped", "Makefile", `A: B`, `B`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			idx := indexOf(c.line, c.pattern)
			if idx < 0 {
				t.Fatalf("pattern %q not found in line %q", c.pattern, c.line)
			}
			got, ok := classifyHit(c.file, c.line, idx, idx+len(c.pattern))
			if ok != c.wantOK || got != c.wantClass {
				t.Errorf("classifyHit(%q,%q) = (%q,%v), want (%q,%v)", c.file, c.line, got, ok, c.wantClass, c.wantOK)
			}
		})
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestSearchComputeFlags(t *testing.T) {
	root := t.TempDir()
	writeFlagFixture(t, root, "pkg/a_test.go", "package pkg\n")
	writeFlagFixture(t, root, "vendor/lib/v.go", "package lib\n")
	writeFlagFixture(t, root, "gen/g.go", "// Code generated by tool. DO NOT EDIT.\npackage gen\n")
	writeFlagFixture(t, root, "pkg/plain.go", "package pkg\n")

	if flags := searchComputeFlags(root, "pkg/a_test.go"); !contains(flags, "test") {
		t.Errorf("pkg/a_test.go flags = %v, want test", flags)
	}
	if flags := searchComputeFlags(root, "vendor/lib/v.go"); !contains(flags, "vendored") {
		t.Errorf("vendor/lib/v.go flags = %v, want vendored", flags)
	}
	if flags := searchComputeFlags(root, "gen/g.go"); !contains(flags, "generated") {
		t.Errorf("gen/g.go flags = %v, want generated", flags)
	}
	if flags := searchComputeFlags(root, "pkg/plain.go"); len(flags) != 0 {
		t.Errorf("pkg/plain.go flags = %v, want none", flags)
	}
}

// TestSearchRunFallsBackWhenRipgrepFails proves the fallback promise
// directly: a PATH whose only "rg" is broken must still produce the exact
// same hits via searchWalk, reporting engine "regexp".
func TestSearchRunFallsBackWhenRipgrepFails(t *testing.T) {
	root := t.TempDir()
	writeFlagFixture(t, root, "a.txt", "needle one\nneedle two\n")

	fakeBin := t.TempDir()
	script := "#!/bin/sh\nexit 2\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "rg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin)

	hits, engine, err := searchRun(context.Background(), root, "needle", "", true, 0)
	if err != nil {
		t.Fatalf("searchRun: %v", err)
	}
	if engine != "regexp" {
		t.Fatalf("engine = %q, want regexp (rg on PATH is broken)", engine)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

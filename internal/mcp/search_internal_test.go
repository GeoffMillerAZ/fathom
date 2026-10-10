package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
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
		// lines:nil throughout — A has a real EndLine and B is the file's
		// last (open-ended) symbol, so neither case exercises the
		// fallback-narrowing lines feeds; that is TestEnclosingSymbol_
		// FallbackNarrowing's job below.
		got, ok := enclosingSymbol(syms, c.line, nil)
		if ok != c.wantOK || (ok && got.Name != c.wantName) {
			t.Errorf("enclosingSymbol(line=%d) = (%q, %v), want (%q, %v)", c.line, got.Name, ok, c.wantName, c.wantOK)
		}
	}
}

// K18 (wave lf2): a Python file's `<module>` symbol spans line 1 to its last
// line, so a definition opening on line 1 shares its start line. The
// definition, the narrower span, encloses its own lines and the module
// symbol the rest, whatever order the store hands the two over in
// (groupSymbolsByFile, then enclosingSymbol: search's receipt, localize's
// frame).
func TestEnclosingSymbol_ModuleSymbolSharesLineOne(t *testing.T) {
	mod := core.SymbolNode{Name: core.ModuleSymbolName, Kind: "PythonModule", FilePath: "a.py", Line: 1, EndLine: 9}
	def := core.SymbolNode{Name: "main", Kind: "PythonFunc", FilePath: "a.py", Line: 1, EndLine: 3}
	want := map[int]string{1: "main", 2: "main", 3: "main", 5: core.ModuleSymbolName, 9: core.ModuleSymbolName}
	for _, nodes := range [][]core.SymbolNode{{mod, def}, {def, mod}} {
		syms := groupSymbolsByFile(nodes)["a.py"]
		for line := 1; line <= 9; line++ {
			w, ok := want[line]
			if !ok {
				continue
			}
			got, found := enclosingSymbol(syms, line, nil)
			if !found || got.Name != w {
				t.Errorf("input order %s, %s: enclosingSymbol(line=%d) = (%q, %v), want %q", nodes[0].Name, nodes[1].Name, line, got.Name, found, w)
			}
		}
	}
}

// TestEnclosingSymbol_FallbackNarrowing is the MAJOR finding's fix: with
// nodes carrying EndLine:0 exactly like the real Go extractor emits today,
// a hit landing in a doc comment or a blank run BEFORE the next symbol
// must not be attributed to the symbol above it — honest absence, not a
// guess.
func TestEnclosingSymbol_FallbackNarrowing(t *testing.T) {
	// A blank line, then a nested doc-comment run, separate B from A —
	// both scenarios must narrow A's naive "up to B.Line-1" fallback.
	syms := []core.SymbolNode{
		{Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 3, EndLine: 0},
		{Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 8, EndLine: 0}, // open-ended, last in file
	}
	lines := []string{
		"package pkg", // 1
		"",            // 2
		"func A() {",  // 3
		"\treturn",    // 4
		"}",           // 5
		"",            // 6 <- blank run: A's fallback body ends at 5
		"// B docs",   // 7 <- B's own doc comment, not A's body
		"func B() {",  // 8
	}
	cases := []struct {
		line     int
		wantName string
		wantOK   bool
	}{
		{3, "A", true},   // A's opening line
		{5, "A", true},   // A's last real line before the blank run
		{6, "", false},   // the blank run itself — unattributable
		{7, "", false},   // B's own doc comment — not A's body, and B has not started
		{8, "B", true},   // B's opening line
		{100, "B", true}, // deep inside B (open-ended: last symbol, unaffected by narrowing)
	}
	for _, c := range cases {
		got, ok := enclosingSymbol(syms, c.line, lines)
		if ok != c.wantOK || (ok && got.Name != c.wantName) {
			t.Errorf("enclosingSymbol(line=%d) = (%q, %v), want (%q, %v)", c.line, got.Name, ok, c.wantName, c.wantOK)
		}
	}
}

// TestEnclosingSymbol_FallbackNarrowing_CommentRunWithoutBlankLine isolates
// the comment-only-run half of narrowFallbackSpan (no blank line at all
// between the two symbols — a doc comment can immediately follow the
// previous symbol's closing brace).
func TestEnclosingSymbol_FallbackNarrowing_CommentRunWithoutBlankLine(t *testing.T) {
	syms := []core.SymbolNode{
		{Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 3, EndLine: 0},
		{Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 8, EndLine: 0},
	}
	lines := []string{
		"package pkg",    // 1
		"",               // 2
		"func A() {",     // 3
		"\treturn",       // 4
		"}",              // 5
		"// B docs",      // 6 <- immediately follows A's closing brace
		"// more B docs", // 7
		"func B() {",     // 8
	}
	cases := []struct {
		line     int
		wantName string
		wantOK   bool
	}{
		{5, "A", true}, // A's last real line
		{6, "", false}, // B's doc comment run — not A's body
		{7, "", false}, // still B's doc comment run
	}
	for _, c := range cases {
		got, ok := enclosingSymbol(syms, c.line, lines)
		if ok != c.wantOK || (ok && got.Name != c.wantName) {
			t.Errorf("enclosingSymbol(line=%d) = (%q, %v), want (%q, %v)", c.line, got.Name, ok, c.wantName, c.wantOK)
		}
	}
}

// TestEnclosingSymbol_FallbackNarrowing_UnreadableFileKeepsNaiveBoundary
// proves the degrade: lines==nil (the file could not be read) leaves the
// pre-fix, wider "up to the next symbol" boundary in place — nothing to
// narrow against, so this is not a regression for that case.
func TestEnclosingSymbol_FallbackNarrowing_UnreadableFileKeepsNaiveBoundary(t *testing.T) {
	syms := []core.SymbolNode{
		{Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 3, EndLine: 0},
		{Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 8, EndLine: 0},
	}
	got, ok := enclosingSymbol(syms, 6, nil)
	if !ok || got.Name != "A" {
		t.Errorf("enclosingSymbol(line=6, lines=nil) = (%q, %v), want (\"A\", true) — unreadable file keeps the naive boundary", got.Name, ok)
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

	hits, engine, _, err := searchRun(context.Background(), root, searchOpts{pattern: "needle", caseSensitive: true})
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

// hitFiles collects the File of each raw hit, for a compact assertion.
func hitFiles(hits []searchRawHit) []string {
	files := make([]string, 0, len(hits))
	for _, h := range hits {
		files = append(files, h.File)
	}
	return files
}

// ─── CRITICAL (V-review): search must mirror the service's .fathomignore
// exactly as ingest/read_source do, on BOTH engines — a file ingest and
// read_source refuse must never be served by search. ──────────────────────

// TestSearchIgnore_FathomignoredDir_RefusedBothEngines proves the fix
// directly on both engines: a service's own .fathomignore excluding a
// directory must be honored by searchWalk (the Go-regexp fallback) AND by
// ripgrep's own -g excludes built from the identical raw patterns — not
// just whichever engine happens to answer a given call.
func TestSearchIgnore_FathomignoredDir_RefusedBothEngines(t *testing.T) {
	t.Setenv("FATHOM_IGNORE_D", t.TempDir()) // isolate: no real service-scoped file leaks in
	root := t.TempDir()
	writeFlagFixture(t, root, ".fathomignore", "secrets/\n")
	writeFlagFixture(t, root, "pkg/a.go", "package pkg\n\nfunc Needle() {}\n")
	writeFlagFixture(t, root, "secrets/prod/keys.go", "package prod\n\nfunc Needle() {}\n")

	ignore, patterns, _, err := connector.LoadIgnoreScopedForWithPatterns(root, "svc")
	if err != nil {
		t.Fatalf("LoadIgnoreScopedForWithPatterns: %v", err)
	}
	if ignore == nil {
		t.Fatal("ignore is nil, want a compiled Matcher for the seeded .fathomignore")
	}

	walkHits, _, err := searchWalk(context.Background(), root, searchOpts{pattern: "Needle", caseSensitive: true, ignore: ignore})
	if err != nil {
		t.Fatalf("searchWalk: %v", err)
	}
	if got := hitFiles(walkHits); len(got) != 1 || got[0] != "pkg/a.go" {
		t.Fatalf("searchWalk hits = %v, want exactly [pkg/a.go] (secrets/ excluded)", got)
	}

	if _, lerr := exec.LookPath("rg"); lerr != nil {
		t.Skip("rg not on PATH — ripgrep engine not exercised directly here")
	}
	rgHits, _, err := searchRipgrep(context.Background(), root, searchOpts{pattern: "Needle", caseSensitive: true, ignorePatterns: patterns})
	if err != nil {
		t.Fatalf("searchRipgrep: %v", err)
	}
	if got := hitFiles(rgHits); len(got) != 1 || got[0] != "pkg/a.go" {
		t.Fatalf("searchRipgrep hits = %v, want exactly [pkg/a.go] (secrets/ excluded)", got)
	}
}

// searchIgnoreTestBackend wraps elisionFakeBackend, overriding ServiceMeta
// to record a real corpus root — the minimal QueryInterface search's
// service branch needs beyond ListAllSymbols (already on the embedded
// fake).
type searchIgnoreTestBackend struct {
	elisionFakeBackend
	svcPath string
}

func (b *searchIgnoreTestBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return b.svcPath, "", true, nil
}

// TestSearchIgnore_LoadErrorFailsClosed proves the fail-closed contract:
// when the scoped loader errors (a user-scope re-include, the same
// *ErrUserScopeReinclude LoadIgnoreScopedFor already refuses), the tool
// handler must REFUSE the search rather than run it without exclusions.
func TestSearchIgnore_LoadErrorFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_IGNORE_D", t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A user-scope re-include is the one thing LoadIgnoreScopedFor refuses
	// loudly (law 13) — the fail-closed path this test exercises.
	if err := os.WriteFile(filepath.Join(home, ".fathom", "ignore"), []byte("!reincluded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeFlagFixture(t, root, "pkg/a.go", "package pkg\n\nfunc Needle() {}\n")

	srv := NewServer(&searchIgnoreTestBackend{svcPath: root})
	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "search", "arguments": map[string]any{"service": "svc", "pattern": "Needle"}},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, hasResp := srv.HandleRequest(context.Background(), raw)
	if !hasResp {
		t.Fatal("search: no response")
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode envelope: %v (raw=%s)", err, resp)
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected search to refuse when the service's exclusions fail to load, got success: %v", env)
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "exclusions could not be loaded") {
		t.Errorf("error message = %q, want it to name the load failure", msg)
	}
}

// ─── MINOR (V-review): the ripgrep path lacked searchWalk's own .env
// exclusion and symlink safety. ─────────────────────────────────────────

// TestSearchEnvFile_RefusedBothEngines proves both engines refuse a
// .env-pattern file the same way read_source does — the ripgrep path had
// no equivalent to searchWalk's isEnvPatternFile check before this fix.
func TestSearchEnvFile_RefusedBothEngines(t *testing.T) {
	root := t.TempDir()
	writeFlagFixture(t, root, "prod.env", "API_KEY=Needle\n")
	writeFlagFixture(t, root, "pkg/a.go", "// Needle marker\n")

	walkHits, _, err := searchWalk(context.Background(), root, searchOpts{pattern: "Needle", caseSensitive: true})
	if err != nil {
		t.Fatalf("searchWalk: %v", err)
	}
	if got := hitFiles(walkHits); len(got) != 1 || got[0] != "pkg/a.go" {
		t.Fatalf("searchWalk hits = %v, want exactly [pkg/a.go] (prod.env excluded)", got)
	}

	if _, lerr := exec.LookPath("rg"); lerr != nil {
		t.Skip("rg not on PATH — ripgrep engine not exercised directly here")
	}
	rgHits, _, err := searchRipgrep(context.Background(), root, searchOpts{pattern: "Needle", caseSensitive: true})
	if err != nil {
		t.Fatalf("searchRipgrep: %v", err)
	}
	if got := hitFiles(rgHits); len(got) != 1 || got[0] != "pkg/a.go" {
		t.Fatalf("searchRipgrep hits = %v, want exactly [pkg/a.go] (prod.env excluded)", got)
	}
}

// TestSearchWalk_SkipsSymlinks proves searchWalk never follows a symlink —
// it can resolve outside root entirely, exactly what read_source refuses.
func TestSearchWalk_SkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFlagFixture(t, outside, "secret.txt", "Needle outside root\n")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}
	writeFlagFixture(t, root, "pkg/a.go", "// Needle real\n")

	hits, _, err := searchWalk(context.Background(), root, searchOpts{pattern: "Needle", caseSensitive: true})
	if err != nil {
		t.Fatalf("searchWalk: %v", err)
	}
	if got := hitFiles(hits); len(got) != 1 || got[0] != "pkg/a.go" {
		t.Fatalf("hits = %v, want exactly [pkg/a.go] (the symlink must never be followed)", got)
	}
}

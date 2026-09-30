package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const draftTestSource = "nvim-treesitter/nvim-treesitter runtime/queries/fixture/locals.scm @ 0123456789abcdef0123456789abcdef01234567 (Apache-2.0)"

func draftFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "langdraft", name)
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runLangDraft runs cmdLangDraft and returns its exit code and both streams.
func runLangDraft(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = cmdLangDraft(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestCmdLangDraft (LB-F47, design §13.2 step 11): a locals.scm becomes a
// draft tags.scm — one definition per kept pattern as @name with
// @definition.<kind> on its outer node — and a draft imports.scm, both under
// the §8.2 header. Goldens are written by hand from the fixture.
func TestCmdLangDraft(t *testing.T) {
	t.Run("golden", func(t *testing.T) {
		dir := t.TempDir()
		tags, imports := filepath.Join(dir, "tags.scm"), filepath.Join(dir, "imports.scm")
		code, out, errb := runLangDraft("-locals", draftFixture(t, "shapes.locals.scm"), "-source", draftTestSource, "-out", tags, "-imports-out", imports)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errb)
		}
		if got, want := mustReadFile(t, tags), mustReadFile(t, draftFixture(t, "shapes.tags.golden.scm")); got != want {
			t.Errorf("tags draft differs from the golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
		}
		if got, want := mustReadFile(t, imports), mustReadFile(t, draftFixture(t, "shapes.imports.golden.scm")); got != want {
			t.Errorf("imports draft differs from the golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
		}
		for _, want := range []string{
			"lang-draft: 19 patterns: kept 11, imports 1, dropped 7, flagged 2\n",
			"skipped L39: untyped definition: no kind for node type declQuark\n",
			"skipped L71: several definitions in one pattern\n",
			"flagged L47: #lua-match?\n",
			"flagged L52: #has-ancestor?\n",
			"inherits: c\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout lacks %q:\n%s", want, out)
			}
		}
	})

	t.Run("header is section 8.2's, and names the source", func(t *testing.T) {
		dir := t.TempDir()
		tags := filepath.Join(dir, "tags.scm")
		if code, _, errb := runLangDraft("-locals", draftFixture(t, "shapes.locals.scm"), "-source", draftTestSource, "-out", tags); code != 0 {
			t.Fatalf("exit %d: %s", code, errb)
		}
		lines := strings.Split(mustReadFile(t, tags), "\n")
		if lines[0] != "; source: "+draftTestSource {
			t.Errorf("line 1 = %q", lines[0])
		}
		if !strings.HasPrefix(lines[1], "; changes: drafted by indexbench lang-draft (locals2tags): ") ||
			!strings.Contains(lines[1], "a draft to tune, never an L1 oracle (ADR-048 amendment)") {
			t.Errorf("line 2 = %q", lines[1])
		}
		if _, err := os.Stat(filepath.Join(dir, "imports.scm")); err == nil {
			t.Error("imports draft written without -imports-out")
		}
	})

	t.Run("output is byte-stable", func(t *testing.T) {
		dir := t.TempDir()
		var first string
		for i := 0; i < 3; i++ {
			tags := filepath.Join(dir, "tags.scm")
			if code, _, errb := runLangDraft("-locals", draftFixture(t, "shapes.locals.scm"), "-source", draftTestSource, "-out", tags); code != 0 {
				t.Fatalf("exit %d: %s", code, errb)
			}
			got := mustReadFile(t, tags)
			if i == 0 {
				first = got
			} else if got != first {
				t.Fatalf("run %d differs from run 0", i)
			}
		}
	})

	t.Run("a bad -source is refused before anything is written", func(t *testing.T) {
		const rev = "0123456789abcdef0123456789abcdef01234567"
		for _, tc := range []struct{ name, source, want string }{
			{"GPL licence", "example/locals runtime/x/locals.scm @ " + rev + " (GPL-3.0-only)", "GPL-3.0-only"},
			{"AGPL licence", "example/locals runtime/x/locals.scm @ " + rev + " (AGPL-3.0-only)", "AGPL-3.0-only"},
			{"MPL-2.0 (a draft modifies the file)", "example/locals runtime/x/locals.scm @ " + rev + " (MPL-2.0)", "MPL-2.0"},
			{"unknown licence", "example/locals runtime/x/locals.scm @ " + rev + " (Whatever)", "Whatever"},
			{"tag, not a commit", "example/locals runtime/x/locals.scm @ v1.2.3 (MIT)", "40-hex"},
			{"short commit", "example/locals runtime/x/locals.scm @ 0123456 (MIT)", "40-hex"},
			{"upper-case commit", "example/locals runtime/x/locals.scm @ 0123456789ABCDEF0123456789ABCDEF01234567 (MIT)", "40-hex"},
			{"no licence", "example/locals runtime/x/locals.scm @ " + rev, "-source"},
			{"no revision", "example/locals runtime/x/locals.scm (MIT)", "-source"},
			{"empty", " ", "-source"},
		} {
			dir := t.TempDir()
			tags := filepath.Join(dir, "tags.scm")
			code, _, errb := runLangDraft("-locals", draftFixture(t, "shapes.locals.scm"), "-source", tc.source, "-out", tags)
			if code != 2 {
				t.Errorf("%s: exit %d, want 2 (%s)", tc.name, code, errb)
			}
			if !strings.Contains(errb, tc.want) {
				t.Errorf("%s: stderr %q lacks %q", tc.name, errb, tc.want)
			}
			if _, err := os.Stat(tags); err == nil {
				t.Errorf("%s: a draft was written", tc.name)
			}
		}
		// Every allowlisted licence except MPL-2.0 is accepted.
		for _, spdx := range []string{"MIT", "Apache-2.0", "BSD-3-Clause", "ISC", "Unlicense", "CC0-1.0", "0BSD"} {
			tags := filepath.Join(t.TempDir(), "tags.scm")
			if code, _, errb := runLangDraft("-locals", draftFixture(t, "shapes.locals.scm"), "-source", "example/locals runtime/x/locals.scm @ "+rev+" ("+spdx+")", "-out", tags); code != 0 {
				t.Errorf("%s refused: exit %d: %s", spdx, code, errb)
			}
		}
	})

	t.Run("usage errors", func(t *testing.T) {
		dir := t.TempDir()
		tags := filepath.Join(dir, "tags.scm")
		locals := draftFixture(t, "shapes.locals.scm")
		for name, args := range map[string][]string{
			"no -locals":       {"-source", draftTestSource, "-out", tags},
			"no -source":       {"-locals", locals, "-out", tags},
			"no -out":          {"-locals", locals, "-source", draftTestSource},
			"a stray argument": {"-locals", locals, "-source", draftTestSource, "-out", tags, "extra"},
			"missing locals":   {"-locals", filepath.Join(dir, "nope.scm"), "-source", draftTestSource, "-out", tags},
			"out is locals":    {"-locals", locals, "-source", draftTestSource, "-out", locals},
			"same two outputs": {"-locals", locals, "-source", draftTestSource, "-out", tags, "-imports-out", tags},
		} {
			if code, _, _ := runLangDraft(args...); code != 2 {
				t.Errorf("%s: exit %d, want 2", name, code)
			}
		}
		if code, _, _ := runLangDraft("-nope"); code != 2 {
			t.Errorf("unknown flag: exit %d, want 2", code)
		}
	})

	t.Run("-dialect compiles the draft against the grammar", func(t *testing.T) {
		dir := t.TempDir()
		tags, imports := filepath.Join(dir, "tags.scm"), filepath.Join(dir, "imports.scm")
		code, out, errb := runLangDraft("-locals", draftFixture(t, "python.locals.scm"), "-source", draftTestSource, "-out", tags, "-imports-out", imports, "-dialect", "python")
		if code != 0 {
			t.Fatalf("a draft from locals-style python patterns did not compile: exit %d: %s", code, errb)
		}
		if !strings.Contains(out, "compile: python ok") {
			t.Errorf("stdout lacks the compile line:\n%s", out)
		}
		got := mustReadFile(t, tags)
		for _, want := range []string{
			"(#match? @name \"^[A-Za-z_][A-Za-z0-9_]*$\")",
			"(class_definition\n  name: (identifier) @name) @definition.type",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("python draft lacks %q:\n%s", want, got)
			}
		}
	})

	t.Run("-dialect names the first compile error, exits 1 and still writes the draft", func(t *testing.T) {
		dir := t.TempDir()
		tags := filepath.Join(dir, "tags.scm")
		code, _, errb := runLangDraft("-locals", draftFixture(t, "python-unsupported.locals.scm"), "-source", draftTestSource, "-out", tags, "-dialect", "python")
		if code != 1 {
			t.Fatalf("exit %d, want 1: %s", code, errb)
		}
		if !strings.Contains(errb, "#has-ancestor?") {
			t.Errorf("stderr does not name the predicate: %q", errb)
		}
		got := mustReadFile(t, tags)
		if !strings.Contains(got, "; UNSUPPORTED predicate #has-ancestor?: rewrite before use") {
			t.Errorf("the draft does not flag the predicate:\n%s", got)
		}
	})

	t.Run("-dialect unknown is a usage error but the draft is written", func(t *testing.T) {
		tags := filepath.Join(t.TempDir(), "tags.scm")
		code, _, errb := runLangDraft("-locals", draftFixture(t, "python.locals.scm"), "-source", draftTestSource, "-out", tags, "-dialect", "cobol")
		if code != 2 || !strings.Contains(errb, "cobol") {
			t.Errorf("exit %d, stderr %q", code, errb)
		}
		if _, err := os.Stat(tags); err != nil {
			t.Errorf("draft not written: %v", err)
		}
	})
}

// draftOne runs the converter over src and returns the rendered tags blocks
// joined (skip notes included) and the result for counts and notes.
func draftOne(src string) (string, draftResult) {
	r := draftFromLocals(src)
	return strings.Join(r.Tags, "\n---\n"), r
}

// TestDraftFromLocalsKinds: the kind mapping — the kinds that keep their
// names, namespace to module, type to type — and the kinds that are dropped.
func TestDraftFromLocalsKinds(t *testing.T) {
	for _, tc := range []struct{ local, want string }{
		{"function", "function"}, {"method", "method"}, {"class", "class"}, {"interface", "interface"},
		{"enum", "enum"}, {"macro", "macro"}, {"constant", "constant"},
		{"namespace", "module"}, {"type", "type"},
	} {
		got, r := draftOne("(decl name: (identifier) @local.definition." + tc.local + ")\n")
		want := "; from locals L1: @local.definition." + tc.local + "\n(decl name: (identifier) @name) @definition." + tc.want
		if got != want || r.Kept != 1 {
			t.Errorf("%s: got %q kept=%d, want %q", tc.local, got, r.Kept, want)
		}
	}
	for _, dropped := range []string{"var", "parameter", "field", "associated", "unheard-of"} {
		got, r := draftOne("(decl name: (identifier) @local.definition." + dropped + ")\n")
		if got != "" || r.Kept != 0 || r.Dropped != 1 || len(r.Notes) != 0 {
			t.Errorf("%s: got %q kept=%d dropped=%d notes=%v; want a silent drop", dropped, got, r.Kept, r.Dropped, r.Notes)
		}
	}
}

// TestDraftFromLocalsShapes: where @definition.<kind> lands and what is
// left alone.
func TestDraftFromLocalsShapes(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{
			"a sequence pattern puts the kind on the node holding the definition",
			"((comment) @doc . (function_definition name: (identifier) @local.definition.function))\n",
			"; from locals L1: @local.definition.function\n((comment) @doc . (function_definition name: (identifier) @name) @definition.function)",
		},
		{
			"a capture on the pattern's own node",
			"(identifier) @local.definition.constant\n",
			"; from locals L1: @local.definition.constant\n(identifier) @definition.constant @name",
		},
		{
			"a trailing scope on its own line goes with its line",
			"(function_definition\n  name: (identifier) @local.definition.function\n  body: (block))\n  @local.scope\n",
			"; from locals L1: @local.definition.function\n(function_definition\n  name: (identifier) @name\n  body: (block)) @definition.function",
		},
		{
			"scope and reference captures inside a kept pattern go too",
			"(function_definition name: (identifier) @local.definition.function @local.reference) @local.scope @local.reference\n",
			"; from locals L1: @local.definition.function\n(function_definition name: (identifier) @name) @definition.function",
		},
		{
			"comments, captures that are not locals ones and quantifiers stay",
			"(class_definition ; the class\n  name: (identifier) @local.definition.class\n  (decorator)* @deco) @keep\n",
			"; from locals L1: @local.definition.class\n(class_definition ; the class\n  name: (identifier) @name\n  (decorator)* @deco) @definition.class @keep",
		},
		{
			"a predicate on the definition capture follows the rename",
			"((identifier) @local.definition.function (#any-of? @local.definition.function \"a\" \"b\"))\n",
			"; from locals L1: @local.definition.function\n((identifier) @definition.function @name (#any-of? @name \"a\" \"b\"))",
		},
		{
			"an alternation inside the pattern is left whole",
			"(call function: [(identifier) (attribute)] @local.definition.function)\n",
			"; from locals L1: @local.definition.function\n(call function: [(identifier) (attribute)] @name) @definition.function",
		},
		{
			"a top-level string with a definition",
			"\"main\" @local.definition.function\n",
			"; from locals L1: @local.definition.function\n\"main\" @definition.function @name",
		},
	} {
		got, r := draftOne(tc.in)
		if got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
		if r.Kept != 1 {
			t.Errorf("%s: kept %d", tc.name, r.Kept)
		}
	}

	for _, tc := range []struct{ name, in, reason string }{
		{"several definitions", "(f name: (identifier) @local.definition.function (p) @local.definition.method)\n", "several definitions in one pattern"},
		{"already a tags pattern", "(class_definition name: (identifier) @name) @definition.class\n", "already carries @name"},
		{"no outer node", "_ @local.definition.function\n", "no outer node"},
		{"unbalanced", "(function_definition name: (identifier) @local.definition.function\n", "unbalanced"},
		{"untyped, no kind", "(declQuark name: (identifier) @local.definition)\n", "no kind for node type declQuark"},
		{"untyped, wildcard node", "((_) @local.definition)\n", "no kind for node type _"},
	} {
		got, r := draftOne(tc.in)
		if !strings.HasPrefix(got, "; SKIPPED L1: ") || !strings.Contains(got, tc.reason) || strings.Contains(got, "\n") {
			t.Errorf("%s: got %q, want one SKIPPED comment line with %q", tc.name, got, tc.reason)
		}
		if r.Kept != 0 || r.Dropped != 1 || len(r.Notes) != 1 || r.Notes[0].Kind != "skipped" || !strings.Contains(r.Notes[0].Msg, tc.reason) || r.Notes[0].Line != 1 {
			t.Errorf("%s: kept=%d dropped=%d notes=%+v; want one skipped note with %q", tc.name, r.Kept, r.Dropped, r.Notes, tc.reason)
		}
	}

	if r := draftFromLocals(""); r.Kept+r.Dropped+r.ImportCount != 0 {
		t.Errorf("empty input: %+v", r)
	}
	if r := draftFromLocals("; only a comment\n"); r.Kept+r.Dropped != 0 {
		t.Errorf("comment-only input: %+v", r)
	}
	if r := draftFromLocals(") ] (module) @local.scope\n"); r.Dropped != 1 {
		t.Errorf("stray closers: %+v", r)
	}
}

// TestDraftUntypedKinds: an untyped @local.definition takes its kind from the
// outer node's type.
func TestDraftUntypedKinds(t *testing.T) {
	for node, want := range map[string]string{
		"declProc": "function", "procedure_specification": "function", "function_declaration": "function",
		"method_definition": "function", "subprogram_body": "function",
		"declEnumValue": "enum_member", "enumeration_literal_specification": "enum_member", "enum_entry": "enum_member",
		"declEnum": "enum", "enum_declaration": "enum",
		"declType": "type", "class_declaration": "type", "record_type": "type", "struct_item": "type",
		"declConstructor": "constructor", "constructor_declaration": "constructor",
		"declModule": "module", "package_declaration": "module", "unit_definition": "module",
		"declConst": "constant", "const_item": "constant",
		"macro_definition": "macro",
	} {
		got, _ := draftOne("(" + node + " name: (identifier) @local.definition)\n")
		if !strings.HasSuffix(got, " @definition."+want) {
			t.Errorf("%s: got %q, want kind %s", node, got, want)
		}
	}
	for _, node := range []string{"declVar", "declQuark", "identifier", "block"} {
		if got, r := draftOne("(" + node + " name: (identifier) @local.definition)\n"); r.Kept != 0 || r.Dropped != 1 || !strings.HasPrefix(got, "; SKIPPED L1: untyped definition: no kind for node type "+node) {
			t.Errorf("%s: got %q kept=%d, want it skipped with a note", node, got, r.Kept)
		}
	}
}

// TestDraftPredicates: supported predicates stay, ignored ones stay, lua-match
// is rewritten when mechanical, anything else is flagged above the pattern.
func TestDraftPredicates(t *testing.T) {
	pat := func(pred string) string {
		return "((function_definition name: (identifier) @local.definition.function)\n " + pred + ")\n"
	}
	keep := []string{
		`(#eq? @local.definition.function "a")`, `(#not-eq? @local.definition.function "a")`,
		`(#match? @local.definition.function "^a")`, `(#not-match? @local.definition.function "^a")`,
		`(#any-of? @local.definition.function "a" "b")`, `(#not-any-of? @local.definition.function "a")`,
		`(#set! definition.function.scope "parent")`, `(#strip! @local.definition.function "x")`,
		`(#select-adjacent! @local.definition.function @doc)`, `(#offset! @local.definition.function 0 1 0 0)`,
	}
	for _, p := range keep {
		got, r := draftOne(pat(p))
		if r.Flagged != 0 || strings.Contains(got, "UNSUPPORTED") {
			t.Errorf("%s flagged:\n%s", p, got)
		}
		if want := strings.ReplaceAll(p, "@local.definition.function", "@name"); !strings.Contains(got, want) {
			t.Errorf("%s not kept as %s:\n%s", p, want, got)
		}
	}

	for _, name := range []string{"#has-ancestor?", "#has-parent?", "#kind-eq?", "#is-not?", "#not-has-ancestor?", "#vim-match?", "#any-eq?"} {
		got, r := draftOne(pat("(" + name + " @local.definition.function foo)"))
		wantLine := "; UNSUPPORTED predicate " + name + ": rewrite before use\n((function_definition"
		if !strings.Contains(got, wantLine) || r.Flagged != 1 || r.Kept != 1 {
			t.Errorf("%s: flagged=%d kept=%d\n%s", name, r.Flagged, r.Kept, got)
		}
		if len(r.Notes) != 1 || r.Notes[0].Kind != "flagged" || r.Notes[0].Msg != name {
			t.Errorf("%s: notes %+v", name, r.Notes)
		}
	}

	// Two different unsupported predicates: two lines, one flagged pattern;
	// the same one twice: one line.
	got, r := draftOne("((function_definition name: (identifier) @local.definition.function)\n (#has-parent? @local.definition.function a)\n (#kind-eq? @local.definition.function b)\n (#has-parent? @local.definition.function c))\n")
	if strings.Count(got, "; UNSUPPORTED predicate #has-parent?") != 1 || strings.Count(got, "; UNSUPPORTED predicate #kind-eq?") != 1 || r.Flagged != 1 {
		t.Errorf("several unsupported predicates:\n%s flagged=%d", got, r.Flagged)
	}
	if strings.Index(got, "#has-parent?: rewrite") > strings.Index(got, "#kind-eq?: rewrite") {
		t.Errorf("unsupported lines are not in order of appearance:\n%s", got)
	}

	// #lua-match? and its negation.
	for _, tc := range []struct{ in, want string }{
		{`(#lua-match? @local.definition.function "^[%a_]")`, `(#match? @name "^[A-Za-z_]")`},
		{`(#not-lua-match? @local.definition.function "^%d")`, `(#not-match? @name "^[0-9]")`},
		{`(#lua-match? @local.definition.function "%.lua$")`, `(#match? @name "\\.lua$")`},
		{`(#lua-match? @local.definition.function "^%s*$")`, `(#match? @name "^[ \\t\\n\\v\\f\\r]*$")`},
	} {
		got, r := draftOne(pat(tc.in))
		if !strings.Contains(got, tc.want) || r.Flagged != 0 {
			t.Errorf("%s:\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
	for _, in := range []string{
		`(#lua-match? @local.definition.function "%f[%a]x")`,
		`(#lua-match? @local.definition.function "a-b")`,
		`(#lua-match? @local.definition.function "(a)")`,
		`(#lua-match? @local.definition.function "a" "b")`,
		`(#lua-match? "^a" @local.definition.function)`,
		`(#lua-match? @local.definition.function)`,
	} {
		got, r := draftOne(pat(in))
		if !strings.Contains(got, "; UNSUPPORTED predicate #lua-match?: rewrite before use") || r.Flagged != 1 {
			t.Errorf("%s not flagged:\n%s", in, got)
		}
		if !strings.Contains(got, "(#lua-match?") {
			t.Errorf("%s: the predicate was rewritten though it does not map:\n%s", in, got)
		}
	}
}

// TestLuaPatternToRE2: the Lua patterns that map mechanically to RE2 — and
// behave alike on sample text — and those that must be refused.
func TestLuaPatternToRE2(t *testing.T) {
	for _, tc := range []struct {
		lua, re string
		match   []string
		nomatch []string
	}{
		{"^[A-Z]", "^[A-Z]", []string{"Foo"}, []string{"foo"}},
		{"^[%a_][%w_]*$", "^[A-Za-z_][A-Za-z0-9_]*$", []string{"_x1", "Abc"}, []string{"1x", "a-b"}},
		{"%d+", "[0-9]+", []string{"a12"}, []string{"abc"}},
		{"^%u%l*$", "^[A-Z][a-z]*$", []string{"Foo"}, []string{"FOO", "foo"}},
		{"%.lua$", `\.lua$`, []string{"x.lua"}, []string{"xlua", "x.lua2"}},
		{"^%s*$", `^[ \t\n\v\f\r]*$`, []string{"", "  "}, []string{"a"}},
		{"%p", "[[:punct:]]", []string{"a!"}, []string{"ab"}},
		{"[%p%s]", `[[:punct:] \t\n\v\f\r]`, []string{"a b"}, []string{"ab"}},
		{"[%a%d]+", "[A-Za-z0-9]+", []string{"x"}, []string{"-"}},
		{"[^%d]", "[^0-9]", []string{"a"}, []string{"1"}},
		{"[a-f]x?", "[a-f]x?", []string{"b"}, []string{"g"}},
		{"[]]", `[\]]`, []string{"]"}, []string{"a"}},
		{"[^]]", `[^\]]`, []string{"a"}, []string{"]"}},
		{"[-a]", `[\-a]`, []string{"-"}, []string{"b"}},
		{"[a-]", `[a\-]`, []string{"-"}, []string{"b"}},
		{"a|b", `a\|b`, []string{"a|b"}, []string{"a", "b"}},
		{"a{2}", `a\{2\}`, []string{"a{2}"}, []string{"aa"}},
		{"a^b$c", `a\^b\$c`, []string{"a^b$c"}, []string{"abc"}},
		{"%%", `\%`, []string{"50%"}, []string{"50"}},
		{`a\b`, `a\\b`, []string{`a\b`}, []string{"ab"}},
		{"^get_", "^get_", []string{"get_x"}, []string{"forget_x"}},
		{".", ".", []string{"a"}, []string{""}},
		{"", "", []string{"anything"}, nil},
	} {
		got, ok := luaPatternToRE2(tc.lua)
		if !ok || got != tc.re {
			t.Errorf("%q: got %q ok=%v, want %q", tc.lua, got, ok, tc.re)
			continue
		}
		re, err := regexp.Compile(got)
		if err != nil {
			t.Errorf("%q -> %q does not compile: %v", tc.lua, got, err)
			continue
		}
		for _, s := range tc.match {
			if !re.MatchString(s) {
				t.Errorf("%q -> %q should match %q", tc.lua, got, s)
			}
		}
		for _, s := range tc.nomatch {
			if re.MatchString(s) {
				t.Errorf("%q -> %q should not match %q", tc.lua, got, s)
			}
		}
	}
	for _, lua := range []string{
		"%f[%a]", "%b()", "%1", "%A", "%D", "%W", "%S", "%x", "%c", "%g", "%", // classes RE2 has no mechanical form of
		"a-", "a-b", ".-x", // Lua's lazy repetition
		"(a)", "()", "a)", // captures
		"*a", "+a", "?a", "^*", "a**", "a+*", // a quantifier with nothing to repeat
		"[a", "[%", "[z-a]", "[a-%d]", // malformed or unmappable sets
		"é", "a%é", // non-ASCII bytes (a quantifier would bind to one byte in Lua and one rune in RE2)
	} {
		if got, ok := luaPatternToRE2(lua); ok {
			t.Errorf("%q mapped to %q, want a refusal", lua, got)
		}
	}
}

// TestDraftTokenizer: strings with escaped quotes and comments containing
// brackets do not confuse pattern boundaries.
func TestDraftTokenizer(t *testing.T) {
	src := "; ( a comment with (parens\n(a name: (identifier) @local.definition.function (#eq? @local.definition.function \"x\\\"(\")) ; ) trailing\n(b)\n"
	r := draftFromLocals(src)
	if r.Kept != 1 || r.Dropped != 1 || len(r.Tags) != 1 {
		t.Fatalf("kept %d dropped %d, want 1 and 1: %+v", r.Kept, r.Dropped, r)
	}
	want := "; from locals L2: @local.definition.function\n(a name: (identifier) @name (#eq? @name \"x\\\"(\")) @definition.function"
	if r.Tags[0] != want {
		t.Errorf("got %q, want %q", r.Tags[0], want)
	}
}

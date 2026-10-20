package extractor

import (
	"bytes"
	"strings"
	"testing"
)

// The css dialect reads a file that carries a Tailwind CSS v4 at-rule with its
// Tailwind variant, and every other stylesheet with its own queries
// (LB-Q-tailwind).
func TestTailwindVariantSelectedByContent(t *testing.T) {
	e, err := NewTSExtractor("css")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"theme", "@theme {\n  --color-a: red;\n}\n", true},
		{"utility", "@utility foo { color: red; }\n", true},
		{"custom variant", "@custom-variant dark (&:where(.dark *));\n", true},
		{"apply in a rule", ".a { @apply p-4; }\n", true},
		{"source", "@import \"x.css\";\n@source \"../src\";\n", true},
		{"plain stylesheet", ":root { --a: 1; }\n.b { color: var(--a); }\n@media (min-width: 1px) { .b { color: red; } }\n", false},
		{"v3 directives", "@tailwind base;\n.c { color: red; }\n", false},
		{"word inside a name", ".theme-dark { color: red; }\n.x::after { content: \"@theme\"; }\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		got := e.variantFor("a.css", []byte(c.src)).id == "tailwind"
		if got != c.want {
			t.Errorf("%s: tailwind variant = %v, want %v", c.name, got, c.want)
		}
	}
	// An extension no css claim reaches never takes a sniffed variant.
	if v := e.variantFor("a.scss", []byte("@theme { --a: 1; }")); v.id != "" {
		t.Errorf(".scss took variant %q", v.id)
	}
}

// A plain stylesheet's custom property stays CSSCustomProperty; the same
// declaration in @theme is a TailwindTheme and nothing else (one definition per
// node).
func TestTailwindThemeIsOneDefinition(t *testing.T) {
	e, err := NewTSExtractor("css")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	kindsOf := func(src string) map[string][]string {
		nodes, err := e.Extract("svc", "a.css", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string][]string{}
		for _, n := range nodes {
			out[n.Name] = append(out[n.Name], string(n.Kind))
		}
		return out
	}
	got := kindsOf(":root { --a: 1; }\n")
	if k := got["--a"]; len(k) != 1 || k[0] != "CSSCustomProperty" {
		t.Errorf("plain --a = %v", k)
	}
	got = kindsOf("@theme {\n  --a: 1;\n}\n:root { --b: 2; }\n")
	if k := got["--a"]; len(k) != 1 || k[0] != "TailwindTheme" {
		t.Errorf("@theme --a = %v, want one TailwindTheme", k)
	}
	if k := got["--b"]; len(k) != 1 || k[0] != "CSSCustomProperty" {
		t.Errorf(":root --b beside @theme = %v", k)
	}
}

// The shorthand @custom-variant does not swallow the rule after it, a
// functional @utility keeps its written name up to the star, a namespace reset
// is no theme variable, and @apply reads each candidate.
func TestTailwindV4Spellings(t *testing.T) {
	e, err := NewTSExtractor("css")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := "@theme {\n  --color-*: initial;\n  --color-a: red;\n}\n@utility tab-* {\n  tab-size: --value(integer);\n}\n@custom-variant dark (&:where(.dark, .dark *));\n@variant foo (&:hover);\n.btn {\n  @apply tab-4 card;\n  @variant dark {\n    color: white;\n  }\n}\n"
	nodes, err := e.Extract("svc", "a.css", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var btnCalls []string
	for _, n := range nodes {
		got[n.Name] = string(n.Kind)
		if n.Name == ".btn" {
			btnCalls = n.Calls
		}
	}
	want := map[string]string{
		"--color-a": "TailwindTheme", "tab-": "TailwindUtility", "dark": "TailwindVariant",
		"foo": "TailwindVariant", ".btn": "CSSSelector",
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s = %q, want %q (all: %v)", name, got[name], kind, got)
		}
	}
	if _, ok := got["--color-"]; ok {
		t.Error("a namespace reset is a theme variable")
	}
	if strings.Join(btnCalls, ",") != "card,tab-4" {
		t.Errorf(".btn calls = %v, want [card tab-4] (a @variant with a block is a use, not a definition)", btnCalls)
	}
}

// The rewrite changes bytes to spaces only: every offset and line survives.
func TestTailwindTextsKeepOffsets(t *testing.T) {
	src := []byte("@import \"tailwindcss\" theme(static);\n@custom-variant dark (&:where(.dark));\n@utility x-* {\n}\n@theme {\n  --c-*: initial;\n  --c-a: 1;\n}\n@source \"../a\";\n.r { color: red; }\n")
	texts := tailwindTexts(src)
	if len(texts) != 1 || len(texts[0]) != len(src) {
		t.Fatalf("texts = %d, lengths differ", len(texts))
	}
	if bytes.Count(texts[0], []byte("\n")) != bytes.Count(src, []byte("\n")) {
		t.Error("line count changed")
	}
	for i := range src {
		if texts[0][i] != src[i] && texts[0][i] != ' ' {
			t.Fatalf("byte %d: %q became %q", i, src[i], texts[0][i])
		}
	}
	if tailwindTexts([]byte(".a { color: red; }\n@theme { --a: 1; }\n")) != nil {
		t.Error("a file with nothing to blank was rewritten")
	}
}

// A candidate names the utility after its variants and without its importance flag.
func TestTailwindCandidateRoot(t *testing.T) {
	for in, want := range map[string]string{
		"card":                 "card",
		"hover:card":           "card",
		"md:hover:card":        "card",
		"has-[:checked]:card":  "card",
		"data-[a:b]:card":      "card",
		"card!":                "card",
		"!card":                "card",
		"dark:card!":           "card",
		"--color-brand":        "--color-brand",
		"bg-[color:var(--x)]":  "bg-[color:var(--x)]",
		"[&:hover]:card":       "card",
		"":                     "",
		"hover:":               "hover:",
		"fade-in":              "fade-in",
		"min-[400px]:max-w-xs": "max-w-xs",
	} {
		if got := tailwindCandidateRoot(in); got != want {
			t.Errorf("tailwindCandidateRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// The rewrite parses the file; the names come from the file as written, so a
// selector the parse needed as `.a>&` still reads `.a &`.
func TestTailwindNamesAreCutFromTheFileAsWritten(t *testing.T) {
	e, err := NewTSExtractor("css")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := "@utility card {\n  color: red;\n}\n.group:hover &,\n[data-x=\"1\"] & {\n  @apply hover:card!;\n}\n"
	nodes, err := e.Extract("svc", "a.css", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	var calls []string
	for _, n := range nodes {
		names = append(names, n.Name)
		calls = append(calls, n.Calls...)
	}
	for _, want := range []string{".group:hover &", "[data-x=\"1\"] &", "card"} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Errorf("no symbol named %q in %v", want, names)
		}
	}
	if strings.Join(calls, ",") != "card" {
		t.Errorf("calls = %v, want [card]", calls)
	}
}

// VariantSniff is the sniff the extractor selects the variant with.
func TestVariantSniffMatchesTheExtractorsChoice(t *testing.T) {
	re, ok := VariantSniff("css", "tailwind")
	if !ok {
		t.Fatal("css has no tailwind variant sniff")
	}
	if !re.MatchString("@theme { --a: 1; }") || re.MatchString(".a { color: red; }") {
		t.Error("the variant sniff does not tell a v4 stylesheet from a plain one")
	}
	if _, ok := VariantSniff("css", "nope"); ok {
		t.Error("an unknown variant has a sniff")
	}
	if _, ok := VariantSniff("scss", "tailwind"); ok {
		t.Error("scss has a tailwind variant")
	}
	pt, err := ProbeTargetForSource("css", "a.css", []byte("@theme { --a: 1; }"))
	if err != nil || !strings.Contains(pt.QueryFile, "tailwind.scm") {
		t.Errorf("probe target = %+v, %v; want the tailwind query", pt, err)
	}
	pt, err = ProbeTargetForSource("css", "a.css", []byte(".a { color: red; }"))
	if err != nil || strings.Contains(pt.QueryFile, "tailwind.scm") {
		t.Errorf("a plain file's probe target = %+v, %v; want the dialect's own query", pt, err)
	}
}

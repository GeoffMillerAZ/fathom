package lang_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

const sentinel = "SENTINEL9f3a"

// urlExtractor puts a URL with a password in every string field of a Result.
type urlExtractor struct{}

func (urlExtractor) Extract(_ context.Context, _ lang.File) (lang.Extracted, error) {
	u := "https://u:" + sentinel + "@h.example/x"
	return lang.Extracted{
		Symbols:    []lang.Symbol{{Name: "N" + u, Package: "p", Kind: "RowDef", Signature: `f("` + u + `") ` + u, Line: 1, Tier: "verified"}},
		References: []lang.Reference{{From: u, To: u, Raw: u, Edge: "uses", Line: 1}},
		Unresolved: []lang.Unresolved{{From: u, Edge: "uses", Raw: u, Reason: "dynamic", Line: 1}},
		Facts:      []lang.Fact{{Entity: u, Key: "k", Value: u, Line: 1}},
	}, nil
}

// The userinfo of a URL is withheld wherever a string from the file lands: a
// symbol's name, key and signature, every reference field, every unresolved
// field and a fact's entity and value. Each field is checked on the path a Go
// extractor takes, for a code dialect (a pack dialect is on no quote-led list, so
// its Signature is withheld) and for one that is no code.
func TestURLUserinfoIsWithheldInEveryField(t *testing.T) {
	for _, kind := range []string{"language", "tooling"} {
		t.Run(kind, func(t *testing.T) {
			m := lang.Match{ID: "url-" + kind, Kind: kind, Extensions: []string{".urlx"}, Kinds: map[string]string{"RowDef": "function"}}
			r := newReader(t, lang.WithExtractor(m, urlExtractor{}))
			got := extract(t, r, "a/b.urlx", "x")
			if len(got.Symbols) != 1 || len(got.References) != 1 || len(got.Unresolved) != 1 || len(got.Facts) != 1 {
				t.Fatalf("the extractor's result is not read: %+v", got)
			}
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), sentinel) {
				t.Errorf("the password is in the Result: %s", b)
			}
			// Each field on its own, so a field that is not scrubbed is named.
			s, ref, un, f := got.Symbols[0], got.References[0], got.Unresolved[0], got.Facts[0]
			for name, v := range map[string]string{
				"Symbol.Name": s.Name, "Symbol.Key": s.Key, "Symbol.Signature": s.Signature,
				"Reference.From": ref.From, "Reference.To": ref.To, "Reference.Raw": ref.Raw,
				"Unresolved.From": un.From, "Unresolved.Raw": un.Raw,
				"Fact.Entity": f.Entity, "Fact.Value": f.Value,
			} {
				if strings.Contains(v, sentinel) {
					t.Errorf("%s keeps the password: %q", name, v)
				}
			}
			if kind == "tooling" && s.Signature != "" {
				t.Errorf("a Signature of a kind nobody listed is returned from a dialect that is no code: %q", s.Signature)
			}
			if kind == "language" && s.Signature != "" {
				t.Errorf("a Signature of a code dialect on no list is returned: %q", s.Signature)
			}
		})
	}
}

// The same rule holds for a pack dialect, which a file reaches through the pack
// layer and not the built-in router.
func TestPackReadWithholdsURLUserinfo(t *testing.T) {
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)))
	u := "https://u:" + sentinel + "@h.example/x"
	got := extract(t, r, "ui/clock.widget.json", `{"name":"Clock","kind":"timer","uses":["`+u+`"]}`)
	if got.Dialect != "widget-spec" || len(got.References) == 0 {
		t.Fatalf("the pack dialect read nothing: %+v", got)
	}
	if b, _ := json.Marshal(got); strings.Contains(string(b), sentinel) {
		t.Errorf("the password is in the Result: %s", b)
	}
}

// A Signature from a dialect that is no code is withheld unless its kind is on
// the list, and the list is held here: each entry is a kind a fixture produced
// from a dialect that is no code, each reason is one line, and a Signature of a
// kind that is not listed is empty in every fixture.
func TestStructuralKindsHoldNoValue(t *testing.T) {
	r := newReader(t)
	kinds := dialectKinds(r)
	seen := map[string]bool{}
	for _, f := range plainReadings(t, r) {
		if kinds[f.res.Dialect] == "language" {
			continue
		}
		for _, s := range f.res.Symbols {
			seen[s.Kind] = true
			if _, listed := lang.StructuralKinds[s.Kind]; !listed && s.Signature != "" {
				t.Errorf("%s %s: a Signature of the unlisted kind %s is returned: %q", f.res.Dialect, f.path, s.Kind, s.Signature)
			}
		}
	}
	for kind, why := range lang.StructuralKinds {
		switch {
		case strings.TrimSpace(why) == "" || strings.ContainsAny(why, "\n") || len(why) > 140:
			t.Errorf("%s: the reason must be one line of at most 140 characters: %q", kind, why)
		case !seen[kind]:
			t.Errorf("%s is on the list and no fixture of a dialect that is no code produces it", kind)
		}
	}
}

var (
	plainOnce sync.Once
	plainRead []reading
)

// plainReadings is every fixture and own sample read as written, read once.
func plainReadings(t *testing.T, r *lang.Reader) []reading {
	t.Helper()
	plainOnce.Do(func() { plainRead = allReadings(t, r, nil) })
	return plainRead
}

type reading struct {
	path string
	res  lang.Result
}

// allReadings reads every dialect fixture and own sample, each source passed
// through transform when it is not nil.
func allReadings(t *testing.T, r *lang.Reader, transform func(dialectKind, path string, src []byte) []byte) []reading {
	t.Helper()
	kinds := dialectKinds(r)
	var out []reading
	one := func(dir, path string, src []byte) {
		if transform != nil {
			src = transform(kinds[dir], path, src)
		}
		res, err := r.Extract(context.Background(), lang.File{Path: path, Src: src})
		if err != nil {
			t.Fatal(err)
		}
		if res.Skipped == "" {
			out = append(out, reading{path, res})
		}
	}
	for _, f := range dialectFixtures(t) {
		one(f.dir, f.rel, f.src)
	}
	ids := make([]string, 0, len(ownSamples))
	for id := range ownSamples {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		one(id, ownSamples[id].path, []byte(ownSamples[id].src))
	}
	return out
}

func dialectKinds(r *lang.Reader) map[string]string {
	kinds := map[string]string{}
	for _, d := range r.Dialects() {
		kinds[d.ID] = d.Kind
	}
	return kinds
}

var (
	quotedRe = regexp.MustCompile("\"(?:[^\"\\\\\\n]|\\\\.)*\"|'(?:[^'\\\\\\n]|\\\\.)*'|`[^`\\n]*`")
	// keyValueRe is a `key: value` or `key = value` line of YAML, INI, TOML or HCL,
	// with an optional list dash; the value is the third group.
	keyValueRe = regexp.MustCompile(`(?m)^(\s*(?:-\s+)?[A-Za-z0-9_.\-/$@ ]+?\s*[:=]\s*)([^\s#"'\[{|>&*!][^#\n]*?)([ \t]*(?:#.*)?)$`)
	xmlTextRe  = regexp.MustCompile(`>([^<>\s][^<>]*)<`)
)

// quotedToSentinel replaces what is between the quotes of every quoted literal.
func quotedToSentinel(src []byte) []byte {
	return quotedRe.ReplaceAllFunc(src, func(m []byte) []byte {
		return append(append([]byte{m[0]}, sentinel...), m[len(m)-1])
	})
}

// valuesToSentinel replaces the value of every `key: value` line and the text of
// every XML element, after the quoted literals.
func valuesToSentinel(src []byte) []byte {
	src = quotedToSentinel(src)
	src = keyValueRe.ReplaceAll(src, []byte("${1}"+sentinel+"${3}"))
	return xmlTextRe.ReplaceAll(src, []byte(">"+sentinel+"<"))
}

// The family test of the literals rule. Every quoted literal of every fixture,
// and in a dialect that is no code every `key: value` value too, is replaced by
// a sentinel, and the sentinel must come back in no Signature, no Note and no
// value under a credential key. Reference targets are the one place a value is
// meant to be returned (Reference.To and Raw, Unresolved.Raw), so they are not
// checked. Fact values under any other key are returned as written and are only
// counted.
func TestSentinelPositionsAreNotReturned(t *testing.T) {
	r := newReader(t)
	for name, transform := range map[string]func(string, string, []byte) []byte{
		"quoted literals": func(_, _ string, src []byte) []byte { return quotedToSentinel(src) },
		"values": func(kind, _ string, src []byte) []byte {
			if kind == "language" {
				return quotedToSentinel(src)
			}
			return valuesToSentinel(src)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			leaks := map[string]string{}
			factValues := 0
			for _, f := range allReadings(t, r, transform) {
				for _, s := range f.res.Symbols {
					if strings.Contains(s.Signature, sentinel) {
						leaks[f.res.Dialect+" "+s.Kind+" Signature"] = fmt.Sprintf("%s: %q", f.path, s.Signature)
					}
				}
				for _, n := range f.res.Notes {
					if strings.Contains(n, sentinel) {
						leaks[f.res.Dialect+" Note"] = fmt.Sprintf("%s: %q", f.path, n)
					}
				}
				for _, fact := range f.res.Facts {
					if !strings.Contains(fact.Value, sentinel) {
						continue
					}
					if lang.CredentialKey(fact.Key) {
						leaks[f.res.Dialect+" Fact value of "+fact.Key] = fmt.Sprintf("%s: %q", f.path, fact.Value)
					} else {
						factValues++
					}
				}
			}
			keys := make([]string, 0, len(leaks))
			for k := range leaks {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t.Errorf("%s returns the sentinel, first in %s", k, leaks[k])
			}
			t.Logf("%d fact values under other keys carry the sentinel, returned as written", factValues)
		})
	}
}

// Notes carry a reason and a line, never the file's text.
func TestNotesCarryNoSourceText(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ name, path, src string }{
		{"go.mod directive", "go.mod", "module example.com/m\n\n" + sentinel + " x\n"},
		{"proto token", "api/x.proto", "syntax = \"proto3\";\nmessage M { string " + sentinel + " = ; }\n"},
		{"libconfig setting", "etc/proxysql.cnf", "admin_variables = { " + sentinel + " }\n"},
		{"json character", "data.json", "{\"a\": " + sentinel + "}"},
		{"toml", "data.toml", "a = \"" + sentinel + "\nb = 1\n"},
		{"yaml", "k/pod.yaml", "a: [" + sentinel + "\nb: c\n"},
		{"hcl", "infra/main.tf", "resource \"a\" \"" + sentinel + "\" {\n  x = \n}\n"},
		{"rego", "p.rego", "package p\n\npassword := \"" + sentinel + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src)
			if len(got.Notes) == 0 {
				t.Skipf("the shape reads without a note (incomplete=%v)", got.Incomplete)
			}
			for _, n := range got.Notes {
				if strings.Contains(n, sentinel) {
					t.Errorf("a note echoes the file: %q", n)
				}
			}
		})
	}
}

// factExtractor reads one fact per `key value` line.
type factExtractor struct{}

func (factExtractor) Extract(_ context.Context, f lang.File) (lang.Extracted, error) {
	var out lang.Extracted
	for i, line := range strings.Split(string(f.Src), "\n") {
		if w := strings.Fields(line); len(w) == 2 {
			out.Facts = append(out.Facts, lang.Fact{Entity: "E", Key: w[0], Value: w[1], Line: i + 1})
		}
	}
	return out, nil
}

// A fact value under a key whose last segment looks like a credential reads ***,
// whatever the value is. Any other fact value is returned as written.
func TestFactValueUnderACredentialKeyIsMasked(t *testing.T) {
	m := lang.Match{ID: "fact-rows", Kind: "tooling", Extensions: []string{".facts"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithExtractor(m, factExtractor{}))
	got := extract(t, r, "a/b.facts", "labels.password "+sentinel+"\nenv.DB_PASSWORD "+sentinel+"\nclientSecret "+sentinel+"\nlabels.app web\nimage reg/a:1\n")
	want := map[string]string{"labels.password": "***", "env.DB_PASSWORD": "***", "clientSecret": "***", "labels.app": "web", "image": "reg/a:1"}
	if len(got.Facts) != len(want) {
		t.Fatalf("facts: %+v", got.Facts)
	}
	for _, f := range got.Facts {
		if f.Value != want[f.Key] {
			t.Errorf("fact %s reads %q, want %q", f.Key, f.Value, want[f.Key])
		}
	}
}

// The literals rule through real dialects: a default written with a nested
// literal, a sigil, a percent literal or a heredoc reads *** in the Signature of
// the function that holds it (MF2). Each source is valid for its tool.
func TestSignatureWithholdsNestedAndNonQuoteLiterals(t *testing.T) {
	r := newReader(t)
	const s = sentinel
	for _, tc := range []struct{ path, src string }{
		{"a.py", `def f(x=f"{d["` + s + `"]}"):` + "\n    pass\n"},
		{"a.rb", `def f(x = "a#{"` + s + `"}b")` + "\nend\n"},
		{"b.rb", `def f(x = %w[` + s + `])` + "\nend\n"},
		{"c.rb", `def f(x = %q(` + s + `))` + "\nend\n"},
		{"d.rb", `def f(x = %i[` + s + `])` + "\nend\n"},
		{"a.ex", "defmodule M do\n  def f(x \\\\ \"a#{\"" + s + "\"}\") do\n    x\n  end\nend\n"},
		{"b.ex", "defmodule M do\n  def f(x \\\\ ~S|" + s + "|) do\n    x\n  end\nend\n"},
		{"c.ex", "defmodule M do\n  def f(x \\\\ ~w(" + s + " " + s + ")) do\n    x\n  end\nend\n"},
		{"a.kt", `fun f(x: String = "a${"` + s + `"}b") {}` + "\n"},
		{"a.scala", `object O { def f(x: String = s"a${"` + s + `"}b") = x }` + "\n"},
		{"a.dart", `void f({String x = '${"` + s + `"}'}) {}` + "\n"},
		{"a.js", "function f(x = `a${`" + s + "`}b`) {}\n"},
		{"a.ts", "function f(x: string = `a${`" + s + "`}b`) {}\n"},
		{"a.php", "<?php\nfunction f($x = <<<EOT\n" + s + "\nEOT\n) {}\n"},
		{"b.php", "<?php\nfunction f($x = <<<'EOT'\n" + s + "\nEOT\n) {}\n"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := extract(t, r, tc.path, tc.src)
			if res.Skipped != "" || len(res.Symbols) == 0 {
				t.Fatalf("the source is not read: %+v", res)
			}
			for _, sym := range res.Symbols {
				if strings.Contains(sym.Signature, sentinel) {
					t.Errorf("%s returns the literal: %q", sym.Name, sym.Signature)
				}
			}
		})
	}
}

// A literal that has no escapes (a C# verbatim string, a C++ raw string, a Rust
// raw string), a backtick that honours a backslash (a JS or TS template) and a
// Ruby percent literal with no type letter end where the language says, so the
// literal after them is masked and not returned (MF9). Each source is valid for
// its tool.
func TestSignatureWithholdsRawVerbatimAndEscapedBacktickLiterals(t *testing.T) {
	r := newReader(t)
	const s = sentinel
	for _, tc := range []struct{ path, src string }{
		{"a.cs", "class C {\n  void F(string p = @\"C:\\temp\\\", string q = \"" + s + "\") {}\n}\n"},
		{"b.cs", "class C {\n  [Obsolete(@\"C:\\\")] [Description(\"" + s + "\")]\n  void G() {}\n}\n"},
		{"a.cpp", "void f(const char* p = R\"(a\"" + s + ")\") {}\n"},
		{"b.cpp", "void f(const char* p = u8R\"x(a)\"" + s + ")x\") {}\n"},
		{"a.js", "function f(x = `a\\`" + s + "`, y = `" + s + "`) {}\n"},
		{"a.ts", "function f(x: string = `a\\`" + s + "`, y: string = `" + s + "`): void {}\n"},
		{"a.rb", "def f(x = %(" + s + "))\nend\n"},
		{"b.rb", "def f(x = %[" + s + "])\nend\n"},
		{"c.rb", "def f(x = %{" + s + "})\nend\n"},
		{"a.rs", "#[doc = r\"C:\\\"]\n#[cfg(feature = \"" + s + "\")]\nfn f() {}\n"},
		{"b.rs", "#[doc = r#\"a\"" + s + "\"#]\nfn f() {}\n"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := extract(t, r, tc.path, tc.src)
			if res.Skipped != "" || len(res.Symbols) == 0 {
				t.Fatalf("the source is not read: %+v", res)
			}
			for _, sym := range res.Symbols {
				if strings.Contains(sym.Signature, s) {
					t.Errorf("%s returns the literal: %q", sym.Name, sym.Signature)
				}
			}
		})
	}
}

// A structural kind's Signature is masked as a code Signature is: a class header
// holds calls with literals in them (MF10). Checked through the dialect Python
// reads the header with; TestStructuralSignaturesMaskTheirLiterals holds every
// kind on the list.
func TestConanRecipeHeaderMasksItsLiterals(t *testing.T) {
	res := extract(t, newReader(t), "conanfile.py", "from conan import ConanFile\n\nclass Recipe(ConanFile, mixin(\""+sentinel+"\")):\n    name = \"x\"\n")
	found := false
	for _, sym := range res.Symbols {
		if sym.Kind == "ConanRecipe" {
			found = true
			if strings.Contains(sym.Signature, sentinel) {
				t.Errorf("the recipe header returns the literal: %q", sym.Signature)
			}
		}
	}
	if !found {
		t.Fatalf("no ConanRecipe symbol: %+v", res.Symbols)
	}
}

// A parse Note names the reason and the line, never a word of the file: go/parser
// prints the identifier or number it stopped at bare (MF11).
func TestGoParseNotesCarryNoIdentifiers(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ name, src string }{
		{"identifier after a var", "package p\n\nvar x = 1 " + sentinel + "\n"},
		{"identifier after a call", "package p\n\nfunc f() {\n\tg(1) " + sentinel + "\n}\n"},
		{"identifier among more errors", "package p\n\nvar x = 1 " + sentinel + "\nvar y = 2 " + sentinel + "a\nvar z = 3 " + sentinel + "b\n"},
		{"number", "package p\n\nvar x = 1 99999\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, "a.go", tc.src)
			if len(got.Notes) == 0 {
				t.Fatalf("the source reads without a note: %+v", got)
			}
			for _, n := range got.Notes {
				if strings.Contains(n, sentinel) || strings.Contains(n, "99999") {
					t.Errorf("a note echoes the file: %q", n)
				}
			}
		})
	}
}

// userinfoNameExtractor names symbols by an image reference with a userinfo, in
// a package, and references each from its Key.
type userinfoNameExtractor struct{}

func (userinfoNameExtractor) Extract(_ context.Context, _ lang.File) (lang.Extracted, error) {
	names := []string{"//u:" + sentinel + "@reg/j", "u:" + sentinel + "@reg/i:1", "https://u:" + sentinel + "@reg/k"}
	var out lang.Extracted
	for i, n := range names {
		key := "pod." + n
		out.Symbols = append(out.Symbols, lang.Symbol{Name: n, Package: "pod", Kind: "RowDef", Line: i + 1, Tier: "verified"})
		out.References = append(out.References, lang.Reference{From: key, To: "x", Edge: "uses", Line: i + 1})
		out.Unresolved = append(out.Unresolved, lang.Unresolved{From: key, Edge: "uses", Raw: "x", Reason: "dynamic", Line: i + 1})
	}
	return out, nil
}

// A Key is Package.Name (design rule 9) after the userinfo is dropped, for every
// spelling of a userinfo, and a Reference or Unresolved From is the Key.
func TestKeyIsPackageDotNameAfterTheUserinfoIsDropped(t *testing.T) {
	m := lang.Match{ID: "ui-names", Kind: "tooling", Extensions: []string{".uin"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithExtractor(m, userinfoNameExtractor{}))
	got := extract(t, r, "a.uin", "x")
	if len(got.Symbols) != 3 {
		t.Fatalf("%+v", got)
	}
	keys := map[string]bool{}
	for _, s := range got.Symbols {
		if s.Key != s.Package+"."+s.Name {
			t.Errorf("Key %q is not Package.Name of %q.%q", s.Key, s.Package, s.Name)
		}
		keys[s.Key] = true
	}
	for _, ref := range got.References {
		if !keys[ref.From] {
			t.Errorf("Reference.From %q is no symbol's Key", ref.From)
		}
	}
	for _, u := range got.Unresolved {
		if !keys[u.From] {
			t.Errorf("Unresolved.From %q is no symbol's Key", u.From)
		}
	}
	if b, _ := json.Marshal(got); strings.Contains(string(b), sentinel) {
		t.Errorf("the password is in the Result: %s", b)
	}
}

func TestKubernetesImageUserinfoIsDroppedFromTheKey(t *testing.T) {
	r := newReader(t)
	res := extract(t, r, "pod.yaml", "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\nspec:\n  containers:\n  - name: a\n    image: //u:"+sentinel+"@reg/j\n  - name: b\n    image: u:"+sentinel+"@reg/i:1\n")
	if res.Dialect == "" || len(res.Symbols) < 3 {
		t.Fatalf("the manifest is not read: %+v", res)
	}
	if b, _ := json.Marshal(res); strings.Contains(string(b), sentinel) {
		t.Errorf("the password is in the Result: %s", b)
	}
	for _, s := range res.Symbols {
		if s.Key != s.Package+"."+s.Name && !(s.Package == "" && s.Key == s.Name) {
			t.Errorf("Key %q is not Package.Name of %q.%q", s.Key, s.Package, s.Name)
		}
	}
}

// A structural kind's Signature ends before any value: a one-line Thrift service
// holds its body (and a parameter's default) on the line the Signature is read
// from, and a GraphQL fragment's directives hold arguments (MF3).
func TestStructuralSignatureEndsBeforeAValue(t *testing.T) {
	r := newReader(t)
	const s = sentinel
	for _, tc := range []struct{ name, path, src, kind, want string }{
		{"thrift one line", "a.thrift", `service A extends B { void f(1: string x = "` + s + `") }` + "\n", "ThriftService", "service A extends B"},
		{"thrift body on the next line", "b.thrift", "service A {\n  void f(1: string x = \"" + s + "\")\n}\n", "ThriftService", "service A"},
		{"graphql string argument", "a.graphql", `fragment F on T @dir(arg: "` + s + `") @other(x: """` + s + `""") { id }` + "\n", "GraphQLFragment", "fragment F on T"},
		{"graphql enum argument", "b.graphql", "fragment F on T @d(a: " + s + ") { id }\n", "GraphQLFragment", "fragment F on T"},
		{"graphql arguments across lines", "c.graphql", "fragment F on T\n  @d(\n    a: " + s + "\n  ) {\n  id\n}\n", "GraphQLFragment", "fragment F on T"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := extract(t, r, tc.path, tc.src)
			found := false
			for _, sym := range res.Symbols {
				if sym.Kind != tc.kind {
					continue
				}
				found = true
				if strings.Contains(sym.Signature, s) {
					t.Errorf("the Signature returns a value: %q", sym.Signature)
				}
				if sym.Signature != tc.want {
					t.Errorf("Signature %q, want %q", sym.Signature, tc.want)
				}
			}
			if !found {
				t.Fatalf("no %s symbol: %+v", tc.kind, res.Symbols)
			}
		})
	}
	for kind := range lang.StructuralCuts {
		if _, ok := lang.StructuralKinds[kind]; !ok {
			t.Errorf("%s is cut and not on the structural list", kind)
		}
	}
}

// The cut rule through real dialects (MF12, MF13): a quote that opens no literal
// (a comment's apostrophe, a digit separator, a quote in a regex, a char literal,
// a Scala symbol) and the quote styles of a raw string used to throw a pairing
// scanner off, and the next literal came back. The Signature is cut at the first
// quote, so no sentinel comes back whatever precedes it.
func TestSignatureCutsAtTheFirstQuote(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ path, src string }{
		{"a.cpp", "void f(int a = 1'000, std::string s = \"it's\", std::string t = \"QZ103\") {}\n"},
		{"b.cpp", "class K { public: void g(int a /* it's */, const char* b = \"can't\", const char* s = \"QZ104\") {} };\n"},
		{"c.cpp", "void h(const char* a = R\"x()\")x\", const char* s = \"QZ105\") {}\n"},
		{"a.kt", "fun f(a: Int /* it's */, b: String = \"can't\", s: String = \"QZ41\") {}\n"},
		{"a.cs", "class C { void F(int a /* it's */, string b = \"can't\", string s = \"QZ42\") {} }\n"},
		{"b.cs", "class C { void F(string p = \"\"\"\"a\"\"QZ30\"\"\"\") {} }\n"},
		{"c.cs", "class C { [A(\"\"\"\"a\"\"\"QZ31\"\"\"\")] void F() {} }\n"},
		{"a.js", "function f(a = /'/, s = 'QZ37') {}\n"},
		{"a.ts", "function f(a = /'/, s = 'QZ38') {}\n"},
		{"a.jsx", "function F({a = /'/, s = 'QZ114'}) { return <a/> }\n"},
		{"a.tsx", "function F({a = /'/, s = 'QZ118'}: P) { return <a/> }\n"},
		{"a.scala", "object O { def f(a: Symbol = 'x, s: String = \"it's\", t: String = \"QZ117\") = 1 }\n"},
		{"a.java", "class C { @A(x = '\"', y = \"QZ116\") void f() {} }\n"},
		{"a.py", "def f(a=1, s='''QZ1'''):\n    pass\n"},
		{"a.go", "package p\n\nconst K = `QZ2`\n\nfunc F(a int) {}\n"},
		{"a.rs", "#[cfg(feature = \"QZ3\")]\nfn f<'a>(x: &'a str) {}\n"},
		{"a.dart", "void f({String a = r'QZ4', String b = 'it\\'s'}) {}\n"},
		// The dialects that withhold their Signature outright.
		{"a.rb", "def f(a = ?\", s = \"QZ62\", b = ?\")\nend\n"},
		{"b.rb", "def f(a = %|QZ33|)\nend\n"},
		{"c.rb", "def f(a = %!QZ34!)\nend\n"},
		{"d.rb", "def f(a = %^QZ35^)\nend\n"},
		{"e.rb", "def f(a = %/QZ36/)\nend\n"},
		{"a.ex", "defmodule M do\n  def f(a \\\\ ?\", s \\\\ \"QZ110\", b \\\\ ?\"), do: s\nend\n"},
		{"a.php", "<?php\nfunction f($a /* it's */, $b = \"can't\", $s = \"QZ115\") {}\n"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := extract(t, r, tc.path, tc.src)
			if res.Skipped != "" || len(res.Symbols) == 0 {
				t.Fatalf("the source is not read: %+v", res)
			}
			for _, sym := range res.Symbols {
				if strings.Contains(sym.Signature, "QZ") {
					t.Errorf("%s returns the literal: %q", sym.Name, sym.Signature)
				}
			}
		})
	}
}

// Ruby, PHP and Elixir are code dialects no quote leads (% literals, ?c, heredocs,
// sigils), so their Signature is withheld: empty, whatever the source.
func TestUnclassifiedCodeDialectsWithholdTheirSignature(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ path, src string }{
		{"a.rb", "def f(a, b = 1)\nend\n"},
		{"a.php", "<?php\nfunction f($a, $b = 1) {}\n"},
		{"a.ex", "defmodule M do\n  def f(a, b \\\\ 1), do: a\nend\n"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := extract(t, r, tc.path, tc.src)
			if res.Skipped != "" || len(res.Symbols) == 0 {
				t.Fatalf("the source is not read: %+v", res)
			}
			for _, sym := range res.Symbols {
				if sym.Signature != "" {
					t.Errorf("%s %s returns a Signature: %q", res.Dialect, sym.Name, sym.Signature)
				}
			}
		})
	}
}

// quoteLedForms is one function or declaration per literal form of the language,
// each holding the sentinel, for the dialects on quoteLed that the Reader reads
// (Swift reads as no dialect today, so its forms are in
// TestQuoteLedDialectsCutTheirFormsDirectly): the family test of the cut rule. A
// form is written where the dialect's Signature carries it.
var quoteLedForms = []struct {
	dialect, path, tmpl string // tmpl holds %s where the literal goes and %d for a unique name
	forms               []string
}{
	{"python", "a.py", "def f%d(a=%s):\n    pass\n", []string{`"S"`, `'S'`, `b"S"`, `r"S"`, `f"{x}S"`, `"""S"""`, `rb'S'`, `u'S'`, `'''S'''`}},
	{"javascript", "a.js", "function f%d(a = %s) {}\n", []string{`"S"`, `'S'`, "`S`", "`${x}S`", `/'/, s = 'S'`}},
	{"typescript", "a.ts", "function f%d(a: string = %s): void {}\n", []string{`"S"`, `'S'`, "`S`", "`${x}S`", `/'/, s = 'S'`}},
	{"cpp", "a.cpp", "void f%d(const char* a = %s) {}\n", []string{`"S"`, `R"(S)"`, `R"x(S)x"`, `u8"S"`, `L"S"`, `u"S"`, `U"S"`, `'S'`}},
	{"cuda", "a.cu", "__global__ void f%d(const char* a = %s) {}\n", []string{`"S"`, `R"(S)"`, `u8"S"`, `'S'`}},
	{"csharp", "a.cs", "class C%d { void F(string a = %s) {} }\n", []string{`"S"`, `@"S"`, `$"S"`, `$@"S"`, `"""S"""`, `""""S""""`, `'S'`}},
	{"kotlin", "a.kt", "fun f%d(a: String = %s) {}\n", []string{`"S"`, `"""S"""`, `'S'`}},
	{"scala", "a.scala", "object O%d { def f(a: String = %s) = a }\n", []string{`"S"`, `"""S"""`, `s"S"`, `'S'`}},
	{"dart", "a.dart", "void f%d({String a = %s}) {}\n", []string{`'S'`, `"S"`, `r'S'`, `r"S"`, `'''S'''`, `"""S"""`}},
	{"java", "a.java", "class C%d { void f(String a, @A(%s) int b) {} }\n", []string{`"S"`, `'S'`, "\"\"\"\n S\n \"\"\""}},
	{"rust", "a.rs", "#[doc = %s]\nfn f%d() {}\n", []string{`"S"`, `r"S"`, `r#"S"#`, `b"S"`, `br#"S"#`, `c"S"`, `'S'`, `b'S'`}},
	{"go", "a.go", "package p\n\nfunc F%d(a [len(%s)]int) {}\n", []string{`"S"`, "`S`"}},
	{"c", "a.c", "static const char *K%d = %s;\nvoid g%d(const char *a) {}\n", []string{`"S"`, `L"S"`, `u8"S"`, `u"S"`, `U"S"`, `'S'`}},
}

func TestQuoteLedDialectsCutEveryLiteralForm(t *testing.T) {
	r := newReader(t)
	covered := map[string]bool{}
	for _, tc := range quoteLedForms {
		covered[tc.dialect] = true
		for i, form := range tc.forms {
			t.Run(fmt.Sprintf("%s/%d", tc.dialect, i), func(t *testing.T) {
				lit := strings.ReplaceAll(form, "S", "QZ"+fmt.Sprint(900+i))
				src := fmt.Sprintf(strings.NewReplacer("%d", "%[1]d", "%s", "%[2]s").Replace(tc.tmpl), i, lit)
				res := extract(t, r, tc.path, src)
				if res.Skipped != "" || len(res.Symbols) == 0 {
					t.Fatalf("the source is not read: %+v", res)
				}
				for _, sym := range res.Symbols {
					if strings.Contains(sym.Signature, "QZ") {
						t.Errorf("%s %s returns the literal: %q", res.Dialect, sym.Name, sym.Signature)
					}
				}
			})
		}
	}
	for id := range lang.QuoteLed {
		if !covered[id] && id != "swift" {
			t.Errorf("%s is on quoteLed and the family test holds no literal form of it", id)
		}
	}
}

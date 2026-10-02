package extractor

import (
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// testTSExtractor builds a TSExtractor over one ad-hoc dialect: grammar
// blob/entry, a query, and a capture-kind table (engine tests that need a
// query shape no embedded dialect has yet).
func testTSExtractor(blob, entry, query string, kinds map[string]tsDefKind, mutate func(*tsDialectRules)) *TSExtractor {
	rules := &tsDialectRules{id: "test", kinds: kinds, methodSep: ".", containerSep: ".", packageStrategy: "dir", sigMax: 200, queryFile: "test.scm", query: query}
	if mutate != nil {
		mutate(rules)
	}
	v := tsVariant{blob: blob, entryPoint: entry, rules: rules}
	return newTSExtractorFromSpec(&tsLangSpec{name: "test", variantForExt: map[string]tsVariant{}, defaultVariant: v})
}

// TestTSReferenceNameCapture (LB-F4, design §3.4): a @name inside
// @reference.call is the callee; without one the call expression's text
// stands (today's behaviour); @reference.class/implementation/type are
// counted, not stored.
func TestTSReferenceNameCapture(t *testing.T) {
	query := `
(function_definition name: (identifier) @name body: (_) @body) @definition.function
(call function: (identifier) @name) @reference.call
(call function: (attribute attribute: (identifier) @name)) @reference.call
(call function: (subscript)) @reference.call
(class_definition superclasses: (argument_list (identifier) @name @reference.class))
(type (identifier) @name) @reference.type
`
	e := testTSExtractor("python", "tree_sitter_python", query,
		map[string]tsDefKind{"function": {Kind: core.KindPyFunc, Callable: true}}, nil)
	defer e.Close()
	src := `def f(a: int):
    foo()
    obj.method(1)
    handlers[0](a)

class A(Base, Mixin):
    pass
`
	nodes, err := e.Extract("svc", "m.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "f" {
		t.Fatalf("nodes = %v", symStrings(nodes))
	}
	if got, want := strings.Join(nodes[0].Calls, ","), "foo,handlers[0](a),method"; got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if got := e.TypeRefs(); got != 3 {
		t.Errorf("TypeRefs() = %d, want 3 (Base, Mixin, int)", got)
	}
}

// withReferences builds id's extractor with its references query named
// (queries.references = references.scm), whether or not the committed
// manifest names it yet: the php and rust queries ship before their
// manifests do (an integrator's line), and this is how they are proven.
func withReferences(t *testing.T, id string) *TSExtractor {
	t.Helper()
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID != id {
			continue
		}
		d.Queries.References = "references.scm"
		rules, err := compileDialectRules(d)
		if err != nil {
			t.Fatal(err)
		}
		v := tsVariant{blob: d.Grammar.Blob, entryPoint: d.Grammar.EntryPoint, rules: rules}
		return newTSExtractorFromSpec(&tsLangSpec{name: id, variantForExt: map[string]tsVariant{}, defaultVariant: v})
	}
	t.Fatalf("no dialect %s", id)
	return nil
}

func refStrings(rs []RefSpec) []string {
	var out []string
	for _, r := range rs {
		var ls []string
		for _, l := range r.Lines {
			ls = append(ls, strconv.Itoa(l))
		}
		out = append(out, r.Name+"@"+strings.Join(ls, ","))
	}
	return out
}

func exportStrings(es []ExportSpec) []string {
	var out []string
	for _, e := range es {
		s := e.Module
		if e.Name != "" {
			s += " " + e.Name
		}
		if e.As != "" {
			s += " as " + e.As
		}
		if len(e.Hide) > 0 {
			s += " hide " + strings.Join(e.Hide, ",")
		}
		out = append(out, s+"@"+strconv.Itoa(e.Line))
	}
	return out
}

// TestExtractFileSpecs (LB-F74, LB-F75) runs each dialect's references query
// through ExtractFileSpecs: the references kept are exactly those whose head
// an import binds or, where unnamed names are in scope, that start with an
// upper case letter — never a definition's own name, a name inside an import
// statement, a local or a parameter — each with its lines, and the
// re-exports carry their names, renames and hidden names.
func TestExtractFileSpecs(t *testing.T) {
	cases := []struct {
		dialect, path, src string
		refs, exports      []string
		viaTestManifest    bool
	}{
		{dialect: "javascript", path: "app.ts", src: `import { User, helper } from './models';
import * as m from './ns';
export * from './a';
export { B, C as D } from './b';
export * as ns2 from './c';

export function make(c: m.Config, local: number): User {
  const x = new User();
  return [helper, m.thing, x, local, Other];
}
`, refs: []string{"User@7,8", "helper@9", "m@7,9", "m.Config@7", "m.thing@9"},
			exports: []string{"./a@3", "./b B@4", "./b C as D@4"}},
		{dialect: "python", path: "app.py", src: `from pkg import User
import pkg.mod as pm

def load(cfg: User, n: int) -> pm.Config:
    local = n
    return isinstance(local, User) and pm.helper
`, refs: []string{"User@4,6", "pm@4,6", "pm.Config@4", "pm.helper@6"}},
		{dialect: "java", path: "Service.java", src: `package com.acme.app;

import com.acme.model.User;

public class Service extends Base {
    private User owner;
    public Helper make(int count) { return Helper.create(count); }
}
`, refs: []string{"Base@5", "Helper@7", "Helper.create@7", "User@6"}},
		{dialect: "csharp", path: "Service.cs", src: `using Acme.Models;
namespace Acme.App {
  public class Service : Base {
    public User Owner;
    public void Run(int n) { Util.Call(n); }
  }
}
`, refs: []string{"Base@3", "User@4", "Util.Call@5"}},
		{dialect: "kotlin", path: "Service.kt", src: `package com.acme.app

import com.acme.util.trimAll

class Service : Base() {
    fun owner(name: String): User? = trimAll(name).let { null }
    fun kind() = Kinds.DEFAULT
    val ref = trimAll
}
`, refs: []string{"Base@5", "Kinds@7", "Kinds.DEFAULT@7", "String@6", "User@6", "trimAll@6,8"}},
		{dialect: "dart", path: "lib/bloc.dart", src: `import 'src/user.dart';
export 'src/bloc.dart';
export 'src/cubit.dart' show Cubit, CubitState;
export 'src/hidden.dart' hide Secret;

class Repo {
  User? find(int id) => null;
}
`, refs: []string{"User@7"},
			exports: []string{"src/bloc.dart@2", "src/cubit.dart Cubit@3", "src/cubit.dart CubitState@3", "src/hidden.dart hide Secret@4"}},
		{dialect: "ruby", path: "lib/app.rb", src: `require 'sinatra/base'

module Sinatra
  class Application < Base
    include Helpers
    def handle
      Sinatra::Helpers::Stream.new
    end
  end
end
`, refs: []string{"Base@4", "Helpers@5", "Sinatra@7", "Sinatra::Helpers@7", "Sinatra::Helpers::Stream@7"}},
		{dialect: "javascript", path: "view.tsx", src: `import { Card } from './card';
import Header from './header';

export function View(): JSX.Element {
  return <Header><Card title="x" /></Header>;
}
`, refs: []string{"Card@5", "Header@5"}},
		{dialect: "protobuf", path: "proto/acme/v1/user.proto", src: `syntax = "proto3";
package acme.v1;
import "acme/v2/common.proto";
message User {
  acme.v2.Common.Kind kind = 1;
  .buf.validate.FieldRules rules = 2;
  int32 id = 3;
}
`, refs: []string{".buf.validate.FieldRules@6", "acme.v2.Common.Kind@5"}},
		{dialect: "php", path: "src/App/Service.php", viaTestManifest: true, src: `<?php
namespace App;
use App\Model\User as U;
class Service extends Base implements Contract {
    public function run(Zed $z): ?U { $x = new Thing(); Other::CONST; \App\Abs\Q::go(); return U::class; }
}
`, refs: []string{"Base@4", "Contract@4", "Other@5", "Thing@5", "U@5", "Zed@5", `\App\Abs\Q@5`}},
		{dialect: "rust", path: "src/lib.rs", viaTestManifest: true, src: `use crate::model::User;
use crate::util::*;

struct S { owner: User, other: m::Bar }
fn make(n: u32) -> Option<Helper> { let x = User::new(); None }
`, refs: []string{"Helper@5", "Option@5", "User@4", "User::new@5"}},
	}
	for _, c := range cases {
		t.Run(c.dialect, func(t *testing.T) {
			var e *TSExtractor
			if c.viaTestManifest {
				e = withReferences(t, c.dialect)
			} else {
				var err error
				if e, err = NewTSExtractor(c.dialect); err != nil {
					t.Fatal(err)
				}
			}
			defer e.Close()
			fs, err := e.ExtractFileSpecs(c.path, []byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			imps, err := e.ExtractImports(c.path, []byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(imps) != len(fs.Imports) {
				t.Errorf("imports %d, ExtractImports %d: one parse must read the same imports", len(fs.Imports), len(imps))
			}
			if got, want := strings.Join(refStrings(fs.Refs), " "), strings.Join(c.refs, " "); got != want {
				t.Errorf("refs\n got %s\nwant %s", got, want)
			}
			if got, want := strings.Join(exportStrings(fs.Exports), " "), strings.Join(c.exports, " "); got != want {
				t.Errorf("exports\n got %s\nwant %s", got, want)
			}
		})
	}
}

// TestRefPath: a name path keeps its separators and an absolute marker,
// drops whitespace and generic arguments, and anything else is no name.
func TestRefPath(t *testing.T) {
	for in, want := range map[string]string{
		"Foo": "Foo", "a.b.C": "a.b.C", "List<Foo>": "List", "p.Outer.Inner<T, U>": "p.Outer.Inner",
		"Sinatra::Base": "Sinatra::Base", "::Top::Y": "::Top::Y", `\App\Foo`: `\App\Foo`, `Sub\R`: `Sub\R`,
		"a .\n b": "a.b", "$el": "$el",
		"foo()": "", "a[0]": "", "a?.b": "", "": "", "a.": "", "1a": "", "Foo<": "", "a>b": "",
	} {
		got, ok := refPath(in)
		if want == "" {
			if ok {
				t.Errorf("refPath(%q) = %q, want no name path", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("refPath(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
}

// TestDialectReferenceParamsStrict: the manifest keys LB-F74, LB-F76 and
// LB-F77 add take only their one value, as the CUE schema says.
func TestDialectReferenceParamsStrict(t *testing.T) {
	base := `{"id":"a","display":"X","kind":"language","engine":"tags","lang_tag":"x",
"match":{"extensions":[".a"]},"grammar":{"blob":"x","entry_point":"tree_sitter_x"},
"queries":{"tags":"tags.scm"},"kinds":{"f":{"source_kind":"AF","canonical":"function"}},
"required_kinds":["f"],"target_level":"L1","queue_id":"a"}`
	files := func(manifest string, extra ...string) fstest.MapFS {
		m := fstest.MapFS{"dialects/a/dialect.json": {Data: []byte(manifest)}, "dialects/a/tags.scm": {Data: nil}}
		for _, x := range extra {
			m["dialects/a/"+x] = &fstest.MapFile{Data: nil}
		}
		return m
	}
	with := func(s string) string { return strings.Replace(base, `"target_level"`, s+`,"target_level"`, 1) }
	for name, c := range map[string]struct {
		manifest string
		extra    []string
		want     string
	}{
		"package_scope off list": {with(`"imports":{"strategy":"namespace-dir","package_scope":"global"}`), nil, "imports.package_scope"},
		"constants off list":     {with(`"imports":{"strategy":"file-symbol","constants":"dynamic"}`), nil, "imports.constants"},
		"module_roots off list":  {with(`"imports":{"strategy":"file-symbol","module_roots":"bazel"}`), nil, "imports.module_roots"},
		"references missing":     {strings.Replace(base, `"tags":"tags.scm"`, `"tags":"tags.scm","references":"references.scm"`, 1), nil, "query references.scm"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadDialects(files(c.manifest, c.extra...))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
	ok := with(`"imports":{"strategy":"namespace-dir","package_scope":"nested"}`)
	ok = strings.Replace(ok, `"tags":"tags.scm"`, `"tags":"tags.scm","references":"references.scm"`, 1)
	ds, err := loadDialects(files(ok, "references.scm"))
	if err != nil || len(ds) != 1 || ds[0].Queries.References != "references.scm" || ds[0].Imports.Params.PackageScope != "nested" {
		t.Fatalf("a valid manifest: %v %+v", err, ds)
	}
}

// TestReferencesQueriesCompile: every dialect whose manifest names a
// references query compiles it on every grammar entry point it serves (the
// typescript dialect's on tsx too), as do the php and rust queries that ship
// before their manifests name them; a query that does not compile would fault
// every file of the language in the file-resolve pass.
func TestReferencesQueriesCompile(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, d := range ds {
		if d.Queries.References == "" && d.ID != "php" && d.ID != "rust" {
			continue
		}
		seen++
		e := withReferences(t, d.ID)
		exts := append([]string(nil), d.Match.Extensions...)
		for _, v := range d.Variants {
			exts = append(exts, v.Extensions...)
		}
		for _, ext := range exts {
			v := e.spec.defaultVariant
			for _, dv := range d.Variants {
				for _, x := range dv.Extensions {
					if x == ext {
						v.entryPoint = dv.EntryPoint
					}
				}
			}
			ve := newTSExtractorFromSpec(&tsLangSpec{name: d.ID, variantForExt: map[string]tsVariant{}, defaultVariant: v})
			if _, err := ve.ExtractFileSpecs("x"+ext, []byte("\n")); err != nil {
				t.Errorf("%s (%s, %s): %v", d.ID, ext, v.entryPoint, err)
			}
			ve.Close()
		}
		e.Close()
	}
	if seen < 11 {
		t.Errorf("%d dialects carry a references query, want the 9 wired and php and rust", seen)
	}
}

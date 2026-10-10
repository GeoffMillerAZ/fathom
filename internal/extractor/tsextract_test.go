package extractor

import (
	"bytes"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

func symStrings(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	sort.Strings(out)
	return out
}

func mustContainAll(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}

func TestNewTSExtractor_UnknownLanguage(t *testing.T) {
	if _, err := NewTSExtractor("cobol"); err == nil {
		t.Fatal("expected error for unknown language")
	}
}

func TestTSExtractor_Python(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`X = 1

class Bar:
    y = 2

    def baz(self):
        return helper()

    def nested_owner(self):
        def inner():
            return 1
        return inner()

def foo(x):
    return x + 1
`)
	nodes, err := e.Extract("svc", "pkg/mod.py", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindPyVar)+":X",
		string(core.KindPyClass)+":Bar",
		string(core.KindPyFunc)+":Bar.baz",
		string(core.KindPyFunc)+":Bar.nested_owner",
		string(core.KindPyFunc)+":foo",
	)
	// y = 2 (inside the class body, not module level) must NOT appear as a
	// KindPyVar, and "inner" must NOT be qualified as "Bar.inner" — it's
	// nested inside a method, not directly inside the class.
	for _, g := range got {
		if g == string(core.KindPyVar)+":y" {
			t.Errorf("class-body assignment leaked as a module var: %v", got)
		}
		if g == string(core.KindPyFunc)+":Bar.inner" {
			t.Errorf("function nested in a method wrongly inherited the class scope: %v", got)
		}
	}
	// "inner" itself should still be found, just unqualified.
	mustContainAll(t, got, string(core.KindPyFunc)+":inner")

	// Calls: baz calls helper(); foo calls nothing (x+1 has no call).
	for _, n := range nodes {
		if n.Name == "Bar.baz" {
			if len(n.Calls) != 1 || n.Calls[0] != "helper" {
				t.Errorf("Bar.baz calls = %v, want [helper]", n.Calls)
			}
		}
	}

	// Package/service/file/tier plumbing.
	for _, n := range nodes {
		if n.Service != "svc" {
			t.Errorf("Service = %q, want svc", n.Service)
		}
		if n.FilePath != "pkg/mod.py" {
			t.Errorf("FilePath = %q, want pkg/mod.py", n.FilePath)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier = %v, want TierVerified", n.Tier)
		}
	}
}

func TestTSExtractor_Python_Determinism(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte("def a():\n    pass\n\nclass B:\n    def c(self):\n        pass\n")
	var first []string
	for i := 0; i < 5; i++ {
		nodes, err := e.Extract("svc", "f.py", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		got := symStrings(nodes)
		if first == nil {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: got %v, want %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: got %v, want %v", i, got, first)
			}
		}
	}
}

func TestTSExtractor_JavaScript(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`function foo(x) {
  return x + 1;
}

class Bar {
  baz() {
    return helper();
  }
}

const qux = (a) => a * 2;
`)
	nodes, err := e.Extract("svc", "src/app.js", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindJSFunc)+":foo",
		string(core.KindJSClass)+":Bar",
		string(core.KindJSMethod)+":Bar.baz",
		string(core.KindJSFunc)+":qux",
	)
	for _, n := range nodes {
		if n.Name == "Bar.baz" {
			if len(n.Calls) != 1 || n.Calls[0] != "helper" {
				t.Errorf("Bar.baz calls = %v, want [helper]", n.Calls)
			}
		}
	}
}

// TestTSExtractor_JavaScript_PrivateAndComputedMethods is the critic-B2
// regression: the incumbent CGO extractor read the method_definition name
// field's raw text regardless of node kind, so #private methods
// (private_property_identifier) and computed names like [Symbol.iterator]
// (computed_property_name) were indexed. A query constrained to
// (property_identifier) silently DELETES them from every modern JS/TS repo
// on the epoch-5 re-extraction. Corpus blindness note: none of the six JS/TS
// calibration repos contains a single #-method (rg-verified), so the
// calibration gate could not catch this — recorded in the plan.
func TestTSExtractor_JavaScript_PrivateAndComputedMethods(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`class Store {
  #load() {
    return 1;
  }
  [Symbol.iterator]() {
    return null;
  }
  get size() {
    return 0;
  }
}
`)
	nodes, err := e.Extract("svc", "src/store.js", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindJSClass)+":Store",
		string(core.KindJSMethod)+":Store.#load",
		string(core.KindJSMethod)+":Store.[Symbol.iterator]",
		string(core.KindJSMethod)+":Store.size",
	)
}

func TestTSExtractor_TypeScript_PrivateAndComputedMethods(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`class Cache<T> {
  #evict(): void {}
  [Symbol.asyncIterator]() {
    return null;
  }
}
`)
	nodes, err := e.Extract("svc", "src/cache.ts", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindJSClass)+":Cache",
		string(core.KindJSMethod)+":Cache.#evict",
		string(core.KindJSMethod)+":Cache.[Symbol.asyncIterator]",
	)
}

func TestTSExtractor_TypeScript(t *testing.T) {
	e, err := NewTSExtractor("javascript") // "javascript" spans .ts/.tsx too
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`interface Point {
  x: number;
}

type ID = string;

enum Color { Red, Green }

class Shape {
  area(): number {
    return 0;
  }
}
`)
	nodes, err := e.Extract("svc", "src/shapes.ts", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindTSInterface)+":Point",
		string(core.KindTSType)+":ID",
		string(core.KindTSEnum)+":Color",
		string(core.KindJSClass)+":Shape",
		string(core.KindJSMethod)+":Shape.area",
	)
}

func TestTSExtractor_TSX(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`interface Props {
  name: string;
}

class Widget {
  render(): number {
    return 1;
  }
}
`)
	nodes, err := e.Extract("svc", "src/widget.tsx", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindTSInterface)+":Props",
		string(core.KindJSClass)+":Widget",
		string(core.KindJSMethod)+":Widget.render",
	)
}

func TestTSExtractor_Ruby(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`module Greeter
  def self.hello(name)
    format(name)
  end
end

class Person
  def initialize(name)
    @name = name
  end

  def greet
    Greeter.hello(@name)
  end
end
`)
	nodes, err := e.Extract("svc", "lib/person.rb", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindRubyModule)+":Greeter",
		string(core.KindRubyMethod)+":Greeter.hello",
		string(core.KindRubyClass)+":Person",
		string(core.KindRubyMethod)+":Person.initialize",
		string(core.KindRubyMethod)+":Person.greet",
	)
	for _, n := range nodes {
		if n.Name == "Person.greet" {
			// LB-Q-ruby: a constant reference is a call target too (design
			// §1.1, tier-C oracle: the grammar's own tags query), so the receiver
			// Greeter is listed with the method it calls.
			if len(n.Calls) != 2 || n.Calls[0] != "Greeter" || n.Calls[1] != "hello" {
				t.Errorf("Person.greet calls = %v, want [Greeter hello]", n.Calls)
			}
		}
	}
}

func TestTSExtractor_Ruby_NestedModuleChaining(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`module Outer
  module Inner
    class Widget
      def run
      end
    end
  end
end
`)
	nodes, err := e.Extract("svc", "lib/nested.rb", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindRubyModule)+":Outer",
		string(core.KindRubyModule)+":Outer::Inner",
		string(core.KindRubyClass)+":Outer::Inner::Widget",
		string(core.KindRubyMethod)+":Outer::Inner::Widget.run",
	)
}

// TestTSExtractor_Ruby_ParityWithIncumbentOnCriticProbes is the M2(a)
// ruling condition: ship-on-tie for Ruby is ratified only if the tie is
// verified real on the critic's construct probes — singleton methods,
// heredoc scanner state (the external scanner's stateful lexing), and
// Struct.new. It ran BOTH extractors (the wazero engine and the in-tree CGO
// incumbent) on the same fixtures; since LB-F393 retired rubyextract.go the
// incumbent's symbol sets on them are written out below (its output at
// e186fbc3), and the dialect is held to them.
func TestTSExtractor_Ruby_ParityWithIncumbentOnCriticProbes(t *testing.T) {
	fixtures := map[string]string{
		"singleton_methods": `class Config
  def self.load(path)
    new
  end

  class << self
    def default
      load("x")
    end
  end

  def save
  end
end
`,
		"heredoc_scanner_state": `class Mailer
  TEMPLATE = <<~BODY
    Hello, def not_a_method(x) — this is heredoc TEXT the scanner must not lex as code.
  BODY

  def deliver
    TEMPLATE
  end
end

def after_heredoc
end
`,
		"struct_new": `Point = Struct.new(:x, :y)

Sized = Struct.new(:w, :h) do
  def area
    w * h
  end
end

class Uses
  def dims
    Point.new(1, 2)
  end
end
`,
	}

	wantConstants := map[string]string{
		"singleton_methods":     "",
		"heredoc_scanner_state": "RubyConstant:Mailer::TEMPLATE",
		"struct_new":            "RubyConstant:Point RubyConstant:Sized",
	}
	// A method defined in a Struct.new block belongs to the constant the struct is
	// assigned to (LB-Q-ruby: a constant is a container), so the dialect names the
	// incumbent's bare `area` Sized.area.
	renamed := map[string]map[string]string{
		"struct_new": {"RubyMethod:Sized.area": "RubyMethod:area"},
	}

	// The CGO incumbent's symbols on each fixture (symStrings order).
	incumbent := map[string][]string{
		"singleton_methods":     {"RubyClass:Config", "RubyMethod:Config.default", "RubyMethod:Config.load", "RubyMethod:Config.save"},
		"heredoc_scanner_state": {"RubyClass:Mailer", "RubyMethod:Mailer.deliver", "RubyMethod:after_heredoc"},
		"struct_new":            {"RubyClass:Uses", "RubyMethod:Uses.dims", "RubyMethod:area"},
	}

	tsE, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}

	for name, src := range fixtures {
		t.Run(name, func(t *testing.T) {
			got, err := tsE.Extract("svc", "probe.rb", []byte(src))
			if err != nil {
				t.Fatalf("tsengine Extract: %v", err)
			}
			// The incumbent emits modules, classes and methods only; the dialect
			// (LB-Q-ruby) also emits constants (Struct.new assignments included),
			// so parity is over the incumbent's three kinds and the extra symbols
			// must be exactly the constants the fixture assigns.
			var g []string
			var extra []string
			for _, s := range symStrings(got) {
				if strings.HasPrefix(s, "RubyConstant:") {
					extra = append(extra, s)
					continue
				}
				if r, ok := renamed[name][s]; ok {
					s = r
				}
				g = append(g, s)
			}
			sort.Strings(g)
			w := incumbent[name]
			if len(g) != len(w) {
				t.Fatalf("symbol set diverges from incumbent:\n wazero    %v\n incumbent %v", g, w)
			}
			for i := range g {
				if g[i] != w[i] {
					t.Fatalf("symbol set diverges from incumbent at %d:\n wazero    %v\n incumbent %v", i, g, w)
				}
			}
			if got, want := strings.Join(extra, " "), wantConstants[name]; got != want {
				t.Fatalf("constants = %q, want %q", got, want)
			}
		})
	}
}

// TestTSExtractor_Python_ParityWithIncumbent: same M2(a) spirit for the
// Python tie — decorated/async defs, nested classes, and module vars,
// compared 1:1 against the CGO incumbent's symbols, written out (its output
// at e186fbc3) since LB-F393 retired pyextract.go.
func TestTSExtractor_Python_ParityWithIncumbent(t *testing.T) {
	src := []byte(`VERSION = "1.0"

def plain():
    pass

async def fetch(url):
    return url

@decorator
def wrapped():
    pass

class Outer:
    @property
    def value(self):
        return 1

    class Inner:
        def m(self):
            pass

    async def poll(self):
        pass
`)
	tsE, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}

	got, err := tsE.Extract("svc", "probe.py", src)
	if err != nil {
		t.Fatalf("tsengine Extract: %v", err)
	}
	// The incumbent predates K18's module symbol (tsextract_module.go): the
	// definitions are compared without it.
	got = slices.DeleteFunc(got, func(n core.SymbolNode) bool { return n.IsModuleSymbol() })
	g := symStrings(got)
	w := []string{
		"PythonClass:Inner", "PythonClass:Outer", "PythonFunc:Inner.m", "PythonFunc:Outer.poll",
		"PythonFunc:Outer.value", "PythonFunc:fetch", "PythonFunc:plain", "PythonFunc:wrapped", "PythonVar:VERSION",
	}
	if len(g) != len(w) {
		t.Fatalf("symbol set diverges from incumbent:\n wazero    %v\n incumbent %v", g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("symbol set diverges from incumbent at %d:\n wazero    %v\n incumbent %v", i, g, w)
		}
	}
}

// TestTSExtractor_Kotlin exercises wave P2's new-language addition (ADR-021
// §4, O69/O81): classes, a nested interface, a named companion object,
// top-level function/property, member methods (qualified), calls, and
// imports — fwcd/tree-sitter-kotlin has no field() names at all, so every
// pattern in kotlin.scm locates the name by node kind/anonymous-keyword
// position rather than a field: constraint (see that file's header).
func TestTSExtractor_Kotlin(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`package com.example

import com.example.util.Helper

class PaymentService(val config: String) {
    fun processPayment(amount: String) {
        validate(amount)
    }

    private fun validate(amount: String): Boolean {
        return amount.isNotEmpty()
    }

    interface Processor {
        fun process(amount: String)
    }

    companion object Factory {
        fun create(): PaymentService = PaymentService("x")
    }
}

object Singleton {
    fun ping() {}
}

val topLevelVar = 42

fun topLevelFunc(x: Int): Int {
    return x + 1
}
`)
	nodes, err := e.Extract("svc", "PaymentService.kt", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindKotlinImport)+":com.example.util.Helper",
		string(core.KindKotlinClass)+":PaymentService",
		string(core.KindKotlinFunction)+":PaymentService.processPayment",
		string(core.KindKotlinFunction)+":PaymentService.validate",
		string(core.KindKotlinInterface)+":PaymentService.Processor",
		string(core.KindKotlinFunction)+":PaymentService.Processor.process",
		string(core.KindKotlinObject)+":PaymentService.Factory",
		string(core.KindKotlinFunction)+":PaymentService.Factory.create",
		string(core.KindKotlinObject)+":Singleton",
		string(core.KindKotlinFunction)+":Singleton.ping",
		string(core.KindKotlinProperty)+":topLevelVar",
		string(core.KindKotlinFunction)+":topLevelFunc",
	)
	for _, n := range nodes {
		if n.Name == "PaymentService.processPayment" {
			if len(n.Calls) != 1 || n.Calls[0] != "validate" {
				t.Errorf("PaymentService.processPayment calls = %v, want [validate]", n.Calls)
			}
		}
	}
}

// TestTSExtractor_Kotlin_PropertyScopes is the property-scoping regression
// (LB-Q-kotlin widened it from top-level only): a module-level property, a
// class member and a primary-constructor val/var parameter are KotlinProperty
// definitions; a local variable inside a function body is not.
func TestTSExtractor_Kotlin_PropertyScopes(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`val topLevel = 1

class Widget(val ctorProp: Int, plain: Int) {
    val memberField = 2

    fun work() {
        val localVar = 3
    }
}
`)
	nodes, err := e.Extract("svc", "widget.kt", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(t, got,
		string(core.KindKotlinProperty)+":topLevel",
		string(core.KindKotlinProperty)+":memberField",
		string(core.KindKotlinProperty)+":ctorProp",
	)
	for _, g := range got {
		for _, leaked := range []string{"localVar", "plain"} {
			if g == string(core.KindKotlinProperty)+":"+leaked {
				t.Errorf("%s leaked as a property: %v", leaked, got)
			}
		}
	}
}

// TestTSExtractor_Kotlin_NestedClassChaining verifies chainContainers (true,
// "." separator — Kotlin nested classes are idiomatic, unlike JS/Python's
// single-level scope).
func TestTSExtractor_Kotlin_NestedClassChaining(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`class Outer {
    class Inner {
        fun run() {}
    }
}
`)
	nodes, err := e.Extract("svc", "nested.kt", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindKotlinClass)+":Outer",
		string(core.KindKotlinClass)+":Outer.Inner",
		string(core.KindKotlinFunction)+":Outer.Inner.run",
	)
}

func TestTSExtractor_Kotlin_KtsExtension(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`plugins {
    kotlin("jvm")
}

fun helper() {}
`)
	nodes, err := e.Extract("svc", "build.gradle.kts", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(t, got, string(core.KindKotlinFunction)+":helper")
}

func TestTSExtractor_Kotlin_Determinism(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`class A {
    fun b() {
        c()
    }
    fun c() {}
}
`)
	var first []string
	for i := 0; i < 5; i++ {
		nodes, err := e.Extract("svc", "f.kt", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		got := symStrings(nodes)
		if first == nil {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: got %v, want %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: got %v, want %v", i, got, first)
			}
		}
	}
}

func TestTSExtractor_EmptyFile(t *testing.T) {
	for _, lang := range []string{"javascript", "python", "ruby", "kotlin"} {
		e, err := NewTSExtractor(lang)
		if err != nil {
			t.Fatalf("NewTSExtractor(%s): %v", lang, err)
		}
		nodes, err := e.Extract("svc", "empty.txt", nil)
		if err != nil {
			t.Fatalf("Extract(%s, empty): %v", lang, err)
		}
		if lang == "python" || lang == "javascript" {
			// K18: an empty Python or JavaScript file is a module all the same
			// (scip-python and scip-typescript define one per document): its
			// module symbol alone.
			if len(nodes) != 1 || !nodes[0].IsModuleSymbol() || nodes[0].Line != 1 {
				t.Errorf("Extract(%s, empty) = %v, want its module symbol alone", lang, nodes)
			}
			continue
		}
		if len(nodes) != 0 {
			t.Errorf("Extract(%s, empty) = %v, want none", lang, nodes)
		}
	}
}

// assertExactSymbols asserts the COMPLETE extracted symbol set — not a
// subset — so both false negatives (missing symbols) and false positives
// (phantom symbols, the failure mode subset assertions can never catch) fail
// the test.
func assertExactSymbols(t *testing.T, nodes []core.SymbolNode, want ...string) {
	t.Helper()
	got := symStrings(nodes)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("symbol set mismatch:\n got  %v\n want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("symbol set mismatch at %d:\n got  %v\n want %v", i, got, want)
		}
	}
}

// Exact-match + negative coverage per language (critic M3): the subset
// assertions above prove recall on the tricky shapes; these prove precision —
// nothing beyond the expected set is emitted (no expression statements, no
// call sites as definitions, no local variables).

func TestTSExtractor_Python_ExactSet(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`TOP = 1

class C:
    attr = 2

    def m(self):
        local = 3
        return helper(local)

def f():
    inner_local = 4
    return inner_local
`)
	nodes, err := e.Extract("svc", "m.py", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Exactly these — NOT local/inner_local (function bodies), NOT helper (a
	// call site, not a definition). The class attribute is C.attr since
	// LB-F242 (universal-ctags tags it as a variable, scip-python defines it);
	// the file's module symbol since K18.
	assertExactSymbols(
		t, nodes,
		"PythonModule:"+core.ModuleSymbolName,
		string(core.KindPyVar)+":TOP",
		string(core.KindPyClass)+":C",
		string(core.KindPyVar)+":C.attr",
		string(core.KindPyFunc)+":C.m",
		string(core.KindPyFunc)+":f",
	)
}

func TestTSExtractor_JavaScript_ExactSet(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`const plain = 5;

function f(x) {
  const g = (y) => y;
  return g(x);
}

class K {
  m() {
    return f(1);
  }
}
`)
	nodes, err := e.Extract("svc", "m.js", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Exactly these — NOT the call sites g(x)/f(1), and NOT the inner arrow
	// binding g: since
	// LB-Q-javascript-rest an arrow binding is a definition only at module
	// scope (a function-local one is a local scip-typescript never reports;
	// the incumbent emitted variable_declarator+arrow anywhere). The file's
	// module symbol since K18 (LB-F381), and plain, a module-level const, since
	// LB-F382.
	assertExactSymbols(
		t, nodes,
		"JSModule:"+core.ModuleSymbolName,
		"JSConst:plain",
		string(core.KindJSFunc)+":f",
		string(core.KindJSClass)+":K",
		string(core.KindJSMethod)+":K.m",
	)
}

// TestTSExtractor_MjsExports is J1 (app-kit O18/O32/O33): find_symbol never
// found either export after ingest because .mjs was absent from the
// extension routing this test now pins directly at the extractor.
func TestTSExtractor_MjsExports(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`export async function createMemoryMeasurements() {
  return { heap: 0 };
}

export function trackOwnedChild(pid) {
  return pid;
}
`)
	nodes, err := e.Extract("svc", "apps/styleguide/browser/memory.mjs", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindJSFunc)+":createMemoryMeasurements",
		string(core.KindJSFunc)+":trackOwnedChild",
	)
	for _, n := range nodes {
		if n.Name == "createMemoryMeasurements" && n.Line != 1 {
			t.Errorf("createMemoryMeasurements line = %d, want 1", n.Line)
		}
		if n.Name == "trackOwnedChild" && n.Line != 5 {
			t.Errorf("trackOwnedChild line = %d, want 5", n.Line)
		}
	}
}

// TestTSExtractor_CjsModuleExports pins the CommonJS sibling: a plain
// function declaration re-exported via `module.exports = {...}` still
// extracts through the .cjs routing (the export object itself is not a new
// definition — the function declaration is what a symbol lookup wants).
func TestTSExtractor_CjsModuleExports(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`function trackOwnedChild(pid) {
  return pid;
}

module.exports = { trackOwnedChild };
`)
	nodes, err := e.Extract("svc", "apps/styleguide/browser/owned_process.cjs", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(t, got, string(core.KindJSFunc)+":trackOwnedChild")
}

// TestTSExtractor_MtsCtsRouting pins the TypeScript module-system pair:
// .mts/.cts parse through the SAME typescript grammar variant .ts already
// uses (interfaces/types included), not the plain JS variant.
func TestTSExtractor_MtsCtsRouting(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`export interface Probe {
  id: string;
}

export function run(p: Probe): void {}
`)
	for _, path := range []string{"src/probe.mts", "src/probe.cts"} {
		nodes, err := e.Extract("svc", path, src)
		if err != nil {
			t.Fatalf("Extract(%s): %v", path, err)
		}
		got := symStrings(nodes)
		mustContainAll(
			t, got,
			string(core.KindTSInterface)+":Probe",
			string(core.KindJSFunc)+":run",
		)
	}
}

func TestTSExtractor_Ruby_ExactSet(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`require "json"

module M
  class C
    attr_reader :name

    def m
      helper(1)
    end
  end
end
`)
	nodes, err := e.Extract("svc", "m.rb", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Exactly these — NOT require/attr_reader/helper (call sites, not
	// definitions). The attr_reader's name is a property (LB-Q-ruby closed the
	// old incumbent-parity gap: attr_accessor-derived names are definitions now).
	assertExactSymbols(
		t, nodes,
		string(core.KindRubyModule)+":M",
		string(core.KindRubyClass)+":M::C",
		string(core.KindRubyMethod)+":M::C.m",
		"RubyProperty:M::C.name",
	)
}

// TestTSExtractor_PathologicalInputRecovery is critic B1+M1 at the extractor
// layer: a pathological file (150KB of `({[` — a truncated minified bundle's
// shape) must (1) abort within the parse deadline instead of holding e.mu at
// 100% CPU for minutes, (2) surface as a counted engine fault (never a
// silent nil,nil that fingerprints the file and poisons the parser forever),
// and (3) leave the extractor healthy — the timed-out module is dead, so the
// NEXT Extract must transparently re-create the parser and succeed.
func TestTSExtractor_PathologicalInputRecovery(t *testing.T) {
	saved := tsengine.ParseTimeout
	tsengine.ParseTimeout = 300 * time.Millisecond
	defer func() { tsengine.ParseTimeout = saved }()

	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}

	// Warm the parser with a healthy file first, so the fault genuinely
	// poisons a LIVE cached parser (the A/B/A shape), not a cold one.
	if _, err := e.Extract("svc", "a.js", []byte("function a() { return 1; }\n")); err != nil {
		t.Fatalf("warm Extract: %v", err)
	}
	if got := e.EngineFaults(); got != 0 {
		t.Fatalf("EngineFaults after healthy parse = %d, want 0", got)
	}

	pathological := bytes.Repeat([]byte("({["), 50000)
	start := time.Now()
	_, err = e.Extract("svc", "bundle.min.js", pathological)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Extract of pathological input returned no error")
	}
	if !errors.Is(err, ErrTSEngineFault) {
		t.Fatalf("error = %v, want errors.Is(err, ErrTSEngineFault)", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("Extract took %v — parse deadline did not bound it", elapsed)
	}
	if got := e.EngineFaults(); got != 1 {
		t.Fatalf("EngineFaults = %d, want 1", got)
	}

	// Recovery: the poisoned parser was dropped; this must rebuild and parse.
	nodes, err := e.Extract("svc", "b.js", []byte("function b() { return 2; }\n"))
	if err != nil {
		t.Fatalf("Extract after fault: %v", err)
	}
	found := false
	for _, n := range nodes {
		if n.Name == "b" {
			found = true
		}
	}
	if !found {
		t.Fatalf("recreated parser did not extract symbols: %v", symStrings(nodes))
	}
}

func TestEngineProvenance(t *testing.T) {
	got := EngineProvenance()
	for _, ext := range []string{".py", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".rb", ".kt", ".kts"} {
		if got[ext] != "tree-sitter-wasm" {
			t.Errorf("EngineProvenance()[%q] = %q, want tree-sitter-wasm", ext, got[ext])
		}
	}
}

package extractor

// scaladialect_test.go — LB-Q-scala (design §13.2 steps 10, 12 and 15). The per-fixture goldens run in
// the generic TestDialectGoldens (dialects/scala/testdata); this file pins the contracts
// scalaextract_test.go held for the CGO walker the dialect retires (the four SourceKinds byte-identical,
// Owner.method qualification, tier, plumbing, determinism, an empty file and the call edges), run against
// both extractors while the walker exists, and what the dialect adds beyond the walker by name.

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// scalaContractFixture and scalaContractCalls are the walker's own fixtures, kept so its contract
// survives its removal.
const scalaContractFixture = `package com.example

class MyClass {
  def myMethod(x: Int): String = x.toString
}

object MyObject {
  def apply(): Unit = {}
}

trait MyTrait {
  def doThing(): Unit
}

def topFunc(): Int = 42
`

const scalaContractCalls = `
def callee(): Unit = {}
def helper(): Unit = {}
def caller(): Unit = {
  callee()
  helper()
}
`

func scalaContractExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("scala")
	if err != nil {
		t.Fatalf("NewTSExtractor(scala): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"dialect": d, "walker": NewScalaExtractor()}
}

func scalaExtract(t *testing.T, e Extractor, path, src string) (map[string]core.SymbolNode, []core.SymbolNode) {
	t.Helper()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract %s: %v", path, err)
	}
	m := map[string]core.SymbolNode{}
	for _, n := range nodes {
		m[n.Name] = n
	}
	return m, nodes
}

// TestScalaDialectKeepsIncumbentContracts: the names and the four kinds (ScalaClass, ScalaObject,
// ScalaTrait, ScalaDef, byte-identical, §11 step 3), Owner.method qualification, tier, plumbing, EndLine,
// determinism, an empty file and the call edges the walker's tests pinned.
func TestScalaDialectKeepsIncumbentContracts(t *testing.T) {
	for who, e := range scalaContractExtractors(t) {
		t.Run(who+"/symbols", func(t *testing.T) {
			seen, nodes := scalaExtract(t, e, "src/Main.scala", scalaContractFixture)
			for _, w := range []struct {
				name string
				kind core.SourceKind
			}{
				{"MyClass", core.KindScalaClass},
				{"MyClass.myMethod", core.KindScalaDef},
				{"MyObject", core.KindScalaObject},
				{"MyObject.apply", core.KindScalaDef},
				{"MyTrait", core.KindScalaTrait},
				{"topFunc", core.KindScalaDef},
			} {
				n, ok := seen[w.name]
				if !ok {
					t.Errorf("missing %s", w.name)
					continue
				}
				if n.Kind != w.kind || n.Tier != core.TierVerified || n.Service != "svc" || n.FilePath != "src/Main.scala" || n.EndLine <= 0 {
					t.Errorf("%s: %+v", w.name, n)
				}
			}
			for _, n := range nodes {
				if n.Tier != core.TierVerified || n.EndLine <= 0 {
					t.Errorf("%s: tier %q EndLine %d", n.Name, n.Tier, n.EndLine)
				}
			}
		})
		t.Run(who+"/qualification", func(t *testing.T) {
			seen, _ := scalaExtract(t, e, "Greeter.scala", `class Greeter {
  def greet(name: String): String = "hello " + name
  def farewell(): Unit = {}
}`)
			for _, name := range []string{"Greeter.greet", "Greeter.farewell"} {
				if _, ok := seen[name]; !ok {
					t.Errorf("missing %s", name)
				}
			}
			if _, ok := seen["greet"]; ok {
				t.Error("an unqualified greet appeared")
			}
		})
		t.Run(who+"/determinism", func(t *testing.T) {
			_, a := scalaExtract(t, e, "Main.scala", scalaContractFixture)
			_, b := scalaExtract(t, e, "Main.scala", scalaContractFixture)
			if len(a) != len(b) {
				t.Fatalf("%d symbols, then %d", len(a), len(b))
			}
			for i := range a {
				if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
					t.Errorf("index %d: %+v vs %+v", i, a[i], b[i])
				}
			}
		})
		t.Run(who+"/empty", func(t *testing.T) {
			if _, nodes := scalaExtract(t, e, "empty.scala", ""); len(nodes) != 0 {
				t.Errorf("an empty file gave %d symbols", len(nodes))
			}
		})
		t.Run(who+"/calls", func(t *testing.T) {
			seen, _ := scalaExtract(t, e, "calls.scala", scalaContractCalls)
			if got := seen["caller"].Calls; !slices.Equal(got, []string{"callee", "helper"}) {
				t.Errorf("caller calls %v, want [callee helper]", got)
			}
			if got := seen["callee"].Calls; len(got) != 0 {
				t.Errorf("callee calls %v, want none", got)
			}
		})
	}
}

// TestScalaDialectBeyondIncumbent names what the dialect reads that scalaextract.go did not: abstract
// members, member vals and vars, a field class parameter (written val/var, or any parameter of a case
// class), Scala 3 enums and their cases, named givens, type definitions, a secondary constructor, an
// operator name, a backticked name without its backticks, a package object, nested containers chained
// (Outer.Inner.m), extension methods under their template, a call named by its callee's last identifier
// and given to the innermost def, and a local val left out.
func TestScalaDialectBeyondIncumbent(t *testing.T) {
	e, err := NewTSExtractor("scala")
	if err != nil {
		t.Fatalf("NewTSExtractor(scala): %v", err)
	}
	defer e.Close()
	src := `package p

trait Repo {
  def find(id: Int): Option[String]
  val size: Int
  var cursor: Int
  type Key
}

case class User(id: Int, var name: String)
class Account(val owner: User, balance: Long) {
  def this(owner: User) = this(owner, 0L)
  def +(other: Account): Account = other
  def ` + "`special case`" + `(): Unit = ()
}

object Outer {
  object Inner {
    def m(): Int = {
      val local = 1
      def helper(): Int = a.b.compute[Int](local)
      helper()
    }
  }
  extension (s: String) def shout: String = s.toUpperCase
  given intShow: Show[Int] with
    def show(x: Int): String = x.toString
}

enum Color:
  case Red, Green

package object util {
  def tool(): Unit = ()
}
`
	nodes, err := e.Extract("svc", "p/Repo.scala", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	type key struct {
		name string
		kind core.SourceKind
	}
	seen := map[key]core.SymbolNode{}
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}
	for _, w := range []key{
		{"Repo.find", core.KindScalaDef},
		{"size", "ScalaVal"},
		{"cursor", "ScalaVar"},
		{"Key", "ScalaType"},
		{"id", "ScalaVal"},
		{"name", "ScalaVar"},
		{"owner", "ScalaVal"},
		{"Account.this", "ScalaConstructor"},
		{"Account.+", core.KindScalaDef},
		{"Account.special case", core.KindScalaDef},
		{"Outer.Inner", core.KindScalaObject},
		{"Outer.Inner.m", core.KindScalaDef},
		{"helper", core.KindScalaDef},
		{"Outer.shout", core.KindScalaDef},
		{"Outer.intShow", "ScalaGiven"},
		{"Outer.intShow.show", core.KindScalaDef},
		{"Color", "ScalaEnum"},
		{"Color.Red", "ScalaEnumCase"},
		{"Color.Green", "ScalaEnumCase"},
		{"util", core.KindScalaObject},
		{"util.tool", core.KindScalaDef},
	} {
		if _, ok := seen[w]; !ok {
			t.Errorf("missing %s %s", w.kind, w.name)
		}
	}
	for _, n := range nodes {
		if n.Name == "local" || n.Name == "balance" {
			t.Errorf("%s is no definition: %+v", n.Name, n)
		}
	}
	if got := seen[key{"helper", core.KindScalaDef}].Calls; !slices.Equal(got, []string{"compute"}) {
		t.Errorf("helper calls %v, want [compute]", got)
	}
	if got := seen[key{"Outer.Inner.m", core.KindScalaDef}].Calls; !slices.Equal(got, []string{"helper"}) {
		t.Errorf("Outer.Inner.m calls %v, want [helper] (compute is the inner def's)", got)
	}
	if got := seen[key{"Account.this", "ScalaConstructor"}].Calls; len(got) != 0 {
		t.Errorf("Account.this calls %v: this(...) is the constructor's own invocation, no call", got)
	}
	if sig := seen[key{"Repo.find", core.KindScalaDef}].Signature; sig != "def find(id: Int): Option[String]" {
		t.Errorf("Repo.find signature %q", sig)
	}
	if sig := seen[key{"Outer.Inner.m", core.KindScalaDef}].Signature; sig != "def m(): Int" {
		t.Errorf("Outer.Inner.m signature %q", sig)
	}
}

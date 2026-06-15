package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// scalaFixture is a deterministic Scala source fragment exercising all symbol
// types the ScalaExtractor emits: class, object, trait, and def (including a
// method qualified with its enclosing class name).
const scalaFixture = `package com.example

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

func TestScalaExtractor_BasicSymbols(t *testing.T) {
	e := NewScalaExtractor()
	nodes, err := e.Extract("svc", "src/Main.scala", []byte(scalaFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct {
		name string
		kind core.SourceKind
	}
	seen := make(map[key]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}

	want := []key{
		{"MyClass", core.KindScalaClass},
		{"MyClass.myMethod", core.KindScalaDef},
		{"MyObject", core.KindScalaObject},
		{"MyObject.apply", core.KindScalaDef},
		{"MyTrait", core.KindScalaTrait},
		{"topFunc", core.KindScalaDef},
	}

	for _, w := range want {
		n, ok := seen[w]
		if !ok {
			t.Errorf("missing symbol name=%q kind=%q", w.name, w.kind)
			continue
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", w.name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", w.name, "svc", n.Service)
		}
		if n.FilePath != "src/Main.scala" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/Main.scala", n.FilePath)
		}
	}
}

func TestScalaExtractor_MethodQualification(t *testing.T) {
	src := []byte(`class Greeter {
  def greet(name: String): String = "hello " + name
  def farewell(): Unit = {}
}`)
	e := NewScalaExtractor()
	nodes, err := e.Extract("svc", "Greeter.scala", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["Greeter.greet"]; !ok {
		t.Error("expected qualified symbol Greeter.greet")
	}
	if _, ok := byName["Greeter.farewell"]; !ok {
		t.Error("expected qualified symbol Greeter.farewell")
	}
	if _, ok := byName["greet"]; ok {
		t.Error("unqualified greet should not appear")
	}
}

func TestScalaExtractor_TierVerified(t *testing.T) {
	e := NewScalaExtractor()
	nodes, err := e.Extract("svc", "Main.scala", []byte(scalaFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestScalaExtractor_Determinism(t *testing.T) {
	e := NewScalaExtractor()
	a, err := e.Extract("svc", "Main.scala", []byte(scalaFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "Main.scala", []byte(scalaFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: first run %d symbols, second %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestScalaExtractor_EmptySource(t *testing.T) {
	e := NewScalaExtractor()
	nodes, err := e.Extract("svc", "empty.scala", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}

// scalaCallFixture has caller() calling callee() and helper() (bare names, so
// call_expression.function is an identifier). callee() and helper() make no calls.
const scalaCallFixture = `
def callee(): Unit = {}
def helper(): Unit = {}
def caller(): Unit = {
  callee()
  helper()
}
`

func TestScalaExtractor_CallEdges(t *testing.T) {
	e := NewScalaExtractor()
	nodes, err := e.Extract("svc", "calls.scala", []byte(scalaCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("caller calls callee and helper", func(t *testing.T) {
		n, ok := byName["caller"]
		if !ok {
			t.Fatal("expected symbol caller")
		}
		wantCalls := map[string]bool{"callee": true, "helper": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in caller.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("caller.Calls missing %q", missing)
		}
	})

	t.Run("caller Calls is sorted", func(t *testing.T) {
		n := byName["caller"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})
}

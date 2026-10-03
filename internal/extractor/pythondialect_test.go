package extractor

// pythondialect_test.go — LB-F65. The python dialect left TestTagsDialectParity's
// legacy projection (see migratedTagsDialects) because its call text changed:
// calls.qualify compacts it and drops a callee that ends in no name, and a
// decorator's or a module-level assignment's calls now belong to the definition
// they decorate or define. Its goldens are now the full ones TestDialectGoldens
// checks. This test keeps the guarantee that mattered: every symbol the
// pre-LB-F65 goldens listed (name, kind, first line) is still emitted. The table
// is those goldens' symbol lists as they were at fdd2bb59; a symbol may be added
// to the fixtures' goldens, never removed from here (the Kotlin and Ruby
// precedent).

import (
	"path/filepath"
	"testing"
)

type legacyPythonSymbol struct {
	name, kind string
	line       int
}

var legacyPythonSymbols = map[string][]legacyPythonSymbol{
	"shapes.py": {
		{"LIMIT", "PythonVar", 8},
		{"counter", "PythonVar", 10},
		{"top", "PythonFunc", 13},
		{"fetch", "PythonFunc", 20},
		{"decorated", "PythonFunc", 25},
		{"Base", "PythonClass", 32},
		{"Base.__init__", "PythonFunc", 35},
		{"Base.size", "PythonFunc", 40},
		{"Base.build", "PythonFunc", 44},
		{"Inner", "PythonClass", 47},
		{"Inner.deep", "PythonFunc", 48},
		{"local", "PythonFunc", 49},
		{"Child", "PythonClass", 54},
		{"Child.method", "PythonFunc", 55},
		{"outer", "PythonFunc", 60},
		{"Local", "PythonClass", 61},
		{"Local.m", "PythonFunc", 62},
	},
	"upgrade.py": {
		{"IntOrStr", "PythonTypeAlias", 4},
		{"Model", "PythonClass", 7},
		{"count", "PythonVar", 9},
		{"Model.label", "PythonFunc", 13},
		{"Model.label", "PythonFunc", 17},
		{"Model.handle", "PythonFunc", 24},
	},
	"broken.py": {
		{"good", "PythonFunc", 1},
		{"broken", "PythonFunc", 4},
		{"Survivor", "PythonClass", 7},
		{"Survivor.ok", "PythonFunc", 8},
	},
}

func TestPythonDialectKeepsLegacySymbols(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor(python): %v", err)
	}
	defer e.Close()
	for fixture, want := range legacyPythonSymbols {
		t.Run(fixture, func(t *testing.T) {
			got := extractGolden(t, e, filepath.Join("dialects", "python", "testdata", fixture))
			have := map[legacyPythonSymbol]bool{}
			for _, s := range got.Symbols {
				have[legacyPythonSymbol{s.Name, s.Kind, s.Line}] = true
			}
			for _, w := range want {
				if !have[w] {
					t.Errorf("%s: legacy symbol %s %s at line %d is no longer emitted", fixture, w.kind, w.name, w.line)
				}
			}
		})
	}
}

// TestPythonKeepsCallsAnOracleCannotType (LB-F182, LB-Q-python-rest, O450): a
// call scip-python writes no occurrence for stays a call of the definition
// that makes it. Python's call_precision .8751 is held down by 1,661
// fathom-only pairs, every one a call the source writes (O450's classes: a
// fixture parameter's receiver, a local the oracle cannot type, a module it
// cannot resolve, an @overload implementation it gives no definition); the
// extractor must never drop one to raise the figure.
func TestPythonKeepsCallsAnOracleCannotType(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor(python): %v", err)
	}
	defer e.Close()
	src := `import os
import pytest
from typing import overload


def test_get(client, app):
    resp = client.get("/")
    local = app.test_client()
    local.post("/x")
    assert pytest.raises(ValueError)
    return os.path.dirname(resp)


@overload
def pick(x: int) -> int: ...
@overload
def pick(x: str) -> str: ...
def pick(x):
    return helper(x).strip()


class Item:
    tags: set[str] = set()
`
	nodes, err := e.Extract("s", "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]map[string]bool{}
	for _, n := range nodes {
		if calls[n.Name] == nil {
			calls[n.Name] = map[string]bool{}
		}
		for _, c := range n.Calls {
			calls[n.Name][c] = true
		}
	}
	for _, w := range []struct{ def, call string }{
		{"test_get", "client.get"}, // an unannotated fixture parameter's receiver
		{"test_get", "app.test_client"},
		{"test_get", "local.post"},    // a local the oracle cannot type
		{"test_get", "pytest.raises"}, // a module it does not resolve
		{"test_get", "os.path.dirname"},
		{"pick", "helper"}, // the implementation of an @overload: the oracle holds no definition for it
		{"pick", "helper().strip"},
		{"tags", "set"}, // an annotated class attribute owns its calls (LB-F65)
	} {
		if !calls[w.def][w.call] {
			t.Errorf("%s no longer calls %q (calls: %v)", w.def, w.call, calls[w.def])
		}
	}
}

// TestPythonDialectMembers (LB-F241, LB-F242, wave lx): every attribute and field
// scip-python 0.6.6 defines is a symbol named Owner.name, once per (Owner,
// name), at its first definition in source order; a shape universal-ctags
// tags as variable is a PythonVar, the others PythonField. A local, a later
// assignment to an attribute already defined, and an attribute on a local
// are no symbols; the annotated class attribute keeps its legacy bare name.
func TestPythonDialectMembers(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor(python): %v", err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", "python", "testdata", "members.py"))
	have := map[legacyPythonSymbol]int{}
	byName := map[string]int{}
	for _, s := range got.Symbols {
		have[legacyPythonSymbol{s.Name, s.Kind, s.Line}]++
		byName[s.Name]++
	}
	for _, w := range []legacyPythonSymbol{
		{"flag", "PythonVar", 6}, // a module-level assignment inside an `if`
		{"flag", "PythonVar", 8}, // and its `else` (module scope is never de-duplicated)
		{"first", "PythonVar", 9},
		{"second", "PythonVar", 9},
		{"head", "PythonVar", 10}, // a module-level chained assignment's later target is no symbol
		{"Model.name", "PythonField", 19},
		{"count", "PythonVar", 20}, // legacy: the annotated class attribute stays bare
		{"Model.untyped", "PythonVar", 21},
		{"Model.c1", "PythonVar", 22},
		{"Model.c2", "PythonField", 22},
		{"Model.t1", "PythonVar", 23},
		{"Model.t2", "PythonVar", 23},
		{"Model.cond", "PythonVar", 25},
		{"Model.value", "PythonField", 28},
		{"Model.client", "PythonField", 29},
		{"Model.typed", "PythonField", 30},
		{"Model.a", "PythonField", 31},
		{"Model.b", "PythonField", 31},
		{"Model.late", "PythonField", 39},
		{"Model.deep", "PythonField", 42},
		{"Inner.x", "PythonField", 53},
		{"Local.field", "PythonField", 58},
		{"Local.y", "PythonField", 61},
	} {
		if have[w] != 1 {
			t.Errorf("%s %s at line %d emitted %d times, want once", w.kind, w.name, w.line, have[w])
		}
	}
	for _, absent := range []string{"tail", "local", "Model.local", "Model.attr", "Model.count", "attr", "Model.label", "Model.x", "Model.y", "Model.field"} {
		if n := byName[absent]; n != 0 && absent != "Model.label" {
			t.Errorf("%s emitted %d times, want none", absent, n)
		}
	}
	if n := byName["Model.label"]; n != 1 {
		t.Errorf("Model.label emitted %d times, want once (the method; `label = property(label)` names it again)", n)
	}
	for _, once := range []string{"Model.value", "Model.client", "Model.name"} {
		if byName[once] != 1 {
			t.Errorf("%s emitted %d times, want once (the first assignment only)", once, byName[once])
		}
	}
}

// TestPythonMemberCallsFollowTheTruth (LB-F242, trap (i)): once an attribute
// is a symbol, scip-python's call truth gives the calls on its first
// definition's line to it (the harness's D5 line span), so the extractor
// attaches them there too; a later assignment to the same attribute is a
// reference, and its calls stay with the method.
func TestPythonMemberCallsFollowTheTruth(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor(python): %v", err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", "python", "testdata", "members.py"))
	calls := map[string]map[string]bool{}
	for _, s := range got.Symbols {
		if calls[s.Name] == nil {
			calls[s.Name] = map[string]bool{}
		}
		for _, c := range s.Calls {
			calls[s.Name][c] = true
		}
	}
	for _, w := range []struct{ def, call string }{
		{"Model.client", "connect"},
		{"Model.reset", "reconnect"},
		{"Model.untyped", "make_default"},
		{"flag", "load_flag"},
		{"work", "compute"},
	} {
		if !calls[w.def][w.call] {
			t.Errorf("%s does not call %q (calls: %v)", w.def, w.call, calls[w.def])
		}
	}
	for _, w := range []struct{ def, call string }{
		{"Model.__init__", "connect"},
		{"Model.client", "reconnect"},
	} {
		if calls[w.def][w.call] {
			t.Errorf("%s calls %q, want it owned elsewhere", w.def, w.call)
		}
	}
}

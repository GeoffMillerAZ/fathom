package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestCHeaderRouting pins CT-3 (docs/plans/cpp-toolchain.md): .h is a
// polyglot extension — LLVM-style codebases keep every C++ header in .h —
// so the header extractor sniffs for C++ signals and routes to the C++
// grammar, while a plain C header keeps C extraction (and C kinds).
func TestCHeaderRouting(t *testing.T) {
	e := NewCHeaderExtractor()

	t.Run("class header routes to cpp", func(t *testing.T) {
		src := []byte(`
#pragma once
namespace ns {
class Widget {
public:
  void render() { }
};
}
`)
		nodes, err := e.Extract("svc", "include/widget.h", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		wantKinds := map[string]core.SourceKind{
			"ns":            core.KindCppNamespace,
			"Widget":        core.KindCppClass,
			"Widget.render": core.KindCppFunc,
		}
		got := map[string]core.SourceKind{}
		for _, n := range nodes {
			got[n.Name] = n.Kind
		}
		for name, kind := range wantKinds {
			if got[name] != kind {
				t.Errorf("symbol %q: want kind %q, got %q (all: %v)", name, kind, got[name], names(nodes))
			}
		}
	})

	t.Run("template header routes to cpp", func(t *testing.T) {
		src := []byte(`
template <typename T>
T identity(T v) { return v; }
`)
		nodes, err := e.Extract("svc", "include/id.h", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		var found bool
		for _, n := range nodes {
			if n.Name == "identity" && n.Kind == core.KindCppFunc {
				found = true
			}
		}
		if !found {
			t.Errorf("want identity as KindCppFunc, have %v", names(nodes))
		}
	})

	t.Run("plain C header keeps C kinds", func(t *testing.T) {
		src := []byte(`
#ifndef POINT_H
#define POINT_H
struct Point { int x; int y; };
static inline int point_sum(Point p) { return p.x + p.y; }
#endif
`)
		nodes, err := e.Extract("svc", "include/point.h", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		got := map[string]core.SourceKind{}
		for _, n := range nodes {
			got[n.Name] = n.Kind
		}
		if got["Point"] != core.KindCStruct {
			t.Errorf("Point: want KindCStruct, got %q", got["Point"])
		}
		if got["point_sum"] != core.KindCFunc {
			t.Errorf("point_sum: want KindCFunc, got %q", got["point_sum"])
		}
		for _, n := range nodes {
			if n.Kind == core.KindCppClass || n.Kind == core.KindCppFunc {
				t.Errorf("C header emitted C++ kind %q for %q", n.Kind, n.Name)
			}
		}
	})

	t.Run("identifier containing class as substring stays C", func(t *testing.T) {
		src := []byte(`
static inline int subclass_count(void) { return 0; }
`)
		nodes, err := e.Extract("svc", "include/sub.h", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		for _, n := range nodes {
			if n.Name == "subclass_count" && n.Kind != core.KindCFunc {
				t.Errorf("subclass_count: want KindCFunc (word-boundary sniff), got %q", n.Kind)
			}
		}
	})
}

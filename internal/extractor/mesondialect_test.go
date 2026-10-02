package extractor

// mesondialect_test.go — LB-Q-meson (design §13.2 steps 10 and 12). The per-fixture goldens run in the generic
// TestDialectGoldens (dialects/meson/testdata); this file holds the obligations a golden cannot express: the
// parser survives a syntax error, the manifest claims the three files the queue entry names, and a target is
// named as Meson names it, by its first string, never by the variable the call is assigned to.

import (
	"os"
	"slices"
	"testing"
)

func TestMesonDialect_SurvivesSyntaxError(t *testing.T) {
	e, err := NewTSExtractor("meson")
	if err != nil {
		t.Fatalf("NewTSExtractor(meson): %v", err)
	}
	defer e.Close()
	src, err := os.ReadFile("dialects/meson/testdata/broken.build")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := e.Extract("svc", "meson.build", src)
	if err != nil {
		t.Fatalf("Extract must not error on a recoverable syntax error: %v", err)
	}
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if want := []string{"broken", "first", "first", "third", "after_error", "last"}; !slices.Equal(names, want) { // first: the target and the variable that holds it
		t.Fatalf("symbols around the error = %v, want %v", names, want)
	}
}

// TestMesonDialect_TargetIsNamedByItsString: a target is named as Meson names it (its first string, what
// `meson introspect --targets` lists); the variable it is assigned to is a second symbol of another kind,
// because that is the name a later link_with spells.
func TestMesonDialect_TargetIsNamedByItsString(t *testing.T) {
	e, err := NewTSExtractor("meson")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", "meson.build", []byte("libfoo = static_library('foo', 'foo.c')\nexe = executable('foo-tool', 'm.c', link_with : libfoo)\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var calls []string
	for _, n := range nodes {
		got[n.Name] = string(n.Kind)
		if n.Name == "foo-tool" {
			calls = n.Calls
		}
	}
	want := map[string]string{"foo": "MesonTarget", "libfoo": "MesonVariable", "foo-tool": "MesonTarget", "exe": "MesonVariable"}
	if len(got) != len(want) {
		t.Fatalf("symbols %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: kind %q, want %q", k, got[k], v)
		}
	}
	if !slices.Equal(calls, []string{"libfoo"}) {
		t.Errorf("foo-tool.Calls = %v, want [libfoo] (the variable the link spells)", calls)
	}
}

func TestMesonDialect_ClaimsItsFiles(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID != "meson" {
			continue
		}
		for _, b := range []string{"meson.build", "meson.options", "meson_options.txt"} {
			if !slices.Contains(d.Match.Basenames, b) {
				t.Errorf("basename %s not claimed", b)
			}
		}
		return
	}
	t.Fatal("meson dialect not registered")
}

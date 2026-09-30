package fusion_test

// tcl_ingest_test.go — LB-Q-tcl, refused (design §14.2): the tcl dialect is built and
// measured, but 62.2% of the 704 tcllib files and 63.4% of the 123 Tk library files the
// harness reads (78 of 123) parse with an error under the pinned grammar
// (tree-sitter-grammars/tree-sitter-tcl 850a72ab, upstream main's head), far over the 5%
// refusal line. A refused language "stays disclosed as unsupported": ingest does not route
// a Tcl file to the dialect, extracts nothing from it, and names it in
// Report.UnsupportedLanguages. The dialect itself stays tested (TestDialectGoldens/tcl)
// for the day a grammar parses the subjects under the line.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestIngest_TclIsRefusedAndDisclosed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/util.tcl", "namespace eval ::util {\n    proc trim {s} {\n        return [string trim $s]\n    }\n}\n")
	writeFile(t, dir, "lib/pkg-1.0.tm", "package provide pkg 1.0\nproc ::pkg::hello {} {\n    puts hi\n}\n")
	writeFile(t, dir, "ui/main.tk", "proc show {} {\n    puts ui\n}\n")
	writeFile(t, dir, "README.md", "# Acme\n")

	if lang, ok := connector.UnsupportedLanguageForPath("lib/util.tcl"); !ok || lang != "tcl" {
		t.Fatalf("UnsupportedLanguageForPath(.tcl) = %q, %v; want tcl disclosed (refused, §14.2)", lang, ok)
	}
	obs, report := ingestDir(t, dir, "tcl-refused")
	for _, o := range obs {
		ref := strings.SplitN(o.SourceRef, ":", 2)[0]
		if strings.HasSuffix(ref, ".tcl") || strings.HasSuffix(ref, ".tm") || strings.HasSuffix(ref, ".tk") {
			t.Errorf("a refused language was extracted: %s %s (%s)", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	if got, want := report.UnsupportedLanguages["tcl"], 1; got != want {
		t.Errorf("UnsupportedLanguages[tcl] = %d, want %d (%v)", got, want, report.UnsupportedLanguages)
	}
}

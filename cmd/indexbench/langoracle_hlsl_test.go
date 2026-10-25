package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// HLSL's #include resolves through the include-path strategy (the includer's
// directory, then the source roots), so the file resolver serves it and
// clang-includes' imports are its file-edge truth (LB-F57, LB-Q-hlsl).
func TestFileResolveLangServesHLSL(t *testing.T) {
	if lang, ok := fileResolveLang["hlsl"]; !ok || lang != "hlsl" {
		t.Fatalf("fileResolveLang[hlsl] = %q, %v; want hlsl", lang, ok)
	}
}

// TestLangProbeNamesTheCandidate (H5-R6): lang-probe reads a file as the
// extractor does and says which text that was: the file as written when it
// parses clean (the pre-parse step is not run), else the pre-parse
// candidate the extractor takes, k of n.
func TestLangProbeNamesTheCandidate(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"clean.hlsl", "float f(float x) { return x; }\n", "; parse: read as written (no error)"},
		{"split.hlsl", "float f(float x, float t)\n{\n#if A\n    if (x > t)\n#else\n    if (x < t)\n#endif\n    {\n        x = 0;\n    }\n    return x;\n}\n", "; parse: pre-parse candidate 1 of 1 (the file holds"},
	} {
		target, err := extractor.ProbeTargetFor("hlsl", c.name)
		if err != nil {
			t.Fatal(err)
		}
		lines, err := probeLines(target, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(lines, "\n"), c.want) {
			t.Errorf("%s: probe lacks %q:\n%s", c.name, c.want, strings.Join(lines[:min(4, len(lines))], "\n"))
		}
	}
}

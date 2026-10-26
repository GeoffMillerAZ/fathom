package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestCmdLangProbe (LB-F20, design §10, §13 step 12): the probe prints a
// file's tree and every capture its dialect's queries make, within 200
// lines, and refuses an unknown dialect.
func TestCmdLangProbe(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "m.py")
	if err := os.WriteFile(small, []byte("class Foo:\n    def bar(self):\n        return baz(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := cmdLangProbe([]string{"-dialect", "python", small}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{
		"; captures",
		"@definition.class L1 \"class Foo:",
		"@name L1 \"Foo\"",
		"@definition.function L2",
		"; tree",
		"(class_definition L1",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("probe output lacks %q:\n%s", want, s)
		}
	}

	// A large file is capped at 200 lines in total, and says so.
	var big strings.Builder
	for i := 0; i < 400; i++ {
		big.WriteString("def f")
		big.WriteString(strings.Repeat("x", i%7))
		big.WriteString("():\n    return g(1)\n")
	}
	bigPath := filepath.Join(dir, "big.py")
	if err := os.WriteFile(bigPath, []byte(big.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := cmdLangProbe([]string{"-dialect", "python", bigPath}, &out, &errb); code != 0 {
		t.Fatalf("big: exit %d: %s", code, errb.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) > 200 {
		t.Errorf("probe printed %d lines, cap is 200", len(lines))
	}
	if !strings.Contains(out.String(), "more captures not shown") || !strings.Contains(out.String(), "more nodes not shown") {
		t.Errorf("capped output does not say what it dropped:\n%s", lines[len(lines)-1])
	}

	errb.Reset()
	if code := cmdLangProbe([]string{"-dialect", "cobol", small}, &out, &errb); code != 2 {
		t.Errorf("unknown dialect: exit %d, want 2", code)
	}
	if code := cmdLangProbe([]string{"-dialect", "python"}, &out, &errb); code != 2 {
		t.Errorf("no file: exit %d, want 2", code)
	}
}

func TestLangProbeReportsErrorsWithoutPreprocess(t *testing.T) {
	target, err := extractor.ProbeTargetFor("c", "broken.c")
	if err != nil {
		t.Fatal(err)
	}
	lines, err := probeLines(target, []byte("void broken( {"))
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(lines, "\n")
	if !strings.Contains(out, "read as written, in error (") || strings.Contains(out, "pre-parse candidates") {
		t.Fatalf("misleading parse diagnostic:\n%s", out)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleRangeCompanionNamesItsKinds (wave lf4, item shell): the
// range companion is not clang's alone. A companion that names its own kind
// map (`kinds`, its kind -> the manifest's canonical) and what its ranges are
// (`scope`) is scored for any dialect: a bash subject whose L1 oracle writes no
// end line (universal-ctags 6.2.1 writes none for Sh) takes range_agreement
// from an independent parser's function ranges, paired by L1's own pairing
// (same file and name, |Δline| <= 2), and the note names that parser and its
// scope instead of clang's. Only the range counts are taken.
func TestCmdLangOracleRangeCompanionNamesItsKinds(t *testing.T) {
	subject := t.TempDir()
	// one: lines 2-4; two: lines 5-8; extensionless bin/tool (D4): lines 2-4.
	if err := os.WriteFile(filepath.Join(subject, "lib.sh"), []byte("#!/usr/bin/env bash\none() {\n  echo 1\n}\ntwo() {\n  echo 2\n  echo 3\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(subject, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "bin", "tool"), []byte("#!/bin/sh\nmain() {\n  :\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "defs.json")
	// The L1 oracle: definitions with no end line (ctags' shape).
	defs := `{"tool":"defs-no-end","tier":"B","version":"1","definitions":[
	 {"path":"lib.sh","name":"one","kind":"function","line":2},
	 {"path":"lib.sh","name":"two","kind":"function","line":5},
	 {"path":"bin/tool","name":"main","kind":"function","line":2}]}`
	if err := os.WriteFile(tool, []byte(defs), 0o644); err != nil {
		t.Fatal(err)
	}
	// one agrees; two's last line is three off; bin/tool's main agrees; a
	// definition in a file the run does not measure is dropped. The kind is the
	// companion's own spelling, mapped by its kinds.
	ranges := `{"tool":"shfmt","tier":"B","version":"v3","units":2,"errored":0,"kinds":{"FuncDecl":"function"},
	 "scope":"the first and last line of each function declaration","ranges":[
	 {"path":"lib.sh","name":"one","kind":"FuncDecl","line":2,"end":4},
	 {"path":"lib.sh","name":"two","kind":"FuncDecl","line":5,"end":11},
	 {"path":"bin/tool","name":"main","kind":"FuncDecl","line":2,"end":4},
	 {"path":"other.sh","name":"gone","kind":"FuncDecl","line":1,"end":2}]}`
	if err := os.WriteFile(filepath.Join(dir, "defs.ranges.json"), []byte(ranges), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "bash.json")
	args := []string{"-dialect", "bash", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "bash", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-tool-oracle", tool,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if r.Counts[langbench.CntMatched] != 3 {
		t.Errorf("matched %d, want 3 (the L1 oracle's, bin/tool's main included)\n%s", r.Counts[langbench.CntMatched], raw)
	}
	if r.Counts[langbench.CntRangeEligible] != 3 || r.Counts[langbench.CntRangeOK] != 2 {
		t.Errorf("range_eligible %d range_ok %d, want 3 and 2\n%s", r.Counts[langbench.CntRangeEligible], r.Counts[langbench.CntRangeOK], raw)
	}
	if len(r.NotMeasured) != 0 {
		t.Errorf("not_measured = %v, want none once a companion measures the range", r.NotMeasured)
	}
	if !bytes.Contains(raw, []byte("range_agreement measured against shfmt (tier B, v3: 2 files, 0 it could not parse): the first and last line of each function declaration")) {
		t.Errorf("the companion and its scope are not disclosed:\n%s", raw)
	}
	if bytes.Contains(raw, []byte("translation units")) {
		t.Errorf("a non-clang companion is described as clang's:\n%s", raw)
	}
}

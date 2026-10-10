package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleSamePackageFileEdges (LB-F433, K19; O519): an imports
// dump's entry marked same_package (a file reaching a top-level name of its
// own package with no import, as Kotlin allows) is file-edge truth and never
// an import statement: it joins file_edges_truth, leaves every
// import_resolution count as the import entries alone set it, is never an
// unmatched import, and the reading discloses how many pairs it added.
func TestCmdLangOracleSamePackageFileEdges(t *testing.T) {
	run := func(withPkg bool) (langbench.Reading, []byte, unmatchedFile) {
		t.Helper()
		subject := copyFixture(t, "ktfixture")
		entries := `{"path": "com/example/app/Main.kt", "line": 3, "to_path": "com/example/util/Helper.kt", "module": "com.example.util.Helper"}`
		if withPkg {
			entries += `,{"path": "com/example/util/Strings.kt", "line": 3, "to_path": "com/example/util/Helper.kt", "module": "com.example.util.helperTop", "same_package": true}`
		}
		dump := filepath.Join(t.TempDir(), "kotlin-psi.json")
		if err := os.WriteFile(dump, []byte(`{"tool": "kotlin-psi", "version": "2.3.20", "tier": "B", "imports": [`+entries+`]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "kotlin.json")
		um := filepath.Join(t.TempDir(), "unmatched.json")
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle([]string{"-dialect", "kotlin", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
			"-tool-oracle", dump, "-unmatched", um, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		r, raw := readValidReading(t, out)
		return r, raw, readUnmatched(t, um)
	}
	base, _, _ := run(false)
	r, raw, u := run(true)
	if got, want := r.Counts[langbench.CntFileEdgesTruth], base.Counts[langbench.CntFileEdgesTruth]+1; got != want {
		t.Errorf("file_edges_truth = %d, want %d (the same-package pair is truth)\n%s", got, want, raw)
	}
	for _, k := range []string{langbench.CntImports, langbench.CntImportsInRepo, langbench.CntImportOracleUnpaired, "import_edges_truth", "import_edges_agree", "imports_resolved_agree", "imports_unresolved"} {
		if r.Counts[k] != base.Counts[k] {
			t.Errorf("%s = %d, want %d: a same-package pair is no import statement", k, r.Counts[k], base.Counts[k])
		}
	}
	for _, m := range u.Imports {
		if m.Path == "com/example/util/Strings.kt" {
			t.Errorf("a same-package pair listed as an unmatched import: %+v", m)
		}
	}
	if !bytes.Contains(raw, []byte("same-package")) {
		t.Errorf("the same-package truth is not disclosed:\n%s", raw)
	}
}

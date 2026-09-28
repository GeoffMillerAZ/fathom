package main

// certify_service_test.go — SR-62 (surface reckoning F-41): every certify
// and tests verb that takes no --service resolves it through
// defaultTestsCoverageService, which reads the repo's own stanza before the
// directory name (serviceCandidates, the fix SR-1 made for the edit hook's
// spec_refs). A checkout whose directory is not its service id
// (…/sextant, registered as "fathom") must not ask the store for the
// directory's name.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCertifyVerbsDefaultToTheStanzaNamedService(t *testing.T) {
	ceil := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(ceil); err == nil {
		ceil = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceil)

	dir := filepath.Join(t.TempDir(), "checkout-b7")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stanza := "<!-- fathom:begin v3 sha256:0 -->\n## fathom code intelligence\n\nThis repo is indexed by fathom as service \"widgets\". Route by\nquestion shape.\n<!-- fathom:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(stanza), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if got := defaultTestsCoverageService(); got != "widgets" {
		t.Errorf("default service = %q, want the stanza-named \"widgets\", not the directory's \"checkout-b7\"", got)
	}
}

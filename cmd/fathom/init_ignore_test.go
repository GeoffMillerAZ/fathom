package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFathomIgnoreDefaults_BuildTrees pins CT-8 (docs/plans/cpp-toolchain.md):
// init's .fathomignore template excludes C-family build trees, so a
// configured CMake/Ninja build directory never double-indexes copied headers
// and generated sources.
func TestFathomIgnoreDefaults_BuildTrees(t *testing.T) {
	root := t.TempDir()
	if err := writeFathomIgnore(root, io.Discard); err != nil {
		t.Fatalf("writeFathomIgnore: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".fathomignore"))
	if err != nil {
		t.Fatalf("read .fathomignore: %v", err)
	}
	content := string(raw)
	for _, want := range []string{"build/", "out/", "cmake-build-*/"} {
		found := false
		for _, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(".fathomignore template missing %q; content:\n%s", want, content)
		}
	}
}

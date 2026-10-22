//go:build darwin || linux

package conserve

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDeprecatedSymbolNamesSkipsFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "a.go"), 0600); err != nil {
		t.Fatal(err)
	}
	writeGoFile(t, root, "b.go", "package b\n// Deprecated: use New.\nfunc Old() {}\n")
	names, err := deprecatedSymbolNames(context.Background(), root)
	if err != nil || len(names) != 1 || !names["Old"] {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

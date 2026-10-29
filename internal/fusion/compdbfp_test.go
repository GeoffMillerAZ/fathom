package fusion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCompDB(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "build")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db := `[{"directory":"` + filepath.ToSlash(root) + `","command":"nvcc -I inc -I /elsewhere/outside -DX=2 -c src/a.cu","file":"src/a.cu"}]`
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), []byte(db), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompileDBFingerprint(t *testing.T) {
	empty, err := CompileDBFingerprint(t.TempDir())
	if err != nil || empty.Databases != 0 || empty.Fingerprint != "" {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
	a, b := t.TempDir(), t.TempDir()
	writeCompDB(t, a)
	writeCompDB(t, b)
	ia, _ := CompileDBFingerprint(a)
	ib, _ := CompileDBFingerprint(b)
	if ia.Databases != 1 || ia.Entries != 1 || len(ia.Fingerprint) != 64 {
		t.Fatalf("identity = %+v", ia)
	}
	if ia != ib {
		t.Fatalf("same database under two roots differs: %+v vs %+v", ia, ib)
	}
	// A changed include root changes the fingerprint.
	p := filepath.Join(b, "build", "compile_commands.json")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(raw), "-I inc", "-I inc2", 1)), 0o644)
	ic, _ := CompileDBFingerprint(b)
	if ic.Fingerprint == ia.Fingerprint {
		t.Fatal("fingerprint ignored include roots")
	}
}

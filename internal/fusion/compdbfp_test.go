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

func writeCompDBEntries(t *testing.T, root string, cmds ...string) {
	t.Helper()
	dir := filepath.Join(root, "build")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var parts []string
	for i, c := range cmds {
		parts = append(parts, `{"directory":"`+filepath.ToSlash(root)+`","command":"`+c+`","file":"src/f`+string(rune('a'+i))+`.cu"}`)
	}
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), []byte("["+strings.Join(parts, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fingerprintOf(t *testing.T, root string) string {
	t.Helper()
	id, err := CompileDBFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	return id.Fingerprint
}

func TestCompileDBFingerprintOrderAndTarget(t *testing.T) {
	one, two := "nvcc -I x -c src/fa.cu", "nvcc -I y -c src/fb.cu"
	a, b := t.TempDir(), t.TempDir()
	writeCompDBEntries(t, a, one, two)
	writeCompDBEntries(t, b, two, one)
	// Entry order changes the header-root unions.
	if fingerprintOf(t, a) == fingerprintOf(t, b) {
		t.Fatal("swapped entry order gave the same fingerprint")
	}
	// An explicit target differs by platform.
	lin, mac := t.TempDir(), t.TempDir()
	writeCompDBEntries(t, lin, "clang --target=x86_64-linux-gnu -I x -c src/fa.cu")
	writeCompDBEntries(t, mac, "clang --target=arm64-apple-darwin -I x -c src/fa.cu")
	if fingerprintOf(t, lin) == fingerprintOf(t, mac) {
		t.Fatal("linux and darwin targets gave the same fingerprint")
	}
	// No target: the host default is part of the identity.
	bare := t.TempDir()
	writeCompDBEntries(t, bare, one)
	old := cHostGOOS
	defer func() { cHostGOOS = old }()
	cHostGOOS = "linux"
	fl := fingerprintOf(t, bare)
	cHostGOOS = "darwin"
	fd := fingerprintOf(t, bare)
	if fl == fd {
		t.Fatal("host default target not in the fingerprint")
	}
}

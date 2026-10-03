package fsfault

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// CS-8: a denied path fails every routed call with the error a refused user
// gets from the operating system — whoever runs the test — and only that
// path and what lies beneath it.
func TestDenyRefusesThePathAndWhatIsBeneathIt(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.md")
	sub := filepath.Join(dir, "sub")
	inner := filepath.Join(sub, "inner.md")
	sibling := filepath.Join(dir, "sibling.md")
	for _, p := range []string{file, inner, sibling} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("denied", func(t *testing.T) {
		Deny(t, file)
		Deny(t, sub)
		refused := func(what string, err error) {
			t.Helper()
			var pe *fs.PathError
			if !errors.As(err, &pe) || !errors.Is(err, syscall.EACCES) || !errors.Is(err, fs.ErrPermission) || !os.IsPermission(err) {
				t.Errorf("%s: err = %v, want a *fs.PathError carrying EACCES", what, err)
			}
			if err != nil && err.Error() != "open "+pe.Path+": permission denied" {
				t.Errorf("%s: the wording is the operating system's own, got %q", what, err)
			}
		}
		_, err := Open(file)
		refused("Open", err)
		_, err = ReadFile(file)
		refused("ReadFile", err)
		_, err = OpenFile(file, os.O_RDONLY, 0)
		refused("OpenFile", err)
		refused("WriteFile", WriteFile(file, []byte("y"), 0o644))
		_, err = ReadDir(sub)
		refused("ReadDir of a denied directory", err)
		_, err = ReadFile(inner)
		refused("ReadFile beneath a denied directory", err)
		refused("Check", Check("open", sub))
		if b, err := ReadFile(sibling); err != nil || string(b) != "x" {
			t.Errorf("a sibling is untouched: %q, %v", b, err)
		}
		if _, err := ReadDir(dir); err != nil {
			t.Errorf("the parent of a denied path still lists: %v", err)
		}
	})

	// The denial ends with the test that made it.
	if b, err := ReadFile(file); err != nil || string(b) != "x" {
		t.Fatalf("after the test: %q, %v", b, err)
	}
	if _, err := ReadDir(sub); err != nil {
		t.Fatalf("after the test: %v", err)
	}
	if active.Load() != 0 || len(denied) != 0 {
		t.Fatalf("registrations left behind: active=%d denied=%v", active.Load(), denied)
	}
}

// A reader that resolved symlinks in its root and a test that named the
// unresolved path (or the other way round) still meet.
func TestDenyMatchesThroughASymlinkedDirectory(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "f.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	Deny(t, filepath.Join(link, "f.md"))
	if _, err := ReadFile(filepath.Join(target, "f.md")); !errors.Is(err, syscall.EACCES) {
		t.Errorf("the resolved path: %v, want EACCES", err)
	}
	if _, err := ReadFile(filepath.Join(link, "f.md")); !errors.Is(err, syscall.EACCES) {
		t.Errorf("the linked path: %v, want EACCES", err)
	}
	Deny(t, filepath.Join(target, "later.md")) // not there yet: resolved through its parent
	if err := WriteFile(filepath.Join(link, "later.md"), []byte("y"), 0o644); !errors.Is(err, syscall.EACCES) {
		t.Errorf("a path that does not exist yet: %v, want EACCES", err)
	}
}

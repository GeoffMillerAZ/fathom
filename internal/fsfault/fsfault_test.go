package fsfault

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

// walkTree is the tree the walk tests read: a readable directory, a file,
// a directory the walker means to list but cannot ("denied") and one the
// walker skips by name before it would list it ("skipped").
func walkTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{"a/x.md", "b.md", "denied/y.md", "skipped/z.md"} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// visitLog records what a walker's function saw: each visit, and each error
// its err branch received. swallow makes that branch return nil (a walker
// that tolerates the refusal) instead of the error.
type visitLog struct {
	root    string
	swallow bool
	seen    []string
}

func (v *visitLog) note(path string, err error) error {
	rel, _ := filepath.Rel(v.root, path)
	rel = filepath.ToSlash(rel)
	if err != nil {
		what := "other"
		if errors.Is(err, syscall.EACCES) {
			what = "EACCES"
		}
		v.seen = append(v.seen, "err "+rel+" "+what)
		if v.swallow {
			return nil
		}
		return err
	}
	v.seen = append(v.seen, rel)
	return nil
}

func (v *visitLog) walkDir(path string, d fs.DirEntry, err error) error {
	if err == nil && d.IsDir() && d.Name() == "skipped" {
		return filepath.SkipDir
	}
	return v.note(path, err)
}

func (v *visitLog) walk(path string, info fs.FileInfo, err error) error {
	if err == nil && info.IsDir() && info.Name() == "skipped" {
		return filepath.SkipDir
	}
	return v.note(path, err)
}

// realRefusal chmods denied and skipped to 000 for a run of walk — the
// operating system's own refusal, which only a non-root walker meets.
func realRefusal(t *testing.T, root string, walk func()) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root lists a mode-000 directory; the fsfault runs above stand for root")
	}
	for _, d := range []string{"denied", "skipped"} {
		p := filepath.Join(root, d)
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
	}
	walk()
}

// CS-8 review: a denied directory reaches the walker's err branch — the
// second call WalkDir makes when a listing fails — never as an error
// returned past it from the first visit, so a walker whose err branch
// wrongly swallowed refusals fails its test. Nothing beneath it is visited,
// a directory the walker skips is never listed, and the calls match what a
// real mode-000 refusal produces.
func TestWalkDirFuncHandsADeniedDirectoryToTheErrBranch(t *testing.T) {
	want := []string{".", "a", "a/x.md", "b.md", "denied", "err denied EACCES"}
	run := func(t *testing.T, root string, swallow bool, wrap bool) ([]string, error) {
		v := &visitLog{root: root, swallow: swallow}
		fn := fs.WalkDirFunc(v.walkDir)
		if wrap {
			fn = WalkDirFunc(fn)
		}
		err := filepath.WalkDir(root, fn)
		return v.seen, err
	}
	for _, swallow := range []bool{false, true} {
		t.Run(map[bool]string{false: "the err branch returns the refusal", true: "the err branch swallows it"}[swallow], func(t *testing.T) {
			root := walkTree(t)
			Deny(t, filepath.Join(root, "denied"))
			Deny(t, filepath.Join(root, "skipped"))
			seen, err := run(t, root, swallow, true)
			if swallow == (err != nil) || (err != nil && !errors.Is(err, syscall.EACCES)) {
				t.Fatalf("walk err = %v (swallow %v)", err, swallow)
			}
			// Either way nothing beneath denied is visited, and skipped —
			// denied too — is never listed: the walker skips it first.
			if !reflect.DeepEqual(seen, want) {
				t.Fatalf("visits = %q, want %q", seen, want)
			}
			t.Run("matches a real refusal", func(t *testing.T) {
				root := walkTree(t)
				realRefusal(t, root, func() {
					real, rerr := run(t, root, swallow, false)
					if !reflect.DeepEqual(real, seen) || (rerr == nil) != (err == nil) {
						t.Fatalf("a mode-000 directory: visits %q (err %v), fsfault: %q (err %v)", real, rerr, seen, err)
					}
				})
			})
		})
	}

	// With nothing denied the wrapper is the walk itself.
	root := walkTree(t)
	plain, perr := run(t, root, false, false)
	wrapped, werr := run(t, root, false, true)
	if perr != nil || werr != nil || !reflect.DeepEqual(plain, wrapped) {
		t.Fatalf("nothing denied: plain %q (%v), wrapped %q (%v)", plain, perr, wrapped, werr)
	}
}

// The same for filepath.Walk, which lists a directory before its one visit:
// the refusal arrives with that visit, before the walker's own skip test,
// as it does from the operating system.
func TestWalkFuncHandsADeniedDirectoryToTheErrBranch(t *testing.T) {
	run := func(t *testing.T, root string, swallow bool, wrap bool) ([]string, error) {
		v := &visitLog{root: root, swallow: swallow}
		fn := filepath.WalkFunc(v.walk)
		if wrap {
			fn = WalkFunc(fn)
		}
		err := filepath.Walk(root, fn)
		return v.seen, err
	}
	for _, tc := range []struct {
		swallow bool
		want    []string
	}{
		{false, []string{".", "a", "a/x.md", "b.md", "err denied EACCES"}},
		{true, []string{".", "a", "a/x.md", "b.md", "err denied EACCES", "err skipped EACCES"}},
	} {
		t.Run(map[bool]string{false: "the err branch returns the refusal", true: "the err branch swallows it"}[tc.swallow], func(t *testing.T) {
			root := walkTree(t)
			Deny(t, filepath.Join(root, "denied"))
			Deny(t, filepath.Join(root, "skipped"))
			seen, err := run(t, root, tc.swallow, true)
			if tc.swallow == (err != nil) || (err != nil && !errors.Is(err, syscall.EACCES)) {
				t.Fatalf("walk err = %v (swallow %v)", err, tc.swallow)
			}
			if !reflect.DeepEqual(seen, tc.want) {
				t.Fatalf("visits = %q, want %q", seen, tc.want)
			}
			t.Run("matches a real refusal", func(t *testing.T) {
				root := walkTree(t)
				realRefusal(t, root, func() {
					real, rerr := run(t, root, tc.swallow, false)
					if !reflect.DeepEqual(real, seen) || (rerr == nil) != (err == nil) {
						t.Fatalf("a mode-000 directory: visits %q (err %v), fsfault: %q (err %v)", real, rerr, seen, err)
					}
				})
			})
		})
	}

	root := walkTree(t)
	plain, perr := run(t, root, false, false)
	wrapped, werr := run(t, root, false, true)
	if perr != nil || werr != nil || !reflect.DeepEqual(plain, wrapped) {
		t.Fatalf("nothing denied: plain %q (%v), wrapped %q (%v)", plain, perr, wrapped, werr)
	}
}

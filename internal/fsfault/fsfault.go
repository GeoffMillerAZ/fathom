// Package fsfault is the seam a test uses to make a path fail to open,
// list, read or write — for every user.
//
// A chmod 000 stages "the file is there and cannot be read" for a normal
// user only: root's DAC override reads a mode-000 file, lists a mode-000
// directory and writes a mode-0444 file. A test that staged its unreadable
// file that way passed on a Mac and failed as root on Linux, which is how
// the Claude Code and Codex cloud session VMs run `task test` (CS-8,
// docs/plans/cloud-sessions.md).
//
// A reader whose unreadable-file handling a test pins goes through Open,
// ReadFile, ReadDir, WriteFile or OpenFile here, or asks Check before
// descending into a directory it walks. Deny makes those fail with the
// error the operating system gives a refused user — a *fs.PathError
// carrying EACCES, "permission denied" — on the denied path and everything
// beneath it, whoever runs the test. With nothing denied (every production
// process) each call is the os call itself after one atomic load.
package fsfault

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	active atomic.Int32 // live Deny registrations; 0 skips every lookup
	mu     sync.RWMutex
	denied = map[string]int{} // cleaned absolute path → registrations
)

// Check returns the error an operation op on name meets: a *fs.PathError
// carrying EACCES when name, or a directory above it, is denied; nil
// otherwise. A walker calls it for each directory it is about to list.
func Check(op, name string) error {
	if active.Load() == 0 {
		return nil
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, p := range forms(name) {
		for q := p; ; {
			if denied[q] > 0 {
				return &fs.PathError{Op: op, Path: name, Err: syscall.EACCES}
			}
			parent := filepath.Dir(q)
			if parent == q {
				break
			}
			q = parent
		}
	}
	return nil
}

// Open is os.Open behind Check.
func Open(name string) (*os.File, error) {
	if err := Check("open", name); err != nil {
		return nil, err
	}
	return os.Open(name)
}

// OpenFile is os.OpenFile behind Check.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if err := Check("open", name); err != nil {
		return nil, err
	}
	return os.OpenFile(name, flag, perm)
}

// ReadFile is os.ReadFile behind Check.
func ReadFile(name string) ([]byte, error) {
	if err := Check("open", name); err != nil {
		return nil, err
	}
	return os.ReadFile(name)
}

// ReadDir is os.ReadDir behind Check.
func ReadDir(name string) ([]os.DirEntry, error) {
	if err := Check("open", name); err != nil {
		return nil, err
	}
	return os.ReadDir(name)
}

// WriteFile is os.WriteFile behind Check.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	if err := Check("open", name); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

// TB is the part of testing.TB that Deny needs, so this package stays out
// of the production binary's import of package testing.
type TB interface {
	Helper()
	Cleanup(func())
}

// Deny makes every Open, OpenFile, ReadFile, ReadDir, WriteFile and Check
// of path — and of anything beneath it — fail with EACCES until t ends: the
// unreadable file or directory a chmod 000 stages for a normal user,
// staged for root too. Safe beside parallel tests: denials are keyed by
// path, and each test's paths are its own.
func Deny(t TB, path string) {
	t.Helper()
	keys := forms(path)
	mu.Lock()
	for _, k := range keys {
		denied[k]++
	}
	mu.Unlock()
	active.Add(1)
	t.Cleanup(func() {
		mu.Lock()
		for _, k := range keys {
			if denied[k]--; denied[k] <= 0 {
				delete(denied, k)
			}
		}
		mu.Unlock()
		active.Add(-1)
	})
}

// forms is name's cleaned absolute path and, when it differs, the same
// path with symlinks resolved (a macOS temp dir under /var is
// /private/var) — through its parent when name itself does not exist yet —
// so a reader that resolved its root and a test that did not still meet.
// Only reached while a denial is live.
func forms(name string) []string {
	p, err := filepath.Abs(name)
	if err != nil {
		p = filepath.Clean(name)
	}
	out := []string{p}
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		if d, derr := filepath.EvalSymlinks(filepath.Dir(p)); derr == nil {
			r, err = filepath.Join(d, filepath.Base(p)), nil
		}
	}
	if err == nil && r != p {
		out = append(out, r)
	}
	return out
}

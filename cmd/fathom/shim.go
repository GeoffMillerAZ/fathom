package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// The PATH shim (plan IO-20; observation O2 in platform-engineering's
// observations, the fleet audit's anomaly 4): every hook and gate that
// spells an absolute binary path drifts the moment the checkout moves or
// the deploy lands elsewhere — and a gate that cannot be invoked looks
// exactly like a gate that passed. The shim is one stable name on PATH
// (~/.local/bin/fathom) pointing at the deployed binary, installed by init
// and reconciled thereafter; hooks and stanzas spell `fathom`, never a
// checkout path.

// deployedBinaryPath is where `task deploy` installs the daemon's binary.
func deployedBinaryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "libexec", "fathom", "fathom")
}

func shimPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "bin", "fathom")
}

// installShim links ~/.local/bin/fathom at the deployed binary when present,
// else at the currently-running executable — and REFUSES to clobber a real
// (non-symlink) file it did not create. Returns what it did, for init's
// report line. w is init's write door (init_dryrun.go); nil is a plain
// real run.
func installShim(w *initWrites) (string, error) {
	shim := shimPath()
	if shim == "" {
		return "", fmt.Errorf("no home directory")
	}
	target := deployedBinaryPath()
	if _, err := os.Stat(target); err != nil {
		self, serr := os.Executable()
		if serr != nil {
			return "", fmt.Errorf("no deployed binary and cannot resolve this executable: %v", serr)
		}
		target = self
	}
	if fi, err := os.Lstat(shim); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return "", fmt.Errorf("%s exists and is a real file, not a symlink — refusing to clobber; move it aside if the name should belong to fathom", shim)
		}
		if existing, rerr := os.Readlink(shim); rerr == nil && existing == target {
			return "shim current: " + shim + " → " + target, nil
		}
		if err := w.remove(shim); err != nil {
			return "", err
		}
	}
	if err := w.mkdirAll(filepath.Dir(shim)); err != nil {
		return "", err
	}
	if err := w.symlink(target, shim); err != nil {
		return "", err
	}
	return "shim installed: " + shim + " → " + target, nil
}

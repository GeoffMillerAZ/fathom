package lang

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

var (
	cacheMu  sync.Mutex
	cacheDir string
)

// EnableCompilationCache turns on wazero's on-disk compilation cache for the
// process, under dir. It is off by default. The cache holds machine code the
// process will execute, so dir must be an absolute path that no group or
// other user can write and, on unix, that no other user owns (an owner can
// rewrite it whatever its mode); a directory that does not exist is created
// with mode 0700. It must be called before the first Reader is built, because the
// runtime is made once, and it errors on a second, different directory.
// Calling it again with the same directory is a no-op.
func EnableCompilationCache(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return fmt.Errorf("lang: compilation cache needs an absolute directory, got %q", dir)
	}
	dir = filepath.Clean(dir)
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheDir != "" {
		if cacheDir == dir {
			return nil
		}
		return fmt.Errorf("lang: compilation cache already enabled in %q; one process has one cache directory", cacheDir)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("lang: compilation cache: %w", err)
	}
	if err := privateDir(dir); err != nil {
		return err
	}
	vdir, err := tsengine.CompilationCacheVersionDir(dir)
	if err != nil {
		return fmt.Errorf("lang: %w", err)
	}
	if err := os.MkdirAll(vdir, 0o700); err != nil {
		return fmt.Errorf("lang: compilation cache: %w", err)
	}
	if err := privateDir(vdir); err != nil {
		return err
	}
	if err := tsengine.EnableCompilationCacheDir(vdir); err != nil {
		return fmt.Errorf("lang: %w", err)
	}
	cacheDir = dir
	return nil
}

// privateDir refuses a directory (after symlinks) that is not a directory, that
// a group or other user can write, or that, on unix, another user owns.
func privateDir(dir string) error {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("lang: compilation cache: %w", err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return fmt.Errorf("lang: compilation cache: %w", err)
	}
	if !fi.IsDir() {
		return errors.New("lang: compilation cache: " + dir + " is not a directory")
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("lang: compilation cache: %s is group or world writable (%v); it holds machine code", dir, fi.Mode().Perm())
	}
	if ownedByOther(fi) {
		return fmt.Errorf("lang: compilation cache: %s is owned by another user; it holds machine code", dir)
	}
	return nil
}

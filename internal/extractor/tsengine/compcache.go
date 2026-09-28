package tsengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// CompilationCacheMaxAge is how long a cached compiled module may go
// unwritten before `fathom gc` ages it out (ADR-047 decision 13). A grammar
// still in use is recompiled once and cached again.
const CompilationCacheMaxAge = 30 * 24 * time.Hour

var (
	cacheMu     sync.Mutex
	cacheDir    string // "" = no compilation cache (the default)
	runtimeMade bool
)

// wazeroVersion is the go.mod version of github.com/tetratelabs/wazero.
// A constant, not debug.ReadBuildInfo: test binaries carry no dependency
// list. TestWazeroVersionMatchesGoMod keeps it in step with go.mod.
const wazeroVersion = "v1.12.0"

// WazeroVersion is the linked wazero version: the cache directory's last
// element, so a wazero upgrade never reads another version's machine code
// and gc ages the old directory out whole.
func WazeroVersion() string { return wazeroVersion }

// compilationCacheRoot is ~/.fathom/cache/wazero under home.
func compilationCacheRoot(home string) string {
	return filepath.Join(home, ".fathom", "cache", "wazero")
}

// compilationCacheDir is the one place wazero's compilation cache may live
// (LB-F26, design §8.3): <home>/.fathom/cache/wazero/<wazero version>/, at
// user scope. home must be an absolute path; there is no way to name any
// other directory, so a repository's configuration cannot point the daemon
// at machine code it supplied.
func compilationCacheDir(home string) (string, error) {
	if home == "" || !filepath.IsAbs(home) {
		return "", fmt.Errorf("tsengine: compilation cache needs an absolute home, got %q", home)
	}
	v := WazeroVersion()
	if v == "" || strings.ContainsAny(v, `/\`) || v == "." || v == ".." {
		return "", fmt.Errorf("tsengine: wazero version %q unusable as a cache directory", v)
	}
	return filepath.Join(compilationCacheRoot(filepath.Clean(home)), v), nil
}

// EnableCompilationCache turns wazero's on-disk compilation cache on for
// this process, under compilationCacheDir(home). Only the managed daemon
// calls it; every other process (the CLI, tests, an unmanaged serve-http)
// compiles in memory and writes nothing. It must run before the first
// parser is made: the process runtime is built once.
func EnableCompilationCache(home string) error {
	dir, err := compilationCacheDir(home)
	if err != nil {
		return err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if runtimeMade {
		return fmt.Errorf("tsengine: compilation cache enabled after the runtime was built; call it at daemon start")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("tsengine: compilation cache: %w", err)
	}
	cacheDir = dir
	return nil
}

// CompilationCacheDir is the cache directory in force, or "" when off.
func CompilationCacheDir() string {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return cacheDir
}

// newRuntime builds a wazero runtime with the engine's configuration and
// the WASI shim, caching compiled code under dir when dir is not empty.
func newRuntime(ctx context.Context, dir string) (wazero.Runtime, wazero.CompilationCache, error) {
	cfg := wazero.NewRuntimeConfig()
	if compilerSupported() {
		cfg = wazero.NewRuntimeConfigCompiler()
	} else {
		cfg = wazero.NewRuntimeConfigInterpreter()
	}
	// Deliberately NO WithCloseOnContextDone: parse cancellation is
	// enforced inside the guest (fathom_parse_bounded, grammars/budget.c)
	// via tree-sitter's own progress callback, so wazero's loop-back-edge
	// termination checks — measured ~3x slower corpus-wide when tried —
	// are not needed. See DefaultParseTimeout's doc.
	cfg = cfg.WithMemoryLimitPages(MemoryLimitPages)
	var cache wazero.CompilationCache
	if dir != "" {
		c, err := wazero.NewCompilationCacheWithDir(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("tsengine: compilation cache %s: %w", dir, err)
		}
		cache = c
		cfg = cfg.WithCompilationCache(cache)
	}
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		return nil, nil, fmt.Errorf("tsengine: instantiate WASI shim: %w", err)
	}
	return r, cache, nil
}

// PrunableCompilationCache lists what `fathom gc` ages out of home's
// compilation cache at now (ADR-047 decision 13): every directory of
// another wazero version, and every entry of this version's directory not
// written for CompilationCacheMaxAge. Sorted.
func PrunableCompilationCache(home string, now time.Time) []string {
	root := compilationCacheRoot(home)
	vers, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	cur := WazeroVersion()
	var out []string
	for _, v := range vers {
		p := filepath.Join(root, v.Name())
		if v.Name() != cur {
			out = append(out, p)
			continue
		}
		if !v.IsDir() {
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, ierr := d.Info(); ierr == nil && now.Sub(info.ModTime()) > CompilationCacheMaxAge {
				out = append(out, path)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

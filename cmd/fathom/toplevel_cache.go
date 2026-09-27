package main

// toplevel_cache.go — gitToplevel memoized in process and cached per cwd
// across hook processes (docs/design/responsible-tenant.md §11.3, RT-130).
// A cached answer is keyed by the cwd and holds for 10 minutes while its
// evidence stands: a toplevel answer while the .git nearest the cwd is
// still <top>/.git, the same file (device and inode) — a nested `git
// init`, a re-init or a moved repository misses; .git's mtime is not
// evidence, since every index refresh (git status, git diff) moves it — and
// a "not a repository" answer while no .git exists at the cwd or above it
// (a `git init` misses). A miss forks git once and writes the answer back
// by rename.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

// toplevelTTL bounds a cached toplevel (§11.3: 10 min).
const toplevelTTL = 10 * time.Minute

// Injectable for tests: the git fork and the clock.
var (
	gitToplevelExec = gitToplevelUncached
	toplevelNow     = time.Now
)

// toplevelMemo is the in-process memo: cwd → entry.
var toplevelMemo sync.Map

type toplevelEntry struct {
	Cwd    string    `json:"cwd"`
	Top    string    `json:"top"`
	GitDev uint64    `json:"git_dev,omitempty"` // <top>/.git's device and inode; 0 for "" answers
	GitIno uint64    `json:"git_ino,omitempty"`
	At     time.Time `json:"at"`
}

// gitToplevelCached is gitToplevel through the in-process memo and the
// shared per-cwd cache.
func gitToplevelCached(dir string) string {
	now := toplevelNow()
	if v, ok := toplevelMemo.Load(dir); ok {
		if e := v.(toplevelEntry); toplevelValid(e, now) {
			return e.Top
		}
	}
	path := toplevelCachePath(dir)
	if b, err := os.ReadFile(path); err == nil {
		var e toplevelEntry
		if json.Unmarshal(b, &e) == nil && e.Cwd == dir && toplevelValid(e, now) {
			toplevelMemo.Store(dir, e)
			return e.Top
		}
	}
	top := gitToplevelExec(dir)
	e := toplevelEntry{Cwd: dir, Top: top, At: now}
	if top != "" {
		id := identify(filepath.Join(top, ".git")) // unmanaged.go's (device, inode)
		if !id.ok {
			return top // no evidence to key on: answer, cache nothing
		}
		e.GitDev, e.GitIno = id.dev, id.ino
	}
	toplevelMemo.Store(dir, e)
	if b, err := json.Marshal(e); err == nil {
		if err := os.MkdirAll(hookcache.ToplevelDir, 0o700); err == nil {
			tmp := fmt.Sprintf("%s.tmp.%d.%d", path, os.Getpid(), time.Now().UnixNano())
			if os.WriteFile(tmp, b, 0o600) == nil {
				if os.Rename(tmp, path) != nil {
					_ = os.Remove(tmp)
				}
			}
		}
	}
	return top
}

// toplevelValid reports whether e's answer still stands at now.
func toplevelValid(e toplevelEntry, now time.Time) bool {
	if now.Sub(e.At) >= toplevelTTL || now.Before(e.At) {
		return false
	}
	// The nearest .git at or above the cwd (a few stats, no fork): for a
	// toplevel answer it must be the recorded <top>/.git; for a "not a
	// repository" answer there must be none.
	for d := e.Cwd; ; {
		if id := identify(filepath.Join(d, ".git")); id.ok {
			return e.Top != "" && e.GitIno != 0 && id.dev == e.GitDev && id.ino == e.GitIno
		}
		parent := filepath.Dir(d)
		if parent == d {
			return e.Top == ""
		}
		d = parent
	}
}

func toplevelCachePath(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return filepath.Join(hookcache.ToplevelDir, hex.EncodeToString(h[:8])+".json")
}

// resetToplevelMemo forgets the in-process memo (tests).
func resetToplevelMemo() { toplevelMemo.Clear() }

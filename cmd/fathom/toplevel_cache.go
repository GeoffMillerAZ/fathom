package main

// toplevel_cache.go — gitToplevel memoized in process and cached per cwd
// across hook processes (docs/design/responsible-tenant.md §11.3, RT-130).
// A cached answer is keyed by the cwd and holds for 10 minutes while its
// evidence stands: a toplevel answer while <top>/.git has the mtime it had
// (a re-init or a moved repository misses), a "not a repository" answer
// while no .git exists at the cwd or above it (a `git init` misses). A
// miss forks git once and writes the answer back by rename.

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
	Cwd      string    `json:"cwd"`
	Top      string    `json:"top"`
	GitMtime int64     `json:"git_mtime_ns,omitempty"` // <top>/.git's mtime; 0 for "" answers
	At       time.Time `json:"at"`
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
		st, err := os.Stat(filepath.Join(top, ".git"))
		if err != nil {
			return top // no evidence to key on: answer, cache nothing
		}
		e.GitMtime = st.ModTime().UnixNano()
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
	if e.Top != "" {
		st, err := os.Stat(filepath.Join(e.Top, ".git"))
		return err == nil && st.ModTime().UnixNano() == e.GitMtime
	}
	// A "not a repository" answer stands while no .git exists at or above
	// the cwd (a few stats, no fork).
	for d := e.Cwd; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return false
		}
		parent := filepath.Dir(d)
		if parent == d {
			return true
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

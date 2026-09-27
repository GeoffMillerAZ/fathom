package connector

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// IgnoreFileName is the per-repo ignore file read from the service root.
// It uses gitignore-style patterns (see internal/filter): one pattern per
// line, # comments, ! negation, ** globs, trailing-slash dir-only rules.
const IgnoreFileName = ".fathomignore"

// LocalConnector reads source files from a local directory tree.
// It satisfies the Connector SPI and is suitable for CLI / dev use and for
// tests that operate directly on an on-disk directory.
//
// Discover walks the directory rooted at svc.Path and returns a sorted slice
// of SourceRefs whose Paths are relative to svc.Path (ADR-017: stores must
// never record machine-local absolute anchors). skipDir controls which
// subdirectory names are skipped, and a .fathomignore file at the service
// root adds repo-specific exclusions.
// Fetch resolves a repo-relative ref against the root recorded by Discover
// for that service; an absolute ref.Path (stores written before ADR-017) is
// read as-is for back-compat.
type LocalConnector struct {
	// RespectGitignore lists every git work-tree corpus through git
	// (RT-68) whatever its own [connector] respect_gitignore says; the
	// default (false) leaves the choice to the corpus.
	RespectGitignore bool

	// roots remembers, per service ID, the root directory of the most recent
	// Discover, so Fetch can resolve the repo-relative paths Discover emits.
	roots sync.Map // service ID (string) -> root dir (string)
	// exclusions remembers, per service ID, what the most recent Discover
	// pruned (RT-145) — see Exclusions.
	exclusions sync.Map // service ID (string) -> Exclusions
	// manifests memoizes each non-git root's content manifest behind its
	// stat manifest (RT-110) — see statGatedManifest.
	manifests sync.Map // root (string) -> manifestMemo
	// probes caches each non-git root's last stat walk for FreshnessProbe
	// (RT-110) — at most one walk per probeTTL.
	probes sync.Map // root (string) -> probeMemo
}

// Exclusions reports what the most recent Discover of service excluded at
// the directory level (RT-145). The zero value when service was never
// discovered by this connector.
func (c *LocalConnector) Exclusions(service string) Exclusions {
	if v, ok := c.exclusions.Load(service); ok {
		ex := v.(Exclusions)
		ex.DefaultIgnored = append([]string(nil), ex.DefaultIgnored...)
		return ex
	}
	return Exclusions{}
}

// NewLocalConnector returns a ready-to-use LocalConnector.
func NewLocalConnector() *LocalConnector { return &LocalConnector{} }

// Discover walks the directory tree at svc.Path, assigns SourceKinds by
// extension, and returns a deterministically sorted slice of SourceRefs.
// Directories named vendor, .git, testdata, generated, node_modules,
// .idea, .vscode, and .claude are skipped entirely. If a .fathomignore file
// exists
// at svc.Path, its patterns are applied to paths relative to svc.Path
// (matched dirs are pruned from the walk; matched files are dropped).
func (c *LocalConnector) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Scoped config (docs/design/scoped-config.md deliverable 1): union the
	// project's .fathomignore with the user-scope ~/.fathom/ignore. This is
	// the enforcement point for law 13's additive-only rule — a violating
	// user-scope re-include ("!pattern") fails Discover outright (loud, per
	// the design's own requirement), never silently dropped or applied.
	// The load is service-aware: svc.ID's own scoped exclusions
	// (~/.fathom/ignore.d/<id>, purge's write-side chokepoint) union in
	// here and ONLY here — a pattern recorded for one service can never
	// reach another service's Discover (V2).
	ignore, _, err := LoadIgnoreScopedFor(svc.Path, svc.ID)
	if err != nil {
		return nil, err
	}
	// F1 (docs/bench/readings/rt-waves-c-f-2026-09-27.md §6): the root is
	// recorded only by a Discover that succeeded (at each return below). This
	// map is process-wide and keyed by service, and Stat and Fetch of an
	// in-flight ingest resolve through it: a concurrent read that
	// Discovers a root that is not there — a service still being
	// onboarded has no service_meta path, so pkg/client falls back to the
	// bare service ID — used to replace the ingest's root before its walk
	// failed, and every file of the ingest failed to stat.
	var refs []core.SourceRef

	// O39: filepath.WalkDir Lstat's its root and never follows a symlink,
	// so a root whose LAST component is a link (a checkout reached through
	// one, or a root recorded in its linked form) read as a non-directory:
	// zero files, nil error — an ingest that indexed nothing and exited 0,
	// and a `references` that found no occurrence of a symbol verify_plan
	// still resolved from the store. Walk the link's target instead; paths
	// stay relative to the root, so every SourceRef is the same one the
	// target itself yields, and Fetch still reads through svc.Path.
	walkRoot := svc.Path
	if fi, lerr := os.Lstat(svc.Path); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
		if target, rerr := filepath.EvalSymlinks(svc.Path); rerr == nil {
			walkRoot = target
		}
	}

	include, gitMode := loadConnectorConfig(svc.Path)
	// RT-68: a corpus that opts in is listed through git in a work tree.
	if gitMode || c.RespectGitignore {
		if paths, gex, ok, gerr := discoverViaGit(ctx, walkRoot, ignore, include); ok {
			if gerr != nil {
				return nil, gerr
			}
			refs = make([]core.SourceRef, 0, len(paths))
			for _, rel := range paths {
				base := path.Base(rel)
				refs = append(refs, core.SourceRef{Service: svc.ID, Path: rel, Kind: kindForFile(base, path.Ext(base))})
			}
			c.exclusions.Store(svc.ID, gex)
			c.roots.Store(svc.ID, svc.Path)
			return refs, nil
		}
	}

	entries := 0
	var ex Exclusions
	err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// RT-69: a cancelled requester stops the walk (checked every 256
		// entries, so the check never dominates a small tree).
		entries++
		if entries%256 == 0 {
			if cerr := ctx.Err(); cerr != nil {
				return cerr
			}
		}
		rel, relErr := filepath.Rel(walkRoot, path)
		if relErr != nil {
			rel = ""
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if skipDir(d.Name()) {
				ex.SkippedDirs++
				return filepath.SkipDir
			}
			if rel == "." || rel == "" {
				// The corpus root itself is never a default-ignored cache
				// (a service rooted at a directory named build is indexed).
				return nil
			}
			if defaultIgnoredDir(d.Name(), rel, include) {
				ex.DefaultIgnored = append(ex.DefaultIgnored, rel)
				return filepath.SkipDir
			}
			if ignore != nil && ignore.Match(rel+"/") {
				ex.IgnoredDirs++
				return filepath.SkipDir
			}
			return nil
		}
		if ignore != nil && rel != "" && ignore.Match(rel) {
			return nil
		}
		// Index only regular files. Symlinks, devices, sockets, and pipes are
		// skipped: WalkDir does not follow symlinks, so a symlink to a directory
		// (e.g. a Nix/devbox profile link like .devbox/nix/profile/default-9-link)
		// surfaces here as a non-dir entry — and Fetch's os.ReadFile on it fails
		// with "is a directory", which would abort the whole ingest over one stray
		// link. A regular-file gate keeps ingest robust on real-world trees.
		if !d.Type().IsRegular() {
			return nil
		}
		refs = append(refs, core.SourceRef{
			Service: svc.ID,
			Path:    rel,
			Kind:    kindForFile(d.Name(), filepath.Ext(d.Name())),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
	sort.Strings(ex.DefaultIgnored)
	c.exclusions.Store(svc.ID, ex)
	c.roots.Store(svc.ID, svc.Path)
	return refs, nil
}

// Fetch returns the raw bytes of the file identified by ref. Repo-relative
// refs (the ADR-017 convention Discover emits) are resolved against the root
// recorded for ref.Service by the preceding Discover. Absolute refs — from
// stores written before ADR-017 — are read as-is.
//
// RT-69: Fetch honours ctx and stats before it reads; a file over
// MaxFetchSize is never opened and comes back as an *OversizeError.
//
// RT-153: before each file it asks the producer governor; a fetch whose
// ctx names no class is a whole-tree walk (tree class), so at refuse it
// halts, and at the exit mark every fetch does.
func (c *LocalConnector) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
		return nil, err
	}
	if filepath.IsAbs(ref.Path) {
		return fetchCapped(ctx, ref.Path, MaxFetchSize)
	}
	root, ok := c.roots.Load(ref.Service)
	if !ok {
		return nil, fmt.Errorf("connector: no root recorded for service %q (Discover must precede Fetch for repo-relative refs)", ref.Service)
	}
	return fetchCapped(ctx, filepath.Join(root.(string), filepath.FromSlash(ref.Path)), MaxFetchSize)
}

// Watermark returns the corpus freshness token: the git HEAD SHA when
// svc.Path is inside a git work tree, else a content-addressed manifest hash
// (ADR-017: non-git corpora — docs folders, findings exports — get honest
// freshness too).
//
// RT-110 (design §6, ADR-017 amended): a git work tree whose HEAD cannot be
// read is freshness unknown — an error wrapping gitro.ErrFreshnessUnknown —
// never the content manifest (one whole-corpus read per query, on the
// request path). A work tree git confirms has no commit yet
// (gitro.ErrUnbornHead) has no HEAD to compare and is treated as the
// non-git corpus it effectively is. A genuine non-git corpus gets the content manifest gated by
// a capped stat manifest (statGatedManifest): recomputed only when a file's
// path, size or mtime moved, and at most once per manifestTTL.
func (c *LocalConnector) Watermark(ctx context.Context, svc core.ServiceRef) (string, error) {
	head, err := gitro.Head(ctx, svc.Path)
	switch {
	case err == nil:
		return head, nil
	case !errors.Is(err, gitro.ErrNotRepository) && !errors.Is(err, gitro.ErrUnbornHead):
		// An unborn branch (no commit yet) has no HEAD to compare: its
		// freshness is its content, as for a non-git corpus.
		return "", err
	}
	return c.statGatedManifest(ctx, svc)
}

// loadIgnoreFile reads root/.fathomignore and compiles it into a Matcher.
// A missing or unreadable file yields nil (no extra ignores).
func loadIgnoreFile(root string) *filter.Matcher {
	data, err := os.ReadFile(filepath.Join(root, IgnoreFileName))
	if err != nil {
		return nil
	}
	return filter.New(strings.Split(string(data), "\n"))
}

// LoadIgnore compiles root/.fathomignore UNIONED with the user-scope
// ~/.fathom/ignore (docs/design/scoped-config.md deliverable 1) into the
// same Matcher Discover applies during ingest (nil when neither
// contributes a pattern). Exported so corpus-reading consumers
// (internal/mcp's read_source) honor the same repo-specific exclusions the
// index was built with, instead of re-implementing the load and drifting.
//
// Unlike Discover (which REFUSES a violating user-scope re-include
// outright, blocking ingest until fixed — the loud enforcement point),
// this read-time convenience degrades gracefully on a violation: the
// user-scope file is dropped entirely, falling back to project-scope
// only, rather than erroring a hot serving path with no direct way to
// surface a config-fix instruction mid-read. `fathom doctor`'s
// user_scope_ignore check is the loud surface for that condition outside
// of ingest.
func LoadIgnore(root string) *filter.Matcher {
	m, _, err := LoadIgnoreScoped(root)
	if err != nil {
		return loadIgnoreFile(root) // project-scope only — safe degrade, never wider than before
	}
	return m
}

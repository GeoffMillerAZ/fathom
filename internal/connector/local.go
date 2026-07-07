package connector

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// IgnoreFileName is the per-repo ignore file read from the service root.
// It uses gitignore-style patterns (see internal/filter): one pattern per
// line, # comments, ! negation, ** globs, trailing-slash dir-only rules.
const IgnoreFileName = ".fathomignore"

// LegacyIgnoreFileName is honoured when IgnoreFileName is absent, so fleets
// that adopted the tool under its former name keep their excludes across the
// rename without touching every repo.
const LegacyIgnoreFileName = ".sextantignore"

// LocalConnector reads source files from a local directory tree.
// It satisfies the Connector SPI and is suitable for CLI / dev use and for
// tests that operate directly on an on-disk directory.
//
// Discover walks the directory rooted at svc.Path and returns a sorted slice
// of SourceRefs. skipDir controls which subdirectory names are skipped, and a
// .fathomignore file at the service root adds repo-specific exclusions.
// Fetch reads and returns the raw bytes of the file at ref.Path (which must
// be an absolute path when using LocalConnector).
type LocalConnector struct{}

// NewLocalConnector returns a ready-to-use LocalConnector.
func NewLocalConnector() *LocalConnector { return &LocalConnector{} }

// Discover walks the directory tree at svc.Path, assigns SourceKinds by
// extension, and returns a deterministically sorted slice of SourceRefs.
// Directories named vendor, .git, testdata, generated, node_modules,
// .idea, and .vscode are skipped entirely. If a .fathomignore file exists
// at svc.Path, its patterns are applied to paths relative to svc.Path
// (matched dirs are pruned from the walk; matched files are dropped).
func (c *LocalConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	ignore := loadIgnoreFile(svc.Path)

	var refs []core.SourceRef

	err := filepath.WalkDir(svc.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(svc.Path, path)
		if relErr != nil {
			rel = ""
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if ignore != nil && rel != "." && rel != "" && ignore.Match(rel+"/") {
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
			Path:    path,
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
	return refs, nil
}

// Fetch returns the raw bytes of the file identified by ref.Path.
// ref.Path must be an absolute path (as returned by Discover).
func (c *LocalConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	return os.ReadFile(ref.Path)
}

// loadIgnoreFile reads root/.fathomignore and compiles it into a Matcher.
// A missing or unreadable file yields nil (no extra ignores).
func loadIgnoreFile(root string) *filter.Matcher {
	data, err := os.ReadFile(filepath.Join(root, IgnoreFileName))
	if err != nil {
		data, err = os.ReadFile(filepath.Join(root, LegacyIgnoreFileName))
		if err != nil {
			return nil
		}
	}
	return filter.New(strings.Split(string(data), "\n"))
}

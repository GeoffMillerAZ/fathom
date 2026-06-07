package connector

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// LocalConnector reads source files from a local directory tree.
// It satisfies the Connector SPI and is suitable for CLI / dev use and for
// tests that operate directly on an on-disk directory.
//
// Discover walks the directory rooted at svc.Path and returns a sorted slice
// of SourceRefs. skipDir controls which subdirectory names are skipped.
// Fetch reads and returns the raw bytes of the file at ref.Path (which must
// be an absolute path when using LocalConnector).
type LocalConnector struct{}

// NewLocalConnector returns a ready-to-use LocalConnector.
func NewLocalConnector() *LocalConnector { return &LocalConnector{} }

// Discover walks the directory tree at svc.Path, assigns SourceKinds by
// extension, and returns a deterministically sorted slice of SourceRefs.
// Directories named vendor, .git, testdata, generated, node_modules,
// .idea, and .vscode are skipped entirely.
func (c *LocalConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	var refs []core.SourceRef

	err := filepath.WalkDir(svc.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
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

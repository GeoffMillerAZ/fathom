package connector

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// FauxConnector is a Connector that reads from the in-repo
// testdata/fauxtown/ fixture directory. It provides deterministic,
// network-free, secret-free source data for integration tests of the
// fusion pipeline, extractor, and query layers.
//
// svc.Path is treated as a path relative to the fauxtown root — i.e. the
// service subdirectory name. Fetch paths (as returned by Discover) are
// also relative to the fauxtown root so that the FauxConnector is
// independent of the process working directory.
type FauxConnector struct {
	root string // absolute path to testdata/fauxtown/
}

// NewFauxConnector returns a FauxConnector rooted at the testdata/fauxtown/
// directory co-located with this source file.
func NewFauxConnector() *FauxConnector {
	return &FauxConnector{root: fauxRootDir()}
}

// fauxRootDir returns the absolute path to the testdata/fauxtown directory
// regardless of the working directory at runtime. It resolves relative to
// this source file using runtime.Caller so that the path remains stable
// whether the tests are run from the repo root or from the package directory.
func fauxRootDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		// Fallback: use the working directory. Only reached if the binary was
		// compiled without debug info, which should never happen in tests.
		wd, _ := os.Getwd()
		return filepath.Join(wd, "testdata", "fauxtown")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "fauxtown")
}

// Discover walks the subdirectory at fauxtown/<svc.Path>, assigns SourceKinds
// by extension, and returns a deterministically sorted slice of SourceRefs
// whose Path fields are relative to the fauxtown root.
func (f *FauxConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	svcDir := filepath.Join(f.root, svc.Path)
	var refs []core.SourceRef

	err := filepath.WalkDir(svcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		// Store path relative to the fauxtown root so that SourceRef.Path
		// matches the repo-relative convention expected by the fusion pipeline.
		rel, relErr := filepath.Rel(f.root, path)
		if relErr != nil {
			return relErr
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
	return refs, nil
}

// Fetch returns the raw bytes of the file at fauxtown/<ref.Path>.
// ref.Path must be a path relative to the fauxtown root (as returned by
// Discover).
func (f *FauxConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	return os.ReadFile(filepath.Join(f.root, ref.Path))
}

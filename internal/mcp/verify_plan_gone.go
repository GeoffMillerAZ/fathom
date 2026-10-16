package mcp

// verify_plan_gone.go — O1068/O1069: what verify_plan needs to answer a
// plan the way `fathom verify --store` does over the same index.
//
// `gone:` rows: verifyEvidence had no verify.GoneEvidence, so every gone:
// row read Unknown ("no gone evidence available") through the daemon while
// --store proved it — 46 claimed-unproven rows on fathom's own plans. Both
// paths now build internal/verify/goneev over their own sources: here the
// backend's path lister and named-file reader (treescan.go), the store's
// persisted typed-pass status, the stage's graph, and the plan tree the
// caller declared (plan_tree) or, undeclared, the indexed root.
//
// plan_path: a plan_path with no plan_markdown read nothing and answered
// total:0 with no error, for any path. It now names a file under the
// service's indexed root; anything else is an error naming that form.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// planPathForm is the accepted plan_path form, named in every refusal.
const planPathForm = "a path relative to the service's indexed root, e.g. docs/plans/<plan>.md"

// planPathRel validates plan_path's form and returns it slash-cleaned: a
// relative path that stays under the root. An absolute path, or one that
// climbs out with "..", is refused.
func planPathRel(raw string) (string, error) {
	p := filepath.FromSlash(strings.TrimSpace(raw))
	if filepath.IsAbs(p) || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("plan_path %q is absolute; want %s", raw, planPathForm)
	}
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("plan_path %q escapes the indexed root; want %s", raw, planPathForm)
	}
	return filepath.ToSlash(filepath.Clean(p)), nil
}

// readPlanUnderRoot reads rel under the service's indexed root. A service
// with no recorded root, a missing file, or a symlink resolving outside
// the root is an error.
func readPlanUnderRoot(ctx context.Context, root string, haveMeta bool, service, rel string) ([]byte, error) {
	if !haveMeta || root == "" {
		return nil, fmt.Errorf("plan_path needs the service's indexed root and %q has none recorded; pass plan_markdown instead", service)
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	resolved, err := filepath.EvalSymlinks(full)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("plan_path %q: no such file under the indexed root %s", rel, root)
	}
	if err != nil {
		return nil, fmt.Errorf("plan_path %q: %v", rel, err)
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("plan_path %q: the indexed root %s: %v", rel, root, err)
	}
	if r, rerr := filepath.Rel(rootResolved, resolved); rerr != nil || !filepath.IsLocal(r) {
		return nil, fmt.Errorf("plan_path %q resolves outside the indexed root %s; want %s", rel, root, planPathForm)
	}
	// Through the corpus reader: capped like every handler read and charged
	// to the request's ticket (RT-91).
	raw, ex, err := corpus.ReadFileCapped(ctx, resolved, 0)
	if err != nil {
		return nil, fmt.Errorf("plan_path %q: %v", rel, err)
	}
	if ex != nil {
		return nil, fmt.Errorf("plan_path %q: not read: %s", rel, ex.Reason)
	}
	return raw, nil
}

// goneCorpus is the daemon's goneev.Corpus: the backend's path lister and
// named-file reader when it has them, else a budgeted scan whose stopping
// short is an error (a partial listing must never read as the corpus).
type goneCorpus struct{ backend QueryInterface }

func (g goneCorpus) SourcePaths(ctx context.Context, service string) ([]string, error) {
	if pl, ok := g.backend.(sourcePathLister); ok {
		return pl.SourcePaths(ctx, service)
	}
	var out []string
	cov, err := g.backend.ScanSources(ctx, service, treeBudget, func(f core.SourceFile) bool {
		out = append(out, f.Path)
		return false
	})
	if err == nil && cov.LowerBound() {
		err = errors.New(partialNote(cov))
	}
	return out, err
}

func (g goneCorpus) ReadSourceFiles(ctx context.Context, service string, paths []string) ([]core.SourceFile, error) {
	if r, ok := g.backend.(sourceFileReader); ok {
		return r.ReadSourceFiles(ctx, service, paths)
	}
	// goneev names every requested file this answer leaves out.
	return sourceFiles(ctx, g.backend, service, paths), nil
}

// goneEvidence builds the shared gone: evidence once per call.
func (e *verifyEvidence) goneEvidence() *goneev.Evidence {
	if e.gone != nil {
		return e.gone
	}
	if !e.metaRead {
		e.metaRead = true
		e.repoPath, e.indexedCommit, e.metaFound, e.metaErr = e.backend.ServiceMeta(e.ctx, e.service)
	}
	plan := goneev.PlanTree{}
	if e.planTree != nil {
		plan = *e.planTree
	} else {
		plan = goneev.ReadPlanTree(e.ctx, e.repoPath)
	}
	cfg := goneev.Config{
		Service:       e.service,
		IndexedCommit: e.indexedCommit,
		MetaFound:     e.metaFound,
		MetaErr:       e.metaErr,
		CorpusRoot:    e.repoPath,
		Plan:          plan,
		Corpus:        goneCorpus{backend: e.backend},
		FindSymbol:    e.FindSymbol,
		Graph:         e.graph,
		ByKey:         goneev.KeyIndex(e.nodes),
	}
	if st, ok := storeOf(e.backend); ok {
		cfg.Store = st
	}
	e.gone = goneev.New(e.ctx, cfg)
	return e.gone
}

var (
	_ verify.GoneEvidence     = (*verifyEvidence)(nil)
	_ verify.GoneReadEvidence = (*verifyEvidence)(nil)
	_ verify.FileEvidence     = verifyEvidenceWithFiles{}
)

// IndexAtHead implements verify.GoneEvidence.
func (e *verifyEvidence) IndexAtHead() (bool, string) { return e.goneEvidence().IndexAtHead() }

// FailedTypedPackages implements verify.GoneEvidence.
func (e *verifyEvidence) FailedTypedPackages() []string {
	return e.goneEvidence().FailedTypedPackages()
}

// GoFiles implements verify.GoneEvidence.
func (e *verifyEvidence) GoFiles() []verify.GoFile { return e.goneEvidence().GoFiles() }

// ModuleFile implements verify.GoneEvidence.
func (e *verifyEvidence) ModuleFile() ([]byte, bool) { return e.goneEvidence().ModuleFile() }

// CorpusFiles implements verify.GoneEvidence.
func (e *verifyEvidence) CorpusFiles(paths []string) []verify.CorpusFile {
	return e.goneEvidence().CorpusFiles(paths)
}

// TypedCallers implements verify.GoneEvidence.
func (e *verifyEvidence) TypedCallers(symbol, pkg string) (nonTest, test []verify.GoneReferent) {
	return e.goneEvidence().TypedCallers(symbol, pkg)
}

// GoneReadFailures implements verify.GoneReadEvidence.
func (e *verifyEvidence) GoneReadFailures() []string { return e.goneEvidence().GoneReadFailures() }

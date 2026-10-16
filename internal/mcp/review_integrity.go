// Package mcp — review_integrity.go, review_diff's own wiring of the
// "integrity" conserve plane (ADR-044 §1, CV-12). Before this file existed,
// review_diff resolved in.Base (reviewDiffWithBase, CV-13) and even ran
// EVERY conserve plane for DimConservation (applyConservationDimension,
// wave 2) — the "integrity" plane's own findings were computed as part of
// that composed run, but never flattened onto review.Input.IntegrityFindings,
// so DimIntegrity (internal/analysis/review) always rendered checked:true
// with ZERO findings once a base resolved: not "needs a base ref", a
// silently clean-shaped pass that looked more honest than the missing wiring
// actually was. cmd/fathom's own review path (reviewintegrity.go) already
// did this correctly; this file is internal/mcp's mirror of it, adapted to
// this package's conserveBackend/gitShowFile seams (selfskip.go) instead of
// cmd/fathom's testable gitRunner.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// applyIntegrityDimension fills in.IntegrityFindings from the "integrity"
// conserve plane when in.Base is set AND backend carries the optional
// conserveBackend capability — a no-op otherwise (DimIntegrity's own "needs
// a base ref" gate already covers the missing-Base case; a backend with no
// Store() capability degrades the same way applyConservationDimension's own
// conserveBackend assertion already does for the conservation dimension).
// Best-effort throughout: any error reading git content degrades to leaving
// IntegrityFindings empty rather than failing the whole review_diff call —
// the SAME fail-open discipline applyConservationDimension already follows.
func applyIntegrityDimension(ctx context.Context, in *review.Input, backend QueryInterface, corpusRoot, service string) {
	if in == nil || in.Base == "" || corpusRoot == "" {
		return
	}
	st, ok := storeOf(backend)
	if !ok {
		return
	}
	var plane conserve.Plane
	for _, p := range conserve.Planes() {
		if p.Name() == "integrity" {
			plane = p
			break
		}
	}
	if plane == nil {
		return // the "integrity" plane package was not linked in (should not happen in this binary; never fatal)
	}

	base := in.Base
	cin := conserve.Input{
		Service:      service,
		Root:         corpusRoot,
		Base:         base,
		Head:         conserve.HeadWorktree,
		ChangedFiles: in.ChangedFiles,
		BaseFile: func(path string) ([]byte, bool, error) {
			b := gitShowFile(ctx, corpusRoot, base, path)
			return b, b != nil, nil
		},
		HeadFile: func(path string) ([]byte, bool, error) { return readWorktreeFileForIntegrity(ctx, corpusRoot, path) },
		Store:    st,
	}
	findings, _ := plane.Check(ctx, cin)
	for _, f := range findings {
		test := f.Key
		file, line := parseIntegrityFileLineCite(f.Base)
		if file == "" {
			file, line = parseIntegrityFileLineCite(f.Head)
		}
		if f.Kind == "hand_edit" {
			test = "" // GeneratedHandEdits names a file, not a test — mirrors review.IntegrityFinding's own doc comment
			file = f.Key
		}
		in.IntegrityFindings = append(in.IntegrityFindings, review.IntegrityFinding{
			Kind: f.Kind, Test: test, File: file, Line: line, Detail: f.Detail,
		})
	}
}

// readWorktreeFileForIntegrity reads path (repo-relative) from disk under
// root — this package's mirror of cmd/fathom/reviewintegrity.go's
// readWorktreeFile.
//
// It reads through the corpus reader (RT-91): stat-capped at the extraction
// cap and charged to the request's ticket. An oversize or binary file is an
// error naming why, never a silent "absent".
func readWorktreeFileForIntegrity(ctx context.Context, root, path string) ([]byte, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path)) // root is the service's own indexed corpus root, path is repo-relative
	b, ex, err := corpus.ReadFileCapped(ctx, full, sourceReadMaxFileBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if ex != nil {
		return nil, false, fmt.Errorf("%s: %s (%d bytes)", path, ex.Reason, ex.Size)
	}
	return b, true, nil
}

// parseIntegrityFileLineCite splits a conserve.Finding.Base/Head citation
// ("path" or "path:line", internal/conserve/plane_integrity.go's own
// citeOf) back into its file and line — this package's mirror of
// cmd/fathom/reviewintegrity.go's parseFileLineCite.
func parseIntegrityFileLineCite(cite string) (file string, line int) {
	if cite == "" {
		return "", 0
	}
	i := strings.LastIndex(cite, ":")
	if i < 0 {
		return cite, 0
	}
	if n, err := strconv.Atoi(cite[i+1:]); err == nil {
		return cite[:i], n
	}
	return cite, 0
}

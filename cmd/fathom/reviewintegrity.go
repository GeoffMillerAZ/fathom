package main

// reviewintegrity.go — CV-12's cmd/fathom wiring: when `fathom review
// --since <ref>` resolves a base (buildReviewInputFromClient already sets
// review.Input.Base, CV-13), this runs internal/conserve's "integrity"
// plane over exactly that base..head range and flattens its findings onto
// review.Input.IntegrityFindings, so DimIntegrity has real data instead of
// merely reporting checked:false. Reuses the plane's OWN manifest/
// invariant/scope-evidence/reach wiring (internal/conserve/
// plane_integrity.go) rather than re-deriving any of it here — this file
// is git access plus a citation-string round-trip, nothing more.
import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// applyIntegrityDimension fills in.IntegrityFindings from the "integrity"
// conserve plane when in.Base is set (a no-op otherwise — DimIntegrity's
// own "needs a base ref" gate already covers that case, so this never
// needs to special-case it beyond skipping the work). Best-effort: any
// error resolving git content degrades to leaving IntegrityFindings empty
// rather than failing the whole review run — the SAME fail-open discipline
// every other optional review capability in this file already follows
// (e.g. ArchRules/CoverageChecked being simply absent).
func applyIntegrityDimension(ctx context.Context, in *review.Input, gitRunner func(args ...string) ([]byte, error), corpusRoot string, st store.Store, service string) {
	if in == nil || in.Base == "" {
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
		BaseFile:     func(path string) ([]byte, bool, error) { return gitShowAtRef(gitRunner, base, path) },
		HeadFile:     func(path string) ([]byte, bool, error) { return readWorktreeFile(corpusRoot, path) },
		Store:        st,
	}
	findings, _ := plane.Check(ctx, cin)
	for _, f := range findings {
		test := f.Key
		file, line := parseFileLineCite(f.Base)
		if file == "" {
			file, line = parseFileLineCite(f.Head)
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

// gitShowAtRef reads path's content at ref via gitRunner("show",
// "<ref>:<path>") — the injected-runner mirror of
// internal/conserve/input.go's own gitShowFile, kept separate because that
// one shells out directly (exec.Command) rather than through cmd/fathom's
// testable gitRunner seam.
func gitShowAtRef(gitRunner func(args ...string) ([]byte, error), ref, path string) ([]byte, bool, error) {
	out, err := gitRunner("show", ref+":"+filepath.ToSlash(path))
	if err != nil {
		return nil, false, nil // git ran (or failed to find the path) — never a hard error for a path merely absent at ref
	}
	return out, true, nil
}

// readWorktreeFile reads path (repo-relative) from disk under root.
func readWorktreeFile(root, path string) ([]byte, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	b, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

// parseFileLineCite splits a conserve.Finding.Base/Head citation ("path" or
// "path:line", internal/conserve/plane_integrity.go's own citeOf) back
// into its file and line — the inverse of that same function, kept local
// since nothing else in this binary needs a general citation parser for
// this exact "path:line" (not core.CiteRef's "path#Lstart-Lend") shape.
func parseFileLineCite(cite string) (file string, line int) {
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

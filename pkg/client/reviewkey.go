package client

// reviewkey.go — FL-19 (docs/plans/findings-loop.md): the ONE code path
// both `fathom review-key` (cmd/fathom/reviewkey.go) and any other Go
// embedder share to compute a review key (internal/analysis/reviewkey,
// FL-17/FL-18) — resolving req into reviewkey.Inputs (reading the corpus's
// rule pack/guard.cue/layer manifests, docs/plans/**, tooling identity, and
// index watermarks) is done here exactly once, so the CLI and this Client
// can never answer differently for the same inputs.

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// ReviewKey computes vision 30 §3.2 / ADR-040 decision 6's review key for
// req.Service over req.Manifest (already resolved by the caller — see
// reviewkey.ManifestEntry's doc comment for the unreadable-path handling),
// diffed against req.Prior when given.
func (c *Client) ReviewKey(ctx context.Context, req reviewkey.Request) (reviewkey.Key, error) {
	wm, _ := c.StoreWatermark(ctx, req.Service)
	if tok := c.SuppressionToken(req.Service); wm != "" && tok != "" {
		wm += "|suppressed:" + tok
	}
	// Use the shared snapshot scope, including its suppression
	// identity and disclosure, so a key and a navigation call share data.
	sc := memo.Scope{Store: c.StoreID(), Service: req.Service, Watermark: wm}
	snap, err := memo.SnapshotForExtra(ctx, sc, func(lctx context.Context) ([]core.SymbolNode, any, error) {
		cctx, col := scope.WithCollector(lctx)
		//fathom:tenancy-allow sources the shared watermarked snapshot loader
		nodes, err := c.ListAllSymbols(cctx, req.Service)
		return nodes, col.For(req.Service), err
	})
	if err != nil {
		return reviewkey.Key{}, err
	}
	if d, ok := snap.Extra.(*scope.Disclosure); ok {
		scope.Record(ctx, req.Service, d)
	}
	graph, typed, _, err := memo.GraphForTyped(ctx, snap, memo.GraphResolved, func(ctx context.Context) ([]core.Edge, error) {
		return c.TypedEdges(ctx, req.Service)
	})
	if err != nil {
		return reviewkey.Key{}, err
	}

	corpusRoot := ""
	if path, _, found, merr := c.ServiceMeta(ctx, req.Service); merr == nil && found {
		corpusRoot = path
	}

	in := reviewkey.Inputs{
		Manifest:      req.Manifest,
		Nodes:         snap.Nodes,
		TypedEdges:    typed,
		ResolvedGraph: graph,
		Fingerprint: func(path string) (string, bool) {
			hash, found, ferr := c.SourceFingerprint(ctx, req.Service, path)
			return hash, ferr == nil && found
		},
		Tooling: map[string]string{"fathom_version": buildinfo.Version()},
		Prior:   req.Prior,
	}

	in.RulesFiles, in.RulesUnknown = reviewKeyRulesFiles(corpusRoot)
	in.PlansFiles, in.PlansUnknown = reviewKeyPlansFiles(corpusRoot)

	if wm, werr := c.ServiceWatermark(ctx, req.Service); werr == nil {
		in.ServiceWatermark = wm
	}
	if sw, serr := c.StoreWatermark(ctx, req.Service); serr == nil {
		in.StoreWatermark = sw
	} else {
		in.IndexUnknown = "could not resolve store watermark: " + serr.Error()
	}
	if corpusRoot == "" && in.IndexUnknown == "" {
		in.IndexUnknown = "service has no recorded corpus root — index identity unknown"
	}

	return reviewkey.Compute(ctx, in)
}

// reviewKeyRulesFiles reads the project's rule pack
// (.fathom/rules/archconformance.cue) + .fathom/guard.cue + any
// project-local layer manifests (.fathom/layers/*/layer.cue) under
// corpusRoot, keyed by their repo-relative path. A missing file is simply
// absent (no rules configured is valid); any OTHER read error is named in
// the returned unknown list, never hashed as empty.
func reviewKeyRulesFiles(corpusRoot string) (map[string][]byte, []string) {
	files := map[string][]byte{}
	var unknown []string
	if corpusRoot == "" {
		return files, unknown
	}
	candidates := []string{
		filepath.Join(corpusRoot, projectrules.RelPath),
		filepath.Join(corpusRoot, ".fathom", "guard.cue"),
	}
	if matches, _ := filepath.Glob(filepath.Join(corpusRoot, ".fathom", "layers", "*", "layer.cue")); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	sort.Strings(candidates)
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			if !os.IsNotExist(err) {
				unknown = append(unknown, p+": "+err.Error())
			}
			continue
		}
		rel, rerr := filepath.Rel(corpusRoot, p)
		if rerr != nil {
			rel = p
		}
		files[rel] = data
	}
	return files, unknown
}

// reviewKeyPlansFiles reads every file under corpusRoot/docs/plans, keyed
// by its repo-relative path. A missing docs/plans directory is simply
// empty (valid); any other walk/read error is named in the returned
// unknown list.
func reviewKeyPlansFiles(corpusRoot string) (map[string][]byte, []string) {
	files := map[string][]byte{}
	var unknown []string
	if corpusRoot == "" {
		return files, unknown
	}
	root := filepath.Join(corpusRoot, "docs", "plans")
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			unknown = append(unknown, p+": "+err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			unknown = append(unknown, p+": "+rerr.Error())
			return nil
		}
		rel, err2 := filepath.Rel(corpusRoot, p)
		if err2 != nil {
			rel = p
		}
		files[rel] = data
		return nil
	})
	return files, unknown
}

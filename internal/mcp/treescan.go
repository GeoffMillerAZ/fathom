package mcp

// treescan.go — the request path's one tree read (RT-71, design §4.5–§4.6).
// mcp.QueryInterface has no whole-tree Sources: a handler that must look
// at file contents streams them through ScanSources under a budget, and a
// scan that stopped early says so in the answer (I38) rather than
// returning a silently shorter corpus.

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// treeBudget bounds one handler's tree scan: the design's reference-scan
// defaults (§4.5, [references] scan_bytes = 64 MiB, scan_wall = 5 s).
var treeBudget = corpus.Budget{Bytes: 64 << 20, Wall: 5 * time.Second}

// collectSources gathers service's files under treeBudget for a handler
// whose analysis needs them together (spec refs, dependency manifests,
// interface declarations). Peak memory is the budget, not the tree; a
// partial scan comes back with its coverage, which the caller discloses
// with partialNote.
func collectSources(ctx context.Context, backend QueryInterface, service string) ([]core.SourceFile, corpus.Coverage, error) {
	var files []core.SourceFile
	cov, err := backend.ScanSources(ctx, service, treeBudget, func(f core.SourceFile) bool {
		files = append(files, f)
		return false
	})
	return files, cov, err
}

// partialNote is the named-partial line for a scan that stopped early
// ("" for a complete one): "partial: scanned 812/2236 files: budget bytes".
func partialNote(cov corpus.Coverage) string {
	if !cov.LowerBound() {
		return ""
	}
	return "partial: " + cov.String()
}

// sourcePathLister is the OPTIONAL backend capability listing a service's
// file paths without reading any (client.SourcePaths): a question answered
// by paths alone never reads contents.
type sourcePathLister interface {
	SourcePaths(ctx context.Context, service string) ([]string, error)
}

// withPartial sets m["partial"] to the scan's named-partial line when it
// stopped early, and returns m.
func withPartial(m map[string]any, cov corpus.Coverage) map[string]any {
	if note := partialNote(cov); note != "" {
		m["partial"] = note
	}
	return m
}

// sourcePaths lists service's paths: from the backend's path lister when it
// has one (no content read), else from a budgeted scan.
func sourcePaths(ctx context.Context, backend QueryInterface, service string) ([]string, error) {
	if pl, ok := backend.(sourcePathLister); ok {
		return pl.SourcePaths(ctx, service)
	}
	var out []string
	_, err := backend.ScanSources(ctx, service, treeBudget, func(f core.SourceFile) bool {
		out = append(out, f.Path)
		return false
	})
	return out, err
}

// sourceFileReader is the OPTIONAL backend capability reading named files
// of a service (client.ReadSourceFiles): a handler that needs one file's
// text reads that file, not the tree.
type sourceFileReader interface {
	ReadSourceFiles(ctx context.Context, service string, paths []string) ([]core.SourceFile, error)
}

// sourceFiles reads paths of service: through the backend's file reader
// when it has one, else by a budgeted scan keeping only those paths.
func sourceFiles(ctx context.Context, backend QueryInterface, service string, paths []string) []core.SourceFile {
	if len(paths) == 0 {
		return nil
	}
	if r, ok := backend.(sourceFileReader); ok {
		files, err := r.ReadSourceFiles(ctx, service, paths)
		if err != nil {
			return nil
		}
		return files
	}
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	var out []core.SourceFile
	_, _ = backend.ScanSources(ctx, service, treeBudget, func(f core.SourceFile) bool {
		if want[f.Path] {
			out = append(out, f)
			delete(want, f.Path)
		}
		return len(want) == 0
	})
	return out
}

// interfaceSourceFiles reads the files declaring the Go interfaces query
// could name a method of (the interface named before the last dot, or
// every interface for a bare name) — interfaceMethodQuery's only input
// beyond the index.
func interfaceSourceFiles(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, query string) []core.SourceFile {
	ifaceName := ""
	if i := strings.LastIndexByte(query, '.'); i >= 0 {
		ifaceName = query[:i]
	}
	seen := map[string]bool{}
	var paths []string
	for _, n := range nodes {
		if n.Kind != core.KindGoInterface || n.FilePath == "" || (ifaceName != "" && n.Name != ifaceName) {
			continue
		}
		if !seen[n.FilePath] {
			seen[n.FilePath] = true
			paths = append(paths, n.FilePath)
		}
	}
	sort.Strings(paths)
	return sourceFiles(ctx, backend, service, paths)
}

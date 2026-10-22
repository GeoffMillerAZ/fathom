package mcp

// treescan.go — the request path's one tree read (RT-71, design §4.5–§4.6).
// mcp.QueryInterface has no whole-tree Sources: a handler that must look
// at file contents streams them through ScanSources under a budget, and a
// scan that stopped early says so in the answer (I38) rather than
// returning a silently shorter corpus.

import (
	"bytes"
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

// readReviewCorpus lists paths without content, then reads only changed
// members. The production backend supports both capabilities. Legacy test
// backends fall back to the budgeted scan, retaining its partial receipt.
func readReviewCorpus(ctx context.Context, backend QueryInterface, service string, changed []string, now func() time.Time) reviewCorpus {
	rc := reviewCorpus{
		files:    map[string][]byte{},
		fileMeta: map[string]core.SourceFile{},
		paths:    map[string]bool{},
		deadline: now().Add(treeBudget.Wall),
		now:      now,
	}
	if err := ctx.Err(); err != nil {
		rc.refuseWindow("partial: source scan canceled: " + err.Error())
		return rc
	}
	_, pathsOK := backend.(sourcePathLister)
	_, filesOK := backend.(sourceFileReader)
	rc.legacy = !pathsOK || !filesOK
	if pl, ok := backend.(sourcePathLister); ok && filesOK {
		readCtx, cancel := context.WithTimeout(ctx, rc.deadline.Sub(rc.now()))
		paths, err := pl.SourcePaths(readCtx, service)
		cancel()
		if err == nil {
			rc.pathsComplete = true
			for _, p := range paths {
				rc.paths[p] = true
			}
		} else {
			rc.partial = "partial: source path inventory: " + err.Error()
		}
	} else {
		readCtx, cancel := context.WithTimeout(ctx, rc.deadline.Sub(rc.now()))
		files, cov, err := collectSources(readCtx, backend, service)
		cancel()
		for _, f := range files {
			rc.paths[f.Path] = true
			if rc.legacy || (int64(len(f.Content)) <= 2<<20 && rc.used+int64(len(f.Content)) <= treeBudget.Bytes) {
				rc.files[f.Path] = f.Content
				rc.fileMeta[f.Path] = f
				rc.used += int64(len(f.Content))
			}
		}
		rc.pathsComplete = err == nil && !cov.LowerBound()
		if err != nil {
			rc.partial = "partial: source path inventory: " + err.Error()
		} else {
			rc.partial = partialNote(cov)
		}
	}
	seen := map[string]bool{}
	for _, p := range changed {
		if err := ctx.Err(); err != nil {
			rc.refuseWindow("partial: source scan canceled: " + err.Error())
			break
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		if rc.pathsComplete && !rc.paths[p] {
			continue // a deleted changed file has no source text
		}
		rc.readRequired(ctx, backend, service, p)
	}
	sort.Strings(rc.gaps)
	return rc
}

// readRequired accounts for one named file under the review's shared time,
// per-file and aggregate caps. A missing result for a known member is a
// coverage gap; it cannot be interpreted as an empty file.
func (rc *reviewCorpus) readRequired(ctx context.Context, backend QueryInterface, service, path string) {
	if err := ctx.Err(); err != nil {
		rc.refuseWindow("partial: required source canceled: " + err.Error())
		return
	}
	// Preserve scan-based backends' best-effort citation behavior. Only a
	// partial scan needs the same named fallback base used; an absent
	// citation in a complete legacy scan never vetoed finding persistence.
	if rc.legacy {
		if _, ok := rc.files[path]; !ok && !rc.pathsComplete {
			for _, f := range sourceFiles(ctx, backend, service, []string{path}) {
				if f.Path == path {
					rc.files[path], rc.fileMeta[path] = f.Content, f
				}
			}
		}
		return
	}
	if _, ok := rc.files[path]; ok {
		return
	}
	if rc.pathsComplete && !rc.paths[path] {
		rc.gaps = append(rc.gaps, path+" (absent citation)")
		return
	}
	if rc.used >= treeBudget.Bytes || !rc.now().Before(rc.deadline) {
		rc.gaps = append(rc.gaps, path+" (review budget)")
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, rc.deadline.Sub(rc.now()))
	defer cancel()
	var files []core.SourceFile
	var err error
	if reader, ok := backend.(sourceFileReader); ok {
		files, err = reader.ReadSourceFiles(readCtx, service, []string{path})
	} else {
		files = sourceFiles(readCtx, backend, service, []string{path})
	}
	if ctx.Err() != nil || readCtx.Err() != nil || !rc.now().Before(rc.deadline) {
		rc.refuseWindow("partial: source window review budget expired or canceled")
		rc.gaps = append(rc.gaps, path+" (review budget)")
		return
	}
	if err == nil {
		for _, f := range files {
			if f.Path != path {
				continue
			}
			if int64(len(f.Content)) > 2<<20 || rc.used+int64(len(f.Content)) > treeBudget.Bytes {
				rc.gaps = append(rc.gaps, path+" (review budget)")
				return
			}
			rc.files[path] = f.Content
			rc.fileMeta[path] = f
			rc.used += int64(len(f.Content))
			return
		}
	}
	rc.gaps = append(rc.gaps, path+" (unreadable)")
}

// revalidate compares the full path set and every byte/metadata field used
// for this review immediately before its first finding write. It shares the
// original 64 MiB / 5 s allowance. This closes the analysis-to-write
// window without treating a live file read as proof of index freshness.
func (rc *reviewCorpus) revalidate(ctx context.Context, backend QueryInterface, service string) bool {
	if err := ctx.Err(); err != nil {
		return rc.refuseWindow("partial: source window canceled: " + err.Error())
	}
	pl, pathsOK := backend.(sourcePathLister)
	reader, filesOK := backend.(sourceFileReader)
	if !pathsOK || !filesOK {
		return true
	} // legacy backends retain their scan-based persistence
	if !rc.now().Before(rc.deadline) {
		return rc.refuseWindow("partial: source window review budget expired")
	}
	readCtx, cancel := context.WithTimeout(ctx, rc.deadline.Sub(rc.now()))
	defer cancel()
	paths, err := pl.SourcePaths(readCtx, service)
	if err != nil {
		return rc.refuseWindow("partial: source path inventory recheck: " + err.Error())
	}
	current := make(map[string]bool, len(paths))
	for _, p := range paths {
		current[p] = true
	}
	if len(current) != len(rc.paths) {
		return rc.refuseWindow("partial: source path membership changed during review")
	}
	for p := range rc.paths {
		if !current[p] {
			return rc.refuseWindow("partial: source path membership changed during review")
		}
	}
	named := make([]string, 0, len(rc.files))
	for p := range rc.files {
		named = append(named, p)
	}
	sort.Strings(named)
	for _, p := range named {
		if rc.used >= treeBudget.Bytes || readCtx.Err() != nil || !rc.now().Before(rc.deadline) {
			return rc.refuseWindow("partial: source window review budget expired")
		}
		fresh, err := reader.ReadSourceFiles(readCtx, service, []string{p})
		if err != nil || len(fresh) != 1 || fresh[0].Path != p {
			return rc.refuseWindow("partial: required source unreadable during review: " + p)
		}
		f := fresh[0]
		if int64(len(f.Content)) > 2<<20 || rc.used+int64(len(f.Content)) > treeBudget.Bytes {
			return rc.refuseWindow("partial: source window review budget exceeded: " + p)
		}
		rc.used += int64(len(f.Content))
		was := rc.fileMeta[p]
		if !bytes.Equal(f.Content, was.Content) || f.Lang != was.Lang || f.Generated != was.Generated {
			return rc.refuseWindow("partial: required source changed during review: " + p)
		}
	}
	if readCtx.Err() != nil || !rc.now().Before(rc.deadline) {
		return rc.refuseWindow("partial: source window review budget expired")
	}
	return true
}

func (rc *reviewCorpus) refuseWindow(note string) bool {
	rc.windowUnverified = true
	rc.partial = note
	return false
}

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
	return sourceFiles(ctx, backend, service, interfaceSourcePaths(nodes, query))
}

// interfaceSourceFilesWithin is interfaceSourceFiles under a byte
// allowance (a references call's one budget, §4.5): files are read one at
// a time and the reading stops once allowance bytes are spent. complete is
// false when a candidate file was left unread — an interface match over a
// subset could name a unique interface that is not one, so the caller
// then skips the check and names the stop.
func interfaceSourceFilesWithin(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, query string, allowance int64) (files []core.SourceFile, used int64, complete bool) {
	paths := interfaceSourcePaths(nodes, query)
	for i, p := range paths {
		if used >= allowance {
			return files, used, i == len(paths)
		}
		for _, f := range sourceFiles(ctx, backend, service, []string{p}) {
			used += int64(len(f.Content))
			files = append(files, f)
		}
	}
	return files, used, used <= allowance
}

// interfaceSourcePaths are the sorted files declaring the Go interfaces
// query could name a method of.
func interfaceSourcePaths(nodes []core.SymbolNode, query string) []string {
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
	return paths
}

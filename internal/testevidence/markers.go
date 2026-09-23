package testevidence

// markers.go — IngestCertifyMarkers (TC-5): walks a service's Go source
// tree for `// certifies:` markers (internal/extractor.ScanCertifiesMarkers)
// and writes one declared-certification fact per entry into the additive
// certify_declarations table, wholesale-replacing the service's prior set
// (a fresh scan IS the current truth, same contract as PutTestSuites). A
// malformed marker is collected as a warning naming file:line, never a
// silent skip (design doc §1) — MarkerReport.Warnings carries every one
// found, across every file.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// MarkerReport is IngestCertifyMarkers' return: what it found.
type MarkerReport struct {
	FilesScanned int
	Declared     int
	// Warnings is one line per malformed marker, "<file>:<line>: <err>" —
	// surfaced verbatim in the ingest report (design doc §1).
	Warnings []string
}

// IngestCertifyMarkers walks root (a service's repo root) for *.go files
// — the same skip-dir convention connector.Discover uses (vendor, .git,
// testdata, generated, node_modules, .idea, .vscode, .claude —
// connector.IsSkippedDir) so a marker inside a vendored or generated tree
// is never scanned — parses each for `// certifies:` markers, and writes
// the declared facts for service, stamped with watermark.
func IngestCertifyMarkers(ctx context.Context, st store.Store, service, watermark, root string) (MarkerReport, error) {
	var report MarkerReport

	files, err := discoverGoFiles(root)
	if err != nil {
		return report, fmt.Errorf("discover Go files under %s: %w", root, err)
	}

	var decls []store.CertifyDeclaration
	for _, relPath := range files {
		abs := filepath.Join(root, relPath)
		src, err := os.ReadFile(abs)
		if err != nil {
			return report, fmt.Errorf("read %s: %w", abs, err)
		}
		found, warnings, err := extractor.ScanCertifiesMarkers(service, relPath, src)
		if err != nil {
			// A file that fails to PARSE (invalid Go) is not this scan's
			// job to diagnose — the ordinary Go extractor already reports
			// it during ingest. Skip it here rather than failing the
			// whole marker scan over one broken file.
			continue
		}
		report.FilesScanned++
		for _, d := range found {
			decls = append(decls, store.CertifyDeclaration{
				TestSymbolKey: d.TestSymbolKey, Kind: string(d.Kind), TargetID: d.TargetID,
				Strength: string(d.Strength), File: d.File, Line: d.Line, Watermark: watermark,
			})
		}
		for _, w := range warnings {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s:%d: %s", w.File, w.Line, w.Err))
		}
	}
	sort.Strings(report.Warnings)

	if err := st.PutCertifyDeclarations(ctx, service, decls); err != nil {
		return report, fmt.Errorf("PutCertifyDeclarations: %w", err)
	}
	report.Declared = len(decls)
	return report, nil
}

// discoverGoFiles returns every *.go file under root (repo-relative,
// forward-slash paths, sorted), pruning the same directories
// connector.Discover always skips.
func discoverGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && connector.IsSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".go") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

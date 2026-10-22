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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// MarkerReport is IngestCertifyMarkers' return: what it found.
type MarkerReport struct {
	FilesScanned    int
	FilesParsed     int
	OversizeSkipped int
	Declared        int
	// Warnings is one line per malformed marker, "<file>:<line>: <err>" —
	// surfaced verbatim in the ingest report (design doc §1).
	Warnings []string
}

// IngestCertifyMarkers discovers the same corpus as local ingest, including
// default, project, user and git exclusions. Only Go files containing the
// marker bytes are parsed. Cancellation or producer refusal preserves the
// previous declaration set rather than replacing it with a partial scan.
func IngestCertifyMarkers(ctx context.Context, st store.Store, service, watermark, root string) (MarkerReport, error) {
	var report MarkerReport

	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassIngest); err != nil {
		return report, err
	}
	conn := connector.NewLocalConnector()
	files, err := conn.Discover(ctx, core.ServiceRef{ID: service, Path: root})
	if err != nil {
		return report, fmt.Errorf("discover Go files under %s: %w", root, err)
	}

	var decls []store.CertifyDeclaration
	for _, ref := range files {
		if !strings.HasSuffix(ref.Path, ".go") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		// Fetch checks the producer before every file and honors ctx.
		src, err := conn.Fetch(ctx, ref)
		if err != nil {
			var over *connector.OversizeError
			if errors.As(err, &over) {
				report.OversizeSkipped++
				continue
			}
			return report, fmt.Errorf("read %s: %w", ref.Path, err)
		}
		report.FilesScanned++
		if !bytes.Contains(src, []byte("certifies:")) {
			continue
		}
		report.FilesParsed++
		found, warnings, err := extractor.ScanCertifiesMarkers(service, ref.Path, src)
		if err != nil {
			// A file that fails to PARSE (invalid Go) is not this scan's
			// job to diagnose — the ordinary Go extractor already reports
			// it during ingest. Skip it here rather than failing the
			// whole marker scan over one broken file.
			continue
		}
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

	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassIngest); err != nil {
		return report, err
	}
	if err := st.PutCertifyDeclarations(ctx, service, decls); err != nil {
		return report, fmt.Errorf("PutCertifyDeclarations: %w", err)
	}
	report.Declared = len(decls)
	return report, nil
}

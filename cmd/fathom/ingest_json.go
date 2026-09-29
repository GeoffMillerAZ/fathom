package main

// ingest_json.go — `fathom ingest --json` (RP-14, docs/plans/reach-parity.md):
// the counts, the read-failed list with its errno class, and the
// degraded/refused outcome as DATA, so CI can assert `read_failed == 0`
// instead of grepping a summary. The envelope is documented in
// docs/usage.md §ingest.

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type ingestReadFailureJSON struct {
	Path  string `json:"path"`
	Err   string `json:"err"`
	Errno string `json:"errno,omitempty"`
}

type ingestTypedEdgesJSON struct {
	// PackagesSeen/PackagesProcessed/Edges/Failed/Error are the ingest
	// honesty disclosure (wave P3 / O82, .fathom/observations.md): a Go
	// tree whose typed-edge pass produced nothing to trust must say so on
	// the ingest response itself, not only via a separate staleness_report
	// call. PackagesSeen is every package go/packages returned, before any
	// were dropped; PackagesProcessed is loaded+cache-hit (kept as
	// `packages` too, for compatibility with the pre-O82 field name).
	PackagesSeen      int    `json:"packages_seen"`
	PackagesProcessed int    `json:"packages_processed"`
	Edges             int    `json:"edges"`
	Failed            bool   `json:"failed"`
	Error             string `json:"error,omitempty"`
	// Fallback names "GOWORK=off" when an ambient Go workspace was
	// discoverable and therefore overridden (defect 2, same wave).
	Fallback string `json:"fallback,omitempty"`

	// Pre-O82 fields, kept for compatibility.
	Packages       int `json:"packages"`
	PackagesFailed int `json:"packages_failed"`
	Direct         int `json:"direct"`
	IfaceDispatch  int `json:"iface_dispatch"`
	Unresolved     int `json:"unresolved"`
}

// ingestCUEEdgesJSON is the CUE typed-edge pass counters (ADR-041 decision
// 5 / SU-10) — the same package-granular shape as ingestTypedEdgesJSON's
// pre-O82 fields, over the CUE producer's own store-key namespace.
type ingestCUEEdgesJSON struct {
	Packages       int `json:"packages"`
	PackagesFailed int `json:"packages_failed"`
	Edges          int `json:"edges"`
	Direct         int `json:"direct"`
	Unresolved     int `json:"unresolved"`
}

// ingestDocsJSON is the knowledge-retrieval doc-unit pass (KR-8/KR-9):
// counts, what the per-service byte budget cut (named), what could not be
// indexed (named) and, when the pass did not run or did not finish, why.
type ingestDocsJSON struct {
	Files        int                  `json:"files"`
	Changed      int                  `json:"changed"`
	Unchanged    int                  `json:"unchanged"`
	Deleted      int                  `json:"deleted"`
	UnitsWritten int                  `json:"units_written"`
	UnitsElided  int                  `json:"units_elided,omitempty"`
	ElidedBytes  int                  `json:"elided_bytes,omitempty"`
	Elided       []ingestDocElidedRow `json:"elided,omitempty"`
	Failures     []ingestDocFailure   `json:"failures,omitempty"`
	Skipped      string               `json:"skipped,omitempty"`
}

type ingestDocElidedRow struct {
	Path    string `json:"path"`
	Heading string `json:"heading"`
	Bytes   int    `json:"bytes"`
}

type ingestDocFailure struct {
	Path string `json:"path"`
	Err  string `json:"err"`
}

type ingestExcludedJSON struct {
	Project int `json:"project"`
	User    int `json:"user"`
	Service int `json:"service"`
}

// ingestScipJSON is one --scip (or auto-detected) index's import outcome
// (ADR-041 decision 5, SU-9) — Producer names the external indexer so a
// reader can tell an imported typed pass from the native Go one
// (TypedEdges above) without a separate lookup.
type ingestScipJSON struct {
	Path       string `json:"path"`
	Producer   string `json:"producer"`
	Language   string `json:"language,omitempty"`
	Documents  int    `json:"documents"`
	Edges      int    `json:"edges"`
	Direct     int    `json:"direct"`
	Unresolved int    `json:"unresolved"`
	// Unmapped discloses O115 (.fathom/observations.md): the count of SCIP
	// definitions this import could not join to the service's own
	// extracted symbol table — a real producer (scip-typescript,
	// scip-python) never sets SymbolInformation.DisplayName, so ImportIndex
	// resolves by definition position instead; a definition it still
	// cannot place contributes NO edge, in either direction, rather than a
	// silently wrong one. Zero on a healthy import; omitted from text
	// output when zero, always present in JSON so CI can assert on it.
	Unmapped int `json:"unmapped"`
}

// ingestJSON is the `fathom ingest --json` envelope.
type ingestJSON struct {
	Service string `json:"service"`
	// Outcome is ok | degraded | refused | incomplete. degraded: every
	// readable file was indexed and read_failed > 0; refused: nothing was
	// written (O16 guard); incomplete: every file failed with no index to
	// protect — nothing was written and the ingest was not recorded
	// complete (F2).
	Outcome    string `json:"outcome"`
	ExitCode   int    `json:"exit_code"`
	Discovered int    `json:"discovered"`
	Extracted  int    `json:"extracted"`
	Skipped    int    `json:"skipped"`
	// StatSkipped (RT-102): of skipped, the files never read — stat and
	// fingerprint unchanged. PeakInflightBytes (RT-103): the most file
	// content the ingest held at once.
	StatSkipped       int                     `json:"stat_skipped,omitempty"`
	PeakInflightBytes int64                   `json:"peak_inflight_bytes,omitempty"`
	Symbols           int                     `json:"symbols"`
	ReadFailed        int                     `json:"read_failed"`
	ReadFailures      []ingestReadFailureJSON `json:"read_failures"`
	Unclassified      int                     `json:"unclassified"`
	Oversize          int                     `json:"oversize"`
	ClassifiedZero    int                     `json:"classified_zero"`
	// SkippedGenerated, ClassifiedBySniff and AmbiguousUnclassified are the
	// dialect gate's counts (LB-F9, LB-F10); omitted when zero.
	SkippedGenerated      int `json:"skipped_generated,omitempty"`
	ClassifiedBySniff     int `json:"classified_by_sniff,omitempty"`
	AmbiguousUnclassified int `json:"ambiguous_unclassified,omitempty"`
	// UnsupportedLanguages names, by language display name (e.g. "kotlin"),
	// files whose extension belongs to a known programming language fathom
	// has no registered extractor for (O69). Omitted when empty.
	UnsupportedLanguages map[string]int        `json:"unsupported_languages,omitempty"`
	TypedEdges           *ingestTypedEdgesJSON `json:"typed_edges,omitempty"`
	ScipImports          []ingestScipJSON      `json:"scip_imports,omitempty"`
	CUEEdges             *ingestCUEEdgesJSON   `json:"cue_edges,omitempty"`
	// FileResolved is the per-file TypeScript/JavaScript/Python resolver's
	// edge count (ADR-041 decision 5 / SU-11) — always heuristic tier.
	FileResolved int `json:"file_resolved,omitempty"`
	// Docs is the doc-unit pass (KR-8/KR-9); present whenever the pass ran
	// over at least one document or was skipped (its reason in docs.skipped).
	Docs     *ingestDocsJSON     `json:"docs,omitempty"`
	Excluded *ingestExcludedJSON `json:"excluded,omitempty"`
	// DefaultIgnoredDirs names each directory the default cache list pruned
	// (RT-145); omitted when none.
	DefaultIgnoredDirs []string `json:"default_ignored_dirs,omitempty"`
	Error              string   `json:"error,omitempty"`
	Hint               string   `json:"hint,omitempty"`
}

// errnoClass maps the connector's errno text to its symbolic class; ""
// when the text names no known class (the text itself is always carried).
func errnoClass(errText string) string {
	t := strings.ToLower(errText)
	switch {
	case strings.Contains(t, "operation not permitted"):
		return "EPERM"
	case strings.Contains(t, "permission denied"):
		return "EACCES"
	case strings.Contains(t, "no such file"):
		return "ENOENT"
	case strings.Contains(t, "input/output error"):
		return "EIO"
	case strings.Contains(t, "is a directory"):
		return "EISDIR"
	}
	return ""
}

// renderIngestJSON writes the envelope and returns the exit code (the same
// 0/1 the text path returns). scipResults carries every --scip (or
// auto-detected) index this ingest imported (ADR-041 decision 5, SU-9);
// nil when none ran.
func renderIngestJSON(w io.Writer, service, svcPath string, report client.Report, readErr *client.ReadFailuresError, scipResults []scipImportResult) int {
	out := ingestJSON{
		Service:           service,
		Outcome:           "ok",
		Discovered:        report.Discovered,
		Extracted:         report.Extracted,
		Skipped:           report.Skipped,
		StatSkipped:       report.StatSkipped,
		PeakInflightBytes: report.PeakInflightBytes,
		Symbols:           report.Symbols,
		ReadFailed:        report.ReadFailed,
		ReadFailures:      []ingestReadFailureJSON{},
		Unclassified:      report.Unclassified,
		Oversize:          report.Oversize,
		ClassifiedZero:    report.ClassifiedZero,

		SkippedGenerated:      report.SkippedGenerated,
		ClassifiedBySniff:     report.ClassifiedBySniff,
		AmbiguousUnclassified: report.AmbiguousUnclassified,
	}
	if len(report.UnsupportedLanguages) > 0 {
		out.UnsupportedLanguages = report.UnsupportedLanguages
	}
	out.DefaultIgnoredDirs = report.DefaultIgnoredDirs
	for _, f := range report.ReadFailures {
		out.ReadFailures = append(out.ReadFailures, ingestReadFailureJSON{Path: f.Path, Err: f.Err, Errno: errnoClass(f.Err)})
	}
	// Ingest honesty disclosure (wave P3 / O82): present whenever a typed
	// pass was actually ATTEMPTED this ingest — PackagesSeen>0 or a
	// disclosed failure with nothing seen yet (e.g. a whole-load error) —
	// never the pre-O82 "non-zero packages/failures only" condition, which
	// is exactly what let a Go tree with ZERO typed edges (packages seen,
	// none processed) report a silently empty block.
	if report.TypedPackagesSeen > 0 || report.TypedPackages > 0 || report.TypedPackagesFailed > 0 || report.TypedFailed {
		edges := report.TypedEdgesDirect + report.TypedEdgesIfaceDispatch + report.TypedEdgesUnresolved
		out.TypedEdges = &ingestTypedEdgesJSON{
			PackagesSeen: report.TypedPackagesSeen, PackagesProcessed: report.TypedPackages,
			Edges: edges, Failed: report.TypedFailed, Error: report.TypedFailReason, Fallback: report.TypedFallback,
			Packages: report.TypedPackages, PackagesFailed: report.TypedPackagesFailed,
			Direct: report.TypedEdgesDirect, IfaceDispatch: report.TypedEdgesIfaceDispatch, Unresolved: report.TypedEdgesUnresolved,
		}
	}
	for _, r := range scipResults {
		out.ScipImports = append(out.ScipImports, ingestScipJSON{
			Path: r.Path, Producer: r.Import.Report.Producer.String(), Language: r.Import.Report.Language,
			Documents: r.Import.Report.Documents, Edges: r.Import.Report.Edges,
			Direct: r.Import.Report.Direct, Unresolved: r.Import.Report.Unresolved,
			Unmapped: r.Import.Report.Unmapped,
		})
	}
	if report.CUEPackages > 0 || report.CUEPackagesFailed > 0 {
		out.CUEEdges = &ingestCUEEdgesJSON{
			Packages: report.CUEPackages, PackagesFailed: report.CUEPackagesFailed,
			Edges:  report.CUEEdgesDirect + report.CUEEdgesUnresolved,
			Direct: report.CUEEdgesDirect, Unresolved: report.CUEEdgesUnresolved,
		}
	}
	out.FileResolved = report.FileResolvedEdges
	if d := report.DocIndex; d.FilesScanned > 0 || d.FilesDeleted > 0 || report.DocIndexSkipped != "" {
		docs := &ingestDocsJSON{
			Files: d.FilesScanned, Changed: d.FilesChanged, Unchanged: d.FilesUnchanged, Deleted: d.FilesDeleted,
			UnitsWritten: d.UnitsWritten, UnitsElided: d.UnitsElided, ElidedBytes: d.ElidedBytes,
			Skipped: report.DocIndexSkipped,
		}
		for _, e := range d.Elided {
			docs.Elided = append(docs.Elided, ingestDocElidedRow{Path: e.Path, Heading: e.HeadingPath, Bytes: e.Bytes})
		}
		for _, f := range d.Failures {
			docs.Failures = append(docs.Failures, ingestDocFailure{Path: f.Path, Err: f.Err})
		}
		out.Docs = docs
	}

	if _, counts, err := connector.LoadIgnoreScopedFor(svcPath, service); err == nil && (counts.Project > 0 || counts.User > 0 || counts.Service > 0) {
		out.Excluded = &ingestExcludedJSON{Project: counts.Project, User: counts.User, Service: counts.Service}
	}
	if readErr != nil {
		out.Outcome = "degraded"
		switch {
		case readErr.Refused:
			out.Outcome = "refused"
		case readErr.Incomplete:
			out.Outcome = "incomplete" // F2: nothing indexed, not recorded complete
		}
		out.ExitCode = 1
		out.Error = readErr.Error()
		out.Hint = readErr.Hint()
	} else if report.TypedFailed {
		// ok:false-grade disclosure (wave P3 / O82): the syntax index is
		// untouched (kept, never rolled back), but a Go tree whose typed
		// pass produced nothing to trust must never read as a clean "ok" —
		// this is the exact silent gap that let a push be refused
		// downstream (test_impact/verify test_reaches read "no test
		// reaches it") with no earlier signal anywhere in the ingest that
		// ran it.
		out.Outcome = "degraded"
		out.ExitCode = 1
		out.Error = "typed-edge pass failed: " + report.TypedFailReason
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	return out.ExitCode
}

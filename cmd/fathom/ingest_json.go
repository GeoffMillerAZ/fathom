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

type ingestExcludedJSON struct {
	Project int `json:"project"`
	User    int `json:"user"`
	Service int `json:"service"`
}

// ingestJSON is the `fathom ingest --json` envelope.
type ingestJSON struct {
	Service string `json:"service"`
	// Outcome is ok | degraded | refused. degraded: every readable file was
	// indexed and read_failed > 0; refused: nothing was written (O16 guard).
	Outcome        string                  `json:"outcome"`
	ExitCode       int                     `json:"exit_code"`
	Discovered     int                     `json:"discovered"`
	Extracted      int                     `json:"extracted"`
	Skipped        int                     `json:"skipped"`
	Symbols        int                     `json:"symbols"`
	ReadFailed     int                     `json:"read_failed"`
	ReadFailures   []ingestReadFailureJSON `json:"read_failures"`
	Unclassified   int                     `json:"unclassified"`
	Oversize       int                     `json:"oversize"`
	ClassifiedZero int                     `json:"classified_zero"`
	// UnsupportedLanguages names, by language display name (e.g. "kotlin"),
	// files whose extension belongs to a known programming language fathom
	// has no registered extractor for (O69). Omitted when empty.
	UnsupportedLanguages map[string]int        `json:"unsupported_languages,omitempty"`
	TypedEdges           *ingestTypedEdgesJSON `json:"typed_edges,omitempty"`
	Excluded             *ingestExcludedJSON   `json:"excluded,omitempty"`
	Error                string                `json:"error,omitempty"`
	Hint                 string                `json:"hint,omitempty"`
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
// 0/1 the text path returns).
func renderIngestJSON(w io.Writer, service, svcPath string, report client.Report, readErr *client.ReadFailuresError) int {
	out := ingestJSON{
		Service:        service,
		Outcome:        "ok",
		Discovered:     report.Discovered,
		Extracted:      report.Extracted,
		Skipped:        report.Skipped,
		Symbols:        report.Symbols,
		ReadFailed:     report.ReadFailed,
		ReadFailures:   []ingestReadFailureJSON{},
		Unclassified:   report.Unclassified,
		Oversize:       report.Oversize,
		ClassifiedZero: report.ClassifiedZero,
	}
	if len(report.UnsupportedLanguages) > 0 {
		out.UnsupportedLanguages = report.UnsupportedLanguages
	}
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
	if _, counts, err := connector.LoadIgnoreScopedFor(svcPath, service); err == nil && (counts.Project > 0 || counts.User > 0 || counts.Service > 0) {
		out.Excluded = &ingestExcludedJSON{Project: counts.Project, User: counts.User, Service: counts.Service}
	}
	if readErr != nil {
		out.Outcome = "degraded"
		if readErr.Refused {
			out.Outcome = "refused"
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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// parseModeFlag converts a user-supplied mode string to a client.Mode constant.
func parseModeFlag(s string) (client.Mode, bool) {
	switch s {
	case "local":
		return client.ModeLocal, true
	case "faux":
		return client.ModeFaux, true
	case "github":
		return client.ModeGitHub, true
	}
	return 0, false
}

// newIngestClient builds the client cmdIngest ingests through. A package
// variable so a test can route the ingest through a connector that simulates
// per-file read failures against a real store (ingest_honesty_test.go,
// IT-14 / IT-15) — never by touching a real TCC grant.
var newIngestClient = func(m client.Mode, storePath string) (*client.Client, error) {
	return client.NewConstantClient(m, storePath)
}

// readFailuresShown caps how many unreadable files the summary names inline;
// the count is always exact and the first entry always carries its errno.
const readFailuresShown = 5

func cmdIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("ingest", stderr)
	service := fs.String("service", "", "logical service ID (required)")
	path := fs.String("path", ".", "path to service source root (local mode)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	mode := fs.String("mode", "local", "connector mode: local or faux")
	jsonOut := fs.Bool("json", false, "emit the run as JSON (counts, read_failures with errno, outcome ok|degraded|refused) — docs/usage.md §ingest")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}

	m, ok := parseModeFlag(*mode)
	if !ok {
		fmt.Fprintf(stderr, "error: unknown --mode %q (want local or faux)\n", *mode)
		return 1
	}

	c, err := newIngestClient(m, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// For ModeFaux the connector resolves svc.Path as the fauxtown subdirectory
	// name, so we pass the service ID.  For ModeLocal we use --path.
	svcPath := *path
	if m == client.ModeFaux {
		svcPath = *service
	}

	report, err := c.Ingest(context.Background(), client.ServiceRef{
		ID:   *service,
		Path: svcPath,
	})
	// Ingest honesty (docs/plans/instrument-truth.md IT-14 / IT-15, O16): a
	// *client.ReadFailuresError arrives beside a COMPLETE report — every
	// readable file was processed — so the summary prints in full with its
	// failed column, then the failure is named on stderr and the exit is
	// non-zero, as a fetch error always was. A Refused one wrote nothing at
	// all (the store already held evidence and this run could read nothing).
	// Any other error is the abort it always was.
	var readErr *client.ReadFailuresError
	if err != nil && !errors.As(err, &readErr) {
		fmt.Fprintf(stderr, "error: ingest failed: %v\n", err)
		return 1
	}
	// Critic finding F2 (BLOCKING, phase-P fix round): bust the
	// service-identity cache (identcache.go) on every ingest that wrote
	// — a stale cached identity must never outlive the ingest that
	// changed the answer, regardless of TTL (the O15 class). A refused
	// ingest changed nothing.
	if readErr == nil || !readErr.Refused {
		hookcache.BustIdent()
	}
	if *jsonOut {
		return renderIngestJSON(stdout, *service, svcPath, report, readErr) // RP-14: ingest_json.go
	}

	fmt.Fprintf(stdout, "service:    %s\n", *service)
	fmt.Fprintf(stdout, "discovered: %d\n", report.Discovered)
	fmt.Fprintf(stdout, "extracted:  %d\n", report.Extracted)
	fmt.Fprintf(stdout, "skipped:    %d\n", report.Skipped)
	fmt.Fprintf(stdout, "symbols:    %d\n", report.Symbols)
	// The failed column (IT-14): every file the connector could not read,
	// with its errno text — a wedged reader must never print a clean
	// zero-symbol summary. Non-zero-only like the counters below.
	if report.ReadFailed > 0 {
		fmt.Fprintf(stdout, "read-failed: %d file(s) — stored evidence for them left untouched\n", report.ReadFailed)
		for i, f := range report.ReadFailures {
			if i == readFailuresShown {
				fmt.Fprintf(stdout, "  ... and %d more\n", report.ReadFailed-readFailuresShown)
				break
			}
			fmt.Fprintf(stdout, "  %s: %s\n", f.Path, f.Err)
		}
	}
	// Unclassified/oversize/classified-zero (ADR-021 §2 / TI-6, F5): print
	// only when non-zero so the common case stays as quiet as it always was
	// — a clean chart/repo classifies everything and these counters are 0.
	if report.Unclassified > 0 {
		fmt.Fprintf(stdout, "unclassified: %d\n", report.Unclassified)
	}
	if report.Oversize > 0 {
		fmt.Fprintf(stdout, "oversize:   %d\n", report.Oversize)
	}
	if report.ClassifiedZero > 0 {
		fmt.Fprintf(stdout, "classified-but-zero-symbols: %d\n", report.ClassifiedZero)
	}
	// Unsupported languages (O69): a known programming language with no
	// registered extractor — named and counted rather than left
	// indistinguishable from the generic unclassified/no-extractor path.
	// Sorted for deterministic output.
	if len(report.UnsupportedLanguages) > 0 {
		langs := make([]string, 0, len(report.UnsupportedLanguages))
		for lang := range report.UnsupportedLanguages {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
		fmt.Fprintf(stdout, "unsupported languages (no extractor):\n")
		for _, lang := range langs {
			fmt.Fprintf(stdout, "  %s: %d file(s)\n", lang, report.UnsupportedLanguages[lang])
		}
	}
	// Typed-edge counters (ADR-021 §3) and the ingest honesty disclosure
	// (wave P3 / O82, .fathom/observations.md): printed whenever a typed
	// pass was actually ATTEMPTED this ingest — PackagesSeen>0, or a
	// disclosed failure with nothing yet seen (e.g. a whole-load error) —
	// never the pre-O82 "non-zero packages/failures only" condition, which
	// is exactly what let a Go tree with ZERO typed edges (packages seen,
	// none processed) print nothing at all.
	if report.TypedPackagesSeen > 0 || report.TypedPackages > 0 || report.TypedPackagesFailed > 0 || report.TypedFailed {
		edges := report.TypedEdgesDirect + report.TypedEdgesIfaceDispatch + report.TypedEdgesUnresolved
		fmt.Fprintf(stdout, "typed-edges: %d/%d pkg(s) processed (%d failed), %d edge(s) (%d direct / %d iface-dispatch / %d unresolved)\n",
			report.TypedPackages, report.TypedPackagesSeen, report.TypedPackagesFailed,
			edges, report.TypedEdgesDirect, report.TypedEdgesIfaceDispatch, report.TypedEdgesUnresolved)
		if report.TypedFallback != "" {
			fmt.Fprintf(stdout, "typed-edges: an ambient Go workspace was overridden (%s)\n", report.TypedFallback)
		}
		if report.TypedFailed {
			fmt.Fprintf(stderr, "warning: typed-edge pass failed: %s — test_impact/verify test_reaches read a syntax-only graph for this service until it is fixed\n", report.TypedFailReason)
		}
	}
	// CUE typed-edge counters (ADR-041 decision 5 / SU-10): printed only
	// when the CUE pass actually ran (packages processed or failed) — a
	// non-CUE service stays as quiet as it always was.
	if report.CUEPackages > 0 || report.CUEPackagesFailed > 0 {
		cueEdges := report.CUEEdgesDirect + report.CUEEdgesUnresolved
		fmt.Fprintf(stdout, "cue: %d pkg(s) processed (%d failed), %d edge(s) (%d direct / %d unresolved)\n",
			report.CUEPackages, report.CUEPackagesFailed, cueEdges, report.CUEEdgesDirect, report.CUEEdgesUnresolved)
	}
	// Scoped config (docs/design/scoped-config.md §4 disclosure): report the
	// project/user pattern-count split that governed this ingest. Ingest has
	// already succeeded above (Discover already ran LoadIgnoreScoped as its
	// own enforcement point — a violating user-scope file would have failed
	// the ingest itself, never reached this line), so a second, cheap read
	// here is purely for display and cannot itself fail loudly.
	if _, counts, ierr := connector.LoadIgnoreScopedFor(svcPath, *service); ierr == nil && (counts.Project > 0 || counts.User > 0 || counts.Service > 0) {
		fmt.Fprintf(stdout, "excluded: %d pattern(s) (%d project, %d user-scope ~/.fathom/ignore, %d service-scoped ignore.d/%s)\n",
			counts.Project+counts.User+counts.Service, counts.Project, counts.User, counts.Service, *service)
	}
	if readErr != nil {
		outcome := "degraded"
		if readErr.Refused {
			outcome = "refused"
		}
		fmt.Fprintf(stderr, "error: ingest %s: %v\n", outcome, readErr)
		if h := readErr.Hint(); h != "" {
			fmt.Fprintf(stderr, "hint: %s\n", h)
		}
		return 1
	}
	// ok:false-grade disclosure (wave P3 / O82): the syntax index is
	// untouched (kept, never rolled back), but a Go tree whose typed pass
	// produced nothing to trust must never exit 0 as though nothing were
	// wrong — the exact silent gap that let a push be refused downstream
	// with no earlier signal in the ingest that ran it.
	if report.TypedFailed {
		fmt.Fprintf(stderr, "error: ingest degraded: typed-edge pass failed: %s\n", report.TypedFailReason)
		return 1
	}
	return 0
}

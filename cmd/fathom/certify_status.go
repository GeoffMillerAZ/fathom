package main

// certify_status.go — "fathom certify status" (TC-11): the certifications
// layer's own declared/live/gap counts and the case-to-symbol join rate,
// over the SAME target universe certify.Gaps computes (no --kind filter):
// a one-line health summary, where `certify gaps` is the itemized list.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func cmdCertifyStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("certify status", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a one-line summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: certify status requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	ctx := context.Background()

	corpusRoot := ""
	if meta, found, merr := st.GetServiceMeta(ctx, svc); merr == nil && found {
		corpusRoot = meta.Path
	}

	report, err := certify.Gaps(ctx, st, svc, corpusRoot, nil)
	if err != nil {
		fmt.Fprintf(stderr, "error: certify status for %q: %v\n", svc, err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(certifyStatusDoc(svc, report)); err != nil {
			return 1
		}
		return 0
	}
	if len(report.Unread) > 0 {
		fmt.Fprintf(stdout, "certify status: INCOMPLETE — %d source(s) could not be read, so these counts are a lower bound: %s\n", len(report.Unread), strings.Join(report.Unread, "; "))
	}
	if report.Summary.Declared == 0 && len(report.Unread) == 0 {
		fmt.Fprintln(stdout, "certify status: no manifests found — nothing declared")
		return 0
	}
	joinRate := 0.0
	if report.Summary.Cases > 0 {
		joinRate = float64(report.Summary.JoinedCases) / float64(report.Summary.Cases)
	}
	fmt.Fprintf(stdout, "certify status: %s: %d target(s), %d live, %d gap(s), joined %d of %d cases (%.0f%%)\n",
		svc, report.Summary.Declared, report.Summary.Live, report.Summary.Gaps,
		report.Summary.JoinedCases, report.Summary.Cases, joinRate*100)
	return 0
}

// certifyStatusDoc is `certify status --json`'s document for one service's
// report — the verb's output and the certifications page's section.
func certifyStatusDoc(svc string, report certify.GapReport) map[string]any {
	return certifyStatusJSON(svc, report.Summary, report.Unread)
}

// certifyStatusJSON is that document from a summary and its unread
// sources — the Now page's per-project section (console_home.go) builds it
// from a projected value, and one function keeps the three from
// disagreeing. Nothing declared and everything read is a skip; an
// unreadable target source is listed ("unread"), the counts are then a
// lower bound, and nothing declared is never the skip's "nothing declared"
// (I38).
func certifyStatusJSON(svc string, s certify.GapSummary, unread []string) map[string]any {
	if s.Declared == 0 && len(unread) == 0 {
		return map[string]any{"service": svc, "skip": "no manifests found — nothing declared"}
	}
	joinRate := 0.0
	if s.Cases > 0 {
		joinRate = float64(s.JoinedCases) / float64(s.Cases)
	}
	doc := map[string]any{
		"service": svc, "declared": s.Declared, "live": s.Live,
		"gaps": s.Gaps, "joined_cases": s.JoinedCases,
		"cases": s.Cases, "join_rate": joinRate,
	}
	if len(unread) > 0 {
		// The counts are a lower bound: these sources were not read (I38).
		doc["unread"] = unread
	}
	return doc
}

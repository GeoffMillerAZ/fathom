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

	if report.Summary.Declared == 0 {
		if *jsonOut {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"service": svc, "skip": "no manifests found — nothing declared"})
			return 0
		}
		fmt.Fprintln(stdout, "certify status: no manifests found — nothing declared")
		return 0
	}

	joinRate := 0.0
	if report.Summary.Cases > 0 {
		joinRate = float64(report.Summary.JoinedCases) / float64(report.Summary.Cases)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"service": svc, "declared": report.Summary.Declared, "live": report.Summary.Live,
			"gaps": report.Summary.Gaps, "joined_cases": report.Summary.JoinedCases,
			"cases": report.Summary.Cases, "join_rate": joinRate,
		}); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "certify status: %s: %d declared, %d live, %d gap(s), joined %d of %d cases (%.0f%%)\n",
		svc, report.Summary.Declared, report.Summary.Live, report.Summary.Gaps,
		report.Summary.JoinedCases, report.Summary.Cases, joinRate*100)
	return 0
}

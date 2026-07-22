package main

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// cmdMigrateStore implements the "migrate-store" subcommand: ports the six
// non-derivable tables (semantic_claims, claim_links, claim_events,
// compile_runs, service_meta, commits) from an existing DuckDB store into a
// freshly created SQLite store (ADR-020's store-swap migration path).
//
// It deliberately does NOT touch observations, fingerprints, knowledge_cards,
// or doc_summaries — those rebuild from source via a normal re-ingest, which
// this command prints the exact invocation for per migrated service.
func cmdMigrateStore(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("migrate-store", stderr)
	from := fs.String("from", "", "source DuckDB store path, must end in .duckdb (required)")
	to := fs.String("to", "", "target SQLite store path, must end in .sqlite (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *from == "" || *to == "" {
		fmt.Fprintln(stderr, "error: migrate-store requires --from and --to")
		fs.Usage()
		return 1
	}

	report, err := store.MigrateNonDerivable(context.Background(), *from, *to)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "migrated %s -> %s\n\n", *from, *to)
	fmt.Fprintln(stdout, "table            rows  spot-hash")
	for _, tr := range report.Tables {
		fmt.Fprintf(stdout, "%-16s %5d  %s\n", tr.Table, tr.RowCount, tr.SpotHash)
	}

	if len(report.ServiceIDs) == 0 {
		fmt.Fprintln(stdout, "\nno services touched — nothing to re-ingest")
		return 0
	}

	fmt.Fprintln(stdout, "\nDerived tables (observations, fingerprints, knowledge_cards, doc_summaries)")
	fmt.Fprintln(stdout, "were NOT migrated — rebuild them with a normal re-ingest against the new store:")
	fmt.Fprintln(stdout)
	for _, svc := range report.ServiceIDs {
		path := report.ServicePaths[svc]
		if path == "" {
			fmt.Fprintf(stdout, "  # %s: no recorded corpus path — supply --path yourself\n", svc)
			fmt.Fprintf(stdout, "  bin/fathom ingest --service %s --path <path-to-%s> --store %s\n", svc, svc, *to)
			continue
		}
		fmt.Fprintf(stdout, "  bin/fathom ingest --service %s --path %s --store %s\n", svc, path, *to)
	}

	return 0
}

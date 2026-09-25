package main

// sciporacle.go — `indexbench scip-oracle` (SU-12, ADR-041 decision 5,
// docs/plans/substrate.md): the free-tier L0 typed-edge oracle for a
// language beyond Go. Given a store already holding fathom's edges for a
// service (its typed edges when a native or SCIP-imported pass has run,
// falling back to the syntax call graph otherwise) and an independent SCIP
// index for the same subject, it scores fathom's answer against the SCIP
// truth and prints internal/indexbench.ScipOracleReport.
//
// docs/bench/scenarios.md documents the real-subject procedure (`task
// calibrate-clone` then `scip-typescript`, then this); this command itself
// never runs an external indexer.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func cmdScipOracle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("indexbench scip-oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	storePath := fs.String("store", "", "fathom SQLite store already holding the subject's edges (required) — never the shared fleet store for a scored run")
	service := fs.String("service", "", "service ID within the store (required)")
	scipPath := fs.String("scip", "", "SCIP index file to score fathom's edges against (required)")
	language := fs.String("language", "", "language label for the report row (default: the SCIP index's own Document.Language)")
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *storePath == "" || *service == "" || *scipPath == "" {
		fmt.Fprintln(stderr, "indexbench scip-oracle: -store, -service and -scip are all required")
		return 2
	}

	ctx := context.Background()
	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench scip-oracle: open store: %v\n", err)
		return 1
	}
	defer func() { _ = c.Close() }()

	raw, err := os.ReadFile(*scipPath)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench scip-oracle: read %s: %v\n", *scipPath, err)
		return 1
	}
	nodes, err := c.ListAllSymbols(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench scip-oracle: list symbols: %v\n", err)
		return 1
	}
	// O115: the truth edges must be mapped against the SAME service symbol
	// table fathom's own `ingest -scip` uses, or this oracle would score
	// ImportIndex's decode against itself again instead of against an
	// independent read of what fathom actually stored.
	imp, err := fscip.ImportIndex(*service, raw, nodes)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench scip-oracle: decode %s: %v\n", *scipPath, err)
		return 1
	}
	var truth []core.Edge
	for _, pkg := range imp.Packages {
		truth = append(truth, pkg.Edges...)
	}

	fathomEdges, err := fathomEdgesForOracle(ctx, c, *service)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench scip-oracle: %v\n", err)
		return 1
	}

	lang := *language
	if lang == "" {
		lang = imp.Report.Language
	}
	rep := indexbench.ScipOracle([]indexbench.ScipSubject{{Language: lang, FathomEdges: fathomEdges, Truth: truth}})
	rep.Corpus = *service

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(stderr, "indexbench scip-oracle: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s — scip truth from %s (%s, %d doc(s))\n", *service, *scipPath, imp.Report.Producer.String(), imp.Report.Documents)
	for _, row := range rep.Rows {
		fmt.Fprintf(stdout, "  %-11s %-10s truth=%-4d predicted=%-4d correct=%-4d precision=%.4f recall=%.4f unresolved_rate=%.4f\n",
			row.Language, row.EdgeKind, row.Truth, row.Predicted, row.Correct, row.Precision, row.Recall, row.UnresolvedRate)
	}
	return 0
}

// fathomEdgesForOracle returns the store's typed edges for service when it
// has any, falling back to the syntax call graph (ADR-021 §3's own
// documented fallback for a service with no typed pass) — the same
// "what does find_callers actually answer today" rule the MCP surfacing
// tools apply.
func fathomEdgesForOracle(ctx context.Context, c *client.Client, service string) ([]core.Edge, error) {
	typed, err := c.TypedEdges(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("typed edges: %w", err)
	}
	if len(typed) > 0 {
		return typed, nil
	}
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("symbols: %w", err)
	}
	g := callgraph.Build(nodes)
	var edges []core.Edge
	for _, n := range nodes {
		for _, ci := range callgraph.CalleesDetail(g, n.Key()) {
			edges = append(edges, core.Edge{FromSymbol: n.Key(), ToSymbol: ci.Key, Kind: ci.Kind})
		}
	}
	return edges, nil
}

// Command indexbench runs the bench pyramid's L0 index oracle
// (docs/design/bench-pyramid.md §3.1, plan bench-pyramid BP-14) over a real
// Go module at HEAD: it ingests the tree into a throwaway probe store, asks
// fathom the same questions the find_callers / references / blast_radius MCP
// tool handlers ask their backend, and scores the answers against an
// independent go/types truth.
//
// It spends no tokens and talks to no daemon. It is slower than the fixture
// oracle in `task validate` (a whole module's type-check plus a whole
// module's ingest), so it runs on demand: `task bench-l0-self`.
//
// Usage:
//
//	indexbench -root <module root> [-service self] [-store <path>] [-out <file>]
//
// The report is the same schema internal/indexbench commits as a golden, so
// a self run and a fixture run are directly comparable.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("indexbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "module root to score (must contain go.mod)")
	service := fs.String("service", "self", "service id to ingest the tree under")
	storePath := fs.String("store", "", "probe store path (default: in-memory; never the shared fleet store)")
	out := fs.String("out", "", "write the report JSON here (default: stdout)")
	maxBlast := fs.Int("max-blast-queries", 20, "bound blast_radius separately — it rebuilds the call graph on every call, so it costs ~10s a query on a ten-thousand-symbol service")
	maxQ := fs.Int("max-queries", 200, "bound each tool to this many deterministic stride-sampled questions (0 = every one; a whole module scored exhaustively is minutes, not seconds)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 2
	}
	ctx := context.Background()

	started := time.Now()
	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: open probe store: %v\n", err)
		return 1
	}
	defer func() { _ = c.Close() }()

	report, err := c.Ingest(ctx, core.ServiceRef{ID: *service, Path: abs})
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: ingest %s: %v\n", abs, err)
		return 1
	}
	ingested := time.Since(started)
	fmt.Fprintf(stderr, "indexbench: ingested %d files, %d symbols in %s\n",
		report.Discovered, report.Symbols, ingested.Round(time.Millisecond))

	// The LocalConnector reports paths relative to the service root, which is
	// the same root go/types walks — so no prefix translation is needed.
	rep, err := indexbench.IndexOracleReport(ctx, c, []indexbench.Subject{
		{Service: *service, Root: abs, MaxQueries: *maxQ, MaxBlastQueries: *maxBlast},
	})
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 1
	}
	rep.Corpus = abs

	data, err := marshal(rep)
	if err != nil {
		fmt.Fprintf(stderr, "indexbench: %v\n", err)
		return 1
	}
	if *out == "" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "indexbench: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "indexbench: wrote %s\n", *out)
	}

	// Every rate is printed WITH its denominator: a bench that prints
	// "precision 0.98" and hides "of 51" has told the reader a feeling, not
	// a measurement (ADR-031).
	for _, svc := range rep.Services {
		fmt.Fprintf(stdout, "\n%s — %d go packages (%d with type errors), %d indexed symbols\n",
			svc.Service, svc.GoPackages, svc.PackagesWithErrors, svc.IndexedSymbols)
		for _, t := range svc.Tools {
			fmt.Fprintf(stdout, "  %-13s precision %.4f (%d/%d)  recall %.4f (%d/%d)  over %d of %d queries\n",
				t.Tool, t.Precision, t.Correct, t.Predicted, t.Recall, t.Correct, t.Truth, t.Queries, t.Population)
			fmt.Fprintf(stdout, "  %-13s %s\n", "", time.Duration(t.ElapsedMS)*time.Millisecond)
		}
	}
	fmt.Fprintf(stdout, "\ntotal %s (ingest %s)\n",
		time.Since(started).Round(time.Millisecond), ingested.Round(time.Millisecond))
	return 0
}

func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/sextant/pkg/client"
)

func cmdQuery(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("query", stderr)
	service := fs.String("service", "", "service ID to query (required)")
	concern := fs.String("concern", "", "concern ID to scope the query (required)")
	keywords := fs.String("keywords", "", "comma-separated keywords to rank symbols")
	top := fs.Int("top", 20, "maximum number of results to return")
	storePath := fs.String("store", "workspace/db/sextant.duckdb", "DuckDB store path")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	if *concern == "" {
		fmt.Fprintln(stderr, "error: --concern is required")
		fs.Usage()
		return 1
	}

	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	var kws []string
	if *keywords != "" {
		for _, kw := range strings.Split(*keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				kws = append(kws, kw)
			}
		}
	}

	ctx := context.Background()

	result, err := c.QuerySymbols(ctx, client.Query{
		ServiceID: *service,
		ConcernID: *concern,
		Keywords:  kws,
		TopN:      *top,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: query failed: %v\n", err)
		return 1
	}

	// Unscoped-store keyword fallback lives in the query engine, so every
	// surface (CLI, MCP, Go client) behaves identically.
	nodes := result.Nodes

	if len(nodes) == 0 {
		fmt.Fprintln(stdout, "no symbols found")
		return 0
	}

	// Honour --top.
	if *top > 0 && len(nodes) > *top {
		nodes = nodes[:*top]
	}

	for i, node := range nodes {
		fmt.Fprintf(stdout, "%3d  %-40s  %s  %s\n",
			i+1, node.Name, node.Kind, node.FilePath)
		if node.Signature != "" {
			fmt.Fprintf(stdout, "     %s\n", node.Signature)
		}
	}

	if result.Staleness.ReingestRecommended {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "warning: staleness detected — re-ingest recommended")
	}

	return 0
}

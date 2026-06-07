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

	// When the concern-scoped query returns no results (e.g. the store has
	// unscoped observations from a plain ingest with no concern tagging),
	// fall back to a keyword search over all symbols for the service.
	nodes := result.Nodes
	if len(nodes) == 0 && len(kws) > 0 {
		allNodes, findErr := c.ListAllSymbols(ctx, *service)
		if findErr != nil {
			fmt.Fprintf(stderr, "error: fallback search failed: %v\n", findErr)
			return 1
		}
		// Filter by keywords manually.
		for _, n := range allNodes {
			for _, kw := range kws {
				if strings.Contains(strings.ToLower(n.Name+" "+n.Signature), strings.ToLower(kw)) {
					nodes = append(nodes, n)
					break
				}
			}
		}
	}

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

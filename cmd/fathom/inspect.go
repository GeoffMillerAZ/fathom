package main

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func cmdInspect(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("inspect", stderr)
	service := fs.String("service", "", "service ID to inspect (required)")
	kind := fs.String("kind", "", "filter by SourceKind (e.g. GoFunc, GoType)")
	storePath := fs.String("store", "workspace/db/fathom.duckdb", "DuckDB store path")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}

	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	nodes, err := c.ListAllSymbols(context.Background(), *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: inspect failed: %v\n", err)
		return 1
	}

	if *kind != "" {
		want := core.SourceKind(*kind)
		filtered := nodes[:0]
		for _, n := range nodes {
			if n.Kind == want {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	fmt.Fprintf(stdout, "service: %s\n", *service)
	fmt.Fprintf(stdout, "symbols: %d\n", len(nodes))
	if len(nodes) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, "")
	for _, n := range nodes {
		fmt.Fprintf(stdout, "  %-10s  %-40s  %s\n", n.Kind, n.Name, n.FilePath)
	}
	return 0
}

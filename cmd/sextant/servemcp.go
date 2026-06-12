package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/sextant/internal/mcp"
	"github.com/geoffmilleraz/sextant/pkg/client"
)

// cmdServeMCP is the entry point for the "serve-mcp" subcommand.
// It reads from os.Stdin and writes to stdout.
func cmdServeMCP(args []string, stdout, stderr io.Writer) int {
	return runServeMCP(args, os.Stdin, stdout, stderr)
}

// runServeMCP is the testable core of "serve-mcp". stdin is injected so
// tests can feed finite byte streams without blocking.
func runServeMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("serve-mcp", stderr)
	storePath := fs.String("store", "", "DuckDB store path (required)")
	mode := fs.String("mode", "local", "connector mode: local or faux")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		fmt.Fprintln(stderr, "error: --store is required")
		fs.Usage()
		return 1
	}

	m, ok := parseModeFlag(*mode)
	if !ok {
		fmt.Fprintf(stderr, "error: unknown --mode %q (want local or faux)\n", *mode)
		return 1
	}

	c, err := client.NewConstantClient(m, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	srv := mcp.NewServerWithFleet(c, &fleetAdapter{c: c, stderr: stderr})

	if err := srv.Serve(context.Background(), stdin, stdout); err != nil {
		// EOF / context cancel are expected end-of-stream signals; only
		// surface genuine errors.
		fmt.Fprintf(stderr, "error: mcp serve: %v\n", err)
		return 1
	}
	return 0
}

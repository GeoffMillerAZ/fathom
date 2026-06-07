package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/sextant/pkg/client"
)

// cmdPlatformIngest is the entry point for the "platform-ingest" subcommand.
//
// It iterates over the immediate subdirectories of --root, treats each as a
// service (using the directory name as the service ID), ingests it via a
// per-service local-mode client into <store-dir>/<service>.duckdb, prints a
// per-service summary, and finally prints totals.
func cmdPlatformIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("platform-ingest", stderr)
	root := fs.String("root", "", "directory containing service subdirs (required)")
	storeDir := fs.String("store-dir", "", "output directory for per-service .duckdb files (required)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *root == "" {
		fmt.Fprintln(stderr, "error: --root is required")
		fs.Usage()
		return 1
	}
	if *storeDir == "" {
		fmt.Fprintln(stderr, "error: --store-dir is required")
		fs.Usage()
		return 1
	}

	// Ensure the output directory exists before opening per-service stores
	// (DuckDB cannot create a file in a non-existent directory).
	if err := os.MkdirAll(*storeDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: creating --store-dir %q: %v\n", *storeDir, err)
		return 1
	}

	// Enumerate immediate subdirectories of root.
	entries, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading --root %q: %v\n", *root, err)
		return 1
	}

	var services []string
	for _, e := range entries {
		if e.IsDir() {
			services = append(services, e.Name())
		}
	}

	if len(services) == 0 {
		fmt.Fprintf(stdout, "no services found in %s\n", *root)
		fmt.Fprintf(stdout, "total: 0 services, 0 symbols\n")
		return 0
	}

	ctx := context.Background()

	var totalSymbols, totalDiscovered int
	var failed []string

	for _, svcID := range services {
		svcPath := filepath.Join(*root, svcID)
		storePath := filepath.Join(*storeDir, svcID+".duckdb")

		c, openErr := client.NewConstantClient(client.ModeLocal, storePath)
		if openErr != nil {
			fmt.Fprintf(stderr, "error: %s: opening client: %v\n", svcID, openErr)
			failed = append(failed, svcID)
			continue
		}

		report, ingestErr := c.Ingest(ctx, client.ServiceRef{
			ID:   svcID,
			Path: svcPath,
		})
		c.Close() //nolint:errcheck

		if ingestErr != nil {
			fmt.Fprintf(stderr, "error: %s: ingest failed: %v\n", svcID, ingestErr)
			failed = append(failed, svcID)
			continue
		}

		fmt.Fprintf(stdout, "service:    %s\n", svcID)
		fmt.Fprintf(stdout, "  discovered: %d\n", report.Discovered)
		fmt.Fprintf(stdout, "  extracted:  %d\n", report.Extracted)
		fmt.Fprintf(stdout, "  skipped:    %d\n", report.Skipped)
		fmt.Fprintf(stdout, "  symbols:    %d\n", report.Symbols)

		totalDiscovered += report.Discovered
		totalSymbols += report.Symbols
	}

	succeeded := len(services) - len(failed)
	fmt.Fprintf(stdout, "\ntotal: %d services, %d discovered, %d symbols\n",
		succeeded, totalDiscovered, totalSymbols)

	if len(failed) > 0 {
		fmt.Fprintf(stderr, "failed services (%d): %v\n", len(failed), failed)
		return 1
	}
	return 0
}

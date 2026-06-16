package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
	autoRefresh := fs.Bool("auto-refresh", true, "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")

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
		if isStoreLockConflict(err) {
			fmt.Fprintf(stderr, "error: another sextant process already holds the store %q.\n", *storePath)
			fmt.Fprintf(stderr, "  %v\n", err)
			fmt.Fprintln(stderr, "  DuckDB is single-writer: only one serve-mcp can hold a store. Reap the other")
			fmt.Fprintln(stderr, "  process (the lock error above names its PID), or point --store at a different file.")
			return 1
		}
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// Announce build identity at startup. A long-running serve-mcp process can
	// outlive several commits; logging the embedded VCS revision makes a stale
	// server self-evident (compare against `git rev-parse HEAD`). See
	// docs/dogfood-observations.md (staleness finding).
	fmt.Fprintf(stderr, "sextant serve-mcp starting: %s\n", BuildRevision())

	// Default: keep the index fresh transparently. A query for a service whose
	// git HEAD has drifted from the indexed commit triggers a hash-gated
	// re-ingest before the answer — removing the manual staleness→ingest loop.
	var backend mcp.QueryInterface = c
	if *autoRefresh {
		backend = mcp.NewAutoRefresh(c, stderr)
	}

	srv := mcp.NewServerWithFleet(backend, &fleetAdapter{c: c, stderr: stderr})

	if err := srv.Serve(context.Background(), stdin, stdout); err != nil {
		// EOF / context cancel are expected end-of-stream signals; only
		// surface genuine errors.
		fmt.Fprintf(stderr, "error: mcp serve: %v\n", err)
		return 1
	}
	return 0
}

// isStoreLockConflict reports whether err is DuckDB's single-writer lock
// conflict (another process already holds the store file).
func isStoreLockConflict(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Conflicting lock") || strings.Contains(s, "Could not set lock")
}

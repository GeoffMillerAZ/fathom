package main

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// parseModeFlag converts a user-supplied mode string to a client.Mode constant.
func parseModeFlag(s string) (client.Mode, bool) {
	switch s {
	case "local":
		return client.ModeLocal, true
	case "faux":
		return client.ModeFaux, true
	case "github":
		return client.ModeGitHub, true
	}
	return 0, false
}

func cmdIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("ingest", stderr)
	service := fs.String("service", "", "logical service ID (required)")
	path := fs.String("path", ".", "path to service source root (local mode)")
	storePath := fs.String("store", "workspace/db/fathom.sqlite", "store path (.duckdb or .sqlite/.db)")
	mode := fs.String("mode", "local", "connector mode: local or faux")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
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

	// For ModeFaux the connector resolves svc.Path as the fauxtown subdirectory
	// name, so we pass the service ID.  For ModeLocal we use --path.
	svcPath := *path
	if m == client.ModeFaux {
		svcPath = *service
	}

	report, err := c.Ingest(context.Background(), client.ServiceRef{
		ID:   *service,
		Path: svcPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: ingest failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "service:    %s\n", *service)
	fmt.Fprintf(stdout, "discovered: %d\n", report.Discovered)
	fmt.Fprintf(stdout, "extracted:  %d\n", report.Extracted)
	fmt.Fprintf(stdout, "skipped:    %d\n", report.Skipped)
	fmt.Fprintf(stdout, "symbols:    %d\n", report.Symbols)
	// Unclassified/oversize/classified-zero (ADR-021 §2 / TI-6, F5): print
	// only when non-zero so the common case stays as quiet as it always was
	// — a clean chart/repo classifies everything and these counters are 0.
	if report.Unclassified > 0 {
		fmt.Fprintf(stdout, "unclassified: %d\n", report.Unclassified)
	}
	if report.Oversize > 0 {
		fmt.Fprintf(stdout, "oversize:   %d\n", report.Oversize)
	}
	if report.ClassifiedZero > 0 {
		fmt.Fprintf(stdout, "classified-but-zero-symbols: %d\n", report.ClassifiedZero)
	}
	// Typed-edge counters (ADR-021 §3), non-zero-only like the above: their
	// absence for a Go service is a signal (the silent relative-path no-op
	// was invisible precisely because the CLI never printed these).
	if report.TypedPackages > 0 || report.TypedPackagesFailed > 0 {
		fmt.Fprintf(stdout, "typed-edges: %d pkg(s) (%d failed), %d direct / %d iface-dispatch / %d unresolved\n",
			report.TypedPackages, report.TypedPackagesFailed,
			report.TypedEdgesDirect, report.TypedEdgesIfaceDispatch, report.TypedEdgesUnresolved)
	}
	return 0
}

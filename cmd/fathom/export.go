// export.go — `fathom export scip` (VB-32, docs/plans/verbs.md;
// docs/vision/29-verbs-not-lookups.md §I): writes one service's symbol and
// typed-edge tables as a SCIP index (internal/analysis/scipexport, over the
// generated wire-format types vendored at third_party/scip — see that
// directory's README.md for why), store-direct only (mirrors
// cmd/fathom/review.go's structure — no daemon round trip). `ingest --scip`
// (importing FROM a SCIP index) is a later, separate decision
// (docs/usage.md).
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/analysis/scipexport"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdExport dispatches `fathom export <subcommand>`. scip is the only
// subcommand today.
func cmdExport(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: export requires a subcommand (scip)")
		return 1
	}
	switch args[0] {
	case "scip":
		return cmdExportSCIP(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "error: unknown export subcommand %q (scip)\n", args[0])
	return 1
}

// cmdExportSCIP implements `fathom export scip --service <s> [--store PATH]
// [-o index.scip] [--module PATH]`:
//
//  1. Opens --store directly (default: defaultStorePath(), the same
//     workspace/db/fathom.sqlite every store-direct command defaults to)
//     and lists service's symbols + typed edges + sources.
//  2. Renders them as a SCIP index (internal/analysis/scipexport.Build) and
//     writes the marshaled bytes to -o.
//  3. Round-trips: internal/analysis/scipexport.RoundTrip decodes the
//     written bytes (or, when scip-go is on PATH and --module names a real
//     Go module root, runs scip-go independently over it) and compares the
//     resulting symbol table against the store's own — printed as one
//     summary line, never fatal to the export (a mismatch is reported, not
//     hidden, but the index file itself is already written).
func cmdExportSCIP(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export scip", stderr)
	service := fs.String("service", "", "service ID to export (required)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	out := fs.String("o", "index.scip", "output SCIP index path")
	modulePath := fs.String("module", "", "optional: a real Go module root on disk for the scip-go round-trip comparison (ignored when scip-go is not on PATH)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: export scip requires --service")
		fs.Usage()
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ctx := context.Background()
	nodes, err := c.ListAllSymbols(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: list symbols for %q: %v\n", *service, err)
		return 1
	}
	if len(nodes) == 0 {
		fmt.Fprintf(stderr, "error: service %q has no indexed symbols in %q — ingest it first\n", *service, *storePath)
		return 1
	}
	idx, err := scipexport.Build(ctx, c, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: build SCIP index: %v\n", err)
		return 1
	}

	data, err := scipexport.Marshal(idx)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write %q: %v\n", *out, err)
		return 1
	}

	report, err := scipexport.RoundTrip(*service, nodes, data, *modulePath)
	if err != nil {
		// The index is already written and valid input for RoundTrip
		// failed only if the bytes we JUST wrote can't be decoded — that
		// would mean scipexport.Marshal produced something scipexport's
		// own DecodeIndex rejects, a real bug worth surfacing loudly.
		fmt.Fprintf(stderr, "error: round-trip check: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "wrote %s: %d document(s), %d symbol(s)\n", *out, len(idx.Documents), len(nodes))
	fmt.Fprintln(stdout, report.String())
	return 0
}

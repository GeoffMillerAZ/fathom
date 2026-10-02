package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// cmdRemap implements the "remap" subcommand: point a service's recorded
// corpus root at a new location (ADR-017 store-level root remap).
//
// Because every anchor in the store is repo-relative, moving a corpus (or the
// whole store to another machine) needs exactly one metadata update — this
// command. Queries never depended on the root; source fetching, freshness
// watermarks, and re-ingest resolve through the remapped path immediately.
func cmdRemap(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("remap", stderr)
	storePath := fs.String("store", "", "path to the store, .sqlite/.db (required)")
	service := fs.String("service", "", "service ID to remap (required)")
	newPath := fs.String("path", "", "new corpus root directory (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" || *newPath == "" {
		fmt.Fprintln(stderr, "error: remap requires --store, --service, and --path")
		return 1
	}
	info, err := os.Stat(*newPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "error: --path %q is not an existing directory\n", *newPath)
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	ctx := context.Background()
	meta, found, err := st.GetServiceMeta(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: read service meta: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "error: service %q has no recorded meta in this store\n", *service)
		return 1
	}
	old := meta.Path
	meta.Path = *newPath
	if err := st.SetServiceMeta(ctx, meta); err != nil {
		fmt.Fprintf(stderr, "error: write service meta: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "remapped %s: %s -> %s\n", *service, old, *newPath)
	return 0
}

// remapRunner runs `fathom remap` (registered below; EW-45).
type remapRunner struct{}

func (remapRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdRemap(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "remap", Group: "plans", Order: 30, Run: remapRunner{},
		Summary: "Point a service's recorded corpus root at a new location"})
}

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fleetAdapter exposes a single multi-service store client as the
// mcp.FleetBackend interface, so fleet tools (list_services,
// query_cross_service, fleet_drift, cross_repo_callers) work when one SQLite
// file holds many services — no per-service store directory required.
//
// The FleetBackend interface is synchronous and error-free by design; errors
// are reported to stderr and yield empty results rather than crashing the
// serve loop.
type fleetAdapter struct {
	c      *client.Client
	stderr io.Writer
}

func (f *fleetAdapter) Services() []string {
	ids, err := f.c.Services(context.Background())
	if err != nil {
		fmt.Fprintf(f.stderr, "fleet: services: %v\n", err)
		return nil
	}
	return ids
}

func (f *fleetAdapter) SymbolsFor(service string) []client.SymbolNode {
	nodes, err := f.c.ListAllSymbols(context.Background(), service)
	if err != nil {
		fmt.Fprintf(f.stderr, "fleet: symbols for %s: %v\n", service, err)
		return nil
	}
	return nodes
}

func (f *fleetAdapter) ObservationsFor(service string) []client.Observation {
	obs, err := f.c.Observations(context.Background(), service)
	if err != nil {
		fmt.Fprintf(f.stderr, "fleet: observations for %s: %v\n", service, err)
		return nil
	}
	return obs
}

// UnsupportedLanguagesFor re-scans service's live source tree (via
// client.Sources, the same connector-backed re-read cloc/secret_scan use)
// and tallies files whose extension names a known programming language with
// no registered extractor (O69) — mcp.FleetBackend's contract. Returns nil
// on a fetch error (logged to stderr, matching every other method here) or
// when nothing is unsupported.
func (f *fleetAdapter) UnsupportedLanguagesFor(service string) map[string]int {
	files, err := f.c.Sources(context.Background(), service)
	if err != nil {
		fmt.Fprintf(f.stderr, "fleet: sources for %s: %v\n", service, err)
		return nil
	}
	var out map[string]int
	for _, sf := range files {
		if sf.Lang != core.LangUnclassified {
			continue
		}
		lang, ok := connector.UnsupportedLanguageForPath(sf.Path)
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]int)
		}
		out[lang]++
	}
	return out
}

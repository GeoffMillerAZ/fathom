package main

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fleetAdapter exposes a single multi-service store client as the
// mcp.FleetBackend interface, so fleet tools (list_services,
// query_cross_service, fleet_drift, cross_repo_callers) work when one DuckDB
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

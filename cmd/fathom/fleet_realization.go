package main

// fleet_realization.go — `fathom fleet realization` (OC-25, RG-5): one
// repository's realization ladder, wired → fresh → queried → preventing →
// gating, each rung with its evidence and the first unmet rung with its
// remedy. It reads the same observatory model the console builds (the
// registry, and over the last fourteen days the call log the daemon writes,
// the session ledger and the verify ledger), and the project overview's
// ladder panel renders this verb's JSON (P10).

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func cmdFleetRealization(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fleet realization", stderr)
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	jsonOut := fs.Bool("json", false, "emit the ladder as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	m := buildObservatoryModel(*url, fleetReportNow())
	if m.DegradedNote != "" {
		fmt.Fprintf(stderr, "error: the service registry could not be read: %s\n", m.DegradedNote)
		return 1
	}
	l, ok := modelLadder(m, *service)
	if !ok {
		fmt.Fprintf(stderr, "error: no indexed service named %q\n", *service)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(l)
		return 0
	}
	fmt.Fprintf(stdout, "%s: %s (%d of %d rungs realized)\n", l.Service, l.Grade, l.Realized, l.Available)
	for _, r := range l.Rungs {
		fmt.Fprintf(stdout, "  %d. %-10s %-14s %s\n", r.Order, r.Name, r.State, r.Evidence)
		if r.Remedy != "" {
			fmt.Fprintf(stdout, "     remedy: %s\n", r.Remedy)
		}
	}
	return 0
}

// modelLadder is one service's ladder from the model.
func modelLadder(m observatoryModel, service string) (console.Ladder, bool) {
	for _, s := range m.Services {
		if strings.EqualFold(s.Service, service) && s.ladder != nil {
			return *s.ladder, true
		}
	}
	return console.Ladder{}, false
}

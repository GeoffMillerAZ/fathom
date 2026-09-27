// Package mcp — cross-service call stitching for symbol_context and
// find_callers against a fleet store (P1-2, closing O12(a): "the fleet
// index exists to answer 'who calls this, anywhere?' — and that was
// exactly its worst measured case").
//
// The per-service QueryInterface backend only ever sees one service's
// symbols, so its callgraph stops dead at a service boundary. When a
// FleetBackend is ALSO wired (fleet stores; see NewServerWithFleet), the
// helpers below extend symbol_context / find_callers using the SAME
// fleet-wide backend path cross_repo_callers already established
// (platform_tools.go's FleetBackend.Services / SymbolsFor / ObservationsFor
// + matchesSymbol) — reused verbatim, not reinvented — and every hit is
// scope-labeled "cross-service" and placed in its own "cross_service_*"
// result field, so it can never be silently merged with the existing
// same-service "callers"/"callees" fields.
package mcp

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// crossServiceScope is the label attached to every cross-service hit
// returned below, so a caller reading (or re-merging) the results can
// always tell a cross-service edge from a same-service one.
const crossServiceScope = "cross-service"

// crossServiceHit is one fleet-wide call-edge hit (caller or callee side).
type crossServiceHit struct {
	Service  string
	Symbol   string
	FilePath string
}

func (h crossServiceHit) asAny() map[string]any {
	return map[string]any{
		"service":   h.Service,
		"symbol":    h.Symbol,
		"file_path": h.FilePath,
		"scope":     crossServiceScope,
	}
}

// crossServiceHitsToAny renders hits (already sorted by the caller) as
// JSON-ready maps, or nil for an empty slice so callers can omit the field.
func crossServiceHitsToAny(hits []crossServiceHit) []any {
	if len(hits) == 0 {
		return nil
	}
	out := make([]any, len(hits))
	for i, h := range hits {
		out[i] = h.asAny()
	}
	return out
}

// fleetCallIndex is the OPTIONAL FleetBackend capability that answers
// "who calls X, anywhere?" from the store's call index (RT-77's
// observation_calls) instead of loading every service whole — the
// wave-B proof measured ~3.8 GiB allocated per symbol_context call through
// the scan below (RT-84, docs/design/responsible-tenant.md §4.5).
// CallsIndexed names the services whose index is complete; CallersOfKeys
// skips the rest, so those keep the scan, one service at a time.
type fleetCallIndex interface {
	CallersOfKeys(ctx context.Context, keys []string) ([]store.CallerRow, error)
	CallsIndexed(ctx context.Context, service string) bool
}

// fleetNameLookup is the OPTIONAL FleetBackend capability resolving
// declarations by exact name over the (service_id, symbol) index.
type fleetNameLookup interface {
	SymbolSourcesForNames(ctx context.Context, service string, names []string) ([]store.SymbolSource, error)
}

// fleetCallers returns every symbol, anywhere in the fleet, whose stored
// Calls list contains a callee matching symbol (matchesSymbol) —
// cross_repo_callers' (platform_tools.go registerCrossRepoCallers) OWN
// matching logic, in one place: the cross_repo_callers tool and
// symbol_context/find_callers's cross-service stitching both call this.
// Sorted by (service, symbol) for determinism. nil fleet or empty symbol
// return nil.
//
// RT-84: against a call-indexed store it is ONE indexed query
// (CallersOfKeys on the symbol and its final name segment — a candidate
// superset narrowed here by matchesSymbol); only a service whose call index
// is not complete is still scanned.
func fleetCallers(ctx context.Context, fleet FleetBackend, symbol string) []crossServiceHit {
	if fleet == nil || symbol == "" {
		return nil
	}
	seen := make(map[string]bool)
	var hits []crossServiceHit
	add := func(svc, name, file string) {
		key := svc + "\x00" + name
		if !seen[key] {
			seen[key] = true
			hits = append(hits, crossServiceHit{Service: svc, Symbol: name, FilePath: file})
		}
	}
	scan := fleet.Services()
	if idx, ok := fleet.(fleetCallIndex); ok {
		rows, err := idx.CallersOfKeys(ctx, []string{symbol, store.CalleeName(symbol)})
		if err == nil {
			for _, r := range rows {
				if matchesSymbol(r.CalleeKey, symbol) {
					add(r.ServiceID, r.Symbol, r.SourceRef)
				}
			}
			var rest []string
			for _, svc := range scan {
				if !idx.CallsIndexed(ctx, svc) {
					rest = append(rest, svc)
				}
			}
			scan = rest
		}
	}
	if len(scan) > 0 {
		scanFleetCallers(ctx, fleet, scan, symbol, add)
	}
	sortCrossServiceHits(hits)
	return hits
}

// fleetScanSlot admits one legacy fleetCallers scan process-wide.
var fleetScanSlot = make(chan struct{}, 1)

// scanFleetCallers is the scan for services without a complete call
// index: each one's symbols and observations are loaded whole. One scan at a time —
// concurrent scans multiplied into the wave-A replay's ~6 GiB.
func scanFleetCallers(ctx context.Context, fleet FleetBackend, services []string, symbol string, add func(svc, name, file string)) {
	select {
	case fleetScanSlot <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-fleetScanSlot }()
	for _, svc := range services {
		if ctx.Err() != nil {
			return
		}
		for _, n := range fleet.SymbolsFor(svc) {
			for _, callee := range n.Calls {
				if matchesSymbol(callee, symbol) {
					add(svc, n.Name, n.FilePath)
					break
				}
			}
		}
		// Observation Calls too, for backends whose symbol view is
		// narrower than their observation set.
		for _, obs := range fleet.ObservationsFor(svc) {
			for _, callee := range obs.Calls {
				if matchesSymbol(callee, symbol) {
					add(svc, obs.Symbol, obs.SourceRef)
					break
				}
			}
		}
	}
}

func sortCrossServiceHits(hits []crossServiceHit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Service != hits[j].Service {
			return hits[i].Service < hits[j].Service
		}
		return hits[i].Symbol < hits[j].Symbol
	})
}

// crossServiceCallers returns fleetCallers(fleet, symbol) filtered to hits
// OUTSIDE homeService. Same-service callers are already covered by the
// per-service QueryInterface backend's own "callers" field, so this only
// ever ADDS new (cross-service) information — never a duplicate of what's
// already there. nil fleet yields nil (the field is simply omitted).
func crossServiceCallers(ctx context.Context, fleet FleetBackend, homeService, symbol string) []any {
	all := fleetCallers(ctx, fleet, symbol)
	var out []crossServiceHit
	for _, h := range all {
		if h.Service != homeService {
			out = append(out, h)
		}
	}
	return crossServiceHitsToAny(out)
}

// crossServiceCallees resolves def's own outbound Calls keys against every
// OTHER fleet service's exported symbols, mirroring platformlink.
// BuildFromSymbols' Phase 2 qualified-match rule (internal/platformlink/
// platformlink.go): a callee key qualifies only when it is package-
// qualified ("pkg.Name") and NOT already service-resolved ("::"-prefixed
// keys are same-service by construction; bare unqualified names are too
// ambiguous fleet-wide to cross-link). Unlike platformlink's own Graph —
// which records the calling FILE, not the calling symbol, into
// core.CrossRepoEdge — this resolves ONE symbol's own Calls list directly,
// which is the precision symbol_context needs (a file can hold more than
// one function; platformlink's file-level granularity would attribute
// another function's cross-service call to this one). nil fleet, or a def
// with no Calls, returns nil.
func crossServiceCallees(ctx context.Context, fleet FleetBackend, homeService string, def core.SymbolNode) []any {
	if fleet == nil || len(def.Calls) == 0 {
		return nil
	}
	// Each qualifying key once (RT-84), split into every (package, name)
	// reading — a name may itself carry a dot ("T.Method").
	type pkgName struct{ pkg, name string }
	want := map[pkgName]bool{}
	var names []string
	for _, callKey := range dedupeStrings(def.Calls) {
		if strings.Contains(callKey, "::") || !strings.Contains(callKey, ".") {
			continue
		}
		for i := 0; i < len(callKey); i++ {
			if callKey[i] == '.' && i > 0 && i < len(callKey)-1 {
				pn := pkgName{callKey[:i], callKey[i+1:]}
				if !want[pn] {
					want[pn] = true
					names = append(names, pn.name)
				}
			}
		}
	}
	if len(want) == 0 {
		return nil
	}
	var out []crossServiceHit
	seen := make(map[string]bool)
	add := func(svc, pkg, name, file string) {
		if pkg == "" || name == "" || !want[pkgName{pkg, name}] {
			return
		}
		key := svc + "\x00" + name
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, crossServiceHit{Service: svc, Symbol: name, FilePath: file})
	}
	lookup, indexed := fleet.(fleetNameLookup)
	for _, svc := range fleet.Services() {
		if svc == homeService || ctx.Err() != nil {
			continue
		}
		if indexed {
			rows, err := lookup.SymbolSourcesForNames(ctx, svc, names)
			if err == nil {
				for _, r := range rows {
					add(svc, r.Package, r.Symbol, r.SourceRef)
				}
				continue
			}
		}
		for _, n := range fleet.SymbolsFor(svc) {
			add(svc, n.Package, n.Name, n.FilePath)
		}
	}
	sortCrossServiceHits(out)
	return crossServiceHitsToAny(out)
}

// dedupeStrings returns ss's distinct values in first-seen order.
func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

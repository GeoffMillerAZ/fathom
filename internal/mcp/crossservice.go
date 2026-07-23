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
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
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

// fleetCallers returns every symbol, anywhere in the fleet, whose stored
// Calls (or observation Calls) list contains a callee matching symbol —
// cross_repo_callers' (platform_tools.go registerCrossRepoCallers) OWN
// matching logic, extracted here so it has exactly one implementation:
// the cross_repo_callers tool and symbol_context/find_callers's
// cross-service stitching both call this, rather than two copies drifting
// apart. Sorted by (service, symbol) for determinism. nil fleet or empty
// symbol return nil.
func fleetCallers(fleet FleetBackend, symbol string) []crossServiceHit {
	if fleet == nil || symbol == "" {
		return nil
	}
	var hits []crossServiceHit
	seen := make(map[string]bool)
	for _, svc := range fleet.Services() {
		for _, n := range fleet.SymbolsFor(svc) {
			for _, callee := range n.Calls {
				if matchesSymbol(callee, symbol) {
					key := svc + "\x00" + n.Name
					if !seen[key] {
						seen[key] = true
						hits = append(hits, crossServiceHit{Service: svc, Symbol: n.Name, FilePath: n.FilePath})
					}
					break
				}
			}
		}
	}
	// Also search observation Calls for backends whose symbol view is
	// narrower than their observation set (mirrors registerCrossRepoCallers).
	for _, svc := range fleet.Services() {
		for _, obs := range fleet.ObservationsFor(svc) {
			key := svc + "\x00" + obs.Symbol
			if seen[key] {
				continue
			}
			for _, callee := range obs.Calls {
				if matchesSymbol(callee, symbol) {
					seen[key] = true
					hits = append(hits, crossServiceHit{Service: svc, Symbol: obs.Symbol})
					break
				}
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Service != hits[j].Service {
			return hits[i].Service < hits[j].Service
		}
		return hits[i].Symbol < hits[j].Symbol
	})
	return hits
}

// crossServiceCallers returns fleetCallers(fleet, symbol) filtered to hits
// OUTSIDE homeService. Same-service callers are already covered by the
// per-service QueryInterface backend's own "callers" field, so this only
// ever ADDS new (cross-service) information — never a duplicate of what's
// already there. nil fleet yields nil (the field is simply omitted).
func crossServiceCallers(fleet FleetBackend, homeService, symbol string) []any {
	all := fleetCallers(fleet, symbol)
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
func crossServiceCallees(fleet FleetBackend, homeService string, def core.SymbolNode) []any {
	if fleet == nil || len(def.Calls) == 0 {
		return nil
	}
	var out []crossServiceHit
	seen := make(map[string]bool)
	for _, callKey := range def.Calls {
		if strings.Contains(callKey, "::") || !strings.Contains(callKey, ".") {
			continue
		}
		for _, svc := range fleet.Services() {
			if svc == homeService {
				continue
			}
			for _, n := range fleet.SymbolsFor(svc) {
				if n.Package == "" || n.Name == "" || n.Package+"."+n.Name != callKey {
					continue
				}
				key := svc + "\x00" + n.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, crossServiceHit{Service: svc, Symbol: n.Name, FilePath: n.FilePath})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].Symbol < out[j].Symbol
	})
	return crossServiceHitsToAny(out)
}

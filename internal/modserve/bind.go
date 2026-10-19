package modserve

import (
	"context"
	"os"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ModuleFederator is the minimal capability Bind needs from a module host —
// satisfied structurally by *modulehost.Host (its FederatedObligations and
// ModuleStaleness methods already have these exact signatures) or a fake
// for testing. Kept narrow (mirrors internal/mcp's own moduleBackend
// interface pattern) rather than depending on the whole of
// modulehost.Host's surface. ModuleStaleness closes critic finding 2a
// (phase-O merge, BLOCKING): design §3.3's "staleness honesty end-to-end"
// requirement — a bound module whose corpus is not fresh must serve its
// rules FLAGGED, never identically to a fresh module's — was previously
// unimplemented; Bind now threads it through.
type ModuleFederator interface {
	FederatedObligations(ctx context.Context, moduleName, file string, max int) ([]modulehost.FederatedObligation, error)
	ModuleStaleness(ctx context.Context, moduleName string) ([]modulehost.ModuleStaleness, bool, error)
}

// tierRank mirrors internal/core.EvidenceTier's documented ceilings
// (internal/core/model.go) — kept as a local literal (modserve does not
// otherwise depend on internal/core) rather than a new exported rank
// function on a type whose doc comment already states these numbers as
// prose, not an API.
var tierRank = map[string]int{
	"verified":    5,
	"pre-scan":    4,
	"doc-claimed": 2,
	"sme-stated":  2,
	"assumed":     1,
}

// BindingsPath returns the expected .fathom/modules.cue path under a
// service's on-disk root.
func BindingsPath(root string) string {
	return projectdir.Modules.Path(root)
}

// Bind augments repoObl — already computed via semantic.ObligationsFor
// against the service's OWN claims, capped at max — with rules federated
// in from every platform module the service's committed .fathom/modules.cue
// binds (docs/design/platform-modules.md §3.3).
//
// root is the service's on-disk path (ServiceMeta.Path, the O15 mechanism
// cmd/fathom/serviceident.go already relies on for repo↔service identity).
// When host is nil, root is empty, or no binding file exists at
// BindingsPath(root), repoObl is returned COMPLETELY UNCHANGED — this is
// the overwhelmingly common case (no bindings configured) and the exact
// backward-compatible no-op every pre-phase-O caller already gets. A
// malformed binding file also returns repoObl unchanged: this is the hot
// serving path, so a broken .fathom/modules.cue fails OPEN here (`fathom
// doctor` is the loud, non-hot-path place that surfaces it).
//
// max is the SAME overall budget passed to ObligationsFor — module rules
// count INSIDE it, never on top of it (design deliverable 3): repo rules
// keep priority. Deterministic: same store + same binding file + same
// federated module state -> same answer (bindings are processed in the
// file's own list order; FederatedObligations itself sorts by
// (module, service)).
func Bind(ctx context.Context, host ModuleFederator, root, file string, max int, repoObl semantic.Obligations) semantic.Obligations {
	if host == nil || root == "" {
		return repoObl
	}
	if max <= 0 || max > semantic.MaxObligations {
		max = semantic.MaxObligations
	}
	path := BindingsPath(root)
	if _, err := os.Stat(path); err != nil {
		return repoObl // no bindings configured -- silent, the common case
	}
	set, err := bindspec.LoadBindings(path)
	if err != nil || len(set.Bindings) == 0 {
		return repoObl // malformed/empty: doctor's job to complain loudly
	}

	out := repoObl
	remaining := max - len(out.Rules)
	displacedRepo := out.Displaced
	var displacedModule int
	var notes []semantic.ModuleNote

	for _, b := range set.Bindings {
		if !scopeAllows(b.Scopes, file) {
			// The binding's OWN scope restriction excludes this file
			// entirely, independent of whether the module is even
			// mounted — nothing to note, nothing to look up.
			continue
		}
		bindingMax := b.MaxRules
		if bindingMax <= 0 {
			bindingMax = bindspec.DefaultMaxRules
		}
		results, ferr := host.FederatedObligations(ctx, b.Module, file, bindingMax)
		if ferr != nil {
			// Not mounted (or any other lookup failure) — explicit note,
			// never silence (design §3.3 doctrine).
			notes = append(notes, semantic.ModuleNote{Module: b.Module, Note: "bound but not mounted on this daemon"})
			continue
		}
		if len(results) == 0 {
			continue // mounted, nothing matched this file -- not note-worthy
		}
		// Finding 2a: per-(module,service) staleness, fetched once per
		// binding (never per rule) and best-effort — a staleness lookup
		// failure never blocks the obligations payload itself, matching
		// this whole function's fail-open hot-path doctrine; it just
		// means the flag is honestly absent rather than fabricated.
		staleByService := map[string]string{}
		if reports, found, serr := host.ModuleStaleness(ctx, b.Module); serr == nil && found {
			for _, sr := range reports {
				staleByService[sr.Service] = sr.Status
			}
		}
		for _, res := range results {
			if res.Error != "" {
				notes = append(notes, semantic.ModuleNote{Module: b.Module, Note: "module lookup error: " + res.Error})
				continue
			}
			displacedModule += res.Obligations.Displaced
			// design §3.3: "a bound module whose staleness verdict is not
			// fresh serves its rules FLAGGED... unknown-detached is
			// stated, not hidden" — reuse the SAME Status field/rendering
			// repo claims already use (writeRuleBullet already prints
			// r.Status when non-empty) rather than inventing a parallel
			// mechanism. Never clobbers a Status the module's OWN store
			// already set (e.g. a contradicted claim) — only fills it in
			// when otherwise empty.
			moduleStatus := staleByService[res.Service]
			flagStale := moduleStatus != "" && moduleStatus != modulehost.StatusFresh
			for _, r := range res.Obligations.Rules {
				if !tierAllows(b.MinTier, r.Tier) {
					continue // below this binding's configured floor -- not a budget eviction
				}
				r.Module = b.Module
				if flagStale && r.Status == "" {
					r.Status = moduleStatus
				}
				if remaining <= 0 {
					displacedModule++
					continue
				}
				out.Rules = append(out.Rules, r)
				remaining--
			}
		}
	}

	out.DisplacedRepo = displacedRepo
	out.DisplacedModule = displacedModule
	out.Displaced = displacedRepo + displacedModule
	out.ModuleNotes = notes
	return out
}

// scopeAllows reports whether file matches at least one of scopes — an
// empty scopes list means "no consumer-side restriction", i.e. the
// module's own scope globs (already applied inside FederatedObligations'
// call to semantic.ObligationsFor) are the only filter (design §3.2:
// "absent -> the module's own scope globs apply as written").
func scopeAllows(scopes []string, file string) bool {
	if len(scopes) == 0 {
		return true
	}
	return filter.New(scopes).Match(file)
}

// tierAllows reports whether tier clears minTier's floor — an empty
// minTier means no floor (every tier the module compiled may serve). An
// unrecognized tier string never clears a configured floor (fails closed,
// never silently served).
func tierAllows(minTier, tier string) bool {
	if minTier == "" {
		return true
	}
	return tierRank[tier] >= tierRank[minTier]
}

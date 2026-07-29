package modulehost

// federated.go — the module host's federated query layer (docs/plans/
// gke-module-host.md GKE-MH-3/4/6): fans a query out across every mounted
// module, tags every result with its owning module's identity, and never
// merges N modules' claims into one ambiguous unattributed set.
//
// Plan-row amendment note: the plan's initial rows for GKE-MH-3/4/6 pointed
// at {symbol: Graph, package: pkg/platform} — pkg/platform.Graph is a
// CALLGRAPH structure (cross-repo callers / blast radius over
// core.SymbolNode); it carries no claim, citation, tier, or status
// semantics at all, so it cannot be the vehicle for an obligations-shaped
// scope lookup. The federated query layer below instead composes
// internal/semantic.ObligationsFor (the SAME function the existing
// single-store `obligations` MCP tool already uses, doctrine 4's "reuse
// existing tool implementations... consolidate, don't grow") once per
// mounted module's store.Store, which already satisfies every requirement
// the design section actually describes: per-module identity (FederatedObligation.Module),
// per-claim tier+status (semantic.Obligation.Tier/Status, unchanged), and
// never-merged attribution (one slice entry per (module, service) pair,
// never flattened). pkg/platform's Build/BuildFromStore precedent remains
// the correct citation for the DESIGN RATIONALE ("a multi-store query layer
// ... ATTACH-shaped in spirit") — it just isn't the literal implementation
// symbol; the plan rows below have been re-pointed at the functions that
// actually carry this contract.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ModuleInfo is one (module, corpus/service) row — GKE-MH-5's list_modules
// floor requirement (expert-module-v0.md §4.1: "Enumerate — list the
// module's services/corpora and, for each, its current watermark and
// service_meta.indexed_commit").
type ModuleInfo struct {
	Module        string `json:"module"`
	Path          string `json:"path"`
	Service       string `json:"service"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	IndexedAt     int64  `json:"indexed_at,omitempty"`
}

// ListModules returns one row per (module, service) pair across every
// mounted module, sorted by (module, service).
func (h *Host) ListModules(ctx context.Context) ([]ModuleInfo, error) {
	var out []ModuleInfo
	for _, m := range h.snapshot() {
		ids, err := m.Store.ListServiceIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("modulehost: list services for module %q: %w", m.Name, err)
		}
		if len(ids) == 0 {
			// A module with no observations rows (e.g. a claims-only,
			// findings-style corpus) never appears via ListServiceIDs
			// (which scans DISTINCT service_id FROM observations) — still
			// list it as a bare module row so it's not silently invisible.
			out = append(out, ModuleInfo{Module: m.Name, Path: m.Path})
			continue
		}
		for _, svc := range ids {
			info := ModuleInfo{Module: m.Name, Path: m.Path, Service: svc}
			if meta, found, err := m.Store.GetServiceMeta(ctx, svc); err == nil && found {
				info.IndexedCommit = meta.IndexedCommit
				info.IndexedAt = meta.IndexedAt
			}
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Service < out[j].Service
	})
	return out, nil
}

// FederatedObligation is one module's contribution to a federated
// scope-lookup — GKE-MH-3's "never merged into an ambiguous unattributed
// set": every entry names its OWN module and service, so a caller fanning
// out across N mounted modules can always attribute a served rule back to
// the module that produced it, even when two modules happen to compile an
// identical rule string. Obligations already carries per-claim tier and
// non-fresh status on every rule (GKE-MH-4, store-format-v0.md §3 Server
// conformance) via semantic.Obligation — unchanged by federation.
type FederatedObligation struct {
	Module      string               `json:"module"`
	Service     string               `json:"service"`
	Obligations semantic.Obligations `json:"obligations"`
	// Error is set when THIS module's own lookup failed (e.g. a corrupted
	// store) — the federated call still returns every other module's
	// answer rather than aborting entirely (one bad module never blocks
	// the rest).
	Error string `json:"error,omitempty"`
}

// FederatedObligations answers the obligations-shaped scope-lookup query
// (expert-module-v0.md §4.2) across every mounted module, or — when
// moduleName is non-empty — scoped to exactly that one module. A module (or
// module/service pair) with zero matching rules is simply absent from the
// result (semantic.ObligationsFor's own "zero rules -> zero output"
// contract, generalized to N modules).
func (h *Host) FederatedObligations(ctx context.Context, moduleName, file string, max int) ([]FederatedObligation, error) {
	modules := h.snapshot()
	if moduleName != "" {
		var only []*Module
		for _, m := range modules {
			if m.Name == moduleName {
				only = append(only, m)
			}
		}
		if len(only) == 0 {
			return nil, fmt.Errorf("modulehost: module %q is not mounted", moduleName)
		}
		modules = only
	}

	var out []FederatedObligation
	for _, m := range modules {
		ids, err := m.Store.ListServiceIDs(ctx)
		if err != nil {
			out = append(out, FederatedObligation{Module: m.Name, Error: err.Error()})
			continue
		}
		for _, svc := range ids {
			ob, err := semantic.ObligationsFor(ctx, m.Store, svc, file, max)
			if err != nil {
				out = append(out, FederatedObligation{Module: m.Name, Service: svc, Error: err.Error()})
				continue
			}
			if len(ob.Rules) == 0 {
				continue
			}
			out = append(out, FederatedObligation{Module: m.Name, Service: svc, Obligations: ob})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Service < out[j].Service
	})
	return out, nil
}

// Staleness status values (GKE-MH-6). A mounted module is a DETACHED
// corpus by construction (store.MountReadOnly opens a bare file — there is
// no live connector wired to it), so the honest v0 default is
// StatusUnknownDetached: store-format-v0.md §6's watermark comparison
// requires a CURRENT watermark to compare the stored one against, and for a
// module that current value is only derivable when the module's own
// service_meta.path is present and re-checkable on THIS machine (the design
// section's exact words). expert-module-v0.md §4.3 requires the ANSWER be
// derivable without a live corpus connection as the FLOOR — it does not
// forbid a bonus recheck when a local checkout happens to be available, so
// StatusFresh/StatusDrifted are reported opportunistically, never assumed.
const (
	StatusUnknownDetached = "unknown-detached"
	StatusFresh           = "fresh"
	StatusDrifted         = "drifted"
)

// ModuleStaleness is one (module, service) pair's staleness report.
type ModuleStaleness struct {
	Module        string `json:"module"`
	Service       string `json:"service"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	CurrentCommit string `json:"current_commit,omitempty"`
	Status        string `json:"status"`
}

// ModuleStaleness reports staleness for every (module, service) pair of the
// named module. found is false when moduleName is not mounted.
func (h *Host) ModuleStaleness(ctx context.Context, moduleName string) ([]ModuleStaleness, bool, error) {
	m, ok := h.findModule(moduleName)
	if !ok {
		return nil, false, nil
	}
	ids, err := m.Store.ListServiceIDs(ctx)
	if err != nil {
		return nil, true, fmt.Errorf("modulehost: list services for module %q: %w", m.Name, err)
	}
	if len(ids) == 0 {
		return []ModuleStaleness{{Module: m.Name, Status: StatusUnknownDetached}}, true, nil
	}
	out := make([]ModuleStaleness, 0, len(ids))
	for _, svc := range ids {
		st := ModuleStaleness{Module: m.Name, Service: svc, Status: StatusUnknownDetached}
		meta, found, mErr := m.Store.GetServiceMeta(ctx, svc)
		if mErr == nil && found {
			st.IndexedCommit = meta.IndexedCommit
			// Only a git-backed corpus is opportunistically re-checkable
			// this way (store-format-v0.md §6's manifest: watermark would
			// require re-walking the whole corpus, which this floor
			// implementation deliberately does not attempt — the honest-
			// refusal default the mission's pitfalls note calls for when
			// the spec is silent on a case).
			if head := localGitHead(meta.Path); head != "" {
				st.CurrentCommit = head
				switch {
				case st.IndexedCommit == "":
					// No stored watermark to compare against — still
					// "unknown", not silently "fresh".
				case head == st.IndexedCommit:
					st.Status = StatusFresh
				default:
					st.Status = StatusDrifted
				}
			}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out, true, nil
}

// localGitHead returns path's current git HEAD when path exists locally and
// is (or is inside) a git working tree; "" otherwise (missing path, not a
// git repo, or git unavailable) — the "locally present and re-checkable"
// gate the design section requires before ANY live recomputation is
// attempted for a detached module corpus.
func localGitHead(path string) string {
	if path == "" {
		return ""
	}
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		return ""
	}
	var out strings.Builder
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

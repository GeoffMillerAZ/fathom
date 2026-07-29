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
// internal/semantic.ObligationsFor and internal/query.Engine (the SAME
// functions the existing single-store `obligations`/`query_symbols` MCP
// tools already use, doctrine 4's "reuse existing tool implementations...
// consolidate, don't grow") once per mounted module's store.Store, which
// already satisfies every requirement the design section actually
// describes: per-module identity (Module/Service fields), per-claim tier+
// status (semantic.Obligation.Tier/Status, unchanged), and never-merged
// attribution (one slice entry per (module, service) pair, never
// flattened). pkg/platform's Build/BuildFromStore precedent remains the
// correct citation for the DESIGN RATIONALE ("a multi-store query layer
// ... ATTACH-shaped in spirit") — it just isn't the literal implementation
// symbol; the plan rows have been re-pointed at the functions that
// actually carry this contract.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// discoverServiceIDs returns the sorted, deduped UNION of st.ListServiceIDs
// (observations-derived) and st.ClaimServiceIDs (semantic_claims-derived) —
// every module-host discovery call site uses this, not ListServiceIDs
// alone, so a claims-only, findings-style module (docs/design/
// expert-modules.md — a corpus with claims compiled over it but no code
// observations at all) is never silently invisible (critic finding 4). A
// service present in only one of the two source tables still appears
// exactly once.
func discoverServiceIDs(ctx context.Context, st store.Store) ([]string, error) {
	obsIDs, err := st.ListServiceIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListServiceIDs: %w", err)
	}
	claimIDs, err := st.ClaimServiceIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("ClaimServiceIDs: %w", err)
	}
	if len(obsIDs) == 0 && len(claimIDs) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(obsIDs)+len(claimIDs))
	for _, id := range obsIDs {
		seen[id] = struct{}{}
	}
	for _, id := range claimIDs {
		seen[id] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// ModuleInfo is one (module, corpus/service) row — GKE-MH-5's list_modules
// floor requirement (expert-module-v0.md §4.1: "Enumerate — list the
// module's services/corpora and, for each, its current watermark and
// service_meta.indexed_commit").
type ModuleInfo struct {
	Module        string `json:"module"`
	Path          string `json:"path"`
	Service       string `json:"service,omitempty"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	IndexedAt     int64  `json:"indexed_at,omitempty"`
	// Error is set when THIS module's own discovery failed — every other
	// mounted module is still listed rather than aborting the whole call
	// (critic finding 6: ListModules now degrades per-module, mirroring
	// FederatedObligations/FederatedQuery's own degradation contract).
	Error string `json:"error,omitempty"`
}

// ListModules returns one row per (module, service) pair across every
// mounted module, sorted by (module, service). A module whose own
// discovery fails contributes a single Error-carrying row instead of
// aborting the entire call.
func (h *Host) ListModules(ctx context.Context) ([]ModuleInfo, error) {
	var out []ModuleInfo
	for _, m := range h.snapshot() {
		ids, err := discoverServiceIDs(ctx, m.Store)
		if err != nil {
			out = append(out, ModuleInfo{Module: m.Name, Path: m.Path, Error: err.Error()})
			continue
		}
		if len(ids) == 0 {
			// A genuinely empty module (mounted, conforming, but nothing
			// ingested into it yet) — still list it so it's not silently
			// invisible.
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

// modulesInScope resolves the modules a federated call should fan out
// across: every mounted module, or — when moduleName is non-empty —
// exactly that one (an error when it isn't mounted at all).
func (h *Host) modulesInScope(moduleName string) ([]*Module, error) {
	modules := h.snapshot()
	if moduleName == "" {
		return modules, nil
	}
	for _, m := range modules {
		if m.Name == moduleName {
			return []*Module{m}, nil
		}
	}
	return nil, fmt.Errorf("modulehost: module %q is not mounted", moduleName)
}

// FederatedObligations answers the obligations-shaped scope-lookup query
// (expert-module-v0.md §4.2) across every mounted module, or — when
// moduleName is non-empty — scoped to exactly that one module. A module (or
// module/service pair) with zero matching rules is simply absent from the
// result (semantic.ObligationsFor's own "zero rules -> zero output"
// contract, generalized to N modules).
func (h *Host) FederatedObligations(ctx context.Context, moduleName, file string, max int) ([]FederatedObligation, error) {
	modules, err := h.modulesInScope(moduleName)
	if err != nil {
		return nil, err
	}

	var out []FederatedObligation
	for _, m := range modules {
		ids, err := discoverServiceIDs(ctx, m.Store)
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

// QueryClaim is module_query's claim-match wire shape — deliberately NOT
// semantic.Obligation (module_obligations' own type, untouched by this
// row): expertchat's cite tokens are `Module#ClaimID` and its refusal/
// feedback path needs durable claim identity, which semantic.Obligation
// never carried (it identifies a claim only by its Rule text). ClaimID is
// the same content-addressed core.SemanticClaim.ClaimID every other claim
// surface (claim_events, claim refute, ClaimsForService) keys on; Anchor is
// the literal cited text from claim_links (store-format-v0.md §5.4 —
// "" for a legacy hash-only link that predates AnchorText, never a guess).
type QueryClaim struct {
	ClaimID string `json:"claim_id"`
	Text    string `json:"text"`
	Src     string `json:"src,omitempty"`
	Anchor  string `json:"anchor,omitempty"`
	Tier    string `json:"tier"`
	// Status is "" for a fresh claim (never the literal "fresh") — the same
	// zero-output-on-clean convention semantic.Obligation.Status already
	// uses, preserved here rather than reinvented.
	Status string `json:"status,omitempty"`
	Kind   string `json:"kind,omitempty"`
}

// FederatedQueryEntry is one module's contribution to a federated free-text
// query (module_query — critic finding 1: the surface's only pre-existing
// scope-lookup tool, module_obligations, required a scope-glob `file` arg,
// which cannot answer a free-text question at all). Nodes are ranked by the
// SAME concern-scoped engine query_symbols already wraps (internal/query.
// Engine); Claims are served claims whose rule/subject text matched,
// carrying durable claim identity (QueryClaim, distinct from
// module_obligations' semantic.Obligation — see QueryClaim's doc comment).
// Both Nodes and Claims are optional and independent — a module may answer
// via symbols, via claims, via both, or (if it errors) via neither.
// Watermark is this (module, service) pair's compile-time watermark
// (ServiceMeta.IndexedCommit) — present on every entry regardless of
// whether it answered, so a caller can attach freshness context to
// whatever it DID get back without a second round-trip to module_staleness.
type FederatedQueryEntry struct {
	Module    string               `json:"module"`
	Service   string               `json:"service"`
	Watermark string               `json:"watermark"`
	Nodes     []core.SymbolNode    `json:"nodes,omitempty"`
	Staleness core.StalenessReport `json:"staleness"`
	Claims    []QueryClaim         `json:"claims,omitempty"`
	// Error is set when THIS (module, service) pair's own lookup failed —
	// every other module/service pair's answer is still returned (mirrors
	// FederatedObligations' per-module degradation).
	Error string `json:"error,omitempty"`
}

// FederatedQuery answers a free-text query (module_query, critic finding 1)
// across every mounted module, or — when moduleName is non-empty — scoped
// to exactly that one. For each (module, service) pair it runs BOTH the
// concern-scoped symbol engine (internal/query.New(m.Store).Query) and a
// cheap substring match over served claims (matchClaimsByText); a pair
// that answers via neither is omitted from the result (mirrors
// FederatedObligations' "zero rules -> zero output" contract). text is
// required — an empty query has no meaningful answer to fan out.
func (h *Host) FederatedQuery(ctx context.Context, moduleName, text string, max int) ([]FederatedQueryEntry, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("modulehost: query text is required")
	}
	modules, err := h.modulesInScope(moduleName)
	if err != nil {
		return nil, err
	}
	keywords := strings.Fields(text)

	var out []FederatedQueryEntry
	for _, m := range modules {
		ids, err := discoverServiceIDs(ctx, m.Store)
		if err != nil {
			out = append(out, FederatedQueryEntry{Module: m.Name, Error: err.Error()})
			continue
		}
		eng := query.New(m.Store)
		for _, svc := range ids {
			entry := FederatedQueryEntry{Module: m.Name, Service: svc}
			if meta, found, mErr := m.Store.GetServiceMeta(ctx, svc); mErr == nil && found {
				entry.Watermark = meta.IndexedCommit
			}

			result, qErr := eng.Query(ctx, core.Query{ServiceID: svc, ConcernID: text, Keywords: keywords, TopN: max})
			if qErr != nil {
				entry.Error = qErr.Error()
			} else {
				entry.Nodes = result.Nodes
				entry.Staleness = result.Staleness
			}

			claims, cErr := matchClaimsByText(ctx, m.Store, svc, text, max)
			if cErr != nil {
				if entry.Error == "" {
					entry.Error = cErr.Error()
				}
			} else {
				entry.Claims = claims
			}

			if len(entry.Nodes) == 0 && len(entry.Claims) == 0 && entry.Error == "" {
				continue // nothing this (module, service) can answer — omit, don't pad
			}
			out = append(out, entry)
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

// matchClaimsByText returns served claims (QueryClaim shape — durable
// claim_id + anchor text, see QueryClaim's doc comment) whose Rule or
// Subject contains text (case-insensitive substring — a deliberately
// simple, cheap match per the critic's own framing: module_query's floor
// is "don't refuse a free-text query outright," not a ranked full-text
// search engine), capped at max and sorted for determinism. Built directly
// from ClaimsForService + ClaimLinks (not routed through
// semantic.ObligationsFor, which has no scope-glob to match a free-text
// query against and would drop ClaimID on the way out) — this path has
// every field the wire contract needs in hand already.
func matchClaimsByText(ctx context.Context, st store.Store, serviceID, text string, max int) ([]QueryClaim, error) {
	if max <= 0 || max > semantic.MaxObligations {
		max = semantic.MaxObligations
	}
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(text)
	var served []core.SemanticClaim
	var ids []string
	for _, cl := range claims {
		if !cl.Status.Served() {
			continue
		}
		if !strings.Contains(strings.ToLower(cl.Rule), needle) && !strings.Contains(strings.ToLower(cl.Subject), needle) {
			continue
		}
		served = append(served, cl)
		ids = append(ids, cl.ClaimID)
	}
	if len(served) == 0 {
		return nil, nil
	}
	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return nil, err
	}
	primaryCite := map[string]string{}
	primaryAnchor := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites {
			if _, ok := primaryCite[l.ClaimID]; !ok {
				primaryCite[l.ClaimID] = l.TargetRef
				primaryAnchor[l.ClaimID] = l.AnchorText
			}
		}
	}
	sort.Slice(served, func(i, j int) bool {
		if served[i].Rule != served[j].Rule {
			return served[i].Rule < served[j].Rule
		}
		return served[i].ClaimID < served[j].ClaimID
	})
	if len(served) > max {
		served = served[:max]
	}
	out := make([]QueryClaim, 0, len(served))
	for _, cl := range served {
		qc := QueryClaim{
			ClaimID: cl.ClaimID,
			Text:    cl.Rule,
			Src:     primaryCite[cl.ClaimID],
			Anchor:  primaryAnchor[cl.ClaimID],
			Tier:    string(cl.Tier),
		}
		if cl.Kind != semantic.KindObligation {
			qc.Kind = cl.Kind
		}
		if cl.Status != core.ClaimFresh {
			qc.Status = string(cl.Status)
		}
		out = append(out, qc)
	}
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
	ids, err := discoverServiceIDs(ctx, m.Store)
	if err != nil {
		return nil, true, fmt.Errorf("modulehost: discover services for module %q: %w", m.Name, err)
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
			st.CurrentCommit, st.Status = recheckWatermark(ctx, meta.Path, meta.IndexedCommit)
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out, true, nil
}

// recheckWatermark opportunistically recomputes the CURRENT watermark for a
// module's corpus at path and compares it against indexedCommit, returning
// (current, status). Critic finding 3 — the load-bearing rule this
// function exists to enforce: a manifest-prefixed indexedCommit (a corpus
// compiled as non-git) MUST NEVER be compared against a live git HEAD. A
// path that happens to be nested inside SOME enclosing git work tree (a
// docs folder inside a checkout; a module expanded into a repo-internal
// temp dir) would otherwise resolve to that ENCLOSING repo's HEAD — which
// can never equal a manifest: watermark — producing a permanent, false
// "drifted" verdict for a module that never drifted at all. So:
//   - indexedCommit has the manifest: prefix -> recompute via the SAME
//     content-addressed algorithm (connector.ManifestWatermark), never
//     git — "" (unknown-detached, no recompute attempted) when path is
//     absent or the recompute itself fails.
//   - otherwise (a bare git SHA, or "") -> the existing git-HEAD recheck,
//     itself gated on path being locally present and a git work tree.
func recheckWatermark(ctx context.Context, path, indexedCommit string) (current, status string) {
	if strings.HasPrefix(indexedCommit, connector.ManifestWatermarkPrefix) {
		if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
			return "", StatusUnknownDetached
		}
		got, err := connector.ManifestWatermark(ctx, connector.NewLocalConnector(), core.ServiceRef{Path: path})
		if err != nil {
			return "", StatusUnknownDetached
		}
		if got == indexedCommit {
			return got, StatusFresh
		}
		return got, StatusDrifted
	}

	// Only a git-backed corpus is opportunistically re-checkable this way.
	head := localGitHead(path)
	if head == "" {
		return "", StatusUnknownDetached
	}
	if indexedCommit == "" {
		// No stored watermark to compare against — still "unknown", not
		// silently "fresh".
		return head, StatusUnknownDetached
	}
	if head == indexedCommit {
		return head, StatusFresh
	}
	return head, StatusDrifted
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

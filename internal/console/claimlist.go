package console

// claimlist.go — the rows behind every claim count (vision 35 §5 "every
// number drills down: a count links to its rows"): the governed claims
// matching a layer, a project and a status, a page at a time. The strata,
// the coverage matrix, the layer page and the packs page count claims
// through readKnowledge; their counts link here, and this list folds the
// same read, so a count and its rows cannot disagree. `fathom claim list
// --json` prints exactly this; /claims renders it.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Claim status filters: a lifecycle status, or "current" (what serving can
// still use: fresh, stale, contradicted) or "all".
const (
	ClaimStatusCurrent = "current"
	ClaimStatusAll     = "all"
)

// ClaimStatusFilters is every status filter, in lifecycle order.
var ClaimStatusFilters = []string{ClaimStatusCurrent, string(core.ClaimFresh), string(core.ClaimStale), string(core.ClaimContradicted), string(core.ClaimSuperseded), string(core.ClaimRefuted), ClaimStatusAll}

// ClaimsPerPage pages the list (design §2: tables over 50 rows are paged).
const ClaimsPerPage = 50

// ClaimFilter selects claims. Empty Layer or Service is every one; an empty
// Status is "current".
type ClaimFilter struct {
	Layer   string `json:"layer,omitempty"`
	Service string `json:"service,omitempty"`
	Status  string `json:"status"`
}

// ClaimList is one page of the claims matching a filter.
type ClaimList struct {
	Filter ClaimFilter `json:"filter"`
	// Total is how many claims match — a lower bound naming the projects
	// whose claims could not be read.
	Total   Value[int] `json:"total"`
	Page    int        `json:"page"`
	Pages   int        `json:"pages"`
	PerPage int        `json:"per_page"`
	Claims  []ClaimRow `json:"claims"`
	// Layers and Services are every layer and project the filter can name.
	Layers   []string `json:"layers"`
	Services []string `json:"services"`
}

// ClaimRow is one claim in the list.
type ClaimRow struct {
	ClaimID   string `json:"claim_id"`
	Service   string `json:"service"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
	Tier      string `json:"tier"`
	Statement string `json:"statement"`
	// Cites is the claim's first line citation (path#Lstart-Lend), "" when
	// it carries none or its links could not be read.
	Cites      string `json:"cites,omitempty"`
	CompiledAt string `json:"compiled_at,omitempty"`
}

// ValidClaimStatus reports whether s is a status filter ("" included).
func ValidClaimStatus(s string) bool {
	if s == "" {
		return true
	}
	for _, f := range ClaimStatusFilters {
		if f == s {
			return true
		}
	}
	return false
}

// claimMatches reports whether a claim's status passes the filter.
func claimMatches(status core.ClaimStatus, filter string) bool {
	switch filter {
	case "", ClaimStatusCurrent:
		return status.Served()
	case ClaimStatusAll:
		return true
	}
	return string(status) == filter
}

// claimStatusOrder sorts current claims first, in lifecycle order.
var claimStatusOrder = map[core.ClaimStatus]int{
	core.ClaimFresh: 0, core.ClaimStale: 1, core.ClaimContradicted: 2, core.ClaimSuperseded: 3, core.ClaimRefuted: 4,
}

// ListClaims pages the claims matching f, ordered by project, layer,
// status (current first), statement and id. page < 1 is the first page; a
// page past the last is the last.
func ListClaims(ctx context.Context, src KnowledgeSource, f ClaimFilter, page int) (ClaimList, error) {
	if f.Status == "" {
		f.Status = ClaimStatusCurrent
	}
	if !ValidClaimStatus(f.Status) {
		return ClaimList{}, fmt.Errorf("unknown status %q (want one of %s)", f.Status, strings.Join(ClaimStatusFilters, ", "))
	}
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return ClaimList{}, err
	}
	out := ClaimList{Filter: f, PerPage: ClaimsPerPage, Claims: []ClaimRow{}, Layers: k.layerNames(), Services: []string{}}
	type hit struct {
		svc string
		cl  core.SemanticClaim
	}
	var hits []hit
	var unreadable []string
	for _, s := range k.Services {
		out.Services = append(out.Services, s.Service)
		if f.Service != "" && s.Service != f.Service {
			continue
		}
		if s.Err != "" {
			unreadable = append(unreadable, s.Service)
			continue
		}
		for _, cl := range s.Claims {
			if (f.Layer == "" || cl.Layer == f.Layer) && claimMatches(cl.Status, f.Status) {
				hits = append(hits, hit{s.Service, cl})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.svc != b.svc {
			return a.svc < b.svc
		}
		if a.cl.Layer != b.cl.Layer {
			return a.cl.Layer < b.cl.Layer
		}
		if oa, ob := claimStatusOrder[a.cl.Status], claimStatusOrder[b.cl.Status]; oa != ob {
			return oa < ob
		}
		if a.cl.Rule != b.cl.Rule {
			return a.cl.Rule < b.cl.Rule
		}
		return a.cl.ClaimID < b.cl.ClaimID
	})
	denom := "claims matching " + f.describe()
	if len(unreadable) > 0 {
		out.Total = Partial(len(hits), "lower bound; claims unreadable for: "+strings.Join(unreadable, ", ")).WithSource(storeSource).WithDenominator(denom)
	} else {
		out.Total = Measured(len(hits), storeSource).WithDenominator(denom)
	}
	out.Pages = max(1, (len(hits)+ClaimsPerPage-1)/ClaimsPerPage)
	out.Page = min(max(page, 1), out.Pages)
	lo := (out.Page - 1) * ClaimsPerPage
	hi := min(lo+ClaimsPerPage, len(hits))
	// Citations for this page's rows only, one read per project.
	ids := map[string][]string{}
	for _, h := range hits[lo:hi] {
		ids[h.svc] = append(ids[h.svc], h.cl.ClaimID)
	}
	cites := map[string]string{}
	for svc, list := range ids {
		links, lerr := src.Store.ClaimLinks(ctx, svc, list)
		if lerr != nil {
			continue // a row without its citation still names its claim
		}
		for _, l := range links {
			if l.Rel != core.RelCites {
				continue
			}
			key := svc + "\x00" + l.ClaimID
			if _, _, _, ok := parseCiteRef(l.TargetRef); ok && cites[key] == "" {
				cites[key] = l.TargetRef
			}
		}
	}
	for _, h := range hits[lo:hi] {
		out.Claims = append(out.Claims, ClaimRow{
			ClaimID: h.cl.ClaimID, Service: h.svc, Layer: h.cl.Layer, Status: string(h.cl.Status), Tier: string(h.cl.Tier),
			Statement: h.cl.Rule, Cites: cites[h.svc+"\x00"+h.cl.ClaimID], CompiledAt: rfc3339(h.cl.CompiledAt),
		})
	}
	return out, nil
}

// describe renders the filter in words.
func (f ClaimFilter) describe() string {
	status := f.Status
	switch status {
	case "", ClaimStatusCurrent:
		status = "current (fresh, stale, contradicted)"
	case ClaimStatusAll:
		status = "any status"
	}
	parts := []string{status}
	if f.Layer != "" {
		parts = append(parts, "layer "+f.Layer)
	}
	if f.Service != "" {
		parts = append(parts, "project "+f.Service)
	} else {
		parts = append(parts, "every project")
	}
	return strings.Join(parts, " · ")
}

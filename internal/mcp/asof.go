package mcp

// asof.go — GL-17 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §7 "Validity windows everywhere a fact
// ends"): `as_of` answers "what did we know on date D". SU-19 gave
// `obligations` the argument; this file is the one place every verb that
// reads a dated fact resolves it, so the verbs cannot disagree about what
// an instant means. An `as_of` is an RFC3339 timestamp or an indexed commit
// sha (semantic.ResolveAsOf); it resolves to one instant, and a claim
// answers for it when its validity window [valid_from, valid_to) contains
// it, whatever its status is now (core.SemanticClaim.ValidAt).
//
// Verbs that take it: `obligations` (SU-19), `staleness_report` (its
// claims-by-layer and doc-link sections), `review_diff` (the obligations
// and certifications it reads) and `findings_near`. `memory_search` is not
// one: anchored memories are being removed (SR-13), so the row's original
// fourth verb went with them. A section that reads the index as it stands
// (sources, commit drift, the typed-edge pass, the code-derived review
// dimensions) says so beside the answer rather than passing off a current
// fact as a dated one (I38).

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// asOfDocLinkCap bounds the doc links staleness_report lists as of a date;
// the count beside the list stays the true total (an honest summary is
// never truncated, only its listed rows).
const asOfDocLinkCap = 200

// claimsListBackend is the optional capability every claim-reading as_of
// path needs: the service's claims with their validity windows.
// pkg/client.Client implements it; a minimal fake may not, and each verb
// then says the answer is not measurable rather than serving the live one.
type claimsListBackend interface {
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
}

// docLinksAsOfBackend is the optional capability behind staleness_report's
// doc links as of a date (GL-16, store.SQLiteStore.DocLinksAsOf).
// pkg/client.Client implements it, returning docindex.ErrDocIndexUnavailable
// when the store keeps no doc index — which the caller reports as absent,
// never as "no links".
type docLinksAsOfBackend interface {
	DocLinksAsOf(ctx context.Context, service string, at time.Time) ([]store.DocLinkView, error)
}

// asOfFilter is one resolved `as_of` instant. The nil filter is "now": every
// method is safe on it and changes nothing, so a verb threads one value
// through whether or not the caller asked for a date.
type asOfFilter struct {
	at time.Time
}

// newAsOfFilter resolves raw for service on behalf of tool. "" is the nil
// filter (the live answer). An unparseable value, or a commit sha the index
// does not know, is a params error naming tool — never a silent fall back to
// now.
func newAsOfFilter(ctx context.Context, backend QueryInterface, service, raw, tool string) (*asOfFilter, *rpcError) {
	if raw == "" {
		return nil, nil
	}
	var commits []CommitInfo
	if cl, err := backend.ListCommits(ctx, service); err == nil {
		commits = cl
	}
	at, err := semantic.ResolveAsOf(raw, commits)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeParams, Message: tool + ": " + err.Error()}
	}
	return &asOfFilter{at: at}, nil
}

// active reports whether the caller asked for a date.
func (f *asOfFilter) active() bool { return f != nil }

// instant is the resolved time (zero for the nil filter).
func (f *asOfFilter) instant() time.Time {
	if f == nil {
		return time.Time{}
	}
	return f.at
}

// label is the RFC3339 instant echoed back in every as_of answer ("" for
// the nil filter, so an omitted field stays byte-identical to the live
// payload).
func (f *asOfFilter) label() string {
	if f == nil {
		return ""
	}
	return f.at.UTC().Format(time.RFC3339)
}

// claims keeps the claims whose validity window contained the instant, in
// their original order; the nil filter returns them all. It looks at the
// window alone — a claim superseded since still answers for a date before it
// was superseded, which is the whole point.
func (f *asOfFilter) claims(in []core.SemanticClaim) []core.SemanticClaim {
	if f == nil {
		return in
	}
	out := make([]core.SemanticClaim, 0, len(in))
	for _, c := range in {
		if c.ValidAt(f.at) {
			out = append(out, c)
		}
	}
	return out
}

// layerTally counts, per layer, the claims valid at the instant: the
// as-of counterpart of ClaimsSummary's status counts. A status count would
// misreport a since-superseded claim as superseded on a date it was in
// force, so the dated answer counts what was in force (`valid`) and nothing
// else.
func (f *asOfFilter) layerTally(in []core.SemanticClaim) map[string]any {
	perLayer := map[string]int{}
	for _, c := range f.claims(in) {
		perLayer[c.Layer]++
	}
	layers := make([]string, 0, len(perLayer))
	for l := range perLayer {
		layers = append(layers, l)
	}
	sort.Strings(layers)
	out := make(map[string]any, len(layers))
	for _, l := range layers {
		out[l] = map[string]any{"valid": perLayer[l]}
	}
	return out
}

// stalenessAsOf adds staleness_report's dated sections to resp: the claims
// valid by layer and the doc links that held. Each is named absent when the
// backend cannot read it. The other sections of staleness_report (sources,
// commit drift, refresh, typed edges) read the index as it stands, and
// as_of_scope says so.
func stalenessAsOf(ctx context.Context, backend QueryInterface, service string, f *asOfFilter, resp map[string]any) {
	resp["as_of"] = f.label()
	resp["as_of_scope"] = "layers and doc_links_as_of are answered as of " + f.label() +
		"; sources, commits, refresh and typed_edges are the index as it stands now, and doc_link_drift (a comparison with now) is left out"
	if cb, ok := backend.(claimsListBackend); ok {
		if claims, err := cb.ClaimsForService(ctx, service, ""); err == nil {
			if tally := f.layerTally(claims); len(tally) > 0 {
				resp["layers"] = tally
			}
		} else {
			resp["layers_unavailable"] = "claims could not be read: " + err.Error()
		}
	} else {
		resp["layers_unavailable"] = "backend does not serve claims, so a dated per-layer count is not measurable"
	}
	if lb, ok := backend.(docLinksAsOfBackend); ok {
		if links, err := lb.DocLinksAsOf(ctx, service, f.at); err == nil {
			rows, elision := renderDocLinksAsOf(links)
			resp["doc_links_as_of"] = rows
			resp["doc_links_as_of_count"] = len(links)
			if elision != "" {
				resp["doc_links_as_of_elision"] = elision
			}
		} else {
			resp["doc_links_unavailable"] = "doc links could not be read: " + err.Error()
		}
	} else {
		resp["doc_links_unavailable"] = "backend keeps no doc-link windows, so doc links as of a date are not measurable"
	}
}

// renderDocLinksAsOf renders the links that held at the instant, capped at
// asOfDocLinkCap rows; elision names the cap when it cut anything (the
// caller's count stays the true total).
func renderDocLinksAsOf(links []store.DocLinkView) (rows []any, elision string) {
	if len(links) > asOfDocLinkCap {
		elision = "listed the first " + strconv.Itoa(asOfDocLinkCap) + " of " + strconv.Itoa(len(links)) + " links; doc_links_as_of_count is the full total"
		links = links[:asOfDocLinkCap]
	}
	out := make([]any, len(links))
	for i, l := range links {
		row := map[string]any{
			"path":        l.Path,
			"heading":     l.HeadingPath,
			"target_kind": l.TargetKind,
			"target":      l.Target,
			"line":        l.Line,
			"valid_from":  time.Unix(0, l.ValidFrom).UTC().Format(time.RFC3339),
		}
		if l.ValidTo != 0 {
			row["valid_to"] = time.Unix(0, l.ValidTo).UTC().Format(time.RFC3339)
		}
		out[i] = row
	}
	return out, elision
}

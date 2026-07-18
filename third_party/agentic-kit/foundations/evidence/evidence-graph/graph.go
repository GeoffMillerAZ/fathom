package evidencegraph

import (
	"sort"
	"time"
)

// Graph is the in-memory evidence graph. Spans are nodes; Edges are typed
// directed links. All mutation and query methods are safe to call on the zero
// value after New() has been used to initialise the graph.
//
// Graph is NOT safe for concurrent use. Callers that share a Graph across
// goroutines must serialise access externally.
type Graph struct {
	spans map[string]Span // keyed by Span.ID
	edges []Edge          // all edges; filtered on query
}

// New returns an empty, ready-to-use Graph.
func New() *Graph {
	return &Graph{
		spans: make(map[string]Span),
	}
}

// Add inserts or replaces a Span in the graph. A Span with the same ID as an
// existing one overwrites it.
func (g *Graph) Add(s Span) {
	g.spans[s.ID] = s
}

// Link records a directed edge from → to with the given relationship label.
// Neither span is required to exist at the time Link is called; the edge is
// stored regardless and simply produces no results in Neighbors when one end
// is missing or expired.
func (g *Graph) Link(from, to, rel string) {
	g.edges = append(g.edges, Edge{From: from, To: to, Rel: rel})
}

// Sweep evicts every span whose ExpiresAt is non-zero and strictly before now.
// Expired spans are removed from the graph permanently; any edges that
// reference only expired spans become inert (they are not removed from
// storage but will produce no query results).
//
// Pass time.Now() for wall-clock eviction, or an injected value in tests for
// determinism.
func (g *Graph) Sweep(now time.Time) {
	for id, s := range g.spans {
		if isExpired(s, now) {
			delete(g.spans, id)
		}
	}
}

// Neighbors returns all non-expired spans that are directly linked to the span
// identified by id (either as the From or To end of any edge). The result is
// sorted by Span.ID for determinism.
//
// now is used to determine whether each candidate span is still live. Pass
// time.Now() for production use or an injected value in tests.
func (g *Graph) Neighbors(id string, now time.Time) []Span {
	seen := make(map[string]struct{})
	for _, e := range g.edges {
		var candidateID string
		switch {
		case e.From == id:
			candidateID = e.To
		case e.To == id:
			candidateID = e.From
		default:
			continue
		}
		seen[candidateID] = struct{}{}
	}

	result := make([]Span, 0, len(seen))
	for cid := range seen {
		s, ok := g.spans[cid]
		if !ok || isExpired(s, now) {
			continue
		}
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Contradictions returns all pairs of non-expired spans that share the same
// (EntityID, Field) but carry different Value strings. Each pair is reported
// exactly once (A.ID < B.ID ordering). Results are sorted by (EntityID, Field,
// A.ID) for determinism.
//
// now is used to exclude expired spans from consideration. Pass time.Now() for
// production use or an injected value in tests.
func (g *Graph) Contradictions(now time.Time) []Contradiction {
	// Group live spans by (EntityID, Field).
	type key struct{ EntityID, Field string }
	buckets := make(map[key][]Span)
	for _, s := range g.spans {
		if isExpired(s, now) {
			continue
		}
		k := key{s.EntityID, s.Field}
		buckets[k] = append(buckets[k], s)
	}

	var result []Contradiction
	for k, spans := range buckets {
		if len(spans) < 2 {
			continue
		}
		// Sort spans within the bucket by ID so pairs are deterministic.
		sort.Slice(spans, func(i, j int) bool { return spans[i].ID < spans[j].ID })
		for i := 0; i < len(spans); i++ {
			for j := i + 1; j < len(spans); j++ {
				if spans[i].Value != spans[j].Value {
					result = append(result, Contradiction{
						EntityID: k.EntityID,
						Field:    k.Field,
						A:        spans[i],
						B:        spans[j],
					})
				}
			}
		}
	}

	// Sort by (EntityID, Field, A.ID) for a stable, deterministic output order.
	sort.Slice(result, func(i, j int) bool {
		if result[i].EntityID != result[j].EntityID {
			return result[i].EntityID < result[j].EntityID
		}
		if result[i].Field != result[j].Field {
			return result[i].Field < result[j].Field
		}
		return result[i].A.ID < result[j].A.ID
	})
	return result
}

// isExpired reports whether span s should be treated as stale at time now.
// A span with a zero ExpiresAt never expires.
func isExpired(s Span, now time.Time) bool {
	return !s.ExpiresAt.IsZero() && s.ExpiresAt.Before(now)
}

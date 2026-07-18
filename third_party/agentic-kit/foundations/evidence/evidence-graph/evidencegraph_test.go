package evidencegraph_test

import (
	"testing"
	"time"

	eg "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-graph"
)

// epoch is a fixed reference instant used throughout the test suite so that
// all time comparisons are deterministic and independent of wall-clock time.
var epoch = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// future and past are helpers relative to epoch.
func future(d time.Duration) time.Time { return epoch.Add(d) }
func past(d time.Duration) time.Time   { return epoch.Add(-d) }

// makeSpan is a small constructor so test cases stay concise.
func makeSpan(id, entityID, field, value string, confidence float64, expiresAt time.Time) eg.Span {
	return eg.Span{
		ID:         id,
		EntityID:   entityID,
		Field:      field,
		Value:      value,
		Confidence: confidence,
		Source:     "test-source",
		ExpiresAt:  expiresAt,
	}
}

// TestAddAndLink verifies that adding spans and linking them produces a
// queryable graph where Neighbors returns the expected linked spans.
func TestAddAndLink(t *testing.T) {
	g := eg.New()

	s1 := makeSpan("s1", "repo-a", "language", "Go", 0.9, future(24*time.Hour))
	s2 := makeSpan("s2", "repo-a", "owner", "platform-team", 0.8, future(24*time.Hour))
	s3 := makeSpan("s3", "repo-b", "language", "Python", 0.7, future(24*time.Hour))

	g.Add(s1)
	g.Add(s2)
	g.Add(s3)

	// Link s1 → s2 (same entity, different fields)
	g.Link("s1", "s2", "corroborates")
	// Link s1 → s3 (cross-entity edge)
	g.Link("s1", "s3", "related")

	neighbors := g.Neighbors("s1", epoch)
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors of s1, got %d: %v", len(neighbors), neighbors)
	}
	if neighbors[0].ID != "s2" || neighbors[1].ID != "s3" {
		t.Errorf("unexpected neighbor IDs: %v", []string{neighbors[0].ID, neighbors[1].ID})
	}

	// s2 should report s1 as a neighbor (edges are bidirectional in Neighbors).
	n2 := g.Neighbors("s2", epoch)
	if len(n2) != 1 || n2[0].ID != "s1" {
		t.Errorf("expected [s1] as neighbor of s2, got %v", n2)
	}

	// s3 has no outbound edge to anything but is reachable from s1.
	n3 := g.Neighbors("s3", epoch)
	if len(n3) != 1 || n3[0].ID != "s1" {
		t.Errorf("expected [s1] as neighbor of s3, got %v", n3)
	}
}

// TestNeighborsExcludesExpired verifies that an expired span does not appear
// in Neighbors even when an edge to it exists.
func TestNeighborsExcludesExpired(t *testing.T) {
	g := eg.New()

	live := makeSpan("live", "ent", "f", "v1", 0.9, future(time.Hour))
	expired := makeSpan("exp", "ent", "g", "v2", 0.8, past(time.Hour)) // already expired

	g.Add(live)
	g.Add(expired)
	g.Link("live", "exp", "related")

	neighbors := g.Neighbors("live", epoch)
	if len(neighbors) != 0 {
		t.Errorf("expired span should not appear in Neighbors; got %v", neighbors)
	}
}

// TestSweepEvictsExpiredSpans verifies that Sweep removes expired spans from
// the graph and that subsequent queries no longer return them.
func TestSweepEvictsExpiredSpans(t *testing.T) {
	g := eg.New()

	live := makeSpan("live", "ent", "field", "alive", 0.9, future(time.Hour))
	stale := makeSpan("stale", "ent", "other", "gone", 0.5, past(time.Minute))

	g.Add(live)
	g.Add(stale)

	// Before sweep: stale is in the graph but excluded from query results.
	contribs := g.Contradictions(epoch)
	if len(contribs) != 0 {
		t.Errorf("expected no contradictions before sweep, got %v", contribs)
	}

	g.Sweep(epoch)

	// After sweep: Neighbors still works for the live span.
	g.Add(makeSpan("peer", "ent2", "x", "y", 1.0, future(time.Hour)))
	g.Link("live", "peer", "related")

	neighbors := g.Neighbors("live", epoch)
	if len(neighbors) != 1 || neighbors[0].ID != "peer" {
		t.Errorf("unexpected neighbors after sweep: %v", neighbors)
	}

	// The stale span must be gone from internal state — add a fresh span with
	// the same ID and verify it is the new value (overwrites should work).
	replacement := makeSpan("stale", "ent", "other", "fresh", 0.9, future(time.Hour))
	g.Add(replacement)
	g.Link("live", "stale", "supersedes")

	n2 := g.Neighbors("live", epoch)
	found := false
	for _, s := range n2 {
		if s.ID == "stale" {
			found = true
			if s.Value != "fresh" {
				t.Errorf("expected replacement span value 'fresh', got %q", s.Value)
			}
		}
	}
	if !found {
		t.Error("replacement span 'stale' not found in neighbors after re-add")
	}
}

// TestSweepDoesNotEvictNonExpired confirms that live spans survive Sweep.
func TestSweepDoesNotEvictNonExpired(t *testing.T) {
	g := eg.New()

	neverExpires := makeSpan("nv", "e", "f", "v", 0.5, time.Time{}) // zero = no expiry
	liveFuture := makeSpan("lf", "e", "g", "w", 0.5, future(time.Hour))

	g.Add(neverExpires)
	g.Add(liveFuture)
	g.Sweep(epoch)

	g.Link("nv", "lf", "corroborates")
	neighbors := g.Neighbors("nv", epoch)
	if len(neighbors) != 1 || neighbors[0].ID != "lf" {
		t.Errorf("live span should survive Sweep; neighbors: %v", neighbors)
	}
}

// TestContradictionsDetected verifies that two spans with the same EntityID
// and Field but different Values are reported as a Contradiction.
func TestContradictionsDetected(t *testing.T) {
	g := eg.New()

	a := makeSpan("s-a", "repo-x", "owner", "team-alpha", 0.9, future(time.Hour))
	b := makeSpan("s-b", "repo-x", "owner", "team-beta", 0.7, future(time.Hour))

	g.Add(a)
	g.Add(b)

	cs := g.Contradictions(epoch)
	if len(cs) != 1 {
		t.Fatalf("expected 1 contradiction, got %d: %v", len(cs), cs)
	}
	c := cs[0]
	if c.EntityID != "repo-x" {
		t.Errorf("wrong EntityID in contradiction: %q", c.EntityID)
	}
	if c.Field != "owner" {
		t.Errorf("wrong Field in contradiction: %q", c.Field)
	}
	// A and B must carry the two distinct values (order is stable A.ID < B.ID).
	values := map[string]bool{c.A.Value: true, c.B.Value: true}
	if !values["team-alpha"] || !values["team-beta"] {
		t.Errorf("unexpected values in contradiction: A=%q B=%q", c.A.Value, c.B.Value)
	}
}

// TestAgreingSpansNotReportedAsContradictions verifies that two spans with the
// same EntityID, Field, AND Value do not trigger a Contradiction.
func TestAgreingSpansNotReportedAsContradictions(t *testing.T) {
	g := eg.New()

	a := makeSpan("agree-a", "svc", "lang", "Go", 0.9, future(time.Hour))
	b := makeSpan("agree-b", "svc", "lang", "Go", 0.8, future(time.Hour))

	g.Add(a)
	g.Add(b)

	cs := g.Contradictions(epoch)
	if len(cs) != 0 {
		t.Errorf("agreeing spans must not be contradictions; got %v", cs)
	}
}

// TestContradictionsExcludeExpiredSpans confirms that an expired span cannot
// form a contradiction with a live span — only live vs. live pairs are checked.
func TestContradictionsExcludeExpiredSpans(t *testing.T) {
	g := eg.New()

	live := makeSpan("live-c", "ent", "field", "alpha", 0.9, future(time.Hour))
	expired := makeSpan("exp-c", "ent", "field", "beta", 0.9, past(time.Minute))

	g.Add(live)
	g.Add(expired)

	cs := g.Contradictions(epoch)
	if len(cs) != 0 {
		t.Errorf("expired span must not participate in contradictions; got %v", cs)
	}
}

// TestContradictionsSorted verifies that the Contradictions slice is returned
// in a stable, deterministic order.
func TestContradictionsSorted(t *testing.T) {
	g := eg.New()

	// Two contradictions on different (EntityID, Field) pairs.
	g.Add(makeSpan("z1", "zzz", "f", "v1", 0.9, future(time.Hour)))
	g.Add(makeSpan("z2", "zzz", "f", "v2", 0.8, future(time.Hour)))
	g.Add(makeSpan("a1", "aaa", "f", "v1", 0.9, future(time.Hour)))
	g.Add(makeSpan("a2", "aaa", "f", "v2", 0.8, future(time.Hour)))

	cs := g.Contradictions(epoch)
	if len(cs) != 2 {
		t.Fatalf("expected 2 contradictions, got %d", len(cs))
	}
	if cs[0].EntityID != "aaa" || cs[1].EntityID != "zzz" {
		t.Errorf("contradictions not sorted by EntityID: %v", []string{cs[0].EntityID, cs[1].EntityID})
	}
}

// TestNeighborsSorted verifies that Neighbors returns spans in ID order.
func TestNeighborsSorted(t *testing.T) {
	g := eg.New()

	g.Add(makeSpan("z", "e", "f1", "v", 0.9, future(time.Hour)))
	g.Add(makeSpan("a", "e", "f2", "w", 0.9, future(time.Hour)))
	g.Add(makeSpan("m", "e", "f3", "x", 0.9, future(time.Hour)))
	g.Add(makeSpan("center", "e", "f4", "y", 0.9, future(time.Hour)))

	g.Link("center", "z", "r")
	g.Link("center", "a", "r")
	g.Link("center", "m", "r")

	ns := g.Neighbors("center", epoch)
	if len(ns) != 3 {
		t.Fatalf("expected 3 neighbors, got %d", len(ns))
	}
	if ns[0].ID != "a" || ns[1].ID != "m" || ns[2].ID != "z" {
		t.Errorf("neighbors not sorted: %v", []string{ns[0].ID, ns[1].ID, ns[2].ID})
	}
}

// TestNeverExpiresSpan verifies that a Span with zero ExpiresAt is never
// evicted by Sweep and always included in query results.
func TestNeverExpiresSpan(t *testing.T) {
	g := eg.New()

	s := makeSpan("eternal", "ent", "field", "value", 1.0, time.Time{})
	g.Add(s)

	// Sweep far into the future.
	g.Sweep(future(100 * 365 * 24 * time.Hour))

	g.Add(makeSpan("peer", "ent2", "f", "v", 0.5, future(200*365*24*time.Hour)))
	g.Link("eternal", "peer", "r")

	ns := g.Neighbors("eternal", future(100*365*24*time.Hour))
	if len(ns) != 1 || ns[0].ID != "peer" {
		t.Errorf("eternal span or its live neighbor missing after far-future Sweep: %v", ns)
	}
}

// TestOverwriteSpan verifies that adding a span with a duplicate ID replaces
// the original.
func TestOverwriteSpan(t *testing.T) {
	g := eg.New()

	original := makeSpan("s1", "ent", "field", "original", 0.5, future(time.Hour))
	updated := makeSpan("s1", "ent", "field", "updated", 0.9, future(time.Hour))

	g.Add(original)
	g.Add(updated)

	g.Add(makeSpan("s2", "ent2", "f", "v", 0.5, future(time.Hour)))
	g.Link("s1", "s2", "r")

	ns := g.Neighbors("s2", epoch)
	if len(ns) != 1 || ns[0].Value != "updated" {
		t.Errorf("expected overwritten span with value 'updated', got %v", ns)
	}
}

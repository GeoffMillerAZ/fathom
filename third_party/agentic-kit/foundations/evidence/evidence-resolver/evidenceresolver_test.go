package evidenceresolver_test

import (
	"testing"

	er "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-resolver"
	es "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
)

func makeSpan(id, entityID, field, value string) es.Span {
	return es.Span{
		ID:         id,
		EntityID:   entityID,
		Field:      field,
		Value:      value,
		Confidence: 0.9,
		SourceID:   "src-test",
	}
}

// TestResolveRegistered verifies a registered span can be resolved.
func TestResolveRegistered(t *testing.T) {
	var r er.Resolver
	s := makeSpan("s1", "repo-a", "language", "Go")
	r.Register(s)

	got, ok := r.Resolve("s1")
	if !ok {
		t.Fatal("expected span to be found")
	}
	if got.ID != "s1" || got.Value != "Go" {
		t.Errorf("unexpected span: %+v", got)
	}
}

// TestResolveMissing verifies Resolve returns false for unknown refs.
func TestResolveMissing(t *testing.T) {
	var r er.Resolver
	_, ok := r.Resolve("nonexistent")
	if ok {
		t.Fatal("expected found=false for unknown ref")
	}
}

// TestRegisterOverwrite verifies later registrations silently overwrite.
func TestRegisterOverwrite(t *testing.T) {
	var r er.Resolver
	r.Register(makeSpan("s1", "repo-a", "language", "Go"))
	r.Register(makeSpan("s1", "repo-a", "language", "Rust")) // overwrite

	got, ok := r.Resolve("s1")
	if !ok {
		t.Fatal("expected span to be found after overwrite")
	}
	if got.Value != "Rust" {
		t.Errorf("expected overwritten value 'Rust', got %q", got.Value)
	}
}

// TestResolveAllOrdering verifies ResolveAll preserves input order.
func TestResolveAllOrdering(t *testing.T) {
	var r er.Resolver
	r.Register(makeSpan("a", "e1", "f", "v1"))
	r.Register(makeSpan("b", "e2", "f", "v2"))
	r.Register(makeSpan("c", "e3", "f", "v3"))

	results := r.ResolveAll([]string{"c", "a", "missing", "b"})
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	if !results[0].Found || results[0].Span.ID != "c" {
		t.Errorf("position 0: expected c, got %+v", results[0])
	}
	if !results[1].Found || results[1].Span.ID != "a" {
		t.Errorf("position 1: expected a, got %+v", results[1])
	}
	if results[2].Found {
		t.Errorf("position 2: expected not found for 'missing'")
	}
	if !results[3].Found || results[3].Span.ID != "b" {
		t.Errorf("position 3: expected b, got %+v", results[3])
	}
}

// TestResolveAllEmpty verifies empty input returns empty output.
func TestResolveAllEmpty(t *testing.T) {
	var r er.Resolver
	results := r.ResolveAll(nil)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

// TestZeroValueResolver verifies the zero value of Resolver is safe to use.
func TestZeroValueResolver(t *testing.T) {
	var r er.Resolver
	_, ok := r.Resolve("any")
	if ok {
		t.Fatal("zero-value resolver should return false for any ref")
	}
	results := r.ResolveAll([]string{"x", "y"})
	for i, res := range results {
		if res.Found {
			t.Errorf("position %d: expected not found in zero-value resolver", i)
		}
	}
}

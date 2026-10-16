package sourcelinker_test

import (
	"reflect"
	"testing"

	sl "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/source-linker"
)

// TestLinkAndSourcesFor verifies SourcesFor returns associated sources.
func TestLinkAndSourcesFor(t *testing.T) {
	var l sl.Linker
	l.Link("span-1", "src-a")
	l.Link("span-1", "src-b")
	l.Link("span-2", "src-a")

	got := l.SourcesFor("span-1")
	want := []string{"src-a", "src-b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SourcesFor(span-1): got %v, want %v", got, want)
	}

	got2 := l.SourcesFor("span-2")
	want2 := []string{"src-a"}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("SourcesFor(span-2): got %v, want %v", got2, want2)
	}
}

// TestLinkAndSpansFor verifies SpansFor returns associated spans.
func TestLinkAndSpansFor(t *testing.T) {
	var l sl.Linker
	l.Link("span-1", "src-a")
	l.Link("span-2", "src-a")
	l.Link("span-3", "src-b")

	got := l.SpansFor("src-a")
	want := []string{"span-1", "span-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SpansFor(src-a): got %v, want %v", got, want)
	}

	got2 := l.SpansFor("src-b")
	want2 := []string{"span-3"}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("SpansFor(src-b): got %v, want %v", got2, want2)
	}
}

// TestLinkIdempotent verifies calling Link with the same pair twice is safe.
func TestLinkIdempotent(t *testing.T) {
	var l sl.Linker
	l.Link("span-1", "src-a")
	l.Link("span-1", "src-a") // duplicate

	sources := l.SourcesFor("span-1")
	if len(sources) != 1 {
		t.Errorf("expected 1 source after duplicate link, got %d: %v", len(sources), sources)
	}
}

// TestSourcesForMissing verifies SourcesFor returns nil for unknown spans.
func TestSourcesForMissing(t *testing.T) {
	var l sl.Linker
	if got := l.SourcesFor("nope"); got != nil {
		t.Errorf("expected nil for unknown span, got %v", got)
	}
}

// TestSpansForMissing verifies SpansFor returns nil for unknown sources.
func TestSpansForMissing(t *testing.T) {
	var l sl.Linker
	if got := l.SpansFor("nope"); got != nil {
		t.Errorf("expected nil for unknown source, got %v", got)
	}
}

// TestDeterministicOrder verifies sorted output is consistent across calls.
func TestDeterministicOrder(t *testing.T) {
	var l sl.Linker
	l.Link("span-z", "src-c")
	l.Link("span-z", "src-a")
	l.Link("span-z", "src-b")

	got1 := l.SourcesFor("span-z")
	got2 := l.SourcesFor("span-z")
	want := []string{"src-a", "src-b", "src-c"}

	if !reflect.DeepEqual(got1, want) {
		t.Errorf("first call: got %v, want %v", got1, want)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Errorf("calls not identical: %v vs %v", got1, got2)
	}
}

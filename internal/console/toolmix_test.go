package console

import "testing"

// PE-18 (docs/design/progress-and-economics.md §5): the tool-call mix
// (share of calls by tool name) and the cache hit rate (cache-read tokens
// over cache-read-plus-fresh-input tokens), both a pure projection over
// counts a caller already has — a session's own (PE-13's
// foldTranscriptCost) or a plan's aggregate.
func TestToolMixAndCacheHit(t *testing.T) {
	calls := map[string]int{"Read": 6, "Edit": 2, "mcp__fathom__search": 2}
	m := ToolMix(calls, 300, 100) // 300 cache-read, 100 fresh input

	if m.TotalCalls != 10 {
		t.Fatalf("TotalCalls = %d, want 10", m.TotalCalls)
	}
	if len(m.Mix) != 3 {
		t.Fatalf("want 3 mix entries, got %+v", m.Mix)
	}
	// Sorted by call count descending, tool name breaking ties.
	if m.Mix[0].Tool != "Read" || m.Mix[0].Calls != 6 || m.Mix[0].Share != 0.6 {
		t.Fatalf("top entry wrong: %+v", m.Mix[0])
	}
	if m.Mix[1].Tool != "Edit" || m.Mix[2].Tool != "mcp__fathom__search" {
		t.Fatalf("tie-break by tool name wrong: %+v", m.Mix)
	}
	wantHit := 300.0 / 400.0
	if m.CacheHit.V != wantHit || m.CacheHit.State != StatePresent {
		t.Fatalf("CacheHit = %+v, want %v present", m.CacheHit, wantHit)
	}

	// No calls at all: an absent mix, never an empty one rendered as 0%.
	empty := ToolMix(nil, 0, 0)
	if empty.Mix != nil || empty.TotalCalls != 0 {
		t.Fatalf("no calls should yield no mix entries: %+v", empty)
	}
	if empty.CacheHit.State != StateAbsent {
		t.Fatalf("no cache or fresh-input tokens: cache hit rate must be absent, never a fabricated 0: %+v", empty.CacheHit)
	}
}

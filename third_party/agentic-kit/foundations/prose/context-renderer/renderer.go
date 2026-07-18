package contextrenderer

import (
	"sort"
	"strings"
)

// Render assembles a bounded context block from segments, respecting budget
// tokens. Segments are sorted by Priority descending — highest-priority
// content is included first. Segments whose Tokens would exceed the remaining
// budget are skipped (they are not partially included). The result is
// deterministic: segments with equal Priority are ordered by ID ascending to
// break ties.
//
// If budget is <= 0 the returned string is empty.
// If segments is empty the returned string is empty.
func Render(segments []Segment, budget int) string {
	if budget <= 0 || len(segments) == 0 {
		return ""
	}

	// Copy and sort: priority descending, then ID ascending to break ties.
	sorted := make([]Segment, len(segments))
	copy(sorted, segments)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		return sorted[i].ID < sorted[j].ID
	})

	remaining := budget
	var parts []string
	for _, seg := range sorted {
		tokens := seg.Tokens
		if tokens <= 0 {
			tokens = 1
		}
		if tokens > remaining {
			continue
		}
		parts = append(parts, seg.Content)
		remaining -= tokens
	}

	return strings.Join(parts, "\n")
}

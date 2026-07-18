package documentparsing

import (
	"strings"
)

// Parse splits doc into a deterministic sequence of Segments.
//
// Splitting rules:
//   - An ATX heading is a line whose first non-space characters are one or
//     more '#' characters followed by a space: e.g. "# Foo", "## Bar".
//   - Each heading starts a new Segment. The text before the first heading
//     (the preamble) becomes a Segment with an empty Heading.
//   - The Text of each Segment is the joined body lines between consecutive
//     headings, with the composite block stripped of leading/trailing
//     whitespace. Internal blank lines are preserved.
//   - Segments are assigned Order values 0, 1, 2, … in document order.
//   - An empty or whitespace-only document returns a single Segment with
//     empty Heading, empty Text, and Order 0.
//
// Parse never returns nil; it always returns at least one Segment.
func Parse(doc string) []Segment {
	lines := strings.Split(doc, "\n")

	type raw struct {
		heading string
		lines   []string
	}

	var chunks []raw
	current := raw{}

	for _, line := range lines {
		if h, ok := extractHeading(line); ok {
			chunks = append(chunks, current)
			current = raw{heading: h}
		} else {
			current.lines = append(current.lines, line)
		}
	}
	chunks = append(chunks, current)

	segments := make([]Segment, 0, len(chunks))
	for i, c := range chunks {
		text := strings.TrimSpace(strings.Join(c.lines, "\n"))
		segments = append(segments, Segment{
			Heading: c.heading,
			Text:    text,
			Order:   i,
		})
	}

	return segments
}

// extractHeading returns the heading text and true when line is an ATX heading
// (one or more '#' chars immediately followed by a space and the heading text).
// Both leading and trailing whitespace on line are ignored.
func extractHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	// Count leading '#' characters.
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	// Must be followed by exactly one space and at least one more character.
	if i >= len(trimmed) || trimmed[i] != ' ' {
		return "", false
	}
	heading := strings.TrimSpace(trimmed[i+1:])
	if heading == "" {
		return "", false
	}
	return heading, true
}

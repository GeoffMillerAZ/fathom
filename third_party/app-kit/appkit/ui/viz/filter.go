package viz

import (
	"slices"
	"strings"
	"time"
)

// EventFilter selects timeline events and spans. The zero value matches
// everything; set only the fields you want to constrain. It is the kit's
// reusable selection toolkit so apps (Throughline) drive a timeline from UI
// controls — a date window, lane/role facets, a search box — without
// reimplementing the predicates. Rendering stays pure: filter first, then pass
// the result to Timeline.
type EventFilter struct {
	// From / To bound the time window (inclusive). Zero means unbounded on that
	// side. A span matches if it overlaps the window at all.
	From time.Time
	To   time.Time
	// Lanes, when non-empty, keeps only items whose Lane is in the set.
	Lanes []string
	// Roles, when non-empty, keeps only items whose Role is in the set.
	Roles []string
	// Query, when non-empty, keeps only items whose Label contains it
	// (case-insensitive substring — the "search box").
	Query string
}

// MatchEvent reports whether an event passes the filter.
func (f EventFilter) MatchEvent(e Event) bool {
	if !f.From.IsZero() && e.At.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && e.At.After(f.To) {
		return false
	}
	return f.matchFacets(e.Lane, e.Role, e.Label)
}

// MatchSpan reports whether a span passes the filter. The time test is overlap:
// a span is kept if any part of it falls within [From, To].
func (f EventFilter) MatchSpan(s Span) bool {
	start, end := s.Start, s.End
	if end.Before(start) {
		start, end = end, start
	}
	if !f.From.IsZero() && end.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && start.After(f.To) {
		return false
	}
	return f.matchFacets(s.Lane, s.Role, s.Label)
}

func (f EventFilter) matchFacets(lane, role, label string) bool {
	if len(f.Lanes) > 0 && !slices.Contains(f.Lanes, lane) {
		return false
	}
	if len(f.Roles) > 0 && !slices.Contains(f.Roles, role) {
		return false
	}
	if f.Query != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(f.Query)) {
		return false
	}
	return true
}

// FilterEvents returns the events that pass the filter, in their original order
// (a new slice; the input is not mutated). Nil filter fields match all.
func (f EventFilter) FilterEvents(events []Event) []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		if f.MatchEvent(e) {
			out = append(out, e)
		}
	}
	return out
}

// FilterSpans returns the spans that pass the filter, in their original order.
func (f EventFilter) FilterSpans(spans []Span) []Span {
	out := make([]Span, 0, len(spans))
	for _, s := range spans {
		if f.MatchSpan(s) {
			out = append(out, s)
		}
	}
	return out
}

// Apply filters a whole TimelineOptions' Spans (and returns filtered events)
// together, so a caller can run one filter over an entire timeline:
//
//	ev, opts := f.Apply(events, opts)
//	html := viz.Timeline(ev, opts)
//
// Markers are reference lines (today, deadlines) and are left intact.
func (f EventFilter) Apply(events []Event, o TimelineOptions) ([]Event, TimelineOptions) {
	o.Spans = f.FilterSpans(o.Spans)
	return f.FilterEvents(events), o
}

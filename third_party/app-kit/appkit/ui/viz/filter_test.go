package viz

import (
	"strings"
	"testing"
)

func filterFixture() []Event {
	return []Event{
		{At: day(2026, 1, 5), Label: "Complaint filed", Role: "accent", Lane: "court"},
		{At: day(2026, 2, 20), Label: "Voicemail left", Role: "info", Lane: "evidence"},
		{At: day(2026, 6, 1), Label: "Email thread", Role: "info", Lane: "evidence"},
		{At: day(2026, 9, 9), Label: "Hearing scheduled", Role: "danger", Lane: "court"},
	}
}

func TestEventFilter_ZeroMatchesAll(t *testing.T) {
	ev := filterFixture()
	got := EventFilter{}.FilterEvents(ev)
	if len(got) != len(ev) {
		t.Errorf("zero filter dropped events: got %d want %d", len(got), len(ev))
	}
}

func TestEventFilter_Window(t *testing.T) {
	got := EventFilter{From: day(2026, 2, 1), To: day(2026, 7, 1)}.FilterEvents(filterFixture())
	if len(got) != 2 {
		t.Fatalf("window: got %d, want 2", len(got))
	}
	if got[0].Label != "Voicemail left" || got[1].Label != "Email thread" {
		t.Errorf("window kept wrong events: %+v", got)
	}
}

func TestEventFilter_Lane(t *testing.T) {
	got := EventFilter{Lanes: []string{"court"}}.FilterEvents(filterFixture())
	if len(got) != 2 {
		t.Fatalf("lane: got %d, want 2", len(got))
	}
	for _, e := range got {
		if e.Lane != "court" {
			t.Errorf("lane filter leaked %q", e.Lane)
		}
	}
}

func TestEventFilter_Role(t *testing.T) {
	got := EventFilter{Roles: []string{"info"}}.FilterEvents(filterFixture())
	if len(got) != 2 {
		t.Errorf("role: got %d, want 2", len(got))
	}
}

func TestEventFilter_QueryCaseInsensitive(t *testing.T) {
	// "THREAD" (upper) matches "Email thread" only — case-insensitive substring.
	got := EventFilter{Query: "THREAD"}.FilterEvents(filterFixture())
	if len(got) != 1 || got[0].Label != "Email thread" {
		t.Errorf("query search failed: %+v", got)
	}
}

func TestEventFilter_Combined(t *testing.T) {
	// evidence lane AND within window AND matching search.
	f := EventFilter{
		From:  day(2026, 1, 1),
		To:    day(2026, 3, 1),
		Lanes: []string{"evidence"},
		Query: "voicemail",
	}
	got := f.FilterEvents(filterFixture())
	if len(got) != 1 || got[0].Label != "Voicemail left" {
		t.Errorf("combined filter failed: %+v", got)
	}
}

func TestEventFilter_SpanOverlap(t *testing.T) {
	spans := []Span{
		{Start: day(2026, 1, 5), End: day(2026, 2, 3), Label: "pleadings", Lane: "court"},
		{Start: day(2026, 8, 1), End: day(2026, 9, 1), Label: "trial prep", Lane: "court"},
	}
	// Window touching only the first span (overlap, not containment).
	got := EventFilter{From: day(2026, 1, 20), To: day(2026, 1, 25)}.FilterSpans(spans)
	if len(got) != 1 || got[0].Label != "pleadings" {
		t.Errorf("span overlap filter failed: %+v", got)
	}
	// Window between the spans matches neither.
	none := EventFilter{From: day(2026, 4, 1), To: day(2026, 5, 1)}.FilterSpans(spans)
	if len(none) != 0 {
		t.Errorf("span filter should match none in the gap, got %+v", none)
	}
}

func TestEventFilter_ApplyAndRenderClean(t *testing.T) {
	ev := filterFixture()
	opts := TimelineOptions{
		Spans:   []Span{{Start: day(2026, 1, 5), End: day(2026, 2, 3), Label: "pleadings", Role: "accent-soft", Lane: "court"}},
		Markers: []Marker{{At: day(2026, 6, 15), Label: "today", Role: "focus"}},
	}
	fev, fopts := EventFilter{Lanes: []string{"court"}}.Apply(ev, opts)
	// Markers preserved; events/spans constrained to the court lane.
	if len(fopts.Markers) != 1 {
		t.Errorf("Apply dropped markers")
	}
	if len(fev) != 2 || len(fopts.Spans) != 1 {
		t.Errorf("Apply filtered wrong: events=%d spans=%d", len(fev), len(fopts.Spans))
	}
	// The filtered result still renders.
	out := Timeline(fev, fopts)
	if !strings.Contains(out, "<svg") || !strings.Contains(out, "Complaint filed") {
		t.Errorf("filtered timeline did not render expected content")
	}
}

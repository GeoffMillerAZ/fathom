package viz

import (
	"strings"
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// A small fixture: three close events in early 2026, then one far later — the
// kind of shape the condensed view exists for.
func tlFixture() ([]Event, TimelineOptions) {
	events := []Event{
		{At: day(2026, 1, 5), Label: "filed", Role: "accent"},
		{At: day(2026, 1, 20), Label: "served", Role: "info"},
		{At: day(2026, 2, 3), Label: "answered", Role: "positive"},
		{At: day(2026, 11, 30), Label: "hearing", Role: "danger"},
	}
	opts := TimelineOptions{
		W: 560,
		Spans: []Span{
			{Start: day(2026, 1, 5), End: day(2026, 2, 3), Label: "pleadings", Role: "accent-soft"},
		},
		Markers: []Marker{
			{At: day(2026, 6, 15), Label: "today", Role: "focus"},
		},
	}
	return events, opts
}

func TestTimeline_Proportional_Golden(t *testing.T) {
	events, opts := tlFixture()
	opts.AriaLabel = "case timeline"
	assertGolden(t, Timeline(events, opts), "timeline_proportional")
}

func TestTimeline_Sequence_Golden(t *testing.T) {
	events, opts := tlFixture()
	opts.View = ViewSequence
	assertGolden(t, Timeline(events, opts), "timeline_sequence")
}

func TestTimeline_Condensed_Golden(t *testing.T) {
	events, opts := tlFixture()
	opts.View = ViewCondensed
	assertGolden(t, Timeline(events, opts), "timeline_condensed")
}

func TestTimeline_Swimlanes_Golden(t *testing.T) {
	events := []Event{
		{At: day(2026, 1, 5), Label: "filed", Role: "accent", Lane: "court"},
		{At: day(2026, 2, 3), Label: "answered", Role: "positive", Lane: "court"},
		{At: day(2026, 1, 20), Label: "voicemail", Role: "info", Lane: "evidence"},
		{At: day(2026, 3, 12), Label: "email", Role: "info", Lane: "evidence"},
	}
	opts := TimelineOptions{
		W:         640,
		AriaLabel: "case by lane",
		Spans: []Span{
			{Start: day(2026, 1, 5), End: day(2026, 2, 3), Label: "pleadings", Role: "accent-soft", Lane: "court"},
		},
		Markers: []Marker{{At: day(2026, 2, 20), Label: "today", Role: "focus"}},
	}
	assertGolden(t, Timeline(events, opts), "timeline_swimlanes")
}

// TestTimeline_SwimlanesGroupRows checks lane mode activates and the labels are
// present and rows distinct.
func TestTimeline_SwimlanesGroupRows(t *testing.T) {
	events := []Event{
		{At: day(2026, 1, 5), Label: "a", Role: "accent", Lane: "alpha"},
		{At: day(2026, 1, 9), Label: "b", Role: "info", Lane: "beta"},
	}
	got := Timeline(events, TimelineOptions{})
	for _, lane := range []string{"alpha", "beta"} {
		if !strings.Contains(got, ">"+lane+"<") {
			t.Errorf("swimlane label %q not rendered", lane)
		}
	}
	// Two distinct lanes → the two dots must be at different y (baselines).
	lanes := timelineLanes(events, nil)
	if len(lanes) != 2 {
		t.Fatalf("expected 2 lanes, got %d (%v)", len(lanes), lanes)
	}
	// No-lane data stays single-axis.
	if timelineLanes([]Event{{At: day(2026, 1, 1), Label: "x", Role: "accent"}}, nil) != nil {
		t.Errorf("events without lanes should not trigger swimlane mode")
	}
}

func TestTimeline_Density_Golden(t *testing.T) {
	events := []Event{
		{At: day(2026, 1, 5), Label: "a", Role: "accent"},
		{At: day(2026, 1, 12), Label: "b", Role: "accent"},
		{At: day(2026, 1, 18), Label: "c", Role: "accent"},
		{At: day(2026, 7, 1), Label: "d", Role: "info"},
		{At: day(2026, 11, 30), Label: "e", Role: "danger"},
	}
	opts := TimelineOptions{
		W:              640,
		View:           ViewDensity,
		DensityBuckets: 12,
		AriaLabel:      "activity density",
		Markers:        []Marker{{At: day(2026, 6, 15), Label: "today", Role: "focus"}},
	}
	assertGolden(t, Timeline(events, opts), "timeline_density")
}

func TestTimeline_Density_Laned(t *testing.T) {
	events := []Event{
		{At: day(2026, 1, 5), Label: "a", Role: "accent", Lane: "court"},
		{At: day(2026, 6, 1), Label: "b", Role: "info", Lane: "evidence"},
	}
	got := Timeline(events, TimelineOptions{View: ViewDensity, DensityBuckets: 10})
	if strings.Count(got, "court")+strings.Count(got, "evidence") < 2 {
		t.Errorf("laned density missing lane labels: %s", got)
	}
	// Heatmap cells are rects with fill-opacity; expect at least 2 rows × 10.
	if c := strings.Count(got, "fill-opacity"); c < 20 {
		t.Errorf("laned density expected >=20 cells, got %d", c)
	}
}

func TestTimeline_Density_Empty(t *testing.T) {
	got := Timeline(nil, TimelineOptions{View: ViewDensity})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty density missing svg")
	}
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("empty density has NaN/Inf: %s", got)
	}
}

func TestTimeline_Empty(t *testing.T) {
	got := Timeline(nil, TimelineOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty timeline missing svg tag")
	}
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("empty timeline has NaN/Inf: %s", got)
	}
}

func TestTimeline_SingleInstant(t *testing.T) {
	// All times equal → zero range; everything centers, no divide-by-zero.
	e := []Event{{At: day(2026, 5, 1), Label: "only", Role: "accent"}}
	got := Timeline(e, TimelineOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("single-instant timeline has NaN/Inf: %s", got)
	}
	if !strings.Contains(got, "<circle") {
		t.Errorf("single-instant timeline should plot the event dot")
	}
}

// TestTimeline_CondensedElidesGap verifies the condensed view actually inserts a
// labeled break for the long empty stretch, and that the proportional view does
// not.
func TestTimeline_CondensedElidesGap(t *testing.T) {
	events, opts := tlFixture()
	opts.View = ViewCondensed
	got := Timeline(events, opts)
	// The Feb→Nov gap is ~10 months and should be labeled.
	if !strings.Contains(got, "mo") {
		t.Errorf("condensed view did not label the elided gap with a duration: %s", got)
	}
	if !strings.Contains(got, "omitted") {
		t.Errorf("condensed view break missing the honest 'omitted' title: %s", got)
	}
	// Proportional must NOT elide.
	prop := Timeline(events, TimelineOptions{W: 560})
	if strings.Contains(prop, "omitted") {
		t.Errorf("proportional view should not elide gaps")
	}
}

// TestTimeline_CondensedKeepsOrder checks that even after eliding, the x
// positions stay monotonic with time (nothing reordered or overlapped).
func TestTimeline_CondensedKeepsOrder(t *testing.T) {
	events, opts := tlFixture()
	opts.View = ViewCondensed
	s := buildScale(events, opts.withDefaults(), tlPadX, 560-2*tlPadX)
	prev := -1.0
	for _, e := range events {
		x := s.at(e.At)
		if x < prev {
			t.Errorf("condensed scale not monotonic: %s at x=%v after %v", e.Label, x, prev)
		}
		prev = x
	}
}

func TestTimeline_Determinism(t *testing.T) {
	events, opts := tlFixture()
	for _, v := range []TimelineView{ViewProportional, ViewSequence, ViewCondensed} {
		opts.View = v
		a := Timeline(events, opts)
		b := Timeline(events, opts)
		if a != b {
			t.Errorf("timeline view %q non-deterministic", v)
		}
	}
}

func TestTimeline_LabelEscaping(t *testing.T) {
	e := []Event{{At: day(2026, 1, 1), Label: `<script>alert(1)</script>`, Role: "accent"}}
	opts := TimelineOptions{
		Markers:   []Marker{{At: day(2026, 1, 2), Label: `<img src=x>`, Role: "focus"}},
		Spans:     []Span{{Start: day(2026, 1, 1), End: day(2026, 1, 3), Label: `</text>`, Role: "accent"}},
		AriaLabel: `<svg onload=evil>`,
	}
	got := Timeline(e, opts)
	// No raw tag/attribute from user input may survive unescaped. (These
	// fragments can't occur in the legitimate markup — e.g. the only real <svg
	// is "<svg xmlns", real text closes are "</text>" with no injected attrs.)
	for _, bad := range []string{"<script", "<img", "<svg onload"} {
		if strings.Contains(got, bad) {
			t.Errorf("timeline left %q unescaped: %s", bad, got)
		}
	}
	// And the escaped forms prove the inputs were escaped rather than dropped.
	for _, want := range []string{"&lt;script&gt;", "&lt;img src=x&gt;", "&lt;/text&gt;"} {
		if !strings.Contains(got, want) {
			t.Errorf("timeline missing escaped input %q", want)
		}
	}
}

func TestTimeline_InvalidRole_FallsBack(t *testing.T) {
	e := []Event{{At: day(2026, 1, 1), Label: "x", Role: "BAD ROLE"}}
	got := Timeline(e, TimelineOptions{})
	if strings.Contains(got, "BAD ROLE") {
		t.Errorf("timeline injected invalid role: %s", got)
	}
}

func TestTimeline_HasAccessibility(t *testing.T) {
	events, opts := tlFixture()
	opts.AriaLabel = "case"
	got := Timeline(events, opts)
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, `aria-label=`) || !strings.Contains(got, "<title>") {
		t.Errorf("timeline missing accessibility attributes")
	}
}

func TestHumanDur(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{3, "3 d"},
		{21, "3 w"},
		{90, "3 mo"},
		{730, "2 y"},
	}
	for _, c := range cases {
		got := humanDur(time.Duration(c.days) * 24 * time.Hour)
		if got != c.want {
			t.Errorf("humanDur(%dd) = %q, want %q", c.days, got, c.want)
		}
	}
}

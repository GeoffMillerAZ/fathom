package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	nethtml "golang.org/x/net/html"
)

// seriesRow builds one call-log row for the series tests.
func seriesRow(ts, tool, service string) adoptionCallEntry {
	return adoptionCallEntry{TS: ts, Tool: tool, Service: service, Surface: "agent"}
}

// seriesFixture is three days of call-log traffic with a hole in the
// middle: the 23rd and 25th have rows, the 24th has none at all.
func seriesFixture() []adoptionCallEntry {
	return []adoptionCallEntry{
		seriesRow("2026-08-23T09:00:00Z", "find_symbol", "alpha"),
		seriesRow("2026-08-23T09:01:00Z", "find_symbol", "alpha"),
		seriesRow("2026-08-23T09:02:00Z", "find_symbol", "alpha"),
		seriesRow("2026-08-23T09:03:00Z", "obligations", "alpha"),
		seriesRow("2026-08-23T09:04:00Z", "obligations", "alpha"),
		seriesRow("2026-08-23T09:05:00Z", "verify_plan", "alpha"),
		seriesRow("2026-08-23T09:06:00Z", "list_services", "beta"),
		seriesRow("2026-08-25T08:00:00Z", "list_services", "alpha"),
		seriesRow("2026-08-25T08:01:00Z", "server_stats", ""),
	}
}

func seriesPointFor(t *testing.T, s dimSeries, date string) seriesPoint {
	t.Helper()
	for _, p := range s.Points {
		if p.Date == date {
			return p
		}
	}
	t.Fatalf("%s: no point for %s", s.Name, date)
	return seriesPoint{}
}

// TestDimensionSeriesAbsenceIsNamed pins RG-6: the per-dimension 30-day
// series is built from real call-log rows, a day with no rows is Absent
// WITH a reason naming the date — never a zero, never interpolated — and
// a day the log was alive but this service/dimension saw nothing is a
// measured zero, which is a different claim.
func TestDimensionSeriesAbsenceIsNamed(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	series := dimensionSeries(seriesFixture(), "alpha", now, 30)

	wantDims := []string{"queried", "discovering", "preventing", "gating"}
	if len(series) != len(wantDims) {
		t.Fatalf("dimensions = %d, want %d", len(series), len(wantDims))
	}
	for i, s := range series {
		if s.Name != wantDims[i] {
			t.Errorf("dimension[%d] = %q, want %q (ladder order is fixed)", i, s.Name, wantDims[i])
		}
		if len(s.Points) != 30 {
			t.Errorf("%s: %d points, want 30", s.Name, len(s.Points))
		}
		if s.Points[0].Date != "2026-07-27" || s.Points[29].Date != "2026-08-25" {
			t.Errorf("%s: window = %s..%s, want 2026-07-27..2026-08-25 oldest first", s.Name, s.Points[0].Date, s.Points[29].Date)
		}
		if s.Measured != 2 || s.Absent != 28 {
			t.Errorf("%s: measured/absent = %d/%d, want 2/28", s.Name, s.Measured, s.Absent)
		}
		// The hole: no rows at all on the 24th → absent, named.
		hole := seriesPointFor(t, s, "2026-08-24")
		if hole.Value != nil {
			t.Errorf("%s: day with no call-log rows must be absent, got value %d", s.Name, *hole.Value)
		}
		if hole.Reason != "no call-log rows for 2026-08-24" {
			t.Errorf("%s: absent day must name itself, got reason %q", s.Name, hole.Reason)
		}
		// Before the log's first row is absent too, never a fabricated zero.
		if p := seriesPointFor(t, s, "2026-07-27"); p.Value != nil || !strings.Contains(p.Reason, "2026-07-27") {
			t.Errorf("%s: pre-log day must be absent with its date, got %+v", s.Name, p)
		}
	}

	// Measured values on the 23rd, per dimension.
	want23 := map[string]int{"queried": 6, "discovering": 3, "preventing": 2, "gating": 1}
	want25 := map[string]int{"queried": 1, "discovering": 0, "preventing": 0, "gating": 0}
	wantTotal := map[string]int{"queried": 7, "discovering": 3, "preventing": 2, "gating": 1}
	for _, s := range series {
		p := seriesPointFor(t, s, "2026-08-23")
		if p.Value == nil || *p.Value != want23[s.Name] {
			t.Errorf("%s on 2026-08-23 = %v, want %d", s.Name, p.Value, want23[s.Name])
		}
		// The 25th has rows (the log was alive) but only one for alpha, and
		// none for three of the dimensions: a MEASURED zero, not absent.
		p = seriesPointFor(t, s, "2026-08-25")
		if p.Value == nil {
			t.Errorf("%s on 2026-08-25: log was alive, must be a measured value not absent (%q)", s.Name, p.Reason)
		} else if *p.Value != want25[s.Name] {
			t.Errorf("%s on 2026-08-25 = %d, want %d", s.Name, *p.Value, want25[s.Name])
		}
		if p.Reason != "" {
			t.Errorf("%s: a measured point carries no absence reason, got %q", s.Name, p.Reason)
		}
		if s.Total == nil || *s.Total != wantTotal[s.Name] {
			t.Errorf("%s total = %v, want %d", s.Name, s.Total, wantTotal[s.Name])
		}
	}

	// A service with no rows on a day the log was alive is a measured zero.
	beta := dimensionSeries(seriesFixture(), "beta", now, 30)
	if p := seriesPointFor(t, beta[0], "2026-08-25"); p.Value == nil || *p.Value != 0 {
		t.Errorf("beta queried on 2026-08-25 = %v, want measured 0 (the log was alive)", p.Value)
	}
	if p := seriesPointFor(t, beta[0], "2026-08-23"); p.Value == nil || *p.Value != 1 {
		t.Errorf("beta queried on 2026-08-23 = %v, want 1", p.Value)
	}

	// Service matching is case-insensitive (the fold lowercases keys).
	if got := dimensionSeries(seriesFixture(), "ALPHA", now, 30); !reflect.DeepEqual(got, series) {
		t.Error("service match must be case-insensitive")
	}

	// Fleet-wide ("" service) counts every row, including service-less ones.
	fleet := dimensionSeries(seriesFixture(), "", now, 30)
	if p := seriesPointFor(t, fleet[0], "2026-08-25"); p.Value == nil || *p.Value != 2 {
		t.Errorf("fleet queried on 2026-08-25 = %v, want 2", p.Value)
	}

	// Determinism: same rows in reverse order → identical series.
	rows := seriesFixture()
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if got := dimensionSeries(rows, "alpha", now, 30); !reflect.DeepEqual(got, series) {
		t.Error("series must not depend on row order")
	}

	// No rows at all: every day absent, nothing measured, no zero anywhere —
	// Total is nil, not 0, because nothing was summed.
	empty := dimensionSeries(nil, "alpha", now, 0)
	for _, s := range empty {
		if len(s.Points) != 30 || s.Measured != 0 || s.Absent != 30 || s.Total != nil {
			t.Errorf("%s with no rows: points=%d measured=%d absent=%d total=%v", s.Name, len(s.Points), s.Measured, s.Absent, s.Total)
		}
		for _, p := range s.Points {
			if p.Value != nil || !strings.HasPrefix(p.Reason, "no call-log rows for ") {
				t.Errorf("%s %s: must be absent with a named reason, got %+v", s.Name, p.Date, p)
			}
		}
		// The JSON view omits "total" entirely: a reader never sees a zero
		// where nothing was measured (RG-8).
		js, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(js), `"total"`) {
			t.Errorf("%s: all-absent series JSON must omit total, got %s", s.Name, js)
		}
		if !strings.Contains(string(js), `"measured_days":0`) || !strings.Contains(string(js), `"absent_days":30`) {
			t.Errorf("%s: all-absent series JSON must still carry the day counts: %s", s.Name, js)
		}
	}
	// A measured series marshals its total, including a measured zero: beta's
	// gating dimension summed 0 over the fixture's 2 measured days.
	if js, _ := json.Marshal(series[0]); !strings.Contains(string(js), `"total":7`) {
		t.Errorf("measured series JSON must carry its total: %s", js)
	}
	if js, _ := json.Marshal(dimensionSeries(seriesFixture(), "beta", now, 30)[3]); !strings.Contains(string(js), `"total":0`) || !strings.Contains(string(js), `"measured_days":2`) {
		t.Errorf("a measured zero over measured days is a true total of 0 beside measured_days: %s", js)
	}

	// The JSON shape keeps absence as absence: no "value" key, a reason.
	js, err := json.Marshal(seriesPointFor(t, series[0], "2026-08-24"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), `"value"`) || !strings.Contains(string(js), `"reason":"no call-log rows for 2026-08-24"`) {
		t.Errorf("absent point JSON must omit value and carry the reason: %s", js)
	}
}

// TestSeriesSegments pins the run split behind the strip: measured and
// absent days partition the window into alternating runs in calendar
// order, each absent run naming its full span.
func TestSeriesSegments(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	segs := seriesSegments(dimensionSeries(seriesFixture(), "alpha", now, 30)[0])
	if len(segs) != 4 {
		t.Fatalf("segments = %d, want 4 (absent · measured · absent · measured)", len(segs))
	}
	want := []struct {
		from, to string
		n        int
		values   []float64
		reason   string
	}{
		{"2026-07-27", "2026-08-22", 27, nil, "no call-log rows for 2026-07-27 to 2026-08-22 (27 days)"},
		{"2026-08-23", "2026-08-23", 1, []float64{6}, ""},
		{"2026-08-24", "2026-08-24", 1, nil, "no call-log rows for 2026-08-24"},
		{"2026-08-25", "2026-08-25", 1, []float64{1}, ""},
	}
	for i, w := range want {
		g := segs[i]
		if g.Run.From != w.from || g.Run.To != w.to || g.Run.N != w.n || !reflect.DeepEqual(g.Values, w.values) || g.Reason != w.reason {
			t.Errorf("segment[%d] = %+v, want %+v", i, g, w)
		}
	}
	if got := seriesSegments(dimensionSeries(nil, "alpha", now, 30)[0]); len(got) != 1 || got[0].Values != nil || got[0].Run.N != 30 {
		t.Errorf("all-absent series must be one absent run of 30, got %+v", got)
	}
}

// TestGradeDistributionSeries pins RG-7: the fleet grade distribution over
// 30 days folds each day's letters in ladder order, and a day with no
// persisted grade snapshot is Absent with a reason — today's fold is the
// only measured day until snapshots accrue, and the series says so.
func TestGradeDistributionSeries(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	today := map[string][]string{
		"2026-08-25": {"B — not preventing", "A", "idle", "F — not wired", "A", "C — not queried"},
	}
	gs := gradeDistributionSeries(today, now, 30)

	if len(gs.Days) != 30 {
		t.Fatalf("days = %d, want 30", len(gs.Days))
	}
	if gs.Days[0].Date != "2026-07-27" || gs.Days[29].Date != "2026-08-25" {
		t.Errorf("window = %s..%s, want 2026-07-27..2026-08-25", gs.Days[0].Date, gs.Days[29].Date)
	}
	if gs.Measured != 1 || gs.Absent != 29 {
		t.Errorf("measured/absent = %d/%d, want 1/29", gs.Measured, gs.Absent)
	}
	last := gs.Days[29]
	wantCounts := []gradeCount{{"A", 2}, {"B", 1}, {"C", 1}, {"F", 1}, {"idle", 1}}
	if !reflect.DeepEqual(last.Counts, wantCounts) {
		t.Errorf("today's counts = %+v, want %+v (ladder order, first-gap suffix folded away)", last.Counts, wantCounts)
	}
	if last.Total != 6 || last.Reason != "" {
		t.Errorf("today: total=%d reason=%q, want 6 and no absence reason", last.Total, last.Reason)
	}
	prior := gs.Days[28]
	if prior.Counts != nil || prior.Total != 0 {
		t.Errorf("a day without a snapshot must carry no counts, got %+v", prior)
	}
	if !strings.Contains(prior.Reason, "no fleet-grade snapshot for 2026-08-24") {
		t.Errorf("absent day must name itself and the missing snapshot, got %q", prior.Reason)
	}
	// JSON: an absent day carries no "total" (RG-8 — never a zero beside a
	// reason); a measured day carries its real one.
	if js, _ := json.Marshal(prior); strings.Contains(string(js), `"total"`) || !strings.Contains(string(js), `"reason"`) {
		t.Errorf("absent day JSON must omit total and carry the reason: %s", js)
	}
	if js, _ := json.Marshal(last); !strings.Contains(string(js), `"total":6`) {
		t.Errorf("measured day JSON must carry its total: %s", js)
	}
	if !strings.Contains(gs.Note, "29") || !strings.Contains(gs.Note, "snapshot") {
		t.Errorf("series note must say how much history is missing and why: %q", gs.Note)
	}
	if got := gs.Letters(); !reflect.DeepEqual(got, []string{"A", "B", "C", "F", "idle"}) {
		t.Errorf("letters = %v, want ladder order across measured days", got)
	}

	// Two measured days: history reads oldest-first and each day folds alone.
	two := map[string][]string{
		"2026-08-25": {"A"},
		"2026-08-20": {"D — not fresh", "idle"},
	}
	gs2 := gradeDistributionSeries(two, now, 30)
	if gs2.Measured != 2 || gs2.Absent != 28 {
		t.Errorf("two-day: measured/absent = %d/%d", gs2.Measured, gs2.Absent)
	}
	if d := gs2.Days[24]; d.Date != "2026-08-20" || !reflect.DeepEqual(d.Counts, []gradeCount{{"D", 1}, {"idle", 1}}) {
		t.Errorf("2026-08-20 = %+v", d)
	}
	if got := gs2.Letters(); !reflect.DeepEqual(got, []string{"A", "D", "idle"}) {
		t.Errorf("letters across days = %v", got)
	}

	// A snapshot outside the window is ignored, not smeared in.
	if gs3 := gradeDistributionSeries(map[string][]string{"2026-01-01": {"A"}}, now, 30); gs3.Measured != 0 || gs3.Absent != 30 {
		t.Errorf("out-of-window snapshot must not count: %d/%d", gs3.Measured, gs3.Absent)
	}

	// No snapshots at all: every day absent, the note says nothing was folded.
	none := gradeDistributionSeries(nil, now, 0)
	if len(none.Days) != 30 || none.Measured != 0 || none.Absent != 30 || len(none.Letters()) != 0 {
		t.Errorf("no snapshots: days=%d measured=%d absent=%d letters=%v", len(none.Days), none.Measured, none.Absent, none.Letters())
	}
	for _, d := range none.Days {
		if d.Counts != nil || !strings.Contains(d.Reason, "no fleet-grade snapshot for "+d.Date) {
			t.Errorf("%s: must be absent with a named reason, got %+v", d.Date, d)
		}
	}

	// Determinism: same input twice is byte-identical.
	if a, b := gradeDistributionSeries(today, now, 30), gradeDistributionSeries(today, now, 30); !reflect.DeepEqual(a, b) {
		t.Error("gradeDistributionSeries must be pure")
	}
}

// tileValue returns the kit stat tile's rendered value for the unique tile
// whose semantic label is label. It scopes the value to that tile's own
// sibling value group, so a neighboring widget cannot satisfy the assertion.
func tileValue(t *testing.T, body, label string) string {
	t.Helper()
	value, err := checkTileValue(body, label)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func checkTileValue(body, label string) (string, error) {
	doc, err := nethtml.Parse(strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parse rendered stat HTML: %w", err)
	}
	var labels, candidates int
	var value *nethtml.Node
	walkHTML(doc, func(node *nethtml.Node) {
		if (!isElement(node, "p") && !isElement(node, "dt")) || nodeText(node) != label {
			return
		}
		labels++
		candidate := statTileValueNode(node)
		if candidate != nil {
			candidates++
			value = candidate
		}
	})
	if labels == 0 {
		return "", fmt.Errorf("no stat tile labeled %q", label)
	}
	if labels != 1 {
		return "", fmt.Errorf("found %d stat labels %q; want exactly 1", labels, label)
	}
	if candidates != 1 {
		return "", fmt.Errorf("stat tile %q has no structurally scoped value span", label)
	}
	return nodeText(value), nil
}

func statTileValueNode(label *nethtml.Node) *nethtml.Node {
	valueRow := nextElementSibling(label)
	if isElement(label, "dt") {
		if !isElement(valueRow, "dd") {
			return nil
		}
	} else if !isElement(valueRow, "div") {
		return nil
	}
	rowChildren := elementChildren(valueRow)
	if len(rowChildren) == 0 || !isElement(rowChildren[0], "div") {
		return nil
	}
	valueChildren := elementChildren(rowChildren[0])
	if len(valueChildren) == 0 || !isElement(valueChildren[0], "span") {
		return nil
	}
	return valueChildren[0]
}

func TestObservatoryTileValueRegressionCanaries(t *testing.T) {
	const tile = `<div><p>queried</p><div><div><span>7</span><span>measured</span></div></div></div>`
	const band = `<dl><div><dt>queried</dt><dd><div><span>7</span><span>measured</span></div></dd></div></dl>`
	for name, body := range map[string]string{"tile": tile, "band": band} {
		t.Run("valid "+name, func(t *testing.T) {
			value, err := checkTileValue(body, "queried")
			if err != nil || value != "7" {
				t.Fatalf("value = %q, err = %v; want 7", value, err)
			}
		})
	}
	t.Run("missing value", func(t *testing.T) {
		if _, err := checkTileValue(`<div><p>queried</p><div></div></div>`, "queried"); err == nil {
			t.Fatal("label without its own value unexpectedly passed")
		}
	})
	t.Run("duplicate label", func(t *testing.T) {
		if _, err := checkTileValue(tile+band, "queried"); err == nil {
			t.Fatal("duplicate labels unexpectedly passed as unambiguous")
		}
	})
	t.Run("neighbor-only value", func(t *testing.T) {
		body := `<div><p>queried</p></div><div><p>other</p><div><div><span>99</span></div></div></div>`
		if _, err := checkTileValue(body, "queried"); err == nil {
			t.Fatal("neighbor tile's value unexpectedly satisfied queried")
		}
	})
}

// kvValue returns the rendered value cell of the kv row labeled `label`.
func kvValue(t *testing.T, body, label string) string {
	t.Helper()
	marker := `<span class="shrink-0 text-ink-muted">` + label + `</span><span class="min-w-0 text-right [overflow-wrap:anywhere]">`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no kv row labeled %q", label)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, "</div>")
	if j < 0 {
		t.Fatalf("kv row %q unterminated", label)
	}
	return rest[:j]
}

func hasDigit(s string) bool {
	return strings.ContainsAny(s, "0123456789")
}

// TestServicePageAllAbsent pins RG-8, the law-3 regression: the service
// page renders through the real handler with EVERY instrument absent —
// no observations file, no ledger, no call-log rows on any day, no config
// panel, no ladder — and fabricates no zero where an absence belongs.
func TestServicePageAllAbsent(t *testing.T) {
	hermeticObsEnv(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	row := obsServiceRow{}
	row.Service = "void"
	row.Path = ""
	row.Freshness = "unknown: daemon did not report"
	row.HooksNote = "not checkable"
	row.StanzaNote = "no CLAUDE.md"
	row.OpenObservations = nil
	row.SessionCount = nil
	row.Config = nil
	row.Realization = nil
	row.Series = dimensionSeries(nil, "void", now, seriesDays)
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "test",
		GeneratedAt:   now.Format(time.RFC3339),
		Arm:           obsArmBanner{Detail: "no measurement arm active"},
		Services:      []obsServiceRow{row},
	}
	// Seed the page cache so the real handler serves this model without a
	// daemon; the TTL is measured from wall-clock now.
	obsCache.Lock()
	obsCache.model, obsCache.at = m, time.Now()
	obsCache.Unlock()
	t.Cleanup(resetObsCache)

	h := observatoryServicePage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/service?id=void", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /observatory/service?id=void = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// open observations: the absence note, never a number.
	if v := kvValue(t, body, "open observations"); !strings.Contains(v, "no observations file") || hasDigit(v) {
		t.Errorf("open observations must render its absence without a digit: %q", v)
	}
	// agent-surface share: the note, never a gauge at 0/0.
	if !strings.Contains(body, "no recorded calls for this service") {
		t.Error("agent-surface share must name the absence of calls")
	}
	if strings.Contains(body, `aria-label="agent-surface share"`) {
		t.Error("no calls → no gauge (a 0/0 gauge is a fabricated zero)")
	}
	// The 30-day series: every dimension tile is Absent with the reason,
	// no sparkline is drawn, and no digit stands in for a measurement.
	if !strings.Contains(body, "activity — last 30 days") {
		t.Fatal("service page must carry the 30-day series section")
	}
	for _, dim := range []string{"queried", "discovering", "preventing", "gating"} {
		if v := tileValue(t, body, dim); v != "—" {
			t.Errorf("%s tile with nothing measured must show —, got %q", dim, v)
		}
	}
	if !strings.Contains(body, "no call-log rows on any of the last 30 days") {
		t.Error("all-absent series must say why every dimension is absent")
	}
	if strings.Contains(body, "<polyline") {
		t.Error("no sparkline may be drawn when no day was measured")
	}
	if !strings.Contains(body, "no call-log rows for 2026-07-27") || !strings.Contains(body, "2026-08-25") {
		t.Error("the absent run must name its dates")
	}
	// The JSON view carries the same absence: no value keys in the series.
	resetObsCache()
	obsCache.Lock()
	obsCache.model, obsCache.at = m, time.Now()
	obsCache.Unlock()
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/service?id=void&format=json", nil))
	var got obsServiceRow
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got.Series) != 4 {
		t.Fatalf("json series = %d dims", len(got.Series))
	}
	for _, s := range got.Series {
		if s.Measured != 0 || s.Absent != seriesDays || s.Total != nil {
			t.Errorf("json %s: measured=%d absent=%d total=%v", s.Name, s.Measured, s.Absent, s.Total)
		}
		for _, p := range s.Points {
			if p.Value != nil {
				t.Errorf("json %s %s: value must be absent, got %d", s.Name, p.Date, *p.Value)
			}
		}
	}
}

// TestServicePageSourcesUnread pins the other half of RG-8: when a SOURCE
// could not be read at all — the call log unreadable, no store reporting
// a symbol count — the real handler renders the symbols, traffic,
// rules-served and agent-share tiles as named absences carrying the read
// error, the 30-day series says why it was not built, the fleet band's
// symbols and tool-call tiles show "—", and no digit stands in for a
// measurement anywhere in those tiles. Before this test the model ignored
// readCallLog's read bool, and an unreadable log rendered as "0 calls
// (0 agent · 0/0)" — a fabricated zero (law 3).
func TestServicePageSourcesUnread(t *testing.T) {
	hermeticObsEnv(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	const logNote = "call log unreadable: open /x/calls.jsonl: operation not permitted — traffic figures are not measured, never zero"
	const symNote = "no store read reported a symbol count for this service"

	row := obsServiceRow{}
	row.Service = "void"
	row.Freshness = "unknown: daemon did not report"
	row.HooksNote = "not checkable"
	row.StanzaNote = "no CLAUDE.md"
	row.SymbolsNote = symNote
	row.Config = &obsConfigPanel{LayersNote: "no semantic layers installed", GuardNote: "no guards", BindingNote: "not inspectable"}
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "test",
		GeneratedAt:   now.Format(time.RFC3339),
		Arm:           obsArmBanner{Detail: "no measurement arm active"},
		Services:      []obsServiceRow{row},
		CallLogNote:   logNote,
	}
	obsCache.Lock()
	obsCache.model, obsCache.at = m, time.Now()
	obsCache.Unlock()
	t.Cleanup(resetObsCache)

	h := observatoryServicePage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/service?id=void", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /observatory/service?id=void = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	for label, note := range map[string]string{
		"symbols":                   symNote,
		"traffic":                   logNote,
		"rules served here (calls)": logNote,
	} {
		v := kvValue(t, body, label)
		if hasDigit(v) {
			t.Errorf("%s: an unread source must render no digit, got %q", label, v)
		}
		if !strings.Contains(v, html.EscapeString(note)) {
			t.Errorf("%s: must carry the read error as its reason, got %q", label, v)
		}
	}
	if strings.Contains(body, "0 calls") || strings.Contains(body, "0 agent") || strings.Contains(body, "0/0") {
		t.Error("an unreadable call log must never render as zero calls")
	}
	if strings.Contains(body, `aria-label="agent-surface share"`) || !strings.Contains(body, "not measured — "+html.EscapeString(logNote)) {
		t.Error("agent-surface share must be a named absence, not a gauge, when the log was not read")
	}
	if !strings.Contains(body, "no 30-day series — "+html.EscapeString(logNote)) {
		t.Error("the series widget must say the log was not read, not claim thirty days of no rows")
	}
	if strings.Contains(body, "no call-log rows for") || strings.Contains(body, "<polyline") {
		t.Error("an unread log must not be folded into per-day absences or a chart")
	}

	// The fleet band and tile read the same model: symbols and tool calls
	// are "—" with the reason, and the tile's traffic row is the note.
	fleet := renderObsFleet(m)
	if v := tileValue(t, fleet, "symbols"); v != "—" {
		t.Errorf("fleet symbols tile = %q, want — when no store reported a count", v)
	}
	if v := tileValue(t, fleet, "tool calls (log)"); v != "—" {
		t.Errorf("fleet tool-calls tile = %q, want — when the log was not read", v)
	}
	if !strings.Contains(fleet, html.EscapeString(logNote)) || !strings.Contains(fleet, html.EscapeString(symNote)) {
		t.Error("fleet band must carry both read errors as reasons")
	}
	if strings.Contains(fleet, "0 calls") || strings.Contains(fleet, "14-day traffic") {
		t.Error("fleet tile must not render zero calls or a traffic sparkline from an unread log")
	}

	// A registry that never answered is the store-unreadable case at fleet
	// scale: the symbols tile carries the degraded note, not a sum of nothing.
	degraded := observatoryModel{GeneratedAt: now.Format(time.RFC3339), DegradedNote: "service registry unreachable (dial tcp: refused) — fleet tiles are not measured, never empty-as-healthy"}
	if v := tileValue(t, renderObsFleet(degraded), "symbols"); v != "—" {
		t.Errorf("degraded fleet symbols tile = %q, want —", v)
	}

	// JSON carries the same state: the row has its symbols note and no
	// series; the model names the call-log read error.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/service?id=void&format=json", nil))
	var got obsServiceRow
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got.SymbolsNote != symNote || got.Series != nil {
		t.Errorf("json row: symbols_note=%q series=%v", got.SymbolsNote, got.Series)
	}
	if js, _ := json.Marshal(m); !strings.Contains(string(js), `"call_log_note":"`+logNote+`"`) || strings.Contains(string(js), `"traffic_days"`) {
		t.Errorf("model JSON must carry call_log_note and no fabricated traffic days: %s", js)
	}
}

// TestBuildObservatoryModel_CallLogUnreadIsNamed pins the plumbing end of
// RG-8 through buildObservatoryModel itself: with the registry answering
// and the call log pointed at a path that does not exist, the model
// carries CallLogNote (readCallLog's read bool as a named absence), no
// traffic days, and no per-service series — never fourteen zero days.
func TestBuildObservatoryModel_CallLogUnreadIsNamed(t *testing.T) {
	dir := hermeticObsEnv(t)
	t.Setenv("FATHOM_CALL_LOG", filepath.Join(dir, "absent", "calls.jsonl"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/rpc"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"alpha","symbol_count":3,"languages":["go"]}],"count":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := buildObservatoryModel(srv.URL, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if m.DegradedNote != "" {
		t.Fatalf("registry answered, DegradedNote = %q", m.DegradedNote)
	}
	if len(m.Services) != 1 {
		t.Fatalf("services = %d, want 1", len(m.Services))
	}
	if !strings.HasPrefix(m.CallLogNote, "no call log at ") || !strings.Contains(m.CallLogNote, "never zero") {
		t.Errorf("CallLogNote = %q, want the honest no-log-yet reason", m.CallLogNote)
	}
	if m.TrafficDays != nil {
		t.Errorf("unread log must yield no traffic days, got %d", len(m.TrafficDays))
	}
	if m.Services[0].Series != nil {
		t.Error("unread log must yield no per-service series")
	}
	if m.Services[0].SymbolsNote != "" || m.Services[0].Symbols != 3 {
		t.Errorf("registry-reported symbols must stay measured: %+v", m.Services[0].fleetServiceRow)
	}
	// Rendered through the real fleet page: the traffic row is the note
	// (which names the path, so digits in the path are fine — a count is
	// not).
	countRe := regexp.MustCompile(`\b\d+ (calls?|agent)\b`)
	if v := kvValue(t, renderObsFleet(m), "traffic"); countRe.MatchString(v) || !strings.Contains(v, "no call log at") {
		t.Errorf("fleet tile traffic = %q, want the read absence", v)
	}
}

// TestDimensionSeriesRenders pins the present half of RG-6: measured days
// draw a kit sparkline over measured days only, absent runs render as
// stat.Absent tiles naming their dates, and a measured zero is drawn as a
// zero (it is a measurement).
func TestDimensionSeriesRenders(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	row := obsServiceRow{}
	row.Service = "alpha"
	row.Series = dimensionSeries(seriesFixture(), "alpha", now, 30)
	out := renderDimensionSeries(row, "")
	for _, want := range []string{
		"activity — last 30 days",
		`aria-label="queried — 2026-08-23, 1 measured day"`,
		`aria-label="queried — 2026-08-25, 1 measured day"`,
		"2 measured days",
		"28 absent",
		"no call-log rows for 2026-07-27 to 2026-08-22 (27 days)",
		"no call-log rows for 2026-08-24",
		"no line is ever drawn across one",
		"<svg",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("series render missing %q", want)
		}
	}
	if v := tileValue(t, out, "queried"); v != "7" {
		t.Errorf("queried total = %q, want 7", v)
	}
	if v := tileValue(t, out, "gating"); v != "1" {
		t.Errorf("gating total = %q, want 1", v)
	}

	// The gap: the 23rd and 25th are measured, the 24th is not. The strip
	// must draw two separate segments (one point each) with the absent
	// chip between them — never one polyline spanning the unmeasured day.
	strip := seriesTile(t, out, "queried")
	if n := strings.Count(strip, "<svg"); n != 2 {
		t.Errorf("queried strip has %d sparklines, want 2 (one per measured run): %s", n, strip)
	}
	for _, pts := range polylinePoints(strip) {
		if strings.Contains(pts, " ") {
			t.Errorf("a polyline spans more than one day across the gap: points=%q", pts)
		}
	}
	first, gap, second := strings.Index(strip, "<svg"), strings.Index(strip, `title="no call-log rows for 2026-08-24"`), strings.LastIndex(strip, "<svg")
	if first < 0 || gap < 0 || second < 0 || !(first < gap && gap < second) {
		t.Errorf("the absent chip must sit between the two segments (svg@%d chip@%d svg@%d)", first, gap, second)
	}
	if !strings.Contains(strip, `title="no call-log rows for 2026-07-27 to 2026-08-22 (27 days)"`) {
		t.Error("the leading absent run must be a chip naming its span")
	}

	// Positive control: an unbroken run IS one line. Three consecutive
	// measured days then the gap then one more day → one 3-point polyline
	// and one 1-point polyline, still nothing across the 24th.
	rows := append(
		seriesFixture(),
		seriesRow("2026-08-21T09:00:00Z", "find_symbol", "alpha"),
		seriesRow("2026-08-22T09:00:00Z", "find_symbol", "alpha"),
		seriesRow("2026-08-22T09:01:00Z", "find_symbol", "alpha"),
	)
	row.Series = dimensionSeries(rows, "alpha", now, 30)
	strip = seriesTile(t, renderDimensionSeries(row, ""), "queried")
	if n := strings.Count(strip, "<svg"); n != 2 {
		t.Errorf("3-day-run strip has %d sparklines, want 2: %s", n, strip)
	}
	var counts []int
	for _, pts := range polylinePoints(strip) {
		counts = append(counts, len(strings.Fields(pts)))
	}
	if !reflect.DeepEqual(counts, []int{3, 1}) {
		t.Errorf("polyline point counts = %v, want [3 1] (one unbroken 3-day run, the gap, one day)", counts)
	}
	if !strings.Contains(strip, `aria-label="queried — 2026-08-21 to 2026-08-23 (3 days), 3 measured days"`) {
		t.Errorf("segment must name its run: %s", strip)
	}

	// Nothing built at all (the model never folded the log) is named too,
	// and when the log itself was unreadable the widget says that instead.
	if out := renderDimensionSeries(obsServiceRow{}, ""); !strings.Contains(out, "no 30-day series was built") || strings.Contains(out, "<svg") {
		t.Errorf("nil series must render a named absence and no chart: %q", out)
	}
	if out := renderDimensionSeries(obsServiceRow{}, "call log unreadable: EPERM"); !strings.Contains(out, "no 30-day series — call log unreadable: EPERM") || strings.Contains(out, "<svg") {
		t.Errorf("unread log must render its read error and no chart: %q", out)
	}
}

// seriesTile returns the one enclosing dimension block whose stat tile has
// the exact label dim. DOM scoping keeps a neighboring dimension's chart out.
func seriesTile(t *testing.T, body, dim string) string {
	t.Helper()
	tile, err := checkSeriesTile(body, dim)
	if err != nil {
		t.Fatal(err)
	}
	return tile
}

func checkSeriesTile(body, dim string) (string, error) {
	doc, err := nethtml.Parse(strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parse rendered series HTML: %w", err)
	}
	var labels int
	var blocks []*nethtml.Node
	walkHTML(doc, func(label *nethtml.Node) {
		if !isElement(label, "p") || nodeText(label) != dim {
			return
		}
		labels++
		if statTileValueNode(label) == nil || !isElement(label.Parent, "div") || !isElement(label.Parent.Parent, "div") {
			return
		}
		block := label.Parent.Parent
		children := elementChildren(block)
		if len(children) == 0 || children[0] != label.Parent {
			return
		}
		blocks = append(blocks, block)
	})
	if labels == 0 {
		return "", fmt.Errorf("no series tile for %q", dim)
	}
	if labels != 1 {
		return "", fmt.Errorf("found %d series labels %q; want exactly 1", labels, dim)
	}
	if len(blocks) != 1 {
		return "", fmt.Errorf("series tile %q has no structurally scoped dimension block", dim)
	}
	var rendered strings.Builder
	if err := nethtml.Render(&rendered, blocks[0]); err != nil {
		return "", fmt.Errorf("render series tile %q: %w", dim, err)
	}
	return rendered.String(), nil
}

func TestObservatorySeriesTileRegressionCanaries(t *testing.T) {
	target := `<div><div><p>queried</p><div><div><span>7</span></div></div></div><div><svg id="target-chart"></svg></div></div>`
	neighbor := `<div><div><p>gating</p><div><div><span>1</span></div></div></div><div><svg id="neighbor-chart"></svg></div></div>`

	t.Run("neighbor chart stays out", func(t *testing.T) {
		block, err := checkSeriesTile(`<div>`+target+neighbor+`</div>`, "queried")
		if err != nil {
			t.Fatalf("valid target rejected: %v", err)
		}
		if !strings.Contains(block, `id="target-chart"`) || strings.Contains(block, `id="neighbor-chart"`) {
			t.Fatalf("series block crossed dimension boundary: %s", block)
		}
	})
	t.Run("missing target", func(t *testing.T) {
		if _, err := checkSeriesTile(neighbor, "queried"); err == nil {
			t.Fatal("missing target unexpectedly passed")
		}
	})
	t.Run("duplicate target", func(t *testing.T) {
		if _, err := checkSeriesTile(target+target, "queried"); err == nil {
			t.Fatal("duplicate targets unexpectedly passed as unambiguous")
		}
	})
}

// polylinePoints lists the points attribute of every polyline in s.
func polylinePoints(s string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`<polyline points="([^"]*)"`).FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestFleetGradeDistributionRenders pins RG-7's render: today's fold draws
// kit Bars in ladder order, the 29 snapshot-less days render as one
// stat.Absent run naming its dates, and a fleet with no ladders draws no
// bars at all.
func TestFleetGradeDistributionRenders(t *testing.T) {
	m := syntheticModel()
	a := realizationReport{Grade: "A"}
	b := realizationReport{Grade: "B — not preventing"}
	idle := realizationReport{Grade: "idle"}
	m.Services[0].Realization = &a
	m.Services = append(m.Services, obsServiceRow{Realization: &b}, obsServiceRow{Realization: &idle}, obsServiceRow{Realization: nil})
	out := renderGradeDistribution(m)
	for _, want := range []string{
		"grade distribution — last 30 days",
		`aria-label="grade distribution on 2026-08-08"`,
		"<svg",
		">A</text>",
		">B</text>",
		">idle</text>",
		"no fleet-grade snapshot for 2026-07-10 to 2026-08-07 (29 days)",
		"1 repository has no ladder",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("grade distribution render missing %q", want)
		}
	}
	if strings.Contains(out, ">C</text>") {
		t.Error("a letter no repository carries must not be drawn as a zero bar")
	}

	none := syntheticModel()
	out = renderGradeDistribution(none)
	if strings.Contains(out, "<svg") || !strings.Contains(out, "no fleet-grade snapshot for 2026-07-10 to 2026-08-08 (30 days)") {
		t.Errorf("no ladders → no bars, every day absent: %q", out)
	}

	bad := syntheticModel()
	bad.GeneratedAt = "not-a-time"
	if out := renderGradeDistribution(bad); strings.Contains(out, "<svg") || !strings.Contains(out, "generated_at") {
		t.Errorf("unparseable clock must render a named absence, not a chart: %q", out)
	}
}

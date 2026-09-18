package stat

import (
	"strings"
	"testing"
)

func TestRender_Basic(t *testing.T) {
	got := Render(Stat{Label: "Evidence items", Value: "142"})
	if !strings.Contains(got, "Evidence items") || !strings.Contains(got, ">142<") {
		t.Errorf("stat missing label/value: %s", got)
	}
	if !strings.Contains(got, "bg-surface-raised") {
		t.Errorf("stat tile should use surface-raised")
	}
}

func TestRender_Delta(t *testing.T) {
	got := Render(Stat{Label: "x", Value: "10", Delta: "+3", DeltaRole: "positive"})
	if !strings.Contains(got, "text-positive") || !strings.Contains(got, ">+3<") {
		t.Errorf("delta not rendered with role: %s", got)
	}
}

func TestRender_DeltaInvalidRoleFallsBack(t *testing.T) {
	got := Render(Stat{Label: "x", Value: "1", Delta: "+1", DeltaRole: "bad role"})
	if strings.Contains(got, "bad role") {
		t.Errorf("invalid delta role leaked")
	}
	if !strings.Contains(got, "text-ink-muted") {
		t.Errorf("invalid delta role should fall back to ink-muted")
	}
}

func TestRender_Sparkline(t *testing.T) {
	with := Render(Stat{Label: "x", Value: "1", Spark: []float64{1, 2, 3, 2, 4}})
	if !strings.Contains(with, "<svg") {
		t.Errorf("stat with spark should embed an svg")
	}
	without := Render(Stat{Label: "x", Value: "1"})
	if strings.Contains(without, "<svg") {
		t.Errorf("stat without spark should not embed an svg")
	}
}

func TestRender_Escaping(t *testing.T) {
	got := Render(Stat{Label: `<script>`, Value: `<b>`, Delta: `<i>`, DeltaRole: "danger"})
	for _, bad := range []string{"<script>", "<b>", "<i>"} {
		if strings.Contains(got, bad) {
			t.Errorf("stat did not escape %q", bad)
		}
	}
}

func TestRenderBand_Structure(t *testing.T) {
	got := RenderBand([]Stat{
		{Label: "Met", Value: "5", ValueRole: "positive"},
		{Label: "Under-evidenced", Value: "1", ValueRole: "caution"},
		{Label: "Unmet", Value: "0", ValueRole: "danger"},
		{Label: "Required", Value: "6", Sub: "elements"},
	}, BandOptions{Title: "Coverage", AriaLabel: "coverage KPIs"})
	for _, want := range []string{
		"<dl", "md:grid-cols-2 lg:grid-cols-4", "border-r border-b border-line", "bg-surface-raised",
		"Met", "text-positive", "Under-evidenced", "text-caution",
		"Unmet", "text-danger", "Required", "elements", ">Coverage<", `aria-label="coverage KPIs"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("band missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderBand_TrendPill(t *testing.T) {
	up := RenderBand([]Stat{{Label: "x", Value: "10", Delta: "12%", DeltaRole: "positive"}}, BandOptions{})
	if !strings.Contains(up, "bg-positive-soft") || !strings.Contains(up, "<svg") || !strings.Contains(up, ">12%<") {
		t.Errorf("positive trend pill wrong: %s", up)
	}
	down := RenderBand([]Stat{{Label: "x", Value: "10", Delta: "4%", DeltaRole: "danger"}}, BandOptions{})
	if !strings.Contains(down, "bg-danger-soft") {
		t.Errorf("danger trend pill should use danger-soft: %s", down)
	}
}

func TestRenderBand_EmptyIsBlank(t *testing.T) {
	if RenderBand(nil, BandOptions{}) != "" {
		t.Error("empty band should render nothing")
	}
}

func TestRenderBand_Escaping(t *testing.T) {
	got := RenderBand([]Stat{{Label: "<script>", Value: "<b>", Sub: "<i>"}}, BandOptions{Title: "<h1>"})
	for _, bad := range []string{"<script>", "<b>", "<i>", "<h1>"} {
		if strings.Contains(got, bad) {
			t.Errorf("band did not escape %q", bad)
		}
	}
}

func TestRender_Deterministic(t *testing.T) {
	s := Stat{Label: "x", Value: "1", Delta: "+1", DeltaRole: "positive", Spark: []float64{1, 2, 3}}
	a := Render(s)
	b := Render(s)
	if a != b {
		t.Errorf("stat non-deterministic")
	}
}

// Browser geometry is separately checked against rendered consumer fixtures;
// these assertions keep the scanner-visible reflow contract and literal data.
func TestMetricReflowPreservesContent(t *testing.T) {
	for name, render := range map[string]func(Stat) string{
		"tile": Render,
		"band": func(s Stat) string { return RenderBand([]Stat{s}, BandOptions{}) },
	} {
		t.Run(name, func(t *testing.T) {
			s := Stat{Label: "schema <tax>", Value: "0 B", Sub: "0 tools carried by every connected session", Delta: "+12%", Spark: []float64{1, 2, 3}}
			got := render(s)
			for _, want := range []string{
				"flex min-w-0 max-w-full flex-wrap items-baseline", "[overflow-wrap:anywhere]",
				"schema &lt;tax&gt;", ">0 B<", s.Sub, ">+12%<",
				`<span class="max-w-full text-2xl font-semibold text-ink [overflow-wrap:anywhere]">0 B</span>`,
				`<span class="min-w-0 text-sm font-medium text-ink-muted [overflow-wrap:anywhere]">` + s.Sub + `</span>`,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("missing reflow or content contract %q: %s", want, got)
				}
			}
			if strings.Contains(got, "truncate") || strings.Contains(got, "line-clamp") || strings.Contains(got, "whitespace-nowrap") {
				t.Error("metrics must not hide content or force oversized values onto one line")
			}
			absent := render(Absent("source", "unavailable <source> & reason"))
			if !strings.Contains(absent, ">—<") || !strings.Contains(absent, "unavailable &lt;source&gt; &amp; reason") {
				t.Error("reflow changed absence semantics")
			}
		})
	}
}

func TestDenseBandsDelayFullColumnCount(t *testing.T) {
	for count, want := range map[int]string{4: "md:grid-cols-2 lg:grid-cols-4", 5: "md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5"} {
		got := RenderBand(make([]Stat, count), BandOptions{})
		if !strings.Contains(got, want) {
			t.Errorf("%d metrics need room beside tablet navigation: %s", count, got)
		}
		if strings.Contains(got, "divide-x") || !strings.Contains(got, `class="-mr-px -mb-px grid`) || strings.Count(got, "border-r border-b border-line") != count {
			t.Errorf("%d metrics need row-independent clipped cell separators: %s", count, got)
		}
	}
}

func TestAbsent_NeverAZero(t *testing.T) {
	s := Absent("verify catches", "instrument absent this session")
	if s.Value != "—" || s.Sub != "instrument absent this session" || s.ValueRole != "ink-faint" {
		t.Errorf("Absent stat = %+v", s)
	}
	out := Render(s)
	if strings.Contains(out, ">0<") {
		t.Error("an absent metric must never render a zero")
	}
	if !strings.Contains(out, "instrument absent this session") {
		t.Error("the named reason must render")
	}
	if d := Absent("x", ""); d.Sub != "not measured" {
		t.Errorf("empty reason must default to a named absence, got %q", d.Sub)
	}
}

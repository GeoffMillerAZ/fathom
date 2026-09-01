// Package stat renders KPI / at-a-glance stat tiles — a big number with a
// label, an optional delta, and an optional trend sparkline — as deterministic
// server-rendered HTML themed through the design tokens (D7). It composes
// appkit/ui/viz for the trend. Pure Go, no Node; output is escaped and uses
// only kit role utilities, so tiles re-theme light/dark with the token CSS.
package stat

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

var roleRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Stat is one KPI tile.
type Stat struct {
	// Label is the metric name (e.g. "Evidence items").
	Label string
	// Value is the formatted headline value (e.g. "142", "78%", "$1.2M").
	Value string
	// ValueRole colors the headline value (e.g. "positive", "caution",
	// "danger"). Defaults to "ink". Used by RenderBand to band-color KPIs.
	ValueRole string
	// Sub is an optional small caption shown next to the value (e.g. "of 6
	// required", "from 70,946"). Empty hides it.
	Sub string
	// Delta is an optional change indicator (e.g. "+12", "-3%"). Empty hides it.
	Delta string
	// DeltaRole colors the delta (e.g. "positive", "danger", "ink-muted").
	// Defaults to ink-muted. The caller owns the good/bad semantics. In
	// RenderBand a "positive" delta draws an up arrow, "danger"/"caution" a
	// down arrow.
	DeltaRole string
	// Spark is an optional trend series drawn as a small sparkline.
	Spark []float64
	// SparkRole is the sparkline stroke role. Default "accent".
	SparkRole string
}

// BandOptions configures RenderBand.
type BandOptions struct {
	// Title is an optional heading rendered above the band.
	Title string
	// AriaLabel labels the band's <dl> for assistive tech.
	AriaLabel string
}

// RenderBand renders a row of KPIs as one shared-border card (the TailwindPlus
// "Stats with shared borders" pattern, re-themed to kit tokens): a responsive
// <dl> grid that stacks on mobile and divides into columns above it. Each item
// is a label, a (role-colorable) value with optional sub-caption, and an
// optional trend pill. Pure tokens — re-themes light/dark.
func RenderBand(items []Stat, o BandOptions) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if o.Title != "" {
		fmt.Fprintf(&b, `<h3 class="text-base font-semibold text-ink">%s</h3>`, esc(o.Title))
	}
	aria := ""
	if o.AriaLabel != "" {
		aria = fmt.Sprintf(` aria-label=%q`, esc(o.AriaLabel))
	}
	cols := gridColsClass(len(items))
	fmt.Fprintf(&b, `<dl class="mt-3 grid grid-cols-1 divide-line overflow-hidden rounded-lg border border-line bg-surface-raised %s md:divide-x md:divide-y-0"%s>`, cols, aria)
	for _, it := range items {
		valRole := it.ValueRole
		if !roleRe.MatchString(valRole) {
			valRole = "ink"
		}
		b.WriteString(`<div class="px-4 py-5 sm:p-6">`)
		fmt.Fprintf(&b, `<dt class="text-sm font-normal text-ink-muted">%s</dt>`, esc(it.Label))
		b.WriteString(`<dd class="mt-1 flex items-baseline justify-between md:block lg:flex">`)
		b.WriteString(`<div class="flex items-baseline gap-2">`)
		fmt.Fprintf(&b, `<span class="text-2xl font-semibold text-%s">%s</span>`, valRole, esc(it.Value))
		if it.Sub != "" {
			fmt.Fprintf(&b, `<span class="text-sm font-medium text-ink-muted">%s</span>`, esc(it.Sub))
		}
		b.WriteString(`</div>`)
		if it.Delta != "" {
			b.WriteString(trendPill(it.Delta, it.DeltaRole))
		}
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

// gridColsClass returns a literal md:grid-cols-N class (literal so Tailwind's
// source scan emits it). Falls back to 3 for unusual counts.
func gridColsClass(n int) string {
	switch n {
	case 1:
		return "md:grid-cols-1"
	case 2:
		return "md:grid-cols-2"
	case 3:
		return "md:grid-cols-3"
	case 4:
		return "md:grid-cols-4"
	case 5:
		return "md:grid-cols-5"
	default:
		return "md:grid-cols-3"
	}
}

// trendPill renders a rounded delta pill with an up/down arrow keyed to role:
// "positive" → up (positive tint), "danger"/"caution" → down. Other roles get a
// neutral pill with no arrow.
func trendPill(delta, role string) string {
	const up = `<svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true" class="mr-0.5 -ml-1 size-5 shrink-0 self-center"><path fill-rule="evenodd" clip-rule="evenodd" d="M10 17a.75.75 0 0 1-.75-.75V5.612L5.29 9.77a.75.75 0 0 1-1.08-1.04l5.25-5.5a.75.75 0 0 1 1.08 0l5.25 5.5a.75.75 0 1 1-1.08 1.04l-3.96-4.158V16.25A.75.75 0 0 1 10 17Z"/></svg>`
	const down = `<svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true" class="mr-0.5 -ml-1 size-5 shrink-0 self-center"><path fill-rule="evenodd" clip-rule="evenodd" d="M10 3a.75.75 0 0 1 .75.75v10.638l3.96-4.158a.75.75 0 1 1 1.08 1.04l-5.25 5.5a.75.75 0 0 1-1.08 0l-5.25-5.5a.75.75 0 1 1 1.08-1.04l3.96 4.158V3.75A.75.75 0 0 1 10 3Z"/></svg>`
	var icon, cls string
	switch role {
	case "positive":
		icon, cls = up, "bg-positive-soft text-positive"
	case "danger":
		icon, cls = down, "bg-danger-soft text-danger"
	case "caution":
		icon, cls = down, "bg-caution-soft text-ink"
	default:
		icon, cls = "", "bg-surface-sunken text-ink-muted"
	}
	return fmt.Sprintf(`<div class="inline-flex items-baseline rounded-full px-2.5 py-0.5 text-sm font-medium %s md:mt-2 lg:mt-0">%s%s</div>`, cls, icon, esc(delta))
}

// Render returns the tile as an HTML string.
func Render(s Stat) string {
	deltaRole := s.DeltaRole
	if !roleRe.MatchString(deltaRole) {
		deltaRole = "ink-muted"
	}

	var b strings.Builder
	b.WriteString(`<div class="rounded-lg border border-line bg-surface-raised p-4">`)
	fmt.Fprintf(&b, `<p class="text-sm text-ink-muted">%s</p>`, esc(s.Label))
	b.WriteString(`<div class="mt-1 flex items-end justify-between gap-3">`)

	b.WriteString(`<div class="flex items-baseline gap-2">`)
	valRole := s.ValueRole
	if !roleRe.MatchString(valRole) {
		valRole = "ink"
	}
	fmt.Fprintf(&b, `<span class="text-2xl font-semibold text-%s">%s</span>`, valRole, esc(s.Value))
	if s.Sub != "" {
		fmt.Fprintf(&b, `<span class="text-sm font-medium text-ink-muted">%s</span>`, esc(s.Sub))
	}
	if s.Delta != "" {
		fmt.Fprintf(&b, `<span class="text-xs font-medium text-%s">%s</span>`, deltaRole, esc(s.Delta))
	}
	b.WriteString(`</div>`)

	if len(s.Spark) > 0 {
		sparkRole := s.SparkRole
		if sparkRole == "" {
			sparkRole = "accent"
		}
		svg := viz.Sparkline(s.Spark, viz.SparklineOptions{
			W: 88, H: 28, StrokeRole: sparkRole, FillRole: sparkRole + "-soft",
			AriaLabel: s.Label + " trend",
		})
		b.WriteString(`<span class="shrink-0">` + svg + `</span>`)
	}

	b.WriteString(`</div></div>`)
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

// Absent returns a Stat for a metric whose instrument did not report: an
// em-dash value with the named reason as the caption. The honest-absence
// pattern (promoted from fathom's observatory, 2026-08-08): a dashboard
// must never render a fabricated zero — absence is a value with a name.
// One consumer + rationale per the ADR-0017 promotion ladder; a second
// consumer confirms it, none refutes it.
func Absent(label, reason string) Stat {
	if reason == "" {
		reason = "not measured"
	}
	return Stat{Label: label, Value: "—", ValueRole: "ink-faint", Sub: reason}
}

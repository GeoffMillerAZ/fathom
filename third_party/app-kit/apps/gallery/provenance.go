package gallery

import (
	"html/template"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

// provenance.go is the Gallery holding the kit to its own rule before the kit
// asks anyone else to follow it (ADR-0017 §6). Every figure on this page states
// where its numbers came from, including the ones invented for the specimen —
// which are labelled Estimate, because that is what they are.
//
// ADR-0012 needed no amendment for this. The Gallery is a Go app that serves
// HTML, so a page rendering a vizframe specimen IS a Gallery page. The gate
// stands as written and vizframe passes it.

type provenanceSpecimen struct {
	Heading string
	Note    string
	Frame   template.HTML
	// Code is the call that produced Frame, so a reader can copy the shape
	// rather than reverse-engineer it from the DOM.
	Code string
}

type provenanceData struct {
	Specimens []provenanceSpecimen
	Refused   template.HTML
	RefusedGo string
}

// sampleFigure is the same chart under every frame on this page, so the only
// thing varying between specimens is the provenance treatment.
func sampleFigure(label string) string {
	return viz.Bars([]viz.Bar{
		{Label: "platform", Value: 48, Role: "accent", Suffix: "%"},
		{Label: "data", Value: 31, Role: "info", Suffix: "%"},
		{Label: "security", Value: 21, Role: "positive", Suffix: "%"},
	}, viz.BarsOptions{W: 320, Max: 100, ShowValue: true, AriaLabel: label})
}

// buildProvenance assembles the Provenance page: all three enforcement
// strengths, every Kind label, the draft flag, and the CarriesMechanism waiver.
func buildProvenance() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }

	specimens := []provenanceSpecimen{
		{
			Heading: "Published — someone else put this number in public",
			Note:    "The strongest kind. Text names the publication precisely enough to find it again; Linked makes it one click.",
			Frame: h(vizframe.MustRender(sampleFigure("spend share by team"), vizframe.Options{
				ID:     "prov-published",
				Title:  "Cloud spend share by team",
				Note:   "FY25, all environments",
				Source: vizframe.Published("CNCF Annual Survey 2024, table 6.").Linked("https://www.cncf.io/reports/"),
			})),
			Code: `vizframe.MustRender(svg, vizframe.Options{
    Title:  "Cloud spend share by team",
    Source: vizframe.Published("CNCF Annual Survey 2024, table 6.").
                Linked("https://www.cncf.io/reports/"),
})`,
		},
		{
			Heading: "Derived — arithmetic the reader could redo",
			Note:    "Name the inputs and where the working is, not just the conclusion. \"Calculated internally\" is not a derivation.",
			Frame: h(vizframe.MustRender(sampleFigure("spend share by team"), vizframe.Options{
				ID:     "prov-derived",
				Title:  "Cloud spend share by team",
				Note:   "FY25, all environments",
				Source: vizframe.Derived("Sum of the FY25 billing export grouped by owner tag; arithmetic in docs/spend.md."),
			})),
			Code: `Source: vizframe.Derived("Sum of the FY25 billing export grouped by " +
    "owner tag; arithmetic in docs/spend.md.")`,
		},
		{
			Heading: "Estimate — the author's own figure, and the frame says so",
			Note:    "Not a loophole; the honest option. The frame prints the word Estimate itself, so the admission cannot be buried in prose.",
			Frame: h(vizframe.MustRender(sampleFigure("spend share by team"), vizframe.Options{
				ID:     "prov-estimate",
				Title:  "Cloud spend share by team",
				Note:   "FY25, all environments",
				Source: vizframe.Estimate("Author's apportionment from three sampled months; no billing export behind it."),
			})),
			Code: `Source: vizframe.Estimate("Author's apportionment from three sampled " +
    "months; no billing export behind it.")`,
		},
		{
			Heading: "Draft — an additional warning, never a substitute",
			Note:    "A draft figure still has to say what it is a draft of. Drafted() adds the badge and the dashed border; it does not waive the source line.",
			Frame: h(vizframe.MustRender(sampleFigure("spend share by team"), vizframe.Options{
				ID:     "prov-draft",
				Title:  "Cloud spend share by team",
				Note:   "FY25, all environments",
				Source: vizframe.Estimate("Author's apportionment from three sampled months; not yet reconciled.").Drafted(),
			})),
			Code: `Source: vizframe.Estimate("…not yet reconciled.").Drafted()`,
		},
		{
			Heading: "CarriesMechanism — the waiver you have to write down",
			Note:    "A diagram that states no quantity is answerable by reading the thing it describes. Demanding a source line for it produces a ritual line, and a ritual line trains the reader to skip the source row. Silence is strict; this escape is visible in a diff, in a grep, and in the DOM as data-viz-carries=\"mechanism\".",
			Frame: h(vizframe.MustRender(
				viz.Steps([]viz.Step{
					{Label: "ingest", State: viz.StepDone},
					{Label: "validate", State: viz.StepDone},
					{Label: "index", State: viz.StepCurrent},
					{Label: "serve", State: viz.StepUpcoming},
				}, viz.StepsOptions{W: 460, AriaLabel: "pipeline stages"}),
				vizframe.Options{
					ID:      "prov-mechanism",
					Title:   "Corpus pipeline",
					Note:    "How the stages connect — no quantity stated",
					Carries: vizframe.CarriesMechanism,
				},
			)),
			Code: `vizframe.MustRender(svg, vizframe.Options{
    Title:   "Corpus pipeline",
    Carries: vizframe.CarriesMechanism,   // states no quantity
})`,
		},
	}

	// The refusal, rendered as itself: Render returns "" for a figure that
	// carries a quantity and names no source. Showing the empty result is the
	// point — a reader has to see that the blank is the mechanism working, not
	// a broken page.
	refused := vizframe.Render(sampleFigure("spend share by team"), vizframe.Options{Title: "Unsourced figure"})

	return htmlf("provenance", provenanceHTML, provenanceData{
		Specimens: specimens,
		Refused:   template.HTML(refused),
		RefusedGo: `svg  := viz.Bars(rows, viz.BarsOptions{})
html := vizframe.Render(svg, vizframe.Options{Title: "Unsourced figure"})
// html == ""   — Validate() returns ErrMissingSource

// The same call, three strengths:
o.Validate()              // error  — wire into your own test or lint
vizframe.Render(svg, o)   // ""     — refuses rather than lie
vizframe.MustRender(svg, o) // panic — dev builds and golden tests`,
	})
}

const provenanceHTML = `
<h1 class="text-2xl font-semibold text-ink">Provenance</h1>
<p class="mt-2 max-w-3xl text-ink-muted">A chart is a claim, and an unsourced claim is worth less than no claim — it invites the reader to wonder what else was drawn rather than measured. <code class="font-mono text-sm text-ink">appkit/ui/vizframe</code> makes the source line structural: <code class="font-mono text-sm text-ink">Render</code> has no code path that emits a figure without saying where it came from.</p>
<p class="mt-3 max-w-3xl text-ink-muted">It is a <strong class="text-ink">sibling</strong> package (ADR-0017 §6). It imports nothing from <code class="font-mono text-sm text-ink">ui/viz</code>, <code class="font-mono text-sm text-ink">ui/viz</code> imports nothing from it, and zero of viz's call sites changed. The kit ships the rule; the app picks the enforcement.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Three strengths, one predicate</h2>
<div class="mt-4 rounded-lg border border-line bg-surface-sunken p-4">
  <pre class="overflow-x-auto font-mono text-xs leading-relaxed text-ink"><code>{{.RefusedGo}}</code></pre>
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Every kind of source the frame recognises</h2>
<p class="mt-2 max-w-3xl text-ink-muted">The vocabulary is deliberately closed and deliberately small: these exhaust the ways a number on a chart can be true, and a fourth would be a way of avoiding the question. The labels are not caller-overridable — an app that could relabel <em>Estimate</em> as <em>Verified</em> would make the rule decorative.</p>

{{range .Specimens}}
<section class="mt-8">
  <h3 class="text-base font-medium text-ink">{{.Heading}}</h3>
  <p class="mt-1 max-w-3xl text-sm text-ink-muted">{{.Note}}</p>
  <div class="mt-3 grid gap-4 lg:grid-cols-2">
    <div>{{.Frame}}</div>
    <div class="rounded-lg border border-line bg-surface-sunken p-4">
      <pre class="overflow-x-auto font-mono text-xs leading-relaxed text-ink"><code>{{.Code}}</code></pre>
    </div>
  </div>
</section>
{{end}}

<h2 class="mt-10 text-lg font-medium text-ink">What refusal looks like</h2>
<p class="mt-2 max-w-3xl text-ink-muted">Below is a real <code class="font-mono text-sm text-ink">Render</code> call for a figure that states quantities and names no source. The frame between these rules is the entire output: the blank is honest — the figure genuinely should not be seen — and it cannot be mistaken for a real chart. <code class="font-mono text-sm text-ink">MustRender</code> is the development-time loudness; the page-level coverage test is what stops a blank shipping unnoticed.</p>
<div class="mt-4 border-y border-dashed border-line py-6">{{.Refused}}</div>
<p class="mt-2 text-xs text-ink-muted">↑ Nothing between those rules. That is the mechanism working.</p>

<h2 class="mt-10 text-lg font-medium text-ink">The kit holds itself to it first</h2>
<p class="mt-2 max-w-3xl text-ink-muted">Every <code class="font-mono text-sm text-ink">&lt;figure class="ak-vizframe"&gt;</code> on every Gallery route carries a source line, asserted by <code class="font-mono text-sm text-ink">TestEveryGalleryFigureCarriesASource</code>. That is a property of a <em>page</em> rather than of a call, so only a test can check it — and it is the answer to the one real objection to returning <code class="font-mono text-sm text-ink">""</code>, which is that a blank region is invisible to review.</p>
`

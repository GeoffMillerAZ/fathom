// Package vizframe is a PROTOTYPE of the provenance rule for kit charts: the
// frame a figure is presented in, carrying a mandatory source line. It is not
// yet wired into appkit/ui/viz — ADR-0017 decides whether and how it is
// adopted. Nothing in appkit/ui/viz imports this package, and this package
// imports nothing from appkit/ui/viz.
//
// # The rule
//
// A chart is a claim. An unsourced claim is worth less than no claim, because
// it invites the reader to wonder what else was drawn rather than measured. So
// a frame that carries a figure must name where the figure came from, and the
// naming is structural rather than advisory: Render has no code path that emits
// a number without the line saying where it came from.
//
// Provenance is one of exactly three things (see Kind): a published source, a
// reproducible derivation, or an explicit own-estimate label. The third is not
// a loophole — it is the honest option, and the frame prints the word
// "Estimate" itself so the admission cannot be buried in prose.
//
// # Three enforcement strengths, one predicate
//
// The kit guides, it does not dictate (see docs/design/provenance-rule.md).
// The same check is therefore exposed at three strengths and the consumer picks:
//
//	Options.Validate() error   // inspect — wire into your own test or lint
//	Render(body, o) string     // guard  — returns "" rather than lie
//	MustRender(body, o) string // panic  — for dev builds and golden tests
//
// # Consumption
//
// body is TRUSTED markup — typically the string returned by an appkit/ui/viz
// component — and is embedded verbatim. Every Options string is escaped here,
// so callers never escape:
//
//	svg := viz.Bars(rows, viz.BarsOptions{AriaLabel: "spend by team"})
//	html := vizframe.Render(svg, vizframe.Options{
//	    Title:  "Spend by team",
//	    Source: vizframe.Derived("Sum of line items in the FY25 export; arithmetic in docs/spend.md."),
//	})
//
// # Theming, determinism
//
// Only kit role utilities — no literal colors, so the frame re-themes light and
// dark with the token CSS (D7). Same inputs produce byte-identical output: no
// time.Now, no rand, no map iteration in any output path. This is an L3b
// package (ADR-0017 §1): it emits Tailwind role utilities, so a consumer must
// run appkit css with an @source glob over this package's *.go.
//
// # ink-faint is banned in this package (hazard H6)
//
// A caveat set quieter than the claim it qualifies is dishonest. The source
// line is set in ink-muted and never in ink-faint, measured 2026-08-07 against
// the kit token set on this frame's own bg-surface-raised ground:
//
//	ink        17.46:1 light / 14.73:1 dark
//	ink-muted   7.33:1 light /  6.78:1 dark
//	ink-faint   3.89:1 light /  3.73:1 dark   ← below tokens.TextFloor (4.5)
//
// Smaller is fine — the size difference is a legible hierarchy at any contrast.
// Fainter is not: it steers the reader toward the claim and away from its
// qualification, by typography rather than by words, which is precisely the
// failure this package exists to prevent. A provenance line the reader skips is
// worse than no provenance line, because it also confers the appearance of
// rigour. TestSourceLineNeverUsesInkFaint asserts it; the full entry is
// docs/theming-hazards.md §H6.
package vizframe

import (
	"errors"
	"fmt"
	"html"
	"strings"
)

// Kind classifies why a figure can be believed. The vocabulary is deliberately
// closed and deliberately small: these three exhaust the ways a number on a
// chart can be true, and a fourth would be a way of avoiding the question.
type Kind string

const (
	// KindPublished — someone other than the author put this number in public.
	// Text names the publication precisely enough to find it again.
	KindPublished Kind = "published"
	// KindDerived — arithmetic the reader could redo. Text names the inputs and
	// where the working is, not just the conclusion.
	KindDerived Kind = "derived"
	// KindEstimate — the author's own figure, unpublished and underived. Legal
	// and often correct; the frame labels it so it is never mistaken for
	// either of the above.
	KindEstimate Kind = "estimate"
)

// kindLabel is the word printed at the head of the source line. It is not
// caller-overridable on purpose: an app that could rename "Estimate" to
// "Verified" would defeat the rule. (Open question for ADR-0017: this makes the
// frame English-only.)
func kindLabel(k Kind) string {
	switch k {
	case KindPublished:
		return "Source"
	case KindDerived:
		return "Derived"
	case KindEstimate:
		return "Estimate"
	}
	return ""
}

// Source is the provenance line printed under a figure. The zero value is
// invalid, which is the point: a Source you forgot to fill in cannot render.
type Source struct {
	// Kind is one of KindPublished, KindDerived, KindEstimate. Required.
	Kind Kind
	// Text is the sentence a reader checks the claim against — the publication,
	// the derivation, or what the estimate is based on. Required.
	Text string
	// Href optionally links Text. Only relative, fragment, http, https and
	// mailto targets are emitted; anything else is dropped rather than
	// rendered, so a bad link degrades to plain text. Optional.
	Href string
	// Draft marks the figure as a placeholder — a number that has not been
	// verified yet. It is an ADDITIONAL warning, never a substitute for
	// provenance: a draft figure still has to say what it is a draft of.
	Draft bool
}

// Published returns a Source citing a published figure.
func Published(text string) Source { return Source{Kind: KindPublished, Text: text} }

// Derived returns a Source citing reproducible arithmetic.
func Derived(text string) Source { return Source{Kind: KindDerived, Text: text} }

// Estimate returns a Source labelled as the author's own estimate.
func Estimate(text string) Source { return Source{Kind: KindEstimate, Text: text} }

// Drafted returns a copy of s marked as a draft/placeholder figure.
//
//	vizframe.Estimate("Author's estimate; no measurement behind it yet.").Drafted()
func (s Source) Drafted() Source { s.Draft = true; return s }

// Linked returns a copy of s whose text links to href.
func (s Source) Linked(href string) Source { s.Href = href; return s }

// Carries says what the frame's body contains. It is the distinction the
// résumé site found load-bearing: a chart asserting a QUANTITY invites
// comparison with the world and must be checkable, while a mechanism diagram
// asserts only "this is how the thing is wired" and is answerable by reading
// the thing. The strict case is the zero value, so silence means strict.
type Carries string

const (
	// CarriesFigure — the body states at least one quantity. Provenance
	// required. Default.
	CarriesFigure Carries = "figure"
	// CarriesMechanism — the body states no quantity: a flow, a topology, a
	// state machine. Provenance optional (and still rendered if supplied).
	CarriesMechanism Carries = "mechanism"
)

// Options configures a Render. The zero value is a valid struct but not a valid
// frame: with no Source and the default CarriesFigure, Render refuses.
type Options struct {
	// Title is the figure's headline. Optional but strongly encouraged.
	Title string
	// Note is a one-line standfirst under the title. Optional.
	Note string
	// Source is the provenance line. Required unless Carries is
	// CarriesMechanism.
	Source Source
	// Carries says whether the body states a quantity. Default CarriesFigure.
	Carries Carries
	// DraftLabel is the badge text on a draft figure. Default "Draft".
	DraftLabel string
	// ID, when set, is placed on the <figure> and used to wire the source line
	// to it with aria-describedby. Empty emits no ids (the kit generates no
	// identifiers — determinism, D7). Optional.
	ID string
	// AriaLabel names the figure for assistive tech when Title is empty. When
	// Title is set the caption already names it and this is ignored. Optional.
	AriaLabel string
}

func (o *Options) withDefaults() Options {
	out := *o
	if out.Carries == "" {
		out.Carries = CarriesFigure
	}
	if out.DraftLabel == "" {
		out.DraftLabel = "Draft"
	}
	return out
}

// Provenance violations. Each is a distinct sentence because each is a distinct
// mistake, and the message is what a consumer sees in a failing test.
var (
	// ErrMissingSource — the frame carries a figure and named no source.
	ErrMissingSource = errors.New("vizframe: a frame carrying a figure must name its source (published, derived, or an explicit estimate)")
	// ErrMissingText — a kind was chosen but the sentence is empty.
	ErrMissingText = errors.New("vizframe: source kind was set but Text is empty — the reader needs the sentence, not the category")
	// ErrUnknownKind — Text was supplied under no recognised kind.
	ErrUnknownKind = errors.New("vizframe: source kind must be one of published, derived, estimate")
)

// Validate reports whether o satisfies the provenance rule, returning the
// specific violation. This is the whole predicate; Render and MustRender are
// two reactions to it. Wire it into a table test to get CI enforcement without
// accepting the runtime guard, or call it yourself to fail your own way — the
// kit owns no logger any more than it owns a transport (D5).
func (o Options) Validate() error {
	opts := o.withDefaults()
	s := opts.Source
	hasKind := kindLabel(s.Kind) != ""
	hasText := strings.TrimSpace(s.Text) != ""

	switch {
	case hasKind && !hasText:
		return ErrMissingText
	case !hasKind && hasText:
		return ErrUnknownKind
	case !hasKind && !hasText:
		// No provenance at all. Only a mechanism diagram may say nothing.
		if opts.Carries == CarriesMechanism {
			return nil
		}
		return ErrMissingSource
	}
	return nil
}

// Render returns the framed figure, or "" if o fails Validate.
//
// Returning "" is the Go form of the résumé site's `return null`: a figure with
// no provenance does not appear at all, rather than appearing and quietly
// asking to be trusted. The absence is loud in the page and silent in the log
// on purpose — use MustRender in development, or Validate in a test, to find
// out which frame vanished and why.
func Render(body string, o Options) string {
	if err := o.Validate(); err != nil {
		return ""
	}
	return render(body, o.withDefaults())
}

// MustRender is Render but panics on a provenance violation. Intended for
// development builds, golden tests, and any app that would rather fail loudly
// at startup than ship a blank panel.
func MustRender(body string, o Options) string {
	if err := o.Validate(); err != nil {
		panic(err)
	}
	return render(body, o.withDefaults())
}

// render assumes opts has already passed Validate and withDefaults.
func render(body string, opts Options) string {
	var b strings.Builder

	frameCls := "rounded-lg border border-line bg-surface-raised p-4"
	if opts.Source.Draft {
		frameCls = "rounded-lg border border-dashed border-caution bg-surface-raised p-4"
	}
	b.WriteString(`<figure class="ak-vizframe ` + frameCls + `"`)
	if opts.ID != "" {
		fmt.Fprintf(&b, ` id=%q`, esc(opts.ID))
	}
	// Data attributes, so an app can restyle or audit the frame entirely from
	// CSS or the DOM without forking this Go (precedent: chat's data-cite-kind).
	fmt.Fprintf(&b, ` data-viz-carries=%q`, esc(string(opts.Carries)))
	if opts.Source.Kind != "" {
		fmt.Fprintf(&b, ` data-viz-source=%q`, esc(string(opts.Source.Kind)))
	}
	if opts.Source.Draft {
		b.WriteString(` data-viz-draft="true"`)
	}
	if opts.Title == "" && opts.AriaLabel != "" {
		fmt.Fprintf(&b, ` aria-label=%q`, esc(opts.AriaLabel))
	}
	if opts.ID != "" && opts.Source.Text != "" {
		fmt.Fprintf(&b, ` aria-describedby=%q`, esc(opts.ID)+"-src")
	}
	b.WriteString(`>`)

	// Head: title, standfirst, draft badge. figcaption first is valid HTML and
	// keeps the source line visually last, where the site trained readers to
	// look for it.
	if opts.Title != "" || opts.Note != "" || opts.Source.Draft {
		b.WriteString(`<figcaption class="mb-3 flex flex-wrap items-baseline gap-x-3 gap-y-1">`)
		if opts.Title != "" {
			fmt.Fprintf(&b, `<span class="text-sm font-semibold text-ink">%s</span>`, esc(opts.Title))
		}
		if opts.Note != "" {
			fmt.Fprintf(&b, `<span class="text-xs text-ink-muted">%s</span>`, esc(opts.Note))
		}
		if opts.Source.Draft {
			fmt.Fprintf(&b,
				`<span class="inline-flex items-center rounded-full border border-dashed border-caution bg-caution-soft px-2 py-0.5 font-mono text-[0.625rem] uppercase tracking-wider text-caution" title="Draft — a placeholder figure, not a verified number">%s</span>`,
				esc(opts.DraftLabel))
		}
		b.WriteString(`</figcaption>`)
	}

	b.WriteString(`<div class="ak-vizframe-body">`)
	b.WriteString(body) // trusted markup, per the package doc
	b.WriteString(`</div>`)

	if opts.Source.Text != "" {
		// ink-muted, never ink-faint: hazard H6 (docs/theming-hazards.md). See
		// the package doc — the source line is smaller than the figure, and no
		// fainter.
		b.WriteString(`<p class="mt-3 border-t border-line pt-2 font-mono text-[0.625rem] leading-relaxed text-ink-muted"`)
		if opts.ID != "" {
			fmt.Fprintf(&b, ` id=%q`, esc(opts.ID)+"-src")
		}
		b.WriteString(`>`)
		fmt.Fprintf(&b, `<span class="uppercase tracking-wider text-ink-muted">%s</span> `,
			esc(kindLabel(opts.Source.Kind)))
		b.WriteString(sourceText(opts.Source))
		b.WriteString(`</p>`)
	}

	b.WriteString(`</figure>`)
	return b.String()
}

// sourceText renders the provenance sentence, linked if the href is one the
// frame is willing to emit.
func sourceText(s Source) string {
	text := esc(strings.TrimSpace(s.Text))
	href := safeHref(s.Href)
	if href == "" {
		return text
	}
	return fmt.Sprintf(`<a class="underline decoration-line underline-offset-2 hover:text-ink" href=%q>%s</a>`, href, text)
}

// safeHref returns an escaped href, or "" if the target is not one of the
// schemes the frame emits. A dropped link degrades to plain text — the sentence
// still renders, which matters more than the link.
func safeHref(href string) string {
	h := strings.TrimSpace(href)
	if h == "" {
		return ""
	}
	lower := strings.ToLower(h)
	switch {
	case strings.HasPrefix(h, "/"), strings.HasPrefix(h, "#"), strings.HasPrefix(h, "./"), strings.HasPrefix(h, "../"):
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "mailto:"):
	default:
		return ""
	}
	return esc(h)
}

func esc(s string) string { return html.EscapeString(s) }

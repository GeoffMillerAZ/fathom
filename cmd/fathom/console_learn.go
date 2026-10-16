package main

// console_learn.go — the Learn group (vision 35 §3, design §3 Learn). This
// file holds what the group's pages share — the chosen project and its
// picker — and the guides (OC-48): the use-case guides fathom ships, the
// glossary and the laws, each rendered as prose (the kit's docs prose, L1)
// beside a table of contents (docs.RenderToCRail from xl up,
// docs.RenderToCPanel below it). How fathom works is
// console_learn_pipeline.go (OC-46); Ask the right way is
// console_learn_ask.go (OC-47).
//
// The documents are the binary's own (docs/fathomdocs, go:embed), parsed by
// internal/console (guides.go): a guide describes the build serving it, and
// the glossary and the laws are read from the intent corpus, never copied.
// A guide renders a document, not a verb, so its pages name no verb chip;
// each names the embedded file it renders instead.

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/a-h/templ"
	fathomdocs "github.com/geoffmilleraz/fathom/docs"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/docs"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func init() {
	declarePage(consolePage{
		ID: "learn-guides", Route: "/learn/guides", Href: "/learn/guides", Group: "Learn", Order: 20, Nav: true,
		Title:    "Guides and glossary",
		Question: "How do I put fathom to work on a task, what do its words mean, and which laws does it keep?",
		Build:    buildLearnGuidesPage,
	})
	declarePage(consolePage{
		ID: "learn-guide", Route: "/learn/guides/{slug}", Group: "Learn", Order: 21, NavParent: "learn-guides",
		Title:    "Guide",
		Question: "How do I put fathom to work on this task, step by step?",
		Build:    buildLearnGuidePage,
	})
}

// ─── the chosen project ─────────────────────────────────────────────────────

// learnChoice is the project a Learn page draws and how it was chosen.
type learnChoice struct {
	Service string
	Row     obsServiceRow
	How     string
	Known   bool
}

// chooseLearnProject reads ?svc=: a named project must be indexed (an
// unknown name is the shell's 404 with the index remedy); none named draws
// the most-queried project in the call log — the one a reader most likely
// works in — with ties and an unread log falling back to name order.
func chooseLearnProject(cr *consoleRequest, m observatoryModel) (learnChoice, error) {
	want := strings.TrimSpace(cr.r.URL.Query().Get("svc"))
	if want != "" {
		for _, s := range m.Services {
			if s.Service == want {
				return learnChoice{Service: want, Row: s, How: "chosen", Known: true}, nil
			}
		}
		return learnChoice{}, unknownProjectError(want, m, cr.deps.selfURL)
	}
	if len(m.Services) == 0 {
		how := "no project is indexed"
		if m.DegradedNote != "" {
			how = "the service registry could not be read"
		}
		return learnChoice{How: how}, nil
	}
	rows := append([]obsServiceRow(nil), m.Services...)
	logRead := console.CallTotal(modelCallFacts(m)).Known()
	sort.SliceStable(rows, func(i, j int) bool {
		if logRead && rows[i].CallsTotal != rows[j].CallsTotal {
			return rows[i].CallsTotal > rows[j].CallsTotal
		}
		return rows[i].Service < rows[j].Service
	})
	how := "the most-queried project in the call log"
	if !logRead {
		how = "the first project by name — the call log was not read"
	}
	return learnChoice{Service: rows[0].Service, Row: rows[0], How: how, Known: true}, nil
}

func learnProjectNames(m observatoryModel) []string {
	names := make([]string, 0, len(m.Services))
	for _, s := range m.Services {
		names = append(names, s.Service)
	}
	sort.Strings(names)
	return names
}

// learnMeta is the heading's meta line: which project, why it, and how many
// there are to pick from (the shared fleet.services metric).
func learnMeta(choice learnChoice, services console.Value[int], m observatoryModel) []string {
	var meta []string
	if choice.Service != "" {
		meta = append(meta, "project "+choice.Service+" ("+choice.How+")")
	} else {
		meta = append(meta, "no project drawn: "+choice.How)
	}
	meta = append(meta, "one of "+valuePlain(services)+" indexed projects")
	return append(meta, modelMeta(m)...)
}

// learnProjectPicker is the heading's project switch: a GET form, so it
// works without JavaScript and the choice is the URL.
func learnProjectPicker(action string, choice learnChoice, m observatoryModel) string {
	names := learnProjectNames(m)
	if len(names) == 0 {
		return ""
	}
	opts := make([]form.Option, 0, len(names))
	for _, n := range names {
		opts = append(opts, form.Option{Value: n, Label: n})
	}
	sel := form.RenderSelect(form.Select{ID: "learn-project", Name: "svc", Value: choice.Service, Options: opts})
	return `<form method="get" action="` + html.EscapeString(action) + `" class="flex min-w-0 flex-wrap items-end gap-2 [&_button]:min-h-11 [&_select]:min-h-11" data-learn-project>` +
		form.RenderField(form.Field{Label: "Project", For: "learn-project", Control: template.HTML(sel)}) +
		form.RenderButton(form.Button{Label: "Redraw", Variant: form.ButtonGhost, Type: "submit"}) +
		`</form>`
}

// ─── the guide library ──────────────────────────────────────────────────────

// guideLibrary is every guide the binary carries, parsed once.
type guideLibrary struct {
	entries  []console.GuideEntry
	markdown map[string][]byte
	terms    []console.GlossaryTerm
	laws     []console.Law
	// problems are the embedded documents that could not be read or parsed
	// (a build without them is a defect, named on the index, never hidden).
	problems []string
}

var (
	guideLibOnce sync.Once
	guideLib     guideLibrary
)

// learnGuides loads the library from the embedded documents.
func learnGuides() guideLibrary {
	guideLibOnce.Do(func() { guideLib = loadGuideLibrary(fathomdocs.FS) })
	return guideLib
}

// loadGuideLibrary builds the library from the documents (paths relative to
// docs/): the README's index first (its order, its descriptions), then any
// use-case file the README does not list, then the glossary and the laws.
func loadGuideLibrary(fsys fs.FS) guideLibrary {
	lib := guideLibrary{markdown: map[string][]byte{}}
	read := func(name string) ([]byte, error) { return fs.ReadFile(fsys, name) }
	readme, err := read("use-cases/README.md")
	if err != nil {
		lib.problems = append(lib.problems, "docs/use-cases/README.md: "+err.Error())
	}
	listed := map[string]bool{}
	for _, e := range console.ParseGuideIndex(readme) {
		md, err := read("use-cases/" + e.Slug + ".md")
		if err != nil {
			lib.problems = append(lib.problems, e.Source+": "+err.Error())
			continue
		}
		if t := console.GuideTitle(md); t != "" {
			e.Title = t
		}
		listed[e.Slug] = true
		lib.entries = append(lib.entries, e)
		lib.markdown[e.Slug] = md
	}
	if names, err := fs.ReadDir(fsys, "use-cases"); err == nil {
		var extra []string
		for _, n := range names {
			slug := strings.TrimSuffix(n.Name(), ".md")
			if n.IsDir() || !strings.HasSuffix(n.Name(), ".md") || n.Name() == "README.md" || listed[slug] || !console.ValidGuideSlug(slug) {
				continue
			}
			extra = append(extra, slug)
		}
		sort.Strings(extra)
		for _, slug := range extra {
			md, err := read("use-cases/" + slug + ".md")
			if err != nil {
				continue
			}
			title := console.GuideTitle(md)
			if title == "" {
				title = slug
			}
			lib.entries = append(lib.entries, console.GuideEntry{Slug: slug, Kind: console.GuideUseCase, Title: title, Source: "docs/use-cases/" + slug + ".md",
				Description: "not listed in the use-case index (docs/use-cases/README.md)"})
			lib.markdown[slug] = md
		}
	}
	if domain, err := read("intent/domain.md"); err != nil {
		lib.problems = append(lib.problems, "docs/intent/domain.md: "+err.Error())
	} else if lib.terms = console.ParseGlossary(domain); len(lib.terms) == 0 {
		lib.problems = append(lib.problems, "docs/intent/domain.md: no Vocabulary terms could be read")
	} else {
		lib.entries = append(lib.entries, console.GuideEntry{Slug: console.GlossarySlug, Kind: console.GuideGlossary, Title: "Glossary",
			Description: fmt.Sprintf("What fathom's own words mean — %d terms from the intent corpus", len(lib.terms)), Source: "docs/intent/domain.md"})
		lib.markdown[console.GlossarySlug] = console.GlossaryMarkdown(lib.terms)
	}
	if principles, err := read("intent/principles.md"); err != nil {
		lib.problems = append(lib.problems, "docs/intent/principles.md: "+err.Error())
	} else if lib.laws = console.ParseLaws(principles); len(lib.laws) == 0 {
		lib.problems = append(lib.problems, "docs/intent/principles.md: no laws could be read")
	} else {
		lib.entries = append(lib.entries, console.GuideEntry{Slug: console.LawsSlug, Kind: console.GuideLaws, Title: "The laws",
			Description: fmt.Sprintf("The %d laws fathom keeps, which pages cite by number", len(lib.laws)), Source: "docs/intent/principles.md"})
		lib.markdown[console.LawsSlug] = console.LawsMarkdown(lib.laws)
	}
	return lib
}

func (lib guideLibrary) entry(slug string) (console.GuideEntry, bool) {
	for _, e := range lib.entries {
		if e.Slug == slug {
			return e, true
		}
	}
	return console.GuideEntry{}, false
}

// ─── the index ──────────────────────────────────────────────────────────────

// guidesModel is /learn/guides's JSON projection.
type guidesModel struct {
	Guides   []console.GuideEntry   `json:"guides"`
	Glossary []console.GlossaryTerm `json:"glossary"`
	Laws     []console.Law          `json:"laws"`
	Problems []string               `json:"problems,omitempty"`
}

func buildLearnGuidesPage(cr *consoleRequest) (consoleOut, error) {
	lib := learnGuides()
	return consoleOut{
		Meta:  []string{"rendered from the documents built into this binary (fathom " + fathomVersionLabel() + ")"},
		Model: guidesModel{Guides: lib.entries, Glossary: lib.terms, Laws: lib.laws, Problems: lib.problems},
		Body:  func() string { return renderGuidesIndex(lib) },
	}, nil
}

func renderGuidesIndex(lib guideLibrary) string {
	var b strings.Builder
	if len(lib.problems) > 0 {
		b.WriteString(renderPanel(consolePanel{
			ID: "guide-problems", SourceKey: "daemon", Title: "Documents that could not be read",
			State: console.StateError, Reason: strings.Join(lib.problems, "; "),
			Source: "the documents built into this binary (docs/fathomdocs)",
		}))
	}
	rows := make([]table.Row, 0, len(lib.entries))
	for _, e := range lib.entries {
		if e.Kind != console.GuideUseCase {
			continue
		}
		link := `<a href="` + html.EscapeString(guideHref(e.Slug)) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(e.Title) + `</a>`
		rows = append(rows, table.Row{
			ID: "guide-" + e.Slug, Cells: []string{e.Title, e.Description},
			HTMLCells: []templ.Component{templ.Raw(link), templ.Raw(`<span class="[overflow-wrap:anywhere]">` + html.EscapeString(e.Description) + `</span>`)},
		})
	}
	useCases := len(rows)
	b.WriteString(renderPanel(consolePanel{
		ID: "guides", SourceKey: "daemon", Title: "Use-case guides",
		Question:    "Which recipe puts fathom to work on the task in front of me?",
		Source:      "docs/use-cases/README.md and the guides it lists, built into this binary",
		Denominator: plural(useCases, "guide"),
		Body: renderKitTable(table.TableParams{
			TableID: "learn-guides", Caption: "Each guide is one use case: the tools and verbs it takes, a worked example, and setup notes",
			Columns: []table.Column{{Key: "guide", Label: "Guide"}, {Key: "covers", Label: "What it covers"}},
			Rows:    rows,
		}),
	}))

	var terms strings.Builder
	if len(lib.terms) > 0 {
		terms.WriteString(`<ul class="flex flex-wrap gap-x-1 gap-y-0" aria-label="Terms">`)
		for i, t := range lib.terms {
			terms.WriteString(`<li><a href="` + html.EscapeString(guideHref(console.GlossarySlug)) + `#term-` + itoa(i+1) + `" class="inline-flex min-h-11 items-center rounded-sm px-1 text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(console.PlainInline(t.Term)) + `</a></li>`)
		}
		terms.WriteString(`</ul>`)
	}
	glossary := consolePanel{
		ID: "glossary", SourceKey: "daemon", Title: "Glossary",
		Question:    "What does fathom mean by a claim, a layer, a service, an arm?",
		Source:      "docs/intent/domain.md (Vocabulary), built into this binary",
		Denominator: plural(len(lib.terms), "term"),
		Body:        terms.String(),
	}
	if len(lib.terms) == 0 {
		glossary.State, glossary.Reason = console.StateError, "the vocabulary could not be read from the embedded intent corpus"
	}
	var laws strings.Builder
	if len(lib.laws) > 0 {
		laws.WriteString(`<ol class="space-y-0" aria-label="The laws">`)
		for _, l := range lib.laws {
			laws.WriteString(`<li class="[overflow-wrap:anywhere]"><a href="` + html.EscapeString(guideHref(console.LawsSlug)) + `#law-` + itoa(l.Number) + `" class="inline-flex min-h-11 items-center gap-2 text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"><span class="font-mono text-xs text-ink-muted">law ` + itoa(l.Number) + `</span>` + html.EscapeString(l.Title) + `</a></li>`)
		}
		laws.WriteString(`</ol>`)
	}
	lawsPanel := consolePanel{
		ID: "laws", SourceKey: "daemon", Title: "The laws",
		Question:    "Which rules does fathom hold itself to, when a page cites \"law 3\"?",
		Source:      "docs/intent/principles.md (P1–P15 carry laws 1–15), built into this binary",
		Denominator: plural(len(lib.laws), "law"),
		Body:        laws.String(),
	}
	if len(lib.laws) == 0 {
		lawsPanel.State, lawsPanel.Reason = console.StateError, "the laws could not be read from the embedded intent corpus"
	}
	b.WriteString(`<div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">` + renderPanel(glossary) + renderPanel(lawsPanel) + `</div>`)
	return b.String()
}

// ─── one guide ──────────────────────────────────────────────────────────────

// guideModel is /learn/guides/{slug}'s JSON projection.
type guideModel struct {
	console.GuideEntry
	Headings []docs.Heading `json:"headings"`
	Markdown string         `json:"markdown"`
}

func buildLearnGuidePage(cr *consoleRequest) (consoleOut, error) {
	slug := cr.r.PathValue("slug")
	lib := learnGuides()
	e, ok := lib.entry(slug)
	if !ok || !console.ValidGuideSlug(slug) {
		var names []string
		for _, g := range lib.entries {
			names = append(names, g.Slug)
		}
		return consoleOut{}, &consoleError{
			Status: http.StatusNotFound, Title: "Unknown guide",
			Reason:      "No guide is named " + slug + ". The guides are: " + strings.Join(names, ", ") + ".",
			Remedy:      "open the guides index and pick one",
			RemedyHref:  "/learn/guides",
			RemedyLabel: "Open the guides",
		}
	}
	md := lib.markdown[slug]
	doc := renderGuideMarkdown(md, lib)
	return consoleOut{
		Title: e.Title,
		Meta:  []string{"rendered from " + e.Source + " as built into this binary (fathom " + fathomVersionLabel() + ")"},
		Model: guideModel{GuideEntry: e, Headings: doc.headings, Markdown: string(md)},
		Body:  func() string { return renderGuidePage(e, doc, lib) },
	}, nil
}

// renderedGuide is one guide as HTML with its outline.
type renderedGuide struct {
	html     string
	headings []docs.Heading
	err      string
}

var guideMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithAttribute()),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(guideLinkRenderer{}, 100))),
)

// renderGuideMarkdown renders a guide: its title heading is dropped (the
// page's one h1 carries it), the outline is its h2 and h3 headings with the
// ids the parser gave them, and every relative link is placed — another
// guide, the glossary, the laws — or shown unresolved with the repository
// path it names, never a link that 404s.
func renderGuideMarkdown(md []byte, lib guideLibrary) renderedGuide {
	doc := guideMarkdown.Parser().Parse(text.NewReader(md))
	var out renderedGuide
	var drop []gast.Node
	seenTitle := false
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *gast.Heading:
			if t.Level == 1 && !seenTitle {
				seenTitle = true
				drop = append(drop, t)
				return gast.WalkSkipChildren, nil
			}
			if t.Level == 1 {
				t.Level = 2 // one h1 per page: a second document h1 becomes a section
			}
			id := ""
			if v, ok := t.AttributeString("id"); ok {
				if b, ok := v.([]byte); ok {
					id = string(b)
				}
			}
			if t.Level <= 3 {
				out.headings = append(out.headings, docs.Heading{Level: t.Level, Text: guideNodeText(t, md), ID: id})
			}
		case *gast.Link:
			dest, unresolved := placeGuideLink(string(t.Destination), lib)
			if unresolved != "" {
				t.SetAttributeString("data-unresolved", []byte(unresolved))
			}
			t.Destination = []byte(dest)
		}
		return gast.WalkContinue, nil
	})
	for _, n := range drop {
		if p := n.Parent(); p != nil {
			p.RemoveChild(p, n)
		}
	}
	var buf bytes.Buffer
	if err := guideMarkdown.Renderer().Render(&buf, md, doc); err != nil {
		out.err = err.Error()
	}
	// GFM tables write bare header cells; every header here heads a column.
	out.html = guideHeaderCellRE.ReplaceAllString(buf.String(), `<th scope="col"$1`)
	return out
}

var guideHeaderCellRE = regexp.MustCompile(`<th(\s|>)`)

// guideNodeText is a node's text content, inline markup flattened.
func guideNodeText(n gast.Node, src []byte) string {
	var b strings.Builder
	_ = gast.Walk(n, func(c gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *gast.Text:
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *gast.String:
			b.Write(t.Value)
		}
		return gast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// placeGuideLink maps a guide's link to a console route. External and
// in-page links stay; a link to another guide, the use-case index, the
// glossary's or the laws' source becomes their page; any other relative
// link has no page here and comes back unresolved with the repository path
// it names.
func placeGuideLink(dest string, lib guideLibrary) (string, string) {
	lower := strings.ToLower(dest)
	switch {
	case dest == "" || strings.HasPrefix(dest, "#"):
		return dest, ""
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "mailto:"):
		return dest, ""
	case strings.Contains(dest, ":") && !strings.Contains(strings.SplitN(dest, "/", 2)[0], "."):
		// Any other scheme (javascript:, data:, file:) never renders as a link.
		return "", dest
	}
	target, frag, _ := strings.Cut(dest, "#")
	repoPath := path.Clean(path.Join("docs/use-cases", target))
	suffix := ""
	if frag != "" {
		suffix = "#" + frag
	}
	switch repoPath {
	case "docs/use-cases", "docs/use-cases/README.md":
		return "/learn/guides" + suffix, ""
	case "docs/intent/domain.md":
		return guideHref(console.GlossarySlug), ""
	case "docs/intent/principles.md":
		return guideHref(console.LawsSlug), ""
	}
	if dir, file := path.Split(repoPath); dir == "docs/use-cases/" && strings.HasSuffix(file, ".md") {
		if _, ok := lib.entry(strings.TrimSuffix(file, ".md")); ok {
			return guideHref(strings.TrimSuffix(file, ".md")) + suffix, ""
		}
	}
	return "", repoPath + suffix
}

// guideLinkRenderer renders a link the guide placed as an ordinary anchor
// and one it could not place in the kit's unresolved-link treatment
// (prose.css: dotted, muted, with a marker naming the repository path) —
// visible, never silently broken.
type guideLinkRenderer struct{}

func (guideLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindLink, func(w util.BufWriter, _ []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
		link := n.(*gast.Link)
		unresolved := ""
		if v, ok := link.AttributeString("data-unresolved"); ok {
			if b, ok := v.([]byte); ok {
				unresolved = string(b)
			}
		}
		if unresolved != "" {
			if entering {
				_, _ = w.WriteString(`<span class="doc-link doc-link--unresolved" data-guide-unresolved>`)
			} else {
				_, _ = w.WriteString(` <span class="doc-link__marker">(in the repository: ` + html.EscapeString(unresolved) + `)</span></span>`)
			}
			return gast.WalkContinue, nil
		}
		if entering {
			_, _ = w.WriteString(`<a href="` + html.EscapeString(string(link.Destination)) + `">`)
		} else {
			_, _ = w.WriteString(`</a>`)
		}
		return gast.WalkContinue, nil
	})
}

// renderGuidePage lays the guide out: the prose, and beside it from xl up
// the sticky outline (docs.RenderToCRail) with the glossary and the laws;
// below xl the outline is a disclosure above the prose
// (docs.RenderToCPanel) — both render, CSS shows one.
func renderGuidePage(e console.GuideEntry, doc renderedGuide, lib guideLibrary) string {
	var b strings.Builder
	if doc.err != "" {
		b.WriteString(renderPanel(consolePanel{ID: "guide-error", SourceKey: "daemon", Title: "The guide could not be rendered", State: console.StateError, Reason: doc.err, Source: e.Source}))
	}
	related := renderGuideRelated(e, lib)
	// data-console-sticky: the page positions something sticky, so the
	// shell's main stops being a scroll container that never scrolls
	// (renderConsoleShell; FATHOM-G-KIT-04).
	b.WriteString(`<div class="grid grid-cols-1 gap-8 xl:grid-cols-[minmax(0,1fr)_16rem]" data-guide data-console-sticky>`)
	// The rail comes first in the document, so a keyboard or screen-reader
	// reader meets the outline before the prose; from xl up the grid places
	// it on the right. One sticky block holds the related links and the
	// outline, bounded by the viewport and scrolling on its own, so neither
	// scrolls out of reach on a long guide; the kit rail's own sticky is
	// made static inside it. Every entry is a 44 px target with a visible
	// focus ring (the kit rail's links are 20 px — FATHOM-G-KIT-04).
	b.WriteString(`<aside class="hidden xl:col-start-2 xl:row-start-1 xl:block" aria-label="On this page" data-guide-rail>`)
	b.WriteString(`<div class="sticky top-6 max-h-[calc(100dvh-3rem)] space-y-6 overflow-y-auto pr-1 [&_.sticky]:static [&_nav_a]:flex [&_nav_a]:min-h-11 [&_nav_a]:items-center [&_nav_a]:rounded-sm [&_nav_a:focus-visible]:outline-2 [&_nav_a:focus-visible]:outline-offset-2 [&_nav_a:focus-visible]:outline-focus" data-guide-rail-sticky>`)
	b.WriteString(related)
	b.WriteString(docs.RenderToCRail(doc.headings, docs.Options{}))
	b.WriteString(`</div></aside>`)
	b.WriteString(`<div class="min-w-0 xl:col-start-1 xl:row-start-1">`)
	if panel := docs.RenderToCPanel(doc.headings, docs.Options{}); panel != "" {
		b.WriteString(`<div class="mb-6 xl:hidden" data-guide-toc-panel>` + panel + `</div>`)
	}
	b.WriteString(`<article class="doc-prose min-w-0 [overflow-wrap:anywhere] [&_:target]:scroll-mt-4" data-guide-article>` + doc.html + `</article>`)
	b.WriteString(`<div class="mt-8 xl:hidden">` + related + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	return b.String()
}

// renderGuideRelated is the guide's companions: the glossary, the laws and
// the index — the words and rules a guide leans on, one step away.
func renderGuideRelated(e console.GuideEntry, lib guideLibrary) string {
	var b strings.Builder
	b.WriteString(`<nav aria-label="Terms, laws and every guide" class="space-y-1 text-sm" data-guide-related>`)
	b.WriteString(`<p class="text-xs font-medium uppercase tracking-wide text-ink-faint">Beside this guide</p><ul>`)
	link := func(href, label string) {
		b.WriteString(`<li><a href="` + html.EscapeString(href) + `" class="inline-flex min-h-11 items-center text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(label) + `</a></li>`)
	}
	if e.Kind != console.GuideGlossary {
		if _, ok := lib.entry(console.GlossarySlug); ok {
			link(guideHref(console.GlossarySlug), "Glossary — what fathom's words mean")
		}
	}
	if e.Kind != console.GuideLaws {
		if _, ok := lib.entry(console.LawsSlug); ok {
			link(guideHref(console.LawsSlug), "The laws fathom keeps")
		}
	}
	link("/learn/guides", "Every guide")
	link("/learn/ask", "Ask the right way")
	b.WriteString(`</ul></nav>`)
	return b.String()
}

// guideHref is a guide's route.
func guideHref(slug string) string { return "/learn/guides/" + slug }

// fathomVersionLabel names the binary for a document's provenance line.
func fathomVersionLabel() string { return buildinfo.Version() }

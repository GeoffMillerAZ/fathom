package gallery

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/callout"
	kitdocs "github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/docs"
)

const staticContentMaxQueryBytes = 4096

type staticContentState string

const (
	staticContentReady       staticContentState = "ready"
	staticContentEmpty       staticContentState = "empty"
	staticContentUnavailable staticContentState = "unavailable"
)

// staticContentModel is deliberately Gallery-owned. It demonstrates one complete
// static reading task; it is not a public documentation or FAQ framework.
type staticContentModel struct {
	Title, Intro string
	Sections     []staticContentSection
	FAQ          []staticContentFAQ
}

type staticContentSection struct {
	ID, Heading, Body string
}

type staticContentFAQ struct {
	ID, Question, Answer, Destination string
}

var staticContentDestinations = map[string]string{
	"choosing-profile": "/content/choosing-a-profile",
	"catalog":          "/catalog",
	"quality":          "/quality",
}

var staticContentReservedIDs = map[string]bool{
	"content-title":             true,
	"content-guide":             true,
	"content-faq":               true,
	"content-faq-heading":       true,
	"content-state":             true,
	"content-profile-title":     true,
	"content-runtime-fit":       true,
	"content-delivery-evidence": true,
	"content-problem-title":     true,
	"gallery-main":              true,
	"gallery-navigation":        true,
}

func staticContentGuide() staticContentModel {
	return staticContentModel{
		Title: "Choose a profile before you integrate a capability",
		Intro: "Use this short guide to decide what belongs in an application, what remains app-owned, and which proof is still needed before reuse.",
		Sections: []staticContentSection{
			{ID: "start-with-task", Heading: "Start with the task", Body: "Name the person, decision and recovery path first. A component count or an attractive reference does not establish fit."},
			{ID: "keep-policy-local", Heading: "Keep policy local", Body: "Routes, authorization, domain wording and destinations remain the application’s responsibility. The kit supplies bounded presentation and interaction building blocks."},
		},
		FAQ: []staticContentFAQ{
			{ID: "profile", Question: "Which profile should I choose?", Answer: "Choose the smallest runtime profile that completes the named task. Start with the profile guide, then verify the delivered artifact in the app that owns the route.", Destination: "choosing-profile"},
			{ID: "source", Question: "Does a reference prove a feature is ready?", Answer: "No. A source or private adaptation establishes only its recorded scope. Preserve provenance, choose a task, and run the applicable acceptance checks.", Destination: "catalog"},
			{ID: "quality", Question: "Where do I find the remaining proof?", Answer: "Quality work records the current narrow evidence and its next step. A partial or not-run gate remains open until the selected task supplies proof.", Destination: "quality"},
		},
	}
}

func validateStaticContent(model staticContentModel) error {
	if err := validStaticContentText(model.Title, 256); err != nil {
		return fmt.Errorf("content title: %w", err)
	}
	if err := validStaticContentText(model.Intro, 1024); err != nil {
		return fmt.Errorf("content intro: %w", err)
	}
	if len(model.Sections) < 1 || len(model.Sections) > 8 || len(model.FAQ) < 1 || len(model.FAQ) > 8 {
		return errors.New("content requires 1-8 sections and FAQ entries")
	}
	seen := make(map[string]bool, len(model.Sections)+len(model.FAQ))
	for _, section := range model.Sections {
		if !staticContentID(section.ID) || staticContentReservedIDs[section.ID] || seen[section.ID] {
			return errors.New("content has invalid or duplicate section ID")
		}
		seen[section.ID] = true
		if err := validStaticContentText(section.Heading, 256); err != nil {
			return fmt.Errorf("content section heading: %w", err)
		}
		if err := validStaticContentText(section.Body, 4096); err != nil {
			return fmt.Errorf("content section body: %w", err)
		}
	}
	for _, item := range model.FAQ {
		if !staticContentID(item.ID) || staticContentReservedIDs[item.ID] || seen[item.ID] {
			return errors.New("content has invalid or duplicate FAQ ID")
		}
		seen[item.ID] = true
		if err := validStaticContentText(item.Question, 512); err != nil {
			return fmt.Errorf("content question: %w", err)
		}
		if err := validStaticContentText(item.Answer, 8192); err != nil {
			return fmt.Errorf("content answer: %w", err)
		}
		if _, ok := staticContentDestinations[item.Destination]; !ok {
			return errors.New("content has an unknown destination")
		}
	}
	return nil
}

func validStaticContentText(value string, max int) error {
	if value == "" || len(value) > max || !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return errors.New("must be nonblank valid UTF-8 within bounds")
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return errors.New("contains control character")
		}
	}
	return nil
}

func staticContentID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') || (r == '-' && (i == 0 || i == len(value)-1)) {
			return false
		}
	}
	return true
}

type staticContentRequest struct {
	Theme string
	State staticContentState
}

func parseStaticContentRequest(raw string) (staticContentRequest, error) {
	in := staticContentRequest{State: staticContentReady}
	if len(raw) > staticContentMaxQueryBytes {
		return in, errors.New("content query exceeds 4 KiB")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return in, errors.New("content query has invalid encoding")
	}
	for key, vals := range values {
		if (key != "theme" && key != "state") || len(vals) != 1 || !utf8.ValidString(key) || strings.ContainsRune(key, 0) || !utf8.ValidString(vals[0]) || strings.ContainsRune(vals[0], 0) {
			return in, errors.New("content query is unsupported")
		}
	}
	in.Theme = values.Get("theme")
	if in.Theme != "" && sanitizeTheme(in.Theme) != in.Theme {
		return in, errors.New("content theme must be light, dark, or system")
	}
	if state := values.Get("state"); state != "" {
		in.State = staticContentState(state)
	}
	if in.State != staticContentReady && in.State != staticContentEmpty && in.State != staticContentUnavailable {
		return in, errors.New("content state is unsupported")
	}
	return in, nil
}

func staticContentURL(path, theme string) string {
	if theme == "" {
		return path
	}
	return path + "?" + url.Values{"theme": {theme}}.Encode()
}

type staticContentView struct {
	Model       staticContentModel
	ToC, Notice template.HTML
	Theme       string
	State       staticContentState
	Destination func(string) string
	Reset       string
	Static      bool // Plain-file export has real relative links, never a server-only state form.
}

func buildStaticContent(model staticContentModel, in staticContentRequest) (template.HTML, error) {
	view, err := newStaticContentView(model, in)
	if err != nil {
		return "", err
	}
	return htmlf("static-content", staticContentHTML, view)
}

func newStaticContentView(model staticContentModel, in staticContentRequest) (staticContentView, error) {
	if err := validateStaticContent(model); err != nil {
		return staticContentView{}, err
	}
	headings := make([]kitdocs.Heading, 0, len(model.Sections)+1)
	headings = append(headings, kitdocs.Heading{Level: 2, ID: "content-guide", Text: "Read this first"})
	for _, section := range model.Sections {
		headings = append(headings, kitdocs.Heading{Level: 2, ID: section.ID, Text: section.Heading})
	}
	headings = append(headings, kitdocs.Heading{Level: 2, ID: "content-faq", Text: "Questions before reuse"})
	view := staticContentView{
		Model: model,
		// Use the panel's Gallery utility classes rather than RenderToC's doc-toc
		// prose classes. Gallery does not scan appkit/ui/docs/prose.css.
		ToC:   template.HTML(kitdocs.RenderToCPanel(headings, kitdocs.Options{})),
		Theme: in.Theme,
		State: in.State,
		Destination: func(id string) string {
			return staticContentURL(staticContentDestinations[id], in.Theme)
		},
		Reset: staticContentURL("/content", in.Theme),
	}
	switch in.State {
	case staticContentEmpty:
		view.Notice = template.HTML(callout.Render(callout.Callout{Kind: callout.Info, Title: "No guide entries are available", Body: "This is a synthetic empty specimen. No content was inferred or replaced; return to the ready guide to continue."}))
	case staticContentUnavailable:
		view.Notice = template.HTML(callout.Render(callout.Callout{Kind: callout.Caution, Title: "The guide is temporarily unavailable", Body: "This is a synthetic unavailable specimen. No real content source was queried and no saved content changed. Return to the ready guide to recover."}))
	}
	return view, nil
}

func registerStaticContent(mux *http.ServeMux) {
	mux.HandleFunc("/content", serveStaticContent)
	mux.HandleFunc("/content/choosing-a-profile", serveStaticContent)
	mux.HandleFunc("/content/", serveStaticContent)
}

func serveStaticContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "This static-content specimen is read-only. Open it with GET; no data changed.", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Opaque != "" || r.URL.EscapedPath() != r.URL.Path {
		writeStaticContentProblem(w, r, http.StatusNotFound, "This content destination does not exist.")
		return
	}
	in, err := parseStaticContentRequest(r.URL.RawQuery)
	if err != nil {
		writeStaticContentProblem(w, r, http.StatusBadRequest, "This content request could not be read. Use one state and light, dark or system appearance.")
		return
	}
	if r.URL.Path == "/content/choosing-a-profile" {
		if in.State != staticContentReady {
			writeStaticContentProblem(w, r, http.StatusBadRequest, "This reading destination does not support specimen state. Return to the integration guide to choose a state.")
			return
		}
		writeStaticContentArticle(w, in)
		return
	}
	if r.URL.Path != "/content" {
		writeStaticContentProblem(w, r, http.StatusNotFound, "This content destination does not exist.")
		return
	}
	content, err := buildStaticContent(staticContentGuide(), in)
	if err != nil {
		http.Error(w, "The static-content specimen could not be prepared.", http.StatusInternalServerError)
		return
	}
	status := http.StatusOK
	if in.State == staticContentUnavailable {
		status = http.StatusServiceUnavailable
	}
	writeStaticContentPage(w, "/content", "Choose an integration profile", content, in.Theme, in.State, status)
}

func writeStaticContentArticle(w http.ResponseWriter, in staticContentRequest) {
	content, err := htmlf("content-profile", staticContentArticleHTML, map[string]string{"Back": staticContentURL("/content", in.Theme)})
	if err != nil {
		http.Error(w, "The static-content specimen could not be prepared.", http.StatusInternalServerError)
		return
	}
	writeStaticContentPage(w, "/content/choosing-a-profile", "Choosing a profile", content, in.Theme, staticContentReady, http.StatusOK)
}

func writeStaticContentProblem(w http.ResponseWriter, r *http.Request, status int, message string) {
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	content, err := htmlf("content-problem", staticContentProblemHTML, map[string]string{"Message": message, "Back": staticContentURL("/content", theme)})
	if err != nil {
		http.Error(w, "The static-content specimen could not be prepared.", http.StatusInternalServerError)
		return
	}
	writeStaticContentPage(w, r.URL.Path, "Content destination unavailable", content, theme, staticContentReady, status)
}

func writeStaticContentPage(w http.ResponseWriter, path, title string, content template.HTML, theme string, state staticContentState, status int) {
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == "content"
		if theme != "" {
			links[i].Href = staticContentURL(links[i].Href, theme)
		}
	}
	withTheme := func(value string) string {
		values := url.Values{}
		if state != staticContentReady {
			values.Set("state", string(state))
		}
		if value != "" {
			values.Set("theme", value)
		}
		if len(values) == 0 {
			return path
		}
		return path + "?" + values.Encode()
	}
	body, err := render(pageData{Title: title, Nav: links, Theme: theme, Page: "content", Content: content, ThemeLight: withTheme("light"), ThemeDark: withTheme("dark"), ThemeSystem: withTheme("")})
	if err != nil {
		http.Error(w, "The static-content specimen could not be rendered.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

const staticContentHTML = `
<section class="max-w-[75ch]" aria-labelledby="content-title">
  <p class="text-sm font-medium text-accent">STATIC CONTENT SPECIMEN</p>
  <h1 id="content-title" class="mt-3 text-3xl font-semibold text-ink">{{.Model.Title}}</h1>
  <p class="mt-4 text-ink-muted">{{.Model.Intro}}</p>
  {{if ne .State "ready"}}<div class="mt-6">{{.Notice}}</div><a href="{{.Reset}}" class="mt-5 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Return to the ready guide</a>{{else}}
  <div class="mt-7">{{.ToC}}</div>
  <section class="mt-9" aria-labelledby="content-guide"><h2 id="content-guide" tabindex="-1" class="text-xl font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Read this first</h2>
    {{range .Model.Sections}}<section id="{{.ID}}" tabindex="-1" class="mt-7 focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2"><h2 class="text-lg font-semibold text-ink">{{.Heading}}</h2><p class="mt-2 text-ink-muted">{{.Body}}</p></section>{{end}}
  </section>
  <section id="content-faq" tabindex="-1" class="mt-10 border-t border-line pt-7 focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2" aria-labelledby="content-faq-heading">
    <h2 id="content-faq-heading" class="text-xl font-semibold text-ink">Questions before reuse</h2><div class="mt-4 divide-y divide-line border-y border-line">{{range .Model.FAQ}}<details id="{{.ID}}" class="py-4"><summary class="min-h-11 cursor-pointer py-2 font-medium text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">{{.Question}}</summary><div class="max-w-[75ch] pt-2 text-sm text-ink-muted"><p>{{.Answer}}</p><a href="{{call $.Destination .Destination}}" class="mt-3 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Read the related guide</a></div></details>{{end}}</div>
  </section>
  <nav class="mt-10 flex flex-wrap gap-x-5 gap-y-2 text-sm" aria-label="Static content resources"><a href="{{call .Destination "catalog"}}" class="inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">{{if .Static}}Match the runtime to the task{{else}}Browse reusable capabilities{{end}}</a><a href="{{call .Destination "quality"}}" class="inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">{{if .Static}}Review delivery evidence{{else}}Read quality work{{end}}</a></nav>{{end}}
  {{if not .Static}}<form action="/content" method="get" class="mt-8 border-t border-line pt-6" aria-label="Static content specimen state">{{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}<label for="content-state" class="block text-sm font-medium text-ink">Specimen state</label><div class="mt-2 flex flex-wrap items-center gap-3"><select id="content-state" name="state" class="min-h-11 rounded-md border border-line bg-surface px-3 text-sm text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2"><option value="ready"{{if eq .State "ready"}} selected{{end}}>Ready guide</option><option value="empty"{{if eq .State "empty"}} selected{{end}}>Synthetic empty state</option><option value="unavailable"{{if eq .State "unavailable"}} selected{{end}}>Synthetic unavailable state</option></select><button class="min-h-11 rounded-md border border-line bg-surface-raised px-4 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Show state</button></div></form>{{end}}
</section>`

const staticContentArticleHTML = `
<article class="max-w-[75ch]" aria-labelledby="content-profile-title"><p class="text-sm font-medium text-accent">READING DESTINATION</p><h1 id="content-profile-title" tabindex="-1" class="mt-3 text-3xl font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Choosing a profile</h1><p class="mt-4 text-ink-muted">Select the smallest profile that can perform the whole task and its recovery path. A richer runtime is not evidence of a better fit.</p><h2 id="content-runtime-fit" tabindex="-1" class="mt-9 text-xl font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Match the runtime to the task</h2><ol class="mt-4 space-y-4 text-ink-muted"><li><strong class="text-ink">Static brochure or documentation:</strong> build-time HTML with deliberate assets and native links or disclosures.</li><li><strong class="text-ink">Ordinary authoritative application:</strong> Go/HTTP with optional Datastar behavior where the task needs it.</li><li><strong class="text-ink">Local-heavy editing:</strong> a client-owned island that preserves draft and history during outer updates.</li></ol><p class="mt-6 text-sm text-ink-muted">Optional Wasm or NATS needs a measured task requirement; neither is implied by this guide. These are profile-fit examples, not claims that every option is shipped or adopted.</p><h2 id="content-delivery-evidence" tabindex="-1" class="mt-9 text-xl font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Check the delivered boundary</h2><p class="mt-3 text-ink-muted">Verify the selected artifact in the application that owns the route, policy and data. Keep optional behavior optional until the task needs it.</p><a href="{{.Back}}" class="mt-8 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Return to the integration guide</a></article>`

const staticContentProblemHTML = `
<section class="max-w-[75ch]" aria-labelledby="content-problem-title"><h1 id="content-problem-title" class="text-2xl font-semibold text-ink">Content destination unavailable</h1><p class="mt-3 text-ink-muted">{{.Message}}</p><a href="{{.Back}}" class="mt-6 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Return to the integration guide</a></section>`

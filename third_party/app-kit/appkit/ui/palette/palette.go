// Package palette renders an experimental command palette (catalog ID
// interaction.command-palette): a real search link, a native modal dialog built
// on overlay.RenderDialog, and one server-rendered results fragment shared by the
// palette and the app's ordinary GET search page.
//
// Layers. palette.js is L2: a classic, dependency-free script with the single
// global window.AppKitPalette, served by Script or Handler. The Go renderers are
// L3b: they emit role utilities, so the consumer's Tailwind build must scan
// appkit/ui/palette/*.go (and appkit/ui/overlay/*.go for the dialog frame) and
// include the kit theme CSS.
//
// Without JavaScript the trigger is an ordinary link to Palette.Action, the app's
// full search page; that page renders RenderSearchForm and RenderResults. With the
// script, the same link (or the configured shortcut, Mod+K by default: Command on
// Apple platforms, Control elsewhere) opens the dialog and focuses its input.
// Typing fetches Palette.ResultsURL (default Action) with the query parameter,
// debounced, and sends the FragmentHeader request header; the app answers with
// the RenderResults fragment for that query and nothing else. Any other answer
// (non-2xx, timeout, a fragment for another query, or a full page such as a
// login redirect, even one that renders this palette) shows the error state. A
// newer query aborts the older request and a late response is ignored. ArrowDown/ArrowUp move an aria-activedescendant selection
// through the options, Enter follows the selected link, Escape closes and focus
// returns to the invoker. Enter with no current selection, or while the shown
// results belong to an older query, submits the native GET form to Action.
//
// Script hooks (all rendered here; apps do not need to author them):
//
//	data-palette-open="ID"   on the trigger link; a primary click opens the dialog
//	data-palette="ID"        on the dialog body; carries action/url/param/shortcut/debounce/timeout/label
//	data-palette-input       the combobox input
//	data-palette-results     the results region the fragment replaces
//	data-palette-fragment    the root of every RenderResults fragment
//	data-palette-list        the element upgraded to role=listbox
//	data-palette-group       one group section, upgraded to role=group
//	data-palette-option      one result link, upgraded to role=option
//	data-palette-error       a <template> holding the client-side failure state
//	data-palette-busy        the visible "Searching…" indicator
//	data-palette-status      the polite live region for result announcements
//	data-palette-page        the "open search page" link, kept in step with the query
//	data-palette-hint        the trigger's shortcut hint, hidden until the script wires the trigger
//
// window.AppKitPalette = { init(scope), open(id, invoker), close(id) }. init runs
// at DOM readiness and is idempotent. init(scope) wires the [data-palette] roots
// in scope (a root itself, any container, or the document), forgets palettes
// whose markup has left the document and re-upgrades every trigger; call it
// after inserting, replacing or removing palette or trigger markup (for example
// after an htmx or Datastar swap).
//
// The app owns: the search routes and their authorization, what is searched and
// how results are ranked, the full search page layout, result destinations,
// caching (send Vary: X-AppKit-Palette when one URL serves both representations),
// and localization beyond the configurable texts here. Built-in secondary copy
// ("Searching…", result counts, key hints) is English.
package palette

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/internal/webref"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/overlay"
)

//go:embed palette.js
var paletteJS string

// FragmentHeader is the request header palette.js sends (value "1") when it
// wants the RenderResults fragment rather than the full search page.
const FragmentHeader = "X-AppKit-Palette"

// Defaults applied by the renderers when the corresponding field is zero.
const (
	DefaultParam       = "q"
	DefaultShortcut    = "mod+k"
	DefaultLabel       = "Search"
	DefaultPlaceholder = "Type to search"
	DefaultIdleText    = "Start typing to search."
	DefaultErrorText   = "Search is unavailable right now. Keep typing to retry, or open the search page."
	DefaultDebounce    = 150 * time.Millisecond
	DefaultTimeout     = 8 * time.Second
)

// Bounds on configuration and on one results fragment.
const (
	MaxDebounce = 2 * time.Second
	MinTimeout  = time.Second
	MaxTimeout  = time.Minute
	MaxGroups   = 32
	MaxResults  = 200
)

// Palette configures one palette instance. The zero value of every optional
// field selects its documented default; invalid values are refused, never
// silently corrected.
type Palette struct {
	// ID is the dialog id and the prefix of every generated id. Required;
	// document-unique; a letter followed by up to 63 letters, digits, "-" or "_".
	ID string
	// Action is the app's full GET search page: the trigger's href and the
	// dialog form's action. Required; a same-origin relative reference without a
	// fragment. Its query parameters (app context such as a scope or theme) are
	// rendered as hidden form fields, because a GET form replaces the action's
	// query; they must not include Param.
	Action string
	// ResultsURL is fetched for fragments. Optional (defaults to Action); a
	// same-origin relative reference, which may carry its own query but no fragment.
	ResultsURL string
	// Param is the query parameter name. Optional (default "q").
	Param string
	// Label names the trigger, the dialog title and the input. Optional (default "Search").
	Label string
	// Placeholder is the input hint text. Optional (default "Type to search").
	Placeholder string
	// Shortcut opens the palette from anywhere on the page: "mod+k" (default),
	// "none" to disable, or up to three of mod/ctrl/meta/alt/shift joined by "+"
	// before one key a-z, 0-9 or "/". "mod" is Command on Apple platforms and
	// Control elsewhere. A shortcut without ctrl/meta/mod/alt is ignored while
	// focus is in an editable field.
	Shortcut string
	// Debounce is the pause after typing before a request. Optional (default
	// 150ms); at most 2s.
	Debounce time.Duration
	// Timeout aborts a request that has not completed. Optional (default 8s);
	// between 1s and 1m. A timeout shows the error state.
	Timeout time.Duration
	// IdleText is shown before anything is typed. Optional.
	IdleText string
	// ErrorText is shown when a request fails, times out, is refused or does not
	// return a results fragment. Optional.
	ErrorText string
}

var (
	idPattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
	paramPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,31}$`)
	keyPattern   = regexp.MustCompile(`^[a-z0-9/]$`)
)

type shortcut struct {
	mods []string // canonical order: mod, ctrl, alt, shift, meta
	key  string
}

// config is a validated, defaulted Palette.
type config struct {
	Palette
	shortcut *shortcut
	path     string     // Action without its query
	context  url.Values // Action's query, carried as hidden fields
}

func textOK(s string) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && r != '\t'
	})
}

func relativeRef(ref string) bool {
	if !webref.Allowed(ref) {
		return false
	}
	u, err := url.Parse(ref)
	return err == nil && u.Scheme == "" && u.Host == "" && !strings.Contains(ref, "#")
}

func parseShortcut(spec string) (*shortcut, error) {
	if spec == "none" {
		return nil, nil
	}
	parts := strings.Split(spec, "+")
	if len(parts) > 4 {
		return nil, fmt.Errorf("shortcut %q has more than three modifiers", spec)
	}
	key := parts[len(parts)-1]
	if !keyPattern.MatchString(key) {
		return nil, fmt.Errorf("shortcut %q must end in one key a-z, 0-9 or \"/\"", spec)
	}
	seen := map[string]bool{}
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "ctrl", "meta", "alt", "shift":
		default:
			return nil, fmt.Errorf("shortcut %q: unknown modifier %q", spec, m)
		}
		if seen[m] {
			return nil, fmt.Errorf("shortcut %q repeats %q", spec, m)
		}
		seen[m] = true
	}
	if seen["mod"] && (seen["ctrl"] || seen["meta"]) {
		return nil, fmt.Errorf("shortcut %q combines mod with ctrl or meta", spec)
	}
	s := &shortcut{key: key}
	for _, m := range []string{"mod", "ctrl", "alt", "shift", "meta"} {
		if seen[m] {
			s.mods = append(s.mods, m)
		}
	}
	return s, nil
}

// spec returns the canonical shortcut text sent to the script.
func (s *shortcut) spec() string {
	if s == nil {
		return "none"
	}
	return strings.Join(append(append([]string(nil), s.mods...), s.key), "+")
}

func (p Palette) validate() (config, error) {
	c := config{Palette: p}
	if !idPattern.MatchString(p.ID) {
		return c, fmt.Errorf("palette: ID %q must be a letter followed by up to 63 letters, digits, '-' or '_'", p.ID)
	}
	if !relativeRef(p.Action) {
		return c, fmt.Errorf("palette %s: Action %q must be a same-origin relative reference without fragment", p.ID, p.Action)
	}
	path, query, _ := strings.Cut(p.Action, "?")
	context, err := url.ParseQuery(query)
	if err != nil || path == "" {
		return c, fmt.Errorf("palette %s: Action %q needs a path and a well-formed query", p.ID, p.Action)
	}
	c.path, c.context = path, context
	if c.ResultsURL == "" {
		c.ResultsURL = p.Action
	} else if !relativeRef(p.ResultsURL) {
		return c, fmt.Errorf("palette %s: ResultsURL %q must be a same-origin relative reference without fragment", p.ID, p.ResultsURL)
	}
	if c.Param == "" {
		c.Param = DefaultParam
	} else if !paramPattern.MatchString(c.Param) {
		return c, fmt.Errorf("palette %s: Param %q must be a letter or '_' followed by up to 31 letters, digits, '_', '.' or '-'", p.ID, p.Param)
	}
	if c.context.Has(c.Param) {
		return c, fmt.Errorf("palette %s: Action query already sets the search parameter %q", p.ID, c.Param)
	}
	for _, text := range []struct {
		name string
		v    *string
		def  string
	}{
		{"Label", &c.Label, DefaultLabel},
		{"Placeholder", &c.Placeholder, DefaultPlaceholder},
		{"IdleText", &c.IdleText, DefaultIdleText},
		{"ErrorText", &c.ErrorText, DefaultErrorText},
	} {
		if *text.v == "" {
			*text.v = text.def
		} else if !textOK(*text.v) {
			return c, fmt.Errorf("palette %s: %s must be nonblank valid UTF-8 without control characters", p.ID, text.name)
		}
	}
	if c.Shortcut == "" {
		c.Shortcut = DefaultShortcut
	}
	s, err := parseShortcut(c.Shortcut)
	if err != nil {
		return c, fmt.Errorf("palette %s: %w", p.ID, err)
	}
	c.shortcut = s
	switch {
	case c.Debounce == 0:
		c.Debounce = DefaultDebounce
	case c.Debounce < time.Millisecond || c.Debounce > MaxDebounce:
		return c, fmt.Errorf("palette %s: Debounce %v must be between 1ms and %v", p.ID, p.Debounce, MaxDebounce)
	}
	switch {
	case c.Timeout == 0:
		c.Timeout = DefaultTimeout
	case c.Timeout < MinTimeout || c.Timeout > MaxTimeout:
		return c, fmt.Errorf("palette %s: Timeout %v must be between %v and %v", p.ID, p.Timeout, MinTimeout, MaxTimeout)
	}
	return c, nil
}

func esc(s string) string { return html.EscapeString(s) }

// hiddenFields renders Action's query as hidden inputs in sorted key order,
// keeping repeated values in their original order.
func (c config) hiddenFields() string {
	keys := make([]string, 0, len(c.context))
	for k := range c.context {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		for _, v := range c.context[k] {
			fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, esc(k), esc(v))
		}
	}
	return b.String()
}

const searchIcon = `<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true" focusable="false"><circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.5"/><path d="M10.5 10.5 14 14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`

const focusRing = "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"

// RenderTrigger returns the palette's trigger: a real link to Action, so it
// navigates to the full search page without JavaScript. With palette.js a
// primary, unmodified click opens the dialog instead; modified clicks (new tab)
// keep the link behavior. It is at least 44px tall and wide.
//
// Only palette.js implements the shortcut, so the trigger never advertises it
// itself: it renders a hidden, empty hint and no aria-keyshortcuts. When the
// script wires a trigger to a mounted palette it sets aria-keyshortcuts and
// reveals the hint in the viewer's platform notation (⌘K on Apple, Ctrl+K
// elsewhere). Without the script, or with no palette on the page, nothing
// promises a shortcut that does not work.
func RenderTrigger(p Palette) (string, error) {
	c, err := p.validate()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<a href="%s" data-palette-open="%s"`, esc(c.Action), esc(c.ID))
	fmt.Fprintf(&b, ` class="inline-flex min-h-11 min-w-11 items-center gap-2 rounded-md border border-line bg-surface px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink %s">`, focusRing)
	b.WriteString(searchIcon)
	fmt.Fprintf(&b, `<span class="text-ink">%s</span>`, esc(c.Label))
	if c.shortcut != nil {
		b.WriteString(`<kbd data-palette-hint hidden aria-hidden="true" class="ml-auto rounded border border-line bg-surface-sunken px-1.5 font-mono text-xs text-ink-muted"></kbd>`)
	}
	b.WriteString(`</a>`)
	return b.String(), nil
}

// RenderDialog returns the palette dialog: overlay.RenderDialog (size "lg",
// backdrop light-dismiss) around a GET search form whose input is an ARIA
// combobox, a fixed-height results region holding the idle state, a polite
// status region, the client failure template and a footer with key hints and a
// link to the full search page. Include Script once per page; overlay.js is not
// required. Render it once per page, anywhere in the body.
func RenderDialog(p Palette) (string, error) {
	c, err := p.validate()
	if err != nil {
		return "", err
	}
	idle, err := RenderResults(Results{Message: c.IdleText})
	if err != nil {
		return "", err
	}
	failure, err := renderFailure(c.ErrorText, "")
	if err != nil {
		return "", err
	}
	id := esc(c.ID)
	var body strings.Builder
	fmt.Fprintf(&body, `<div data-palette="%s" data-palette-action="%s" data-palette-url="%s" data-palette-param="%s" data-palette-shortcut="%s" data-palette-debounce="%d" data-palette-timeout="%d" data-palette-label="%s" class="flex flex-col gap-3">`,
		id, esc(c.Action), esc(c.ResultsURL), esc(c.Param), esc(c.shortcut.spec()), c.Debounce.Milliseconds(), c.Timeout.Milliseconds(), esc(c.Label))
	fmt.Fprintf(&body, `<form role="search" action="%s" method="get" data-palette-form>%s`, esc(c.path), c.hiddenFields())
	fmt.Fprintf(&body, `<label for="%s-input" class="sr-only">%s</label>`, id, esc(c.Label))
	body.WriteString(`<div class="relative"><span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted">` + searchIcon + `</span>`)
	fmt.Fprintf(&body, `<input id="%s-input" name="%s" type="search" data-palette-input role="combobox" aria-autocomplete="list" aria-expanded="false" aria-controls="%s-results" autocomplete="off" autocapitalize="off" spellcheck="false" enterkeyhint="search" placeholder="%s" class="block min-h-11 w-full rounded-md border border-line-strong bg-surface py-2 pl-10 pr-3 text-base text-ink placeholder:text-ink-muted sm:text-sm %s">`,
		id, esc(c.Param), id, esc(c.Placeholder), focusRing)
	// A reserved row keeps the results from shifting when the indicator toggles.
	body.WriteString(`</div></form><div class="-my-1 flex h-4 items-center justify-end px-3"><span data-palette-busy hidden class="text-xs text-ink-muted">Searching…</span></div>`)
	body.WriteString(`<p data-palette-status role="status" aria-atomic="true" class="sr-only"></p>`)
	fmt.Fprintf(&body, `<div id="%s-results" data-palette-results class="h-[min(22rem,50dvh)] overflow-y-auto overscroll-contain">%s</div>`, id, idle)
	fmt.Fprintf(&body, `<template data-palette-error>%s</template>`, failure)
	body.WriteString(`</div>`)

	footer := fmt.Sprintf(`<p class="mr-auto hidden text-xs text-ink-muted sm:block"><kbd class="font-mono">↑</kbd> <kbd class="font-mono">↓</kbd> move · <kbd class="font-mono">Enter</kbd> open · <kbd class="font-mono">Esc</kbd> close</p>`+
		`<a href="%s" data-palette-page class="inline-flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-ink underline underline-offset-4 hover:bg-surface-sunken %s">Open search page</a>`,
		esc(c.Action), focusRing)
	return overlay.RenderDialog(overlay.Dialog{
		ID:           c.ID,
		Title:        c.Label,
		Body:         template.HTML(body.String()),
		Footer:       template.HTML(footer),
		Size:         "lg",
		LightDismiss: true,
	}), nil
}

// RenderSearchForm returns the full search page's GET form: a visible label, a
// search input prefilled with query, and a submit button, all 44px targets.
// It is the no-JavaScript path; place RenderResults below it.
func RenderSearchForm(p Palette, query string) (string, error) {
	c, err := p.validate()
	if err != nil {
		return "", err
	}
	id := esc(c.ID) + "-page-input"
	return fmt.Sprintf(`<form role="search" action="%s" method="get" class="flex flex-wrap items-end gap-2">%s`+
		`<div class="min-w-0 flex-1 basis-60"><label for="%s" class="block text-sm font-medium text-ink">%s</label>`+
		`<input id="%s" name="%s" type="search" value="%s" autocomplete="off" enterkeyhint="search" placeholder="%s" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-base text-ink placeholder:text-ink-muted sm:text-sm %s"></div>`+
		`<button type="submit" class="inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover %s">%s</button></form>`,
		esc(c.path), c.hiddenFields(), id, esc(c.Label), id, esc(c.Param), esc(query), esc(c.Placeholder), focusRing, focusRing, esc(c.Label)), nil
}

// Result is one destination. Label and Href are required; Href is a relative
// web reference or an absolute HTTP(S) URL. All text is escaped.
type Result struct {
	Label  string
	Href   string
	Detail string // optional secondary line
	Meta   string // optional short trailing hint, such as a kind or count
}

// Group is one headed section of results. Empty groups are omitted.
type Group struct {
	Heading string
	Results []Result
}

// Results is one server-rendered fragment. Its state follows from the fields:
// Error set → error; blank Query → idle; Notice set → partial (with or without
// results: a source that was not searched makes zero results inconclusive); no
// results → empty; otherwise results. Error excludes results: report a partial
// answer with Notice instead. Set Query to the request's query parameter as
// received: palette.js inserts only a fragment that answers the query it asked for.
type Results struct {
	Query  string
	Groups []Group
	// Message replaces the default idle or no-results guidance.
	Message string
	// Notice states that the results are partial, e.g. one source was unavailable.
	Notice string
	// Error explains a failed search.
	Error string
	// HeadingLevel is the group heading level, 2–6; zero means 3, beneath the
	// dialog's h2 title. A full page under an h1 passes 2.
	HeadingLevel int
}

const optionClass = "group flex min-h-11 items-center gap-3 rounded-md px-3 py-2 text-ink no-underline hover:bg-surface-sunken " + focusRing +
	" aria-selected:bg-accent-soft aria-selected:outline-2 aria-selected:-outline-offset-2 aria-selected:outline-focus"

// RenderResults validates and renders one results fragment. The same markup
// serves the palette (palette.js upgrades it to listbox/group/option roles)
// and the no-JavaScript search page (headings, lists and links). Invalid input
// returns an error and no markup: a blank group heading or result label, an
// unsafe or empty Href, Error combined with results, or more than MaxGroups
// groups or MaxResults results.
func RenderResults(r Results) (string, error) {
	level := r.HeadingLevel
	if level == 0 {
		level = 3
	}
	if level < 2 || level > 6 {
		return "", fmt.Errorf("palette: HeadingLevel %d must be 2-6", r.HeadingLevel)
	}
	if len(r.Groups) > MaxGroups {
		return "", fmt.Errorf("palette: %d groups exceed %d", len(r.Groups), MaxGroups)
	}
	total := 0
	for gi, g := range r.Groups {
		if len(g.Results) > 0 && !textOK(g.Heading) {
			return "", fmt.Errorf("palette: group %d needs a nonblank heading", gi)
		}
		for ri, res := range g.Results {
			if !textOK(res.Label) {
				return "", fmt.Errorf("palette: group %d result %d needs a nonblank label", gi, ri)
			}
			if !webref.Allowed(res.Href) {
				return "", fmt.Errorf("palette: group %d result %d has an unsafe or empty destination", gi, ri)
			}
		}
		total += len(g.Results)
	}
	if total > MaxResults {
		return "", fmt.Errorf("palette: %d results exceed %d", total, MaxResults)
	}
	for _, text := range []struct{ name, v string }{{"Message", r.Message}, {"Notice", r.Notice}, {"Error", r.Error}} {
		if text.v != "" && !textOK(text.v) {
			return "", fmt.Errorf("palette: %s must be nonblank valid UTF-8 without control characters", text.name)
		}
	}
	if r.Error != "" {
		if total > 0 {
			return "", fmt.Errorf("palette: Error excludes results; use Notice for a partial answer")
		}
		return renderFailure(r.Error, r.Query)
	}
	if strings.TrimSpace(r.Query) == "" {
		msg := r.Message
		if msg == "" {
			msg = DefaultIdleText
		}
		return fragment("idle", r.Query, 0, "", `<p class="px-3 py-8 text-center text-sm text-ink-muted">`+esc(msg)+`</p>`), nil
	}
	quoted := "“" + r.Query + "”"
	if total == 0 {
		// With a Notice a source was not searched, so zero results is not a
		// definitive absence: the state and announcement say partial, and the
		// default advice does not blame the query.
		state, announce, msg := "empty", "No results for "+quoted, r.Message
		if r.Notice != "" {
			state, announce = "partial", announce+" (partial)"
		}
		if msg == "" {
			msg = "Check the spelling or try a shorter query."
			if r.Notice != "" {
				msg = "Matches may exist in sources that were not searched."
			}
		}
		inner := `<div class="px-3 py-8 text-center"><p class="font-medium text-ink [overflow-wrap:anywhere]">No results for ` + esc(quoted) + `</p>`
		if r.Notice != "" {
			inner += notice(r.Notice)
		}
		inner += `<p class="mt-1 text-sm text-ink-muted">` + esc(msg) + `</p></div>`
		return fragment(state, r.Query, 0, announce, inner), nil
	}
	summary := strconv.Itoa(total) + " results for " + quoted
	if total == 1 {
		summary = "1 result for " + quoted
	}
	state, announce := "results", summary
	if r.Notice != "" {
		state, announce = "partial", summary+" (partial)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<p data-palette-summary class="px-3 pb-2 text-xs text-ink-muted [overflow-wrap:anywhere]">%s</p>`, esc(summary))
	if r.Notice != "" {
		b.WriteString(notice(r.Notice))
	}
	b.WriteString(`<div data-palette-list class="flex flex-col gap-4">`)
	for _, g := range r.Groups {
		if len(g.Results) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<section data-palette-group><h%d class="px-3 pb-1 text-xs font-semibold uppercase tracking-wide text-ink-muted">%s</h%d><ul class="flex flex-col gap-0.5">`, level, esc(g.Heading), level)
		for _, res := range g.Results {
			fmt.Fprintf(&b, `<li><a href="%s" data-palette-option class="%s"><span class="min-w-0 flex-1 [overflow-wrap:anywhere]"><span class="block font-medium">%s</span>`, esc(res.Href), optionClass, esc(res.Label))
			if res.Detail != "" {
				fmt.Fprintf(&b, `<span class="block text-xs text-ink-muted group-aria-selected:text-ink">%s</span>`, esc(res.Detail))
			}
			b.WriteString(`</span>`)
			if res.Meta != "" {
				fmt.Fprintf(&b, `<span class="shrink-0 text-xs text-ink-muted group-aria-selected:text-ink">%s</span>`, esc(res.Meta))
			}
			b.WriteString(`</a></li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	b.WriteString(`</div>`)
	return fragment(state, r.Query, total, announce, b.String()), nil
}

func notice(text string) string {
	return `<p data-palette-notice class="mx-1 mb-3 rounded-md border border-caution bg-caution-soft px-3 py-2 text-left text-sm text-ink"><span class="font-medium">Partial results: </span>` + esc(text) + `</p>`
}

func renderFailure(message, query string) (string, error) {
	if !textOK(message) {
		return "", fmt.Errorf("palette: error text must be nonblank valid UTF-8 without control characters")
	}
	inner := `<div class="mx-1 rounded-md border border-danger bg-danger-soft px-3 py-3 text-sm text-ink"><p class="font-medium">Search failed</p><p class="mt-1">` + esc(message) + `</p></div>`
	return fragment("error", query, 0, "Search failed. "+message, inner), nil
}

func fragment(state, query string, count int, announce, inner string) string {
	return fmt.Sprintf(`<div data-palette-fragment data-palette-state="%s" data-palette-query="%s" data-palette-count="%d" data-palette-announce="%s">%s</div>`,
		state, esc(query), count, esc(announce), inner)
}

// WantsFragment reports whether palette.js requested the results fragment.
// Serve the fragment for these requests and the full search page otherwise, and
// add "Vary: X-AppKit-Palette" when one URL serves both.
func WantsFragment(r *http.Request) bool {
	return r.Header.Get(FragmentHeader) == "1"
}

// Script returns palette.js for inlining into a page (<script> tag).
func Script() string { return paletteJS }

// Handler serves palette.js with revalidation at a stable, app-chosen URL,
// including GET, HEAD and conditional requests.
func Handler() http.Handler {
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(paletteJS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", etag)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeContent(w, r, "palette.js", time.Time{}, strings.NewReader(paletteJS))
	})
}

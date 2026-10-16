package main

// console_palette.go — OC-12 (docs/design/observatory-console.md §2): the
// ⌘K command palette over every console page, on the kit's command palette
// (appkit/ui/palette, catalog interaction.command-palette — FATHOM-KIT-01,
// which fathom originated and returned to the kit).
//
// The kit owns the dialog, the keyboard contract and the script: the
// shell's top bar carries the palette trigger (a real link to /search), the
// dialog is rendered once per page, and palette.js opens it on the trigger
// or Mod+K (Command on Apple platforms, Control elsewhere), fetches results
// as the reader types, moves the selection with the arrow keys, opens the
// selected result on Enter and closes on Escape with focus returned to the
// invoker. Fathom owns what is searched and how it ranks: console.Search,
// served by GET /search. The palette fetches /search?format=fragment and
// gets the kit's results fragment (palette.RenderResults) for exactly the
// query it asked; without JavaScript the trigger is a link to /search, a
// full page with its own search form and the same ranked results, so every
// palette result is also a real route.

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/palette"
)

// consolePalette is the one palette every console page carries.
var consolePalette = palette.Palette{
	ID:          "console-palette",
	Action:      "/search",
	ResultsURL:  "/search?format=fragment",
	Label:       "Search",
	Placeholder: "Jump to a project, plan, row, symbol, rule, verb or page",
	IdleText:    "Type a project, plan or row ID, symbol, rule, verb or page — every word must match.",
	// Mod+K is the kit default; named so a reader of this file sees it.
	Shortcut: palette.DefaultShortcut,
}

// consolePaletteScriptPath serves palette.js (palette.Handler).
const consolePaletteScriptPath = "/assets/palette.js"

// consolePaletteTrigger is the top bar's palette control: a link to the
// search page that the script turns into the dialog's opener. The
// configuration is a constant, so a refusal is a programming error that
// every page test catches; it degrades to a plain link rather than no way
// to search.
func consolePaletteTrigger() string {
	out, err := palette.RenderTrigger(consolePalette)
	if err != nil {
		return `<a href="/search" class="inline-flex min-h-11 items-center px-3 text-accent">Search</a>`
	}
	return out
}

// consolePaletteDialog is the dialog, rendered once per page.
func consolePaletteDialog() string {
	out, err := palette.RenderDialog(consolePalette)
	if err != nil {
		return ""
	}
	return out
}

// consolePaletteSearchForm is the search page's own form: the no-JavaScript
// path now that the top bar carries the palette trigger instead of a field.
func consolePaletteSearchForm(q string) string {
	out, err := palette.RenderSearchForm(consolePalette, q)
	if err != nil {
		return ""
	}
	return out
}

// wantsPaletteFragment reports whether a /search request asked for the
// palette's results fragment: ?format=fragment (the palette's ResultsURL)
// or the kit script's request header.
func wantsPaletteFragment(r *http.Request) bool {
	return r.URL.Query().Get("format") == "fragment" || palette.WantsFragment(r)
}

// paletteText makes a result's text acceptable to the kit's fragment
// renderer, which refuses control characters: titles come from plan rows,
// records and symbols read off disk.
func paletteText(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s))
}

// paletteResults maps one console.SearchWith answer onto the kit's results
// fragment. query is the request's q exactly as the page read it (trimmed,
// as the palette script trims what it sends), because the script inserts
// only a fragment that answers the query it asked. A group capped at its
// top results names its exact total in its heading and ends with a link to
// the group's own search page, so the palette never hides how much more
// there is. A result with no page of its own opens its search page.
func paletteResults(res console.SearchResults, query string) palette.Results {
	out := palette.Results{Query: query}
	var notes []string
	notes = append(notes, res.Notes...)
	for _, g := range res.Groups {
		if g.Note != "" {
			notes = append(notes, string(g.Group)+": "+g.Note)
		}
		heading := string(g.Group)
		if g.Total > len(g.Results) {
			heading += " · top " + itoa(len(g.Results)) + " of " + fmtThousands(g.Total)
		}
		pg := palette.Group{Heading: heading}
		for _, r := range g.Results {
			label := paletteText(r.Title)
			if label == "" {
				continue
			}
			href := r.Href
			if href == "" {
				href = searchHref(r.Title, g.Group, 1)
			}
			var detail []string
			for _, s := range []string{r.Subtitle, r.Service, r.Ref} {
				if t := paletteText(s); t != "" {
					detail = append(detail, t)
				}
			}
			pg.Results = append(pg.Results, palette.Result{Label: label, Href: href, Detail: strings.Join(detail, " · "), Meta: paletteText(r.Kind)})
		}
		if g.Total > len(g.Results) {
			pg.Results = append(pg.Results, palette.Result{
				Label: "Show all " + fmtThousands(g.Total) + " in " + string(g.Group),
				Href:  searchHref(query, g.Group, 1), Meta: "search page",
			})
		}
		if len(pg.Results) > 0 {
			out.Groups = append(out.Groups, pg)
		}
	}
	if len(notes) > 0 {
		out.Notice = paletteText(strings.Join(notes, "; "))
	}
	return out
}

// renderPaletteFragment renders the fragment for res. A result the kit
// refuses (it never should: labels and destinations are checked above) is
// answered as the kit's error state naming why, never as an empty list.
func renderPaletteFragment(res console.SearchResults, query string) string {
	out, err := palette.RenderResults(paletteResults(res, query))
	if err == nil {
		return out
	}
	failed, ferr := palette.RenderResults(palette.Results{Query: query, Error: "the results could not be rendered: " + paletteText(err.Error())})
	if ferr != nil {
		return ""
	}
	return failed
}

// renderPaletteQueryError is the fragment for a query search refuses (past
// its word or character bound): the kit's error state with the bound, so the
// palette says why instead of its generic "unavailable".
func renderPaletteQueryError(query string, err error) string {
	out, rerr := palette.RenderResults(palette.Results{Query: query, Error: paletteText(err.Error()) + "."})
	if rerr != nil {
		return ""
	}
	return out
}

// writePaletteFragment answers a fragment request. Vary names the header
// the kit script sends, because /search answers both representations.
func writePaletteFragment(w http.ResponseWriter, fragment string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Vary", palette.FragmentHeader)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(fragment))
}

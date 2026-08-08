package main

// observatory_brief.go — EX-6: the management-ready brief as a PAGE. Same
// honest numbers as `fathom brief`, rendered for a reader who has never
// heard of fathom; ?format=md downloads the markdown. Law 13 note stays on
// the page: fathom never sends this anywhere — a human does.

import (
	"html"
	"net/http"
	"strings"
	"time"
)

// mdInline renders the three inline spans the brief uses — **bold**,
// *emphasis* and `code` — after escaping. Escaping FIRST and emitting tags second is what
// keeps this safe: the delimiters are matched over already-escaped text, so
// no author-supplied character can become markup (BR-1).
func mdInline(s string) string {
	out := html.EscapeString(s)
	// Doubles first: once **bold** has consumed its pairs, every remaining
	// asterisk is unambiguously an emphasis delimiter.
	out = mdWrapPairs(out, "**", `<strong class="font-semibold text-ink">`, "</strong>")
	out = mdWrapPairs(out, "*", `<em>`, "</em>")
	out = mdWrapPairs(out, "`", `<code class="rounded bg-surface-sunken px-1 py-0.5 font-mono text-[0.85em] text-ink">`, "</code>")
	return out
}

// mdWrapPairs replaces each *balanced pair* of delim with open/close tags.
// An unpaired trailing delimiter is left as literal text rather than
// producing an unclosed tag.
func mdWrapPairs(s, delim, open, closing string) string {
	var b strings.Builder
	rest := s
	for {
		head, after, ok := strings.Cut(rest, delim)
		if !ok {
			b.WriteString(rest)
			return b.String()
		}
		inner, tail, closed := strings.Cut(after, delim)
		if !closed {
			b.WriteString(head)
			b.WriteString(delim)
			b.WriteString(after)
			return b.String()
		}
		b.WriteString(head)
		b.WriteString(open)
		b.WriteString(inner)
		b.WriteString(closing)
		rest = tail
	}
}

// mdToHTML is a deliberately tiny markdown renderer for the brief's own
// output shape (headings, bullets, blockquote, hr) — not a general parser.
func mdToHTML(md string) string {
	var b strings.Builder
	inList := false
	closeList := func() {
		if inList {
			b.WriteString("</ul>")
			inList = false
		}
	}
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "## "):
			closeList()
			b.WriteString(`<h2 class="mb-2 mt-8 border-b border-line pb-1 text-lg font-semibold text-ink">` + mdInline(t[3:]) + `</h2>`)
		case strings.HasPrefix(t, "# "):
			closeList()
			b.WriteString(`<h1 class="text-xl font-semibold text-ink">` + mdInline(t[2:]) + `</h1>`)
		case strings.HasPrefix(t, "- "):
			if !inList {
				b.WriteString(`<ul class="list-disc space-y-1 pl-5 text-ink">`)
				inList = true
			}
			b.WriteString(`<li class="py-0.5 leading-relaxed">` + mdInline(t[2:]) + `</li>`)
		case strings.HasPrefix(t, "> "):
			closeList()
			b.WriteString(`<blockquote class="my-2 border-l-4 border-line pl-3 text-ink-muted">` + mdInline(t[2:]) + `</blockquote>`)
		case t == "---":
			closeList()
			b.WriteString(`<hr class="my-4 border-line">`)
		case t == "":
			closeList()
		default:
			closeList()
			b.WriteString(`<p class="my-2 leading-relaxed text-ink">` + mdInline(t) + `</p>`)
		}
	}
	closeList()
	return b.String()
}

func observatoryBriefPage(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m := cachedObservatoryModel(baseURL, time.Now())
		md := renderBriefMD(m)
		if r.URL.Query().Get("format") == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="fathom-brief.md"`)
			_, _ = w.Write([]byte(md))
			return
		}
		var b strings.Builder
		b.WriteString(`<div class="mb-3"><a class="text-accent" href="/observatory/brief?format=md">download as markdown ↓</a></div>`)
		b.WriteString(`<div class="max-w-3xl rounded-lg border border-line bg-surface-raised p-5">` + mdToHTML(md) + `</div>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · brief", "brief", b.String(), m)))
	}
}

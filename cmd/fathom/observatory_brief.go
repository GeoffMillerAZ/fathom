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
			b.WriteString(`<h2 class="mb-2 mt-6 text-base font-semibold text-ink">` + html.EscapeString(t[3:]) + `</h2>`)
		case strings.HasPrefix(t, "# "):
			closeList()
			b.WriteString(`<h1 class="text-lg font-semibold text-ink">` + html.EscapeString(t[2:]) + `</h1>`)
		case strings.HasPrefix(t, "- "):
			if !inList {
				b.WriteString(`<ul class="list-disc pl-5 text-ink">`)
				inList = true
			}
			b.WriteString(`<li class="py-0.5">` + html.EscapeString(t[2:]) + `</li>`)
		case strings.HasPrefix(t, "> "):
			closeList()
			b.WriteString(`<blockquote class="my-2 border-l-4 border-line pl-3 text-ink-muted">` + html.EscapeString(t[2:]) + `</blockquote>`)
		case t == "---":
			closeList()
			b.WriteString(`<hr class="my-4 border-line">`)
		case t == "":
			closeList()
		default:
			closeList()
			b.WriteString(`<p class="my-1 text-ink">` + html.EscapeString(t) + `</p>`)
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

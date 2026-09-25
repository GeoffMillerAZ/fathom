// Package directoryjourney is a private, synthetic P04 GET-only fixture.
// It deliberately owns no listener, persistence, account state, or renderer API.
package directoryjourney

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxHTML        = 1 << 20
	pageSize       = 3
	datasetVersion = "01c6683c7c6f9d9cf2a975996d803c20b1b9c9c46be6c3a93af7c9b50f2278f0"
	journeyCSP     = "default-src 'none'; style-src 'self'; font-src data:; img-src 'self' data:; script-src 'none'; connect-src 'none'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
)

// Row is one fixed, non-production synthetic directory record.
type Row struct {
	ID, Name, Title, Email, Role string
}

// View is the small private seam the build wrapper maps to the converted component.
// DetailURLs matches Rows by index. Its values are app-owned and already scoped.
type View struct {
	Rows                                    []Row
	Heading, Introduction, AddLabel, AddURL string
	EditLabel                               string
	DetailURLs                              []string
}

// Renderer renders the converted directory fragment. It must not write an HTTP response.
type Renderer func(context.Context, View) ([]byte, error)

var frozenRows = []Row{
	{ID: "p01", Name: "Ada Vale", Title: "Software Engineer", Email: "p01@example.invalid", Role: "Member"},
	{ID: "p02", Name: "Bruno Chen", Title: "Product Designer", Email: "p02@example.invalid", Role: "Owner"},
	{ID: "p03", Name: "Cora Nunez", Title: "QA Analyst", Email: "p03@example.invalid", Role: "Viewer"},
	{ID: "p04", Name: "Dae Park", Title: "Software Engineer", Email: "p04@example.invalid", Role: "Member"},
	{ID: "p05", Name: "Erin Holt", Title: "Product Designer", Email: "p05@example.invalid", Role: "Member"},
	{ID: "p06", Name: "Farah Issa", Title: "Customer Support", Email: "p06@example.invalid", Role: "Member"},
	{ID: "p07", Name: "Gita Sen", Title: "Platform Engineer", Email: "p07@example.invalid", Role: "Member"},
	{ID: "p08", Name: "Hugo Reed", Title: "Finance Analyst", Email: "p08@example.invalid", Role: "Member"},
	{ID: "p09", Name: "Imani Cole", Title: "Product Designer", Email: "p09@example.invalid", Role: "Member"},
	{ID: "p10", Name: "Jules Mora", Title: "Software Engineer", Email: "p10@example.invalid", Role: "Member"},
	{ID: "p11", Name: `Kai "Quoted", Vale`, Title: "QA Analyst", Email: "p11@example.invalid", Role: "Member"},
	{ID: "p12", Name: "=1+1", Title: "Security Analyst", Email: "p12@example.invalid", Role: "Member"},
}

// Rows returns an independent copy in fixed ID order.
func Rows() []Row { return append([]Row(nil), frozenRows...) }

// DatasetRevision identifies the frozen literal dataset, not a live directory.
func DatasetRevision() string { return datasetVersion }

func frozenDatasetRevision() string {
	body, err := json.Marshal(Rows())
	if err != nil {
		panic("directory journey: frozen rows cannot encode")
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum)
}

type handler struct {
	render Renderer
	css    []byte
}

// NewHandler creates a loopback-only, read-only handler. It does not listen itself.
func NewHandler(render Renderer, css []byte) (http.Handler, error) {
	if render == nil {
		return nil, errors.New("directory journey: renderer required")
	}
	if len(css) == 0 || len(css) > maxHTML {
		return nil, errors.New("directory journey: css required and bounded")
	}
	return &handler{render: render, css: append([]byte(nil), css...)}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setHeaders(w.Header())
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		h.write(w, r, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", []byte("method not allowed\n"))
		return
	}
	if !requestAllowed(r) {
		h.writeErrorDocument(w, r, http.StatusForbidden, "light", "Loopback request required.", "Reset directory", defaultURL("light"))
		return
	}
	if r.URL == nil || r.URL.IsAbs() || r.URL.Host != "" || r.URL.Fragment != "" || r.URL.EscapedPath() != r.URL.Path {
		h.writeErrorDocument(w, r, http.StatusNotFound, "light", "Directory route not found.", "Back to directory", defaultURL("light"))
		return
	}
	switch r.URL.Path {
	case "/assets/style.css":
		h.write(w, r, http.StatusOK, "text/css; charset=utf-8", h.css)
	case "/assets/journey.css":
		h.write(w, r, http.StatusOK, "text/css; charset=utf-8", []byte(firstPartyCSS))
	case "/directory", "/directory.csv":
		s, err := parseSelection(r.URL.RawQuery)
		if err != nil {
			h.badSelection(w, r)
			return
		}
		if s.state == "unavailable" {
			h.unavailable(w, r, s)
			return
		}
		matches := matchingRows(s)
		if r.URL.Path == "/directory.csv" {
			if s.page > max(1, (len(matches)+pageSize-1)/pageSize) {
				h.badSelection(w, r)
				return
			}
			h.writeCSV(w, r, s, matches)
			return
		}
		h.writeList(w, r, s, matches)
	default:
		const prefix = "/directory/"
		if !strings.HasPrefix(r.URL.Path, prefix) || strings.Contains(strings.TrimPrefix(r.URL.Path, prefix), "/") {
			h.writeErrorDocument(w, r, http.StatusNotFound, knownTheme(r.URL.RawQuery), "Directory route not found.", "Back to directory", defaultURL(knownTheme(r.URL.RawQuery)))
			return
		}
		s, err := parseSelection(r.URL.RawQuery)
		if err != nil {
			h.badSelection(w, r)
			return
		}
		if s.state == "unavailable" {
			h.unavailable(w, r, s)
			return
		}
		h.writeDetail(w, r, s, strings.TrimPrefix(r.URL.Path, prefix))
	}
}

type selection struct {
	q            string
	page         int
	theme, state string
}

func parseSelection(raw string) (selection, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return selection{}, err
	}
	s := selection{page: 1, theme: "light", state: "complete"}
	for key, list := range values {
		if (key != "q" && key != "page" && key != "theme" && key != "state") || len(list) != 1 || !utf8.ValidString(list[0]) || hasControl(list[0]) {
			return selection{}, errors.New("invalid selection")
		}
		switch key {
		case "q":
			s.q = strings.TrimSpace(list[0])
			if len(s.q) > 120 {
				return selection{}, errors.New("query too long")
			}
		case "page":
			if list[0] == "" || strings.Trim(list[0], "0123456789") != "" {
				return selection{}, errors.New("invalid page")
			}
			s.page, err = strconv.Atoi(list[0])
			if err != nil || s.page < 1 {
				return selection{}, errors.New("invalid page")
			}
		case "theme":
			if list[0] != "light" && list[0] != "dark" {
				return selection{}, errors.New("invalid theme")
			}
			s.theme = list[0]
		case "state":
			if list[0] != "complete" && list[0] != "partial" && list[0] != "unavailable" {
				return selection{}, errors.New("invalid state")
			}
			s.state = list[0]
		}
	}
	return s, nil
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

func observedRows(s selection) []Row {
	rows := Rows()
	if s.state == "partial" {
		return rows[:6]
	}
	return rows
}

func matchingRows(s selection) []Row {
	needle := strings.ToLower(s.q)
	var matches []Row
	for _, row := range observedRows(s) {
		if needle == "" || strings.Contains(strings.ToLower(row.Name), needle) || strings.Contains(strings.ToLower(row.Title), needle) || strings.Contains(strings.ToLower(row.Email), needle) || strings.Contains(strings.ToLower(row.Role), needle) {
			matches = append(matches, row)
		}
	}
	return matches
}

func (h *handler) writeList(w http.ResponseWriter, r *http.Request, s selection, matches []Row) {
	pages := max(1, (len(matches)+pageSize-1)/pageSize)
	if s.page > pages {
		h.badSelection(w, r)
		return
	}
	start := (s.page - 1) * pageSize
	end := min(start+pageSize, len(matches))
	view := View{Rows: append([]Row(nil), matches[start:end]...), Heading: "Synthetic directory", AddLabel: "Reset directory", AddURL: resetURL(s), EditLabel: "View"}
	if s.state == "partial" {
		view.Introduction = fmt.Sprintf("Showing %d observed records. Partial observation: total coverage is unknown.", len(matches))
	} else {
		view.Introduction = fmt.Sprintf("Showing %d matching synthetic records from the complete fixed dataset.", len(matches))
	}
	for _, row := range view.Rows {
		view.DetailURLs = append(view.DetailURLs, selectedURL("/directory/"+row.ID, s))
	}
	fragment, err := h.render(r.Context(), cloneView(view))
	if err != nil || len(fragment) == 0 || len(fragment) > maxHTML {
		h.renderFailure(w, r, s.theme)
		return
	}
	content := listContent(s, matches, pages, fragment)
	h.writeDocument(w, r, http.StatusOK, s.theme, content)
}

func (h *handler) writeDetail(w http.ResponseWriter, r *http.Request, s selection, id string) {
	matches := matchingRows(s)
	pages := max(1, (len(matches)+pageSize-1)/pageSize)
	if s.page > pages {
		h.badSelection(w, r)
		return
	}
	for _, row := range matches {
		if row.ID == id {
			content := `<a id="directory-back" class="journey-control" href="` + escapeURL(selectedURL("/directory", s)) + `">← Back to results</a>` + statusContent(s, len(matches), pages) + detailContent(row)
			h.writeDocument(w, r, http.StatusOK, s.theme, content)
			return
		}
	}
	h.writeErrorDocument(w, r, http.StatusNotFound, s.theme, "Observed record not found in this selection.", "Back to results", selectedURL("/directory", s))
}

func listContent(s selection, matches []Row, pages int, fragment []byte) string {
	var b strings.Builder
	b.WriteString(`<form action="/directory" method="get" class="journey-filters" aria-label="Directory filters"><label for="directory-query">Find colleagues</label><input id="directory-query" class="journey-control" name="q" value="`)
	b.WriteString(html.EscapeString(s.q))
	b.WriteString(`" maxlength="120"><input type="hidden" name="theme" value="`)
	b.WriteString(s.theme)
	b.WriteString(`"><input type="hidden" name="state" value="`)
	b.WriteString(s.state)
	b.WriteString(`"><button class="journey-control">Filter directory</button></form>`)
	b.WriteString(statusContent(s, len(matches), pages))
	if len(matches) == 0 {
		b.WriteString(`<p role="status">No observed records match this selection.</p>`)
	}
	b.Write(fragment)
	b.WriteString(`<nav aria-label="Directory pages" class="journey-pages">`)
	if s.page > 1 {
		previous := s
		previous.page--
		b.WriteString(`<a id="directory-previous" class="journey-control" href="` + escapeURL(selectedURL("/directory", previous)) + `">Previous page</a>`)
	}
	if s.page < pages {
		next := s
		next.page++
		b.WriteString(`<a id="directory-next" class="journey-control" href="` + escapeURL(selectedURL("/directory", next)) + `">Next page</a>`)
	}
	b.WriteString(`<a id="directory-export" class="journey-control" href="` + escapeURL(selectedURL("/directory.csv", s)) + `">Download all ` + strconv.Itoa(len(matches)) + ` matching observed records (CSV)</a></nav>`)
	return b.String()
}

func statusContent(s selection, matching, pages int) string {
	status := fmt.Sprintf("Page %d of %d. %d matching observed records.", s.page, pages, matching)
	if s.state == "partial" {
		status += " total coverage is unknown because only a partial observation is available."
	}
	return `<p id="directory-status" data-page-count="` + strconv.Itoa(pages) + `" data-matching-count="` + strconv.Itoa(matching) + `" data-coverage="` + s.state + `">` + html.EscapeString(status) + `</p>`
}

func detailContent(row Row) string {
	return `<section aria-label="Observed record"><dl data-record-id="` + html.EscapeString(row.ID) + `"><dt>Name</dt><dd data-field="Name">` + html.EscapeString(row.Name) + `</dd><dt>Title</dt><dd data-field="Title">` + html.EscapeString(row.Title) + `</dd><dt>Email</dt><dd data-field="Email">` + html.EscapeString(row.Email) + `</dd><dt>Role</dt><dd data-field="Role">` + html.EscapeString(row.Role) + `</dd></dl></section>`
}

func (h *handler) writeCSV(w http.ResponseWriter, r *http.Request, s selection, rows []Row) {
	var b bytes.Buffer
	c := csv.NewWriter(&b)
	_ = c.Write([]string{"dataset_revision", "coverage", "query", "id", "name", "title", "email", "role"})
	for _, row := range rows {
		_ = c.Write([]string{DatasetRevision(), s.state, spreadsheetText(s.q), row.ID, spreadsheetText(row.Name), spreadsheetText(row.Title), spreadsheetText(row.Email), spreadsheetText(row.Role)})
	}
	c.Flush()
	if c.Error() != nil || b.Len() > maxHTML {
		h.renderFailure(w, r, s.theme)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="directory.csv"`)
	h.write(w, r, http.StatusOK, "text/csv; charset=utf-8", b.Bytes())
}

func spreadsheetText(s string) string {
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}

func selectedURL(path string, s selection) string {
	v := url.Values{"page": {strconv.Itoa(s.page)}, "state": {s.state}, "theme": {s.theme}}
	if s.q != "" {
		v.Set("q", s.q)
	}
	return path + "?" + v.Encode()
}

func resetURL(s selection) string {
	s.page, s.q = 1, ""
	return selectedURL("/directory", s)
}

func defaultURL(theme string) string {
	return selectedURL("/directory", selection{page: 1, theme: theme, state: "complete"})
}

func knownTheme(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil || len(values["theme"]) != 1 || hasControl(values.Get("theme")) {
		return "light"
	}
	if values.Get("theme") == "dark" {
		return "dark"
	}
	return "light"
}

func cloneView(v View) View {
	v.Rows = append([]Row(nil), v.Rows...)
	v.DetailURLs = append([]string(nil), v.DetailURLs...)
	return v
}

func (h *handler) writeDocument(w http.ResponseWriter, r *http.Request, status int, theme, content string) {
	doc := "<!doctype html><html lang=\"en\" class=\"" + theme + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><meta name=\"dataset-revision\" content=\"" + DatasetRevision() + "\"><title>Synthetic directory journey</title><link rel=\"stylesheet\" href=\"/assets/style.css\"><link rel=\"stylesheet\" href=\"/assets/journey.css\"></head><body><main class=\"journey-main\">" + content + "</main></body></html>"
	if len(doc) > maxHTML {
		h.renderFailure(w, r, theme)
		return
	}
	h.write(w, r, status, "text/html; charset=utf-8", []byte(doc))
}

func (h *handler) badSelection(w http.ResponseWriter, r *http.Request) {
	theme := knownTheme(r.URL.RawQuery)
	h.writeErrorDocument(w, r, http.StatusBadRequest, theme, "Invalid directory selection.", "Reset directory", defaultURL(theme))
}

func (h *handler) unavailable(w http.ResponseWriter, r *http.Request, s selection) {
	content := `<section class="journey-error" data-coverage="unavailable" aria-label="Directory recovery"><p role="alert">Directory observation is unavailable. No selection or export is available.</p><a class="journey-control" href="` + escapeURL(defaultURL(s.theme)) + `">Reset directory</a></section>`
	h.writeDocument(w, r, http.StatusServiceUnavailable, s.theme, content)
}

func (h *handler) renderFailure(w http.ResponseWriter, r *http.Request, theme string) {
	h.writeErrorDocument(w, r, http.StatusInternalServerError, theme, "Directory rendering is unavailable.", "Reset directory", defaultURL(theme))
}

func (h *handler) writeErrorDocument(w http.ResponseWriter, r *http.Request, status int, theme, message, label, href string) {
	content := `<section class="journey-error" aria-label="Directory recovery"><p role="alert">` + html.EscapeString(message) + `</p><a class="journey-control" href="` + escapeURL(href) + `">` + html.EscapeString(label) + `</a></section>`
	h.writeDocument(w, r, status, theme, content)
}

func (h *handler) write(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func requestAllowed(r *http.Request) bool {
	if r == nil || !loopbackAuthority(r.Host) || !loopbackPeer(r.RemoteAddr) {
		return false
	}
	if origins, present := r.Header["Origin"]; present && (len(origins) != 1 || origins[0] != "http://"+r.Host) {
		return false
	}
	if sites, present := r.Header["Sec-Fetch-Site"]; present && (len(sites) != 1 || (sites[0] != "same-origin" && sites[0] != "none")) {
		return false
	}
	return true
}

func loopbackAuthority(authority string) bool {
	host, port, err := net.SplitHostPort(authority)
	if err != nil || port == "" {
		return false
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return false
	}
	ip := net.ParseIP(host)
	return (ip != nil && ip.IsLoopback()) || strings.EqualFold(host, "localhost")
}

func loopbackPeer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	ip := net.ParseIP(host)
	return err == nil && ip != nil && ip.IsLoopback()
}

func setHeaders(h http.Header) {
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", journeyCSP)
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
}

func escapeURL(s string) string { return html.EscapeString(s) }

const firstPartyCSS = `
:root { color: var(--color-gray-900); background: var(--color-white); }
:root.dark { color: var(--color-white); background: var(--color-gray-900); }
body { min-height: 100vh; }
.journey-main { max-width: 72rem; margin: 0 auto; padding: 1rem; }
.journey-filters, .journey-pages { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; margin-block: 1rem; }
.journey-filters label { flex-basis: 100%; font-weight: 600; }
.journey-control { min-height: 44px; box-sizing: border-box; display: inline-flex; align-items: center; padding: .5rem .75rem; }
input.journey-control { display: inline-block; flex: 1 1 16rem; min-width: 0; border: 1px solid currentColor; border-radius: .375rem; background: transparent; }
.journey-filters button, .journey-pages a, .journey-error a, #directory-back { border: 1px solid currentColor; border-radius: .375rem; }
.journey-control:focus-visible { outline: 3px solid currentColor; outline-offset: 2px; }
#directory-status { margin-block: 1rem; }
dl dt { font-weight: 600; margin-top: 1rem; }
dl dd { overflow-wrap: anywhere; }
`

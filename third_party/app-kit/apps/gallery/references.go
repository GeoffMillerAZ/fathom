package gallery

import (
	"encoding/json"
	"errors"
	"html/template"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

const referenceNotice = "Private licensed source reference, not a proven kit recipe or working preview. Exact metadata is verified against the original ZIP; file listings are metadata only. Dependency ranges are not installed or resolved pins."

type referenceIndexView struct {
	Page                                      designassets.Page
	Query                                     designassets.Query
	Revision, Theme, Next, Reset, DetailQuery string
	ConversionsURL, EvidenceURL               string
}

type referenceDetailView struct {
	Asset                                                         designassets.Summary
	Projects                                                      []designassets.Project
	Files                                                         designassets.FilePage
	FileQuery                                                     designassets.FileQuery
	Revision, Theme, Back, Next, ResetFiles, ClearFile, SourceURL string
	FileLinks                                                     map[string]string
	Source                                                        *designassets.TextFile
	SourceTooLarge                                                bool
	SourceProblem                                                 string
	Notice                                                        string
	ConversionsURL, SourceConversionsURL                          string
}

type referenceProblemView struct {
	Title, Message, Back, Theme string
	Setup                       bool
}

type referenceMetadata struct {
	Revision string                 `json:"revision"`
	Asset    designassets.Summary   `json:"asset"`
	Projects []designassets.Project `json:"projects"`
	Notice   string                 `json:"notice"`
}

// All private reference surfaces share this access/method/query boundary.
func privateReferenceRegistrar(mux *http.ServeMux) func(string, bool, http.HandlerFunc) {
	return func(pattern string, api bool, handle http.HandlerFunc) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
			if !referenceRequestAllowed(r) {
				referenceProblem(w, r, api, http.StatusForbidden, "Private local reference library", "Use the Gallery's loopback address from the same origin. Cross-origin access to licensed references is disabled.", false)
				return
			}
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				referenceProblem(w, r, api, http.StatusMethodNotAllowed, "Read-only reference library", "This route accepts GET requests only.", false)
				return
			}
			if len(r.URL.RawQuery) > 16*1024 {
				referenceProblem(w, r, api, 400, "Request is too large", "Reset the reference filters and try a shorter query.", false)
				return
			}
			v, err := url.ParseQuery(r.URL.RawQuery)
			if err != nil {
				referenceProblem(w, r, api, 400, "Invalid reference request", "Query parameters must use valid URL encoding.", false)
				return
			}
			for _, values := range v {
				if len(values) != 1 {
					referenceProblem(w, r, api, 400, "Ambiguous reference request", "Supply each query parameter only once.", false)
					return
				}
			}
			handle(w, r)
		})
	}
}

// registerReferences adds only the explicitly configured private store. It never
// chooses a default, imports, extracts, executes, or downloads vendor content.
func registerReferences(mux *http.ServeMux, root string) {
	register := privateReferenceRegistrar(mux)
	registerConversions(root, register)
	registerTargetEvidence(root, register)
	registerPricingReference(root, register)
	register("/references", false, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, false)
		if !ok {
			return
		}
		q, err := referenceSearchQuery(r.URL.Query())
		if err != nil {
			referenceBadQuery(w, r, false)
			return
		}
		page, err := c.Search(q)
		if err != nil {
			referenceBadQuery(w, r, false)
			return
		}
		context := referenceSearchValues(r.URL.Query())
		v := referenceIndexView{Page: page, Query: q, Revision: c.Revision, Theme: context.Get("theme"), DetailQuery: "?" + context.Encode(), Reset: referenceLink("/references", referenceThemeValues(context))}
		v.ConversionsURL = referenceLink("/conversions", referenceThemeValues(context))
		v.EvidenceURL = referenceLink("/target-evidence", referenceThemeValues(context))
		if page.NextCursor != "" {
			next := maps.Clone(context)
			next.Set("cursor", page.NextCursor)
			v.Next = referenceLink("/references", next)
		}
		body, err := htmlf("reference-index", referenceIndexHTML, v)
		writeReferenceStatus(w, r, "Private reference library", body, err, 200)
	})
	register("/references/{id}", false, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, false)
		if !ok {
			return
		}
		a, ok := referenceExact(w, r, root, c, false)
		if !ok {
			return
		}
		q, err := referenceFilesQuery(r.PathValue("id"), r.URL.Query())
		if err != nil {
			referenceBadFiles(w, r, false)
			return
		}
		page, err := c.FilesPage(q)
		if err != nil {
			referenceBadFiles(w, r, false)
			return
		}
		values := referenceDetailValues(r.URL.Query())
		base := "/references/" + url.PathEscape(a.ID)
		v := referenceDetailView{Asset: designassets.Summarize(a), Projects: a.Projects, Files: page, FileQuery: q, Revision: c.Revision, Theme: values.Get("theme"), Back: referenceLink("/references", referenceSearchValues(values)), FileLinks: map[string]string{}, Notice: referenceNotice}
		attempts := referenceThemeValues(values)
		attempts.Set("archive_id", a.ID)
		v.ConversionsURL = referenceLink("/conversions", attempts)
		cleared := maps.Clone(values)
		cleared.Del("file")
		v.ClearFile = referenceLink(base, cleared)
		reset := referenceSearchValues(values)
		v.ResetFiles = referenceLink(base, reset)
		if page.NextCursor != "" {
			next := maps.Clone(values)
			next.Set("file_cursor", page.NextCursor)
			v.Next = referenceLink(base, next)
		}
		for _, f := range page.Items {
			selected := maps.Clone(values)
			selected.Set("file", f.Path)
			v.FileLinks[f.Path] = referenceLink(base, selected) + "#reference-source"
		}
		status := http.StatusOK
		if path := r.URL.Query().Get("file"); path != "" {
			attempts.Set("path", path)
			v.SourceConversionsURL = referenceLink("/conversions", attempts)
			text, code, message := referenceSource(root, a, path)
			if code != 0 {
				status = code
				v.SourceProblem = message
			} else {
				v.Source = text
				v.SourceURL = referenceLink("/api/references/"+url.PathEscape(a.ID)+"/source", url.Values{"path": {path}})
				if len(text.Content) > 32*1024 {
					copy := *text
					copy.Content = ""
					v.Source = &copy
					v.SourceTooLarge = true
				}
			}
		}
		body, err := htmlf("reference-detail", referenceDetailHTML, v)
		writeReferenceStatus(w, r, "Private reference: "+a.Family, body, err, status)
	})
	register("/api/references/search", true, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, true)
		if !ok {
			return
		}
		q, err := referenceSearchQuery(r.URL.Query())
		if err != nil {
			referenceBadQuery(w, r, true)
			return
		}
		page, err := c.Search(q)
		if err != nil {
			referenceBadQuery(w, r, true)
			return
		}
		writeReferenceJSON(w, page, 200)
	})
	register("/api/references/{id}", true, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, true)
		if !ok {
			return
		}
		a, ok := referenceExact(w, r, root, c, true)
		if !ok {
			return
		}
		writeReferenceJSON(w, referenceMetadata{c.Revision, designassets.Summarize(a), a.Projects, referenceNotice}, 200)
	})
	register("/api/references/{id}/files", true, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, true)
		if !ok {
			return
		}
		if _, ok := c.Get(r.PathValue("id")); !ok {
			referenceProblem(w, r, true, 404, "Reference not found", "Choose an exact archive ID from reference search.", false)
			return
		}
		q, err := referenceFilesQuery(r.PathValue("id"), r.URL.Query())
		if err != nil {
			referenceBadFiles(w, r, true)
			return
		}
		page, err := c.FilesPage(q)
		if err != nil {
			referenceBadFiles(w, r, true)
			return
		}
		w.Header().Set("X-Reference-Integrity", "metadata-only")
		writeReferenceJSON(w, page, 200)
	})
	register("/api/references/{id}/source", true, func(w http.ResponseWriter, r *http.Request) {
		c, ok := referenceReady(w, r, root, true)
		if !ok {
			return
		}
		a, ok := referenceExact(w, r, root, c, true)
		if !ok {
			return
		}
		text, status, message := referenceSource(root, a, r.URL.Query().Get("path"))
		if status != 0 {
			referenceProblem(w, r, true, status, "Cannot show this source file", message, false)
			return
		}
		writeReferenceJSON(w, text, 200)
	})
}

// Request authority is intentionally narrower than first-party Gallery routes.
// Hostnames other than exact localhost or loopback IP literals are not resolved.
func referenceRequestAllowed(r *http.Request) bool {
	peer, peerPort, peerOK := referenceAuthority(r.RemoteAddr)
	peerIP := net.ParseIP(peer)
	if !peerOK || peerPort == "" || peerIP == nil || !peerIP.IsLoopback() {
		return false
	}
	host, port, ok := referenceAuthority(r.Host)
	if !ok {
		return false
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	site := r.Header.Values("Sec-Fetch-Site")
	if len(site) > 1 {
		return false
	}
	if len(site) == 1 && site[0] != "" && site[0] != "same-origin" && site[0] != "none" {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	}
	if len(origins) != 1 || origins[0] == "" {
		return false
	}
	u, err := url.Parse(origins[0])
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if u.Scheme != scheme {
		return false
	}
	oHost, oPort, ok := referenceAuthority(u.Host)
	if !ok || oHost != host {
		return false
	}
	defaultPort := "80"
	if scheme == "https" {
		defaultPort = "443"
	}
	if port == "" {
		port = defaultPort
	}
	if oPort == "" {
		oPort = defaultPort
	}
	return port == oPort
}

func referenceAuthority(authority string) (string, string, bool) {
	if authority == "" || strings.ContainsAny(authority, " /\\\t\r\n@?#%") {
		return "", "", false
	}
	host, port := authority, ""
	if strings.Contains(authority, ":") {
		if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
			host = authority[1 : len(authority)-1]
		} else {
			var err error
			host, port, err = net.SplitHostPort(authority)
			if err != nil {
				return "", "", false
			}
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
				return "", "", false
			}
		}
	}
	if host == "" || strings.ContainsAny(host, "[]") {
		return "", "", false
	}
	return strings.ToLower(host), port, true
}

func referenceReady(w http.ResponseWriter, r *http.Request, root string, api bool) (*designassets.Catalog, bool) {
	var c *designassets.Catalog
	var err error
	if root == "" {
		err = os.ErrNotExist
	} else {
		c, err = designassets.Load(root)
	}
	if err == nil {
		return c, true
	}
	if errors.Is(err, os.ErrNotExist) {
		status := http.StatusOK
		if api {
			status = http.StatusServiceUnavailable
		}
		referenceProblem(w, r, api, status, "Set up the private reference library", "No imported private library is configured or available. Import your licensed ZIPs with appkit data assets-import, then select that private store for the Gallery. First-party capabilities remain available.", true)
	} else {
		referenceProblem(w, r, api, 500, "Private reference library needs repair", "The private catalog could not be validated. Check the selected store with appkit data assets-list and repair or reimport from your preserved original ZIPs. No source was returned.", false)
	}
	return nil, false
}

func referenceExact(w http.ResponseWriter, r *http.Request, root string, c *designassets.Catalog, api bool) (designassets.Archive, bool) {
	id := r.PathValue("id")
	if _, ok := c.Get(id); !ok {
		referenceProblem(w, r, api, 404, "Reference not found", "Choose an exact archive ID from reference search.", false)
		return designassets.Archive{}, false
	}
	a, err := designassets.Verify(root, id)
	if err != nil {
		referenceProblem(w, r, api, 500, "Reference integrity check failed", "The original ZIP or its inventory no longer matches the catalog. Repair or reimport this private store from preserved originals before reading source.", false)
		return designassets.Archive{}, false
	}
	w.Header().Set("X-Reference-Integrity", "archive-and-metadata-verified")
	return a, true
}

func referenceSource(root string, a designassets.Archive, path string) (*designassets.TextFile, int, string) {
	var member *designassets.File
	for _, f := range a.Files {
		if f.Path == path {
			copy := f
			member = &copy
			break
		}
	}
	if member == nil {
		return nil, 404, "Choose an exact source path from this archive's file listing."
	}
	if member.Size > designassets.MaxTextBytes || member.Kind == "sensitive" || member.Kind == "asset" && !strings.HasSuffix(strings.ToLower(member.Path), ".svg") {
		return nil, 400, "This file is binary, sensitive, or exceeds the 256 KiB verified-text limit. No source was returned."
	}
	f, err := designassets.ReadFile(root, a.ID, path)
	if err != nil {
		return nil, 400, "This member could not be returned as verified safe UTF-8 text. It may contain binary or credential-shaped content; no source was returned."
	}
	return f, 0, ""
}

func referenceSearchQuery(v url.Values) (designassets.Query, error) {
	limit, err := referenceLimit(v.Get("limit"))
	return designassets.Query{Text: v.Get("q"), Kind: v.Get("kind"), Family: v.Get("family"), Limit: limit, Cursor: v.Get("cursor")}, err
}

func referenceFilesQuery(id string, v url.Values) (designassets.FileQuery, error) {
	limit, err := referenceLimit(v.Get("file_limit"))
	return designassets.FileQuery{ID: id, Prefix: v.Get("prefix"), Kind: v.Get("file_kind"), Limit: limit, Cursor: v.Get("file_cursor")}, err
}

func referenceLimit(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 50 {
		return 0, errors.New("limit must be 1..50")
	}
	return n, nil
}

func referenceSearchValues(v url.Values) url.Values {
	out := url.Values{}
	for _, key := range []string{"q", "kind", "family", "limit", "cursor"} {
		if value := v.Get(key); value != "" {
			out.Set(key, value)
		}
	}
	if theme := sanitizeTheme(v.Get("theme")); theme != "" {
		out.Set("theme", theme)
	}
	return out
}

func referenceDetailValues(v url.Values) url.Values {
	out := referenceSearchValues(v)
	for _, key := range []string{"prefix", "file_kind", "file_limit", "file_cursor", "file"} {
		if value := v.Get(key); value != "" {
			out.Set(key, value)
		}
	}
	return out
}

func referenceThemeValues(v url.Values) url.Values {
	out := url.Values{}
	if theme := sanitizeTheme(v.Get("theme")); theme != "" {
		out.Set("theme", theme)
	}
	return out
}
func referenceLink(path string, v url.Values) string {
	if len(v) == 0 {
		return path
	}
	return path + "?" + v.Encode()
}

func referenceBadQuery(w http.ResponseWriter, r *http.Request, api bool) {
	referenceProblem(w, r, api, 400, "Reset reference filters", "The search filters, limit, or cursor are invalid. A catalog update can invalidate an earlier cursor; clear filters and try again.", false)
}
func referenceBadFiles(w http.ResponseWriter, r *http.Request, api bool) {
	referenceProblem(w, r, api, 400, "Reset file filters", "The file prefix, kind, limit, or cursor are invalid. Reset file filters to continue; archive search filters use separate parameters.", false)
}

func referenceProblem(w http.ResponseWriter, r *http.Request, api bool, status int, title, message string, setup bool) {
	if api {
		writeReferenceJSON(w, struct {
			Error string `json:"error"`
		}{message}, status)
		return
	}
	v := referenceProblemView{Title: title, Message: message, Back: referenceLink("/references", referenceThemeValues(r.URL.Query())), Theme: sanitizeTheme(r.URL.Query().Get("theme")), Setup: setup}
	body, err := htmlf("reference-problem", referenceProblemHTML, v)
	// Recovery pages reset malformed/private request context rather than echoing
	// oversized filters or arbitrary paths through the shell's theme links.
	recovery := r.Clone(r.Context())
	u := *r.URL
	u.Path, u.RawPath, u.RawQuery = "/references", "", referenceThemeValues(r.URL.Query()).Encode()
	recovery.URL = &u
	writeReferenceStatus(w, recovery, title, body, err, status)
}

func writeReferenceJSON(w http.ResponseWriter, value any, status int) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "Reference response encoding failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeReferenceStatus(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error, status int) {
	writePrivateReferenceStatus(w, r, title, content, err, status, referenceDetailValues(r.URL.Query()))
}

// Each private view supplies its own validated context for theme round trips.
func writePrivateReferenceStatus(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error, status int, values url.Values) {
	writePrivateReferenceStatusWithCSP(w, r, title, content, err, status, values, "")
}

// A narrowly admitted prepared comparison may allow frames only after successful
// rendering. Errors keep the enclosing source reader's restrictive policy.
func writePrivateReferenceStatusWithCSP(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error, status int, values url.Values, successCSP string) {
	if err != nil {
		http.Error(w, "Reference view could not be rendered", 500)
		return
	}
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == "references"
		links[i].Href = referenceLink(links[i].Href, referenceThemeValues(url.Values{"theme": {theme}}))
	}
	page := pageData{Title: title, Nav: links, Theme: theme, Page: "references", Content: content}
	withTheme := func(value string) string {
		q := maps.Clone(values)
		q.Del("theme")
		if value != "" {
			q.Set("theme", value)
		}
		return referenceLink(r.URL.Path, q)
	}
	page.ThemeLight = withTheme("light")
	page.ThemeDark = withTheme("dark")
	page.ThemeSystem = withTheme("")
	body, err := render(page)
	if err != nil {
		http.Error(w, "Reference page could not be rendered", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status == http.StatusOK && successCSP != "" {
		w.Header().Set("Content-Security-Policy", successCSP)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

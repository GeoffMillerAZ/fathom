// Package styleguide is an experimental, local-development token review tool.
// Apps mount it explicitly with immutable source and trusted, side-effect-free
// specimen callbacks. It is not an application settings or authentication service.
package styleguide

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// Specimen renders an app-owned fragment, not a full document. Render is trusted
// Go code: callers must keep it deterministic, synthetic and side-effect-free.
// The handler bounds output, not a callback's arbitrary computation or I/O.
type Specimen struct {
	ID, Name string
	Render   func(context.Context, io.Writer) error
}

// Options are startup configuration, never HTTP input. Stylesheets must be
// explicit same-origin CSS paths. This profile does not load font/image assets.
// Consumer authentication, when needed, wraps the entire handler and asset routes.
type Options struct {
	Engine                  *Engine
	BasePath, Origin, Title string
	Stylesheets             []string
	Specimens               []Specimen
}

type Handler struct {
	engine                                 *Engine
	base, origin, authority, scheme, title string
	stylesheets                            []string
	specimens                              []Specimen
	css                                    []byte
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var safePath = regexp.MustCompile(`^/[a-zA-Z0-9/_\.-]+$`)

// New creates a fileless local-development handler. Mount BasePath and its
// descendants without StripPrefix. The exact configured origin and loopback peer
// are required even when the caller mounts this beside a publicly reachable app.
func New(o Options) (*Handler, error) {
	u, err := url.Parse(o.Origin)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != o.Origin {
		return nil, errors.New("styleguide: configure an exact http/https loopback origin")
	}
	ip := net.ParseIP(u.Hostname())
	port, err := strconv.Atoi(u.Port())
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != u.Port() {
		return nil, errors.New("styleguide: origin needs a loopback IP and explicit port")
	}
	if o.Engine == nil || !safePath.MatchString(o.BasePath) || o.BasePath == "/" || path.Clean(o.BasePath) != o.BasePath || len(o.BasePath) > 120 || len(o.Title) < 1 || len(o.Title) > 160 || len(o.Specimens) < 1 || len(o.Specimens) > 16 || len(o.Stylesheets) > 8 {
		return nil, errors.New("styleguide: invalid engine, path, title or specimen configuration")
	}
	seen := map[string]bool{}
	for _, specimen := range o.Specimens {
		if !identifier.MatchString(specimen.ID) || seen[specimen.ID] || specimen.Name == "" || len(specimen.Name) > 100 || specimen.Render == nil {
			return nil, errors.New("styleguide: invalid or duplicate specimen")
		}
		seen[specimen.ID] = true
	}
	for _, stylesheet := range o.Stylesheets {
		if !safePath.MatchString(stylesheet) || path.Clean(stylesheet) != stylesheet || strings.HasPrefix(stylesheet, "//") || !strings.HasSuffix(stylesheet, ".css") || len(stylesheet) > 240 {
			return nil, errors.New("styleguide: stylesheets must be canonical same-origin CSS paths")
		}
	}
	return &Handler{engine: o.Engine, base: o.BasePath, origin: o.Origin, authority: u.Host, scheme: u.Scheme, title: o.Title, stylesheets: slices.Clone(o.Stylesheets), specimens: slices.Clone(o.Specimens), css: []byte(tokens.ThemeCSS() + "\n" + guideCSS)}, nil
}

func (h *Handler) allowed(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() || r.Host != h.authority || (r.TLS != nil) != (h.scheme == "https") {
		return false
	}
	if origins, present := r.Header["Origin"]; present && (len(origins) != 1 || origins[0] != h.origin) {
		return false
	}
	if sites, present := r.Header["Sec-Fetch-Site"]; present && (len(sites) != 1 || (sites[0] != "same-origin" && sites[0] != "none")) {
		return false
	}
	return true
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	if !h.allowed(r) {
		http.Error(w, "Local, same-origin style guide only", 403)
		return
	}
	if r.URL.EscapedPath() != r.URL.Path {
		http.Error(w, "Noncanonical guide route", 400)
		return
	}
	switch r.URL.Path {
	case h.base:
		if r.URL.RawQuery != "" {
			http.Error(w, "Use the guide form without URL parameters", 400)
			return
		}
		if r.Method == http.MethodGet {
			h.page(w, r, nil, "", h.specimens[0].ID, "light", "wide", "baseline", "", 200)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		h.post(w, r)
	case h.base + "/style.css":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		if r.URL.RawQuery != "" {
			http.Error(w, "No asset parameters", 400)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(h.css)))
		_, _ = w.Write(h.css)
	case h.base + "/preview":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		h.preview(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) specimen(id string) *Specimen {
	for i := range h.specimens {
		if h.specimens[i].ID == id {
			return &h.specimens[i]
		}
	}
	return nil
}

func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != h.origin {
		http.Error(w, "A same-origin form submission is required", 403)
		return
	}
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/x-www-form-urlencoded" || (len(params) != 0 && (len(params) != 1 || strings.ToLower(params["charset"]) != "utf-8")) {
		http.Error(w, "Use a URL-encoded form", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid or oversized guide form", 400)
		return
	}
	known := map[string]bool{"action": true, "specimen": true, "scheme": true, "viewport": true, "review_digest": true}
	for _, field := range h.engine.Fields() {
		known[field.Path] = true
	}
	for key, values := range r.PostForm {
		if !known[key] || len(values) != 1 || len(values[0]) > 512 {
			http.Error(w, "Unknown, duplicate or oversized field", 400)
			return
		}
	}
	specimen, scheme, viewport, action := r.PostForm.Get("specimen"), r.PostForm.Get("scheme"), r.PostForm.Get("viewport"), r.PostForm.Get("action")
	if h.specimen(specimen) == nil || (scheme != "light" && scheme != "dark") || (viewport != "narrow" && viewport != "wide") || (action != "preview" && action != "baseline" && action != "export") {
		http.Error(w, "Invalid guide selection", 400)
		return
	}
	edits := map[string]string{}
	for _, field := range h.engine.Fields() {
		value := r.PostForm.Get(field.Path)
		if field.Path == "fonts.display" && value == "__inherit__" {
			edits[field.Path] = ""
		} else if value != "" {
			edits[field.Path] = value
		}
	}
	candidate, err := h.engine.Prepare(edits)
	if err != nil {
		h.page(w, r, edits, "", specimen, scheme, viewport, "baseline", err.Error(), 422)
		return
	}
	if action == "export" {
		body, err := h.engine.Export(edits, r.PostForm.Get("review_digest"))
		if err != nil {
			h.page(w, r, edits, "", specimen, scheme, viewport, "candidate", "Export refused: preview the current candidate, inspect the diff and resolve failed checks before exporting.", 409)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="app-theme-candidate.zip"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("X-Content-SHA256", fmt.Sprintf("%x", sha256.Sum256(body)))
		_, _ = w.Write(body)
		return
	}
	version := "candidate"
	reviewed := candidate.ReviewSHA256
	if action == "baseline" {
		version = "baseline"
		// Showing the source must not acknowledge newly edited candidate bytes.
		if r.PostForm.Get("review_digest") != reviewed {
			reviewed = ""
		}
	}
	h.page(w, r, edits, reviewed, specimen, scheme, viewport, version, "", 200)
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.RawQuery) > 20<<10 {
		http.Error(w, "Preview URL is too large", 400)
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Invalid preview query", 400)
		return
	}
	for k, v := range q {
		if len(v) != 1 || (k != "draft" && k != "scheme" && k != "specimen") {
			http.Error(w, "Unknown or duplicate preview field", 400)
			return
		}
	}
	specimen := h.specimen(q.Get("specimen"))
	scheme := q.Get("scheme")
	if specimen == nil || (scheme != "light" && scheme != "dark") {
		http.Error(w, "Invalid preview selection", 400)
		return
	}
	edits := map[string]string{}
	if encoded := q.Get("draft"); encoded != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
		if err != nil || len(raw) > 12<<10 || base64.RawURLEncoding.EncodeToString(raw) != encoded || json.Unmarshal(raw, &edits) != nil || edits == nil {
			http.Error(w, "Invalid draft encoding", 400)
			return
		}
		canonical, _ := json.Marshal(edits)
		if !bytes.Equal(raw, canonical) {
			http.Error(w, "Draft must use canonical, unique fields", 400)
			return
		}
	}
	candidate, err := h.engine.Prepare(edits)
	if err != nil {
		http.Error(w, "Invalid preview draft", 422)
		return
	}
	buf := limitedFragment{limit: 512 << 10}
	if err := specimen.Render(r.Context(), &buf); err != nil || buf.failed {
		http.Error(w, "App specimen could not be rendered within its output budget", 500)
		return
	}
	css := string(candidate.CSS) + previewCSS
	styleHash := sha256.Sum256([]byte(css))
	view := struct {
		Name, Scheme string
		Styles       []string
		CSS          template.CSS
		Fragment     template.HTML
	}{specimen.Name, scheme, h.stylesheets, template.CSS(css), template.HTML(buf.buffer.String())}
	body, err := renderGuide("specimen", previewHTML, view)
	if err != nil {
		http.Error(w, "Preview render failed", 500)
		return
	}
	// Authored app CSS/renderers are trusted; SVG style attributes remain allowed.
	// No source uploads, scripts, font requests, images or active forms are admitted.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'self' 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'; style-src-attr 'unsafe-inline'; font-src 'none'; img-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("X-Theme-Candidate-SHA256", candidate.SHA256)
	_, _ = w.Write(body)
}

type limitedFragment struct {
	buffer bytes.Buffer
	limit  int
	failed bool
}

func (w *limitedFragment) Write(p []byte) (int, error) {
	if w.failed || len(p) > w.limit-w.buffer.Len() {
		w.failed = true
		return 0, errors.New("specimen output limit exceeded")
	}
	return w.buffer.Write(p)
}

func renderGuide(name, src string, v any) ([]byte, error) {
	t, err := template.New(name).Parse(src)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err = t.Execute(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

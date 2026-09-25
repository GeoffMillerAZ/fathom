package receipts

import (
	_ "embed"
	"errors"
	"html"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const (
	maxReportRawQuery = 2048
	reportCSP         = "default-src 'none'; style-src 'self'; img-src data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
)

//go:embed fixture.cue
var embeddedFixtureCUE []byte

var (
	fixtureOnce sync.Once
	fixture     fixtureData
	fixtureErr  error
)

type reportHandler struct {
	fixture fixtureData
	css     []byte
	render  func(projection, string) (string, error)
}

// New returns the complete, immutable HTTP surface for the synthetic receipts
// reference. The handler has no stores, ambient file reads, or live providers.
func New() (http.Handler, error) {
	fixtureOnce.Do(func() {
		value := cuecontext.New().CompileBytes(embeddedFixtureCUE, cue.Filename("fixture.cue"))
		if err := value.Validate(cue.Concrete(true), cue.Final()); err != nil {
			fixtureErr = errors.New("receipts: embedded fixture is invalid")
			return
		}
		if err := value.LookupPath(cue.ParsePath("fixture")).Decode(&fixture); err != nil {
			fixtureErr = errors.New("receipts: embedded fixture could not be decoded")
			return
		}
		_, fixtureErr = project(fixture, defaultSelection())
		if fixtureErr != nil {
			fixtureErr = errors.New("receipts: embedded fixture failed validation")
		}
	})
	if fixtureErr != nil {
		return nil, fixtureErr
	}
	return &reportHandler{fixture: fixture, css: append([]byte(nil), reportCSS...), render: renderReport}, nil
}

func defaultSelection() selection {
	return selection{Period: "2024-09", Channel: "all", Compare: "previous"}
}

func (h *reportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setReportHeaders(w.Header())
	if !loopbackHost(r.Host) || !sameOriginRequest(r) {
		writeReportProblem(w, r, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeReportProblem(w, r, http.StatusMethodNotAllowed)
		return
	}
	if r.URL == nil || r.URL.IsAbs() || r.URL.Host != "" || r.URL.Fragment != "" {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}

	switch {
	case r.URL.Path == "/":
		h.serveReport(w, r, "")
	case r.URL.Path == "/export.csv":
		h.serveExport(w, r)
	case r.URL.Path == "/assets/app.css":
		h.serveCSS(w, r)
	case strings.HasPrefix(r.URL.Path, "/product/"):
		id := strings.TrimPrefix(r.URL.Path, "/product/")
		if !validProductID(id) || r.URL.EscapedPath() != "/product/"+id {
			writeReportProblem(w, r, http.StatusNotFound)
			return
		}
		h.serveReport(w, r, id)
	default:
		writeReportProblem(w, r, http.StatusNotFound)
	}
}

func (h *reportHandler) serveReport(w http.ResponseWriter, r *http.Request, detailID string) {
	if r.URL.EscapedPath() != r.URL.Path {
		writeReportProblem(w, r, http.StatusNotFound)
		return
	}
	selected, theme, err := parseReportQuery(r.URL.RawQuery, false)
	if err != nil {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}
	selected.DetailID = detailID
	projected, err := project(h.fixture, selected)
	if err != nil {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}
	body, err := h.render(projected, theme)
	if err != nil {
		writeReportProblem(w, r, http.StatusInternalServerError)
		return
	}
	writeReportBody(w, r, http.StatusOK, "text/html; charset=utf-8", []byte(body), "")
}

func (h *reportHandler) serveExport(w http.ResponseWriter, r *http.Request) {
	if r.URL.EscapedPath() != r.URL.Path {
		writeReportProblem(w, r, http.StatusNotFound)
		return
	}
	selected, _, err := parseReportQuery(r.URL.RawQuery, true)
	if err != nil {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}
	if _, err := project(h.fixture, selected); err != nil {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}
	body, err := buildEvidenceCSV(h.fixture, selected)
	if err != nil {
		writeReportProblem(w, r, http.StatusInternalServerError)
		return
	}
	name := "receipts-" + selected.Period + "-" + selected.Channel + "-" + selected.Compare
	if selected.Stale {
		name += "-stale"
	} else {
		name += "-fresh"
	}
	if selected.DetailID != "" {
		name += "-" + selected.DetailID
	}
	name += ".csv"
	writeReportBody(w, r, http.StatusOK, "text/csv; charset=utf-8", body, `attachment; filename="`+name+`"`)
}

func (h *reportHandler) serveCSS(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path {
		writeReportProblem(w, r, http.StatusBadRequest)
		return
	}
	writeReportBody(w, r, http.StatusOK, "text/css; charset=utf-8", h.css, "")
}

func parseReportQuery(raw string, export bool) (selection, string, error) {
	selected := defaultSelection()
	theme := "light"
	if len(raw) > maxReportRawQuery {
		return selection{}, "", errors.New("query too large")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return selection{}, "", errors.New("invalid query")
	}
	allowed := map[string]bool{"period": true, "channel": true, "compare": true, "stale": true, "theme": true}
	if export {
		allowed["product"] = true
	}
	for key, candidates := range values {
		if !allowed[key] || len(candidates) != 1 || candidates[0] == "" {
			return selection{}, "", errors.New("invalid query")
		}
	}
	if candidate, ok := values["period"]; ok {
		selected.Period = candidate[0]
	}
	if candidate, ok := values["channel"]; ok {
		selected.Channel = candidate[0]
	}
	if candidate, ok := values["compare"]; ok {
		selected.Compare = candidate[0]
	}
	if candidate, ok := values["stale"]; ok {
		switch candidate[0] {
		case "true":
			selected.Stale = true
		case "false":
			selected.Stale = false
		default:
			return selection{}, "", errors.New("invalid query")
		}
	}
	if candidate, ok := values["theme"]; ok {
		theme = candidate[0]
	}
	if theme != "light" && theme != "dark" {
		return selection{}, "", errors.New("invalid query")
	}
	if candidate, ok := values["product"]; ok {
		selected.DetailID = candidate[0]
		if !validProductID(selected.DetailID) {
			return selection{}, "", errors.New("invalid query")
		}
	}
	return selected, theme, nil
}

func validProductID(id string) bool {
	return id == "atlas" || id == "beacon" || id == "cedar"
}

// reportURL emits the complete canonical selection in a fixed order. Detail is
// path state for report pages and an explicit product selector for CSV exports.
func reportURL(path string, selected selection, theme string) string {
	if path == "/" && selected.DetailID != "" {
		path = "/product/" + url.PathEscape(selected.DetailID)
	}
	query := "period=" + url.QueryEscape(selected.Period) +
		"&channel=" + url.QueryEscape(selected.Channel) +
		"&compare=" + url.QueryEscape(selected.Compare) +
		"&stale=" + strconv.FormatBool(selected.Stale) +
		"&theme=" + url.QueryEscape(theme)
	if path == "/export.csv" && selected.DetailID != "" {
		query += "&product=" + url.QueryEscape(selected.DetailID)
	}
	return path + "?" + query
}

func loopbackHost(authority string) bool {
	if authority == "" || strings.ContainsAny(authority, "/?#@") {
		return false
	}
	host := authority
	if split, port, err := net.SplitHostPort(authority); err == nil {
		portNumber, portErr := strconv.Atoi(port)
		if portErr != nil || portNumber < 1 || portNumber > 65535 {
			return false
		}
		host = split
	} else if strings.Contains(authority, ":") {
		if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(authority, "["), "]")
		} else {
			return false
		}
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.Zone() == "" && address.IsLoopback()
}

func sameOriginRequest(r *http.Request) bool {
	origins, hasOrigin := r.Header["Origin"]
	if hasOrigin && (len(origins) != 1 || (origins[0] != "" && origins[0] != "http://"+r.Host)) {
		return false
	}
	sites, hasSite := r.Header["Sec-Fetch-Site"]
	return !hasSite || (len(sites) == 1 && (sites[0] == "same-origin" || sites[0] == "none"))
}

func setReportHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", reportCSP)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
}

func writeReportProblem(w http.ResponseWriter, r *http.Request, status int) {
	title := map[int]string{
		http.StatusBadRequest:          "Invalid report request",
		http.StatusForbidden:           "Report request refused",
		http.StatusNotFound:            "Report page not found",
		http.StatusMethodNotAllowed:    "Method not allowed",
		http.StatusInternalServerError: "Report unavailable",
	}[status]
	if title == "" {
		title = "Report unavailable"
	}
	body := []byte("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>" + html.EscapeString(title) + "</title></head><body><main><h1>" + html.EscapeString(title) + "</h1><p><a href=\"/\">Return to receipts report</a></p></main></body></html>")
	writeReportBody(w, r, status, "text/html; charset=utf-8", body, "")
}

func writeReportBody(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte, disposition string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// Package routes extracts HTTP route registrations from source code using
// regex patterns for common web frameworks. All matching is line-oriented and
// uses only stdlib regexp.
package routes

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Route represents a single HTTP route registration found in source code.
type Route struct {
	Method  string // HTTP method, upper-cased: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS, ANY, WS
	Path    string // URL path literal, e.g. "/users/:id"
	Handler string // handler name or anonymous func indicator
	File    string // source file path
	Line    int    // 1-based line number
}

// -----------------------------------------------------------------
// Pattern registry
// -----------------------------------------------------------------

type langPatterns struct {
	patterns []*routePattern
}

type routePattern struct {
	re      *regexp.Regexp
	method  string // fixed method, or "" to read from capture group "method"
	pathGrp string // capture group name for path
	hndGrp  string // capture group name for handler (may be empty)
	// defaultMethod is used when method == "" and the "method" capture group did
	// not match (e.g. a Go net/http route registered without a method prefix).
	// Empty falls back to the global "GET" default.
	defaultMethod string
}

// buildPatterns returns compiled patterns for a given language/framework.
func buildPatterns(lang string) ([]*routePattern, error) {
	switch strings.ToLower(lang) {
	case "go":
		return goPatterns(), nil
	case "js", "javascript", "ts", "typescript":
		return expressPatterns(), nil
	case "python", "py":
		return pythonPatterns(), nil
	default:
		return nil, fmt.Errorf("routes: unsupported language %q", lang)
	}
}

// -----------------------------------------------------------------
// Go patterns: gin, echo, chi, net/http mux
// -----------------------------------------------------------------

func goPatterns() []*routePattern {
	// Common method names for typed-method routers (gin/echo/chi/httprouter).
	// e.g. r.GET("/path", handler) or e.POST("/path", handler)
	typedMethod := regexp.MustCompile(
		`(?m)[\w.]+\.(?P<method>GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any|WS|Connect|Trace)\(\s*` +
			`"(?P<path>[^"]+)"` +
			`(?:\s*,\s*(?P<handler>[A-Za-z_][\w.]*))?\s*`,
	)
	// net/http: mux.HandleFunc("/path", handler) or http.HandleFunc("/path", handler).
	// Go 1.22+ method-prefix patterns are supported: HandleFunc("GET /path", h)
	// → the leading method is captured and the path is the remainder.
	handleFunc := regexp.MustCompile(
		`(?m)[\w.]*\.?HandleFunc\(\s*` +
			`"(?:(?P<method>GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+)?(?P<path>[^"]+)"` +
			`(?:\s*,\s*(?P<handler>[A-Za-z_][\w.]*))?`,
	)
	// mux.Handle("/path", handler) — also supports the Go 1.22+ method prefix.
	handle := regexp.MustCompile(
		`(?m)[\w.]*\.?Handle\(\s*` +
			`"(?:(?P<method>GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+)?(?P<path>[^"]+)"` +
			`(?:\s*,\s*(?P<handler>[A-Za-z_][\w.]*))?`,
	)

	return []*routePattern{
		{re: typedMethod, pathGrp: "path", hndGrp: "handler"},
		// method "" so the optional method-prefix group is read; defaultMethod
		// "ANY" preserves the previous behavior for prefix-less net/http routes.
		{re: handleFunc, method: "", pathGrp: "path", hndGrp: "handler", defaultMethod: "ANY"},
		{re: handle, method: "", pathGrp: "path", hndGrp: "handler", defaultMethod: "ANY"},
	}
}

// -----------------------------------------------------------------
// Express (JS/TS) patterns
// -----------------------------------------------------------------

func expressPatterns() []*routePattern {
	// app.get('/path', handler) or router.post('/path', handler) etc.
	// Supports single or double quoted paths.
	appMethod := regexp.MustCompile(
		`(?m)[\w.]+\.(?P<method>get|post|put|delete|patch|head|options|all|use)\(\s*` +
			`['"](?P<path>[^'"]+)['"]` +
			`(?:\s*,\s*(?P<handler>[A-Za-z_$][\w.$]*))?`,
	)
	return []*routePattern{
		{re: appMethod, pathGrp: "path", hndGrp: "handler"},
	}
}

// -----------------------------------------------------------------
// Python patterns: Flask, FastAPI decorators
// -----------------------------------------------------------------

func pythonPatterns() []*routePattern {
	// @app.route('/path', methods=['GET', 'POST'])
	// @app.route('/path') — defaults to GET
	flaskRoute := regexp.MustCompile(
		`(?m)@[\w.]+\.route\(\s*['"](?P<path>[^'"]+)['"]` +
			`(?:[^)]*methods\s*=\s*\[(?P<methods>[^\]]+)\])?`,
	)
	// @app.get('/path') @app.post('/path') etc. (FastAPI / Flask 2.x)
	fastAPIMethod := regexp.MustCompile(
		`(?m)@[\w.]+\.(?P<method>get|post|put|delete|patch|head|options)\(\s*` +
			`['"](?P<path>[^'"]+)['"]`,
	)
	return []*routePattern{
		{re: flaskRoute, method: "", pathGrp: "path", hndGrp: ""},
		{re: fastAPIMethod, pathGrp: "path", hndGrp: ""},
	}
}

// -----------------------------------------------------------------
// Extract
// -----------------------------------------------------------------

// Extract scans src (source file content) for HTTP route registrations and
// returns a sorted []Route. lang selects the pattern set ("go", "js", "py").
// path is used only to populate Route.File.
func Extract(lang, path string, src []byte) []Route {
	patterns, err := buildPatterns(lang)
	if err != nil {
		return nil
	}

	// Build a line-start byte-offset index for O(1) line lookup.
	lineStarts := buildLineStarts(src)

	var out []Route

	for _, p := range patterns {
		matches := p.re.FindAllSubmatchIndex(src, -1)
		for _, loc := range matches {
			// loc is []int with pairs [start,end] for each capture group.
			// Group 0 = full match, then named groups follow the order in SubexpNames.
			names := p.re.SubexpNames()

			method := p.method
			routePath := ""
			handler := ""

			for gi, name := range names {
				if gi == 0 || gi*2 >= len(loc) {
					continue
				}
				start, end := loc[gi*2], loc[gi*2+1]
				if start < 0 {
					continue
				}
				val := string(src[start:end])
				switch name {
				case "method":
					if method == "" {
						method = strings.ToUpper(val)
					}
				case p.pathGrp:
					routePath = val
				case p.hndGrp:
					if p.hndGrp != "" {
						handler = val
					}
				}
			}

			// For Flask @app.route: parse methods list from the match text.
			if lang == "python" || lang == "py" {
				if p.method == "" && !strings.Contains(p.re.String(), "method>") {
					// This is the flask route pattern; extract methods.
					fullMatch := string(src[loc[0]:loc[1]])
					method = flaskMethods(fullMatch)
				}
			}

			if method == "" {
				method = p.defaultMethod
			}
			if method == "" {
				method = "GET"
			}
			if routePath == "" {
				continue
			}

			line := byteOffsetToLine(lineStarts, loc[0])
			out = append(out, Route{
				Method:  strings.ToUpper(method),
				Path:    routePath,
				Handler: handler,
				File:    path,
				Line:    line,
			})
		}
	}

	sortRoutes(out)
	return out
}

// flaskMethods extracts methods from a flask @app.route match string.
// Returns comma-joined uppercased methods, or "GET" if absent.
func flaskMethods(matchText string) string {
	methodsRe := regexp.MustCompile(`methods\s*=\s*\[([^\]]+)\]`)
	m := methodsRe.FindStringSubmatch(matchText)
	if m == nil {
		return "GET"
	}
	raw := m[1]
	var methods []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `'"`)
		if part != "" {
			methods = append(methods, strings.ToUpper(part))
		}
	}
	sort.Strings(methods)
	return strings.Join(methods, ",")
}

// buildLineStarts returns a slice where index i holds the byte offset of line i+1.
func buildLineStarts(src []byte) []int {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' && i+1 < len(src) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// byteOffsetToLine does a binary search over lineStarts to find the 1-based line.
func byteOffsetToLine(lineStarts []int, offset int) int {
	lo, hi := 0, len(lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1 // 1-based
}

// sortRoutes sorts routes by (Method, Path, File, Line) for determinism.
func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		a, b := routes[i], routes[j]
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}

// splitLines is a helper used in tests.
func splitLines(src []byte) []string {
	return strings.Split(string(bytes.TrimRight(src, "\n")), "\n")
}

// ensure splitLines is referenced (avoids "declared and not used" if only in tests)
var _ = splitLines

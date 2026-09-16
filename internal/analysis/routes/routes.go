// Package routes extracts HTTP route registrations from source code. The Go
// path is a real go/ast walk (go/parser + go/ast, never a byte-level
// regex — VB-31 M8, docs/plans/verbs.md, docs/bench/readings/
// vb31-service-ref-m8.md); JS/TS and Python stay line-oriented regex over
// stdlib regexp, unchanged and out of scope for that measurement.
package routes

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
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

// Extract scans src (source file content) for HTTP route registrations and
// returns a sorted []Route. lang selects the pattern set ("go", "js", "py").
// path is used only to populate Route.File. The Go path is a real AST walk
// (extractGo); JS/TS and Python remain the pre-existing regex extractors.
func Extract(lang, path string, src []byte) []Route {
	switch strings.ToLower(lang) {
	case "go":
		return extractGo(path, src)
	case "js", "javascript", "ts", "typescript", "python", "py":
		return extractRegex(lang, path, src)
	default:
		return nil
	}
}

// -----------------------------------------------------------------
// Go: AST-based extraction (VB-31 M8)
// -----------------------------------------------------------------
//
// extractGo recognizes three route-registration shapes, all found by
// walking real ast.CallExpr nodes (a route written inside a `//` comment
// or `/* */` block is plain text to go/parser, never an AST node, so it
// can never manufacture a route — the same comment-immunity
// internal/analysis/serviceref.ExtractClientCalls documents for client
// calls):
//
//   - Typed-method routers (gin/echo/chi/httprouter): r.GET(path, handler),
//     r.POST(...), etc. — the method name IS the HTTP method.
//   - net/http: mux.HandleFunc(path, handler), mux.Handle(path, handler),
//     including the Go 1.22+ method-prefix convention embedded in the path
//     literal itself ("GET /path").
//   - gorilla/mux call chains, decomposed as a real chain of selector
//     calls regardless of the order they're written in:
//     r.HandleFunc(path, handler).Methods("GET", ...), the reversed
//     r.Methods("POST").Path(path).HandlerFunc(handler), and
//     PathPrefix(prefix).Subrouter() — a subrouter assigned to a variable
//     propagates its accumulated prefix to every route later registered
//     through that variable, tracked across statements in the same file
//     (collectRouterPrefixes). A chain's own PathPrefix link(s) stack on
//     top of that.
//
// A route path built by string concatenation is folded the way go/constant
// would evaluate it: a package-level string const (e.g. NameExpression in
// `"/x/{name:["+NameExpression+"]+}"`) resolves to its literal value: a
// genuinely non-constant fragment (a variable, a field access) normalizes
// to the placeholder "{}" instead of aborting the whole route — parity
// with ExtractClientCalls's own "parameters normalize to {}" rule — but an
// entirely unrecognized path expression shape (a function call, an index
// expression, …) skips the route rather than guessing: precision over
// recall, the same doctrine the client side applies. A file that fails to
// parse contributes no routes.
func extractGo(path string, src []byte) []Route {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil
	}

	g := &goRouteExtractor{
		fset:         fset,
		src:          src,
		path:         path,
		consts:       collectStringConsts(file),
		routerPrefix: map[string]string{},
	}
	g.collectRouterPrefixes(file)

	consumed := map[*ast.CallExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if consumed[call] {
			return true // already handled as part of an outer chain (see markChainConsumed)
		}
		base, links := decomposeChain(call)
		markChainConsumed(call, consumed)
		g.tryRoute(base, links, call.Pos())
		return true
	})

	sortRoutes(g.out)
	return g.out
}

type goRouteExtractor struct {
	fset         *token.FileSet
	src          []byte
	path         string
	consts       map[string]string // package-level string consts, for concatenation folding
	routerPrefix map[string]string // variable name -> accumulated PathPrefix (subrouter tracking)
	out          []Route
}

// chainLink is one `.Name(args...)` step of a decomposed method-call chain.
type chainLink struct {
	name string
	args []ast.Expr
}

// decomposeChain unwinds e as a chain of selector calls, outermost first:
// `base.A(...).B(...).C(...)` becomes (base, [C, B, A]). Stops at the first
// expression that isn't itself `<selector-call>.X` — typically an
// identifier (a router variable), but harmlessly anything else too (e.g. a
// bare function call with no further chaining).
func decomposeChain(e ast.Expr) (ast.Expr, []chainLink) {
	var links []chainLink
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return e, links
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return e, links
		}
		links = append(links, chainLink{name: sel.Sel.Name, args: call.Args})
		e = sel.X
	}
}

// markChainConsumed marks every ast.CallExpr node decomposeChain would walk
// through as already handled, so the top-down ast.Inspect traversal in
// extractGo does not re-decompose (and duplicate) an inner link of a chain
// it already processed via its outer call.
func markChainConsumed(e ast.Expr, consumed map[*ast.CallExpr]bool) {
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return
		}
		consumed[call] = true
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		e = sel.X
	}
}

// collectRouterPrefixes scans `name := <chain>` / `name = <chain>`
// assignments whose chain contains a PathPrefix and/or Subrouter link
// (gorilla/mux's subrouter idiom) and records each variable's accumulated
// path prefix. A single top-to-bottom pass (ast.Inspect visits statements
// in source order), so a subrouter derived from another tracked subrouter
// resolves correctly as long as it's declared after its base — the
// ordinary way this idiom is written; out of scope: a subrouter declared
// before the variable it derives from.
func (g *goRouteExtractor) collectRouterPrefixes(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok || id.Name == "_" {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		base, links := decomposeChain(call)

		var prefixArgs []ast.Expr
		sawPrefixOrSub := false
		for _, l := range links {
			switch l.name {
			case "PathPrefix":
				sawPrefixOrSub = true
				if len(l.args) > 0 {
					prefixArgs = append(prefixArgs, l.args[0])
				}
			case "Subrouter":
				sawPrefixOrSub = true
			}
		}
		if !sawPrefixOrSub {
			return true
		}

		basePrefix := ""
		if bi, ok := base.(*ast.Ident); ok {
			basePrefix = g.routerPrefix[bi.Name] // "" for the untracked root router — the correct default
		}
		seg := ""
		for _, pe := range prefixArgs {
			if folded, ok := g.foldPathExpr(pe); ok {
				seg = joinPathSegments(seg, folded)
			}
		}
		g.routerPrefix[id.Name] = joinPathSegments(basePrefix, seg)
		return true
	})
}

// typedMethodNames is the typed-method-router alternation (gin/echo/chi/
// httprouter): a call named exactly one of these keys IS a route
// registration with that fixed HTTP method. Case follows the pre-AST
// regex's own alternation exactly (Any/WS/Connect/Trace are mixed-case).
var typedMethodNames = map[string]string{
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE",
	"PATCH": "PATCH", "HEAD": "HEAD", "OPTIONS": "OPTIONS",
	"Any": "ANY", "WS": "WS", "Connect": "CONNECT", "Trace": "TRACE",
}

// go122MethodPrefixRe splits net/http's Go 1.22+ method-prefix convention
// ("GET /path") off a resolved HandleFunc/Handle path literal.
var go122MethodPrefixRe = regexp.MustCompile(`^(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+(.*)$`)

// tryRoute inspects one decomposed call chain (base + links) for a
// recognized route-registration shape and, if found, appends one Route per
// resulting (method, path) pair to g.out — see extractGo's doc comment for
// the three recognized shapes.
func (g *goRouteExtractor) tryRoute(base ast.Expr, links []chainLink, pos token.Pos) {
	var pathExpr, handlerExpr ast.Expr
	var methodArgs, prefixArgs []ast.Expr
	fixedMethod := ""
	sawPathBearingLink := false

	for _, l := range links {
		switch {
		case typedMethodNames[l.name] != "":
			fixedMethod = typedMethodNames[l.name]
			sawPathBearingLink = true
			if len(l.args) > 0 {
				pathExpr = l.args[0]
			}
			if len(l.args) > 1 {
				handlerExpr = l.args[1]
			}
		case l.name == "HandleFunc" || l.name == "Handle":
			sawPathBearingLink = true
			if len(l.args) > 0 {
				pathExpr = l.args[0]
			}
			if len(l.args) > 1 {
				handlerExpr = l.args[1]
			}
		case l.name == "Path":
			sawPathBearingLink = true
			if len(l.args) > 0 {
				pathExpr = l.args[0]
			}
		case l.name == "HandlerFunc":
			if len(l.args) > 0 {
				handlerExpr = l.args[0]
			}
		case l.name == "Methods":
			methodArgs = append(methodArgs, l.args...)
		case l.name == "PathPrefix":
			if len(l.args) > 0 {
				prefixArgs = append(prefixArgs, l.args[0])
			}
		}
	}
	if !sawPathBearingLink || pathExpr == nil {
		return
	}

	rawPath, ok := g.foldPathExpr(pathExpr)
	if !ok {
		return // unrecognized path expression shape — skip rather than guess
	}

	prefix := ""
	if id, ok := base.(*ast.Ident); ok {
		prefix = g.routerPrefix[id.Name]
	}
	for _, pe := range prefixArgs {
		if seg, ok := g.foldPathExpr(pe); ok {
			prefix = joinPathSegments(prefix, seg)
		}
	}
	fullPath := joinPathSegments(prefix, rawPath)

	handler := ""
	if handlerExpr != nil {
		handler = exprLabel(handlerExpr)
	}
	line := g.fset.Position(pos).Line

	if fixedMethod != "" {
		g.out = append(g.out, Route{Method: fixedMethod, Path: fullPath, Handler: handler, File: g.path, Line: line})
		return
	}

	if len(methodArgs) > 0 {
		seen := map[string]bool{}
		for _, ma := range methodArgs {
			m, ok := resolveHTTPMethodArg(ma)
			if !ok || seen[m] {
				continue
			}
			seen[m] = true
			g.out = append(g.out, Route{Method: m, Path: fullPath, Handler: handler, File: g.path, Line: line})
		}
		return
	}

	// No explicit .Methods(...) chain: net/http's Go 1.22+ method-prefix
	// convention, if present, lives inside the path literal itself.
	method := "ANY"
	if m := go122MethodPrefixRe.FindStringSubmatch(fullPath); m != nil {
		method = m[1]
		fullPath = m[2]
	}
	g.out = append(g.out, Route{Method: method, Path: fullPath, Handler: handler, File: g.path, Line: line})
}

// foldPathExpr resolves a route-path expression the way go/constant would
// evaluate it: a string literal or a package-level string const resolves
// to its literal value; string concatenation (+) folds both sides
// independently and joins them; a genuinely non-constant leaf (a plain
// variable or field reference — the value obviously varies per request,
// e.g. a route-table entry) normalizes to the placeholder "{}" rather than
// guessing its runtime value. Any other expression shape (a function call,
// an index expression, …) is unrecognized and returns ("", false) — the
// caller skips the route entirely rather than fabricate a path.
func (g *goRouteExtractor) foldPathExpr(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return g.foldPathExpr(v.X)
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return s, true
			}
		}
		return "", false
	case *ast.Ident:
		if s, ok := g.consts[v.Name]; ok {
			return s, true
		}
		return "{}", true
	case *ast.SelectorExpr:
		return "{}", true
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		lhs, ok1 := g.foldPathExpr(v.X)
		rhs, ok2 := g.foldPathExpr(v.Y)
		if !ok1 || !ok2 {
			return "", false
		}
		return lhs + rhs, true
	}
	return "", false
}

// joinPathSegments concatenates a prefix and a path segment with exactly
// one "/" between them (never collapsing or Cleaning further — a folded
// "{}" placeholder must survive byte-for-byte).
func joinPathSegments(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return strings.TrimRight(a, "/") + "/" + strings.TrimLeft(b, "/")
}

// resolveHTTPMethodArg resolves one `.Methods(...)` argument: a quoted
// string literal ("GET") or a net/http Method* constant referenced as
// http.MethodGet, resolved by name (this package has no go/types pass) —
// the same heuristic internal/analysis/serviceref.httpMethodConstants
// documents.
func resolveHTTPMethodArg(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return strings.ToUpper(s), true
			}
		}
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "http" {
			if m, ok := httpMethodConstantsGo[v.Sel.Name]; ok {
				return m, true
			}
		}
	}
	return "", false
}

var httpMethodConstantsGo = map[string]string{
	"MethodGet": "GET", "MethodHead": "HEAD", "MethodPost": "POST",
	"MethodPut": "PUT", "MethodPatch": "PATCH", "MethodDelete": "DELETE",
	"MethodConnect": "CONNECT", "MethodOptions": "OPTIONS", "MethodTrace": "TRACE",
}

// exprLabel renders a best-effort, non-load-bearing handler label — never
// scored (goldencorpus_test.go's ground truth never checks Route.Handler),
// purely descriptive.
func exprLabel(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprLabel(v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprLabel(v.Fun)
	case *ast.UnaryExpr:
		return exprLabel(v.X)
	case *ast.ParenExpr:
		return exprLabel(v.X)
	}
	return ""
}

// collectStringConsts gathers every package-level `const Name = "literal"`
// declaration in file — a simple, non-recursive map used only to fold a
// route path's string concatenation (foldPathExpr). Consts defined via an
// expression other than a plain string literal (iota, a reference to
// another const, …) are not resolved — same "skip rather than guess"
// doctrine as everything else in this file.
func collectStringConsts(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if s, err := strconv.Unquote(lit.Value); err == nil {
					out[name.Name] = s
				}
			}
		}
	}
	return out
}

// -----------------------------------------------------------------
// JS/TS and Python: line-oriented regex extraction (unchanged)
// -----------------------------------------------------------------

type routePattern struct {
	re      *regexp.Regexp
	method  string // fixed method, or "" to read from capture group "method"
	pathGrp string // capture group name for path
	hndGrp  string // capture group name for handler (may be empty)
	// defaultMethod is used when method == "" and the "method" capture group did
	// not match. Empty falls back to the global "GET" default.
	defaultMethod string
}

// buildPatterns returns compiled patterns for a given language/framework.
func buildPatterns(lang string) ([]*routePattern, error) {
	switch strings.ToLower(lang) {
	case "js", "javascript", "ts", "typescript":
		return expressPatterns(), nil
	case "python", "py":
		return pythonPatterns(), nil
	default:
		return nil, fmt.Errorf("routes: unsupported language %q", lang)
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
// extractRegex
// -----------------------------------------------------------------

// extractRegex scans src for HTTP route registrations using the
// line-oriented regex patterns for JS/TS and Python. Unchanged by VB-31 —
// only the Go path (extractGo) is AST-based.
func extractRegex(lang, path string, src []byte) []Route {
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

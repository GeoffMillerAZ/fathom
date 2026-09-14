// Package serviceref extracts the `service_ref` cross-repo edge kind
// (VB-30, docs/vision/29-verbs-not-lookups.md §I; docs/plans/verbs.md
// VB-30/VB-31): a directed, tier-`heuristic` link from a client call site
// in one service to a route registration in ANOTHER service, joined on a
// normalized (HTTP method, path template) key.
//
// Two extraction halves feed the join:
//
//   - Server side: internal/analysis/routes.Extract (net/http, chi, gin,
//     echo route registrations) — reused unchanged, since it already
//     resolves method + path + handler + file:line for every fauxtown
//     language routes.Extract supports.
//   - Client side: ExtractClientCalls in this package — Go only for VB-30
//     (vision 02 §4.7: edge kinds grow one at a time, and so does language
//     coverage for a brand-new one), parsed with go/parser and walked as
//     real ast.CallExpr nodes (never a byte-level regex over raw source —
//     a call written inside a `//` comment is plain text to the parser,
//     never an AST node, so it can never manufacture a hop) detecting
//     http.NewRequest / http.NewRequestWithContext / http.Get / http.Post /
//     http.Head / http.PostForm call sites. The method argument accepts a
//     quoted literal ("GET") or a net/http Method* constant
//     (http.MethodGet); the URL argument accepts a string literal or an
//     inline fmt.Sprintf("template", args...) call, whose %-verbs
//     NormalizeTemplate collapses to "{}".
//
// NormalizeTemplate collapses `:param` (segment-anchored), `{param}`, a
// `*wildcard` segment (anywhere in the path, not only the last one), and
// printf verbs (`%s`, `%d`, `%v`, …) into one canonical placeholder ("{}"),
// strips a leading scheme+host (only when the string STARTS with one — see
// its doc comment for why that anchor matters), strips a query
// string/fragment, and normalises a trailing slash — so a server route
// written ":id"-style and a client URL template written "%s"-style compare
// equal.
//
// The join (Join / ServiceRefEdges) is pure, deterministic (sorted output),
// and cross-repo only: a client call and a route in the SAME service never
// produce a hop (that is an ordinary intra-service call edge, not a
// service_ref). A (method, template) key registered by MORE than one
// service fans out to every one of them (the true blast radius — edges are
// never dropped for being ambiguous) and each resulting Hop carries
// Ambiguous:true plus the full sorted Candidates list, so a consumer can
// tell a fan-out from a unique join instead of silently trusting one.
// Every edge this package produces is tier `heuristic` — never typed
// (core.EdgeConfidenceOf's vocabulary; see core.CrossRepoEdgeConfidenceOf)
// — because a normalized-string match is never confirmed by a
// compiler-grade resolver.
package serviceref

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Kind is the core.CrossRepoEdge.Kind / edge-report "kind" value every hop
// this package produces carries (core.CrossRepoEdgeServiceRef).
const Kind = string(core.CrossRepoEdgeServiceRef)

// Tier is the edge-confidence "tier" value every hop this package produces
// carries — always heuristic (core.EdgeConfidenceHeuristic), never typed.
const Tier = string(core.EdgeConfidenceHeuristic)

// ClientCall is one outbound HTTP client call site detected by
// ExtractClientCalls.
type ClientCall struct {
	Method   string // HTTP method, upper-cased: GET, POST, PUT, DELETE, PATCH, HEAD
	Template string // NormalizeTemplate'd path, e.g. "/items/{}"
	RawURL   string // the URL expression's exact source text before normalization (debugging/docs)
	Symbol   string // best-effort enclosing function name; "" if unknown
	File     string
	Line     int // 1-based
}

// Hop is one cross-repo service_ref edge: a client call in FromService
// joined to a route registration in ToService on a matching normalized
// (Method, Template).
type Hop struct {
	FromService string
	FromFile    string
	FromLine    int
	FromSymbol  string // enclosing function of the client call site; "" if unknown
	ToService   string
	ToFile      string
	ToLine      int
	ToHandler   string // route handler name; "" for an anonymous/unnamed registration
	Method      string // normalized upper-case HTTP method
	Template    string // normalized path template, e.g. "/items/{}"
	// Ambiguous is true when more than one service registers a route
	// matching (Method, Template) for this call — Candidates then names
	// every one of them (sorted, including ToService), so a consumer can
	// tell a genuine fan-out from a unique join. false with a
	// single-element Candidates for a unique join.
	Ambiguous  bool
	Candidates []string
}

// CrossRepoEdge renders h as a core.CrossRepoEdge (Kind ==
// core.CrossRepoEdgeServiceRef, ToKey == "METHOD /template" — the same
// "METHOD path" convention internal/mcp's traverse tool uses for
// graph.KindRoute).
func (h Hop) CrossRepoEdge() core.CrossRepoEdge {
	return core.CrossRepoEdge{
		FromService: h.FromService,
		FromFile:    h.FromFile,
		ToService:   h.ToService,
		ToKey:       h.Method + " " + h.Template,
		ToFile:      h.ToFile,
		Kind:        core.CrossRepoEdgeServiceRef,
	}
}

// Payload renders h as a plain map[string]any for MCP tool output —
// shared by blast_radius, cross_repo_blast_radius and review_diff so their
// service_ref reporting never drifts into three slightly different shapes.
// Always carries kind:"service_ref" and tier:"heuristic".
func (h Hop) Payload() map[string]any {
	return map[string]any{
		"from_service": h.FromService,
		"from_file":    h.FromFile,
		"from_line":    h.FromLine,
		"from_symbol":  h.FromSymbol,
		"to_service":   h.ToService,
		"to_file":      h.ToFile,
		"to_line":      h.ToLine,
		"to_handler":   h.ToHandler,
		"method":       h.Method,
		"template":     h.Template,
		"kind":         Kind,
		"tier":         Tier,
		"ambiguous":    h.Ambiguous,
		"candidates":   h.Candidates,
	}
}

// sortHops sorts hops for deterministic output. Every identity field is a
// comparison key (in declaration order) so the sort is TOTAL: two distinct
// hops never compare equal and land in an order that depends on map
// iteration. Ambiguous/Candidates are omitted — they are functionally
// determined by (FromService, FromFile, FromLine, Method, Template) alone,
// so they can never distinguish two hops the fields above call equal.
func sortHops(hops []Hop) {
	sort.Slice(hops, func(i, j int) bool {
		a, b := hops[i], hops[j]
		switch {
		case a.FromService != b.FromService:
			return a.FromService < b.FromService
		case a.FromFile != b.FromFile:
			return a.FromFile < b.FromFile
		case a.FromLine != b.FromLine:
			return a.FromLine < b.FromLine
		case a.FromSymbol != b.FromSymbol:
			return a.FromSymbol < b.FromSymbol
		case a.ToService != b.ToService:
			return a.ToService < b.ToService
		case a.ToFile != b.ToFile:
			return a.ToFile < b.ToFile
		case a.ToLine != b.ToLine:
			return a.ToLine < b.ToLine
		case a.ToHandler != b.ToHandler:
			return a.ToHandler < b.ToHandler
		case a.Method != b.Method:
			return a.Method < b.Method
		default:
			return a.Template < b.Template
		}
	})
}

// routeKey is the join key: a normalized (Method, Template) pair.
type routeKey struct {
	method   string
	template string
}

// ServiceRefEdges is the package's main entry point (VB-30, symbol anchor
// docs/plans/verbs.md): joins each service's outbound HTTP client calls
// against every OTHER service's registered HTTP routes, matching on
// normalized (method, path template). Pure and deterministic — sorted
// output, no I/O, no shared state. A client call and a route belonging to
// the SAME service never join (that is an intra-service call edge, not a
// cross-repo service_ref). A key matched by more than one OTHER service
// fans out to all of them (edges are never dropped for being ambiguous —
// see Hop.Ambiguous).
func ServiceRefEdges(routesByService map[string][]routes.Route, callsByService map[string][]ClientCall) []Hop {
	// Index every service's routes by (method, normalized template).
	routeIdx := make(map[string]map[routeKey][]routes.Route, len(routesByService))
	for svc, rts := range routesByService {
		m := make(map[routeKey][]routes.Route)
		for _, r := range rts {
			k := routeKey{method: strings.ToUpper(r.Method), template: NormalizeTemplate(r.Path)}
			m[k] = append(m[k], r)
		}
		routeIdx[svc] = m
	}

	var hops []Hop
	for fromSvc, calls := range callsByService {
		for _, call := range calls {
			k := routeKey{method: call.Method, template: call.Template}

			// Candidate services: every OTHER service whose route index has
			// at least one route matching k. Collected once per call (not
			// per resulting hop) so ambiguity is judged over the WHOLE set
			// this call joins to, not per edge.
			var candidates []string
			for toSvc, m := range routeIdx {
				if toSvc == fromSvc {
					continue // intra-service: not a cross-repo edge
				}
				if len(m[k]) > 0 {
					candidates = append(candidates, toSvc)
				}
			}
			if len(candidates) == 0 {
				continue
			}
			sort.Strings(candidates)
			ambiguous := len(candidates) > 1

			for _, toSvc := range candidates {
				for _, r := range routeIdx[toSvc][k] {
					hops = append(hops, Hop{
						FromService: fromSvc,
						FromFile:    call.File,
						FromLine:    call.Line,
						FromSymbol:  call.Symbol,
						ToService:   toSvc,
						ToFile:      r.File,
						ToLine:      r.Line,
						ToHandler:   r.Handler,
						Method:      k.method,
						Template:    k.template,
						Ambiguous:   ambiguous,
						Candidates:  candidates,
					})
				}
			}
		}
	}
	sortHops(hops)
	return hops
}

// Join is an alias for ServiceRefEdges kept for readability at call sites
// that already have both maps in hand (e.g. tests); identical behavior.
func Join(routesByService map[string][]routes.Route, callsByService map[string][]ClientCall) []Hop {
	return ServiceRefEdges(routesByService, callsByService)
}

// ---------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------

var (
	// schemeRe matches a URI scheme+"://" ANCHORED at the string's start
	// ("^") — see NormalizeTemplate's doc comment for why that anchor is
	// load-bearing, not cosmetic.
	schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
	// braceParamRe and printfVerbRe both match patterns that cannot
	// contain "/" by their own grammar, so applying them to the whole
	// path string (rather than per path segment) can never cross a
	// segment boundary.
	braceParamRe = regexp.MustCompile(`\{[^/{}]+\}`)
	// printfVerbRe matches a fmt verb: %, optional flags, optional
	// width/precision, one conversion letter (%s, %d, %v, %02d, %.2f, …).
	printfVerbRe = regexp.MustCompile(`%[-+ 0#]*[0-9]*(?:\.[0-9]+)?[a-zA-Z]`)
	// colonParamSegRe / starParamSegRe are matched against one PATH
	// SEGMENT at a time (never the whole path) — see NormalizeTemplate.
	colonParamSegRe = regexp.MustCompile(`^:[A-Za-z_][A-Za-z0-9_]*$`)
	starParamSegRe  = regexp.MustCompile(`^\*[A-Za-z_][A-Za-z0-9_]*$`)
)

// NormalizeTemplate collapses a server route path or a client URL template
// into one canonical form so the two conventions compare equal:
//
//   - A leading scheme+host is stripped ONLY when the string STARTS with
//     one (schemeRe's "^" anchor) — an unanchored search for "://" would
//     be fooled by a decoy like "/redirect?next=http://evil.com/path"
//     (which must normalise to "/redirect", never "/path": the fake
//     scheme+host lives inside the QUERY STRING, not at the template's
//     own start).
//   - A query string ("?...") and/or fragment ("#...") is stripped —
//     whichever starts first.
//   - A trailing slash is normalised away, except for the bare root "/",
//     so "/items/" joins "/items".
//   - `{param}` (gorilla/echo, including a `{param:regex}` constraint) and
//     a printf verb (`%s`, `%d`, `%v`, `%02d`, …) are replaced with one
//     placeholder, "{}", wherever they appear in the path.
//   - `:param` (chi/gin/express) and a `*wildcard` segment are replaced
//     with "{}" too, but ONLY when SEGMENT-ANCHORED: a colon must be the
//     FIRST character of its "/"-delimited segment ("/events/at:noon"
//     stays completely literal — the colon is mid-segment, not a param
//     marker), and a "*" must form the segment's ENTIRE content (bare "*",
//     or a named "*rest") — checked in every segment, not only the last
//     one, since a real router can register a wildcard anywhere.
//
// The result always starts with "/". Pure and deterministic.
func NormalizeTemplate(raw string) string {
	s := raw

	if loc := schemeRe.FindStringIndex(s); loc != nil {
		rest := s[loc[1]:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			s = rest[slash:]
		} else {
			s = "/"
		}
	}

	if idx := strings.IndexAny(s, "?#"); idx >= 0 {
		s = s[:idx]
	}

	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}

	if len(s) > 1 && strings.HasSuffix(s, "/") {
		s = strings.TrimRight(s, "/")
		if s == "" {
			s = "/"
		}
	}

	s = printfVerbRe.ReplaceAllString(s, "{}")
	s = braceParamRe.ReplaceAllString(s, "{}")

	segs := strings.Split(s, "/")
	for i, seg := range segs {
		switch {
		case colonParamSegRe.MatchString(seg):
			segs[i] = "{}"
		case seg == "*" || starParamSegRe.MatchString(seg):
			segs[i] = "{}"
		}
	}
	return strings.Join(segs, "/")
}

// ---------------------------------------------------------------------
// Client-call extraction (Go only for VB-30)
// ---------------------------------------------------------------------

// httpCallPattern names one detected stdlib net/http client call shape,
// matched against a *ast.SelectorExpr whose X is the identifier "http".
type httpCallPattern struct {
	methodArg   int    // arg index carrying the HTTP method; -1 = fixed method
	fixedMethod string // used when methodArg == -1
	urlArg      int    // arg index carrying the URL expression
}

var httpCallPatterns = map[string]httpCallPattern{
	"NewRequestWithContext": {methodArg: 1, urlArg: 2},
	"NewRequest":            {methodArg: 0, urlArg: 1},
	"Get":                   {methodArg: -1, fixedMethod: "GET", urlArg: 0},
	"Head":                  {methodArg: -1, fixedMethod: "HEAD", urlArg: 0},
	"Post":                  {methodArg: -1, fixedMethod: "POST", urlArg: 0},
	"PostForm":              {methodArg: -1, fixedMethod: "POST", urlArg: 0},
}

// httpMethodConstants resolves a net/http Method* constant reference
// (http.MethodGet) to its string value — BY NAME, not by type-checked
// identifier resolution (this package has no go/types pass), which is a
// heuristic like the rest of this extractor: it trusts that a selector
// expression whose package identifier reads "http" really is net/http,
// true for every import-alias-free usage a real codebase writes.
var httpMethodConstants = map[string]string{
	"MethodGet":     "GET",
	"MethodHead":    "HEAD",
	"MethodPost":    "POST",
	"MethodPut":     "PUT",
	"MethodPatch":   "PATCH",
	"MethodDelete":  "DELETE",
	"MethodConnect": "CONNECT",
	"MethodOptions": "OPTIONS",
	"MethodTrace":   "TRACE",
}

// funcRange is one *ast.FuncDecl's [start, end) position span — used only
// to resolve ClientCall.Symbol, a best-effort label never load-bearing for
// the join itself.
type funcRange struct {
	name       string
	start, end token.Pos
}

// ExtractClientCalls parses src as Go source and walks it as an AST — a
// call written inside a `//` comment or a `/* */` block is plain text to
// go/parser, never an ast.CallExpr, so it can never manufacture a hop the
// way a byte-level regex over raw source could — looking for outbound
// net/http client call sites, and returns each one's normalized (method,
// template), sorted. lang selects the pattern set; only "go" is supported
// today (nil for anything else — language coverage for a new edge kind
// grows one language at a time, same as VB-30 grows one edge kind at a
// time). A src that fails to parse (a fragment, a file mid-refactor, a
// generated file with unusual syntax) contributes no calls rather than a
// guess — stricter than internal/analysis/routes.Extract's line-regex
// tolerance of unparsable fragments, a deliberate asymmetry: only a real
// AST walk can distinguish a call expression from one inside a comment,
// and that needs a complete, parseable file.
func ExtractClientCalls(lang, path string, src []byte) []ClientCall {
	if strings.ToLower(lang) != "go" {
		return nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil
	}

	var ranges []funcRange
	ast.Inspect(file, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			ranges = append(ranges, funcRange{name: fd.Name.Name, start: fd.Pos(), end: fd.End()})
		}
		return true
	})

	var out []ClientCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "http" {
			return true
		}
		pat, ok := httpCallPatterns[sel.Sel.Name]
		if !ok {
			return true
		}

		method := pat.fixedMethod
		if pat.methodArg >= 0 {
			if pat.methodArg >= len(call.Args) {
				return true
			}
			m, ok := methodFromExpr(call.Args[pat.methodArg])
			if !ok {
				return true // method not resolvable — skip rather than guess
			}
			method = m
		}
		if pat.urlArg >= len(call.Args) {
			return true
		}
		rawURL, template, ok := templateFromExpr(call.Args[pat.urlArg], src, fset)
		if !ok {
			return true // URL expression not resolvable to a literal/Sprintf template — skip
		}

		out = append(out, ClientCall{
			Method:   method,
			Template: NormalizeTemplate(template),
			RawURL:   rawURL,
			Symbol:   enclosingFuncName(ranges, call.Pos()),
			File:     path,
			Line:     fset.Position(call.Pos()).Line,
		})
		return true
	})

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		return a.Template < b.Template
	})
	return out
}

// enclosingFuncName returns the name of the FuncDecl whose range contains
// pos — a best-effort, non-load-bearing label for ClientCall.Symbol. ""
// when none is found (a call at file scope, or inside a FuncLit — a
// closure inherits no name here, which is honest: an anonymous function
// has none).
func enclosingFuncName(ranges []funcRange, pos token.Pos) string {
	for _, r := range ranges {
		if pos >= r.start && pos < r.end {
			return r.name
		}
	}
	return ""
}

// methodFromExpr resolves a call argument to an HTTP method string: a
// quoted string literal ("GET"), or a net/http Method* constant referenced
// as http.MethodGet (see httpMethodConstants's doc comment for the
// resolve-by-name heuristic). Any other expression shape returns ("",
// false) rather than guessing.
func methodFromExpr(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return strings.ToUpper(s), true
			}
		}
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "http" {
			if m, ok := httpMethodConstants[v.Sel.Name]; ok {
				return m, true
			}
		}
	}
	return "", false
}

// templateFromExpr resolves a client call's URL argument expression to a
// (rawText, template) pair:
//
//   - A plain string literal is used as-is.
//   - An inline fmt.Sprintf("template", args...) call resolves to its own
//     first argument (the format-string literal) — the args themselves
//     become printf verbs NormalizeTemplate collapses to "{}".
//
// rawText is the expression's exact original source text (ClientCall.RawURL,
// debugging/docs only, never used for extraction logic). Any other
// expression shape (a bare identifier, a multi-line concat, a non-Sprintf
// helper) is out of scope for VB-30 and returns ("", "", false) rather than
// guessing.
func templateFromExpr(e ast.Expr, src []byte, fset *token.FileSet) (raw string, template string, ok bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return sourceText(src, fset, e), s, true
			}
		}
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", "", false
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "fmt" || sel.Sel.Name != "Sprintf" || len(v.Args) == 0 {
			return "", "", false
		}
		lit, ok := v.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", "", false
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return "", "", false
		}
		return sourceText(src, fset, e), s, true
	}
	return "", "", false
}

// sourceText returns e's exact original source text via its recorded file
// positions.
func sourceText(src []byte, fset *token.FileSet, e ast.Expr) string {
	start := fset.Position(e.Pos()).Offset
	end := fset.Position(e.End()).Offset
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

// ---------------------------------------------------------------------
// Fleet-wide, store-backed extraction
// ---------------------------------------------------------------------

// SourceLister is the minimal backend capability BuildFromStore needs — any
// backend that can list a service's source files (internal/mcp's
// QueryInterface and pkg/client.Client both satisfy it structurally).
type SourceLister interface {
	Sources(ctx context.Context, service string) ([]core.SourceFile, error)
}

// routesLang maps a core.SourceFile.Lang tag to the language token
// routes.Extract expects. "" (routes.Extract's own accepted set already
// covers go/js/ts/py directly) passes through unchanged.
func routesLang(lang string) string {
	switch lang {
	case "go", "js", "javascript", "ts", "typescript", "python", "py":
		return lang
	default:
		return ""
	}
}

// BuildFromStore extracts every service's route registrations and outbound
// HTTP client calls from already-ingested source (backend.Sources) and
// joins them into the fleet's service_ref hops (ServiceRefEdges). Language-
// agnostic on the SERVER side (routes.Extract's own go/js/py support);
// Go-only on the CLIENT side today (ExtractClientCalls). Deterministic for
// a fixed store state: serviceIDs is sorted before processing and every
// per-file result set is already sorted by its extractor.
func BuildFromStore(ctx context.Context, backend SourceLister, serviceIDs []string) ([]Hop, error) {
	svcs := append([]string(nil), serviceIDs...)
	sort.Strings(svcs)

	routesByService := make(map[string][]routes.Route, len(svcs))
	callsByService := make(map[string][]ClientCall, len(svcs))
	for _, svc := range svcs {
		files, err := backend.Sources(ctx, svc)
		if err != nil {
			return nil, fmt.Errorf("serviceref: sources %q: %w", svc, err)
		}
		for _, f := range files {
			if rl := routesLang(f.Lang); rl != "" {
				routesByService[svc] = append(routesByService[svc], routes.Extract(rl, f.Path, f.Content)...)
			}
			callsByService[svc] = append(callsByService[svc], ExtractClientCalls(f.Lang, f.Path, f.Content)...)
		}
	}
	return ServiceRefEdges(routesByService, callsByService), nil
}

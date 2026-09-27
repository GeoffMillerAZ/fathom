// Package serviceref extracts the `service_ref` cross-repo edge kind
// (VB-30, docs/vision/29-verbs-not-lookups.md §I; docs/plans/verbs.md
// VB-30/VB-31): a directed, tier-`heuristic` link from a client call site
// in one service to a route registration in ANOTHER service, joined on a
// normalized (HTTP method, path template) key.
//
// Two extraction halves feed the join:
//
//   - Server side: internal/analysis/routes.Extract — AST-based for Go
//     (net/http, chi, gin, echo, gorilla/mux chains), regex-based for
//     JS/TS/Python — reused unchanged from this package's side, since it
//     already resolves method + path + handler + file:line for every
//     fauxtown language routes.Extract supports.
//   - Client side: ExtractClientCalls in this package — Go only for VB-30
//     (vision 02 §4.7: edge kinds grow one at a time, and so does language
//     coverage for a brand-new one), parsed with go/parser and walked as
//     real ast.CallExpr nodes (never a byte-level regex over raw source —
//     a call written inside a `//` comment is plain text to the parser,
//     never an AST node, so it can never manufacture a hop) detecting
//     http.NewRequest / http.NewRequestWithContext / http.Get / http.Post /
//     http.Head / http.PostForm call sites. The method argument accepts a
//     quoted literal ("GET") or a net/http Method* constant
//     (http.MethodGet); the URL argument accepts a string literal, an
//     inline fmt.Sprintf/fmt.Sprint call, a package-level string const, a
//     path.Join/url.JoinPath call, a `*url.URL.String()` call whose
//     receiver's .Path field was built earlier in the SAME function
//     (bounded intra-function dataflow — templateFromExpr's doc comment),
//     or a local variable resolved back to its own most recent assignment
//     of one of those shapes. Any %-verb, and any value substituted via a
//     resolved-but-unknown fragment (a parameter, a field access),
//     collapses to "{}" — an expression this doesn't recognize at all
//     resolves nothing rather than guessing (precision over recall): no
//     ClientCall is produced for it.
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
	stdpath "path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
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

	consts := collectStringConsts(file)

	var ranges []funcRange
	var decls []*ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			ranges = append(ranges, funcRange{name: fd.Name.Name, start: fd.Pos(), end: fd.End()})
			decls = append(decls, fd)
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
		fn := enclosingFuncDecl(decls, call.Pos())
		rawURL, template, ok := templateFromExpr(call.Args[pat.urlArg], fn, consts, src, fset)
		if !ok {
			return true // URL expression not resolvable — skip rather than guess
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

// enclosingFuncDecl returns the *ast.FuncDecl whose range contains pos, or
// nil (a call at file scope, or inside a FuncLit closure — the bounded
// dataflow resolver below only looks within a named enclosing function,
// same non-goal as enclosingFuncName's own closure carve-out).
func enclosingFuncDecl(decls []*ast.FuncDecl, pos token.Pos) *ast.FuncDecl {
	for _, d := range decls {
		if pos >= d.Pos() && pos < d.End() {
			return d
		}
	}
	return nil
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

// maxResolveDepth bounds the client-URL resolver's recursion — a small,
// bounded intra-function dataflow step (chasing a handful of local
// assignments and nested helper calls), never a general SSA/points-to
// pass. Generous enough for any realistic call chain (Ident -> Sprintf ->
// arg -> …) while still guaranteeing termination on adversarial input.
const maxResolveDepth = 8

// templateFromExpr resolves a client call's URL argument expression e to a
// (rawText, template) pair. rawText is e's own exact original source text
// (ClientCall.RawURL, debugging/docs only, never used for extraction
// logic) — captured once, regardless of how deep the resolution below
// goes. See clientResolver.resolveTemplate for the resolvable shapes.
// Anything else returns ("", "", false) — precision over recall: no
// ClientCall is produced rather than a guessed template.
func templateFromExpr(e ast.Expr, fn *ast.FuncDecl, consts map[string]string, src []byte, fset *token.FileSet) (raw string, template string, ok bool) {
	r := &clientResolver{fn: fn, consts: consts, src: src, fset: fset}
	template, ok = r.resolveTemplate(e, 0)
	if !ok {
		return "", "", false
	}
	return sourceText(src, fset, e), template, true
}

// clientResolver resolves a client call's URL argument expression within
// ONE enclosing function — the "bounded intra-function dataflow" VB-31's
// M8 fix calls for: local assignments, fmt.Sprintf/fmt.Sprint, constant
// concatenation, url.URL{Path:…}-style field assignment, path.Join /
// url.JoinPath, and *url.URL.String() when that receiver's .Path field was
// built earlier in the SAME function. No cross-function, no branch/loop
// modeling, no go/types — just enough to see through the handful of
// idioms real Go client code writes to build a URL.
type clientResolver struct {
	fn     *ast.FuncDecl // enclosing function; nil disables Ident/`.String()` dataflow (no scope to search)
	consts map[string]string
	src    []byte
	fset   *token.FileSet
}

// resolveTemplate resolves e as the WHOLE url argument (or a variable's
// RHS while chasing an Ident): the shapes it accepts either fully resolve
// or make it fail outright (false) — never a "{}"-padded guess. Compare
// resolveFragment, used for a sub-piece already known to sit inside a
// larger recognized construct (a Join/Sprint argument, one side of a
// concatenation), which always succeeds.
func (r *clientResolver) resolveTemplate(e ast.Expr, depth int) (string, bool) {
	if depth > maxResolveDepth {
		return "", false
	}
	switch v := e.(type) {
	case *ast.ParenExpr:
		return r.resolveTemplate(v.X, depth+1)
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return s, true
			}
		}
		return "", false
	case *ast.Ident:
		if s, ok := r.consts[v.Name]; ok {
			return s, true
		}
		if r.fn == nil {
			return "", false
		}
		rhs, ok := findLastAssign(r.fn, v.Name, v.Pos())
		if !ok {
			return "", false // unresolved — no edge, rather than guess
		}
		return r.resolveTemplate(rhs, depth+1)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		// A bare top-level concatenation is a much weaker signal than a
		// recognized wrapping call (Sprintf/Sprint/Join all commit to
		// "this IS URL-building" before any fragment gets wildcarded) — so
		// require at least ONE side to resolve to genuine literal content;
		// two placeholders glued together (e.g. a test helper's
		// `ts.URL + path`, neither side statically known) is not
		// distinguishable from arbitrary, unrelated string arithmetic, and
		// precision wins the tie: no edge.
		lhs, lok := r.resolveFragmentOk(v.X, depth+1)
		rhs, rok := r.resolveFragmentOk(v.Y, depth+1)
		if !lok && !rok {
			return "", false
		}
		return lhs + rhs, true
	case *ast.CallExpr:
		return r.resolveCall(v, depth)
	}
	return "", false
}

// resolveFragment resolves e as ONE piece of an already-recognized
// URL-building construct — always succeeds: a literal/const resolves to
// its value, a real (but statically unknown) value — a parameter, a field
// access — normalizes to the placeholder "{}" (parity with a printf verb),
// and anything else falls back to resolveTemplate, itself defaulting to
// "{}" on failure.
func (r *clientResolver) resolveFragment(e ast.Expr, depth int) string {
	s, _ := r.resolveFragmentOk(e, depth)
	return s
}

// resolveFragmentOk is resolveFragment plus whether the result reflects
// genuine literal content (true) rather than a placeholder fallback
// (false) — used by resolveTemplate's bare-concatenation case to tell
// "a real base plus a substituted value" from "two unknowns glued
// together".
func (r *clientResolver) resolveFragmentOk(e ast.Expr, depth int) (string, bool) {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return r.resolveFragmentOk(v.X, depth)
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return s, true
			}
		}
		return "{}", false
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, lok := r.resolveFragmentOk(v.X, depth)
			rr, rok := r.resolveFragmentOk(v.Y, depth)
			return l + rr, lok || rok
		}
		return "{}", false
	case *ast.Ident:
		if s, ok := r.consts[v.Name]; ok {
			return s, true
		}
		return "{}", false
	case *ast.SelectorExpr:
		return "{}", false
	default:
		if depth < maxResolveDepth {
			if s, ok := r.resolveTemplate(e, depth+1); ok {
				return s, true
			}
		}
		return "{}", false
	}
}

// resolveCall resolves a CallExpr URL argument: fmt.Sprintf, fmt.Sprint,
// path.Join, url.JoinPath (package function or *url.URL method), or a
// `.String()` call.
func (r *clientResolver) resolveCall(call *ast.CallExpr, depth int) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkgIdent, isIdent := sel.X.(*ast.Ident)

	switch {
	case isIdent && pkgIdent.Name == "fmt" && sel.Sel.Name == "Sprintf":
		return r.resolveSprintf(call)
	case isIdent && pkgIdent.Name == "fmt" && sel.Sel.Name == "Sprint":
		var b strings.Builder
		for _, a := range call.Args {
			b.WriteString(r.resolveFragment(a, depth+1))
		}
		return b.String(), true
	case isIdent && pkgIdent.Name == "path" && sel.Sel.Name == "Join":
		return r.resolveJoin(call.Args, "", depth), true
	case sel.Sel.Name == "JoinPath":
		// url.JoinPath(base, elem...) (package func) or u.JoinPath(elem...)
		// (*url.URL method) — either way every argument is a path element.
		return r.resolveJoin(call.Args, "", depth), true
	case sel.Sel.Name == "String" && len(call.Args) == 0:
		return r.resolveStringer(sel.X, depth)
	}
	return "", false
}

// leadingVerbRe matches a printf verb anchored at a string's very start.
var leadingVerbRe = regexp.MustCompile(`^%[-+ 0#]*[0-9]*(?:\.[0-9]+)?[a-zA-Z]`)

// resolveSprintf resolves fmt.Sprintf(format, args...): format must be a
// plain string literal (a computed format string is out of scope — the
// verbs it contains matter, not the argument values, which NormalizeTemplate
// collapses uniformly later). One exception: a format beginning with a
// printf verb whose paired argument is itself a `.String()` call is
// idiomatic Go for building on top of a *url.URL/fmt.Stringer base (e.g.
// fmt.Sprintf("%ssystem/functions", base.URL.String())) — such a base
// conventionally supplies its own trailing "/", so that leading verb is
// replaced with a path-root marker rather than the ordinary "{}"
// placeholder (which would wrongly claim a real path segment sits at the
// template's start). Anchored at position 0 only, mirroring
// NormalizeTemplate's own scheme-stripping anchor rule — a verb appearing
// later in the format is an ordinary path-segment placeholder.
func (r *clientResolver) resolveSprintf(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	format, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	if loc := leadingVerbRe.FindStringIndex(format); loc != nil && len(call.Args) > 1 && isStringerCall(call.Args[1]) {
		format = "/" + format[loc[1]:]
	}
	return format, true
}

// isStringerCall reports whether e is a zero-argument `.String()` call —
// the shape resolveSprintf treats as evidence a leading printf verb
// interpolates a URL/host base rather than an ordinary path segment.
func isStringerCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "String"
}

// resolveJoin folds the arguments of a path.Join/url.JoinPath call into one
// joined path via the real stdlib path.Join (so "clean" duplicate-slash /
// dot-segment behavior matches exactly what the resolved code would do at
// runtime). selfRef, when non-empty, is the exact source text of a
// self-referential accumulator argument to drop rather than fold (see
// resolveStringer): `X.Path = path.Join(X.Path, "literal")` is idiomatic
// "extend the existing path", and the pre-existing base is not a
// substituted value worth wildcarding — it contributes nothing, not "{}".
func (r *clientResolver) resolveJoin(args []ast.Expr, selfRef string, depth int) string {
	var parts []string
	for _, a := range args {
		if selfRef != "" && sourceText(r.src, r.fset, a) == selfRef {
			continue
		}
		parts = append(parts, r.resolveFragment(a, depth+1))
	}
	return stdpath.Join(parts...)
}

// resolveStringer resolves a `<recv>.String()` call by finding the LAST
// assignment to `<recv>.Path` textually preceding it in the SAME function
// (findLastFieldAssign) and folding that assignment's RHS — the
// "*url.URL.String() when Path/JoinPath is visible in the function" shape.
// Falls back to a `recv := url.URL{Path: "…"}` composite-literal
// initializer when no field assignment is found. When neither is visible,
// this resolves nothing (false) rather than guessing what the receiver
// holds.
func (r *clientResolver) resolveStringer(recv ast.Expr, depth int) (string, bool) {
	if r.fn == nil {
		return "", false
	}
	recvText := sourceText(r.src, r.fset, recv)
	if recvText == "" {
		return "", false
	}
	if rhs, ok := findLastFieldAssign(r.fn, recvText, "Path", recv.Pos(), r.src, r.fset); ok {
		return r.resolvePathAssignRHS(rhs, recvText+".Path", depth+1), true
	}
	if id, ok := recv.(*ast.Ident); ok {
		if init, ok := findLastAssign(r.fn, id.Name, recv.Pos()); ok {
			if lit, ok := init.(*ast.CompositeLit); ok {
				if pathExpr, ok := compositeLitField(lit, "Path"); ok {
					return r.resolvePathAssignRHS(pathExpr, "", depth+1), true
				}
			}
		}
	}
	return "", false
}

// compositeLitField returns the value expression keyed by field in a
// keyed composite literal (e.g. `url.URL{Path: "…"}`), false if absent.
func compositeLitField(lit *ast.CompositeLit, field string) (ast.Expr, bool) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
			return kv.Value, true
		}
	}
	return nil, false
}

// resolvePathAssignRHS folds the RHS of an `X.Path = …` assignment found by
// resolveStringer, understanding path.Join/url.JoinPath specially (so the
// selfRef accumulator-drop rule applies to their arguments) and falling
// back to resolveFragment (with the same selfRef check) for everything
// else — a bare `X.Path = "literal"` or `X.Path = X.Path + "/literal"`.
func (r *clientResolver) resolvePathAssignRHS(e ast.Expr, selfRef string, depth int) string {
	if depth > maxResolveDepth {
		return "{}"
	}
	switch v := e.(type) {
	case *ast.ParenExpr:
		return r.resolvePathAssignRHS(v.X, selfRef, depth)
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return r.resolvePathAssignRHS(v.X, selfRef, depth) + r.resolvePathAssignRHS(v.Y, selfRef, depth)
		}
	case *ast.CallExpr:
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "path" && sel.Sel.Name == "Join" {
				return r.resolveJoin(v.Args, selfRef, depth)
			}
			if sel.Sel.Name == "JoinPath" {
				return r.resolveJoin(v.Args, selfRef, depth)
			}
		}
	}
	if sourceText(r.src, r.fset, e) == selfRef {
		return ""
	}
	return r.resolveFragment(e, depth)
}

// findLastAssign returns the RHS of the LAST simple (`len(Lhs)==len(Rhs)`)
// assignment to the identifier name, textually before pos, within fn's
// body — the bounded, single-hop "resolve a local variable back to its own
// definition" step. false when no such assignment exists (name is a
// parameter that's never reassigned, or genuinely unresolvable).
func findLastAssign(fn *ast.FuncDecl, name string, before token.Pos) (ast.Expr, bool) {
	if fn.Body == nil {
		return nil, false
	}
	var best ast.Expr
	bestPos := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Pos() >= before || len(as.Lhs) != len(as.Rhs) {
			return true
		}
		for i, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && id.Name == name && as.Pos() >= bestPos {
				bestPos = as.Pos()
				best = as.Rhs[i]
			}
		}
		return true
	})
	return best, best != nil
}

// findLastFieldAssign returns the RHS of the LAST assignment to
// `<recvText>.<field>` (matched by exact source text of the assignment's
// own selector receiver) textually before pos, within fn's body.
func findLastFieldAssign(fn *ast.FuncDecl, recvText, field string, before token.Pos, src []byte, fset *token.FileSet) (ast.Expr, bool) {
	if fn.Body == nil {
		return nil, false
	}
	var best ast.Expr
	bestPos := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Pos() >= before || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		sel, ok := as.Lhs[0].(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != field {
			return true
		}
		if sourceText(src, fset, sel.X) != recvText {
			return true
		}
		if as.Pos() >= bestPos {
			bestPos = as.Pos()
			best = as.Rhs[0]
		}
		return true
	})
	return best, best != nil
}

// collectStringConsts gathers every package-level `const Name = "literal"`
// declaration in file — used to fold a URL template's string concatenation
// or constant reference (e.g. a shared path-segment const). Consts defined
// via anything other than a plain string literal are not resolved.
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

// RefReader is the minimal backend capability BuildFromStore needs: a
// service's routes and client calls as its ingest extracted them (RT-89),
// and whether every readable file has been extracted.
// pkg/client.Client and store.Store satisfy it.
type RefReader interface {
	ServiceRefsFor(ctx context.Context, service string) (store.ServiceRefs, bool, error)
}

// ExtractFile is the ingest-time extraction (RT-89): path's route
// registrations and outbound HTTP client calls as store rows. A binary
// file, or a language neither extractor reads, yields none.
func ExtractFile(path string, content []byte) store.ServiceRefs {
	var out store.ServiceRefs
	if connector.IsBinary(content) {
		return out
	}
	lang := connector.LanguageForPath(path)
	if rl := routesLang(lang); rl != "" {
		for _, r := range routes.Extract(rl, path, content) {
			out.Routes = append(out.Routes, store.ServiceRoute{Method: r.Method, Path: r.Path, Handler: r.Handler, File: r.File, Line: r.Line})
		}
	}
	for _, c := range ExtractClientCalls(lang, path, content) {
		out.Calls = append(out.Calls, store.ServiceHTTPCall{Method: c.Method, Template: c.Template, RawURL: c.RawURL, Symbol: c.Symbol, File: c.File, Line: c.Line})
	}
	return out
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

// BuildFromStore joins every service's ingest-extracted route
// registrations and outbound HTTP client calls (RT-89: rows written at
// ingest, read here — never a per-call read of source) into the fleet's
// service_ref hops (ServiceRefEdges). unextracted names the services whose
// rows are not complete yet (not re-ingested since RT-89): their hops are
// missing, and callers say so. Deterministic for a fixed store state.
func BuildFromStore(ctx context.Context, backend RefReader, serviceIDs []string) (hops []Hop, unextracted []string, err error) {
	svcs := append([]string(nil), serviceIDs...)
	sort.Strings(svcs)

	routesByService := make(map[string][]routes.Route, len(svcs))
	callsByService := make(map[string][]ClientCall, len(svcs))
	for _, svc := range svcs {
		refs, complete, err := backend.ServiceRefsFor(ctx, svc)
		if err != nil {
			return nil, nil, fmt.Errorf("serviceref: refs %q: %w", svc, err)
		}
		if !complete {
			unextracted = append(unextracted, svc)
		}
		for _, r := range refs.Routes {
			routesByService[svc] = append(routesByService[svc], routes.Route{Method: r.Method, Path: r.Path, Handler: r.Handler, File: r.File, Line: r.Line})
		}
		for _, c := range refs.Calls {
			callsByService[svc] = append(callsByService[svc], ClientCall{Method: c.Method, Template: c.Template, RawURL: c.RawURL, Symbol: c.Symbol, File: c.File, Line: c.Line})
		}
	}
	return ServiceRefEdges(routesByService, callsByService), unextracted, nil
}

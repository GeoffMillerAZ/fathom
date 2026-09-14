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
//   - Client side: ExtractClientCalls in this package — regex-based, Go
//     only for VB-30 (vision 02 §4.7: edge kinds grow one at a time, and
//     so does language coverage for a brand-new one), detecting
//     http.NewRequest / http.NewRequestWithContext / http.Get / http.Post /
//     http.Head / http.PostForm call sites and resolving each one's URL
//     argument — a string literal, or the first %-verb literal of an
//     inline fmt.Sprintf(...) — to a path template.
//
// NormalizeTemplate collapses `:param`, `{param}`, a trailing `*wildcard`,
// and printf verbs (`%s`, `%d`, `%v`, …) into one canonical placeholder
// ("{}"), and strips a leading scheme+host, so a server route written
// ":id"-style and a client URL template written "%s"-style compare equal.
//
// The join (Join / ServiceRefEdges) is pure, deterministic (sorted output),
// and cross-repo only: a client call and a route in the SAME service never
// produce a hop (that is an ordinary intra-service call edge, not a
// service_ref). Every edge this package produces is tier `heuristic` —
// never typed (core.EdgeConfidenceOf's vocabulary; see
// core.CrossRepoEdgeConfidenceOf) — because a normalized-string match is
// never confirmed by a compiler-grade resolver.
package serviceref

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
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
	RawURL   string // the URL expression's literal text before normalization (debugging/docs)
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
	}
}

// sortHops sorts hops for deterministic output.
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
		case a.ToService != b.ToService:
			return a.ToService < b.ToService
		case a.ToFile != b.ToFile:
			return a.ToFile < b.ToFile
		case a.ToLine != b.ToLine:
			return a.ToLine < b.ToLine
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
// cross-repo service_ref).
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
			for toSvc, m := range routeIdx {
				if toSvc == fromSvc {
					continue // intra-service: not a cross-repo edge
				}
				for _, r := range m[k] {
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
	colonParamRe   = regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*`)
	braceParamRe   = regexp.MustCompile(`\{[^/{}]+\}`)
	trailingStarRe = regexp.MustCompile(`\*[A-Za-z_]*$`)
	// printfVerbRe matches a fmt verb: %, optional flags, optional
	// width/precision, one conversion letter (%s, %d, %v, %02d, %.2f, …).
	printfVerbRe = regexp.MustCompile(`%[-+ 0#]*[0-9]*(?:\.[0-9]+)?[a-zA-Z]`)
)

// NormalizeTemplate collapses a server route path or a client URL template
// into one canonical form so the two conventions compare equal:
//
//   - A leading "scheme://host" (if present) is stripped, leaving only the
//     path.
//   - `:param` (chi/gin/express), `{param}` (gorilla/echo, including a
//     `{param:regex}` constraint), a trailing `*` or `*param` wildcard, and
//     a printf verb (`%s`, `%d`, `%v`, `%02d`, …) are all replaced with one
//     placeholder, "{}".
//
// The result always starts with "/". Pure and deterministic.
func NormalizeTemplate(raw string) string {
	s := raw
	if idx := strings.Index(s, "://"); idx >= 0 {
		rest := s[idx+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			s = rest[slash:]
		} else {
			s = "/"
		}
	}
	s = colonParamRe.ReplaceAllString(s, "{}")
	s = braceParamRe.ReplaceAllString(s, "{}")
	s = trailingStarRe.ReplaceAllString(s, "{}")
	s = printfVerbRe.ReplaceAllString(s, "{}")
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

// ---------------------------------------------------------------------
// Client-call extraction (Go only for VB-30)
// ---------------------------------------------------------------------

// clientCallPattern names one detected stdlib net/http client call shape.
type clientCallPattern struct {
	fn          string // e.g. "http.NewRequestWithContext"
	methodArg   int    // arg index carrying the literal HTTP method; -1 = fixed method
	fixedMethod string // used when methodArg == -1
	urlArg      int    // arg index carrying the URL expression
}

var clientCallPatterns = []clientCallPattern{
	{fn: "http.NewRequestWithContext", methodArg: 1, urlArg: 2},
	{fn: "http.NewRequest", methodArg: 0, urlArg: 1},
	{fn: "http.Get", methodArg: -1, fixedMethod: "GET", urlArg: 0},
	{fn: "http.Head", methodArg: -1, fixedMethod: "HEAD", urlArg: 0},
	{fn: "http.Post", methodArg: -1, fixedMethod: "POST", urlArg: 0},
	{fn: "http.PostForm", methodArg: -1, fixedMethod: "POST", urlArg: 0},
}

// callHeaderRe finds a call site's function name and the opening '(' — the
// argument list itself is then extracted by balanced-paren scanning
// (splitArgs) since an inline fmt.Sprintf(...) argument nests parens a
// single non-nesting regex cannot safely capture.
var callHeaderRe = regexp.MustCompile(`\b(http\.NewRequestWithContext|http\.NewRequest|http\.Get|http\.Head|http\.PostForm|http\.Post)\s*\(`)

// enclosingFuncRe matches a Go top-level function or method declaration
// header — used only for the best-effort FromSymbol name (never load-
// bearing for the join itself).
var enclosingFuncRe = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// ExtractClientCalls scans src for outbound HTTP client call sites and
// returns each one's normalized (method, template), sorted. lang selects
// the pattern set; only "go" is supported today (nil for anything else —
// language coverage for a new edge kind grows one language at a time, same
// as VB-30 grows one edge kind at a time).
func ExtractClientCalls(lang, path string, src []byte) []ClientCall {
	if strings.ToLower(lang) != "go" {
		return nil
	}

	var out []ClientCall
	for _, loc := range callHeaderRe.FindAllSubmatchIndex(src, -1) {
		fnStart, fnEnd := loc[2], loc[3]
		fnName := string(src[fnStart:fnEnd])
		var pat *clientCallPattern
		for i := range clientCallPatterns {
			if clientCallPatterns[i].fn == fnName {
				pat = &clientCallPatterns[i]
				break
			}
		}
		if pat == nil {
			continue
		}
		openParen := loc[1] - 1 // position of the '(' the header regex matched
		args, _, ok := splitArgs(src, openParen)
		if !ok {
			continue
		}

		method := pat.fixedMethod
		if pat.methodArg >= 0 {
			if pat.methodArg >= len(args) {
				continue
			}
			lit, ok := stringLiteral(args[pat.methodArg])
			if !ok {
				continue // method not a literal — out of scope, skip rather than guess
			}
			method = lit
		}
		if pat.urlArg >= len(args) {
			continue
		}
		rawURL, template, ok := resolveURLTemplate(args[pat.urlArg])
		if !ok {
			continue // URL expression not resolvable to a literal/Sprintf template — skip
		}

		line := 1 + bytes.Count(src[:loc[0]], []byte("\n"))
		out = append(out, ClientCall{
			Method:   strings.ToUpper(strings.TrimSpace(method)),
			Template: NormalizeTemplate(template),
			RawURL:   rawURL,
			Symbol:   enclosingFunc(src, loc[0]),
			File:     path,
			Line:     line,
		})
	}

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

// enclosingFunc returns the name of the last `func` declaration whose
// header starts before offset — a best-effort, non-load-bearing label for
// Hop.FromSymbol. "" when none is found.
func enclosingFunc(src []byte, offset int) string {
	name := ""
	for _, loc := range enclosingFuncRe.FindAllSubmatchIndex(src, -1) {
		if loc[0] >= offset {
			break
		}
		name = string(src[loc[2]:loc[3]])
	}
	return name
}

// stringLiteral returns the content of expr when expr is exactly one
// double-quoted Go string literal (optionally surrounded by whitespace),
// else ("", false). Escape sequences are not interpreted — fixture and
// real-world method/URL literals never need one.
func stringLiteral(expr string) (string, bool) {
	e := strings.TrimSpace(expr)
	if len(e) >= 2 && e[0] == '"' && e[len(e)-1] == '"' {
		return e[1 : len(e)-1], true
	}
	return "", false
}

// resolveURLTemplate resolves a client call's URL argument expression to a
// (rawText, template) pair:
//
//   - A plain string literal is used as-is.
//   - An inline fmt.Sprintf("template", args...) call resolves to its own
//     first argument (the format-string literal) — the args themselves
//     become printf verbs NormalizeTemplate collapses to "{}".
//
// Any other expression shape (a bare identifier, a multi-line concat, a
// non-Sprintf helper) is out of scope for VB-30 and returns ("", false)
// rather than guessing.
func resolveURLTemplate(expr string) (raw string, template string, ok bool) {
	e := strings.TrimSpace(expr)
	if lit, ok := stringLiteral(e); ok {
		return lit, lit, true
	}
	if strings.HasPrefix(e, "fmt.Sprintf(") && strings.HasSuffix(e, ")") {
		inner := e[len("fmt.Sprintf(") : len(e)-1]
		innerArgs := splitTopLevelArgs(inner)
		if len(innerArgs) == 0 {
			return "", "", false
		}
		lit, ok := stringLiteral(innerArgs[0])
		if !ok {
			return "", "", false
		}
		return e, lit, true
	}
	return "", "", false
}

// splitArgs extracts the balanced-paren argument list opening at src[open]
// (which must be '(') and splits it into top-level comma-separated
// arguments (respecting nested parens/brackets and quoted strings). Returns
// the arguments, the offset just past the matching ')', and whether a
// matching close was found.
func splitArgs(src []byte, open int) ([]string, int, bool) {
	if open >= len(src) || src[open] != '(' {
		return nil, 0, false
	}
	depth := 0
	inString := false
	var quote byte
	start := open + 1
	for i := open; i < len(src); i++ {
		c := src[i]
		if inString {
			if c == '\\' {
				i++ // skip escaped char
				continue
			}
			if c == quote {
				inString = false
			}
			continue
		}
		switch c {
		case '"', '`':
			inString = true
			quote = c
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return splitTopLevelArgs(string(src[start:i])), i + 1, true
			}
		}
	}
	return nil, 0, false
}

// splitTopLevelArgs splits a raw argument-list string on top-level commas
// (respecting nested parens/brackets/braces and quoted strings) — the same
// balancing splitArgs applies, reusable for an already-isolated inner
// argument list (e.g. an inline fmt.Sprintf(...)'s own arguments).
func splitTopLevelArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	depth := 0
	inString := false
	var quote byte
	last := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				inString = false
			}
			continue
		}
		switch c {
		case '"', '`':
			inString = true
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[last:i])
				last = i + 1
			}
		}
	}
	out = append(out, s[last:])
	return out
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

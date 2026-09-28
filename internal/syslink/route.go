package syslink

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
)

// routesLang maps an ADR-048 dialect id to routes.Extract's language
// argument ("" when routes.Extract does not read the dialect).
func routesLang(dialect string) string {
	switch dialect {
	case "go":
		return "go"
	case "javascript":
		return "js"
	case "typescript", "tsx":
		return "ts"
	case "python":
		return "python"
	}
	return ""
}

// routeProvides turns routes.Extract's registrations into route: provides
// rows (SG-4). The node is the route, named by its handler (or by its own
// "METHOD /path" when the handler is anonymous). The tier is `literal`: a
// registration read from code, never bound through a contract document.
// A registration whose method or path the key grammar refuses is dropped,
// not guessed at.
func routeProvides(dialect, file string, content []byte) []KeyRow {
	lang := routesLang(dialect)
	if lang == "" {
		return nil
	}
	var out []KeyRow
	for _, r := range routes.Extract(lang, file, content) {
		key, err := ParseContractKey("route:" + r.Method + " " + r.Path)
		if err != nil {
			continue
		}
		node := r.Handler
		if node == "" || strings.HasPrefix(node, "func") || strings.HasPrefix(node, "<") {
			node = key.Body
		}
		out = append(out, KeyRow{
			Key: key, Role: RoleProvides, NodeKind: "route", NodeID: node,
			File: file, Line: r.Line, TierHint: TierLiteral,
		})
	}
	SortRows(out)
	return out
}

// goClientConsumes turns serviceref.ExtractClientCalls's Go net/http call
// sites into route: consumes rows (SG-5), at the tier consumerTier names.
func goClientConsumes(file string, content []byte) []KeyRow {
	var out []KeyRow
	for _, c := range serviceref.ExtractClientCalls("go", file, content) {
		if row, ok := clientRow(file, c.Line, c.Method, c.Template, c.RawURL, c.Symbol); ok {
			out = append(out, row)
		}
	}
	SortRows(out)
	return out
}

// clientRow builds one route: consumes row from a detected client call.
func clientRow(file string, line int, method, template, raw, symbol string) (KeyRow, bool) {
	path, tier := consumerTier(template, raw)
	key, err := ParseContractKey("route:" + method + " " + path)
	if err != nil {
		return KeyRow{}, false
	}
	return KeyRow{
		Key: key, Role: RoleConsumes, NodeKind: "call", NodeID: symbol,
		File: file, Line: line, TierHint: tier,
		Attrs: map[string]string{"raw_url": raw},
	}, true
}

// consumerTier classifies a client call's normalized path template (§4):
//
//   - A leading hole that stands for a base URL (the URL expression starts
//     with a variable, or a format string starts with a verb) is explained:
//     it is the host, which normalization strips from a literal URL too, so
//     it is dropped from the key.
//   - What remains is `literal` when every hole is a whole path segment
//     after at least one literal segment (an id after "/orders/").
//   - It is `template` when a hole stands where a resource name must be
//     (the first segment) or fills only part of a segment: the key then has
//     an unexplained hole, and it matches providers only as a pattern.
func consumerTier(template, raw string) (string, Tier) {
	path := template
	if strings.HasPrefix(path, "/{}") && startsWithBase(raw) {
		path = strings.TrimPrefix(path, "/{}")
		if path == "" {
			path = "/"
		}
	}
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, s := range segs {
		if !strings.Contains(s, "{}") {
			continue
		}
		if i == 0 || s != "{}" {
			return path, TierTemplate
		}
	}
	return path, TierLiteral
}

// startsWithBase reports whether a URL expression's source text begins
// with something other than a literal path: a variable or call (a base
// URL), or a format string whose first byte is a verb. Known wrappers
// (fmt.Sprintf, path.Join, url.JoinPath, template literals) are looked
// through to their first argument.
func startsWithBase(raw string) bool {
	s := strings.TrimSpace(raw)
	for _, w := range []string{"fmt.Sprintf(", "fmt.Sprint(", "path.Join(", "url.JoinPath(", "f\"", "f'"} {
		if strings.HasPrefix(s, w) {
			s = strings.TrimSpace(s[len(w)-1:])
			if strings.HasSuffix(w, "(") {
				s = strings.TrimSpace(s[1:])
			}
			break
		}
	}
	if s == "" {
		return false
	}
	switch s[0] {
	case '"', '\'', '`':
		rest := s[1:]
		return strings.HasPrefix(rest, "%") || strings.HasPrefix(rest, "${") || strings.HasPrefix(rest, "{")
	}
	return true
}

type routeSource struct{}

func (routeSource) ID() string { return "routes" }
func (routeSource) Epoch() int { return 1 }
func (routeSource) Keys(file string, content []byte, dialect string) []KeyRow {
	return routeProvides(dialect, file, content)
}

type goClientSource struct{}

func (goClientSource) ID() string { return "go-http-client" }
func (goClientSource) Epoch() int { return 1 }
func (goClientSource) Keys(file string, content []byte, _ string) []KeyRow {
	return goClientConsumes(file, content)
}

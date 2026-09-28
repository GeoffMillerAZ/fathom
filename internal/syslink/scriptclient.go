package syslink

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
)

// The JS/TS and Python HTTP client consumers are dialect data (§5: "a
// framework extractor is a query or selector file where possible"): the
// call shapes live in selectors/http_clients.json; this file holds only
// the small comment- and string-aware scanner that applies them.

//go:embed selectors/http_clients.json
var httpClientSelectorsJSON []byte

type clientCallSel struct {
	Callee        string `json:"callee"`
	URLArg        int    `json:"url_arg"`
	Method        string `json:"method"`
	DefaultMethod string `json:"default_method"`
	OptionsArg    *int   `json:"options_arg"`
}

type clientReceiverSel struct {
	Constructor string            `json:"constructor"`
	Methods     map[string]string `json:"methods"`
}

type clientDialectSel struct {
	Calls     []clientCallSel     `json:"calls"`
	Receivers []clientReceiverSel `json:"receivers"`
}

// httpClientSelectors is parsed once from the embedded file; it is
// read-only after init.
var httpClientSelectors = func() map[string]clientDialectSel {
	var f struct {
		Dialects map[string]clientDialectSel `json:"dialects"`
	}
	if err := json.Unmarshal(httpClientSelectorsJSON, &f); err != nil {
		panic("syslink: selectors/http_clients.json: " + err.Error())
	}
	return f.Dialects
}()

// selectorDialect maps an ADR-048 dialect id to its selector table and
// its comment syntax.
func selectorDialect(dialect string) (table string, python bool) {
	switch dialect {
	case "javascript", "typescript", "tsx":
		return "javascript", false
	case "python":
		return "python", true
	}
	return "", false
}

// scriptClientConsumes reads JS/TS or Python source for the HTTP client
// calls the selector file names (fetch, axios, ky; requests, httpx,
// aiohttp) and returns route: consumes rows at literal or template tier
// (consumerTier). A call inside a comment or a string is never a call; a
// URL argument with no literal part (a bare variable) resolves nothing
// rather than guessing.
func scriptClientConsumes(dialect, file string, content []byte) []KeyRow {
	table, python := selectorDialect(dialect)
	sel, ok := httpClientSelectors[table]
	if !ok {
		return nil
	}
	src := string(content)
	mask := maskSource(src, python)
	consts := scriptStringConsts(src, mask, python)

	type shape struct {
		method, defMethod string
		urlArg            int
		optionsArg        *int
	}
	shapes := map[string]shape{}
	for _, c := range sel.Calls {
		shapes[c.Callee] = shape{c.Method, c.DefaultMethod, c.URLArg, c.OptionsArg}
	}
	// Receivers: a variable bound to a client constructor makes its
	// methods calls (`x = axios.create(...)`, `with httpx.Client() as x`).
	for _, r := range sel.Receivers {
		for _, v := range boundReceivers(mask, r.Constructor) {
			for m, method := range r.Methods {
				shapes[v+"."+m] = shape{method: method}
			}
		}
	}
	callees := make([]string, 0, len(shapes))
	for c := range shapes {
		callees = append(callees, regexp.QuoteMeta(c))
	}
	sort.Strings(callees)
	if len(callees) == 0 {
		return nil
	}
	callRe := regexp.MustCompile(`(?:^|[^\w.$])(` + strings.Join(callees, "|") + `)\s*\(`)

	var out []KeyRow
	for _, m := range callRe.FindAllStringSubmatchIndex(mask, -1) {
		callee := mask[m[2]:m[3]]
		sh := shapes[callee]
		args := splitArgs(src, mask, m[1])
		if sh.urlArg >= len(args) {
			continue
		}
		raw := strings.TrimSpace(args[sh.urlArg])
		tmpl, ok := scriptURLTemplate(raw, consts)
		if !ok {
			continue
		}
		method := sh.method
		if method == "" {
			method = sh.defMethod
			if sh.optionsArg != nil && *sh.optionsArg < len(args) {
				if mm := optMethodRe.FindStringSubmatch(args[*sh.optionsArg]); mm != nil {
					method = strings.ToUpper(mm[1])
				}
			}
		}
		line := 1 + strings.Count(src[:m[2]], "\n")
		if row, ok := clientRow(file, line, method, serviceref.NormalizeTemplate(tmpl), raw, enclosingScriptFunc(mask[:m[2]], python)); ok {
			out = append(out, row)
		}
	}
	SortRows(out)
	return out
}

var optMethodRe = regexp.MustCompile(`method\s*:\s*["'` + "`" + `]([A-Za-z]+)["'` + "`" + `]`)

// maskSource returns src with every comment and every string literal's
// content blanked to spaces (delimiters and newlines kept), so a regex over
// the mask finds only code, at the same byte offsets as src.
func maskSource(src string, python bool) string {
	b := []byte(src)
	n := len(b)
	blank := func(i, j int) {
		for k := i; k < j && k < n; k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < n; {
		c := b[i]
		switch {
		case python && c == '#':
			j := i
			for j < n && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case !python && c == '/' && i+1 < n && b[i+1] == '/':
			j := i
			for j < n && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case !python && c == '/' && i+1 < n && b[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			end := n
			if j >= 0 {
				end = i + 2 + j + 2
			}
			blank(i, end)
			i = end
		case c == '"' || c == '\'' || (!python && c == '`'):
			delim := string(c)
			if python && i+2 < n && b[i+1] == c && b[i+2] == c {
				delim = strings.Repeat(string(c), 3)
			}
			j := i + len(delim)
			for j < n {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if strings.HasPrefix(src[j:], delim) {
					break
				}
				if len(delim) == 1 && c != '`' && src[j] == '\n' {
					break
				}
				j++
			}
			blank(i+len(delim), j)
			i = j + len(delim)
		default:
			i++
		}
	}
	return string(b)
}

// splitArgs returns the top-level argument texts (from src) of the call
// whose '(' ends just before open; nesting and strings are read on mask.
func splitArgs(src, mask string, open int) []string {
	var args []string
	depth, start := 0, open
	for i := open; i < len(mask); i++ {
		switch mask[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				if s := strings.TrimSpace(src[start:i]); s != "" || len(args) > 0 {
					args = append(args, src[start:i])
				}
				return args
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, src[start:i])
				start = i + 1
			}
		}
	}
	return nil // unterminated call: nothing, not a guess
}

var (
	jsInterpRe = regexp.MustCompile(`\$\{([^}]*)\}`)
	pyInterpRe = regexp.MustCompile(`\{([^}]*)\}`)
	identOnly  = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
)

// scriptURLTemplate renders a URL argument's source text as a path
// template: string literals verbatim, `+`-joined identifiers by their
// module-level string constant or as a hole, interpolations likewise. An
// argument with no string literal and no known constant resolves nothing.
func scriptURLTemplate(raw string, consts map[string]string) (string, bool) {
	parts := splitTopLevel(raw, '+')
	var sb strings.Builder
	anyLiteral := false
	for _, p := range parts {
		p = strings.TrimSpace(p)
		lit, interp, ok := stringLiteral(p)
		switch {
		case ok:
			anyLiteral = true
			if interp != nil {
				lit = interp.ReplaceAllStringFunc(lit, func(m string) string {
					name := strings.TrimSpace(interp.FindStringSubmatch(m)[1])
					if v, ok := consts[name]; ok {
						return v
					}
					return "{}"
				})
			}
			sb.WriteString(lit)
		case identOnly.MatchString(p):
			if v, ok := consts[p]; ok {
				anyLiteral = true
				sb.WriteString(v)
			} else {
				sb.WriteString("{}")
			}
		default:
			sb.WriteString("{}")
		}
	}
	return sb.String(), anyLiteral
}

// stringLiteral returns a literal's content and, for a template literal or
// f-string, the interpolation pattern to resolve in it.
func stringLiteral(p string) (string, *regexp.Regexp, bool) {
	var interp *regexp.Regexp
	switch {
	case strings.HasPrefix(p, "f\"") || strings.HasPrefix(p, "f'"):
		p, interp = p[1:], pyInterpRe
	case strings.HasPrefix(p, "`"):
		interp = jsInterpRe
	}
	if len(p) < 2 {
		return "", nil, false
	}
	q := p[0]
	if (q != '"' && q != '\'' && q != '`') || p[len(p)-1] != q {
		return "", nil, false
	}
	return p[1 : len(p)-1], interp, true
}

// splitTopLevel splits s on sep outside strings and brackets.
func splitTopLevel(s string, sep byte) []string {
	mask := maskSource(s, false)
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(mask); i++ {
		switch mask[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

var (
	jsConstRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(["'])`)
	pyConstRe = regexp.MustCompile(`(?m)^([A-Za-z_]\w*)\s*=\s*(["'])`)
)

// scriptStringConsts collects module-level `NAME = "literal"` bindings (a
// single plain string on the right-hand side, nothing after it).
func scriptStringConsts(src, mask string, python bool) map[string]string {
	re := jsConstRe
	if python {
		re = pyConstRe
	}
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatchIndex(mask, -1) {
		name := mask[m[2]:m[3]]
		q := src[m[4]]
		rest := src[m[4]+1:]
		end := strings.IndexByte(rest, q)
		if end < 0 {
			continue
		}
		tail := strings.TrimSpace(strings.SplitN(rest[end+1:], "\n", 2)[0])
		if tail != "" && tail != ";" {
			continue
		}
		out[name] = rest[:end]
	}
	return out
}

// boundReceivers returns the variable names bound to a constructor call:
// `x = ctor(`, `const x = ctor(` and Python's `with ctor(...) as x`.
func boundReceivers(mask, ctor string) []string {
	q := regexp.QuoteMeta(ctor)
	assign := regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*=\s*(?:await\s+)?` + q + `\s*\(`)
	with := regexp.MustCompile(q + `\s*\([^)]*\)\s+as\s+([A-Za-z_]\w*)`)
	seen := map[string]bool{}
	for _, re := range []*regexp.Regexp{assign, with} {
		for _, m := range re.FindAllStringSubmatch(mask, -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

var (
	jsFuncRe = regexp.MustCompile(`(?:function\s*\*?\s*([A-Za-z_$][\w$]*)|([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>)`)
	pyFuncRe = regexp.MustCompile(`def\s+([A-Za-z_]\w*)`)
)

// enclosingScriptFunc names the last function declared before a call —
// a best-effort node label, never load-bearing for a key.
func enclosingScriptFunc(before string, python bool) string {
	re := jsFuncRe
	if python {
		re = pyFuncRe
	}
	all := re.FindAllStringSubmatch(before, -1)
	if len(all) == 0 {
		return ""
	}
	last := all[len(all)-1]
	for _, g := range last[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

type scriptClientSource struct{}

func (scriptClientSource) ID() string { return "script-http-client" }
func (scriptClientSource) Epoch() int { return 1 }
func (scriptClientSource) Keys(file string, content []byte, dialect string) []KeyRow {
	return scriptClientConsumes(dialect, file, content)
}

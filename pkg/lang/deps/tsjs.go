package deps

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// The ordinary import/export captures also used by the language reader are
// recovered through ExtractFileSpecs. The supplementary tree query supplies
// the physical operand positions, grammar faults and constructs that do not
// create file-resolver bindings.
const tsjsQuery = `
[(ERROR) (MISSING)] @error
(import_statement source: (string) @operand) @static
(export_statement source: (string) @operand) @static
(call_expression function: (import) @dynamic arguments: (arguments) @args)
(call_expression function: (import) @dynamic arguments: (arguments . (_) @operand))
((call_expression function: (identifier) @direct arguments: (arguments) @args)
 (#eq? @direct "require"))
((call_expression function: (identifier) @direct arguments: (arguments . (_) @operand))
 (#eq? @direct "require"))
((call_expression function: (member_expression object: (identifier) @loader property: (property_identifier) @method) @member arguments: (arguments) @args)
 (#eq? @loader "require") (#eq? @method "resolve"))
((call_expression function: (member_expression object: (identifier) @loader property: (property_identifier) @method) @member arguments: (arguments . (_) @operand))
 (#eq? @loader "require") (#eq? @method "resolve"))
((identifier) @loader-use
 (#match? @loader-use "^(require|createRequire|requirejs)$"))
((call_expression function: (identifier) @other-loader)
 (#match? @other-loader "^(eval|Function|define)$"))
((new_expression constructor: (identifier) @other-loader)
 (#eq? @other-loader "Function"))
((member_expression object: (identifier) @module property: (property_identifier) @method) @other-loader
 (#eq? @module "module") (#eq? @method "require"))
(meta_property) @meta
((member_expression object: (meta_property) property: (property_identifier) @method) @other-loader
 (#match? @method "^(resolve|glob|globEager)$"))
(subscript_expression object: (meta_property)) @other-loader
((new_expression constructor: (identifier) @url arguments: (arguments (member_expression object: (meta_property)))) @other-loader
 (#eq? @url "URL"))
((shorthand_property_identifier_pattern) @loader-use
 (#eq? @loader-use "require"))
(comment) @comment
`

const tsjsJSXQuery = `
[(jsx_element) (jsx_self_closing_element)] @jsx
`

const tsjsTSQuery = `
(import_require_clause source: (string) @operand) @equals
(ambient_declaration (module name: (string) @operand)) @ambient
`

type tsjsRequest struct {
	raw  string
	line int
	mode tsjsMode
}

type tsjsSite struct {
	start, end uint32
}

type tsjsSyntax struct {
	requests []tsjsRequest
	esm      bool
	jsx      bool
}

func inferTSJSDialect(ext string) string {
	switch ext {
	case ".ts", ".tsx", ".mts", ".cts":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	}
	return ""
}

func readTSJSDependencies(ctx context.Context, repo repositoryFS, src []byte, override Resolver, out *result) {
	g := tsjsRead{ctx: ctx, repo: repo, out: out}
	syntax, ok := g.syntax(src)
	if !ok {
		return
	}
	g.loadConfig()
	g.sourceManifest = g.nearestManifest()
	if syntax.jsx && g.config.automaticJSX {
		g.unknown(UnsupportedConstruct, "", 1, Toolchain, ConfigFile)
	}
	// Sanitized relative operands on one physical line cannot be uniquely
	// attributed by the wire format when a second request shares the line.
	byLine := make(map[int]tsjsRequest)
	for _, req := range syntax.requests {
		prev, ok := byLine[req.line]
		if ok && (prev.raw != req.raw || prev.mode != req.mode) &&
			(safeRaw(prev.raw) == "***" || safeRaw(req.raw) == "***") {
			g.unknown(UnsupportedConstruct, "", req.line, Declaration, Toolchain)
		}
		if !ok || safeRaw(req.raw) == "***" {
			byLine[req.line] = req
		}
	}
	for _, req := range syntax.requests {
		if ctx.Err() != nil {
			return
		}
		if override != nil {
			out.resolveExtension(repo, override, req.raw, req.line)
		} else {
			g.resolve(req, syntax.esm)
		}
	}
}

func (g *tsjsRead) syntax(src []byte) (tsjsSyntax, bool) {
	if !utf8.Valid(src) || bytes.IndexByte(src, 0) >= 0 {
		g.unknown(ParseError, "", 1, Toolchain)
		return tsjsSyntax{}, false
	}
	grammar := g.out.Dialect
	entry := "tree_sitter_" + grammar
	query := tsjsQuery
	if grammar == "typescript" {
		query += tsjsTSQuery
		if strings.HasSuffix(g.out.Path, ".tsx") {
			entry = "tree_sitter_tsx"
		}
	}
	if entry != "tree_sitter_typescript" {
		query += tsjsJSXQuery
	}
	l, err := tsengine.AcquireParser(g.ctx, "deps:"+entry, grammar, entry)
	if err != nil {
		g.unknown(ReadLimit, "", 1, Toolchain)
		return tsjsSyntax{}, false
	}
	defer l.Release()
	parser := l.Parser()
	tree, err := parser.Parse(src)
	if err != nil {
		l.Discard()
		g.unknown(ReadLimit, "", 1, Toolchain)
		return tsjsSyntax{}, false
	}
	q, ok := parser.Cached("dependency-captures").(*tsengine.Query)
	if !ok {
		q, err = parser.CompileQuery(query)
		if err != nil {
			tree.Close()
			l.Discard()
			g.unknown(UnsupportedConstruct, "", 1, Toolchain)
			return tsjsSyntax{}, false
		}
		parser.SetCached("dependency-captures", q)
	}
	ms, err := q.Run(tree)
	tree.Close()
	if err != nil {
		l.Discard()
		g.unknown(ReadLimit, "", 1, Toolchain)
		return tsjsSyntax{}, false
	}
	l.Release()
	return g.requests(src, ms), true
}

type tsjsCall struct {
	args, operand tsengine.Node
	mode          tsjsMode
	line          int
}

func (g *tsjsRead) requests(src []byte, matches []tsengine.Match) tsjsSyntax {
	var syntax tsjsSyntax
	loaders := make(map[uint32]tsengine.Node)
	used := make(map[uint32]bool)
	calls := make(map[tsjsSite]tsjsCall)
	callOrder := []tsjsSite{}
	var existing map[tsjsRequest]string
	for _, match := range matches {
		var kind string
		var node, args, operand, loader tsengine.Node
		for _, c := range match.Captures {
			switch c.Name {
			case "args":
				args = c.Node
			case "operand":
				operand = c.Node
			case "loader":
				loader = c.Node
			case "method", "module", "url":
			default:
				kind, node = c.Name, c.Node
			}
		}
		switch kind {
		case "static", "equals":
			mode := tsjsStatic
			syntax.esm = true
			if kind == "equals" {
				mode = tsjsRequire
			}
			if existing == nil {
				existing = g.fileSpecs(src)
			}
			g.addRequest(&syntax, src, operand, node.Line(), mode, existing)
			if kind == "static" && operand.EndByte < node.EndByte {
				tail := strings.TrimSpace(string(src[operand.EndByte:node.EndByte]))
				if tail != "" && tail != ";" {
					g.unknown(UnsupportedConstruct, "", node.Line(), Toolchain, ConfigFile)
				}
			}
		case "dynamic", "direct", "member":
			mode := tsjsImport
			if kind != "dynamic" {
				mode = tsjsRequire
				if kind == "direct" {
					loader = node
				}
				used[loader.StartByte] = true
			}
			key := tsjsSite{node.StartByte, node.EndByte}
			call, ok := calls[key]
			if !ok {
				callOrder = append(callOrder, key)
			}
			call.mode, call.line = mode, node.Line()
			if args.EndByte != 0 {
				call.args = args
			}
			if operand.EndByte != 0 && (call.operand.EndByte == 0 || operand.StartByte < call.operand.StartByte) {
				call.operand = operand
			}
			calls[key] = call
		case "loader-use":
			loaders[node.StartByte] = node
		case "error":
			g.unknown(ParseError, "", node.Line(), Toolchain)
		case "ambient", "other-loader":
			g.unknown(UnsupportedConstruct, "", node.Line(), Toolchain, ConfigFile)
		case "meta":
			syntax.esm = true
		case "jsx":
			syntax.jsx = true
		case "comment":
			g.comment(node.Text(src), node.Line())
		}
	}
	for _, key := range callOrder {
		call := calls[key]
		g.addRequest(&syntax, src, call.operand, call.line, call.mode, nil)
		if call.operand.EndByte != 0 && call.args.EndByte > call.operand.EndByte &&
			!tsjsCallTail(src[call.operand.EndByte:call.args.EndByte]) {
			g.unknown(UnsupportedConstruct, "", call.line, Toolchain, Observation)
		}
	}
	for offset, node := range loaders {
		if !used[offset] {
			g.unknown(UnsupportedConstruct, "", node.Line(), Toolchain, Observation)
		}
	}
	return syntax
}

// TS syntax often puts the operand on a later line than its declaration.
// Existing binding evidence names the declaration, while dependencies use
// the actual operand line so repeated imports remain attributable.
var tsjsTypeScriptSpecs = sync.OnceValues(func() (*extractor.TSExtractor, error) {
	return extractor.NewTSExtractor("typescript")
})

var tsjsJavaScriptSpecs = sync.OnceValues(func() (*extractor.TSExtractor, error) {
	return extractor.NewTSExtractor("javascript")
})

func (g *tsjsRead) fileSpecs(src []byte) map[tsjsRequest]string {
	spellings := make(map[tsjsRequest]string)
	factory := tsjsJavaScriptSpecs
	if g.out.Dialect == "typescript" {
		factory = tsjsTypeScriptSpecs
	}
	e, err := factory()
	if err != nil {
		return spellings
	}
	specs, err := e.ExtractFileSpecs(g.out.Path, src)
	if err != nil {
		return spellings
	}
	for _, item := range specs.Imports {
		key := tsjsRequest{raw: item.Module, line: item.Line, mode: tsjsStatic}
		spellings[key] = item.Module
	}
	for _, item := range specs.Exports {
		if item.Module != "" {
			key := tsjsRequest{raw: item.Module, line: item.Line, mode: tsjsStatic}
			spellings[key] = item.Module
		}
	}
	return spellings
}

func (g *tsjsRead) addRequest(s *tsjsSyntax, src []byte, operand tsengine.Node, stmtLine int, mode tsjsMode, existing map[tsjsRequest]string) {
	token := operand.Text(src)
	raw, literal := tsjsLiteral(token)
	if !literal {
		code := DynamicImport
		if mode == tsjsStatic || existing != nil {
			code = UnsupportedConstruct
		}
		g.unknown(code, "***", stmtLine, Toolchain, Observation)
		return
	}
	if spelling := existing[tsjsRequest{raw: raw, line: stmtLine, mode: tsjsStatic}]; spelling != "" {
		raw = spelling
	}
	s.requests = append(s.requests, tsjsRequest{raw, operand.Line(), mode})
}

func (g *tsjsRead) comment(comment string, line int) {
	if strings.HasPrefix(comment, "///") && strings.Contains(comment, "<reference") ||
		strings.HasPrefix(comment, "/**") && (strings.Contains(comment, "@import") ||
			strings.Contains(comment, "import(") || strings.Contains(comment, "import (")) {
		g.unknown(UnsupportedConstruct, "", line, Toolchain, ConfigFile)
	}
}

func tsjsCallTail(tail []byte) bool {
	return bytes.Equal(bytes.TrimSpace(tail), []byte(")")) ||
		bytes.Equal(bytes.TrimSpace(tail), []byte(",)"))
}

// Only a single string or static template is a literal. Decode the JS
// escapes that can occur in portable paths. Non-portable decoded operands
// are screened again by the resolver and output sanitation.
func tsjsLiteral(token string) (string, bool) {
	if len(token) < 2 || token[len(token)-1] != token[0] || !strings.ContainsRune("'\"`", rune(token[0])) {
		return "", false
	}
	quote := token[0]
	token = token[1 : len(token)-1]
	var out strings.Builder
	for len(token) > 0 {
		c := token[0]
		token = token[1:]
		if c == quote || quote == '`' && c == '$' && strings.HasPrefix(token, "{") {
			return "", false
		}
		if c != '\\' {
			out.WriteByte(c)
			continue
		}
		if len(token) == 0 {
			return "", false
		}
		escape := token[0]
		token = token[1:]
		switch escape {
		case '\\', '\'', '"', '`', '/', '$':
			out.WriteByte(escape)
		case '\n':
		case '\r':
			token = strings.TrimPrefix(token, "\n")
		case 'x', 'u':
			size := 2
			if escape == 'u' {
				size = 4
			}
			digits := ""
			if escape == 'u' && strings.HasPrefix(token, "{") {
				end := strings.IndexByte(token, '}')
				if end <= 1 || end > 7 {
					return "", false
				}
				digits, token = token[1:end], token[end+1:]
			} else {
				if len(token) < size {
					return "", false
				}
				digits, token = token[:size], token[size:]
			}
			value, err := strconv.ParseUint(digits, 16, 32)
			if err != nil || !utf8.ValidRune(rune(value)) {
				return "", false
			}
			out.WriteRune(rune(value))
		default:
			return "", false
		}
	}
	return out.String(), true
}

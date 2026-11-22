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
(
 [(variable_declarator name: (identifier) @shadow)
  (assignment_expression left: (identifier) @shadow)
  (function_declaration name: (identifier) @shadow)
  (class_declaration name: (_) @shadow)
  (import_clause (identifier) @shadow)
  (namespace_import (identifier) @shadow)
  (import_specifier alias: (identifier) @shadow)
  (arrow_function parameter: (identifier) @shadow)
  (catch_clause parameter: (identifier) @shadow)]
 (#eq? @shadow "require"))
((identifier) @escaped-loader
 (#match? @escaped-loader "\\\\u"))
((identifier) @loader-use
 (#match? @loader-use "^(require|createRequire|requirejs)$"))
((call_expression function: (identifier) @other-loader)
 (#match? @other-loader "^(eval|Function|define)$"))
((new_expression constructor: (identifier) @other-loader)
 (#eq? @other-loader "Function"))
((member_expression object: (identifier) @module property: (property_identifier) @method) @other-loader
 (#match? @module "^(module|global|globalThis)$") (#eq? @method "require"))
((subscript_expression object: (identifier) @module index: (_) @index) @module-subscript
 (#match? @module "^(module|global|globalThis)$"))
((member_expression object: (identifier) @module property: (property_identifier) @method) @other-loader
 (#eq? @module "System") (#eq? @method "import"))
(meta_property) @meta
((member_expression object: (meta_property) property: (property_identifier) @method) @other-loader
 (#match? @method "^(resolve|glob|globEager)$"))
(subscript_expression object: (meta_property)) @other-loader
((new_expression constructor: (identifier) @url arguments: (arguments (member_expression object: (meta_property)))) @other-loader
 (#eq? @url "URL"))
((shorthand_property_identifier_pattern) @shadow
 (#eq? @shadow "require"))
(comment) @comment
`

const tsjsJSXQuery = `
[(jsx_element) (jsx_self_closing_element)] @jsx
`

const tsjsJavaScriptQuery = `
((formal_parameters (identifier) @shadow)
 (#eq? @shadow "require"))
(_ (comment) @doc . (comment) @doc-next)
(_ (comment) @doc . (parenthesized_expression) @doc-cast)
(_ (comment) @doc .
 [(lexical_declaration) (variable_declaration) (function_declaration)
  (generator_function_declaration) (class_declaration) (export_statement)
  (expression_statement) (return_statement) (method_definition)
  (field_definition) (pair)] @doc-site)
`

const tsjsTSQuery = `
([(required_parameter pattern: (identifier) @shadow)
  (optional_parameter pattern: (identifier) @shadow)]
 (#eq? @shadow "require"))
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
		g.unknown(UnknownUnsupportedConstruct, "", 0, DecideToolchain, DecideConfigFile)
	}
	// Sanitized relative operands on one physical line cannot be uniquely
	// attributed by the wire format when a second request shares the line.
	byLine := make(map[int]tsjsRequest)
	for _, req := range syntax.requests {
		prev, ok := byLine[req.line]
		if ok && (prev.raw != req.raw || prev.mode != req.mode) &&
			(safeRaw(prev.raw) == "***" || safeRaw(req.raw) == "***") {
			g.unknown(UnknownUnsupportedConstruct, "", req.line, DecideDeclaration, DecideToolchain)
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
			out.resolveExtension(ctx, override, ResolveRequest{Repo: repo, From: out.Path, Raw: req.raw, Kind: KindImport, Line: req.line})
		} else {
			g.resolve(req, syntax.esm)
		}
	}
}

func (g *tsjsRead) syntax(src []byte) (tsjsSyntax, bool) {
	if !utf8.Valid(src) || bytes.IndexByte(src, 0) >= 0 {
		g.unknown(UnknownParseError, "", 0, DecideToolchain)
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
	if grammar == "javascript" {
		query += tsjsJavaScriptQuery
	}
	if entry != "tree_sitter_typescript" {
		query += tsjsJSXQuery
	}
	l, err := tsengine.AcquireParser(g.ctx, "deps:"+entry, grammar, entry)
	if err != nil {
		g.unknown(UnknownReadLimit, "", 0, DecideToolchain)
		return tsjsSyntax{}, false
	}
	defer l.Release()
	parser := l.Parser()
	tree, err := parser.Parse(src)
	if err != nil {
		l.Discard()
		g.unknown(UnknownReadLimit, "", 0, DecideToolchain)
		return tsjsSyntax{}, false
	}
	q, ok := parser.Cached("dependency-captures").(*tsengine.Query)
	if !ok {
		q, err = parser.CompileQuery(query)
		if err != nil {
			tree.Close()
			l.Discard()
			g.unknown(UnknownUnsupportedConstruct, "", 0, DecideToolchain)
			return tsjsSyntax{}, false
		}
		parser.SetCached("dependency-captures", q)
	}
	ms, err := q.Run(tree)
	tree.Close()
	if err != nil {
		l.Discard()
		g.unknown(UnknownReadLimit, "", 0, DecideToolchain)
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
	var comments []tsengine.Node
	followers := make(map[uint32]tsjsDocFollower)
	shadowed := false
	for _, match := range matches {
		var kind string
		var node, args, operand, loader, index, doc tsengine.Node
		for _, c := range match.Captures {
			switch c.Name {
			case "args":
				args = c.Node
			case "operand":
				operand = c.Node
			case "loader":
				loader = c.Node
			case "index":
				index = c.Node
			case "doc":
				doc = c.Node
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
					g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideConfigFile)
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
		case "shadow":
			shadowed = true
			g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideObservation)
		case "escaped-loader":
			name, _ := tsjsLiteral("'" + node.Text(src) + "'")
			switch name {
			case "require", "createRequire", "requirejs", "eval", "Function", "define":
				g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideObservation)
			}
		case "error":
			g.unknown(UnknownParseError, "", node.Line(), DecideToolchain)
		case "module-subscript":
			name, literal := tsjsLiteral(index.Text(src))
			if !literal || name == "require" {
				g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideObservation)
			}
		case "ambient", "other-loader":
			g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideConfigFile)
		case "meta":
			syntax.esm = true
		case "jsx":
			syntax.jsx = true
		case "comment":
			comment := node.Text(src)
			if g.out.Dialect == "javascript" {
				comments = append(comments, node)
				if strings.HasPrefix(comment, "/**") {
					continue
				}
			}
			g.comment(comment, node.Line())
		case "doc-next", "doc-site", "doc-cast":
			followers[doc.StartByte] = tsjsDocFollower{node, kind}
		}
	}
	g.jsdocComments(&syntax, src, comments, followers)
	for _, key := range callOrder {
		call := calls[key]
		if shadowed && call.mode == tsjsRequire {
			continue
		}
		g.addRequest(&syntax, src, call.operand, call.line, call.mode, nil)
		if call.operand.EndByte != 0 && call.args.EndByte > call.operand.EndByte &&
			!tsjsCallTail(src[call.operand.EndByte:call.args.EndByte]) {
			g.unknown(UnknownUnsupportedConstruct, "", call.line, DecideToolchain, DecideObservation)
		}
	}
	for offset, node := range loaders {
		if !used[offset] {
			g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideObservation)
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
		code := UnknownDynamicImport
		if mode == tsjsStatic || existing != nil {
			code = UnknownUnsupportedConstruct
		}
		g.unknown(code, "***", stmtLine, DecideToolchain, DecideObservation)
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
		g.unknown(UnknownUnsupportedConstruct, "", line, DecideToolchain, DecideConfigFile)
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

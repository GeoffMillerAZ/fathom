package deps

import (
	"bytes"
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

var tsjsDocMention = regexp.MustCompile(`import[\p{Zs}\t\r\n\v\f\x{2028}\x{2029}\x{feff}]*\(`)
var tsjsDocInlineLink = regexp.MustCompile(`\{@(?:link|linkcode|linkplain)\b[^}]*\}?`)
var tsjsDocName = regexp.MustCompile(`^[$_\pL]`)
var tsjsDocNonLF = regexp.MustCompile(`\r(?:[^\n]|$)|[\x{2028}\x{2029}]`)
var tsjsDocTag = regexp.MustCompile(`(?m)^[ \t]*@([a-zA-Z]+)\b`)

const tsjsDocTypePrefix = "type Doc = "

const tsjsDocTypeQuery = `
(program . (type_alias_declaration) @type .)
[(ERROR) (MISSING) (comment)] @invalid
(call_expression function: (_) @callee) @call
(call_expression function: (import) @keyword arguments: (arguments . (string) @operand .))
`

type tsjsDocFollower struct {
	node tsengine.Node
	kind string
}

func (g *tsjsRead) jsdocComments(syntax *tsjsSyntax, src []byte, comments []tsengine.Node, followers map[uint32]tsjsDocFollower) {
	// A run of leading comments shares the following declaration. Walk the
	// captured siblings backwards to prove each link once.
	slices.SortFunc(comments, func(a, b tsengine.Node) int { return cmp.Compare(b.StartByte, a.StartByte) })
	attached := make(map[uint32]bool)
	var parser tsjsDocParser
	lineBreaksChecked, physicalRows := false, false
	defer func() {
		if parser.lease != nil {
			parser.lease.Release()
		}
	}()
	for _, node := range comments {
		follower := followers[node.StartByte]
		if follower.node.StartByte >= node.EndByte &&
			len(bytes.TrimSpace(src[node.EndByte:follower.node.StartByte])) == 0 {
			attached[node.StartByte] = follower.kind != "doc-next" || attached[follower.node.StartByte]
		}
		comment := node.Text(src)
		if !strings.HasPrefix(comment, "/**") || !strings.Contains(comment, "import") {
			continue
		}
		body := tsjsDocBody(comment)
		hasImportTag := strings.Contains(comment, "@import")
		mentions := tsjsDocMention.FindAllIndex(body, -1)
		if len(mentions) == 0 && !hasImportTag {
			continue
		}
		if !lineBreaksChecked {
			// Tree-sitter's rows use LF. Do not bind a site to a row that
			// might disagree with JavaScript's line terminators.
			physicalRows = !tsjsDocNonLF.Match(src)
			lineBreaksChecked = true
		}
		lineStart := bytes.LastIndexByte(src[:node.StartByte], '\n') + 1
		leading := len(bytes.TrimSpace(src[lineStart:node.StartByte])) == 0
		if !physicalRows || !attached[node.StartByte] || !leading && follower.kind != "doc-cast" ||
			!g.jsdocComment(syntax, &parser, body, mentions, node.Line()) || hasImportTag {
			g.unknown(UnknownUnsupportedConstruct, "", node.Line(), DecideToolchain, DecideConfigFile)
		}
	}
}

func tsjsDocBody(comment string) []byte {
	if len(comment) < 5 || !strings.HasSuffix(comment, "*/") {
		return []byte(comment)
	}
	// Blank the delimiters and line decoration in place so offsets still
	// refer to the original comment.
	body := []byte(comment)
	copy(body, "   ")
	copy(body[len(body)-2:], "  ")
	for i := 0; i < len(body); i++ {
		if body[i] != '\n' {
			continue
		}
		j := i + 1
		for j < len(body) && (body[j] == ' ' || body[j] == '\t') {
			j++
		}
		if j < len(body) && body[j] == '*' {
			body[j] = ' '
		}
	}
	return body
}

func (g *tsjsRead) jsdocComment(syntax *tsjsSyntax, parser *tsjsDocParser, body []byte, mentions [][]int, line int) bool {
	pending := make(map[int]bool)
	for _, at := range mentions {
		pending[at[0]] = true
	}
	end, lastOperand := 0, 0
	seenType, seenReturn, objectTypedef, templateForbidden := false, false, false, false
	links := tsjsDocInlineLink.FindAllIndex(body, -1)
	for _, tag := range tsjsDocTag.FindAllSubmatchIndex(body, -1) {
		for len(links) != 0 && links[0][1] <= tag[0] {
			links = links[1:]
		}
		if tag[0] < end || len(links) != 0 && links[0][0] < tag[0] {
			continue
		}
		name := string(body[tag[2]:tag[3]])
		tail := bytes.TrimLeft(body[tag[1]:], " \t\r\n")
		start := len(body) - len(tail)
		end = tsjsDocGroupEnd(body, start, '{', '}')
		if end < 0 {
			break
		}
		property := (name == "property" || name == "prop") && objectTypedef
		if name != "property" && name != "prop" {
			objectTypedef = false
		}
		duplicate := false
		switch name {
		case "type":
			duplicate, seenType = seenType, true
		case "return", "returns":
			duplicate, seenReturn = seenReturn, true
		case "typedef", "callback", "overload":
			templateForbidden = true
		}
		if end <= start || duplicate || name == "template" && templateForbidden {
			continue
		}
		payload := body[start+1 : end]
		if name == "typedef" {
			switch string(bytes.TrimSpace(payload)) {
			case "Object", "object", "Object[]", "object[]":
				objectTypedef = true
			}
		}
		if (!property && !tsjsDocTypeTag(name)) || !tsjsDocMention.Match(payload) {
			continue
		}
		if !tsjsDocTagTail(name, body[end+1:]) {
			continue
		}
		if parser.lease == nil && !parser.open(g) {
			return false
		}
		for _, call := range parser.literals(payload) {
			keyword := start + 1 + int(call.keyword.StartByte) - len(tsjsDocTypePrefix)
			operand := start + 1 + int(call.operand.StartByte) - len(tsjsDocTypePrefix)
			if !pending[keyword] {
				continue
			}
			literal := call.operand.Text(parser.input)
			raw, ok := tsjsLiteral(literal)
			if !ok || strings.ContainsAny(literal, "\r\n") {
				continue
			}
			delete(pending, keyword)
			line += bytes.Count(body[lastOperand:operand], []byte{'\n'})
			lastOperand = operand
			syntax.requests = append(syntax.requests, tsjsRequest{raw, line, tsjsStatic})
		}
	}
	return len(pending) == 0
}

func tsjsDocTypeTag(name string) bool {
	switch name {
	case "type", "typedef", "param", "arg", "argument", "return", "returns", "template", "this", "enum", "satisfies":
		return true
	}
	return false
}

func tsjsDocTagTail(name string, tail []byte) bool {
	switch name {
	case "param", "arg", "argument", "property", "prop", "template":
		tail = bytes.TrimLeft(tail, " \t\r\n")
		if len(tail) != 0 && tail[0] == '[' {
			close := tsjsDocGroupEnd(tail, 0, '[', ']')
			if close < 0 {
				return false
			}
			tail = bytes.TrimSpace(tail[1:close])
		}
		return tsjsDocName.Match(tail)
	}
	return true
}

// tsjsDocGroupEnd finds the matching delimiter while respecting quotes. It
// returns start for an absent group and -1 for an unterminated group.
func tsjsDocGroupEnd(body []byte, start int, open, close byte) int {
	if start >= len(body) || body[start] != open {
		return start
	}
	depth := 1
	var quote byte
	for i := start + 1; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type tsjsDocParser struct {
	lease *tsengine.Lease
	query *tsengine.Query
	input []byte
}

func (p *tsjsDocParser) open(g *tsjsRead) bool {
	lease, err := tsengine.AcquireParser(g.ctx, "deps:jsdoc-type", "typescript", "tree_sitter_typescript")
	if err != nil {
		return false
	}
	p.lease = lease
	parser := lease.Parser()
	p.query, _ = parser.Cached("dependency-jsdoc-type").(*tsengine.Query)
	if p.query == nil {
		p.query, err = parser.CompileQuery(tsjsDocTypeQuery)
		if err != nil {
			lease.Discard()
			lease.Release()
			p.lease = nil
			return false
		}
		parser.SetCached("dependency-jsdoc-type", p.query)
	}
	return true
}

type tsjsDocCall struct {
	keyword, operand tsengine.Node
}

func (p *tsjsDocParser) literals(payload []byte) []tsjsDocCall {
	p.input = append(append(append(p.input[:0], tsjsDocTypePrefix...), payload...), ';')
	tree, err := p.lease.Parser().Parse(p.input)
	if err != nil {
		p.lease.Discard()
		p.lease = nil
		return nil
	}
	matches, err := p.query.Run(tree)
	tree.Close()
	if err != nil {
		p.lease.Discard()
		p.lease = nil
		return nil
	}
	var whole bool
	operands := make(map[uint32]tsengine.Node)
	var calls []tsengine.Node
	for _, match := range matches {
		var call, callee, keyword, operand tsengine.Node
		for _, c := range match.Captures {
			switch c.Name {
			case "invalid":
				return nil
			case "type":
				whole = c.Node.StartByte == 0 && int(c.Node.EndByte) == len(p.input)
			case "call":
				call = c.Node
			case "callee":
				callee = c.Node
			case "keyword":
				keyword = c.Node
			case "operand":
				operand = c.Node
			}
		}
		if call.EndByte != 0 {
			if callee.Text(p.input) != "import" {
				return nil
			}
			calls = append(calls, call)
		}
		if operand.EndByte != 0 && operand.StartByte > keyword.EndByte &&
			strings.ContainsRune("'\"", rune(p.input[operand.StartByte])) {
			operands[keyword.StartByte] = operand
		}
	}
	if !whole {
		return nil
	}
	slices.SortFunc(calls, func(a, b tsengine.Node) int { return cmp.Compare(a.StartByte, b.StartByte) })
	var out []tsjsDocCall
	var end uint32
	for _, call := range calls {
		if call.StartByte < end {
			continue
		}
		end = call.EndByte
		operand := operands[call.StartByte]
		// Require the whole operand: no computed suffix, second argument,
		// attributes, or trailing comma may enter the existing resolver.
		if operand.EndByte == 0 ||
			string(bytes.TrimSpace(p.input[call.StartByte+uint32(len("import")):operand.StartByte])) != "(" ||
			string(bytes.TrimSpace(p.input[operand.EndByte:call.EndByte])) != ")" {
			continue
		}
		out = append(out, tsjsDocCall{call, operand})
	}
	return out
}

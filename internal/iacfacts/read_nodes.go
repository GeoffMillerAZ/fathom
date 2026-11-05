package iacfacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// This file is the shared front of the CloudFormation and Pulumi YAML readers: one node
// tree with exact spans, built from YAML or from JSON. yaml.v3 gives a node's start only,
// and rejects JSON's \/ escape, so JSON has its own parser and a YAML node's end is found
// in the source.

type nodeKind int

const (
	nodeMap nodeKind = iota + 1
	nodeSeq
	nodeScalar
	nodeAlias
)

// node is one value of a template.
type node struct {
	kind nodeKind
	// tag is a custom YAML tag such as !Ref, with its leading !. Standard tags are not kept.
	tag string
	// value is a scalar's text, or an alias's anchor name.
	value string
	// typ is a scalar's type: str, int, float, bool or null.
	typ   string
	pairs []pair
	items []*node
	span  Span
}

// pair is one entry of a mapping. span runs from the key's start to the value's end.
type pair struct {
	key, val *node
	span     Span
}

func (n *node) isStr() bool { return n != nil && n.kind == nodeScalar && n.typ == "str" }

// get returns the value under a mapping key, the first when it repeats.
func (n *node) get(key string) *node {
	if n == nil || n.kind != nodeMap {
		return nil
	}
	for _, p := range n.pairs {
		if p.key.kind == nodeScalar && p.key.value == key {
			return p.val
		}
	}
	return nil
}

// hasMerge reports whether a mapping holds a YAML merge key (`<<: *anchor`), whose entries the reader does not
// expand: the mapping may hold more keys than its pairs show.
func (n *node) hasMerge() bool {
	if n == nil || n.kind != nodeMap {
		return false
	}
	for _, p := range n.pairs {
		if p.key.kind == nodeScalar && p.key.value == "<<" {
			return true
		}
	}
	return false
}

// unreadableSection reports whether n stands where a mapping of resources should and is something the reader cannot
// expand: an alias (`*all`), a tagged value (`!If`), or a string that interpolates (`${v}`). Which resources such a
// section declares is not known, which the reader counts as a syntax error, since a resource that is in neither
// Resources nor Dropped must not exist in the file. A plain list or scalar declares none, and an absent or empty
// section has none.
func (n *node) unreadableSection() bool {
	if n == nil {
		return false
	}
	switch {
	case n.kind == nodeAlias, n.tag != "":
		return true
	case n.kind == nodeScalar && n.isStr():
		return strings.Contains(n.value, "${")
	}
	return false
}

// scalarValue is a scalar's value as Go sees it.
func (n *node) scalarValue() any {
	switch n.typ {
	case "int":
		if i, err := strconv.ParseInt(n.value, 0, 64); err == nil {
			return i
		}
	case "float":
		if f, err := strconv.ParseFloat(n.value, 64); err == nil {
			return f
		}
	case "bool":
		switch strings.ToLower(n.value) {
		case "true":
			return true
		case "false":
			return false
		}
	case "null":
		return nil
	}
	return n.value
}

// errSyntax marks input a parser could not read. It carries no source text.
var errSyntax = errors.New("iacfacts: syntax error")

// srcIndex maps byte offsets to line and column and back. Columns count characters.
type srcIndex struct {
	file   string
	src    []byte
	starts []int // byte offset of each line's start
	// For a file with non-ASCII text: the column of every byte, and the byte of every
	// character, so neither lookup walks a line.
	cols       []int32
	charToByte []int32
	lineChar   []int32 // global character index of each line's start
}

func newSrcIndex(file string, src []byte) *srcIndex {
	ix := &srcIndex{file: file, src: src, starts: []int{0}}
	ascii := true
	for i, b := range src {
		// A line ends at \n, at \r\n and at a lone \r, which is how YAML counts lines.
		if b == '\n' || (b == '\r' && (i+1 >= len(src) || src[i+1] != '\n')) {
			ix.starts = append(ix.starts, i+1)
		}
		if b >= 0x80 {
			ascii = false
		}
	}
	if ascii {
		return ix
	}
	ix.cols = make([]int32, len(src)+1)
	ix.charToByte = make([]int32, 0, len(src)+1)
	ix.lineChar = make([]int32, len(ix.starts))
	line, col := 0, int32(1)
	for i := 0; i <= len(src); {
		if line+1 < len(ix.starts) && i == ix.starts[line+1] {
			line++
			col = 1
		}
		if i == ix.starts[line] {
			ix.lineChar[line] = int32(len(ix.charToByte))
		}
		w := 1
		if i < len(src) {
			if _, size := utf8.DecodeRune(src[i:]); size > 1 {
				w = size
			}
		}
		ix.charToByte = append(ix.charToByte, int32(i))
		for k := 0; k < w && i+k <= len(src); k++ {
			ix.cols[i+k] = col
		}
		col++
		i += w
	}
	return ix
}

// pos returns the 1-based line and column of a byte offset.
func (ix *srcIndex) pos(off int) (int, int) {
	if off < 0 {
		off = 0
	}
	if off > len(ix.src) {
		off = len(ix.src)
	}
	line := sort.Search(len(ix.starts), func(i int) bool { return ix.starts[i] > off }) - 1
	if ix.cols != nil {
		return line + 1, int(ix.cols[off])
	}
	return line + 1, off - ix.starts[line] + 1
}

// offset returns the byte offset of a 1-based line and column.
func (ix *srcIndex) offset(line, col int) int {
	if line < 1 {
		line = 1
	}
	if line > len(ix.starts) {
		return len(ix.src)
	}
	if col < 1 {
		col = 1
	}
	if ix.charToByte == nil {
		off := ix.starts[line-1] + col - 1
		if off > len(ix.src) {
			off = len(ix.src)
		}
		return off
	}
	g := int(ix.lineChar[line-1]) + col - 1
	if g >= len(ix.charToByte) {
		return len(ix.src)
	}
	return int(ix.charToByte[g])
}

func (ix *srcIndex) span(start, end int) Span {
	sl, sc := ix.pos(start)
	el, ec := ix.pos(end)
	return Span{File: ix.file, StartLine: sl, StartCol: sc, EndLine: el, EndCol: ec}
}

func spanEnd(s Span) (int, int) { return s.EndLine, s.EndCol }

// laterEnd returns the later of two spans' ends.
func laterEnd(a, b Span) Span {
	al, ac := spanEnd(a)
	bl, bc := spanEnd(b)
	if bl > al || (bl == al && bc > ac) {
		return b
	}
	return a
}

func pairSpan(k, v *node) Span {
	s := k.span
	e := laterEnd(k.span, v.span)
	s.EndLine, s.EndCol = e.EndLine, e.EndCol
	return s
}

// tmplInfo is what parsing a template found besides the tree, counted as syntax errors by the readers: keys a
// mapping repeats, and YAML documents after the first, which are not read.
type tmplInfo struct {
	dupKeys   int
	extraDocs int
}

func (i tmplInfo) syntaxErrors() int { return i.dupKeys + i.extraDocs }

// maxExtraDocs bounds how many documents after the first are counted.
const maxExtraDocs = 64

// parseTemplate parses a YAML or JSON document into a node tree. A document whose first
// byte is { is read as JSON and, if that fails, as YAML. A template that does not parse
// returns errSyntax, and one past a limit returns ErrLimit.
func parseTemplate(file string, src []byte) (*node, tmplInfo, error) {
	var info tmplInfo
	if err := checkSize(file, len(src)); err != nil {
		return nil, info, err
	}
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	ix := newSrcIndex(file, src)
	if t := bytes.TrimLeft(src, " \t\r\n"); len(t) > 0 && t[0] == '{' {
		n, dups, err := parseJSON(ix)
		if err == nil || errors.Is(err, ErrLimit) {
			info.dupKeys = dups
			return n, info, err
		}
	}
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, info, nil
		}
		return nil, info, errSyntax
	}
	for i := 0; i < maxExtraDocs; i++ {
		var more yaml.Node
		if err := dec.Decode(&more); err != nil {
			if !errors.Is(err, io.EOF) {
				info.extraDocs++ // a later document that does not parse
			}
			break
		}
		info.extraDocs++
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, info, nil
	}
	st := &convState{ix: ix}
	n, err := st.convert(doc.Content[0], false, -1, false, 0)
	if err != nil {
		return nil, info, err
	}
	info.dupKeys = st.dups
	return n, info, nil
}

// countDupKeys counts the scalar keys of a mapping that repeat an earlier key.
func countDupKeys(pairs []pair) int {
	if len(pairs) < 2 {
		return 0
	}
	n := 0
	if len(pairs) <= 8 {
		for i := 1; i < len(pairs); i++ {
			if pairs[i].key.kind != nodeScalar {
				continue
			}
			for j := 0; j < i; j++ {
				if pairs[j].key.kind == nodeScalar && pairs[j].key.value == pairs[i].key.value {
					n++
					break
				}
			}
		}
		return n
	}
	seen := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		if p.key.kind != nodeScalar {
			continue
		}
		if seen[p.key.value] {
			n++
		}
		seen[p.key.value] = true
	}
	return n
}

type convState struct {
	ix    *srcIndex
	nodes int
	dups  int
}

func (st *convState) enter(depth int) error {
	if st.nodes++; st.nodes > MaxNodes {
		return limitErr(st.ix.file, fmt.Sprintf("more than %d nodes", MaxNodes))
	}
	if depth > MaxDepth {
		return limitErr(st.ix.file, fmt.Sprintf("nodes nest past %d deep", MaxDepth))
	}
	return nil
}

// convert builds a node from a yaml.Node. flow says the parent is a flow collection,
// indent is the 0-based column of the enclosing block collection, and isKey marks a
// mapping key.
func (st *convState) convert(y *yaml.Node, flow bool, indent int, isKey bool, depth int) (*node, error) {
	if err := st.enter(depth); err != nil {
		return nil, err
	}
	ix := st.ix
	start := ix.offset(y.Line, y.Column)
	n := &node{}
	if strings.HasPrefix(y.Tag, "!") && !strings.HasPrefix(y.Tag, "!!") {
		n.tag = y.Tag
	}
	switch y.Kind {
	case yaml.AliasNode:
		n.kind, n.value = nodeAlias, y.Value
		n.span = ix.span(start, start+1+len(y.Value))
		return n, nil
	case yaml.ScalarNode:
		n.kind, n.value = nodeScalar, y.Value
		switch y.Tag {
		case "!!int":
			n.typ = "int"
		case "!!float":
			n.typ = "float"
		case "!!bool":
			n.typ = "bool"
		case "!!null":
			n.typ = "null"
		default:
			n.typ = "str"
		}
		n.span = ix.span(start, st.scalarEnd(y, start, flow, indent, isKey))
		return n, nil
	case yaml.MappingNode, yaml.SequenceNode:
		isFlow := y.Style&yaml.FlowStyle != 0
		childIndent := y.Column - 1
		if y.Kind == yaml.MappingNode {
			n.kind = nodeMap
			for i := 0; i+1 < len(y.Content); i += 2 {
				k, err := st.convert(y.Content[i], isFlow, childIndent, true, depth+1)
				if err != nil {
					return nil, err
				}
				v, err := st.convert(y.Content[i+1], isFlow, childIndent, false, depth+1)
				if err != nil {
					return nil, err
				}
				n.pairs = append(n.pairs, pair{key: k, val: v, span: pairSpan(k, v)})
			}
			st.dups += countDupKeys(n.pairs)
		} else {
			n.kind = nodeSeq
			for _, c := range y.Content {
				it, err := st.convert(c, isFlow, childIndent, false, depth+1)
				if err != nil {
					return nil, err
				}
				n.items = append(n.items, it)
			}
		}
		end := start
		var last *node
		if n.kind == nodeMap && len(n.pairs) > 0 {
			last = n.pairs[len(n.pairs)-1].val
		} else if n.kind == nodeSeq && len(n.items) > 0 {
			last = n.items[len(n.items)-1]
		}
		if last != nil {
			end = ix.offset(last.span.EndLine, last.span.EndCol)
		}
		if isFlow {
			if last == nil {
				end = skipProps(ix.src, start) + 1
			}
			end = st.flowCloser(end)
		}
		if end < start {
			end = start
		}
		n.span = ix.span(start, end)
		return n, nil
	}
	return nil, errSyntax
}

// skipProps skips a node's tag and anchor, and the space after them.
func skipProps(s []byte, i int) int {
	for i < len(s) && (s[i] == '!' || s[i] == '&') {
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			i++
		}
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
			i++
		}
	}
	return i
}

// flowCloser moves from the end of a flow collection's last child past commas, space and
// comments to the closing bracket, and returns the offset after it.
func (st *convState) flowCloser(i int) int {
	s := st.ix.src
	for j := i; j < len(s); j++ {
		switch s[j] {
		case ' ', '\t', '\n', '\r', ',':
		case '#':
			for j < len(s) && s[j] != '\n' && s[j] != '\r' {
				j++
			}
		case ']', '}':
			return j + 1
		default:
			return i
		}
	}
	return i
}

// eol is the offset of the line break that ends the line holding i: \n, \r\n or a lone \r.
func eol(s []byte, i int) int {
	for i < len(s) && s[i] != '\n' && s[i] != '\r' {
		i++
	}
	return i
}

// nextLine is the offset of the line after the break at e.
func nextLine(s []byte, e int) int {
	if e+1 < len(s) && s[e] == '\r' && s[e+1] == '\n' {
		return e + 2
	}
	return e + 1
}

// scalarEnd finds the end offset of a scalar that starts at start.
func (st *convState) scalarEnd(y *yaml.Node, start int, flow bool, indent int, isKey bool) int {
	s := st.ix.src
	i := skipProps(s, start)
	if i >= len(s) {
		return len(s)
	}
	switch {
	case y.Style&yaml.DoubleQuotedStyle != 0 && s[i] == '"':
		for j := i + 1; j < len(s); j++ {
			if s[j] == '\\' {
				j++
			} else if s[j] == '"' {
				return j + 1
			}
		}
		return len(s)
	case y.Style&yaml.SingleQuotedStyle != 0 && s[i] == '\'':
		for j := i + 1; j < len(s); j++ {
			if s[j] == '\'' {
				if j+1 < len(s) && s[j+1] == '\'' {
					j++
					continue
				}
				return j + 1
			}
		}
		return len(s)
	case y.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 && (s[i] == '|' || s[i] == '>'):
		last := eol(s, i)
		for ls := nextLine(s, last); ls < len(s); {
			le := eol(s, ls)
			lineEnd := le
			if lineEnd > ls && s[lineEnd-1] == '\r' {
				lineEnd--
			}
			sp := 0
			for ls+sp < lineEnd && s[ls+sp] == ' ' {
				sp++
			}
			if ls+sp < lineEnd && s[ls+sp] != '\t' {
				if sp <= indent {
					break
				}
				last = lineEnd
			} else if ls+sp < lineEnd {
				last = lineEnd
			}
			ls = nextLine(s, le)
		}
		return last
	}
	// Plain.
	end := plainLineEnd(s, i, flow, isKey)
	if !flow && !isKey && y.Value != "" {
		for {
			ls := nextLine(s, eol(s, end))
			for ls < len(s) {
				le := eol(s, ls)
				blank := true
				for k := ls; k < le; k++ {
					if s[k] != ' ' && s[k] != '\t' && s[k] != '\r' {
						blank = false
					}
				}
				if !blank {
					break
				}
				ls = nextLine(s, le)
			}
			if ls >= len(s) {
				break
			}
			sp := 0
			for ls+sp < len(s) && s[ls+sp] == ' ' {
				sp++
			}
			if sp <= indent || s[ls+sp] == '#' || s[ls+sp] == '\n' || s[ls+sp] == '\r' {
				break
			}
			end = plainLineEnd(s, ls+sp, false, false)
		}
	}
	return end
}

// plainLineEnd is the end of a plain scalar's text on the line that holds offset i.
func plainLineEnd(s []byte, i int, flow, isKey bool) int {
	j := i
	for j < len(s) && s[j] != '\n' && s[j] != '\r' {
		c := s[j]
		if c == '#' && j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
			break
		}
		if isKey && c == ':' && (j+1 >= len(s) || s[j+1] == ' ' || s[j+1] == '\n' || s[j+1] == '\r' || s[j+1] == '\t' ||
			(flow && (s[j+1] == ',' || s[j+1] == ']' || s[j+1] == '}'))) {
			break
		}
		if flow && (c == ',' || c == ']' || c == '}') {
			break
		}
		j++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\r') {
		j--
	}
	return j
}

// parseJSON reads a JSON document into a node tree with exact spans.
func parseJSON(ix *srcIndex) (*node, int, error) {
	p := &jsonParser{ix: ix, s: ix.src}
	p.ws()
	n, err := p.value(0)
	if err != nil {
		return nil, 0, err
	}
	p.ws()
	if p.i != len(p.s) {
		return nil, 0, errSyntax
	}
	return n, p.dups, nil
}

type jsonParser struct {
	ix    *srcIndex
	s     []byte
	i     int
	nodes int
	dups  int
}

func (p *jsonParser) ws() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *jsonParser) value(depth int) (*node, error) {
	if p.nodes++; p.nodes > MaxNodes {
		return nil, limitErr(p.ix.file, fmt.Sprintf("more than %d nodes", MaxNodes))
	}
	if depth > MaxDepth {
		return nil, limitErr(p.ix.file, fmt.Sprintf("nodes nest past %d deep", MaxDepth))
	}
	if p.i >= len(p.s) {
		return nil, errSyntax
	}
	start := p.i
	switch c := p.s[p.i]; {
	case c == '{':
		n := &node{kind: nodeMap}
		p.i++
		p.ws()
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			n.span = p.ix.span(start, p.i)
			return n, nil
		}
		for {
			p.ws()
			if p.i >= len(p.s) || p.s[p.i] != '"' {
				return nil, errSyntax
			}
			k, err := p.str(depth + 1)
			if err != nil {
				return nil, err
			}
			p.ws()
			if p.i >= len(p.s) || p.s[p.i] != ':' {
				return nil, errSyntax
			}
			p.i++
			p.ws()
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			n.pairs = append(n.pairs, pair{key: k, val: v, span: pairSpan(k, v)})
			p.ws()
			if p.i >= len(p.s) {
				return nil, errSyntax
			}
			if p.s[p.i] == ',' {
				p.i++
				continue
			}
			if p.s[p.i] == '}' {
				p.i++
				p.dups += countDupKeys(n.pairs)
				n.span = p.ix.span(start, p.i)
				return n, nil
			}
			return nil, errSyntax
		}
	case c == '[':
		n := &node{kind: nodeSeq}
		p.i++
		p.ws()
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			n.span = p.ix.span(start, p.i)
			return n, nil
		}
		for {
			p.ws()
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, v)
			p.ws()
			if p.i >= len(p.s) {
				return nil, errSyntax
			}
			if p.s[p.i] == ',' {
				p.i++
				continue
			}
			if p.s[p.i] == ']' {
				p.i++
				n.span = p.ix.span(start, p.i)
				return n, nil
			}
			return nil, errSyntax
		}
	case c == '"':
		return p.str(depth + 1)
	case c == 't' && bytes.HasPrefix(p.s[p.i:], []byte("true")):
		p.i += 4
		return &node{kind: nodeScalar, typ: "bool", value: "true", span: p.ix.span(start, p.i)}, nil
	case c == 'f' && bytes.HasPrefix(p.s[p.i:], []byte("false")):
		p.i += 5
		return &node{kind: nodeScalar, typ: "bool", value: "false", span: p.ix.span(start, p.i)}, nil
	case c == 'n' && bytes.HasPrefix(p.s[p.i:], []byte("null")):
		p.i += 4
		return &node{kind: nodeScalar, typ: "null", span: p.ix.span(start, p.i)}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		for p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+' || p.s[p.i] == '.' || p.s[p.i] == 'e' || p.s[p.i] == 'E' ||
			(p.s[p.i] >= '0' && p.s[p.i] <= '9')) {
			p.i++
		}
		text := string(p.s[start:p.i])
		typ := "int"
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			if _, err := strconv.ParseFloat(text, 64); err != nil {
				return nil, errSyntax
			}
			typ = "float"
		}
		return &node{kind: nodeScalar, typ: typ, value: text, span: p.ix.span(start, p.i)}, nil
	}
	return nil, errSyntax
}

// str reads a JSON string at p.i and decodes it.
func (p *jsonParser) str(depth int) (*node, error) {
	if p.nodes++; p.nodes > MaxNodes {
		return nil, limitErr(p.ix.file, fmt.Sprintf("more than %d nodes", MaxNodes))
	}
	start := p.i
	j := p.i + 1
	for j < len(p.s) {
		if p.s[j] == '\\' {
			j += 2
			continue
		}
		if p.s[j] == '"' {
			break
		}
		if p.s[j] < 0x20 {
			return nil, errSyntax
		}
		j++
	}
	if j >= len(p.s) {
		return nil, errSyntax
	}
	var v string
	if err := json.Unmarshal(p.s[start:j+1], &v); err != nil {
		return nil, errSyntax
	}
	p.i = j + 1
	return &node{kind: nodeScalar, typ: "str", value: v, span: p.ix.span(start, p.i)}, nil
}

// walkTree emits the attributes of a value. A node the classifier handles (an intrinsic
// function) is one attribute. A mapping descends by key, a list by [*], and a scalar is
// one attribute. An empty mapping or list is one literal attribute.
type treeWalker struct {
	emitter
	file     string
	classify func(n *node, depth int) (readRes, bool)
}

func (w *treeWalker) walk(n *node, path string, span Span, out *[]Attr, depth int) {
	if w.err != nil || n == nil {
		return
	}
	if depth > MaxDepth {
		w.fail(w.file, fmt.Sprintf("nodes nest past %d deep", MaxDepth))
		return
	}
	if res, ok := w.classify(n, 0); ok {
		w.emit(w.file, out, path, res, span)
		return
	}
	switch n.kind {
	case nodeMap:
		if len(n.pairs) == 0 {
			w.emitLit(out, path, span, map[string]any{})
			return
		}
		for _, p := range n.pairs {
			w.walk(p.val, joinPath(path, p.key.value), p.span, out, depth+1)
		}
	case nodeSeq:
		if len(n.items) == 0 {
			w.emitLit(out, path, span, []any{})
			return
		}
		for _, it := range n.items {
			w.walk(it, path+"[*]", it.span, out, depth+1)
		}
	case nodeScalar:
		w.emit(w.file, out, path, readRes{class: ClassLiteral, val: n.scalarValue(), hasV: w.cfg.values}, span)
	default:
		w.emit(w.file, out, path, readRes{class: ClassUnknown}, span)
	}
}

func (w *treeWalker) emitLit(out *[]Attr, path string, span Span, v any) {
	w.emit(w.file, out, path, readRes{class: ClassLiteral, val: v, hasV: w.cfg.values}, span)
}

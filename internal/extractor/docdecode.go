package extractor

// docdecode.go — the document engine's decoders (LB-F8, design §4): YAML
// (single or multi-document), JSON, JSONC, TOML and XML into one docNode
// tree with lines. A map value's line is its key's line: a symbol selected
// from a map is "defined" where its key is written, as every bespoke config
// extractor reports it.

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// decodeDoc returns the document roots of src (one unless multi, YAML).
// yamlTags names the scheme a YAML local tag is read by ("" drops it, as
// every dialect but CloudFormation does; "cloudformation" reads a short-form
// intrinsic as its long form, cfnIntrinsic).
func decodeDoc(format string, src []byte, multi bool, yamlTags string) ([]*docNode, error) {
	var docs []*docNode
	var err error
	switch format {
	case "yaml":
		if looksTOML(src) {
			// A tool that reads a configuration as YAML or TOML (Traefik's
			// static and file-provider files) names one spec: a file that
			// opens as a TOML document is read as one, with the same selectors.
			if n, terr := decodeTOMLDoc(src); terr == nil {
				docs = []*docNode{n}
				break
			}
		}
		docs, err = decodeYAMLDoc(src, multi, yamlTags)
	case "json":
		var n *docNode
		n, err = decodeJSONDoc(src)
		docs = []*docNode{n}
	case "jsonc":
		var n *docNode
		n, err = decodeJSONDoc(stripJSONC(src))
		docs = []*docNode{n}
	case "toml":
		var n *docNode
		n, err = decodeTOMLDoc(src)
		docs = []*docNode{n}
	case "xml":
		var n *docNode
		n, err = decodeXMLDoc(src)
		docs = []*docNode{n}
	case "ini":
		var n *docNode
		n, err = decodeINIDoc(src)
		docs = []*docNode{n}
	case "compile-commands":
		var n *docNode
		n, err = decodeCompDB(src)
		docs = []*docNode{n}
	default:
		return nil, fmt.Errorf("doc: unknown decode %q", format)
	}
	if err != nil {
		return nil, err
	}
	out := docs[:0]
	for _, d := range docs {
		if d != nil {
			fixEndLines(d)
			out = append(out, d)
		}
	}
	return out, nil
}

// tomlOpenRe matches the first meaningful line of a TOML document: a table
// header ([a.b], [[a]]) or an assignment (key = value, quoted keys included).
// No YAML document a dialect reads opens with either: a root flow sequence
// is not a configuration, and `key = value` is not a mapping entry.
var tomlOpenRe = regexp.MustCompile(`^(\[\[?[A-Za-z0-9_.\-"' ]+\]\]?\s*(#.*)?|("[^"]*"|'[^']*'|[A-Za-z0-9_.\-]+)\s*=[^=].*)$`)

// looksTOML reports whether src's first line that is neither blank nor a
// comment opens a TOML table or assignment.
func looksTOML(src []byte) bool {
	for _, ln := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return tomlOpenRe.MatchString(t)
	}
	return false
}

// ── YAML ──────────────────────────────────────────────────────────────────

func decodeYAMLDoc(src []byte, multi bool, yamlTags string) ([]*docNode, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var out []*docNode
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(out) > 0 {
				return out, nil // keep the documents before the bad one
			}
			return nil, err
		}
		if len(doc.Content) > 0 {
			out = append(out, fromYAML(doc.Content[0], 0, yamlTags))
		}
		if !multi {
			break
		}
	}
	return out, nil
}

// fromYAML converts a yaml.v3 node, following aliases (bounded depth, so
// an alias cycle cannot recurse forever). A local tag (`!Name`) is dropped —
// the node reads as its untagged value — unless yamlTags names a scheme that
// reads it (tagYAML).
func fromYAML(n *yaml.Node, depth int, yamlTags string) *docNode {
	if n == nil || depth > 64 {
		return nil
	}
	if n.Kind == yaml.AliasNode {
		out := fromYAML(n.Alias, depth+1, yamlTags)
		if out != nil {
			cp := *out
			cp.line = n.Line
			return &cp
		}
		return nil
	}
	if yamlTags == yamlTagsCloudFormation {
		if name, ok := cfnIntrinsic(n.Tag); ok {
			return cfnLongForm(n, name, depth, yamlTags)
		}
	}
	d := &docNode{line: n.Line}
	switch n.Kind {
	case yaml.MappingNode:
		d.kind = docMap
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			vn := fromYAML(v, depth+1, yamlTags)
			if vn == nil {
				vn = &docNode{kind: docNull}
			}
			if yamlTags == yamlTagsCloudFormation && k.Value == "Fn::GetAtt" && vn.kind == docScalar {
				vn = cfnGetAttList(vn.scalar, v.Line)
			}
			vn.line = k.Line
			d.set(k.Value, vn)
		}
	case yaml.SequenceNode:
		d.kind = docSeq
		for _, it := range n.Content {
			if in := fromYAML(it, depth+1, yamlTags); in != nil {
				d.items = append(d.items, in)
			}
		}
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			d.kind = docNull
		} else {
			d.kind = docScalar
			d.scalar = n.Value
		}
	default:
		return nil
	}
	return d
}

// yamlTagsCloudFormation is the one yaml_tags scheme (DocSpec.YAMLTags).
const yamlTagsCloudFormation = "cloudformation"

// cfnIntrinsic names the long form of a CloudFormation short-form tag, the
// rule cfn-lint's decoder applies (cfnlint/decode/cfn_yaml.py): `!Ref` and
// `!Condition` are their own keys, any other local tag `!Name` is `Fn::Name`
// (`!Sub`, `!GetAtt`, `!If`, `!Join`, `!Select`, `!Equals`, …, and a
// transform's own, `!Rain::Embed`). A standard tag (`!!str`) is not one.
func cfnIntrinsic(tag string) (string, bool) {
	if len(tag) < 2 || tag[0] != '!' || tag[1] == '!' {
		return "", false
	}
	switch name := tag[1:]; name {
	case "Ref", "Condition":
		return name, true
	default:
		return "Fn::" + name, true
	}
}

// cfnLongForm reads a tagged node `!Name v` as the one-key map {long name:
// v}, on the tag's line; `!GetAtt A.B` is {Fn::GetAtt: [A, B]}, the list the
// short form's dotted string abbreviates.
func cfnLongForm(n *yaml.Node, name string, depth int, yamlTags string) *docNode {
	var v *docNode
	if name == "Fn::GetAtt" && n.Kind == yaml.ScalarNode {
		v = cfnGetAttList(n.Value, n.Line)
	} else {
		inner := *n
		inner.Tag = ""
		if inner.Kind == yaml.ScalarNode && inner.Value == "" {
			inner.Tag = "!!null"
		}
		v = fromYAML(&inner, depth+1, yamlTags)
	}
	if v == nil {
		v = &docNode{kind: docNull}
	}
	v.line = n.Line
	d := &docNode{kind: docMap, line: n.Line}
	d.set(name, v)
	return d
}

// cfnGetAttList splits a GetAtt's `resource.attribute` at its first dot
// into the two-item list the long form writes ([resource, attribute]); an
// attribute may itself hold dots (Endpoint.Address).
func cfnGetAttList(s string, line int) *docNode {
	d := &docNode{kind: docSeq, line: line}
	for _, part := range strings.SplitN(s, ".", 2) {
		d.items = append(d.items, &docNode{kind: docScalar, scalar: part, line: line})
	}
	return d
}

// ── JSON / JSONC ──────────────────────────────────────────────────────────

// lineIndex answers the 1-based line of a byte offset.
type lineIndex []int

func newLineIndex(src []byte) lineIndex {
	idx := lineIndex{0}
	for i, b := range src {
		if b == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (li lineIndex) line(off int) int {
	return sort.Search(len(li), func(i int) bool { return li[i] > off })
}

// tokenStart skips the whitespace and separators json.Decoder leaves
// between InputOffset and the next token.
func tokenStart(src []byte, off int) int {
	for off < len(src) {
		switch src[off] {
		case ' ', '\t', '\r', '\n', ',', ':':
			off++
		default:
			return off
		}
	}
	return off
}

func decodeJSONDoc(src []byte) (*docNode, error) {
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	li := newLineIndex(src)
	n, err := jsonValue(dec, src, li)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func jsonValue(dec *json.Decoder, src []byte, li lineIndex) (*docNode, error) {
	start := tokenStart(src, int(dec.InputOffset()))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d := &docNode{line: li.line(start)}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			d.kind = docMap
			for dec.More() {
				kstart := tokenStart(src, int(dec.InputOffset()))
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				v, err := jsonValue(dec, src, li)
				if err != nil {
					return nil, err
				}
				v.line = li.line(kstart)
				d.set(k, v)
			}
		case '[':
			d.kind = docSeq
			for dec.More() {
				v, err := jsonValue(dec, src, li)
				if err != nil {
					return nil, err
				}
				d.items = append(d.items, v)
			}
		default:
			return nil, fmt.Errorf("json: unexpected %v", t)
		}
		end := int(dec.InputOffset())
		if _, err := dec.Token(); err != nil { // the closing delimiter
			return nil, err
		}
		d.endLine = li.line(tokenStart(src, end))
	case nil:
		d.kind = docNull
	case string:
		d.kind, d.scalar = docScalar, t
	case json.Number:
		d.kind, d.scalar = docScalar, t.String()
	case bool:
		d.kind, d.scalar = docScalar, strconv.FormatBool(t)
	}
	return d, nil
}

// stripJSONC blanks JSONC's comments and trailing commas with spaces,
// keeping every newline (so lines stay true) and every string intact.
func stripJSONC(src []byte) []byte {
	out := append([]byte(nil), src...)
	inStr := false
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch {
		case inStr:
			if c == '\\' {
				i++
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			for ; i < len(out) && out[i] != '\n'; i++ {
				out[i] = ' '
			}
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			out[i], out[i+1] = ' ', ' '
			for i += 2; i < len(out); i++ {
				if out[i] == '*' && i+1 < len(out) && out[i+1] == '/' {
					out[i], out[i+1] = ' ', ' '
					i++
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
			}
		case c == ',':
			if j := skipJSONCSpace(out, i+1); j < len(out) && (out[j] == '}' || out[j] == ']') {
				out[i] = ' '
			}
		}
	}
	return out
}

// skipJSONCSpace returns the first offset from j that is neither
// whitespace nor inside a comment.
func skipJSONCSpace(b []byte, j int) int {
	for j < len(b) {
		switch {
		case b[j] == ' ' || b[j] == '\t' || b[j] == '\r' || b[j] == '\n':
			j++
		case b[j] == '/' && j+1 < len(b) && b[j+1] == '/':
			for j < len(b) && b[j] != '\n' {
				j++
			}
		case b[j] == '/' && j+1 < len(b) && b[j+1] == '*':
			j += 2
			for j+1 < len(b) && !(b[j] == '*' && b[j+1] == '/') {
				j++
			}
			j += 2
		default:
			return j
		}
	}
	return j
}

// ── TOML ──────────────────────────────────────────────────────────────────

// decodeTOMLDoc decodes with go-toml into a generic tree, then recovers
// lines from the source: a table's header line (`[a.b]`, the i-th
// `[[a.b]]` for an array-of-tables item) or a key's assignment line within
// its table. Keys keep source order where a line is known.
func decodeTOMLDoc(src []byte) (*docNode, error) {
	var v map[string]any
	if err := toml.Unmarshal(src, &v); err != nil {
		return nil, err
	}
	loc := newTOMLLocator(src)
	return fromTOML(v, nil, loc, 1), nil
}

func fromTOML(v any, keyPath []string, loc *tomlLocator, line int) *docNode {
	switch t := v.(type) {
	case map[string]any:
		d := &docNode{kind: docMap, line: line}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		lines := map[string]int{}
		for _, k := range keys {
			lines[k] = loc.lineOf(append(append([]string(nil), keyPath...), k), line)
		}
		sort.Slice(keys, func(i, j int) bool {
			if lines[keys[i]] != lines[keys[j]] {
				return lines[keys[i]] < lines[keys[j]]
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			kp := append(append([]string(nil), keyPath...), k)
			d.set(k, fromTOMLAt(t[k], kp, loc, lines[k]))
		}
		return d
	case []any:
		d := &docNode{kind: docSeq, line: line}
		for i, it := range t {
			il := line
			if _, isTable := it.(map[string]any); isTable {
				il = loc.arrayTableLine(keyPath, i, line)
			}
			d.items = append(d.items, fromTOML(it, keyPath, loc, il))
		}
		return d
	case nil:
		return &docNode{kind: docNull, line: line}
	default:
		return &docNode{kind: docScalar, scalar: fmt.Sprint(t), line: line}
	}
}

func fromTOMLAt(v any, keyPath []string, loc *tomlLocator, line int) *docNode {
	return fromTOML(v, keyPath, loc, line)
}

var (
	tomlHeaderRe = regexp.MustCompile(`^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)
	tomlAssignRe = regexp.MustCompile(`^\s*("[^"]*"|'[^']*'|[A-Za-z0-9_.\-" ]+?)\s*=`)
)

type tomlLine struct {
	table    string // the enclosing header's dotted path ("" top level)
	array    bool   // the header is [[...]]
	isHeader bool
	key      string // the assignment's dotted key, when not a header
	line     int
}

type tomlLocator struct{ lines []tomlLine }

func normTOMLKey(k string) string {
	parts := strings.Split(k, ".")
	for i, p := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(p), `"'`)
	}
	return strings.Join(parts, ".")
}

func newTOMLLocator(src []byte) *tomlLocator {
	loc := &tomlLocator{}
	table := ""
	for i, raw := range strings.Split(string(src), "\n") {
		if m := tomlHeaderRe.FindStringSubmatch(raw); m != nil {
			table = normTOMLKey(m[2])
			loc.lines = append(loc.lines, tomlLine{table: table, array: m[1] == "[[", isHeader: true, line: i + 1})
			continue
		}
		if m := tomlAssignRe.FindStringSubmatch(raw); m != nil {
			loc.lines = append(loc.lines, tomlLine{table: table, key: normTOMLKey(m[1]), line: i + 1})
		}
	}
	return loc
}

// lineOf is the line keyPath is written on: its header, or its assignment
// (plain or dotted) within an enclosing table; else, for a table that is
// only implicit — reached through deeper headers ([tool.x.sub]) or dotted
// keys (x.y = 1) — the first line that writes under it; fallback when none.
func (l *tomlLocator) lineOf(keyPath []string, fallback int) int {
	if line, ok := l.explicitLine(keyPath, fallback); ok {
		return line
	}
	prefix := strings.Join(keyPath, ".") + "."
	for _, t := range l.lines {
		full := t.table
		if !t.isHeader {
			full = t.key
			if t.table != "" {
				full = t.table + "." + t.key
			}
		}
		if strings.HasPrefix(full, prefix) {
			return t.line
		}
	}
	return fallback
}

func (l *tomlLocator) explicitLine(keyPath []string, fallback int) (int, bool) {
	full := strings.Join(keyPath, ".")
	for _, t := range l.lines {
		if t.isHeader && t.table == full {
			return t.line, true
		}
	}
	for _, t := range l.lines {
		if t.isHeader {
			continue
		}
		if t.table == "" && t.key == full {
			return t.line, true
		}
		if t.table != "" && strings.HasPrefix(full, t.table+".") && t.key == strings.TrimPrefix(full, t.table+".") {
			if t.line >= fallback {
				return t.line, true
			}
		}
	}
	return 0, false
}

// arrayTableLine is the line of the i-th `[[keyPath]]` header.
func (l *tomlLocator) arrayTableLine(keyPath []string, i, fallback int) int {
	full := strings.Join(keyPath, ".")
	n := 0
	for _, t := range l.lines {
		if t.isHeader && t.array && t.table == full {
			if n == i {
				return t.line
			}
			n++
		}
	}
	return fallback
}

// ── XML ───────────────────────────────────────────────────────────────────

// decodeXMLDoc maps an XML document to the tree: the root element is the
// document; an element with neither attributes nor child elements is a
// scalar (its trimmed text); any other is a map of its attributes
// ("@name"), its child elements by local name (a repeated name becomes a
// sequence) and non-blank text ("#text").
func decodeXMLDoc(src []byte) (*docNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	li := newLineIndex(src)
	for {
		off := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return xmlElement(dec, se, li.line(xmlTagStart(src, off)), src, li)
		}
	}
}

func xmlTagStart(src []byte, off int) int {
	for off < len(src) && src[off] != '<' {
		off++
	}
	return off
}

func xmlElement(dec *xml.Decoder, se xml.StartElement, line int, src []byte, li lineIndex) (*docNode, error) {
	d := &docNode{kind: docMap, line: line}
	for _, a := range se.Attr {
		d.set("@"+a.Name.Local, &docNode{kind: docScalar, scalar: a.Value, line: line})
	}
	var text strings.Builder
	children := 0
	for {
		off := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			cl := li.line(xmlTagStart(src, off))
			c, err := xmlElement(dec, t, cl, src, li)
			if err != nil {
				return nil, err
			}
			children++
			name := t.Name.Local
			if prev := d.get(name); prev != nil {
				if prev.kind != docSeq || !prev.xmlRepeat {
					seq := &docNode{kind: docSeq, line: prev.line, items: []*docNode{prev}, xmlRepeat: true}
					d.set(name, seq)
					prev = seq
				}
				prev.items = append(prev.items, c)
			} else {
				d.set(name, c)
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			d.endLine = li.line(int(dec.InputOffset()) - 1)
			txt := strings.TrimSpace(text.String())
			if children == 0 && len(se.Attr) == 0 {
				return &docNode{kind: docScalar, scalar: txt, line: line, endLine: d.endLine}, nil
			}
			if txt != "" {
				d.set("#text", &docNode{kind: docScalar, scalar: txt, line: line})
			}
			return d, nil
		}
	}
}

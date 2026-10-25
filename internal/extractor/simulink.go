package extractor

// simulink.go -- the Simulink model reader (LB-F587, docs/plans/language-breadth.simulink.md;
// ADR-021 decision (1) as amended 2026-10-24). A Simulink model is a block diagram: blocks
// nested in subsystems, each identified by its full path (`model/Sub/Gain`, Simulink's own
// getfullname) and, from R2010b on, by a SID unique in the model. Two on-disk forms carry it:
//
//   - `.slx`, an OPC zip package, read in memory (archive/zip over the bytes, nothing written
//     to disk) under three caps: SimulinkMaxEntries entries in the package, SimulinkMaxInflated
//     bytes inflated across the parts read, and SimulinkMaxRatio between a part's inflated and
//     compressed sizes. Only the parts below are inflated. Layout eras:
//     before R2020a every block is inline in `simulink/blockdiagram.xml`
//     (`ModelInformation/Model|Library/System/Block/System/...`); from R2020a each system is
//     a part of its own, `simulink/systems/system_<n>.xml`, linked by a `<System Ref=…/>`
//     stub inside its SubSystem block, which the reader splices in place (`system_root`
//     first, then any part no other part names, each part walked once); model references
//     are read from `simulink/graphicalInterface.json` from R2024b (wrapped in
//     `GraphicalInterface` until R2026a, flat in R2026b), else `graphicalInterface.xml`, else
//     the diagram's inline `GraphicalInterface`. R2026b's `simulink/blockDiagram.json`
//     carries no block, so it is not read.
//   - `.mdl`, the classic nested-brace text (`Model|Library { System { Block { System {…} } } }`).
//     The text-package `.mdl` (R2027a: a `# MathWorks OPC Text Package` or `# Simulink Text
//     Package` banner, then the package's parts as text) is disclosed, never read:
//     ErrSimulinkTextPackage.
//
// Only a block under the diagram's System is a block: BlockDefaults and
// BlockParameterDefaults hold one template block per type, which no model has. A block's
// name keeps its spaces and a `/` is doubled in its path (Simulink's escape); a line break
// in a name (`&#xA;` in a .slx, `\n` in a .mdl) reads as one space, so a symbol name stays
// one line. A block's lines are those of its element (or brace node) in the part that holds
// it, recorded on the block (Part, "" for a .mdl).

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// The caps a .slx package is read under (ADR-021 (1), amended 2026-10-24).
const (
	// SimulinkMaxEntries is the most entries a package may list.
	SimulinkMaxEntries = 4096
	// SimulinkMaxInflated is the most bytes the parts read may inflate to, together.
	SimulinkMaxInflated = 32 << 20
	// SimulinkMaxRatio is the most a part read may inflate per compressed byte.
	SimulinkMaxRatio = 200
)

var (
	// ErrSimulinkCap: a package over one of the caps, read as nothing.
	ErrSimulinkCap = errors.New("simulink: package over a read cap")
	// ErrSimulinkTextPackage: a text-package .mdl, disclosed and not read.
	ErrSimulinkTextPackage = errors.New("simulink: text-package .mdl is not read")
)

// The layouts ReadSimulinkModel reports.
const (
	SimulinkLayoutInline  = "slx-inline"  // before R2020a: blocks inline in blockdiagram.xml
	SimulinkLayoutSystems = "slx-systems" // from R2020a: one systems/*.xml part per system
	SimulinkLayoutMDL     = "mdl"         // the classic brace text
)

// The source kinds the reader's extractor emits.
const (
	KindSimulinkModel     core.SourceKind = "SimulinkModel"
	KindSimulinkSubsystem core.SourceKind = "SimulinkSubsystem"
	KindSimulinkBlock     core.SourceKind = "SimulinkBlock"
)

// SimulinkBlock is one block of a model.
type SimulinkBlock struct {
	Path      string // full path from the model name, a `/` in a name doubled
	Parent    string // the enclosing subsystem's Path, or the model name
	Name      string // the block's own name, a line break read as a space
	SID       string
	BlockType string
	Subsystem bool   // the block owns a System
	Part      string // the package part holding the block ("" for a .mdl)
	Line      int
	EndLine   int
}

// SimulinkModelRef is a model reference: the referencing block's path and the model it names.
type SimulinkModelRef struct {
	BlockPath string
	ModelName string
}

// SimulinkModel is what ReadSimulinkModel reads of a model, blocks in depth-first order.
type SimulinkModel struct {
	Name      string // the model name: the file's base name, its extension dropped
	Layout    string
	Blocks    []SimulinkBlock
	ModelRefs []SimulinkModelRef
	SIDPath   map[string]string // SID -> Path
	// Signals is every signal or connection line, source block to each destination
	// block, a system's lines after those of the subsystems it holds; an endpoint that names no block of the model is dropped.
	Signals []SimulinkSignal
	ends    []simulinkEnds
}

// SimulinkSignal is one line from a source block to a destination block, by path.
type SimulinkSignal struct {
	Src, Dst string
}

// simulinkEnd is a line endpoint as written: a SID (`7#out:1`, a .slx's Src and Dst) or
// a block name in the line's system (a .mdl's SrcBlock and DstBlock).
type simulinkEnd struct{ sid, name string }

// simulinkEnds is one line before its endpoints resolve: its system's path, its source,
// its destinations (its own, then each branch's, depth first).
type simulinkEnds struct {
	system string
	src    simulinkEnd
	dsts   []simulinkEnd
}

// sidEnd reads `7#out:1` as SID 7.
func sidEnd(v string) simulinkEnd {
	sid, _, _ := strings.Cut(v, "#")
	return simulinkEnd{sid: strings.TrimSpace(sid)}
}

// resolve names an endpoint's block path, "" when no block of the model is it.
func (m *SimulinkModel) resolve(system string, e simulinkEnd, paths map[string]bool) string {
	if e.sid != "" {
		return m.SIDPath[e.sid]
	}
	if e.name != "" {
		if p := simulinkJoin(system, simulinkName(e.name)); paths[p] {
			return p
		}
	}
	return ""
}

// ReadSimulinkModel reads a .slx package or a .mdl file. A package over a cap is
// ErrSimulinkCap, a text-package .mdl ErrSimulinkTextPackage; a .mdl is read as far as its
// braces go.
func ReadSimulinkModel(relPath string, src []byte) (SimulinkModel, error) {
	m := SimulinkModel{Name: strings.TrimSuffix(path.Base(relPath), path.Ext(relPath)), SIDPath: map[string]string{}}
	var err error
	if bytes.HasPrefix(src, []byte("PK\x03\x04")) {
		err = m.readSLX(src)
	} else {
		err = m.readMDL(src)
	}
	if err != nil {
		return SimulinkModel{Name: m.Name, SIDPath: map[string]string{}}, err
	}
	for _, b := range m.Blocks {
		if b.SID != "" {
			if _, dup := m.SIDPath[b.SID]; !dup {
				m.SIDPath[b.SID] = b.Path
			}
		}
	}
	paths := make(map[string]bool, len(m.Blocks))
	for _, b := range m.Blocks {
		paths[b.Path] = true
	}
	for _, l := range m.ends {
		src := m.resolve(l.system, l.src, paths)
		if src == "" {
			continue
		}
		for _, d := range l.dsts {
			if dst := m.resolve(l.system, d, paths); dst != "" {
				m.Signals = append(m.Signals, SimulinkSignal{Src: src, Dst: dst})
			}
		}
	}
	m.ends = nil
	return m, nil
}

// simulinkName is a block name as a symbol holds it: each line break one space.
func simulinkName(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

// simulinkJoin appends a block's name to its parent's path, doubling a `/` in the name.
func simulinkJoin(parent, name string) string {
	return parent + "/" + strings.ReplaceAll(name, "/", "//")
}

// refPath reads a model reference's block path, `$bdroot` the model.
func (m *SimulinkModel) refPath(p string) string {
	if rest, ok := strings.CutPrefix(p, "$bdroot"); ok {
		return m.Name + rest
	}
	return p
}

// --- .slx ---------------------------------------------------------------------------

// xnode is one element of a part, as far as the reader keeps it.
type xnode struct {
	name     string
	attrs    map[string]string
	kids     []*xnode
	text     string
	line, to int
}

func (n *xnode) attr(k string) string { return n.attrs[k] }

func (n *xnode) children(name string) []*xnode {
	var out []*xnode
	for _, k := range n.kids {
		if k.name == name {
			out = append(out, k)
		}
	}
	return out
}

// prop is a <P Name="k"> child's text.
func (n *xnode) prop(k string) string {
	for _, c := range n.children("P") {
		if c.attr("Name") == k {
			return c.text
		}
	}
	return ""
}

// all is every element named name under n, at any depth, not descending into a match.
func (n *xnode) all(name string, out []*xnode) []*xnode {
	for _, k := range n.kids {
		if k.name == name {
			out = append(out, k)
		} else {
			out = k.all(name, out)
		}
	}
	return out
}

// parseXMLPart reads a part into its element tree, lines from the token offsets.
func parseXMLPart(b []byte) (*xnode, error) {
	var nl []int64
	for i, c := range b {
		if c == '\n' {
			nl = append(nl, int64(i))
		}
	}
	lineOf := func(off int64) int { return sort.Search(len(nl), func(i int) bool { return nl[i] >= off }) + 1 }
	dec := xml.NewDecoder(bytes.NewReader(b))
	doc := &xnode{}
	stack := []*xnode{doc}
	for {
		off := dec.InputOffset()
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		top := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xnode{name: t.Name.Local, line: lineOf(off)}
			if len(t.Attr) > 0 {
				n.attrs = make(map[string]string, len(t.Attr))
				for _, a := range t.Attr {
					n.attrs[a.Name.Local] = a.Value
				}
			}
			top.kids = append(top.kids, n)
			stack = append(stack, n)
		case xml.EndElement:
			// off is the `<` of an end tag, or just past a self-closed tag's `/>`.
			top.to = lineOf(off)
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if top.name == "P" { // the only text the reader keeps
				top.text += string(t)
			}
		}
	}
	return doc, nil
}

// slxParts reads the parts the model's blocks and references come from, under the caps.
func slxParts(src []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		return nil, fmt.Errorf("simulink: %w", err)
	}
	if len(zr.File) > SimulinkMaxEntries {
		return nil, fmt.Errorf("%w: %d entries, cap %d", ErrSimulinkCap, len(zr.File), SimulinkMaxEntries)
	}
	parts := map[string][]byte{}
	budget := int64(SimulinkMaxInflated)
	for _, f := range zr.File {
		name := f.Name
		wanted := name == "simulink/blockdiagram.xml" || name == "simulink/graphicalInterface.json" ||
			name == "simulink/graphicalInterface.xml" ||
			(strings.HasPrefix(name, "simulink/systems/") && strings.HasSuffix(name, ".xml") && !strings.Contains(name[len("simulink/systems/"):], "/"))
		if !wanted {
			continue
		}
		if f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 >= SimulinkMaxRatio {
			return nil, fmt.Errorf("%w: %s inflates %d:1, cap %d:1", ErrSimulinkCap, name, f.UncompressedSize64/f.CompressedSize64, SimulinkMaxRatio)
		}
		if int64(f.UncompressedSize64) > budget {
			return nil, fmt.Errorf("%w: parts inflate past %d bytes", ErrSimulinkCap, SimulinkMaxInflated)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("simulink: %s: %w", name, err)
		}
		// The declared size is not trusted: reading stops one byte past it or the budget.
		limit := min(budget, int64(f.UncompressedSize64))
		b, err := io.ReadAll(io.LimitReader(rc, limit+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("simulink: %s: %w", name, err)
		}
		if int64(len(b)) > limit {
			return nil, fmt.Errorf("%w: %s inflates past its declared size", ErrSimulinkCap, name)
		}
		budget -= int64(len(b))
		parts[name] = b
	}
	return parts, nil
}

func (m *SimulinkModel) readSLX(src []byte) error {
	parts, err := slxParts(src)
	if err != nil {
		return err
	}
	var diagram *xnode // ModelInformation/Model|Library
	if b, ok := parts["simulink/blockdiagram.xml"]; ok {
		doc, err := parseXMLPart(b)
		if err != nil {
			return fmt.Errorf("simulink: blockdiagram.xml: %w", err)
		}
		for _, info := range doc.children("ModelInformation") {
			for _, k := range info.kids {
				if (k.name == "Model" || k.name == "Library") && diagram == nil {
					diagram = k
				}
			}
		}
	}
	systems := map[string]*xnode{}
	var refs []string
	for name, b := range parts {
		if !strings.HasPrefix(name, "simulink/systems/") {
			continue
		}
		doc, err := parseXMLPart(b)
		if err != nil {
			return fmt.Errorf("simulink: %s: %w", name, err)
		}
		ref := strings.TrimSuffix(strings.TrimPrefix(name, "simulink/systems/"), ".xml")
		for _, s := range doc.children("System") {
			systems[ref] = s
		}
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	if len(systems) > 0 {
		m.Layout = SimulinkLayoutSystems
		w := slxWalk{m: m, systems: systems, seen: map[string]bool{}}
		named := map[string]bool{}
		for _, s := range systems {
			for _, st := range s.all("System", nil) {
				if r := st.attr("Ref"); r != "" {
					named[r] = true
				}
			}
		}
		// system_root first, then each part no part names, then each part still unreached.
		order := []string{}
		if _, ok := systems["system_root"]; ok {
			order = append(order, "system_root")
		}
		for _, r := range refs {
			if r != "system_root" && !named[r] {
				order = append(order, r)
			}
		}
		order = append(order, refs...)
		for _, r := range order {
			w.descend(r, m.Name)
		}
	} else if diagram != nil {
		m.Layout = SimulinkLayoutInline
		w := slxWalk{m: m, part: "simulink/blockdiagram.xml"}
		for _, s := range diagram.children("System") {
			w.system(s, m.Name)
		}
	} else {
		m.Layout = SimulinkLayoutSystems
	}
	m.slxModelRefs(parts, diagram)
	return nil
}

// slxWalk walks systems, splicing a <System Ref> stub's part in place.
type slxWalk struct {
	m       *SimulinkModel
	systems map[string]*xnode
	seen    map[string]bool
	part    string
}

func (w *slxWalk) descend(ref, parent string) {
	if w.seen[ref] || w.systems[ref] == nil {
		return
	}
	w.seen[ref] = true
	saved := w.part
	w.part = "simulink/systems/" + ref + ".xml"
	w.system(w.systems[ref], parent)
	w.part = saved
}

func (w *slxWalk) system(s *xnode, parent string) {
	for _, b := range s.children("Block") {
		name := simulinkName(b.attr("Name"))
		p := simulinkJoin(parent, name)
		inner := b.children("System")
		w.m.Blocks = append(w.m.Blocks, SimulinkBlock{
			Path: p, Parent: parent, Name: name, SID: b.attr("SID"), BlockType: b.attr("BlockType"),
			Subsystem: len(inner) > 0, Part: w.part, Line: b.line, EndLine: b.to,
		})
		for _, in := range inner {
			if r := in.attr("Ref"); r != "" {
				w.descend(r, p)
			} else {
				w.system(in, p)
			}
		}
	}
	for _, l := range s.children("Line") {
		e := simulinkEnds{system: parent, src: sidEnd(l.prop("Src"))}
		if v := l.prop("SrcBlock"); v != "" {
			e.src = simulinkEnd{name: v}
		}
		e.dsts = xmlDsts(l, e.dsts)
		w.m.ends = append(w.m.ends, e)
	}
}

// xmlDsts appends a line's or branch's destinations: its own Dst, then each branch's.
func xmlDsts(n *xnode, out []simulinkEnd) []simulinkEnd {
	for _, k := range n.kids {
		switch {
		case k.name == "P" && k.attr("Name") == "Dst":
			out = append(out, sidEnd(k.text))
		case k.name == "P" && k.attr("Name") == "DstBlock":
			out = append(out, simulinkEnd{name: k.text})
		case k.name == "Branch":
			out = xmlDsts(k, out)
		}
	}
	return out
}

// slxModelRefs reads the model references: graphicalInterface.json, else its .xml, else the
// diagram's inline GraphicalInterface.
func (m *SimulinkModel) slxModelRefs(parts map[string][]byte, diagram *xnode) {
	if b, ok := parts["simulink/graphicalInterface.json"]; ok {
		var doc map[string]json.RawMessage
		if json.Unmarshal(b, &doc) != nil {
			return
		}
		if inner, ok := doc["GraphicalInterface"]; ok {
			doc = nil
			if json.Unmarshal(inner, &doc) != nil {
				return
			}
		}
		raw := bytes.TrimSpace(doc["ModelReferences"])
		type ref struct{ BlockPath, ModelName string }
		var list []ref
		if bytes.HasPrefix(raw, []byte("{")) {
			var one ref
			if json.Unmarshal(raw, &one) == nil {
				list = append(list, one)
			}
		} else if len(raw) > 0 {
			_ = json.Unmarshal(raw, &list)
		}
		for _, r := range list {
			if r.BlockPath != "" && r.ModelName != "" {
				m.ModelRefs = append(m.ModelRefs, SimulinkModelRef{BlockPath: m.refPath(r.BlockPath), ModelName: r.ModelName})
			}
		}
		return
	}
	var gi *xnode
	if b, ok := parts["simulink/graphicalInterface.xml"]; ok {
		if doc, err := parseXMLPart(b); err == nil {
			gi = doc
		}
	} else if diagram != nil {
		gi = &xnode{kids: diagram.children("GraphicalInterface")}
	}
	if gi == nil {
		return
	}
	for _, r := range gi.all("ModelReference", nil) {
		m.addPackedRef(r.prop("ModelRefBlockPath"))
	}
}

// addPackedRef reads `$bdroot/Child|model`, split at the last `|`.
func (m *SimulinkModel) addPackedRef(packed string) {
	cut := strings.LastIndex(packed, "|")
	if cut <= 0 || cut == len(packed)-1 {
		return
	}
	m.ModelRefs = append(m.ModelRefs, SimulinkModelRef{BlockPath: m.refPath(packed[:cut]), ModelName: packed[cut+1:]})
}

// --- .mdl ---------------------------------------------------------------------------

// mnode is one brace node of a .mdl: its properties (first value per key) and children.
type mnode struct {
	name     string
	props    map[string]string
	kids     []*mnode
	line, to int
}

func (n *mnode) children(name string) []*mnode {
	var out []*mnode
	for _, k := range n.kids {
		if k.name == name {
			out = append(out, k)
		}
	}
	return out
}

// mdlUnquote reads one quoted string at the start of s: its value and the rest of s.
func mdlUnquote(s string) (string, string) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(s[i])
			}
		case c == '"':
			return b.String(), s[i+1:]
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), ""
}

// parseMDL reads the brace tree. A property's value is the rest of its line, or the
// concatenation of its quoted strings, a continuation line opening with `"` included.
func parseMDL(src []byte) *mnode {
	root := &mnode{}
	stack := []*mnode{root}
	var lastKey string
	var last *mnode
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 64<<10), MaxSimulinkLine)
	for line := 1; sc.Scan(); line++ {
		t := strings.TrimSpace(sc.Text())
		top := stack[len(stack)-1]
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
			lastKey = ""
		case t == "}":
			if len(stack) > 1 {
				top.to = line
				stack = stack[:len(stack)-1]
			}
			lastKey = ""
		case mdlOpens(t):
			n := &mnode{name: strings.TrimSpace(strings.TrimSuffix(t, "{")), line: line, to: line}
			top.kids = append(top.kids, n)
			stack = append(stack, n)
			lastKey = ""
		case strings.HasPrefix(t, "\""):
			if lastKey != "" && last == top {
				v, _ := mdlQuoted(t)
				top.props[lastKey] += v
			}
		default:
			key, val := t, ""
			if i := strings.IndexAny(t, " \t"); i >= 0 {
				key, val = t[:i], strings.TrimSpace(t[i:])
			}
			if strings.HasPrefix(val, "\"") {
				val, _ = mdlQuoted(val)
			}
			if top.props == nil {
				top.props = map[string]string{}
			}
			if _, dup := top.props[key]; !dup {
				top.props[key] = val
				lastKey, last = key, top
			} else {
				lastKey = ""
			}
		}
	}
	return root
}

// mdlOpens reports whether a trimmed line opens a node: `Name {`.
func mdlOpens(t string) bool {
	name, ok := strings.CutSuffix(t, "{")
	name = strings.TrimSpace(name)
	return ok && name != "" && !strings.ContainsAny(name, " \t\"[")
}

// MaxSimulinkLine is the longest .mdl line the reader reads (a longer one ends the read).
const MaxSimulinkLine = 4 << 20

// mdlQuoted concatenates the quoted strings of a value written `"a" "b"`.
func mdlQuoted(s string) (string, string) {
	var out strings.Builder
	for strings.HasPrefix(s, "\"") {
		v, rest := mdlUnquote(s)
		out.WriteString(v)
		s = strings.TrimSpace(rest)
	}
	return out.String(), s
}

func (m *SimulinkModel) readMDL(src []byte) error {
	head := src[:min(len(src), 4096)]
	if bytes.Contains(head, []byte("# MathWorks OPC Text Package")) || bytes.Contains(head, []byte("# Simulink Text Package")) ||
		bytes.Contains(src, []byte("__MWOPC_PACKAGE_BEGIN__")) || bytes.Contains(src, []byte("__MWSL_PART_BEGIN__")) {
		return ErrSimulinkTextPackage
	}
	m.Layout = SimulinkLayoutMDL
	root := parseMDL(src)
	var model *mnode
	for _, k := range root.kids {
		if (k.name == "Model" || k.name == "Library") && model == nil {
			model = k
		}
	}
	if model == nil {
		return nil
	}
	var walk func(s *mnode, parent string)
	walk = func(s *mnode, parent string) {
		for _, b := range s.children("Block") {
			name := simulinkName(b.props["Name"])
			p := simulinkJoin(parent, name)
			inner := b.children("System")
			m.Blocks = append(m.Blocks, SimulinkBlock{
				Path: p, Parent: parent, Name: name, SID: b.props["SID"], BlockType: b.props["BlockType"],
				Subsystem: len(inner) > 0, Line: b.line, EndLine: b.to,
			})
			for _, in := range inner {
				walk(in, p)
			}
		}
		for _, l := range s.children("Line") {
			e := simulinkEnds{system: parent, src: sidEnd(l.props["Src"])}
			if v := l.props["SrcBlock"]; v != "" {
				e.src = simulinkEnd{name: v}
			}
			e.dsts = mdlDsts(l, e.dsts)
			m.ends = append(m.ends, e)
		}
	}
	for _, s := range model.children("System") {
		walk(s, m.Name)
	}
	for _, gi := range model.children("GraphicalInterface") {
		for _, r := range gi.children("ModelReference") {
			m.addPackedRef(r.props["ModelRefBlockPath"])
		}
	}
	return nil
}

// mdlDsts appends a line's or branch's destinations: its own, then each branch's.
func mdlDsts(n *mnode, out []simulinkEnd) []simulinkEnd {
	if v := n.props["Dst"]; v != "" {
		out = append(out, sidEnd(v))
	} else if v := n.props["DstBlock"]; v != "" {
		out = append(out, simulinkEnd{name: v})
	}
	for _, b := range n.children("Branch") {
		out = mdlDsts(b, out)
	}
	return out
}

// --- the extractor ------------------------------------------------------------------

// SimulinkExtractor reads Simulink models into symbols: the model, then each block by its
// full path, a subsystem as SimulinkSubsystem, its block type as the signature.
type SimulinkExtractor struct{}

var _ Extractor = (*SimulinkExtractor)(nil)

// NewSimulinkExtractor returns the Simulink model extractor.
func NewSimulinkExtractor() *SimulinkExtractor { return &SimulinkExtractor{} }

// SimulinkModelRefEdge is a model reference block's edge to the model it names.
const SimulinkModelRefEdge = "model_ref"

var _ DocResulter = (*SimulinkExtractor)(nil)

// Extract implements Extractor. A model the reader refuses (a cap, a corrupt package, a
// text-package .mdl) yields no symbols and no error, as the doc engine reads malformed input.
func (x SimulinkExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter: the symbols, the model as the file's root, and each
// model reference as a module-source reference by the model's name, which the doc-xref
// pass joins to the one model of that name in another file (none when several share it).
func (SimulinkExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	m, err := ReadSimulinkModel(relPath, src)
	if err != nil {
		return DocResult{}, nil
	}
	pkg := pkgFromPath(relPath)
	calls := map[string][]string{}
	for _, s := range m.Signals {
		calls[s.Src] = append(calls[s.Src], s.Dst)
	}
	res := DocResult{Roots: []string{m.Name}}
	res.Nodes = make([]core.SymbolNode, 0, len(m.Blocks)+1)
	res.Nodes = append(res.Nodes, core.SymbolNode{Service: service, Package: pkg, Name: m.Name, Kind: KindSimulinkModel,
		FilePath: relPath, Line: 1, EndLine: 1, Tier: core.TierPreScan})
	lines := map[string]int{}
	for _, b := range m.Blocks {
		kind := KindSimulinkBlock
		if b.Subsystem {
			kind = KindSimulinkSubsystem
		}
		res.Nodes = append(res.Nodes, core.SymbolNode{Service: service, Package: pkg, Name: b.Path, Kind: kind,
			FilePath: relPath, Line: b.Line, EndLine: b.EndLine, Signature: capString(b.BlockType, 200), Tier: core.TierPreScan,
			Calls: uniqueSorted(calls[b.Path])})
		lines[b.Path] = b.Line
	}
	for _, r := range m.ModelRefs {
		line := lines[r.BlockPath]
		if line == 0 {
			line = 1
		}
		res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: r.BlockPath, Edge: SimulinkModelRefEdge,
			Raw: r.ModelName, Target: r.ModelName, Strategy: "module-source", Line: line})
	}
	return res, nil
}

// uniqueSorted is s sorted without repeats (nil for none).
func uniqueSorted(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := append([]string(nil), s...)
	sort.Strings(out)
	return slices.Compact(out)
}

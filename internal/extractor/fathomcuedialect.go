package extractor

// fathomcuedialect.go — fathom's own CUE configuration formats as native
// dialects (LB-Q-fathom-concerns, LB-Q-fathom-layer-packs, engine "native":
// cuelang.org/go's parser and evaluator, the ones the CUE extractor already
// reads with). A project's .fathom/concerns.cue, a layer pack's layer.cue and a
// rule pack under .fathom/rules/ are plain CUE with a contract schema/ states;
// this extractor COMPOSES NewCUEExtractor() (every CUE node and reference edge
// the CUE extraction gives, unchanged) and adds the entities the contract names.
//
// Entities (dialects/fathom-concerns, dialects/fathom-layer-packs). Names are
// slash-separated so they never collide with a CUE field's dotted name:
//
//   - concern/<id> (FathomConcern): one per member of a top-level `concerns:`
//     struct, including a member a comprehension generates, which only
//     evaluation names. A written member spans its own field; a generated one
//     spans the dynamic `(id): {...}` field that CUE's evaluation positions it
//     at. The signature is the concern's `want` when that is a concrete string;
//   - layer/<name> (FathomLayer): a layer.cue's pack, named by its `name` value
//     and spanning that field; the signature is its `kind` and `version`;
//   - ruleset/<stem> (FathomRuleSet): a file under .fathom/rules/ that declares
//     `rules:`, named by the file's stem and spanning the first `rules` field;
//     the signature counts the rules when the file evaluates.
//
// References are the CUE extraction's own (a field to the hidden table or
// definition its value names); this extractor adds none. A file that does not
// evaluate on its own (an unresolved import, a syntax error) is read from its
// syntax alone: the labels and literals it writes, never a guess at the rest.

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	// KindFathomConcern, KindFathomLayer and KindFathomRuleSet are the
	// entities of fathom's CUE configuration dialects. No connector
	// classification uses them, so no constant in internal/core (the PlanRow
	// precedent).
	KindFathomConcern core.SourceKind = "FathomConcern"
	KindFathomLayer   core.SourceKind = "FathomLayer"
	KindFathomRuleSet core.SourceKind = "FathomRuleSet"
)

// FathomCUEExtractor implements Extractor and DocResulter for the native
// fathom-concerns and fathom-layer-packs dialects, over the CUE extraction.
type FathomCUEExtractor struct {
	id   string
	base *CUEExtractor
}

var (
	_ Extractor   = (*FathomCUEExtractor)(nil)
	_ DocResulter = (*FathomCUEExtractor)(nil)
)

// NewFathomCUEExtractor returns the extractor of dialect id, "fathom-concerns"
// or "fathom-layer-packs".
func NewFathomCUEExtractor(id string) *FathomCUEExtractor {
	return &FathomCUEExtractor{id: id, base: NewCUEExtractor()}
}

// FathomCUEDialectExtractor is the extractor of the native dialect a fathom CUE
// format defines, by dialect id; ok is false for any other id.
func FathomCUEDialectExtractor(id string) (Extractor, bool) {
	if id == "fathom-concerns" || id == "fathom-layer-packs" {
		return NewFathomCUEExtractor(id), true
	}
	return nil, false
}

// Extract implements Extractor.
func (x *FathomCUEExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter: the CUE extraction, then this dialect's
// entities.
func (x *FathomCUEExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	var res DocResult
	var baseErr error
	if d, ok := any(x.base).(DocResulter); ok {
		res, baseErr = d.ExtractDoc(service, relPath, src)
	} else {
		res.Nodes, baseErr = x.base.Extract(service, relPath, src)
	}
	f, _ := parser.ParseFile(relPath, src)
	if f == nil {
		return res, baseErr
	}
	pkg := f.PackageName()
	if pkg == "" {
		pkg = pkgFromPath(relPath)
	}
	w := &fathomCUEWalk{service: service, pkg: pkg, relPath: relPath, f: f, val: evalCUEFile(f)}
	switch x.id {
	case "fathom-concerns":
		w.concerns()
	case "fathom-layer-packs":
		if path.Base(relPath) == "layer.cue" {
			w.layer()
		} else {
			w.ruleSet()
		}
	}
	res.Nodes = append(res.Nodes, w.nodes...)
	sort.SliceStable(res.Nodes, func(i, j int) bool {
		if res.Nodes[i].Line != res.Nodes[j].Line {
			return res.Nodes[i].Line < res.Nodes[j].Line
		}
		return res.Nodes[i].Name < res.Nodes[j].Name
	})
	return res, baseErr
}

// evalCUEFile evaluates one file on its own. The zero Value (no evaluation)
// answers a file that does not build, or that makes the evaluator panic.
func evalCUEFile(f *ast.File) (v cue.Value) {
	defer func() {
		if recover() != nil {
			v = cue.Value{}
		}
	}()
	v = cuecontext.New().BuildFile(f)
	if v.Err() != nil {
		return cue.Value{}
	}
	return v
}

type fathomCUEWalk struct {
	service, pkg, relPath string
	f                     *ast.File
	val                   cue.Value
	nodes                 []core.SymbolNode
}

func (w *fathomCUEWalk) add(name string, kind core.SourceKind, start, end int, sig string) {
	if end < start {
		end = start
	}
	w.nodes = append(w.nodes, core.SymbolNode{
		Service: w.service, Package: w.pkg, Name: name, Kind: kind,
		FilePath: w.relPath, Line: start, EndLine: end,
		Signature: capString(sig, 200), Tier: core.TierVerified,
	})
}

// topFields are the file's top-level fields named name, in source order.
func (w *fathomCUEWalk) topFields(name string) []*ast.Field {
	var out []*ast.Field
	for _, d := range w.f.Decls {
		if fd, ok := d.(*ast.Field); ok {
			if n, _, err := ast.LabelName(fd.Label); err == nil && n == name {
				out = append(out, fd)
			}
		}
	}
	return out
}

// concernDecls are the declarations of the members of every `concerns:`
// struct: those written with a static label by id (the first of an id wins),
// and the dynamic ones, a comprehension's `(id): {...}`, by their start line.
func (w *fathomCUEWalk) concernDecls() (static map[string]*ast.Field, dynamic map[int]*ast.Field) {
	static, dynamic = map[string]*ast.Field{}, map[int]*ast.Field{}
	var walk func(elts []ast.Decl)
	walk = func(elts []ast.Decl) {
		for _, e := range elts {
			switch d := e.(type) {
			case *ast.Field:
				if n, _, err := ast.LabelName(d.Label); err == nil && n != "" {
					if static[n] == nil {
						static[n] = d
					}
				} else {
					dynamic[d.Pos().Line()] = d
				}
			case *ast.Comprehension:
				if s, ok := d.Value.(*ast.StructLit); ok {
					walk(s.Elts)
				}
			}
		}
	}
	for _, fd := range w.topFields("concerns") {
		if s, ok := fd.Value.(*ast.StructLit); ok {
			walk(s.Elts)
		}
	}
	return static, dynamic
}

func (w *fathomCUEWalk) concerns() {
	static, dynamic := w.concernDecls()
	seen := map[string]bool{}
	sig := func(id string) string {
		if w.val.Exists() {
			if s, err := w.val.LookupPath(cue.MakePath(cue.Str("concerns"), cue.Str(id), cue.Str("want"))).String(); err == nil {
				return s
			}
		}
		return staticString(static[id], "want")
	}
	if w.val.Exists() {
		it, err := w.val.LookupPath(cue.ParsePath("concerns")).Fields(cue.Hidden(false))
		if err == nil {
			for it.Next() {
				id := it.Selector().Unquoted()
				if seen[id] {
					continue
				}
				seen[id] = true
				if fd := static[id]; fd != nil {
					w.add("concern/"+id, KindFathomConcern, fd.Pos().Line(), fieldEndLine(fd), sig(id))
					continue
				}
				line := it.Value().Pos().Line()
				end := line
				if fd := dynamic[line]; fd != nil {
					end = fieldEndLine(fd)
				}
				w.add("concern/"+id, KindFathomConcern, line, end, sig(id))
			}
			return
		}
	}
	for id, fd := range static {
		w.add("concern/"+id, KindFathomConcern, fd.Pos().Line(), fieldEndLine(fd), sig(id))
	}
}

func (w *fathomCUEWalk) layer() {
	fds := w.topFields("name")
	if len(fds) == 0 {
		return
	}
	name := w.stringAt("name")
	if name == "" {
		name = staticString(&ast.Field{Value: &ast.StructLit{Elts: []ast.Decl{fds[0]}}}, "name")
	}
	if name == "" {
		return
	}
	sig := strings.TrimSpace(w.stringAt("kind") + " " + w.stringAt("version"))
	w.add("layer/"+name, KindFathomLayer, fds[0].Pos().Line(), fieldEndLine(fds[0]), sig)
}

func (w *fathomCUEWalk) ruleSet() {
	fds := w.topFields("rules")
	if len(fds) == 0 {
		return
	}
	stem := strings.TrimSuffix(path.Base(w.relPath), ".cue")
	sig := ""
	if w.val.Exists() {
		if it, err := w.val.LookupPath(cue.ParsePath("rules")).List(); err == nil {
			n := 0
			for it.Next() {
				n++
			}
			sig = strconv.Itoa(n) + " rules"
		}
	}
	w.add("ruleset/"+stem, KindFathomRuleSet, fds[0].Pos().Line(), fieldEndLine(fds[0]), sig)
}

// stringAt is the evaluated string at a top-level field, else the written
// literal; "" when neither is a string.
func (w *fathomCUEWalk) stringAt(name string) string {
	if w.val.Exists() {
		if s, err := w.val.LookupPath(cue.ParsePath(name)).String(); err == nil {
			return s
		}
	}
	for _, fd := range w.topFields(name) {
		if s := litString(fd.Value); s != "" {
			return s
		}
	}
	return ""
}

// staticString is a string literal member of fd's struct value, "" otherwise.
func staticString(fd *ast.Field, key string) string {
	if fd == nil {
		return ""
	}
	s, ok := fd.Value.(*ast.StructLit)
	if !ok {
		return ""
	}
	for _, e := range s.Elts {
		if m, ok := e.(*ast.Field); ok {
			if n, _, err := ast.LabelName(m.Label); err == nil && n == key {
				return litString(m.Value)
			}
		}
	}
	return ""
}

func litString(e ast.Expr) string {
	if b, ok := e.(*ast.BasicLit); ok {
		if u, err := literal.Unquote(b.Value); err == nil {
			return u
		}
	}
	return ""
}

// fieldEndLine is the last line of a field's value (a Field's own End runs to
// the token after it, which may sit on the next line).
func fieldEndLine(fd *ast.Field) int {
	if fd.Value != nil {
		return fd.Value.End().Line()
	}
	return fd.End().Line()
}

package langbench

// scip_rust.go — LB-F249: rust-analyzer's macro-generated definitions, for L4. The index
// expands macros, so it defines items no source line writes: ripgrep's `rgtest!(r16, ..)`
// defines `fn r16`, globset's `syntax!(literal1, ..)` a test function, `lazy_static!` a
// struct. The definition's range sits on the macro invocation, with no declaration keyword
// before the name; no syntactic extractor can find it, and the references written inside
// its body have no function symbol of fathom's to come from. Precedent: LB-F116's
// clangMacroGenerated (a definition a macro writes) and LB-F55 and LB-F239 (a reference to
// a local or a parameter is out of typed_recall's denominator).
//
// The rule is Rust-only (SCIPOptions.RustMacroDefs, on in SCIPOptionsFor("rust") with a
// source reader) and counted twice, never silent: a definition is counted under
// CntScipDefsMacroGenerated instead of CntScipDefs, and a reference whose innermost
// definition is one is counted under CntScipEdgesMacroGenerated instead of CntScipEdges.
// A file module (`mod x;`, a whole-file Module definition) is not touched: whether it gets
// a symbol of its own or a reference to it is read at file granularity is the owner's
// question (docs/plans/language-breadth.md F.28), and nothing here answers it.

import (
	"bytes"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

const (
	// CntScipDefsMacroGenerated counts the SCIP definitions no source line writes
	// (rust-analyzer's macro expansion), left out of scip_defs and scip_defs_mapped.
	CntScipDefsMacroGenerated = "scip_defs_macro_generated"
	// CntScipEdgesMacroGenerated counts the references written inside such a definition,
	// left out of scip_edges (typed_recall's denominator): fathom has no symbol for the
	// function they belong to.
	CntScipEdgesMacroGenerated = "scip_edges_macro_generated"
)

// rustGeneratedKinds are the kinds whose definition is written with a declaration keyword
// (rustDeclSite), so one reported with none was written by a macro. A field, an enum
// variant, a parameter and a local carry no keyword and are never judged; a Module is the
// owner's question.
var rustGeneratedKinds = map[scippb.SymbolInformation_Kind]bool{
	scippb.SymbolInformation_Function:       true,
	scippb.SymbolInformation_Method:         true,
	scippb.SymbolInformation_StaticMethod:   true,
	scippb.SymbolInformation_TraitMethod:    true,
	scippb.SymbolInformation_Struct:         true,
	scippb.SymbolInformation_Enum:           true,
	scippb.SymbolInformation_Union:          true,
	scippb.SymbolInformation_Trait:          true,
	scippb.SymbolInformation_TypeAlias:      true,
	scippb.SymbolInformation_Constant:       true,
	scippb.SymbolInformation_StaticVariable: true,
}

// rustGeneratedDef reports whether a definition of the given kind at range r (0-based line
// and columns) is written by no source line: its name is on one line and the text before it
// ends in no Rust declaration keyword.
func rustGeneratedDef(kind scippb.SymbolInformation_Kind, lines [][]byte, r [4]int32) bool {
	if !rustGeneratedKinds[kind] || r[0] != r[2] || r[0] < 0 || int(r[0]) >= len(lines) {
		return false
	}
	line := lines[r[0]]
	if r[1] < 0 || r[1] > r[3] || int(r[3]) > len(line) {
		return false
	}
	return !rustDeclSite(string(line[:r[1]]))
}

// rustDocKinds is the SymbolInformation kind of each symbol a document defines.
func rustDocKinds(d *scippb.Document) map[string]scippb.SymbolInformation_Kind {
	kinds := make(map[string]scippb.SymbolInformation_Kind, len(d.GetSymbols()))
	for _, si := range d.GetSymbols() {
		kinds[si.GetSymbol()] = si.GetKind()
	}
	return kinds
}

// rustWithoutGenerated is the index with every occurrence inside a macro-generated
// definition's enclosing_range removed (the definition's own included), so
// internal/scip's import counts the edges fathom could attribute and no others. It returns
// the index as bytes and whether any definition was generated. A definition with no
// enclosing_range cannot be cut out and stays.
func rustWithoutGenerated(idx *scippb.Index, src func(path string) []byte) ([]byte, bool, error) {
	changed := false
	out := &scippb.Index{Metadata: idx.GetMetadata(), ExternalSymbols: idx.GetExternalSymbols()}
	for _, d := range idx.GetDocuments() {
		kinds := rustDocKinds(d)
		var lines [][]byte
		var cuts [][4]int32
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			r, ok := rangeOf(o.GetRange())
			enc, hasEnc := rangeOf(o.GetEnclosingRange())
			if !ok || !hasEnc {
				continue
			}
			if lines == nil {
				lines = bytes.Split(src(d.GetRelativePath()), []byte("\n"))
			}
			if rustGeneratedDef(kinds[o.GetSymbol()], lines, r) {
				cuts = append(cuts, enc)
			}
		}
		if len(cuts) == 0 {
			out.Documents = append(out.Documents, d)
			continue
		}
		changed = true
		nd := proto.Clone(d).(*scippb.Document)
		nd.Occurrences = nd.Occurrences[:0]
		for _, o := range d.GetOccurrences() {
			if r, ok := rangeOf(o.GetRange()); ok && inAny(cuts, r) {
				continue
			}
			nd.Occurrences = append(nd.Occurrences, o)
		}
		out.Documents = append(out.Documents, nd)
	}
	if !changed {
		return nil, false, nil
	}
	raw, err := proto.Marshal(out)
	return raw, true, err
}

func inAny(outers [][4]int32, inner [4]int32) bool {
	for _, o := range outers {
		if contains(o, inner) {
			return true
		}
	}
	return false
}

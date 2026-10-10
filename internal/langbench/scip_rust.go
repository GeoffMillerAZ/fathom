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
// question (docs/plans/language-breadth.md F.30), and nothing here answers it.

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
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
// internal/scip's import counts the edges fathom could attribute and no others. It
// reports whether any definition was generated. A definition with no enclosing_range
// cannot be cut out and stays.
func rustWithoutGenerated(idx *scippb.Index, src func(path string) []byte) (*scippb.Index, bool) {
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
	return out, changed
}

// CntScipEdgesNamespaceFrom counts the references made at a Rust module's top level (a
// `use` line, an impl header: no definition but the module encloses them), left out of
// typed_recall with the namespace (K18, LB-F389). The references to a module are
// CntScipEdgesNamespaceTarget, the definitions CntScipDefsNamespace (Go's LB-F344 keys).
const CntScipEdgesNamespaceFrom = "scip_edges_namespace_from"

// rustNamespaceNote is the reading's disclosure of K18's Rust half for a subject, "" when
// the rule moved nothing.
func rustNamespaceNote(name string, counts map[string]int) string {
	d, to, from := counts[CntScipDefsNamespace], counts[CntScipEdgesNamespaceTarget], counts[CntScipEdgesNamespaceFrom]
	if d+to+from == 0 {
		return ""
	}
	return fmt.Sprintf("%s: a Rust module is a namespace, file granularity, never a fathom symbol (K18, LB-F389): %d module definitions (a file module's whole file, a crate root, an inline mod) are out of def_mapping (%s), %d references to a module (%s) and %d made at a module's top level, a use line or an impl header (%s), out of typed_recall; module references are graded at L3 as file edges", name, d, CntScipDefsNamespace, to, CntScipEdgesNamespaceTarget, from, CntScipEdgesNamespaceFrom)
}

// rustNamespace reports whether sym is a module rust-analyzer defines: a namespace
// descriptor (`a/b/`, `crate/`).
func rustNamespace(sym string) bool {
	return isRustAnalyzer(sym) && strings.HasSuffix(sym, "/")
}

// rustWithoutNamespaces is the index less every reference to a module (K18: no fathom
// symbol is one) or, with defs, less every module definition too, so that a reference
// whose only enclosing definition was a module (made at its top level) has none. It
// reports whether anything was removed.
func rustWithoutNamespaces(idx *scippb.Index, defs bool) (*scippb.Index, bool) {
	changed := false
	out := &scippb.Index{Metadata: idx.GetMetadata(), ExternalSymbols: idx.GetExternalSymbols()}
	for _, d := range idx.GetDocuments() {
		var nd *scippb.Document
		for i, o := range d.GetOccurrences() {
			isDef := o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0
			if !rustNamespace(o.GetSymbol()) || isDef != defs {
				if nd != nil {
					nd.Occurrences = append(nd.Occurrences, o)
				}
				continue
			}
			if nd == nil {
				nd = proto.Clone(d).(*scippb.Document)
				nd.Occurrences = append(nd.Occurrences[:0], d.GetOccurrences()[:i]...)
			}
		}
		if nd == nil {
			out.Documents = append(out.Documents, d)
			continue
		}
		changed = true
		out.Documents = append(out.Documents, nd)
	}
	return out, changed
}

// rustRecount re-imports a cut index and moves the edges it lost from scip_edges (and
// the typed ones from scip_edges_typed) to the count key.
func rustRecount(service string, cut *scippb.Index, nodes []core.SymbolNode, counts map[string]int, key string) error {
	raw, err := proto.Marshal(cut)
	if err != nil {
		return err
	}
	imp, err := fscip.ImportIndex(service, raw, nodes)
	if err != nil {
		return err
	}
	kept := imp.Report.Direct + imp.Report.Skipped["unmapped-endpoint"]
	if n := counts[CntScipEdges] - kept; n > 0 {
		counts[key] += n
		counts[CntScipEdges] = kept
		counts[CntScipEdgesTyped] = imp.Report.Direct
	}
	return nil
}

func inAny(outers [][4]int32, inner [4]int32) bool {
	for _, o := range outers {
		if contains(o, inner) {
			return true
		}
	}
	return false
}

// CntScipFileEdgesDuplicateNarrowed counts the file-edge truth pairs K25 ruling 3 (every
// defining document) would draw for a reference to a symbol rust-analyzer defines in more
// than one document that rustDuplicateDefiners leaves out (LB-F387).
const CntScipFileEdgesDuplicateNarrowed = "scip_file_edges_duplicate_narrowed"

// rustDuplicateDefiners is which of a symbol's defining documents a reference in doc
// means, for a symbol rust-analyzer defines in more than one document; nil (every one,
// K25 ruling 3) for any other producer. written is the source text at the reference (""
// when unread).
//
// rust-analyzer names a definition by its package and its path inside the crate, never by
// the crate target, and reports each collision as a "Duplicate symbol" itself: every
// target of one package shares `crate/` (clap's 66 test, example and library roots), two
// test targets' `utils` modules share `utils/` and its items, and the examples that each
// declare a `Cli` struct share `Cli#`. Each is a per-target copy (Rust merges no
// declarations across files), so:
//
//   - `crate/` written as a crate name (`use clap::Parser;` in a clap example or in
//     another package, never `crate`) is the package's library target: its definers named
//     lib.rs;
//   - a reference written inside a defining document is to that document's own copy: no
//     cross-file edge;
//   - any other item resolves to every defining document (ruling 3: tests/builder/
//     command.rs's `utils::FULL_TEMPLATE` is an edge to both utils.rs files);
//   - any other module, a namespace and file granularity (K18), to the definers in the
//     nearest directory above the referring document (a crate target's modules sit under
//     its root's directory, Cargo's layout: tests/builder/action.rs's `crate::` is
//     tests/builder/main.rs, not examples/git.rs), else, from outside every definer's
//     directory, the definers named lib.rs, else every one.
//
// Read every defining document instead and a reference to `crate::` in any clap test is
// an edge to all 66 roots: file_edge_recall .0583 on ripgrep and clap.
func rustDuplicateDefiners(sym, doc, written string, tos []string) []string {
	if len(tos) < 2 || !isRustAnalyzer(sym) {
		return nil
	}
	libs := func() []string {
		var out []string
		for _, t := range tos {
			if path.Base(t) == "lib.rs" {
				out = append(out, t)
			}
		}
		return out
	}
	if strings.HasSuffix(sym, " crate/") && written != "" && written != "crate" && written != "$crate" {
		if l := libs(); l != nil {
			return l
		}
	}
	if slices.Contains(tos, doc) {
		return []string{doc}
	}
	if !strings.HasSuffix(sym, "/") {
		return nil
	}
	docDir := relDir(doc)
	var near []string
	best := -1
	for _, t := range tos {
		td := relDir(t)
		if td != "" && td != docDir && !strings.HasPrefix(docDir+"/", td+"/") {
			continue
		}
		switch n := len(td); {
		case n > best:
			near, best = []string{t}, n
		case n == best:
			near = append(near, t)
		}
	}
	if near != nil {
		return near
	}
	if l := libs(); l != nil {
		return l
	}
	return tos
}

// relDir is a document's directory, "" at the subject's root.
func relDir(doc string) string {
	if d := path.Dir(doc); d != "." {
		return d
	}
	return ""
}

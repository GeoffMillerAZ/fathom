package langbench

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// scip_go.go — scip-go's file-edge rules (language-breadth F.34, wave lf1).

// CntScipFileEdgesMultiDef counts the file edges the index would draw only
// through a reference to a symbol it defines in more than one document: scip-go
// writes a package symbol at the package clause of every file of the package,
// so a reference to the package (an import spec, the `b` of `b.Foo`) names no
// one file. Left out of the file-edge truth and counted (LB-F298).
const CntScipFileEdgesMultiDef = "scip_file_edges_multidef"

// goMultiDefRefs collects the file edges drawn through multiply-defined
// symbols. The nil value (any producer but scip-go) holds nothing.
type goMultiDefRefs map[[2]string]bool

// newGoMultiDefRefs is the collector for a producer, nil unless scip-go.
func newGoMultiDefRefs(tool string) goMultiDefRefs {
	if tool != "scip-go" {
		return nil
	}
	return goMultiDefRefs{}
}

// holds reports whether the symbol, defined in the index, is defined in more
// than one document (defDoc records "" for such a symbol); a symbol the index
// does not define is not.
func (g goMultiDefRefs) holds(defDoc map[string]string, sym string) bool {
	if g == nil {
		return false
	}
	d, defined := defDoc[sym]
	return defined && d == ""
}

// finish counts the edges only a multiply-defined symbol drew.
func (g goMultiDefRefs) finish(fileEdges map[[2]string]bool, counts map[string]int) {
	n := 0
	for k := range g {
		if !fileEdges[k] {
			n++
		}
	}
	if n > 0 {
		counts[CntScipFileEdgesMultiDef] += n
	}
}

// goDefClasses counts scip-go's named definitions by descriptor class, and
// those fathom has no node for beside (LB-F299), so def_mapping's misses are
// classified in the reading. The nil value (any producer but scip-go) counts
// nothing.
type goDefClasses struct {
	kinds         map[string]scippb.SymbolInformation_Kind // the current document's symbol kinds
	defined, miss map[string]int
}

// newGoDefClasses is the counter for a producer, nil unless scip-go.
func newGoDefClasses(tool string) *goDefClasses {
	if tool != "scip-go" {
		return nil
	}
	return &goDefClasses{defined: map[string]int{}, miss: map[string]int{}}
}

// document reads a document's symbol kinds, which tell a field from an
// interface method (both a term of a type) and a constant from a variable.
func (g *goDefClasses) document(d *scippb.Document) {
	if g == nil {
		return
	}
	g.kinds = map[string]scippb.SymbolInformation_Kind{}
	for _, si := range d.GetSymbols() {
		g.kinds[si.GetSymbol()] = si.GetKind()
	}
}

// count notes one named definition and whether fathom has a node for it.
func (g *goDefClasses) count(sym string, mapped bool) {
	if g == nil {
		return
	}
	c := goDefClass(sym, g.kinds[sym])
	g.defined[c]++
	if !mapped {
		g.miss[c]++
	}
}

// finish writes the counts: scip_go_defs_<class> and
// scip_go_defs_unmapped_<class> (only a class with a miss).
func (g *goDefClasses) finish(counts map[string]int) {
	if g == nil {
		return
	}
	for c, n := range g.defined {
		counts["scip_go_defs_"+c] = n
	}
	for c, n := range g.miss {
		counts["scip_go_defs_unmapped_"+c] = n
	}
}

// goDefClass is the class of a scip-go definition's symbol: package (a
// namespace, defined at every file's package clause), type, func, method,
// field, interface-method (scip-go writes one as a term of its interface),
// package-var, package-const, or other.
func goDefClass(sym string, kind scippb.SymbolInformation_Kind) string {
	switch {
	case strings.HasSuffix(sym, "/"):
		return "package"
	case strings.HasSuffix(sym, "#"):
		return "type"
	case strings.HasSuffix(sym, ")."):
		if strings.Contains(sym[:strings.LastIndex(sym, "(")], "#") {
			return "method"
		}
		return "func"
	case strings.HasSuffix(sym, "."):
		body := strings.TrimSuffix(sym, ".")
		if strings.Contains(body[strings.LastIndexByte(body, '/')+1:], "#") {
			if kind == scippb.SymbolInformation_Method {
				return "interface-method"
			}
			return "field"
		}
		if kind == scippb.SymbolInformation_Constant {
			return "package-const"
		}
		return "package-var"
	}
	return "other"
}

// CntScipGoDocsOutside counts the documents of a scip-go index whose path
// leaves the subject, which are left out of every count (LB-F299).
const CntScipGoDocsOutside = "scip_go_docs_outside"

// goWithoutOutsideDocs drops the documents of a scip-go index whose path
// leaves the subject: `scip-go index ./...` also indexes the testmain
// `go test` generates into the Go build cache for each test package, named
// by a path outside the subject (`../../cache/ab/…-d`), an init() and a
// main() and the references that call into the package under test. They are
// no file the subject holds (design §1.4). It returns the index (and its
// encoding) unchanged for any other producer or when none leaves the subject,
// and counts the documents dropped.
func goWithoutOutsideDocs(idx *scippb.Index, raw []byte, counts map[string]int) (*scippb.Index, []byte, error) {
	if idx.GetMetadata().GetToolInfo().GetName() != "scip-go" {
		return idx, raw, nil
	}
	kept := idx.Documents[:0:0]
	for _, d := range idx.GetDocuments() {
		p := d.GetRelativePath()
		if strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
			continue
		}
		kept = append(kept, d)
	}
	dropped := len(idx.GetDocuments()) - len(kept)
	if dropped == 0 {
		return idx, raw, nil
	}
	counts[CntScipGoDocsOutside] = dropped
	cut := proto.Clone(idx).(*scippb.Index)
	cut.Documents = kept
	enc, err := proto.Marshal(cut)
	if err != nil {
		return nil, nil, err
	}
	return cut, enc, nil
}

// goSkipCounts writes the unmapped-endpoint references scip-go's import
// counted by class (scip.Report.SkippedByClass) as scip_go_unmapped_<side>_<class>
// counts: typed_recall's misses, classified (LB-F299).
func goSkipCounts(byClass map[string]int, counts map[string]int) {
	for k, n := range byClass {
		counts["scip_go_unmapped_"+strings.ReplaceAll(k, ":", "_")] = n
	}
}

// importedDirectEdges is the import's edges between mapped definitions.
func importedDirectEdges(imp fscip.Import) []core.Edge {
	var out []core.Edge
	for _, p := range imp.Packages {
		for _, e := range p.Edges {
			if e.Kind == core.EdgeDirect {
				out = append(out, e)
			}
		}
	}
	return out
}

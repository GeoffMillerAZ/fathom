package langbench

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// SCIPResult is one subject's tier-A measurement from a SCIP index.
type SCIPResult struct {
	Tool, Version string
	Counts        map[string]int
	// FileEdges are the index's cross-file edges at file granularity:
	// (referencing document, defining document), both in this index.
	FileEdges map[[2]string]bool
}

// scipBareName returns a SCIP symbol's leaf descriptor name and whether it
// is callable (a method descriptor, "name()."). Locals return "".
func scipBareName(sym string) (string, bool) {
	if sym == "" || strings.HasPrefix(sym, "local ") {
		return "", false
	}
	callable := strings.HasSuffix(sym, ").")
	s := sym
	// Drop the trailing descriptor suffix: "()." method, "." term, "#"
	// type, "/" namespace, ":" meta, "!" macro, "[x]" type parameter,
	// "(x)" parameter.
	switch {
	case strings.HasSuffix(s, ")."):
		s = s[:strings.LastIndex(s, "(")]
	case strings.HasSuffix(s, "]"), strings.HasSuffix(s, ")"):
		return "", false // type parameters and parameters are not definitions fathom maps
	default:
		s = s[:len(s)-1]
	}
	if strings.HasSuffix(s, "`") {
		if i := strings.LastIndex(s[:len(s)-1], "`"); i >= 0 {
			return s[i+1 : len(s)-1], callable
		}
	}
	i := strings.LastIndexAny(s, "/#.: ")
	return s[i+1:], callable
}

type scipDef struct {
	doc       string
	line      int // 1-based
	enc       [4]int32
	hasEnc    bool
	name      string
	fathomDef string // the mapped fathom node's bare name, "" when unmapped
}

func rangeOf(r []int32) ([4]int32, bool) {
	switch len(r) {
	case 3:
		return [4]int32{r[0], r[1], r[0], r[2]}, true
	case 4:
		return [4]int32{r[0], r[1], r[2], r[3]}, true
	}
	return [4]int32{}, false
}

func contains(outer, inner [4]int32) bool {
	before := func(l1, c1, l2, c2 int32) bool { return l1 < l2 || (l1 == l2 && c1 <= c2) }
	return before(outer[0], outer[1], inner[0], inner[1]) && before(inner[2], inner[3], outer[2], outer[3])
}

// ScoreSCIP measures nodes (fathom's extraction of the subject) against a
// SCIP index (tier A, §1.3): def_mapping (definitions matched to a fathom
// definition in the same file by name, |Δline| ≤ 2), call pairs (§1.4:
// callable references inside a mapped definition), typed_recall (the share
// of the index's reference edges fathom's SCIP import keeps as typed
// edges, internal/scip.ImportIndex), and the cross-file edge set.
func ScoreSCIP(raw []byte, service string, nodes []core.SymbolNode) (SCIPResult, error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return SCIPResult{}, fmt.Errorf("scip: decode index: %w", err)
	}
	res := SCIPResult{Counts: map[string]int{}, FileEdges: map[[2]string]bool{}}
	if ti := idx.GetMetadata().GetToolInfo(); ti != nil {
		res.Tool, res.Version = ti.GetName(), ti.GetVersion()
	}

	type fk struct{ path, name string }
	fathomAt := map[fk][]int{}
	for _, n := range nodes {
		k := fk{n.FilePath, BareName(n.Name)}
		fathomAt[k] = append(fathomAt[k], n.Line)
	}
	definedIn := map[string]string{} // symbol -> defining document
	defsByDoc := map[string][]*scipDef{}
	docs := map[string]bool{}
	for _, d := range idx.GetDocuments() {
		docs[d.GetRelativePath()] = true
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			name, _ := scipBareName(o.GetSymbol())
			r, ok := rangeOf(o.GetRange())
			if name == "" || !ok {
				continue
			}
			if _, seen := definedIn[o.GetSymbol()]; !seen {
				definedIn[o.GetSymbol()] = d.GetRelativePath()
			}
			def := &scipDef{doc: d.GetRelativePath(), line: int(r[0]) + 1, name: name}
			if enc, ok := rangeOf(o.GetEnclosingRange()); ok {
				def.enc, def.hasEnc = enc, true
			}
			res.Counts[CntScipDefs]++
			for _, line := range fathomAt[fk{def.doc, name}] {
				if d := line - def.line; d >= -2 && d <= 2 {
					def.fathomDef = name
					res.Counts[CntScipDefsMapped]++
					break
				}
			}
			defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
		}
	}

	truth := map[CallPair]bool{}
	for _, d := range idx.GetDocuments() {
		doc := d.GetRelativePath()
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0 {
				continue
			}
			r, ok := rangeOf(o.GetRange())
			if !ok {
				continue
			}
			if to, ok := definedIn[o.GetSymbol()]; ok && to != doc {
				res.FileEdges[[2]string{doc, to}] = true
			}
			callee, callable := scipBareName(o.GetSymbol())
			if !callable || callee == "" {
				continue
			}
			var inner *scipDef
			for _, def := range defsByDoc[doc] {
				if def.hasEnc && contains(def.enc, r) && (inner == nil || contains(inner.enc, def.enc)) {
					inner = def
				}
			}
			if inner == nil || inner.fathomDef == "" {
				continue
			}
			truth[CallPair{doc, inner.fathomDef, callee}] = true
		}
	}
	found := FathomCallPairs(nodes, docs)
	ScorePairs(truth, found, res.Counts, CntCallTruth, CntCallTruthFound, CntCallFathom, CntCallFathomInTrue)

	imp, err := fscip.ImportIndex(service, raw, nodes)
	if err != nil {
		return SCIPResult{}, err
	}
	res.Counts[CntScipEdgesTyped] = imp.Report.Direct
	res.Counts[CntScipEdges] = imp.Report.Direct + imp.Report.Skipped["unmapped-endpoint"]
	return res, nil
}

// ScoreFileEdges compares fathom's file-resolved edges (FromFile ->
// ToSymbol's file) against SCIP's cross-file edges at file granularity.
func ScoreFileEdges(truth map[[2]string]bool, edges []core.Edge, nodes []core.SymbolNode, counts map[string]int) {
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	mine := map[[2]string]bool{}
	for _, e := range edges {
		to, ok := fileOf[e.ToSymbol]
		if !ok || to == e.FromFile {
			continue
		}
		mine[[2]string{e.FromFile, to}] = true
	}
	keys := make([][2]string, 0, len(mine))
	for k := range mine {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]+keys[i][1] < keys[j][0]+keys[j][1] })
	counts[CntFileEdgesTruth] += len(truth)
	counts[CntFileEdgesFathom] += len(mine)
	for _, k := range keys {
		if truth[k] {
			counts[CntFileEdgesAgree]++
		}
	}
}

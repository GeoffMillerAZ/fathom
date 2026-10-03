package langbench

// imports.go — LB-F40: L3 import_resolution (design §1.2: "in-repo import
// specs resolved to a file ÷ in-repo specs", oracle: the files a SCIP
// index's references resolve to, or a language-native import dump).
//
// The unit is an import: the name one import statement binds, its
// alternative readings merged (the rust dialect reads `use a::b::c` both as
// a named import of c from a::b and as the whole module a::b::c, both bound
// to c — one import, resolved when either reading is). An oracle import
// reference (a SCIP occurrence, a dump entry) belongs to the statement
// whose lines hold it; the statement's oracle targets are the files of the
// subject those references resolve to. An import is in-repo when its
// statement has an oracle target, and agrees when fathom's strategy
// resolved it to one of those files.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Raw counts for import resolution, and its oracle family.
const (
	// CntImports is fathom's imports on the subject's files.
	CntImports = "imports"
	// CntImportsInRepo is the imports whose statement the oracle resolves
	// to a file of the subject: import_resolution's denominator.
	CntImportsInRepo = "imports_in_repo"
	// CntImportsAgree is the in-repo imports fathom resolved to a file the
	// oracle names: import_resolution's numerator.
	CntImportsAgree = "imports_resolved_agree"
	// CntImportsUnresolved is the in-repo imports fathom resolved to no file.
	CntImportsUnresolved = "imports_unresolved"
	// CntImportsElsewhere is the in-repo imports fathom resolved only to
	// files the oracle does not name.
	CntImportsElsewhere = "imports_resolved_elsewhere"
	// CntImportsUnattested is the imports fathom resolved to a file of the
	// subject where the oracle names none (an import the oracle resolves
	// outside the subject, or one it does not see).
	CntImportsUnattested = "imports_unattested"
	// Import edges are (importing file, statement, target file) triples:
	// precision and recall of what the statements reach, reported beside
	// the gated metric.
	CntImportEdgesTruth  = "import_edges_truth"
	CntImportEdgesFathom = "import_edges_fathom"
	CntImportEdgesAgree  = "import_edges_agree"
	// CntImportsExternal is the imports whose every reading names a crate
	// outside the subject (Rust's std, core, a registry dependency; LB-F197):
	// no file of the subject could be their target, so they are out of scope,
	// left out of imports and of the in-repo count, never unresolved.
	CntImportsExternal = "imports_external"
	// CntImportOracleUnpaired is the oracle's own imports (a dump entry, a
	// SCIP occurrence with the Import role) on no statement fathom
	// extracted.
	CntImportOracleUnpaired = "import_oracle_unpaired"
	// CntImportOracleOutOfScope is the oracle import targets outside the
	// files measured (a file ingest's gates or discovery leave out): no
	// strategy over fathom's universe can reach them (§1.4 Scope).
	CntImportOracleOutOfScope = "import_oracle_out_of_scope"
	// CntImportOracleAmbiguous is the oracle references on import
	// statements left out of the truth because their symbol is defined in
	// more than one file (which file is meant would be the index's guess),
	// one per statement line; recorded only when there are any.
	CntImportOracleAmbiguous = "import_oracle_ambiguous"

	FamilyImports = "imports"
)

// ImportSite is one import spec fathom extracted: its file, the line span
// of the statement that binds it, the name it binds and the files its
// strategy resolved it to (fusion.ResolveImportSpecs).
type ImportSite struct {
	Path               string
	StartLine, EndLine int
	Local              string
	Targets            []string
	// External marks a spec resolved to no file because its path is rooted
	// at a crate outside the subject (fusion.ResolvedImport.External).
	External bool
}

// OracleImport is one oracle import reference: the importing file, the
// line it is written on (1-based) and the file it resolves to. Explicit is
// true when the oracle itself calls it an import (a dump entry, a SCIP
// occurrence with the Import role); a plain reference counts only where a
// fathom import statement holds its line.
type OracleImport struct {
	Path     string
	Line     int
	ToPath   string
	Explicit bool
	// Ambiguous marks a reference whose symbol the oracle defines in more
	// than one file (ToPath is then ""): never truth, counted when an
	// import statement holds it.
	Ambiguous bool
}

// ScoreImports counts the oracle's import references against fathom's
// import sites into counts. inScope is the subject's measured files: a
// reference from any other file is not the subject's, and one to any other
// file is counted out of scope, never as fathom's miss.
func ScoreImports(truth []OracleImport, sites []ImportSite, inScope map[string]bool, counts map[string]int) {
	if len(truth) == 0 && len(sites) == 0 {
		return
	}
	type stmt struct {
		path     string
		lo, hi   int
		oracle   map[string]bool
		bindings map[string]map[string]bool // local name -> targets
		external map[string]bool            // local name -> every reading names an outside crate
	}
	byFile := map[string][]*stmt{}
	var files []string
	for _, s := range sites {
		if !inScope[s.Path] {
			continue
		}
		if _, ok := byFile[s.Path]; !ok {
			files = append(files, s.Path)
		}
		byFile[s.Path] = append(byFile[s.Path], &stmt{path: s.Path, lo: s.StartLine, hi: max(s.EndLine, s.StartLine)})
	}
	// Overlapping spans are one statement (a query whose match stops short
	// of the statement still overlaps the one that does not).
	for _, f := range files {
		ss := byFile[f]
		sort.SliceStable(ss, func(i, j int) bool { return ss[i].lo < ss[j].lo })
		var merged []*stmt
		for _, s := range ss {
			if n := len(merged); n > 0 && s.lo <= merged[n-1].hi {
				merged[n-1].hi = max(merged[n-1].hi, s.hi)
				continue
			}
			merged = append(merged, &stmt{path: f, lo: s.lo, hi: s.hi, oracle: map[string]bool{}, bindings: map[string]map[string]bool{}, external: map[string]bool{}})
		}
		byFile[f] = merged
	}
	statementAt := func(path string, line int) *stmt {
		ss := byFile[path]
		i := sort.Search(len(ss), func(i int) bool { return ss[i].hi >= line })
		if i < len(ss) && ss[i].lo <= line {
			return ss[i]
		}
		return nil
	}
	for _, s := range sites {
		if !inScope[s.Path] {
			continue
		}
		st := statementAt(s.Path, s.StartLine)
		b := st.bindings[s.Local]
		if b == nil {
			b = map[string]bool{}
			st.bindings[s.Local] = b
			st.external[s.Local] = true
		}
		if !s.External || len(s.Targets) > 0 {
			st.external[s.Local] = false
		}
		for _, t := range s.Targets {
			if t != s.Path && inScope[t] {
				b[t] = true
			}
		}
	}
	unpaired := map[OracleImport]bool{}
	outOfScope := map[OracleImport]bool{}
	ambiguous := map[OracleImport]bool{}
	for _, o := range truth {
		if !inScope[o.Path] || o.ToPath == o.Path || (o.ToPath == "" && !o.Ambiguous) {
			continue
		}
		st := statementAt(o.Path, o.Line)
		key := OracleImport{Path: o.Path, Line: o.Line, ToPath: o.ToPath}
		switch {
		case st == nil && !o.Explicit:
			// a reference outside every import statement: not an import
		case o.Ambiguous:
			ambiguous[key] = true
		case !inScope[o.ToPath]:
			outOfScope[key] = true
		case st == nil:
			unpaired[key] = true
		default:
			st.oracle[o.ToPath] = true
		}
	}

	for _, f := range files {
		for _, st := range byFile[f] {
			for local, targets := range st.bindings {
				if st.external[local] && len(targets) == 0 {
					counts[CntImportsExternal]++
					continue
				}
				counts[CntImports]++
				hit := false
				for t := range targets {
					hit = hit || st.oracle[t]
				}
				switch {
				case len(st.oracle) == 0:
					if len(targets) > 0 {
						counts[CntImportsUnattested]++
					}
				case hit:
					counts[CntImportsInRepo]++
					counts[CntImportsAgree]++
				case len(targets) == 0:
					counts[CntImportsInRepo]++
					counts[CntImportsUnresolved]++
				default:
					counts[CntImportsInRepo]++
					counts[CntImportsElsewhere]++
				}
			}
		}
	}
	// Edges: (file, statement, target) triples, each counted once.
	for _, f := range files {
		for _, st := range byFile[f] {
			reach := map[string]bool{}
			for _, targets := range st.bindings {
				for t := range targets {
					reach[t] = true
				}
			}
			counts[CntImportEdgesFathom] += len(reach)
			counts[CntImportEdgesTruth] += len(st.oracle)
			for t := range reach {
				if st.oracle[t] {
					counts[CntImportEdgesAgree]++
				}
			}
		}
	}
	counts[CntImportOracleUnpaired] += len(unpaired)
	counts[CntImportOracleOutOfScope] += len(outOfScope)
	if len(ambiguous) > 0 {
		counts[CntImportOracleAmbiguous] += len(ambiguous)
	}
}

// noteImportsPrefix opens a reading's import_resolution disclosure, which
// recompute rewrites from the summed counts on every merge.
const noteImportsPrefix = "import_resolution: "

// noteImportsNotMeasured is the pre-LB-F40 disclosure; a reading drops it
// once any subject measures the imports family.
const noteImportsNotMeasured = "import_resolution not measured: in-repo spec counts arrive with the L3 strategies (LB-F11)"

func importsNote(counts map[string]int) string {
	fr := func(num, den int) string {
		if v := ratio(num, den); v != nil {
			return strconv.FormatFloat(*v, 'f', -1, 64)
		}
		return "n/a"
	}
	var ex, good float64
	for _, t := range languageThresholds["L3"] {
		if t.metric == MImportResolution {
			ex, good = t.excellent, t.good
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s%d of %d in-repo imports resolve to a file the oracle names (%d unresolved, %d only to other files; L3 bar: excellent >= %g, good >= %g)",
		noteImportsPrefix, counts[CntImportsAgree], counts[CntImportsInRepo], counts[CntImportsUnresolved], counts[CntImportsElsewhere], ex, good)
	fmt.Fprintf(&b, "; import edges precision %s (%d of %d) and recall %s (%d of %d)",
		fr(counts[CntImportEdgesAgree], counts[CntImportEdgesFathom]), counts[CntImportEdgesAgree], counts[CntImportEdgesFathom],
		fr(counts[CntImportEdgesAgree], counts[CntImportEdgesTruth]), counts[CntImportEdgesAgree], counts[CntImportEdgesTruth])
	fmt.Fprintf(&b, "; %d of %d imports resolved where the oracle names no file of the subject; %d oracle imports on no statement fathom extracted; %d oracle import targets outside the files measured",
		counts[CntImportsUnattested], counts[CntImports], counts[CntImportOracleUnpaired], counts[CntImportOracleOutOfScope])
	if n := counts[CntImportsExternal]; n > 0 {
		fmt.Fprintf(&b, "; %d imports name only a crate outside the subject (std, core, a registry dependency) and are out of scope, left out of the count above (LB-F197)", n)
	}
	if n := counts[CntImportOracleAmbiguous]; n > 0 {
		fmt.Fprintf(&b, "; %d oracle references on import statements left out because their symbol is defined in more than one file", n)
	}
	return b.String()
}

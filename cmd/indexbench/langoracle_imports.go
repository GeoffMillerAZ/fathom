package main

// langoracle_imports.go — LB-F40 (design §1.2 import_resolution): a
// language dialect's imports, resolved by its manifest's strategy
// (fusion.ResolveImportSpecs, the file resolver's own module resolution),
// scored against an oracle's import references — a SCIP index's cross-file
// references on the import statements' lines (tier A), or a neutral oracle
// file's `imports` (a language-native import dump, at the file's tier).
// Go's imports are resolved by Go's own loader (LB-F58, langoracle_go.go).
// LB-F57: with no SCIP index, a dump's (path, to_path) pairs are also the
// truth file_edge_precision and file_edge_recall are scored against.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// importOracleEntry is one neutral-file import: the importing file, the
// line of the import statement, and the file of the subject it resolves
// to (module is the spec as written, for the reader; never scored).
//
// SamePackage marks a pair no import statement writes (LB-F433, K19): the
// file reaches a top-level name of its own package, which Kotlin lets it do
// with no import. It is file-edge truth only (scoreImportFileEdges), never an
// import statement import_resolution scores.
type importOracleEntry struct {
	Path        string `json:"path"`
	Line        int    `json:"line"`
	ToPath      string `json:"to_path"`
	Module      string `json:"module,omitempty"`
	SamePackage bool   `json:"same_package,omitempty"`
}

// importStatements is entries less the same-package pairs: the import
// statements an imports oracle scores.
func importStatements(entries []importOracleEntry) []importOracleEntry {
	out := make([]importOracleEntry, 0, len(entries))
	for _, e := range entries {
		if !e.SamePackage {
			out = append(out, e)
		}
	}
	return out
}

// importLang is the file resolver's registry key for a dialect whose
// manifest names an import strategy (design §6); ok is false otherwise.
func importLang(dialect string) (string, bool) {
	if lang, ok := fileResolveLang[dialect]; ok {
		return lang, true
	}
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		return "", false
	}
	for _, l := range langs {
		if l.Lang == dialect {
			return l.Lang, true
		}
	}
	return "", false
}

// noteIncludePath discloses the include-path strategy's counted outcomes
// (fusion.FileResolveReport).
const noteIncludePath = "%s: include-path resolution read %d compile database(s); %d quoted include(s) resolved only rooted at an ancestor directory of the includer; header-prototype binding bound %d call(s) (caller file and name) to the one file defining the function (%d of them chosen among definers by the files the build compiles), left %d unbound for two or more definers, %d for no definer among the files read and %d for definers the build does not compile"

// includePathLang reports whether lang's manifest names the include-path
// strategy.
func includePathLang(lang string) bool {
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		return false
	}
	for _, l := range langs {
		if l.Lang == lang {
			return l.Strategy == "include-path"
		}
	}
	return false
}

// subjectImports is one subject's resolved imports, computed on first use
// and shared by every imports oracle of the subject.
type subjectImports struct {
	ctx     context.Context
	dialect string
	root    string
	name    string
	files   []core.SourceFile
	nodes   []core.SymbolNode
	inScope map[string]bool

	done   bool
	ok     bool
	sites  []langbench.ImportSite
	notes  []string
	misses []unmatchedImport // the dump's imports fathom's statements bind no such file for (-unmatched)
}

// unmatchedImport is one oracle import (file, line, imported file) that no
// import statement of fathom covering that line binds to the imported file:
// Targets is what the statements on the line did bind (-unmatched, LB-F235).
type unmatchedImport struct {
	Path    string   `json:"path"`
	Line    int      `json:"line"`
	ToPath  string   `json:"to_path"`
	Module  string   `json:"module,omitempty"`
	Targets []string `json:"targets"`
}

// importMisses is the oracle imports whose imported file is a measured file
// that no fathom import statement spanning the oracle's line resolved to,
// sorted by file, line and imported file: the diagnosis an import_resolution
// or import-edge recall below its bar needs.
func importMisses(entries []importOracleEntry, sites []langbench.ImportSite, inScope map[string]bool) []unmatchedImport {
	out := []unmatchedImport{}
	for _, e := range entries {
		if e.ToPath == "" || e.ToPath == e.Path || !inScope[e.Path] || !inScope[e.ToPath] {
			continue
		}
		hit := false
		var targets []string
		for _, st := range sites {
			if st.Path != e.Path || e.Line < st.StartLine || e.Line > max(st.EndLine, st.StartLine) {
				continue
			}
			targets = append(targets, st.Targets...)
			hit = hit || slices.Contains(st.Targets, e.ToPath)
		}
		if hit {
			continue
		}
		sort.Strings(targets)
		out = append(out, unmatchedImport{Path: e.Path, Line: e.Line, ToPath: e.ToPath, Module: e.Module, Targets: slices.Compact(append([]string{}, targets...))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ToPath < b.ToPath
	})
	return out
}

func (si *subjectImports) resolve() bool {
	if si.done {
		return si.ok
	}
	si.done = true
	if si.dialect == goDialect {
		sites, faults, err := goImportSites(si.ctx, si.root, si.files, si.inScope)
		if err != nil {
			si.notes = append(si.notes, si.name+": import_resolution not measured: "+err.Error())
			return false
		}
		if len(faults) > 0 {
			si.notes = append(si.notes, fmt.Sprintf("%s: %d files whose imports could not be parsed (%s) resolve no import", si.name, len(faults), strings.Join(firstNames(faults, 5), ", ")))
		}
		si.sites, si.ok = sites, true
		return true
	}
	lang, ok := importLang(si.dialect)
	if !ok {
		si.notes = append(si.notes, "import_resolution not measured: the dialect names no import strategy (design §6)")
		return false
	}
	ris, rep := fusion.ResolveImportSpecs(si.ctx, lang, si.root, si.files, si.nodes)
	if rep.Skipped {
		si.notes = append(si.notes, si.name+": import_resolution not measured: "+rep.SkipReason)
		return false
	}
	if rep.Faults > 0 {
		si.notes = append(si.notes, fmt.Sprintf("%s: %d files whose imports could not be parsed (%s) resolve no import", si.name, rep.Faults, strings.Join(rep.FaultFiles, ", ")))
	}
	if includePathLang(lang) {
		// LB-Q-c-rest: what the include-path resolution settled by fallback
		// or by prototype, each counted (the file-edge pass's own report).
		if _, fr := fusion.ResolveFileEdges(si.ctx, lang, si.root, si.files, si.nodes); !fr.Skipped {
			si.notes = append(si.notes, fmt.Sprintf(noteIncludePath, si.name, fr.CompileDBs, fr.AncestorIncludes,
				fr.ProtoBound, fr.ProtoBuildFiltered, fr.ProtoAmbiguous, fr.ProtoNoDefiner, fr.ProtoNotBuilt))
		}
	}
	for _, ri := range ris {
		si.sites = append(si.sites, langbench.ImportSite{Path: ri.File, StartLine: ri.StartLine, EndLine: ri.EndLine, Local: ri.Spec.LocalName, Targets: ri.Targets, External: ri.External})
	}
	si.ok = true
	return true
}

// score scores refs, the oracle's import references at tier, into sub.
func (si *subjectImports) score(refs []langbench.OracleImport, tier, tool string, sub *langbench.Subject) error {
	if !si.resolve() {
		return nil
	}
	if _, done := sub.Families[langbench.FamilyImports]; done {
		return fmt.Errorf("%s: imports given alongside another imports oracle (a SCIP index or an earlier file); measure one per subject", tool)
	}
	before := sub.Counts[langbench.CntImportsInRepo]
	langbench.ScoreImports(refs, si.sites, si.inScope, sub.Counts)
	sub.Families[langbench.FamilyImports] = tier
	if sub.Counts[langbench.CntImportsInRepo] == before {
		si.notes = append(si.notes, fmt.Sprintf("%s: %s resolves none of fathom's imports to a file of the subject, so import_resolution has no denominator here", si.name, tool))
	}
	return nil
}

// scoreImportsOracle scores a neutral oracle file's imports, when it has
// the field; one without it leaves import_resolution to another oracle.
func scoreImportsOracle(path string, si *subjectImports, sub *langbench.Subject) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var tf struct {
		Tool    string               `json:"tool"`
		Tier    string               `json:"tier"`
		Imports *[]importOracleEntry `json:"imports"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := scoreRangeCompanion(path, si, sub); err != nil {
		return err
	}
	if tf.Imports == nil {
		return nil
	}
	stmts := importStatements(*tf.Imports)
	refs := make([]langbench.OracleImport, 0, len(stmts))
	for _, e := range stmts {
		refs = append(refs, langbench.OracleImport{Path: e.Path, Line: e.Line, ToPath: e.ToPath, Explicit: true})
	}
	tier, err := oracleFileTier(path, tf.Tier) // round 4 C4-R2
	if err != nil {
		return err
	}
	if err := si.score(refs, tier, orDefault(tf.Tool, path), sub); err != nil {
		return err
	}
	if si.ok {
		si.misses = importMisses(stmts, si.sites, si.inScope)
	}
	return nil
}

// firstNames is names' first n entries.
func firstNames(names []string, n int) []string {
	if len(names) > n {
		return names[:n]
	}
	return names
}

// noteFileEdgesFromImports discloses LB-F57's truth: import statements, not
// references.
const noteFileEdgesFromImports = "file_edge_precision and file_edge_recall measured against an import dump's (file, imported file) pairs, there being no SCIP index: the truth is what each file's import statements name, not the files its references reach as a SCIP index's cross-file edges are, so a file reached without an import of its own (a file another one required, an autoloaded constant) is outside the truth and a resolved edge to it counts against precision, while a file imported and never referenced is in it (LB-F57)"

// noteSamePackageEdges discloses LB-F433's same-package pairs (K19, O519).
const noteSamePackageEdges = "%s: %d of the %d file-edge truth pairs are same-package references no import statement writes (the dump's same_package entries: a file reaching a top-level name of its own package, which the language lets it do with no import; LB-F433, O519); they score file_edge_precision and file_edge_recall and never import_resolution"

// scoreImportFileEdges (LB-F57) scores fathom's file-resolved edges
// (fusion.ResolveFileEdges under lang) against a neutral oracle file's
// imports when no SCIP index gives cross-file edges: each (path, to_path)
// pair with both files measured and path != to_path is a truth edge, scored
// by langbench.ScoreFileEdges into the file_edges family at the file's
// tier. A file with no such pair scores nothing.
func scoreImportFileEdges(ctx context.Context, path, lang, root, name string, files []core.SourceFile, nodes []core.SymbolNode, sub *langbench.Subject, inScope map[string]bool) (notes []string, diff *fileEdgeDiff, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var tf struct {
		Tool    string              `json:"tool"`
		Tier    string              `json:"tier"`
		Imports []importOracleEntry `json:"imports"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	truth := map[[2]string]bool{}
	pkgOnly := map[[2]string]bool{}
	for _, e := range tf.Imports {
		if e.ToPath != "" && e.Path != e.ToPath && inScope[e.Path] && inScope[e.ToPath] {
			k := [2]string{e.Path, e.ToPath}
			if e.SamePackage && !truth[k] {
				pkgOnly[k] = true
			} else if !e.SamePackage {
				delete(pkgOnly, k)
			}
			truth[k] = true
		}
	}
	if len(truth) == 0 {
		return nil, nil, nil
	}
	if _, done := sub.Families[langbench.FamilyFileEdges]; done {
		return nil, nil, fmt.Errorf("%s: file-edge truth given alongside another (an earlier file's imports); measure one per subject", orDefault(tf.Tool, path))
	}
	tier, err := oracleFileTier(path, tf.Tier) // round 4 C4-R2
	if err != nil {
		return nil, nil, err
	}
	edges, rep := fusion.ResolveFileEdges(ctx, lang, root, files, nodes)
	if rep.Skipped {
		return []string{name + ": file-resolved edges not measured: " + rep.SkipReason}, nil, nil
	}
	langbench.ScoreFileEdges(truth, edges, nodes, sub.Counts)
	sub.Families[langbench.FamilyFileEdges] = tier
	diff = &fileEdgeDiff{truth: truth, mine: fileEdgePairs(edges, nodes), via: fileEdgeVia(edges, nodes)}
	diff.by = fileEdgeBy(edges, nodes)
	notes = []string{noteFileEdgesFromImports}
	if len(pkgOnly) > 0 {
		notes = append(notes, fmt.Sprintf(noteSamePackageEdges, name, len(pkgOnly), len(truth)))
	}
	return notes, diff, nil
}

// rangeCompanionFile is the definition-range oracle that sits beside a tool
// oracle file (LB-Q-c-rest step 5): <name>.json -> <name>.ranges.json.
func rangeCompanionFile(toolOracle string) string {
	return strings.TrimSuffix(toolOracle, ".json") + ".ranges.json"
}

// rangeCompanion is a definition-range oracle's output: calibration/oracles/
// clang-ast-defs' (C and C++), or any parser that reports where a definition
// ends (calibration/oracles/parser-ranges: shfmt for Bash, pglast for SQL,
// wave lf4). Kinds maps the companion's own kind spelling to the manifest's
// canonical kind, and Scope says which definitions it reports; a companion
// that names neither is clang-ast-defs, whose kinds are already canonical.
type rangeCompanion struct {
	Tool    string            `json:"tool"`
	Tier    string            `json:"tier"`
	Version string            `json:"version"`
	Units   int               `json:"units"`
	Errored int               `json:"errored"`
	Kinds   map[string]string `json:"kinds,omitempty"`
	Scope   string            `json:"scope,omitempty"`
	Ranges  []struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Kind string `json:"kind"`
		Line int    `json:"line"`
		End  int    `json:"end"`
	} `json:"ranges"`
}

// noteRangeCompanion discloses what range_agreement was measured against when
// the companion is clang-ast-defs (no scope of its own).
const noteRangeCompanion = "range_agreement measured against %s's AST (tier %s, %s: %d translation units, %d it could not dump): the first and last line of each function with a body, struct or union with a body, enum and typedef, %d of fathom's definitions paired (L1's own pairing: same file and name, |Δline| <= 2); a macro has no AST node and is not scored, and the index's L1 oracle (scip-clang, no end line) is unchanged"

// noteRangeCompanionScoped discloses a companion that names its own scope.
const noteRangeCompanionScoped = "range_agreement measured against %s (tier %s, %s: %d files, %d it could not parse): %s, %d of fathom's definitions paired (L1's own pairing: same file and name, |Δline| <= 2); only the range counts are taken, so the L1 oracle is unchanged"

// rangeCompanionOracle is the OracleKinds key the companion's kinds are
// scored under.
const rangeCompanionOracle = "range-companion"

// scoreRangeCompanion scores range_agreement against a companion file of
// definition ranges when one sits beside the tool oracle (scip-clang and
// universal-ctags' Sh and SQL parsers write no end line, so neither can).
// Only the range counts are taken: the pairing is langbench.ScoreL1's, run
// against the companion's definitions, and its recall, precision and kind
// counts are not used, so the L1 oracle stays the reading's. A missing
// companion measures nothing.
func scoreRangeCompanion(toolOracle string, si *subjectImports, sub *langbench.Subject) error {
	b, err := os.ReadFile(rangeCompanionFile(toolOracle))
	if err != nil {
		return nil // no companion: range_agreement stays the L1 oracle's
	}
	var rc rangeCompanion
	if err := json.Unmarshal(b, &rc); err != nil {
		return fmt.Errorf("%s: %w", rangeCompanionFile(toolOracle), err)
	}
	tier, err := oracleFileTier(rangeCompanionFile(toolOracle), rc.Tier) // round 4 C4-R2
	if err != nil {
		return err
	}
	if _, ok := sub.Families[langbench.FamilyL1]; !ok {
		return nil // no L1 oracle ran for this subject: range_agreement has no family
	}
	spec, _, err := resolveBenchSpec(si.dialect, "")
	if err != nil {
		return err
	}
	m := spec.Manifest
	table := rc.Kinds
	if len(table) == 0 {
		table = map[string]string{}
		for _, c := range []string{"function", "struct", "union", "enum", "type_alias"} {
			table[c] = c
		}
	}
	oks := map[string]map[string]string{}
	for k, v := range m.OracleKinds {
		oks[k] = v
	}
	oks[rangeCompanionOracle] = table
	m.OracleKinds = oks
	var defs []langbench.OracleDef
	for _, r := range rc.Ranges {
		if si.inScope[r.Path] {
			defs = append(defs, langbench.OracleDef{Path: r.Path, Name: r.Name, Line: r.Line, End: r.End, Kind: r.Kind})
		}
	}
	res := langbench.ScoreL1(m, rangeCompanionOracle, si.nodes, defs)
	sub.Counts[langbench.CntRangeEligible] += res.Counts[langbench.CntRangeEligible]
	sub.Counts[langbench.CntRangeOK] += res.Counts[langbench.CntRangeOK]
	if rc.Scope == "" {
		si.notes = append(si.notes, fmt.Sprintf("%s: "+noteRangeCompanion, si.name, orDefault(rc.Tool, "clang"), tier, orDefault(rc.Version, "unversioned"), rc.Units, rc.Errored, res.Counts[langbench.CntRangeEligible]))
		return nil
	}
	si.notes = append(si.notes, fmt.Sprintf("%s: "+noteRangeCompanionScoped, si.name, orDefault(rc.Tool, "unnamed"), tier, orDefault(rc.Version, "unversioned"), rc.Units, rc.Errored, rc.Scope, res.Counts[langbench.CntRangeEligible]))
	return nil
}

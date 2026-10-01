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
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// importOracleEntry is one neutral-file import: the importing file, the
// line of the import statement, and the file of the subject it resolves
// to (module is the spec as written, for the reader; never scored).
type importOracleEntry struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
	Module string `json:"module,omitempty"`
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

	done  bool
	ok    bool
	sites []langbench.ImportSite
	notes []string
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
	for _, ri := range ris {
		si.sites = append(si.sites, langbench.ImportSite{Path: ri.File, StartLine: ri.StartLine, EndLine: ri.EndLine, Local: ri.Spec.LocalName, Targets: ri.Targets})
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
	if tf.Imports == nil {
		return nil
	}
	refs := make([]langbench.OracleImport, 0, len(*tf.Imports))
	for _, e := range *tf.Imports {
		refs = append(refs, langbench.OracleImport{Path: e.Path, Line: e.Line, ToPath: e.ToPath, Explicit: true})
	}
	return si.score(refs, orDefault(tf.Tier, "A"), orDefault(tf.Tool, path), sub)
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

// scoreImportFileEdges (LB-F57) scores fathom's file-resolved edges
// (fusion.ResolveFileEdges under lang) against a neutral oracle file's
// imports when no SCIP index gives cross-file edges: each (path, to_path)
// pair with both files measured and path != to_path is a truth edge, scored
// by langbench.ScoreFileEdges into the file_edges family at the file's
// tier. A file with no such pair scores nothing.
func scoreImportFileEdges(ctx context.Context, path, lang, root, name string, files []core.SourceFile, nodes []core.SymbolNode, sub *langbench.Subject, inScope map[string]bool) (notes []string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tf struct {
		Tool    string              `json:"tool"`
		Tier    string              `json:"tier"`
		Imports []importOracleEntry `json:"imports"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	truth := map[[2]string]bool{}
	for _, e := range tf.Imports {
		if e.ToPath != "" && e.Path != e.ToPath && inScope[e.Path] && inScope[e.ToPath] {
			truth[[2]string{e.Path, e.ToPath}] = true
		}
	}
	if len(truth) == 0 {
		return nil, nil
	}
	if _, done := sub.Families[langbench.FamilyFileEdges]; done {
		return nil, fmt.Errorf("%s: file-edge truth given alongside another (an earlier file's imports); measure one per subject", orDefault(tf.Tool, path))
	}
	edges, rep := fusion.ResolveFileEdges(ctx, lang, root, files, nodes)
	if rep.Skipped {
		return []string{name + ": file-resolved edges not measured: " + rep.SkipReason}, nil
	}
	langbench.ScoreFileEdges(truth, edges, nodes, sub.Counts)
	sub.Families[langbench.FamilyFileEdges] = orDefault(tf.Tier, "A")
	return []string{noteFileEdgesFromImports}, nil
}

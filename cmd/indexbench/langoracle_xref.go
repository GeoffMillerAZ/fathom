package main

// langoracle_xref.go — LB-F30 (owner decision D4, 2026-09-28): a tool
// oracle's cross_references scored against the doc engine's resolved
// cross-file references, computed with fusion.ResolveDocXrefs — the same
// resolver ingest stores edges from. LB-F56 (O167): a tags-engine tool
// format (protobuf, then cmake and graphql) is no doc engine; its
// cross-file references are the file resolver's edges over its import
// strategy (fusion.ResolveFileEdges, the pass ingest stores file-resolved
// edges from), scored at file granularity.

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

// xrefOracleEntry is one oracle cross-file reference: the referring
// entity (path, from) and the referenced one (to_path, to — to may be
// empty: file granularity, langbench.ScoreXrefs).
type xrefOracleEntry struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to,omitempty"`
}

// scoreXrefOracle scores the oracle file's cross_references, when it has
// the field, into sub (family xref, at the oracle's tier) and returns how
// many entries were dropped as out of scope. An oracle without the field
// leaves L3 unmeasured.
func scoreXrefOracle(path string, doc extractor.DocResulter, service string, files []core.SourceFile, sub *langbench.Subject, inScope map[string]bool) (int, error) {
	entries, tier, err := readXrefOracle(path)
	if err != nil || entries == nil {
		return 0, err
	}
	dropped := 0
	var truth []langbench.XrefPair
	for _, x := range entries {
		if !inScope[x.Path] || !inScope[x.ToPath] {
			dropped++
			continue
		}
		truth = append(truth, langbench.XrefPair{Path: x.Path, From: x.From, ToPath: x.ToPath, To: x.To})
	}
	var docs []fusion.DocFile
	for _, f := range files {
		r, err := doc.ExtractDoc(service, f.Path, f.Content)
		if err != nil {
			continue
		}
		docs = append(docs, fusion.DocFile{Path: f.Path, Result: r})
	}
	var found []langbench.XrefPair
	for _, x := range fusion.ResolveDocXrefs(docs) {
		found = append(found, langbench.XrefPair{Path: x.FromFile, From: x.From, ToPath: x.ToFile, To: x.To})
	}
	langbench.ScoreXrefs(truth, found, sub.Counts)
	sub.Families[langbench.FamilyXref] = tier
	return dropped, nil
}

// readXrefOracle reads an oracle file's cross_references and its tier;
// entries is nil when the file has no such field.
func readXrefOracle(path string) (entries []xrefOracleEntry, tier string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var tf struct {
		Tier            string             `json:"tier"`
		CrossReferences *[]xrefOracleEntry `json:"cross_references"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	if tf.CrossReferences == nil {
		return nil, "", nil
	}
	return append([]xrefOracleEntry{}, *tf.CrossReferences...), orDefault(tf.Tier, "A"), nil
}

// noteXrefFileResolved discloses what a tags-engine tool format's xref
// metrics compare (LB-F56).
const noteXrefFileResolved = "xref_precision and xref_recall of a tags-engine tool format score the file resolver's file-resolved edges (fusion.ResolveFileEdges over the dialect's import strategy, the edges ingest stores) against the oracle's cross_references at file granularity: (file, referring definition, referenced file), the referenced definition's name not compared (LB-F56)"

// fileXrefs is one subject's cross-file references from the file resolver
// (LB-F56, O167): each edge fusion.ResolveFileEdges draws between two
// measured files, read as (file, referring definition by the dialect's name
// matching, referenced file). Computed on first use and shared by every
// oracle of the subject.
type fileXrefs struct {
	ctx   context.Context
	m     langbench.Manifest
	root  string
	name  string
	files []core.SourceFile
	nodes []core.SymbolNode

	done  bool
	ok    bool
	found []langbench.XrefPair
	notes []string
}

func (fx *fileXrefs) resolve() bool {
	if fx.done {
		return fx.ok
	}
	fx.done = true
	lang, ok := importLang(fx.m.ID)
	if !ok {
		fx.notes = append(fx.notes, "xref not measured: the dialect is no doc engine and names no import strategy, so no cross-file reference is resolved (design §6)")
		return false
	}
	edges, rep := fusion.ResolveFileEdges(fx.ctx, lang, fx.root, fx.files, fx.nodes)
	if rep.Skipped {
		fx.notes = append(fx.notes, fx.name+": xref not measured: "+rep.SkipReason)
		return false
	}
	if rep.Faults > 0 {
		fx.notes = append(fx.notes, fmt.Sprintf("%s: %d files whose imports could not be parsed (%s) resolve no cross-file reference", fx.name, rep.Faults, strings.Join(rep.FaultFiles, ", ")))
	}
	byKey := make(map[string]core.SymbolNode, len(fx.nodes))
	for _, n := range fx.nodes {
		byKey[n.Key()] = n
	}
	for _, e := range edges {
		from, okFrom := byKey[e.FromSymbol]
		to, okTo := byKey[e.ToSymbol]
		if !okFrom || !okTo || to.FilePath == e.FromFile {
			continue
		}
		fx.found = append(fx.found, langbench.XrefPair{Path: e.FromFile, From: fx.m.MatchName(from.Name), ToPath: to.FilePath})
	}
	fx.notes = append(fx.notes, noteXrefFileResolved)
	fx.ok = true
	return true
}

// scoreFileXrefOracle scores the oracle file's cross_references, when it
// has the field, against fx's file-resolved references at file granularity
// (the oracle's referring name read by the dialect's name matching, its
// referenced name dropped), into sub (family xref, at the oracle's tier); it
// returns how many entries were dropped as out of scope.
func scoreFileXrefOracle(path string, fx *fileXrefs, sub *langbench.Subject, inScope map[string]bool) (int, error) {
	entries, tier, err := readXrefOracle(path)
	if err != nil || entries == nil || !fx.resolve() {
		return 0, err
	}
	dropped := 0
	var truth []langbench.XrefPair
	for _, x := range entries {
		if !inScope[x.Path] || !inScope[x.ToPath] {
			dropped++
			continue
		}
		truth = append(truth, langbench.XrefPair{Path: x.Path, From: fx.m.MatchName(x.From), ToPath: x.ToPath})
	}
	langbench.ScoreXrefs(truth, fx.found, sub.Counts)
	sub.Families[langbench.FamilyXref] = tier
	return dropped, nil
}

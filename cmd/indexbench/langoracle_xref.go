package main

// langoracle_xref.go — LB-F30 (owner decision D4, 2026-09-28): a tool
// oracle's cross_references scored against the doc engine's resolved
// cross-file references, computed with fusion.ResolveDocXrefs — the same
// resolver ingest stores edges from.

import (
	"encoding/json"
	"fmt"
	"os"

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
func scoreXrefOracle(path string, doc *extractor.DocExtractor, service string, files []core.SourceFile, sub *langbench.Subject, inScope map[string]bool) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var tf struct {
		Tier            string             `json:"tier"`
		CrossReferences *[]xrefOracleEntry `json:"cross_references"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	if tf.CrossReferences == nil {
		return 0, nil
	}
	dropped := 0
	var truth []langbench.XrefPair
	for _, x := range *tf.CrossReferences {
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
	sub.Families[langbench.FamilyXref] = orDefault(tf.Tier, "A")
	return dropped, nil
}

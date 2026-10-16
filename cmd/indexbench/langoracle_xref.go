package main

// langoracle_xref.go — LB-F30 (owner decision D4, 2026-09-28): a tool
// oracle's cross_references scored against the doc engine's resolved
// cross-file references, computed with fusion.ResolveDocXrefs — the same
// resolver ingest stores edges from. LB-F56 (O167): a tags-engine tool
// format (protobuf, then cmake and graphql) is no doc engine; its
// cross-file references are the file resolver's edges over its import
// strategy (fusion.ResolveFileEdges, the pass ingest stores file-resolved
// edges from), scored at file granularity. The jenkinsfile dialect, a tags
// tool format with no import strategy, is scored from the doc-xref pass's
// Jenkins resolution instead (fusion's JenkinsXrefsForBench; LB-Q-jenkinsfile
// L3, design §1.8 "Jenkins pipelines' L3"), at (file, file).

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
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
	// Edge names the reference's relation when the oracle says (LB-F370:
	// "handler", a Lambda function's handler symbol in a code file the
	// dialect does not read, kept in scope when the subject holds the file).
	Edge string `json:"edge,omitempty"`
}

// scoreXrefOracle scores the oracle file's cross_references, when it has
// the field, into sub (family xref, at the oracle's tier) and returns how
// many entries were dropped as out of scope. An oracle without the field
// leaves L3 unmeasured.
//
// The doc-xref pass's other edges ingest stores beside ResolveDocXrefs' are
// scored with them (LB-F369, LB-F370): the identity joins
// (fusion.ResolveIdentityXrefs) and each Lambda function's handler symbol
// (fusion.ResolveLambdaHandlers over code, the subject's files read and
// extracted as ingest dispatches them); a handler's own counts are returned
// as a note.
func scoreXrefOracle(path string, doc extractor.DocResulter, service string, files []core.SourceFile, code fusion.LambdaCode, sub *langbench.Subject, inScope map[string]bool) (int, string, error) {
	entries, tier, err := readXrefOracle(path)
	if err != nil || entries == nil {
		return 0, "", err
	}
	dropped := 0
	var truth, handlerTruth []langbench.XrefPair
	for _, x := range entries {
		handler := x.Edge == fusion.LambdaHandlerEdge && code.Paths[x.ToPath]
		if !inScope[x.Path] || (!inScope[x.ToPath] && !handler) {
			dropped++
			continue
		}
		p := langbench.XrefPair{Path: x.Path, From: x.From, ToPath: x.ToPath, To: x.To}
		truth = append(truth, p)
		if x.Edge == fusion.LambdaHandlerEdge {
			handlerTruth = append(handlerTruth, p)
		}
	}
	var docs []fusion.DocFile
	for _, f := range files {
		r, err := doc.ExtractDoc(service, f.Path, f.Content)
		if err != nil {
			continue
		}
		docs = append(docs, fusion.DocFile{Path: f.Path, Result: r})
	}
	var found, handlerFound []langbench.XrefPair
	xs := append(fusion.ResolveDocXrefs(docs), fusion.ResolveIdentityXrefs(docs)...)
	hs, st := fusion.ResolveLambdaHandlers(docs, code)
	for _, x := range append(xs, hs...) {
		p := langbench.XrefPair{Path: x.FromFile, From: x.From, ToPath: x.ToFile, To: x.To}
		found = append(found, p)
		if x.Edge == fusion.LambdaHandlerEdge {
			handlerFound = append(handlerFound, p)
		}
	}
	langbench.ScoreXrefs(truth, found, sub.Counts)
	sub.Families[langbench.FamilyXref] = tier
	note := ""
	if len(handlerTruth) > 0 || st.Functions > 0 {
		hc := map[string]int{}
		langbench.ScoreXrefs(handlerTruth, handlerFound, hc)
		note = fmt.Sprintf("Lambda handler bindings (LB-F370, in xref): %d functions declared, %d bound, unbound by reason %v; against the oracle's %d handler references, %d found (recall %s), %d of fathom's %d in the truth (precision %s)",
			st.Functions, st.Bound, st.Unbound, hc[langbench.CntXrefTruth], hc[langbench.CntXrefTruthFound], ratioText(hc[langbench.CntXrefTruthFound], hc[langbench.CntXrefTruth]),
			hc[langbench.CntXrefFathomInTrue], hc[langbench.CntXrefFathom], ratioText(hc[langbench.CntXrefFathomInTrue], hc[langbench.CntXrefFathom]))
	}
	return dropped, note, nil
}

// ratioText is n/d to four places, "n/a" for none.
func ratioText(n, d int) string {
	if d == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.4f", float64(n)/float64(d))
}

// lambdaCode is a subject's files as the handler binding reads them: every
// path discovery lists, read from the checkout and extracted through the
// pipeline's dispatch (LB-F370).
func lambdaCode(pipe *fusion.Pipeline, service, root string, refs []core.SourceRef) fusion.LambdaCode {
	paths := make(map[string]bool, len(refs))
	for _, r := range refs {
		paths[r.Path] = true
	}
	read := func(rel string) ([]byte, bool) {
		if !paths[rel] {
			return nil, false
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return b, err == nil
	}
	return fusion.LambdaCode{Paths: paths, Read: read, Symbols: func(rel string) []core.SymbolNode {
		b, ok := read(rel)
		if !ok {
			return nil
		}
		ex, err := pipe.ExtractForBench(service, refs, rel, b)
		if err != nil {
			return nil
		}
		return ex.Nodes
	}}
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
	// targets is, for the jenkinsfile dialect, every file of the subject the
	// dialect read: an edge's possible targets, -only aside (resolveJenkins).
	targets map[string]bool
}

func (fx *fileXrefs) resolve() bool {
	if fx.done {
		return fx.ok
	}
	fx.done = true
	if fx.m.ID == "jenkinsfile" {
		fx.ok = fx.resolveJenkins()
		return fx.ok
	}
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

// noteJenkinsXrefUnit discloses what the jenkinsfile dialect's xref metrics
// compare (design §1.8 "Jenkins pipelines' L3").
const noteJenkinsXrefUnit = "xref_precision and xref_recall of the jenkinsfile dialect score the doc-xref pass's Jenkins edges (fusion's JenkinsXrefsForBench over the subject's files, the resolution ingest stores the edges from) against the oracle's cross_references at (file, file): a load step's edge and a shared-library step call's (LB-F375), from the pipelines -only measures to any file of the subject the dialect reads; a shell step's first-command edge (LB-F374) is out of the unit, counted (design §1.8)"

// resolveJenkins reads the subject's Jenkins edges (LB-Q-jenkinsfile L3):
// every file of the subject discovered, its Jenkins files resolved by
// fusion's JenkinsXrefsForBench, and the load and library-step edges whose
// referring file this run measures kept at (file, file).
func (fx *fileXrefs) resolveJenkins() bool {
	refs, err := connector.NewLocalConnector().Discover(fx.ctx, core.ServiceRef{ID: "lang-oracle", Path: fx.root})
	if err != nil {
		fx.notes = append(fx.notes, fx.name+": xref not measured: "+err.Error())
		return false
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	jb := pipe.JenkinsXrefsForBench("lang-oracle", refs, func(r core.SourceRef) ([]byte, error) {
		return os.ReadFile(filepath.Join(fx.root, filepath.FromSlash(r.Path)))
	})
	measured := make(map[string]bool, len(fx.files))
	for _, f := range fx.files {
		measured[f.Path] = true
	}
	fx.targets = make(map[string]bool, len(jb.Read))
	for _, p := range jb.Read {
		fx.targets[p] = true
	}
	shell, outside := 0, 0
	for _, x := range jb.Xrefs {
		switch {
		case x.Edge != fusion.JenkinsLoadEdge && x.Edge != fusion.JenkinsVarEdge:
			shell++
		case !measured[x.FromFile]:
			outside++
		default:
			fx.found = append(fx.found, langbench.XrefPair{Path: x.FromFile, ToPath: x.ToFile})
		}
	}
	var unresolved []string
	for _, k := range slices.Sorted(maps.Keys(jb.Unresolved)) {
		if n := jb.Unresolved[k]; n > 0 {
			unresolved = append(unresolved, fmt.Sprintf("%s %d", k, n))
		}
	}
	fx.notes = append(fx.notes, noteJenkinsXrefUnit, fmt.Sprintf("%s: Jenkins steps read in the %d files the dialect reads: %d load steps, %d literal shell steps; %d shell-step edges left out of the unit, %d load or library-step edges from files -only does not measure left out of scope; unresolved: [%s]",
		fx.name, len(jb.Read), jb.Loads, jb.Shells, shell, outside, strings.Join(unresolved, ", ")))
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
		if fx.targets != nil {
			// jenkinsfile: (file, file); a target is any file the dialect read
			if !inScope[x.Path] || !fx.targets[x.ToPath] {
				dropped++
				continue
			}
			truth = append(truth, langbench.XrefPair{Path: x.Path, ToPath: x.ToPath})
			continue
		}
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

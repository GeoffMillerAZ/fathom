package fusion

// docxref.go — LB-F30 (owner decision D4, 2026-09-28; docs/design/
// language-breadth.md §4): the doc engine's cross_file references resolved
// to entities and persisted, so a tool format's L3 is scored on edges the
// store actually holds. One resolver serves ingest (enrichDocXrefEdges) and
// the language-breadth harness (indexbench lang-oracle), so the bench never
// scores an edge ingest drops.
//
// Edge shape: a file-tier typed edge (core.EdgeFileResolved, tier
// pre-scan, never presented as typed) from the referring entity to the
// referenced entity, stored under the typed_edges key "doc-xref" and
// replaced wholesale each pass — the same table and label ResolveFileEdges
// writes. A reference that leaves the repository (a registry package, a
// remote action, an image) resolves to nothing here; crossing repositories
// is the service_ref/module_ref layer's job (design §1.1).

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// DocFile is one doc-engine file's extraction.
type DocFile struct {
	Path   string
	Result extractor.DocResult
}

// DocXref is one resolved cross-file reference: entity to entity.
type DocXref struct {
	FromFile, From, FromKey string
	ToFile, To, ToKey       string
	Edge, Strategy          string
}

// docXrefPackageKey is the typed_edges key the doc-xref pass replaces.
const docXrefPackageKey = "doc-xref"

const docXrefStateKey = typedPassStatePrefix + "doc-xref"

// ResolveDocXrefs resolves every file's cross_file references against the
// set (deterministic, sorted): repo-path, repo-root and a relative
// module-source name a file, the one doc file directly in a named
// directory, or every such file in the directories a glob names, and then
// that file's first root entity; a non-relative module-source or a
// service-name resolves to the one entity (a root entity, for
// module-source) of that name in another file; a label-selector joins
// every labelled entity in its scope whose labels include the selector.
// image-ref and anything unresolved yield nothing. A reference resolving
// to its own origin's symbol key is dropped (it would be a self-edge).
func ResolveDocXrefs(files []DocFile) []DocXref {
	byPath := map[string]*DocFile{}
	byDir := map[string][]string{}
	type fk struct{ file, name string }
	nodeAt := map[fk]core.SymbolNode{}
	rootByName := map[string][]fk{}
	byName := map[string][]fk{}
	for i := range files {
		f := &files[i]
		byPath[f.Path] = f
		byDir[path.Dir(f.Path)] = append(byDir[path.Dir(f.Path)], f.Path)
		for _, n := range f.Result.Nodes {
			k := fk{f.Path, n.Name}
			if _, dup := nodeAt[k]; !dup {
				nodeAt[k] = n
				byName[n.Name] = append(byName[n.Name], k)
			}
		}
		for _, r := range f.Result.Roots {
			rootByName[r] = append(rootByName[r], fk{f.Path, r})
		}
	}
	rootOf := func(file string) (fk, bool) {
		f := byPath[file]
		if f == nil || len(f.Result.Roots) == 0 {
			return fk{}, false
		}
		return fk{file, f.Result.Roots[0]}, true
	}
	fileTargets := func(target string) []string {
		if target == "" {
			return nil
		}
		if byPath[target] != nil {
			return []string{target}
		}
		var dirs []string
		if strings.ContainsAny(target, "*?[") {
			for d := range byDir {
				if ok, _ := path.Match(target, d); ok {
					dirs = append(dirs, d)
				}
			}
			sort.Strings(dirs)
		} else {
			dirs = []string{target}
		}
		var out []string
		for _, d := range dirs {
			if fs := byDir[d]; len(fs) == 1 {
				out = append(out, fs[0])
			}
		}
		return out
	}
	isRel := func(raw string) bool {
		return raw == "." || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") || strings.HasPrefix(raw, "/")
	}

	var out []DocXref
	seen := map[string]bool{}
	add := func(from fk, to fk, c extractor.DocCrossFileRef) {
		fn, ok1 := nodeAt[from]
		tn, ok2 := nodeAt[to]
		if !ok1 || !ok2 || fn.Key() == tn.Key() {
			return
		}
		x := DocXref{FromFile: from.file, From: from.name, FromKey: fn.Key(), ToFile: to.file, To: to.name, ToKey: tn.Key(), Edge: c.Edge, Strategy: c.Strategy}
		id := x.FromFile + "\x00" + x.From + "\x00" + x.ToFile + "\x00" + x.To + "\x00" + x.Edge
		if !seen[id] {
			seen[id] = true
			out = append(out, x)
		}
	}
	unique := func(ks []fk, notFile string) (fk, bool) {
		var hit []fk
		for _, k := range ks {
			if k.file != notFile {
				hit = append(hit, k)
			}
		}
		if len(hit) == 1 {
			return hit[0], true
		}
		return fk{}, false
	}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			from := fk{f.Path, c.From}
			if c.From == "" {
				r, ok := rootOf(f.Path)
				if !ok {
					continue
				}
				from = r
			}
			switch c.Strategy {
			case "repo-path", "repo-root", "module-source":
				if c.Strategy == "repo-path" || isRel(c.Raw) {
					for _, t := range fileTargets(c.Target) {
						if to, ok := rootOf(t); ok {
							add(from, to, c)
						}
					}
					continue
				}
				if to, ok := unique(rootByName[c.Raw], f.Path); ok {
					add(from, to, c)
				}
			case "service-name":
				if to, ok := unique(byName[c.Raw], f.Path); ok {
					add(from, to, c)
				}
			case "label-selector":
				for _, g := range files {
					for _, ls := range g.Result.Labels {
						if ls.Scope == c.Scope && labelsInclude(ls.Labels, c.Labels) {
							add(from, fk{g.Path, ls.Entity}, c)
						}
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	return out
}

// labelsInclude: every selector label is present with the same value (a
// Kubernetes equality selector; an empty selector selects nothing here).
func labelsInclude(have, sel map[string]string) bool {
	if len(sel) == 0 {
		return false
	}
	for k, v := range sel {
		if have[k] != v {
			return false
		}
	}
	return true
}

// DocXrefEdges is the store form of resolved references: one file-tier
// typed edge per (from, to) symbol pair, sorted.
func DocXrefEdges(xs []DocXref) []core.Edge {
	seen := map[[2]string]bool{}
	var edges []core.Edge
	for _, x := range xs {
		k := [2]string{x.FromKey, x.ToKey}
		if seen[k] {
			continue
		}
		seen[k] = true
		edges = append(edges, core.Edge{FromFile: x.FromFile, FromSymbol: x.FromKey, ToSymbol: x.ToKey, Kind: core.EdgeFileResolved})
	}
	sortEdges(edges)
	return edges
}

// isDocCandidate: a path a doc-engine dialect may read (its decode's
// extensions), or a native dialect's manifest by basename (go.mod, go.work).
// Classification decides; this only gates the pass.
func isDocCandidate(p string) bool {
	switch path.Base(p) {
	case "go.mod", "go.work": // the native go-mod dialect (LB-Q-go-mod)
		return true
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".yaml", ".yml", ".json", ".jsonc", ".toml", ".xml":
		return true
	}
	return false
}

// enrichDocXrefEdges is the Ingest hook for LB-F30: when a doc candidate
// changed (or the last pass did not complete), every doc candidate is read
// through ingest's gates and dispatch, each file a doc-engine dialect
// governs is walked with ExtractDoc, and the resolved references replace
// the "doc-xref" typed edges. Best-effort per file; a halt is returned.
func (p *Pipeline) enrichDocXrefEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (int, string, error) {
	if p.cfg.DisableTypedEdges {
		return 0, "typed edges disabled", nil
	}
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, docXrefStateKey), isDocCandidate) {
		return 0, SkippedPassReason, nil
	}
	p.beginPassState(ctx, svc.ID, docXrefStateKey)
	idx := buildSiblingIndex(refs)
	var files []DocFile
	for _, r := range refs {
		if !isDocCandidate(r.Path) {
			continue
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return 0, "", h
			}
			continue
		}
		if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			continue
		}
		e, _, ok, _ := p.resolveExtractor(r.Path, content, idx)
		doc, isDoc := e.(extractor.DocResulter)
		if !ok || !isDoc {
			continue
		}
		res, err := doc.ExtractDoc(svc.ID, r.Path, content)
		if err != nil {
			continue
		}
		files = append(files, DocFile{Path: r.Path, Result: res})
	}
	edges := DocXrefEdges(ResolveDocXrefs(files))
	if err := p.st.PutTypedEdges(ctx, svc.ID, docXrefPackageKey, edges); err != nil {
		return 0, "", fmt.Errorf("doc-xref edges: store: %w", err)
	}
	p.commitPassState(ctx, svc.ID, docXrefStateKey)
	return len(edges), "", nil
}

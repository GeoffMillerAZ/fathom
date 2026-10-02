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
// remote action, an image nothing in the repository builds) resolves to
// nothing here; crossing repositories is the service_ref/module_ref layer's
// job (design §1.1). An image the repository builds resolves through the
// image-tag index (imageref.go, LB-X-docker-image-ref).

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

// DocFile is one doc-engine file's extraction. Builds are the image build
// sites the file writes outside its DocResult (a docker-bake target, a
// literal `docker build` in a script, a Makefile or a CI run step), read
// into the image-tag index (LB-X-docker-image-ref).
type DocFile struct {
	Path   string
	Result extractor.DocResult
	Builds []ImageBuild
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
// directory (of several, the one the referring file's dialect owns by its
// basenames and globs, O165), or every such file in the directories a glob
// names, and then that file's first root entity; a non-relative
// module-source or a service-name resolves to the one entity (a root
// entity, for module-source) of that name in another file; a
// label-selector joins every labelled entity in its scope whose labels
// include the selector; an image-ref resolves through the image-tag index
// to the root entity of the one Dockerfile of the set that builds it, and
// a build context (edge "build", with "build_dockerfile") to that of the
// Dockerfile it names (LB-X-docker-image-ref). Anything unresolved yields
// nothing. A reference resolving to its own origin's symbol key is dropped
// (it would be a self-edge), as is an image-ref back into its own file.
func ResolveDocXrefs(files []DocFile) []DocXref {
	xs, _ := resolveDocXrefs(files)
	return xs
}

// imageRefStats is what the image-ref half of a resolution read and
// skipped, for the pass's disclosure.
type imageRefStats struct {
	Sites, TemplatedBuilds, Ambiguous, TemplatedRefs int
}

func resolveDocXrefs(files []DocFile) ([]DocXref, imageRefStats) {
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
	fileTargets := func(target, referrer string) []string {
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
			if f, ok := oneDocFile(byDir[d], referrer); ok {
				out = append(out, f)
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
	// label sets by join scope, in file then label order (O320): a
	// label-selector compares only the sets in its own scope, so a repository
	// of many terraform modules (every definition labelled) costs what its
	// largest module costs, not references times every label set.
	type scopedLabels struct {
		file string
		ls   extractor.DocLabelSet
	}
	labelsByScope := map[string][]scopedLabels{}
	for _, g := range files {
		for _, ls := range g.Result.Labels {
			labelsByScope[ls.Scope] = append(labelsByScope[ls.Scope], scopedLabels{g.Path, ls})
		}
	}
	img := BuildImageIndex(files)
	stats := imageRefStats{Sites: img.Sites, TemplatedBuilds: img.TemplatedBuilds, Ambiguous: img.Ambiguous}
	for _, f := range files {
		origin := func(name string) (fk, bool) {
			if name == "" {
				return rootOf(f.Path)
			}
			return fk{f.Path, name}, true
		}
		// a build context, with the Dockerfile named relative to it,
		// resolves once per origin to the Dockerfile it names
		builds := composeBuildsOf(f)
		for _, name := range sortedBuildOrigins(builds) {
			b := builds[name]
			from, ok := origin(name)
			if !ok {
				continue
			}
			if t, ok := b.file(f.Path); ok {
				if to, ok := rootOf(t); ok {
					add(from, to, extractor.DocCrossFileRef{From: name, Edge: buildEdge, Strategy: "repo-path", Line: b.line})
				}
			}
		}
		for _, c := range f.Result.CrossFile {
			from, ok := origin(c.From)
			if !ok {
				continue
			}
			if c.Strategy == "repo-path" && (c.Edge == buildEdge || c.Edge == buildDockerfileEdge) {
				continue
			}
			if c.Edge == PolicyRefEdge {
				continue // the policy-ref pass resolves these (policyref.go, LB-F135)
			}
			switch c.Strategy {
			case "image-ref":
				if templated(c.Raw) {
					stats.TemplatedRefs++
					continue
				}
				if t, ok := img.Lookup(c.Raw); ok && t != f.Path {
					if to, ok := rootOf(t); ok {
						add(from, to, c)
					}
				}
			case "repo-path", "repo-root", "module-source":
				if c.Strategy == "repo-path" || isRel(c.Raw) {
					for _, t := range fileTargets(c.Target, f.Path) {
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
				for _, g := range labelsByScope[c.Scope] {
					if labelsInclude(g.ls.Labels, c.Labels) {
						add(from, fk{g.file, g.ls.Entity}, c)
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
	return out, stats
}

// sortedBuildOrigins is the origins of a file's builds, in name order.
func sortedBuildOrigins(m map[string]*composeBuild) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// oneDocFile is the file a directory reference names among the doc files
// directly in the directory (design §4): the only one, or, of several, the
// only one the referring file's dialect owns by its manifest's basenames and
// globs (O165: a Taskfile include of website/, which holds a package.json
// beside its Taskfile; a kustomize base beside its Kubernetes manifests) —
// the set an ingest resolves over holds every doc file, the harness's only the
// dialect's own, so the rule must not depend on which.
func oneDocFile(fs []string, referrer string) (string, bool) {
	if len(fs) == 1 {
		return fs[0], true
	}
	own := ownedDialect(referrer)
	if own == "" {
		return "", false
	}
	var hit []string
	for _, f := range fs {
		if ownedDialect(f) == own {
			hit = append(hit, f)
		}
	}
	if len(hit) == 1 {
		return hit[0], true
	}
	return "", false
}

// ownedDialect is the dialect whose manifest claims p by basename or glob
// ("" for none, or an extension-only claim).
func ownedDialect(p string) string {
	v := connector.ClassifyDialect(dialectRules(), p, nil)
	if v.By != connector.ClassifiedByBasename && v.By != connector.ClassifiedByGlob {
		return ""
	}
	return v.Dialect
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
// extensions, and the extensions an XML dialect owns instead of .xml: MSBuild's),
// or a native dialect's file by basename (go.mod, go.work) or extension (.tf).
// Classification decides; this only gates the pass.
func isDocCandidate(p string) bool {
	switch path.Base(p) {
	case "go.mod", "go.work": // the native go-mod dialect (LB-Q-go-mod)
		return true
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".yaml", ".yml", ".json", ".jsonc", ".toml", ".xml":
		return true
	case ".csproj", ".fsproj", ".vbproj", ".props", ".targets": // MSBuild (LB-Q-msbuild): XML by another name
		return true
	case ".tf": // the native terraform dialect (LB-Q-terraform, LB-F84)
		return true
	}
	return false
}

// enrichDocXrefEdges is the Ingest hook for LB-F30: when a doc candidate
// or an image build candidate changed (or the last pass did not complete),
// every candidate is read through ingest's gates and dispatch, each file a
// doc-engine dialect governs is walked with ExtractDoc, each Dockerfile the
// dockerfile dialect governs is read as its stages (DockerfileDocFile), a
// bake file, script, Makefile or CI file contributes its image build sites,
// and the resolved references replace the "doc-xref" typed edges.
// Best-effort per file; a halt is returned.
func (p *Pipeline) enrichDocXrefEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (int, string, error) {
	if p.cfg.DisableTypedEdges {
		return 0, "typed edges disabled", nil
	}
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, docXrefStateKey), isDocXrefInput) {
		return 0, SkippedPassReason, nil
	}
	p.beginPassState(ctx, svc.ID, docXrefStateKey)
	idx := buildSiblingIndex(refs)
	dockerfile := p.exts["Dockerfile"]
	var files []DocFile
	for _, r := range refs {
		img := isImageBuildCandidate(r.Path)
		if !isDocCandidate(r.Path) && !img {
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
		f := DocFile{Path: r.Path}
		keep := false
		e, _, ok, _ := p.resolveExtractor(r.Path, content, idx)
		if doc, isDoc := e.(extractor.DocResulter); ok && isDoc {
			if res, err := doc.ExtractDoc(svc.ID, r.Path, content); err == nil {
				f.Result, keep = res, true
			}
		} else if ok && img && dockerfile != nil && e == dockerfile {
			if nodes, err := e.Extract(svc.ID, r.Path, content); err == nil {
				f, keep = DockerfileDocFile(r.Path, content, nodes), true
			}
		}
		if img {
			if b := imageBuildSites(r.Path, content); len(b) > 0 {
				f.Builds, keep = b, true
			}
		}
		if keep {
			files = append(files, f)
		}
	}
	xs, st := resolveDocXrefs(files)
	if st.TemplatedBuilds > 0 || st.TemplatedRefs > 0 || st.Ambiguous > 0 {
		p.diagInfo("fusion: doc-xref pass %q: image-tag index: %d build sites; %d build sites and %d image references skipped as templated (${X}, $(X), $X); %d tags ambiguous (two Dockerfiles build them)\n",
			svc.ID, st.Sites, st.TemplatedBuilds, st.TemplatedRefs, st.Ambiguous)
	}
	edges := DocXrefEdges(xs)
	if err := p.st.PutTypedEdges(ctx, svc.ID, docXrefPackageKey, edges); err != nil {
		return 0, "", fmt.Errorf("doc-xref edges: store: %w", err)
	}
	p.commitPassState(ctx, svc.ID, docXrefStateKey)
	return len(edges), "", nil
}

// isDocXrefInput: a path whose change re-runs the doc-xref pass — a doc
// candidate, or a file the image-tag index reads.
func isDocXrefInput(p string) bool { return isDocCandidate(p) || isImageBuildCandidate(p) }

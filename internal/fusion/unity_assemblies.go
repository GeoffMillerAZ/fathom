package fusion

// Unity assembly ownership is an L3 file-resolution strategy over the doc
// dialect, not C# type checking. Rules: documented behavior of Unity 2021
// and later; native CompilationPipeline calibration on Unity 6000.3 pending.
// https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompilationAssemblyDefinitionFiles.html
// https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompileOrderFolders.html
// Reference: https://github.com/Unity-Technologies/UnityCsReference/blob/master/Editor/Mono/Scripting/ScriptCompilation/EditorBuildRules.cs
// Everything here remains derived (the existing pre-scan tier).

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const unityAssemblyEdgeKey = "unity-assembly"
const unityRules = extractor.UnityAssemblyRules

type unityAssemblyFile struct {
	path    string
	content []byte
	fault   string
}

type unityAssemblyResult struct {
	nodes      []core.SymbolNode
	edges      []core.Edge
	unresolved int
	problems   []string
	ran        bool
}

type unityDefinitionJSON = extractor.UnityDefinitionJSON

type unityDefinition struct {
	unityDefinitionJSON
	node        core.SymbolNode
	constraints []core.SymbolNode
}

func (d *unityDefinition) scope() string { return d.Scope() }

func unityAssemblyInput(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".cs", ".asmdef", ".asmref":
		return true
	case ".meta":
		return strings.HasSuffix(strings.ToLower(p), ".asmdef.meta")
	}
	return path.Base(p) == "ProjectVersion.txt" && path.Base(path.Dir(p)) == "ProjectSettings"
}

func unityDerivedNode(n core.SymbolNode) core.SymbolNode {
	if strings.HasPrefix(string(n.Kind), "Unity") {
		n.Tier = core.TierPreScan
	}
	return n
}

// ResolveUnityAssemblies computes deterministic assembly/file/constraint facts
// over an ingest's bounded, already-discovered inputs. No filesystem reads,
// clocks, Unity installation, compiler, network, or retained cache are involved.
// A faulted or ambiguous definition is a folder barrier, never permission to
// fall back to a parent or predefined assembly. GUIDs resolve only via the
// target asmdef's sibling .meta, never by assembly or file name.
func ResolveUnityAssemblies(service string, input []unityAssemblyFile) (unityAssemblyResult, error) {
	var out unityAssemblyResult
	var files []unityAssemblyFile
	active := false
	for _, f := range input {
		if !unityImportedPath(f.path) {
			continue
		}
		files = append(files, f)
		if unityMetadataInput(f.path) && path.Ext(f.path) != ".meta" {
			active = true
		}
	}
	if !active {
		return out, nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	roots := unityProjectRoots(files)
	projects := make(map[string][]unityAssemblyFile, len(roots))
	for _, f := range files {
		root := unityRootFor(f.path, roots)
		projects[root] = append(projects[root], f)
	}
	for i, root := range roots {
		for _, other := range roots[i+1:] {
			if unityUnderRoot(other, root) {
				out.problems = append(out.problems, "ambiguous Unity project roots: "+root+" and "+other)
			}
		}
		r, err := resolveUnityProject(service, root, projects[root])
		if err != nil {
			return out, err
		}
		out.nodes = append(out.nodes, r.nodes...)
		out.edges = append(out.edges, r.edges...)
		out.problems = append(out.problems, r.problems...)
		out.unresolved += r.unresolved
	}
	sort.Slice(out.nodes, func(i, j int) bool {
		a, b := out.nodes[i], out.nodes[j]
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	sortEdges(out.edges)
	sort.Strings(out.problems)
	return out, nil
}

// Unity's asset import exclusions apply to every segment, including metadata
// basenames: ignored declarations must never become ownership barriers.
func unityImportedPath(file string) bool { return extractor.UnityImportedPath(file) }

func unityMetadataInput(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".asmdef", ".asmref", ".meta":
		return true
	}
	return path.Base(file) == "ProjectVersion.txt" && path.Base(path.Dir(file)) == "ProjectSettings"
}

func unityUnderRoot(file, root string) bool { return root == "." || strings.HasPrefix(file, root+"/") }

func unityProjectRoots(files []unityAssemblyFile) []string {
	roots := map[string]bool{}
	for _, f := range files {
		if path.Base(f.path) == "ProjectVersion.txt" && path.Base(path.Dir(f.path)) == "ProjectSettings" {
			roots[path.Dir(path.Dir(f.path))] = true
		}
		parts := strings.Split(f.path, "/")
		for i, part := range parts[:len(parts)-1] {
			if strings.EqualFold(part, "Assets") {
				root := "."
				if i > 0 {
					root = strings.Join(parts[:i], "/")
				}
				roots[root] = true
			}
		}
	}
	if len(roots) == 0 {
		roots["."] = true
	}
	out := make([]string, 0, len(roots))
	for root := range roots {
		out = append(out, root)
	}
	sort.Strings(out)
	return out
}

func unityRootFor(file string, roots []string) string {
	best := ""
	for _, root := range roots {
		if unityUnderRoot(file, root) && (best == "" || best == "." || len(root) > len(best)) {
			best = root
		}
	}
	return best
}

func resolveUnityProject(service, root string, files []unityAssemblyFile) (unityAssemblyResult, error) {
	var out unityAssemblyResult
	x, err := extractor.NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		return out, err
	}
	byPath := map[string]*unityDefinition{}
	byName := map[string][]*unityDefinition{}
	byGUID := map[string][]*unityDefinition{}
	barriers := map[string][]string{}
	redirects := map[string]string{}
	var sources []string
	for _, f := range files {
		ext := path.Ext(f.path)
		rel := strings.TrimPrefix(f.path, root+"/")
		if ext == ".cs" && (strings.HasPrefix(strings.ToLower(rel), "assets/") || strings.HasPrefix(strings.ToLower(rel), "packages/")) {
			sources = append(sources, f.path)
		}
		if ext != ".asmdef" && ext != ".asmref" {
			continue
		}
		barriers[path.Dir(f.path)] = append(barriers[path.Dir(f.path)], f.path)
		if f.fault != "" {
			out.problems = append(out.problems, f.path+": "+f.fault)
			if ext == ".asmref" {
				out.unresolved++
			}
			continue
		}
		if ext == ".asmref" {
			var ref struct {
				Reference string `json:"reference"`
			}
			if err := json.Unmarshal(f.content, &ref); err != nil || ref.Reference == "" {
				out.problems = append(out.problems, f.path+": invalid assembly reference")
				out.unresolved++
				continue
			}
			redirects[f.path] = ref.Reference
			continue
		}
		d := &unityDefinition{}
		if err := json.Unmarshal(f.content, &d.unityDefinitionJSON); err != nil || strings.TrimSpace(d.Name) == "" {
			out.problems = append(out.problems, f.path+": invalid assembly definition")
			continue
		}
		doc, err := x.ExtractDoc(service, f.path, f.content)
		if err != nil {
			return out, err
		}
		for _, n := range doc.Nodes {
			n = unityDerivedNode(n)
			if n.Kind == "UnityAssemblyDefinition" {
				d.node = n
			}
			if n.Kind == "UnityDefineConstraint" {
				d.constraints = append(d.constraints, n)
			}
		}
		if d.node.Name == "" {
			out.problems = append(out.problems, f.path+": no assembly declaration")
			continue
		}
		d.Normalize()
		byPath[f.path], byName[d.Name] = d, append(byName[d.Name], d)
	}
	for dir, declarations := range barriers {
		if len(declarations) > 1 {
			out.problems = append(out.problems, dir+": multiple assembly definitions/references: "+strings.Join(declarations, ", "))
		}
	}
	for name, defs := range byName {
		if len(defs) > 1 {
			paths := make([]string, 0, len(defs))
			for _, d := range defs {
				paths = append(paths, d.node.FilePath)
			}
			out.problems = append(out.problems, "duplicate assembly name "+name+": "+strings.Join(paths, ", "))
		}
	}
	for _, f := range files {
		if !strings.HasSuffix(f.path, ".asmdef.meta") {
			continue
		}
		d := byPath[strings.TrimSuffix(f.path, ".meta")]
		if d == nil {
			continue
		}
		if f.fault != "" {
			out.problems = append(out.problems, f.path+": "+f.fault)
			continue
		}
		guid := unityMetaGUID(f.content)
		if guid == "" {
			out.problems = append(out.problems, f.path+": missing or invalid guid")
			continue
		}
		byGUID[guid] = append(byGUID[guid], d)
	}
	resolve := func(ref string) *unityDefinition {
		matches := byName[ref]
		if strings.HasPrefix(ref, "GUID:") {
			guid := strings.TrimPrefix(ref, "GUID:")
			if !unityGUIDValid(guid) {
				return nil
			}
			matches = byGUID[strings.ToLower(guid)]
		}
		if len(matches) == 1 && len(byName[matches[0].Name]) == 1 && len(barriers[path.Dir(matches[0].node.FilePath)]) == 1 {
			return matches[0]
		}
		return nil
	}
	addEdge := func(from, to core.SymbolNode) {
		out.edges = append(out.edges, core.Edge{FromService: service, FromFile: from.FilePath, FromSymbol: from.Key(), ToSymbol: to.Key(), Kind: core.EdgeFileResolved})
	}
	owners := map[string]*unityDefinition{}
	for _, f := range files {
		if d := byPath[f.path]; d != nil {
			if len(byName[d.Name]) == 1 && len(barriers[path.Dir(f.path)]) == 1 {
				owners[f.path] = d
			}
			var unresolved []string
			for _, ref := range d.References {
				if target := resolve(ref); target != nil {
					addEdge(d.node, target.node)
				} else {
					unresolved = append(unresolved, ref)
					out.unresolved++
				}
			}
			d.node.Signature = d.Claim(unresolved, "")
			out.nodes = append(out.nodes, d.node)
			for _, n := range d.constraints {
				n.Signature = unityJSON(map[string]string{"evidence": "derived", "rules": unityRules, "expression": n.Name})
				out.nodes = append(out.nodes, n)
				addEdge(d.node, n)
			}
		}
		if ref, ok := redirects[f.path]; ok {
			d := resolve(ref)
			owners[f.path] = d
			doc, err := x.ExtractDoc(service, f.path, f.content)
			if err != nil {
				return out, err
			}
			var n core.SymbolNode
			for _, candidate := range doc.Nodes {
				if candidate.Kind == "UnityAssemblyReference" {
					n = candidate
					break
				}
			}
			if n.Name == "" {
				continue
			}
			unresolved := d == nil
			n.Signature = extractor.UnityReferenceClaim(ref, &unresolved, "")
			out.nodes = append(out.nodes, n)
			if d == nil {
				out.unresolved++
				out.problems = append(out.problems, f.path+": unresolved assembly reference "+ref)
			} else {
				addEdge(n, d.node)
			}
		}
	}
	predefined := map[string]core.SymbolNode{}
	marker := ""
	for _, f := range files {
		if f.path == path.Join(root, "ProjectSettings/ProjectVersion.txt") {
			marker = f.path
		}
	}
	for _, file := range sources {
		assembly, scope, status, reason := "", "unknown", "unknown", "package has no assembly definition"
		var target core.SymbolNode
		barrier := false
		for dir := path.Dir(file); dir != root && dir != "." && dir != "/"; dir = path.Dir(dir) {
			if defs := barriers[dir]; len(defs) > 0 {
				barrier, reason = true, "unresolved or ambiguous nearest assembly definition/reference"
				if len(defs) == 1 && owners[defs[0]] != nil {
					d := owners[defs[0]]
					assembly, scope, target = d.Name, d.scope(), d.node
				}
				break
			}
		}
		rel := strings.TrimPrefix(file, root+"/")
		if !barrier && strings.HasPrefix(strings.ToLower(rel), "assets/") {
			assembly, scope = unityPredefinedAssembly(rel)
			var ok bool
			target, ok = predefined[assembly]
			if !ok && marker != "" {
				target = core.SymbolNode{Service: service, FilePath: marker, Package: "unity-assemblies", Name: assembly, Kind: "UnityAssemblyDefinition", Tier: core.TierPreScan, Line: 1}
				target.Signature = unityJSON(map[string]string{"evidence": "derived", "rules": unityRules, "name": assembly, "scope": scope, "origin": "predefined"})
				predefined[assembly] = target
				out.nodes = append(out.nodes, target)
			}
		}
		if assembly != "" {
			status, reason = "known", ""
		}
		n := core.SymbolNode{Service: service, FilePath: file, Package: path.Base(file), Name: "unity:ownership", Kind: "UnityAssemblyOwnership", Tier: core.TierPreScan}
		n.Signature = unityJSON(map[string]string{"evidence": "derived", "rules": unityRules, "assembly": assembly, "scope": scope, "status": status, "reason": reason})
		out.nodes = append(out.nodes, n)
		if target.Name != "" {
			addEdge(n, target)
		}
	}
	sort.Slice(out.nodes, func(i, j int) bool {
		a, b := out.nodes[i], out.nodes[j]
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	sortEdges(out.edges)
	// Repeated declaration references are one graph edge, as for doc xrefs.
	if len(out.edges) > 1 {
		n := 1
		for _, e := range out.edges[1:] {
			if e != out.edges[n-1] {
				out.edges[n] = e
				n++
			}
		}
		out.edges = out.edges[:n]
	}
	sort.Strings(out.problems)
	return out, nil
}

func unityPredefinedAssembly(file string) (string, string) {
	parts := strings.Split(strings.ToLower(path.Dir(file)), "/")
	editor := false
	for _, part := range parts {
		if part == "editor" {
			editor = true
		}
	}
	firstpass := len(parts) > 1 && (parts[1] == "plugins" || parts[1] == "standard assets" || parts[1] == "pro standard assets" || parts[1] == "iphone standard assets")
	name, scope := "Assembly-CSharp", "player-and-editor"
	if editor {
		name, scope = name+"-Editor", "editor-only"
	}
	if firstpass {
		name += "-firstpass"
	}
	return name, scope
}

func unityGUIDValid(guid string) bool {
	if len(guid) != 32 {
		return false
	}
	_, err := hex.DecodeString(guid)
	return err == nil
}

func unityMetaGUID(content []byte) string {
	guid := ""
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "guid:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "guid:"))
		if guid != "" || !unityGUIDValid(v) {
			return ""
		}
		guid = strings.ToLower(v)
	}
	return guid
}

func unityJSON(v any) string {
	// Only concrete structs and maps of JSON scalars are passed here.
	b, _ := json.Marshal(v)
	return string(b)
}

// enrichUnityAssemblies uses existing observations and file-resolved edges.
// It replaces only this strategy's facts while retaining each file's C# nodes.
// There is no additional table, column, cache, pass-state row or background job.
func (p *Pipeline) enrichUnityAssemblies(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, now time.Time) (unityAssemblyResult, error) {
	var files []unityAssemblyFile
	var paths []string
	for _, ref := range refs {
		if !unityAssemblyInput(ref.Path) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return unityAssemblyResult{}, err
		}
		paths = append(paths, ref.Path)
		// Read old facts even for an ignored path so upgrades remove the earlier
		// resolver's Unity claims while preserving ordinary C# observations.
		if !unityImportedPath(ref.Path) {
			continue
		}
		f := unityAssemblyFile{path: ref.Path}
		if path.Ext(ref.Path) != ".cs" {
			b, err := p.conn.Fetch(ctx, ref)
			if h := haltError(ctx, err); h != nil {
				return unityAssemblyResult{}, h
			}
			if err != nil {
				f.fault = err.Error()
			} else if len(b) > MaxExtractSize {
				f.fault = "exceeds extraction size limit"
			} else if binaryExcluded(ref.Path, b) {
				f.fault = "binary assembly metadata"
			} else {
				f.content = b
			}
		}
		files = append(files, f)
	}
	result, err := ResolveUnityAssemblies(svc.ID, files)
	if err != nil {
		return result, err
	}
	fis, ok := p.st.(store.FileImportsStore)
	if !ok {
		if len(result.nodes) == 0 {
			return result, nil
		}
		return result, fmt.Errorf("store cannot read source observations")
	}
	stored, err := fis.SourceObservations(ctx, svc.ID, paths)
	if err != nil {
		return result, err
	}
	old := map[string][]core.Observation{}
	previous := map[string][]core.Observation{}
	for _, o := range stored {
		if strings.HasPrefix(string(o.SourceKind), "Unity") {
			previous[o.SourceRef] = append(previous[o.SourceRef], o)
		} else {
			old[o.SourceRef] = append(old[o.SourceRef], o)
		}
	}
	facts := map[string][]core.Observation{}
	for _, n := range result.nodes {
		hash, _, err := p.st.Fingerprint(ctx, svc.ID, n.FilePath)
		if err != nil {
			return result, err
		}
		facts[n.FilePath] = append(facts[n.FilePath], core.Observation{ServiceID: svc.ID, SourceRef: n.FilePath, SourceKind: n.Kind, EvidenceTier: core.TierPreScan, Symbol: n.Name, Package: n.Package, Claim: n.Signature, Line: n.Line, EndLine: n.EndLine, ContentHash: hash, IngestedAt: now, ToolKind: "unity-asmdef"})
	}
	sort.Strings(paths)
	for _, file := range paths {
		if unityObservationsEqual(previous[file], facts[file]) {
			continue
		}
		if err := p.st.PurgeSource(ctx, svc.ID, file); err != nil {
			return result, err
		}
		if err := p.st.PutObservations(ctx, append(old[file], facts[file]...)); err != nil {
			return result, err
		}
	}
	if !p.cfg.DisableTypedEdges {
		if err := p.st.PutTypedEdges(ctx, svc.ID, unityAssemblyEdgeKey, result.edges); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Ignore receipt timestamps while comparing facts. A no-op refresh must leave
// rows (and their IDs used by derived tables) untouched.
func unityObservationsEqual(a, b []core.Observation) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(o core.Observation) string { o.IngestedAt = time.Time{}; return unityJSON(o) }
	left, right := make([]string, len(a)), make([]string, len(b))
	for i := range a {
		left[i] = key(a[i])
		right[i] = key(b[i])
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// No pass-state row or cache: cheaply reject non-Unity inventories, but still
// reconcile ownership when the last Unity metadata file was deleted.
func (p *Pipeline) enrichUnityAssembliesChanged(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string, now time.Time) (unityAssemblyResult, error) {
	needed := false
	for _, file := range changed {
		if unityMetadataInput(file) {
			needed = true
			break
		}
	}
	if !needed {
		for _, ref := range refs {
			if unityMetadataInput(ref.Path) && path.Ext(ref.Path) != ".meta" {
				needed = true
				break
			}
		}
	}
	if !needed {
		return unityAssemblyResult{}, nil
	}
	result, err := p.enrichUnityAssemblies(ctx, svc, refs, now)
	result.ran = true
	return result, err
}

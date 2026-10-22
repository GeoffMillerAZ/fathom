package fusion

// Unity assembly ownership is an L3 file-resolution strategy over the doc
// dialect, not C# type checking. The rules follow the Unity 2021+ Manual:
// https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompilationAssemblyDefinitionFiles.html
// https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompileOrderFolders.html
// CompilationPipeline is the independent oracle, run by the consumer later.
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

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const unityAssemblyEdgeKey = "unity-assembly"
const unityRules = "Unity 2021+"

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
}

type unityVersionDefine struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Define     string `json:"define"`
}

type unityDefinitionJSON struct {
	Name                  string               `json:"name"`
	RootNamespace         string               `json:"rootNamespace"`
	References            []string             `json:"references"`
	IncludePlatforms      []string             `json:"includePlatforms"`
	ExcludePlatforms      []string             `json:"excludePlatforms"`
	AllowUnsafeCode       bool                 `json:"allowUnsafeCode"`
	OverrideReferences    bool                 `json:"overrideReferences"`
	PrecompiledReferences []string             `json:"precompiledReferences"`
	AutoReferenced        *bool                `json:"autoReferenced"`
	DefineConstraints     []string             `json:"defineConstraints"`
	VersionDefines        []unityVersionDefine `json:"versionDefines"`
	NoEngineReferences    bool                 `json:"noEngineReferences"`
}

type unityDefinition struct {
	unityDefinitionJSON
	node        core.SymbolNode
	constraints []core.SymbolNode
}

func (d *unityDefinition) scope() string {
	if len(d.IncludePlatforms) == 1 && d.IncludePlatforms[0] == "Editor" {
		return "editor-only"
	}
	return "player-and-editor"
}

func unityAssemblyInput(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".cs", ".asmdef", ".asmref":
		return true
	case ".meta":
		return strings.HasSuffix(strings.ToLower(p), ".asmdef.meta")
	}
	return p == "ProjectSettings/ProjectVersion.txt"
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
	files := append([]unityAssemblyFile(nil), input...)
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	active := false
	for _, f := range files {
		if path.Ext(f.path) == ".asmdef" || path.Ext(f.path) == ".asmref" || f.path == "ProjectSettings/ProjectVersion.txt" {
			active = true
		}
	}
	if !active {
		return out, nil
	}
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
		if ext == ".cs" && (strings.HasPrefix(f.path, "Assets/") || strings.HasPrefix(f.path, "Packages/")) {
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
		if d.AutoReferenced == nil {
			v := true
			d.AutoReferenced = &v
		}
		for _, values := range [][]string{d.References, d.IncludePlatforms, d.ExcludePlatforms, d.PrecompiledReferences, d.DefineConstraints} {
			sort.Strings(values)
		}
		sort.Slice(d.VersionDefines, func(i, j int) bool {
			a, b := d.VersionDefines[i], d.VersionDefines[j]
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			if a.Expression != b.Expression {
				return a.Expression < b.Expression
			}
			return a.Define < b.Define
		})
		byPath[f.path], byName[d.Name] = d, append(byName[d.Name], d)
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
		if len(matches) == 1 {
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
			owners[f.path] = d
			var unresolved []string
			for _, ref := range d.References {
				if target := resolve(ref); target != nil {
					addEdge(d.node, target.node)
				} else {
					unresolved = append(unresolved, ref)
					out.unresolved++
				}
			}
			fact := struct {
				unityDefinitionJSON
				Evidence        string   `json:"evidence"`
				Rules           string   `json:"rules"`
				Scope           string   `json:"scope"`
				Unresolved      int      `json:"unresolvedReferences"`
				UnresolvedNames []string `json:"unresolvedReferenceNames"`
			}{d.unityDefinitionJSON, "derived", unityRules, d.scope(), len(unresolved), unresolved}
			d.node.Signature = unityJSON(fact)
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
			n := core.SymbolNode{Service: service, FilePath: f.path, Package: path.Base(f.path), Name: ref, Kind: "UnityAssemblyReference", Tier: core.TierPreScan, Line: 1}
			n.Signature = unityJSON(map[string]any{"evidence": "derived", "rules": unityRules, "reference": ref, "unresolved": d == nil})
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
	for _, file := range sources {
		assembly, scope, status, reason := "", "unknown", "unknown", "package has no assembly definition"
		var target core.SymbolNode
		barrier := false
		for dir := path.Dir(file); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if defs := barriers[dir]; len(defs) > 0 {
				barrier, reason = true, "unresolved or ambiguous nearest assembly definition/reference"
				if len(defs) == 1 && owners[defs[0]] != nil {
					d := owners[defs[0]]
					assembly, scope, target = d.Name, d.scope(), d.node
				}
				break
			}
		}
		if !barrier && strings.HasPrefix(file, "Assets/") {
			assembly, scope = unityPredefinedAssembly(file)
			var ok bool
			target, ok = predefined[assembly]
			if !ok {
				target = core.SymbolNode{Service: service, FilePath: file, Package: "unity-assemblies", Name: assembly, Kind: "UnityAssemblyDefinition", Tier: core.TierPreScan}
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
		if assembly != "" {
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
	parts := strings.Split(path.Dir(file), "/")
	editor := false
	for _, part := range parts {
		if part == "Editor" {
			editor = true
		}
	}
	firstpass := len(parts) > 1 && (parts[1] == "Plugins" || parts[1] == "Standard Assets" || parts[1] == "Pro Standard Assets")
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
		f := unityAssemblyFile{path: ref.Path}
		paths = append(paths, ref.Path)
		if path.Ext(ref.Path) != ".cs" {
			b, err := p.conn.Fetch(ctx, ref)
			if h := haltError(ctx, err); h != nil {
				return unityAssemblyResult{}, h
			}
			if err != nil {
				f.fault = err.Error()
			} else if len(b) > MaxExtractSize {
				f.fault = "exceeds extraction size limit"
			} else if connector.IsBinary(b) {
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
	hadUnity := map[string]bool{}
	for _, o := range stored {
		if strings.HasPrefix(string(o.SourceKind), "Unity") {
			hadUnity[o.SourceRef] = true
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
		if !hadUnity[file] && len(facts[file]) == 0 {
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

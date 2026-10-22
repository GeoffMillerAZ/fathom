package extractor

// Shared declaration construction keeps the dialect, full ownership pass and
// single-file metadata refresh on the same identity and derived evidence tier.
import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const UnityAssemblyRules = "documented behavior of Unity 2021 and later; native CompilationPipeline calibration on Unity 6000.3 pending"
const UnityMetadataRefreshNote = "Unity metadata refresh needs a full ingest"

type UnityVersionDefine struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Define     string `json:"define"`
}

type UnityDefinitionJSON struct {
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
	VersionDefines        []UnityVersionDefine `json:"versionDefines"`
	NoEngineReferences    bool                 `json:"noEngineReferences"`
}

func (d *UnityDefinitionJSON) Normalize() {
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
}

// Scope describes declared eligibility, never evaluates a build target/define.
func (d *UnityDefinitionJSON) Scope() string {
	if len(d.IncludePlatforms) == 1 && d.IncludePlatforms[0] == "Editor" {
		return "editor-only"
	}
	for _, p := range d.ExcludePlatforms {
		if p == "Editor" {
			return "player-only"
		}
	}
	if len(d.IncludePlatforms) > 0 {
		for _, p := range d.IncludePlatforms {
			if p == "Editor" {
				return "player-and-editor"
			}
		}
		return "player-only"
	}
	return "player-and-editor"
}

func (d *UnityDefinitionJSON) Claim(unresolved []string, note string) string {
	var count *int
	if note == "" {
		n := len(unresolved)
		count = &n
	}
	b, _ := json.Marshal(struct {
		*UnityDefinitionJSON
		Evidence        string   `json:"evidence"`
		Rules           string   `json:"rules"`
		Scope           string   `json:"scope"`
		Unresolved      *int     `json:"unresolvedReferences,omitempty"`
		UnresolvedNames []string `json:"unresolvedReferenceNames"`
		RefreshNote     string   `json:"refreshNote,omitempty"`
	}{d, "derived", UnityAssemblyRules, d.Scope(), count, unresolved, note})
	return string(b)
}

func UnityReferenceClaim(ref string, unresolved *bool, note string) string {
	b, _ := json.Marshal(struct {
		Evidence    string `json:"evidence"`
		Rules       string `json:"rules"`
		Reference   string `json:"reference"`
		Unresolved  *bool  `json:"unresolved,omitempty"`
		RefreshNote string `json:"refreshNote,omitempty"`
	}{"derived", UnityAssemblyRules, ref, unresolved, note})
	return string(b)
}

// unityMetadataNodes enriches the doc dialect's nodes without changing keys or
// source spans. Cross-file claims are explicitly provisional on this path.
func unityMetadataNodes(nodes []core.SymbolNode, relPath string, content []byte) []core.SymbolNode {
	if !UnityImportedPath(relPath) {
		return nil
	}
	var d UnityDefinitionJSON
	if json.Unmarshal(content, &d) != nil {
		return nil
	}
	d.Normalize()
	for i := range nodes {
		n := &nodes[i]
		n.Tier = core.TierPreScan
		n.ToolKind = "unity-asmdef"
		switch n.Kind {
		case "UnityAssemblyDefinition":
			n.Signature = d.Claim(nil, UnityMetadataRefreshNote)
		case "UnityAssemblyReference":
			n.Signature = UnityReferenceClaim(n.Name, nil, UnityMetadataRefreshNote)
		case "UnityDefineConstraint":
			b, _ := json.Marshal(map[string]string{"evidence": "derived", "rules": UnityAssemblyRules, "expression": n.Name})
			n.Signature = string(b)
		}
	}
	return nodes
}

// UnityImportedPath applies Unity's asset import exclusions to every segment.
func UnityImportedPath(file string) bool {
	for part := range strings.SplitSeq(file, "/") {
		lower := strings.ToLower(part)
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, "~") || lower == "cvs" || strings.HasSuffix(lower, ".tmp") {
			return false
		}
	}
	return true
}

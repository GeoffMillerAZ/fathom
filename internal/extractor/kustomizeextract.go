package extractor

import (
	"bytes"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// KustomizeExtractor implements Extractor for kustomization.yaml / .yml
// files (ADR-021 §2 / TI-3).
//
// Grounding: kustomization.yaml is valid k8s-shaped YAML (it has apiVersion:
// and kind: Kustomization) but essentially never has metadata.name, so
// routing it through K8sExtractor — which only emits a workload node when
// both kind AND metadata.name are present — silently produced zero symbols
// for every kustomization file in the fleet. This extractor reads it for
// what it actually is: a manifest of resource/patch/image references.
//
// Output (Tier = TierVerified, real YAML parse):
//   - KindKustomizeResource — one per entry under `resources:` or the legacy
//     `bases:` key. Name = the resource path/reference.
//   - KindKustomizePatch — one per entry under `patches:`,
//     `patchesStrategicMerge:`, or `patchesJson6902:`. Name = the patch file
//     path; Signature = "kind/name" of the patch target when the modern
//     `patches:` form specifies one.
//   - KindKustomizeImage — one per entry under `images:`. Name = the image's
//     original `name`; Signature = the replacement "newName:newTag" (or
//     "name:newTag" / "...@digest" when newName is absent).
//
// Extract never returns a non-nil error; malformed YAML yields zero nodes.
type KustomizeExtractor struct{}

// NewKustomizeExtractor returns a new KustomizeExtractor.
func NewKustomizeExtractor() *KustomizeExtractor { return &KustomizeExtractor{} }

// kustomizeImageEntry is the typed shape of one `images:` list entry.
type kustomizeImageEntry struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
	Digest  string `yaml:"digest"`
}

// kustomizeJSON6902Entry is the typed shape of one modern `patches:` (or
// legacy `patchesJson6902:`) list entry when it is a mapping rather than a
// bare path string.
type kustomizeJSON6902Entry struct {
	Path   string `yaml:"path"`
	Target struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`
	} `yaml:"target"`
}

// Extract parses src as a kustomization manifest and returns SymbolNodes for
// its resources, patches, and image overrides.
func (k *KustomizeExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}

	var nodes []core.SymbolNode
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		val := root.Content[i+1]
		switch key {
		case "resources", "bases":
			nodes = append(nodes, kustomizeStringListNodes(val, service, pkg, relPath, core.KindKustomizeResource)...)
		case "patchesStrategicMerge":
			nodes = append(nodes, kustomizeStringListNodes(val, service, pkg, relPath, core.KindKustomizePatch)...)
		case "patches", "patchesJson6902":
			nodes = append(nodes, kustomizePatchesNodes(val, service, pkg, relPath)...)
		case "images":
			nodes = append(nodes, kustomizeImagesNodes(val, service, pkg, relPath)...)
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// kustomizeStringListNodes converts a sequence of bare scalar strings (the
// `resources:`, `bases:`, and `patchesStrategicMerge:` shapes) into nodes of
// kind.
func kustomizeStringListNodes(seq *yaml.Node, service, pkg, relPath string, kind core.SourceKind) []core.SymbolNode {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	var nodes []core.SymbolNode
	for _, item := range seq.Content {
		if item.Kind != yaml.ScalarNode || item.Value == "" {
			continue
		}
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     item.Value,
			Kind:     kind,
			FilePath: relPath,
			Line:     item.Line,
			Tier:     core.TierVerified,
		})
	}
	return nodes
}

// kustomizePatchesNodes handles the `patches:` / `patchesJson6902:` shape,
// whose entries may be a bare path string OR a mapping with `path:` and an
// optional `target:`.
func kustomizePatchesNodes(seq *yaml.Node, service, pkg, relPath string) []core.SymbolNode {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	var nodes []core.SymbolNode
	for _, item := range seq.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			if item.Value == "" {
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service: service, Package: pkg, Name: item.Value,
				Kind: core.KindKustomizePatch, FilePath: relPath, Line: item.Line,
				Tier: core.TierVerified,
			})
		case yaml.MappingNode:
			var entry kustomizeJSON6902Entry
			if err := item.Decode(&entry); err != nil || entry.Path == "" {
				continue
			}
			sig := entry.Target.Kind
			if entry.Target.Name != "" {
				if sig != "" {
					sig += "/"
				}
				sig += entry.Target.Name
			}
			nodes = append(nodes, core.SymbolNode{
				Service: service, Package: pkg, Name: entry.Path,
				Kind: core.KindKustomizePatch, FilePath: relPath, Line: item.Line,
				Signature: sig, Tier: core.TierVerified,
			})
		}
	}
	return nodes
}

// kustomizeImagesNodes handles the `images:` shape: a sequence of mappings
// with name/newName/newTag/digest.
func kustomizeImagesNodes(seq *yaml.Node, service, pkg, relPath string) []core.SymbolNode {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	var nodes []core.SymbolNode
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		var entry kustomizeImageEntry
		if err := item.Decode(&entry); err != nil || entry.Name == "" {
			continue
		}
		sig := entry.NewName
		if entry.NewTag != "" {
			if sig == "" {
				sig = entry.Name
			}
			sig += ":" + entry.NewTag
		}
		if entry.Digest != "" {
			if sig != "" {
				sig += "@"
			}
			sig += entry.Digest
		}
		nodes = append(nodes, core.SymbolNode{
			Service: service, Package: pkg, Name: entry.Name,
			Kind: core.KindKustomizeImage, FilePath: relPath, Line: item.Line,
			Signature: sig, Tier: core.TierVerified,
		})
	}
	return nodes
}

// ensure KustomizeExtractor satisfies the Extractor interface at compile time.
var _ Extractor = (*KustomizeExtractor)(nil)

package extractor

import (
	"bytes"
	"io"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// K8sExtractor implements Extractor for Kubernetes YAML manifests. It parses a
// stream of YAML documents (separated by "---"), extracting workload metadata
// and container image references from each document.
//
// Output:
//   - KindK8sWorkload  — one per document that has kind + metadata.name.
//     Name = "Kind/namespace/name" (namespace omitted when absent → "Kind/name").
//     Signature = apiVersion. Line = absolute line of the document start.
//     Tier = TierVerified (real YAML parse).
//   - KindK8sContainer — one per container/initContainer entry with an image.
//     Name = image string. Signature = container name. Tier = TierVerified.
//
// Best-effort: documents that fail to decode (e.g. Helm mustache templates) are
// silently skipped; Extract never returns a non-nil error.
type K8sExtractor struct{}

// NewK8sExtractor returns a new K8sExtractor.
func NewK8sExtractor() *K8sExtractor { return &K8sExtractor{} }

// Per-document tool-identity values set on SymbolNode.ToolKind (ADR-021 §2,
// critic P15): tool identity is a per-DOCUMENT fact in a multi-document YAML
// stream (one file can hold a CRD and a Deployment), so the fusion
// classifier's per-file verdict is refined here, where document boundaries
// are visible. The string values mirror fusion's ToolK8sManifest/ToolK8sCRD
// constants — extractor cannot import fusion (fusion imports extractor), so
// the parity is pinned by a test in internal/fusion instead
// (TestK8sExtractorToolKindParity).
const (
	ToolKindK8sManifest = "k8s"
	ToolKindK8sCRD      = "k8s-crd"
)

// k8sDocToolKind returns the per-document tool identity for a decoded doc:
// apiextensions.k8s.io apiVersions are CRDs, everything else is a plain
// manifest.
func k8sDocToolKind(apiVersion string) string {
	if strings.HasPrefix(apiVersion, "apiextensions.k8s.io") {
		return ToolKindK8sCRD
	}
	return ToolKindK8sManifest
}

// k8sDoc is the typed subset of a Kubernetes manifest we care about.
type k8sDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec k8sSpec `yaml:"spec"`
}

// k8sSpec covers the three container slice locations we need to traverse.
type k8sSpec struct {
	// Pod-level: kind: Pod.
	Containers     []k8sContainer `yaml:"containers"`
	InitContainers []k8sContainer `yaml:"initContainers"`
	// Deployment/StatefulSet/DaemonSet/Job/CronJob etc: spec.template.spec.
	Template struct {
		Spec struct {
			Containers     []k8sContainer `yaml:"containers"`
			InitContainers []k8sContainer `yaml:"initContainers"`
		} `yaml:"spec"`
	} `yaml:"template"`
}

type k8sContainer struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

// k8sChunk is one YAML document text chunk with its starting line number (1-based)
// in the original file.
type k8sChunk struct {
	text      string
	lineStart int // 1-based line number of the first line of this chunk in the original
}

// Extract parses src as a stream of YAML documents and returns SymbolNodes.
// Each document is decoded independently so a malformed document (e.g. a Helm
// template with mustache expressions) is silently skipped without affecting
// the rest of the file.
func (k *K8sExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode
	pkg := pkgFromPath(relPath)

	for _, chunk := range k8sSplitDocs(src) {
		dec := yaml.NewDecoder(bytes.NewReader([]byte(chunk.text)))
		var rawNode yaml.Node
		err := dec.Decode(&rawNode)
		if err != nil {
			if err == io.EOF {
				continue
			}
			// Malformed document — skip it (best-effort).
			continue
		}
		if len(rawNode.Content) == 0 {
			continue
		}

		// Absolute line number of the document-mapping node in the original file.
		docLine := chunk.lineStart + rawNode.Line - 1

		// Decode into our typed struct (best-effort — ignore partial decode errors).
		var doc k8sDoc
		if decErr := rawNode.Decode(&doc); decErr != nil {
			continue
		}

		docToolKind := k8sDocToolKind(doc.APIVersion)

		// Emit workload node.
		if doc.Kind != "" && doc.Metadata.Name != "" {
			symName := doc.Kind + "/" + doc.Metadata.Name
			if doc.Metadata.Namespace != "" {
				symName = doc.Kind + "/" + doc.Metadata.Namespace + "/" + doc.Metadata.Name
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      symName,
				Kind:      core.KindK8sWorkload,
				FilePath:  relPath,
				Line:      docLine,
				Signature: doc.APIVersion,
				Tier:      core.TierVerified,
				ToolKind:  docToolKind,
			})
		}

		// Collect containers from all candidate locations.
		var containers []k8sContainer
		containers = append(containers, doc.Spec.Containers...)
		containers = append(containers, doc.Spec.InitContainers...)
		containers = append(containers, doc.Spec.Template.Spec.Containers...)
		containers = append(containers, doc.Spec.Template.Spec.InitContainers...)

		for _, c := range containers {
			if c.Image == "" {
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      c.Image,
				Kind:      core.KindK8sContainer,
				FilePath:  relPath,
				Line:      docLine,
				Signature: c.Name,
				Tier:      core.TierVerified,
				ToolKind:  docToolKind,
			})
		}
	}

	// Deterministic output: sort by Line then Name.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// k8sSplitDocs splits YAML bytes into per-document chunks on "---" separator
// lines, tracking each chunk's starting line number in the original file.
func k8sSplitDocs(src []byte) []k8sChunk {
	var chunks []k8sChunk
	var current strings.Builder
	lineNum := 1
	chunkStart := 1

	for _, rawLine := range bytes.Split(src, []byte("\n")) {
		s := strings.TrimSpace(string(rawLine))
		if s == "---" {
			if current.Len() > 0 {
				chunks = append(chunks, k8sChunk{text: current.String(), lineStart: chunkStart})
				current.Reset()
			}
			lineNum++
			chunkStart = lineNum
			continue
		}
		current.WriteString(string(rawLine))
		current.WriteByte('\n')
		lineNum++
	}
	if current.Len() > 0 {
		chunks = append(chunks, k8sChunk{text: current.String(), lineStart: chunkStart})
	}
	return chunks
}

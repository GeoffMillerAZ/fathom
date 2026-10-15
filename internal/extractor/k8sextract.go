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
//   - KindK8sContainer — one per container/initContainer entry with an image,
//     from all three pod paths (a CronJob's job template included, LB-F451).
//     Name = image string. Signature = container name. Tier = TierVerified.
//   - core.KindK8sFact — one per fact the workload writes (LB-F451,
//     k8sContainerFacts): Name = "<workload>#<key>", Signature = the value as
//     written, Line = EndLine = the key's line.
//   - KindTelemetryIdentity — one per telemetry identity a container's env
//     declares literally (LB-F314, TL-5; k8sTelemetryIdentities).
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

// k8sSpec covers the three pod paths whose container lists we traverse.
type k8sSpec struct {
	// Pod-level: kind: Pod.
	Containers     []k8sContainer `yaml:"containers"`
	InitContainers []k8sContainer `yaml:"initContainers"`
	// Deployment/StatefulSet/DaemonSet/ReplicaSet/Job etc: spec.template.spec.
	Template struct {
		Spec k8sPodContainers `yaml:"spec"`
	} `yaml:"template"`
	// CronJob: spec.jobTemplate.spec.template.spec (LB-F451). Read by this
	// extractor only: the Helm template extractor's image collector keeps the
	// two paths above (helmtemplateyaml.go).
	JobTemplate struct {
		Spec struct {
			Template struct {
				Spec k8sPodContainers `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	} `yaml:"jobTemplate"`
}

// k8sPodContainers is a pod spec's two container lists.
type k8sPodContainers struct {
	Containers     []k8sContainer `yaml:"containers"`
	InitContainers []k8sContainer `yaml:"initContainers"`
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
	identities := map[string]bool{}

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
			for _, f := range k8sContainerFacts(rawNode.Content[0], doc.Kind) {
				line := chunk.lineStart + f.line - 1
				nodes = append(nodes, core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      symName + "#" + f.key,
					Kind:      core.KindK8sFact,
					FilePath:  relPath,
					Line:      line,
					EndLine:   line,
					Signature: f.value,
					Tier:      core.TierVerified,
					ToolKind:  docToolKind,
				})
			}
		}

		// Collect containers from all three pod paths.
		var containers []k8sContainer
		containers = append(containers, doc.Spec.Containers...)
		containers = append(containers, doc.Spec.InitContainers...)
		containers = append(containers, doc.Spec.Template.Spec.Containers...)
		containers = append(containers, doc.Spec.Template.Spec.InitContainers...)
		containers = append(containers, doc.Spec.JobTemplate.Spec.Template.Spec.Containers...)
		containers = append(containers, doc.Spec.JobTemplate.Spec.Template.Spec.InitContainers...)

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

		for _, id := range k8sTelemetryIdentities(rawNode.Content[0]) {
			if k := id.scheme + "=" + id.name; identities[k] {
				continue // one node per identity a file declares
			} else {
				identities[k] = true
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      id.name,
				Kind:      KindTelemetryIdentity,
				FilePath:  relPath,
				Line:      chunk.lineStart + id.line - 1,
				Signature: id.scheme + "=" + id.name,
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

// KindTelemetryIdentity is TL-5's source kind (docs/design/telemetry-links.md
// §4): a telemetry identity a repository declares, named by the literal
// value and signed `<scheme>=<value>`. The doc-engine dialects emit the same
// string as data; the Kubernetes incumbent emits it here.
const KindTelemetryIdentity core.SourceKind = "TelemetryIdentity"

// The identity schemes a Kubernetes container's environment declares.
const (
	telemetrySchemeServiceName = "service.name"
	telemetrySchemeNewRelicApp = "newrelic.app_name"
)

// k8sIdentity is one telemetry identity and the line of the env entry that
// declares it (relative to the document).
type k8sIdentity struct {
	name, scheme string
	line         int
}

// k8sTelemetryIdentities (LB-F314, TL-5) reads the telemetry identities a
// workload's containers declare in their environment: OTEL_SERVICE_NAME,
// the service.name entry of OTEL_RESOURCE_ATTRIBUTES
// (`k=v,k=v`, the OpenTelemetry environment variable specification), and
// NEW_RELIC_APP_NAME, whose semicolon-separated names are each one (New
// Relic's rollup names). Only a literal value is one: a `$(VAR)` (the
// kubelet's own substitution), a `${VAR}` or a `{{ }}` template and a
// valueFrom are decided at deploy time and yield none. The pod specs read are
// a Pod's, a workload's template and a CronJob's job template; containers and
// init containers both. Deterministic: document order, each (scheme, name)
// once.
func k8sTelemetryIdentities(root *yaml.Node) []k8sIdentity {
	var out []k8sIdentity
	seen := map[[2]string]bool{}
	add := func(name, scheme string, line int) {
		name = strings.TrimSpace(name)
		if !k8sLiteral(name) || seen[[2]string{scheme, name}] {
			return
		}
		seen[[2]string{scheme, name}] = true
		out = append(out, k8sIdentity{name: name, scheme: scheme, line: line})
	}
	for _, pod := range []*yaml.Node{
		k8sYPath(root, "spec"),
		k8sYPath(root, "spec", "template", "spec"),
		k8sYPath(root, "spec", "jobTemplate", "spec", "template", "spec"),
	} {
		for _, list := range []string{"initContainers", "containers"} {
			for _, c := range k8sYSeq(k8sYPath(pod, list)) {
				for _, e := range k8sYSeq(k8sYPath(c, "env")) {
					val := k8sYPath(e, "value")
					if val == nil || val.Kind != yaml.ScalarNode || k8sYPath(e, "valueFrom") != nil {
						continue
					}
					switch name := k8sYScalar(k8sYPath(e, "name")); name {
					case "OTEL_SERVICE_NAME":
						add(val.Value, telemetrySchemeServiceName, e.Line)
					case "OTEL_RESOURCE_ATTRIBUTES":
						for _, kv := range strings.Split(val.Value, ",") {
							if k, v, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(k) == "service.name" {
								add(v, telemetrySchemeServiceName, e.Line)
							}
						}
					case "NEW_RELIC_APP_NAME":
						for _, n := range strings.Split(val.Value, ";") {
							add(n, telemetrySchemeNewRelicApp, e.Line)
						}
					}
				}
			}
		}
	}
	return out
}

// k8sLiteral: a value that is not decided at deploy time (TL-5: a template
// or a variable yields no identity).
func k8sLiteral(v string) bool {
	if v == "" {
		return false
	}
	for _, m := range []string{"$(", "${", "{{"} {
		if strings.Contains(v, m) {
			return false
		}
	}
	return true
}

func k8sYPath(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == k {
				next = n.Content[i+1]
				break
			}
		}
		n = next
	}
	return n
}

func k8sYSeq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func k8sYScalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

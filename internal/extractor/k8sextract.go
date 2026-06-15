package extractor

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// K8sExtractor implements Extractor for Kubernetes YAML manifests. It splits
// multi-document YAML on "---" separators and extracts the kind: and
// metadata.name: (plus optional metadata.namespace:) from each document.
// Each manifest → KindK8sWorkload, Name = "kind/namespace/name" or "kind/name".
type K8sExtractor struct{}

// NewK8sExtractor returns a new K8sExtractor.
func NewK8sExtractor() *K8sExtractor { return &K8sExtractor{} }

var (
	k8sKindRe      = regexp.MustCompile(`(?i)^kind:\s*(\S+)`)
	k8sNameRe      = regexp.MustCompile(`(?i)^\s{0,2}name:\s*(\S+)`)
	k8sNamespaceRe = regexp.MustCompile(`(?i)^\s{0,2}namespace:\s*(\S+)`)
)

// Extract splits the YAML src on "---" and processes each document.
func (k *K8sExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode
	pkg := pkgFromPath(relPath)

	// Split into documents on "---" separator lines.
	docs := splitYAMLDocs(src)

	startLine := 1
	for _, doc := range docs {
		docLines := strings.Split(doc, "\n")
		docLen := len(docLines)

		kind, name, namespace := "", "", ""
		inMetadata := false

		for i, line := range docLines {
			stripped := strings.TrimSpace(line)
			if stripped == "" || strings.HasPrefix(stripped, "#") {
				continue
			}

			// Detect 'metadata:' block start (unindented or top-level key).
			if strings.HasPrefix(line, "metadata:") {
				inMetadata = true
				continue
			}
			// Detect end of metadata block: another top-level key.
			if inMetadata && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(line, "metadata:") {
				inMetadata = false
			}

			if m := k8sKindRe.FindStringSubmatch(line); m != nil {
				kind = m[1]
			}

			if inMetadata {
				if name == "" {
					if m := k8sNameRe.FindStringSubmatch(line); m != nil {
						name = m[1]
					}
				}
				if namespace == "" {
					if m := k8sNamespaceRe.FindStringSubmatch(line); m != nil {
						namespace = m[1]
					}
				}
			}
			_ = i
		}

		if kind != "" && name != "" {
			symName := kind + "/" + name
			if namespace != "" {
				symName = kind + "/" + namespace + "/" + name
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     symName,
				Kind:     core.KindK8sWorkload,
				FilePath: relPath,
				Line:     startLine,
				Tier:     core.TierPreScan,
			})
		}

		startLine += docLen + 1 // +1 for the "---" separator line
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// splitYAMLDocs splits YAML bytes into individual documents on "---" separator
// lines. The separator lines themselves are not included in the returned docs.
func splitYAMLDocs(src []byte) []string {
	var docs []string
	var current strings.Builder

	for _, line := range bytes.Split(src, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if s == "---" {
			if current.Len() > 0 {
				docs = append(docs, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(string(line))
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		docs = append(docs, current.String())
	}
	return docs
}

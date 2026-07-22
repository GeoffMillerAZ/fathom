package extractor

import (
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// httpMethodKeys are the OpenAPI/Swagger path-item keys that denote an
// operation (as opposed to shared fields like `parameters:` or `$ref:`).
var httpMethodKeys = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// OpenAPIExtractor implements Extractor for OpenAPI v3 / Swagger v2 YAML
// documents, detected by a top-level `openapi:` or `swagger:` key (ADR-021
// §2 / TI-3 — see the fusion pass-3 classifier). Before this extractor
// existed, such documents fell through to the generic k8s extractor via
// path/extension dispatch alone and silently produced zero symbols (no
// apiVersion/kind top-level keys to match).
//
// Output (Tier = TierVerified, real YAML parse):
//   - KindOpenAPIOperation — one per (method, path) pair under `paths:`.
//     Name = "METHOD /path" (method upper-cased). Signature = operationId,
//     falling back to summary when operationId is absent.
//   - KindOpenAPISchema — one per entry under `components.schemas:` (v3) or
//     the top-level `definitions:` (v2). Name = schema name.
//
// Extract never returns a non-nil error; malformed YAML yields zero nodes.
type OpenAPIExtractor struct{}

// NewOpenAPIExtractor returns a new OpenAPIExtractor.
func NewOpenAPIExtractor() *OpenAPIExtractor { return &OpenAPIExtractor{} }

// Extract parses src as an OpenAPI/Swagger document and returns SymbolNodes
// for its operations and schemas.
func (o *OpenAPIExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}

	var pathsNode, schemasNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		val := root.Content[i+1]
		switch key {
		case "paths":
			pathsNode = val
		case "components":
			if val.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(val.Content); j += 2 {
					if val.Content[j].Value == "schemas" {
						schemasNode = val.Content[j+1]
					}
				}
			}
		case "definitions": // Swagger 2.0
			schemasNode = val
		}
	}

	var nodes []core.SymbolNode
	nodes = append(nodes, openAPIOperationNodes(pathsNode, service, pkg, relPath)...)
	nodes = append(nodes, openAPISchemaNodes(schemasNode, service, pkg, relPath)...)

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func openAPIOperationNodes(pathsNode *yaml.Node, service, pkg, relPath string) []core.SymbolNode {
	if pathsNode == nil || pathsNode.Kind != yaml.MappingNode {
		return nil
	}
	var nodes []core.SymbolNode
	for i := 0; i+1 < len(pathsNode.Content); i += 2 {
		pathKey := pathsNode.Content[i]
		pathVal := pathsNode.Content[i+1]
		if pathVal.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(pathVal.Content); j += 2 {
			methodKey := pathVal.Content[j]
			method := strings.ToLower(methodKey.Value)
			if !httpMethodKeys[method] {
				continue
			}
			methodVal := pathVal.Content[j+1]
			sig := ""
			if methodVal.Kind == yaml.MappingNode {
				for k := 0; k+1 < len(methodVal.Content); k += 2 {
					switch methodVal.Content[k].Value {
					case "operationId":
						sig = methodVal.Content[k+1].Value
					case "summary":
						if sig == "" {
							sig = methodVal.Content[k+1].Value
						}
					}
				}
			}
			nodes = append(nodes, core.SymbolNode{
				Service: service, Package: pkg,
				Name:      strings.ToUpper(method) + " " + pathKey.Value,
				Kind:      core.KindOpenAPIOperation,
				FilePath:  relPath,
				Line:      methodKey.Line,
				Signature: sig,
				Tier:      core.TierVerified,
			})
		}
	}
	return nodes
}

func openAPISchemaNodes(schemasNode *yaml.Node, service, pkg, relPath string) []core.SymbolNode {
	if schemasNode == nil || schemasNode.Kind != yaml.MappingNode {
		return nil
	}
	var nodes []core.SymbolNode
	for i := 0; i+1 < len(schemasNode.Content); i += 2 {
		nameNode := schemasNode.Content[i]
		if nameNode.Value == "" {
			continue
		}
		nodes = append(nodes, core.SymbolNode{
			Service: service, Package: pkg, Name: nameNode.Value,
			Kind: core.KindOpenAPISchema, FilePath: relPath, Line: nameNode.Line,
			Tier: core.TierVerified,
		})
	}
	return nodes
}

// ensure OpenAPIExtractor satisfies the Extractor interface at compile time.
var _ Extractor = (*OpenAPIExtractor)(nil)

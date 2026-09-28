package syslink

import (
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

var openAPIMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// openAPIProvides reads an OpenAPI 3.x document (YAML or JSON) and returns
// one route: provides row per operation at spec tier (SG-7), bound by its
// operationId (the node id; "METHOD /path" when the operation has none).
// When every server URL shares one base path, it prefixes each key, since
// that is the path a client calls. Anything that is not an OpenAPI 3.x
// document — Swagger 2.0, other YAML, a parse error — provides nothing.
func openAPIProvides(file string, content []byte) []KeyRow {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	ver := mapValue(root, "openapi")
	if ver == nil || !strings.HasPrefix(ver.Value, "3.") {
		return nil
	}
	base := serversBasePath(mapValue(root, "servers"))
	paths := mapValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return nil
	}
	var out []KeyRow
	for i := 0; i+1 < len(paths.Content); i += 2 {
		p, item := paths.Content[i].Value, paths.Content[i+1]
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			mk, op := item.Content[j], item.Content[j+1]
			if !openAPIMethods[strings.ToLower(mk.Value)] {
				continue
			}
			key, err := ParseContractKey("route:" + strings.ToUpper(mk.Value) + " " + base + p)
			if err != nil {
				continue
			}
			node := key.Body
			if id := mapValue(op, "operationId"); id != nil && id.Value != "" {
				node = id.Value
			}
			out = append(out, KeyRow{
				Key: key, Role: RoleProvides, NodeKind: "operation", NodeID: node,
				File: file, Line: mk.Line, TierHint: TierSpec,
			})
		}
	}
	SortRows(out)
	return out
}

// serversBasePath returns the path every server URL shares ("" when there
// are none, they differ, or the path is the root).
func serversBasePath(servers *yaml.Node) string {
	if servers == nil || servers.Kind != yaml.SequenceNode || len(servers.Content) == 0 {
		return ""
	}
	base := ""
	for i, s := range servers.Content {
		u := mapValue(s, "url")
		if u == nil {
			return ""
		}
		p := u.Value
		if parsed, err := url.Parse(u.Value); err == nil && parsed.Host != "" {
			p = parsed.Path
		}
		p = strings.TrimRight(p, "/")
		if i == 0 {
			base = p
		} else if p != base {
			return ""
		}
	}
	if strings.Contains(base, "{") { // a templated server variable: not a fixed base
		return ""
	}
	return base
}

// mapValue returns the value node for key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

type openAPISource struct{}

func (openAPISource) ID() string { return "openapi" }
func (openAPISource) Epoch() int { return 1 }
func (openAPISource) Keys(file string, content []byte, _ string) []KeyRow {
	return openAPIProvides(file, content)
}

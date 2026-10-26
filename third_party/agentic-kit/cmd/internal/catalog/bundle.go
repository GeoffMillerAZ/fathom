package catalog

import (
	"fmt"
	"sort"
	"strings"

	model "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

// ResolveBundle projects the existing live Go-package inventory through Catalog
// Kit. It adds intermediate groups and records source-relative paths, retaining
// package count as the "package" facet rather than counting synthetic groups.
// Revision is caller-supplied provenance, not an authenticated VCS assertion.
// It does not change Render or the generated Markdown compatibility format.
func ResolveBundle(entries []Entry, revision string) (catalogkit.Bundle, error) {
	type branch struct {
		node     model.AuthoredNode
		children map[string]*branch
	}
	root := &branch{children: map[string]*branch{}}
	seen := map[string]bool{}
	limits := catalogkit.DefaultLimits()
	created := 0
	if len(entries) > limits.MaxNodes {
		return catalogkit.Bundle{}, fmt.Errorf("catalog: package count exceeds catalog bound")
	}
	for _, e := range entries {
		if seen[e.RelPath] {
			return catalogkit.Bundle{}, fmt.Errorf("catalog: ambiguous package path %s", e.RelPath)
		}
		seen[e.RelPath] = true
		parent := root
		parts := strings.Split(e.RelPath, "/")
		if len(parts) > limits.MaxDepth {
			return catalogkit.Bundle{}, fmt.Errorf("catalog: package depth exceeds catalog bound")
		}
		for i, slug := range parts {
			if err := model.ValidateSlug(slug); err != nil {
				return catalogkit.Bundle{}, err
			}
			id := strings.Join(parts[:i+1], "/")
			child := parent.children[slug]
			if child == nil {
				created++
				if created > limits.MaxNodes {
					return catalogkit.Bundle{}, fmt.Errorf("catalog: projected node count exceeds catalog bound")
				}
				child = &branch{node: model.AuthoredNode{Slug: slug, Title: slug, StableKey: "go:" + id, Source: id, Extractor: "go-package-scan", Watermark: revision, Facets: map[string][]string{"kind": {"group"}}}, children: map[string]*branch{}}
				parent.children[slug] = child
			}
			if i == len(parts)-1 {
				child.node.Title = e.Name
				child.node.Synopsis = e.Synopsis
				child.node.Facets["kind"] = []string{"package"}
			}
			parent = child
		}
	}
	var forest func(*branch) []model.AuthoredNode
	forest = func(p *branch) []model.AuthoredNode {
		names := make([]string, 0, len(p.children))
		for name := range p.children {
			names = append(names, name)
		}
		sort.Strings(names)
		nodes := make([]model.AuthoredNode, 0, len(names))
		for _, name := range names {
			child := p.children[name]
			n := child.node
			n.Children = forest(child)
			nodes = append(nodes, n)
		}
		return nodes
	}
	return catalogkit.Resolve(catalogkit.Spec{Version: 1, Layers: []catalogkit.Layer{{Name: "go-source", Nodes: forest(root)}}}, catalogkit.Policy{})
}

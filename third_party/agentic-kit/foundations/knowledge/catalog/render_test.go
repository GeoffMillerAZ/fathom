package catalog

import (
	"strings"
	"testing"
)

// buildCatalogAscending and buildCatalogDescending construct the exact
// same logical node set via genuinely different code paths (append order,
// map-derived vs literal-derived facets) — this is what "independently
// constructed node sets" means for the byte-equality tests below.
func buildCatalogAscending(t *testing.T) Catalog {
	t.Helper()
	var nodes []Node
	nodes = append(nodes, Node{Slug: "foundations"})
	nodes = append(nodes, Node{ParentID: "foundations", Slug: "knowledge"})
	nodes = append(nodes, Node{
		ParentID: "foundations/knowledge", Slug: "catalog", Title: "catalog", Synopsis: "Catalogs that cannot lie.",
		Facets: map[string][]string{"framework": {"html", "react"}, "version": {"3", "4"}},
	})
	cat, err := New(nodes)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func buildCatalogDescending(t *testing.T) Catalog {
	t.Helper()
	facets := map[string][]string{}
	facets["version"] = []string{"4", "3"} // reverse insertion order, unsorted
	facets["framework"] = []string{"react", "html"}
	leaf := Node{
		ParentID: "foundations/knowledge", Slug: "catalog", Title: "catalog", Synopsis: "Catalogs that cannot lie.",
		Facets: facets,
	}
	group := Node{ParentID: "foundations", Slug: "knowledge"}
	root := Node{Slug: "foundations"}
	// Deliberately different construction/append order than the ascending builder.
	nodes := []Node{leaf, root, group}
	cat, err := New(nodes)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestRenderMarkdownByteStableAcrossIndependentConstruction(t *testing.T) {
	a := RenderMarkdown(buildCatalogAscending(t), MarkdownOptions{Title: "# Test catalog", RegenCommand: "task docs-index"})
	b := RenderMarkdown(buildCatalogDescending(t), MarkdownOptions{Title: "# Test catalog", RegenCommand: "task docs-index"})
	if a != b {
		t.Errorf("RenderMarkdown not byte-stable:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}

	// Render the *same* catalog twice too: no hidden state, no randomness.
	cat := buildCatalogAscending(t)
	first := RenderMarkdown(cat, MarkdownOptions{Title: "# Test catalog", RegenCommand: "task docs-index"})
	second := RenderMarkdown(cat, MarkdownOptions{Title: "# Test catalog", RegenCommand: "task docs-index"})
	if first != second {
		t.Error("RenderMarkdown must return identical bytes on repeated calls")
	}
}

func TestRenderMarkdownShape(t *testing.T) {
	cat := buildCatalogAscending(t)
	md := RenderMarkdown(cat, MarkdownOptions{Title: "# Test catalog", RegenCommand: "task docs-index"})
	if !strings.HasPrefix(md, "<!-- Code generated. DO NOT EDIT. Regenerate: task docs-index -->") {
		t.Errorf("expected DO-NOT-EDIT banner naming the regen command, got:\n%s", md[:min(120, len(md))])
	}
	if !strings.Contains(md, "framework=[html,react]") {
		t.Errorf("expected sorted facet rendering, got:\n%s", md)
	}
	// Nesting: catalog should be indented two levels under foundations/knowledge.
	if !strings.Contains(md, "    - `foundations/knowledge/catalog`") {
		t.Errorf("expected nested indentation, got:\n%s", md)
	}
}

func TestRenderMarkdownNoRegenCommandNoBanner(t *testing.T) {
	cat := buildCatalogAscending(t)
	md := RenderMarkdown(cat, MarkdownOptions{})
	if strings.Contains(md, "DO NOT EDIT") {
		t.Error("expected no banner when RegenCommand is empty")
	}
	if !strings.HasPrefix(md, "# Catalog\n\n") {
		t.Errorf("expected default title, got:\n%s", md[:min(40, len(md))])
	}
}

func TestRenderJSONByteStableAcrossIndependentConstruction(t *testing.T) {
	a, err := RenderJSON(buildCatalogAscending(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderJSON(buildCatalogDescending(t))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("RenderJSON not byte-stable:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}

func TestRenderJSONFacetsAreSortedSlice(t *testing.T) {
	data, err := RenderJSON(buildCatalogAscending(t))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"axis": "framework"`) {
		t.Errorf("expected facets rendered as axis/values objects, got:\n%s", s)
	}
	fi := strings.Index(s, `"framework"`)
	vi := strings.Index(s, `"version"`)
	if fi < 0 || vi < 0 || fi > vi {
		t.Errorf("expected facet axes in sorted order (framework before version), got:\n%s", s)
	}
}

func TestRenderTreeByteStableAcrossIndependentConstruction(t *testing.T) {
	a := RenderTree(buildCatalogAscending(t))
	b := RenderTree(buildCatalogDescending(t))
	if a != b {
		t.Errorf("RenderTree not byte-stable:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
	if !strings.Contains(a, "└── foundations") {
		t.Errorf("expected a root connector, got:\n%s", a)
	}
}

func TestRenderSubtreeDepthLimit(t *testing.T) {
	cat := buildCatalogAscending(t)
	full := RenderSubtree(cat, "foundations", 0)
	if !strings.Contains(full, "catalog") {
		t.Errorf("expected unlimited depth to include the leaf, got:\n%s", full)
	}
	limited := RenderSubtree(cat, "foundations", 1)
	if strings.Contains(limited, "catalog") {
		t.Errorf("expected maxDepth=1 to stop before the leaf, got:\n%s", limited)
	}
	if !strings.Contains(limited, "knowledge") {
		t.Errorf("expected maxDepth=1 to still include the immediate child, got:\n%s", limited)
	}
}

func TestRenderSubtreeUnknownID(t *testing.T) {
	cat := buildCatalogAscending(t)
	if got := RenderSubtree(cat, "does-not-exist", 0); got != "" {
		t.Errorf("expected empty string for an unknown ID, got %q", got)
	}
}

package catalog

// ToolSpec describes one MCP tool contract a consumer can register on its
// own server (design §6 — the kit ships the shape and handlers-as-
// functions, never a server: it does not own anyone's surface, and it
// does not name the consumer's actual verb).
type ToolSpec struct {
	Name        string
	Description string
	// InputFields documents the parameter names a consumer's own tool
	// schema should declare, in order.
	InputFields []string
}

// ToolSpecs returns the three MCP tool contracts this package suggests:
// catalog_search, catalog_get, catalog_tree. A consumer registers these
// (under whatever names its own server owns) wired to SearchHandler,
// GetHandler, and TreeHandler.
func ToolSpecs() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "catalog_search",
			Description: "BM25 search over a catalog's titles, synopses, and facet values; returns scored results or an explicit no-match.",
			InputFields: []string{"query"},
		},
		{
			Name:        "catalog_get",
			Description: "Fetch one node by ID, with its immediate children.",
			InputFields: []string{"id"},
		},
		{
			Name:        "catalog_tree",
			Description: "Render a subtree (or the whole catalog) as a compact tree, optionally depth-limited.",
			InputFields: []string{"id", "max_depth"},
		},
	}
}

// SearchHandler returns a function shaped like an MCP tool handler for
// catalog_search: bind cat and ranker once, then wire the returned
// function to a server's dispatch under whatever verb name it owns.
func SearchHandler(cat Catalog, ranker Ranker) func(query string) Outcome {
	return func(query string) Outcome {
		return Search(cat, query, ranker)
	}
}

// GetResult is catalog_get's shape: the node itself (if found) plus its
// immediate children.
type GetResult struct {
	Node     Node
	Found    bool
	Children []Node
}

// GetHandler returns a function shaped like an MCP tool handler for catalog_get.
func GetHandler(cat Catalog) func(id string) GetResult {
	return func(id string) GetResult {
		n, ok := cat.Get(id)
		if !ok {
			return GetResult{Found: false}
		}
		return GetResult{Node: n, Found: true, Children: cat.Children(id)}
	}
}

// TreeHandler returns a function shaped like an MCP tool handler for
// catalog_tree: id == "" renders the whole catalog; maxDepth <= 0 means
// unlimited.
func TreeHandler(cat Catalog) func(id string, maxDepth int) string {
	return func(id string, maxDepth int) string {
		return RenderSubtree(cat, id, maxDepth)
	}
}

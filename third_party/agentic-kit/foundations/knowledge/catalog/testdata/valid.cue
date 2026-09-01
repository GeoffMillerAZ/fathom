// valid.cue — concrete authored catalogs that satisfy #Node/#Catalog as
// defined in ../schema.cue. Running `cue vet ./...` from the package root
// or the repo root validates these against the schema.
package catalog

// A small three-level taxonomy: section > group > item, with an item
// carrying 3 facet axes to show facets are metadata, not extra nesting.
_smallCatalog: #Catalog & [
	{
		slug: "foundations"
		children: [
			{
				slug:  "knowledge"
				title: "Knowledge"
				children: [
					{
						slug:       "catalog"
						title:      "catalog"
						synopsis:   "Catalogs that cannot lie."
						source:     "foundations/knowledge/catalog"
						stable_key: "pkg:foundations/knowledge/catalog"
						facets: {
							framework: ["html", "react", "vue"]
							version: ["3", "4"]
							mode: ["light", "dark", "system"]
						}
					},
				]
			},
		]
	},
]

// A single top-level node with no children and no facets — the minimal
// valid #Node.
_minimalCatalog: #Catalog & [
	{slug: "standalone"},
]

// A node marked hidden — an overlay suppression, still a valid #Node.
_hiddenNode: #Catalog & [
	{slug: "deprecated-tool", hidden: true},
]

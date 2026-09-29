package dialect

// #Doc is the shape of a doc-engine dialect's doc.json
// (docs/design/language-breadth.md §4, plan row LB-F8). Closed: the
// selectors are the §4 subset and never an expression language or a
// template. extractor.NewDocExtractor compiles the selectors and checks the
// cross-references (from / to_kind name an entity kind).
#Doc: {
	decode:          "yaml" | "json" | "jsonc" | "toml" | "xml"
	multi_document?: bool
	if multi_document != _|_ if multi_document {
		decode: "yaml"
	}
	detect: {
		basenames?: [...(string & !="")]
		globs?: [...(string & !="")]
		require_keys?: [...(string & !="")]
		content_sniff?: null | (string & !="")
	}
	entities: [#DocEntity, ...#DocEntity]
	references?: [...#DocRef]
	cross_file?: [...#DocCrossRef]
	// labels: the label maps a label-selector join matches (LB-F30, D4).
	labels?: [...#DocLabels]
}

// #DocSelector: `$` or `.`, then `.key`, `.*` and `[*]` steps.
#DocSelector: =~"^(\\$|\\.)"

// #DocValue: selectors, `$key` and `$path` (the dotted keys down to the
// node), alternated with `|`.
#DocValue: string & !=""

#DocWhere: {
	is_map?:    bool
	is_seq?:    bool
	is_scalar?: bool
	key_in?: [...string]
	not_key_in?: [...string]
	key_prefix?:     string
	key_not_prefix?: string
	has_key?: [...string]
	equals?: {[string]: string}
}

#DocEntity: {
	select:      #DocSelector
	where?:      #DocWhere
	kind:        #Canonical
	source_kind: =~"^[A-Z][A-Za-z0-9]*$"
	name:        #DocValue
	signature?:  #DocValue
	// files: only for files whose basename is listed (a spec shared by
	// several document types, Helm's Chart.yaml and values.yaml).
	files?: [...(string & !="")]
	// include_null: keep a node written with no value (`key:`); selectors
	// drop those by default.
	include_null?: bool
	// qualify: values prefixed to the name, "/"-joined, empty ones skipped.
	qualify?: [...#DocValue]
}

#DocRef: {
	from:    string & !=""
	select:  #DocSelector
	value:   #DocValue
	to_kind: string & !=""
	edge:    string & !=""
}

#DocCrossRef: {
	files?: [...(string & !="")]
	from?:   string & !=""
	where?:  #DocWhere
	select:  #DocSelector
	value:   #DocValue
	resolve: "repo-path" | "repo-root" | "image-ref" | "module-source" | "service-name" | "label-selector"
	edge:    string & !=""
	scope?:         #DocValue
	scope_default?: string & !=""
}

#DocLabels: {
	from:           string & !=""
	where?:         #DocWhere
	select:         #DocSelector
	scope?:         #DocValue
	scope_default?: string & !=""
}

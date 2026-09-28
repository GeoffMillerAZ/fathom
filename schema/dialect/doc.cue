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
}

// #DocSelector: `$` or `.`, then `.key`, `.*` and `[*]` steps.
#DocSelector: =~"^(\\$|\\.)"

// #DocValue: selectors and `$key`, alternated with `|`.
#DocValue: string & !=""

#DocEntity: {
	select: #DocSelector
	where?: {
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
	kind:        #Canonical
	source_kind: =~"^[A-Z][A-Za-z0-9]*$"
	name:        #DocValue
	signature?:  #DocValue
}

#DocRef: {
	from:    string & !=""
	select:  #DocSelector
	value:   #DocValue
	to_kind: string & !=""
	edge:    string & !=""
}

#DocCrossRef: {
	from?:   string & !=""
	select:  #DocSelector
	value:   #DocValue
	resolve: "repo-path" | "image-ref" | "module-source" | "service-name"
	edge:    string & !=""
}

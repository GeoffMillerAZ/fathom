package dialect

// #Doc is the shape of a doc-engine dialect's doc.json
// (docs/design/language-breadth.md §4, plan row LB-F8). Closed: the
// selectors are the §4 subset and never an expression language; a value
// composes strings in one form only (`fmt:`, LB-Q-openapi). extractor.NewDocExtractor compiles the selectors and checks the
// cross-references (from / to_kind name an entity kind).
#Doc: {
	decode:          "yaml" | "json" | "jsonc" | "toml" | "xml"
	multi_document?: bool
	if multi_document != _|_ if multi_document {
		decode: "yaml"
	}
	// yaml_tags: how a YAML local tag (`!Name value`) is read
	// (LB-Q-cloudformation). Unset, it is dropped (the node reads as its
	// untagged value, GitLab's `!reference` a plain list); "cloudformation"
	// reads a short-form intrinsic as its long form (`!Ref X` is {Ref: X},
	// `!GetAtt A.B` is {"Fn::GetAtt": [A, B]}, any other `!Name v` is
	// {"Fn::Name": v}), cfn-lint's rule.
	yaml_tags?: "cloudformation"
	if yaml_tags != _|_ {
		decode: "yaml"
	}
	detect: {
		basenames?: [...(string & !="")]
		globs?: [...(string & !="")]
		require_keys?: [...(string & !="")]
		// content_sniff: a regular expression over the whole file, for a
		// dialect no basename, glob or extension claims (a CloudFormation
		// template): fusion routes a file to it when it matches.
		content_sniff?: null | (string & !="")
	}
	entities: [#DocEntity, ...#DocEntity]
	references?: [...#DocRef]
	cross_file?: [...#DocCrossRef]
	// labels: the label maps a label-selector join matches (LB-F30, D4).
	labels?: [...#DocLabels]
}

// #DocSelector: `$` or `.`, then `.key`, `.*`, `[*]`, `[N]` (a sequence's
// item N, from 0) and `..key` (the value of every entry named key at any
// depth, recursive descent) steps. A key may begin with `$` (`..$ref`).
#DocSelector: =~"^(\\$|\\.)"

// #DocValue: selectors, `$key`, `$path` (the dotted keys down to the node)
// and `$file` (the file's base name, extension included), alternated with `|`.
// `$parent` is the key one step up (an operation's path). An alternative may
// end in transforms, each ` @upper` or ` @file` (a `$ref`'s file part). A
// value `fmt:` + text with `{expr}` placeholders is a template (LB-Q-openapi).
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
	// from: an entity kind, or several joined by `|` (with pointer).
	from:    string & !=""
	select:  #DocSelector
	value:   #DocValue
	to_kind?: string & !=""
	edge:    string & !=""
	// pointer: the value is a local JSON Pointer; the first prefix that
	// begins it gives the target kind, and its first segment after the
	// prefix the name (LB-Q-openapi). to_kind is each prefix's then, and
	// is otherwise required.
	pointer?: [...{prefix: string & !="", to_kind: string & !=""}]
	if pointer == _|_ {
		to_kind: string & !=""
	}
	if pointer != _|_ {
		to_kind?: _|_
	}
	// declared_only: record a value only when it names an entity of
	// to_kind declared in the same document (a CloudFormation Ref to a
	// pseudo parameter is not a reference to one of the template's).
	declared_only?: bool
	// pattern (LB-F145): an RE2 with exactly one capture group, applied to
	// every scalar string the selector reaches (a map or sequence is walked
	// for its strings, never into another entity's node); every
	// non-overlapping match is one value, its group read by value as a
	// scalar. extractor.NewDocExtractor refuses a pattern that does not
	// compile or has another number of groups.
	pattern?: string & =~"\\(" & !=""
}

#DocCrossRef: {
	files?: [...(string & !="")]
	from?:   string & !=""
	where?:  #DocWhere
	select:  #DocSelector
	value:   #DocValue
	resolve: "repo-path" | "repo-root" | "image-ref" | "module-source" | "service-name" | "label-selector" | "route-host"
	edge:    string & !=""
	scope?:         #DocValue
	scope_default?: string & !=""
	// pattern: as #DocRef's; a label-selector reads a map, never a pattern.
	pattern?: string & =~"\\(" & !=""
	if pattern != _|_ {
		resolve: !="label-selector"
	}
}

#DocLabels: {
	from:           string & !=""
	where?:         #DocWhere
	select:         #DocSelector
	scope?:         #DocValue
	scope_default?: string & !=""
}

package dialect

// #Doc is the shape of a doc-engine dialect's doc.json
// (docs/design/language-breadth.md §4, plan row LB-F8). Closed: the
// selectors are the §4 subset and never an expression language; a value
// composes strings in one form only (`fmt:`, LB-Q-openapi). extractor.NewDocExtractor compiles the selectors and checks the
// cross-references (from / to_kind name an entity kind).
#Doc: {
	// "compile-commands" is JSON read through extractor's compile-commands decoder
	// (LB-Q-compile-commands, LB-F127): each entry of a Clang compilation database
	// gains the include directories, macros and output of its flags as keys.
	decode:          "yaml" | "json" | "jsonc" | "toml" | "xml" | "ini" | "compile-commands"
	// decode_files: a decode for the basenames it lists, in place of decode
	// (one dialect reading a YAML file and an INI file, Conan's).
	decode_files?: [string & !=""]: "yaml" | "json" | "jsonc" | "toml" | "xml" | "ini" | "compile-commands"
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
// item N, from 0), `..key` (the value of every entry named key at any
// depth, recursive descent) and `..{key=value}` (every map at any depth
// whose entry key is that scalar) steps. A key may begin with `$` (`..$ref`).
#DocSelector: =~"^(\\$|\\.)"

// #DocValue: selectors, `$key`, `$path` (the dotted keys down to the node)
// and `$file` (the file's base name, extension included), alternated with `|`.
// `$index` is a sequence item's position from 0. `$parent` is the key one step up (an operation's path). An alternative may
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
	// scan: a regular expression (one capture group at least) run over every
	// string under each selected node; each match's first group is a value
	// (an Azure Policy `[parameters('x')]`, wave pc). value is then "$".
	scan?: string & !=""
	// pattern (LB-F145): an RE2 with exactly one capture group, applied to
	// every scalar string the selector reaches (a map or sequence is walked
	// for its strings, never into another entity's node); every
	// non-overlapping match is one value, its group read by value as a
	// scalar. extractor.NewDocExtractor refuses a pattern that does not
	// compile or has another number of groups.
	pattern?: string & =~"\\(" & !=""
	// promql (LB-F317): the value is a PromQL expression, and every metric
	// name its selectors read is a value (a recording rule another rule's
	// expr uses). It reads no pattern, scan or pointer, and names no labels
	// (those belong to a route-host cross-file reference).
	promql?: {}
	if promql != _|_ {
		pattern?: _|_
		scan?:    _|_
		pointer?: _|_
	}
}

#DocCrossRef: {
	files?: [...(string & !="")]
	from?:   string & !=""
	where?:  #DocWhere
	select:  #DocSelector
	value:   #DocValue
	resolve: "repo-path" | "repo-root" | "image-ref" | "module-source" | "service-name" | "label-selector" | "route-host" | "aws-arn" | "resource-type"
	edge:    string & !=""
	scope?:         #DocValue
	scope_default?: string & !=""
	// target_map (resource-type only): the value as written to its Target,
	// "" for an unmapped one (a Custodian resource to its CloudFormation type).
	target_map?: {[string]: string}
	if target_map != _|_ {
		resolve: "resource-type"
	}
	// pattern: as #DocRef's; a label-selector reads a map, never a pattern.
	pattern?: string & =~"\\(" & !=""
	if pattern != _|_ {
		resolve: !="label-selector"
	}
	// via: each value the pattern gives names a declared entity of kind,
	// whose strings via's own pattern reads for the reference (an ARM
	// deployment's `[variables('netUri')]`).
	via?: {
		kind:    string & !=""
		pattern: string & =~"\\(" & !=""
	}
	if via != _|_ {
		pattern: string
	}
	// promql (LB-F317, route-host only): the value is a PromQL expression;
	// each selector's equality matcher on an identifying label is one route
	// value, scoped by its namespace matcher, else by scope. labels replaces
	// the default identifying set (internal/promql/identity_labels.json).
	promql?: #DocPromQL
	if promql != _|_ {
		resolve:  "route-host"
		pattern?: _|_
		via?:     _|_
	}
}

#DocPromQL: {
	labels?: [...(string & !="")]
}

#DocLabels: {
	from:           string & !=""
	where?:         #DocWhere
	select:         #DocSelector
	scope?:         #DocValue
	scope_default?: string & !=""
}

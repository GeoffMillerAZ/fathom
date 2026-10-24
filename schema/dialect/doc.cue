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
	decode:          "yaml" | "json" | "jsonc" | "toml" | "xml" | "ini" | "compile-commands" | "pgbouncer" | "libconfig"
	// decode_files: a decode for the basenames it lists, in place of decode
	// (one dialect reading a YAML file and an INI file, Conan's).
	decode_files?: [string & !=""]: "yaml" | "json" | "jsonc" | "toml" | "xml" | "ini" | "compile-commands" | "pgbouncer" | "libconfig"
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
	// matchers (wave c3, LB-F361): an entity's label matchers, read by
	// internal/promql (an Alertmanager route's, an inhibit rule's).
	matchers?: [...#DocMatchers]
	// facts (wave c3, LB-F361): scalars an entity writes, kept as data a
	// join reads (a route's receiver and continue, a rule's static labels).
	facts?: [...#DocFacts]
}

// #DocSelector: `$` or `.`, then `.key`, `.*`, `[*]`, `[N]` (a sequence's
// item N, from 0), `..key` (the value of every entry named key at any
// depth, recursive descent) and `..{key=value}` (every map at any depth
// whose entry key is that scalar) steps. A key may begin with `$` (`..$ref`).
#DocSelector: =~"^(\\$|\\.)"

// #DocValue: selectors, `$key`, `$path` (the dotted keys down to the node),
// `$ipath` (the keys and sequence indexes down to the node,
// `route.routes[0]`, wave c3) and `$file` (the file's base name, extension
// included), alternated with `|`.
// `$index` is a sequence item's position from 0. `$parent` is the key one step up (an operation's path). An alternative may
// end in transforms, each ` @upper`, ` @file` (a `$ref`'s file part) or
// ` kv:<KEY>` (K20: KEY's value among comma-separated key=value pairs, an
// OTEL_RESOURCE_ATTRIBUTES service.name or a Compose `NAME=value`; nothing
// when no pair names it), applied left to right. A value `fmt:` + text with
// `{expr}` placeholders is a template (LB-Q-openapi).
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
	// not_has_key (K20, O631): the key-absent selector; none of the keys is
	// an entry of the node (a node that is not a map has none).
	not_has_key?: [...(string & !="")]
	// matches (K20): a selector (relative to the node, `$` the node itself)
	// to an RE2 the engine anchors (^...$) and compiles once at load,
	// refusing one that does not compile; some scalar the selector reaches
	// must match it whole.
	matches?: {[#DocSelector]: string & !=""}
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
	// refuse_templates (K20): a name decided at deploy time yields no
	// entity: one carrying `${`, `{{` or `$(`, or read from a scalar with a
	// YAML local tag (`!Ref`, `!Sub`). On by data for every telemetry
	// identity a deploy-time tool interpolates.
	refuse_templates?: bool
	// unique (K20): one entity per name and source kind in a file, its first
	// occurrence (a GitHub Actions pin, named by the action it pins).
	unique?: bool
	// signature_fields (wave c3): the fields the entity writes, each signed
	// `key=value`; signature_keys: the keys it writes, signed by name only
	// (an Alertmanager receiver's integrations, never their settings).
	signature_fields?: [...(string & !="")]
	signature_keys?: [...(string & !="")]
	// sensitive (wave c3): the entity sits in a file whose values may be
	// secrets; its source is never served as a snippet.
	sensitive?: bool
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
	resolve: "repo-path" | "repo-root" | "image-ref" | "module-source" | "service-name" | "label-selector" | "route-host" | "aws-arn" | "resource-type" | "entity-name" | "repo-tree" | "identity"
	edge:    string & !=""
	// to_kind (wave c3): the kind an entity-name reference joins (every
	// entity of that kind and name, fusion's ResolveObsXrefs), or the root
	// kind of the files a repo-tree reference reaches.
	to_kind?: string & !=""
	if resolve == "entity-name" {
		to_kind: string & !=""
	}
	if resolve != "entity-name" && resolve != "repo-tree" {
		to_kind?: _|_
	}
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
	// promql (LB-F317): the value is a PromQL expression. With route-host,
	// each selector's equality matcher on an identifying label is one route
	// value, scoped by its namespace matcher, else by scope; labels replaces
	// the default identifying set (internal/promql/identity_labels.json).
	// With entity-name (wave c3), each selector's metric name is one value.
	promql?: #DocPromQL
	if promql != _|_ {
		resolve:  "route-host" | "entity-name"
		pattern?: _|_
		via?:     _|_
	}
	if resolve == "entity-name" {
		promql?: labels?: _|_
	}
}

#DocPromQL: {
	labels?: [...(string & !="")]
	// grafana (wave c3): a Grafana panel's or alert query's expression, read
	// only for a Prometheus datasource, its macros substituted and its
	// template variables refused as names.
	grafana?: bool
}

// #DocMatchers (wave c3): for each entity of kind from, select reaches its
// label matchers: with syntax alertmanager each scalar is a matcher list;
// with op each entry of a map is one matcher `key op value`. One of them.
#DocMatchers: {
	from:    string & !=""
	select:  #DocSelector
	syntax?: "alertmanager"
	op?:     "=" | "!=" | "=~" | "!~"
	edge:    string & !=""
	if syntax == _|_ {
		op: "=" | "!=" | "=~" | "!~"
	}
	if syntax != _|_ {
		op?: _|_
	}
}

// #DocFacts (wave c3): for each entity of kind from, the scalar select
// reaches is the fact key; with map, each scalar entry of a map reached is
// the fact `key.<entry>`.
#DocFacts: {
	from:   string & !=""
	select: #DocSelector
	key:    string & !=""
	map?:   bool
}

#DocLabels: {
	from:           string & !=""
	where?:         #DocWhere
	select:         #DocSelector
	scope?:         #DocValue
	scope_default?: string & !=""
}

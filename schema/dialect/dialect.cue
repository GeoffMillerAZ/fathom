package dialect

import "struct"

// #Dialect is the shape of internal/extractor/dialects/<id>/dialect.json
// (docs/design/language-breadth.md §2.2, ADR-048). Closed: a manifest never
// carries a claimed level (the reading computes it, §10), so an unknown
// field — "level_met", "grade", a typo — is refused. Cross-dialect rules
// (globally unique source_kind, one owner per extension) need every
// manifest at once and are enforced in Go (TestDialectManifests, LB-F1).
#Dialect: {
	id:       #ID
	display:  string & !=""
	kind:     "language" | "tooling"
	engine:   "tags" | "doc" | "native" | "tags+doc"
	lang_tag: string & !=""

	match: #Match

	// grammar is required for a tags-engine dialect and absent otherwise.
	if engine == "tags" || engine == "tags+doc" {
		grammar: #Grammar
	}

	variants: [...#Variant] | *[]

	queries?: {
		tags?:       =~"\\.scm$"
		imports?:    =~"\\.scm$"
		injections?: =~"\\.scm$"
	}
	if engine == "doc" || engine == "tags+doc" {
		doc: =~"\\.json$" | *"doc.json"
	}

	// kinds: at least one, keyed by the dialect's own kind vocabulary.
	kinds: struct.MinFields(1) & {[string]: #Kind}

	// required_kinds are keys of kinds (§1.4: the harness refuses a reading
	// that excludes one).
	required_kinds: [...or([for k, _ in kinds {k}])]

	// oracle_kinds maps an oracle's kind vocabulary (ctags kind names, SCIP
	// descriptor suffixes, a tool-native oracle's kinds) to canonical kinds.
	oracle_kinds: {[#OracleName]: {[string]: #Canonical}} | *{}

	// name_match is how the harness compares a definition's name with an
	// oracle's (§1.4): "bare" (the default) keeps the qualified name's last
	// segment; "identity" compares the whole name, for dialects whose
	// identifiers carry punctuation (CSS `.a` `#b` `:hover`, npm
	// `@babel/core`, script `codesandbox:build`, export `./utils`).
	name_match: *"bare" | "identity"

	qualify: {
		chain_containers: bool | *true
		container_sep:    string | *"."
		method_sep:       string | *"."
		// scope_sep: the language's own scope operator ("::" for C++); a
		// definition name or callee spelled with it is normalized to
		// container_sep (template arguments dropped).
		scope_sep?: string & !=""
	} | *{chain_containers: true, container_sep: ".", method_sep: "."}

	// names rewrites every captured @name before qualification: a quoted
	// string literal is the symbol without its quotes, an accessor prefix
	// (`.Values.`) is not part of the key it reads.
	names?: {
		trim_quotes?: bool
		strip_prefix?: [...(string & !="")]
	}

	package: {
		strategy: "dir" | "capture" | "none"
	} | *{strategy: "dir"}

	// imports.strategy is from §6's closed set; the parameters are the
	// closed set extractor.ImportParams decodes (LB-F11) — each strategy
	// reads the ones internal/fusion/resolvestrategy.go documents for it.
	imports?: {
		strategy: #ImportStrategy
		source_roots?: [...string]
		suffixes?: [...string]
		index_files?: [...string]
		separator?:     string & !=""
		relative?:      bool
		case?:          "snake"
		labels?:        "bazel"
		type_wildcard?: "members"
	}

	signature: {
		max_chars: int & >0 & <=200 | *200
	} | *{max_chars: 200}

	// ctags is the dialect's own tier-B (§1.3) oracle invocation — universal-
	// ctags' own language name(s) and any extra flags (design §13.4 step 18)
	// — for a dialect internal/langbench's embedded incumbents.json (the
	// five pre-manifest languages) does not already cover.
	ctags?: {
		languages: [...(string & !="")]
		args?: [...string]
	}

	target_level: #Level
	// queue_id equals id: the dialect is its queue entry's (§2.2).
	queue_id: id
}

#ID: =~"^[a-z0-9-]+$"

#Level: "L1" | "L2" | "L3" | "L4"

#Match: {
	extensions: [...=~"^\\.[A-Za-z0-9._+-]+$"] | *[]
	basenames: [...(string & !="")] | *[]
	globs: [...(string & !="")] | *[]
	shebangs: [...(string & !="")] | *[]
	sniff: null | #Sniff | *null
}

// #Sniff splits an extension more than one dialect claims (§5): a regex
// over the first 1 KiB, evaluated in ascending priority across dialects.
#Sniff: {
	extensions: [...=~"^\\.[A-Za-z0-9._+-]+$"] & [_, ...]
	pattern:  string & !=""
	priority: int & >=0
}

#Grammar: {
	blob:        =~"^[a-z0-9_-]+$"
	entry_point: =~"^tree_sitter_[a-z0-9_]+$"
}

// #Variant is a second entry point in the same blob with its own
// extensions (TypeScript's tsx).
#Variant: {
	id:          #ID
	entry_point: =~"^tree_sitter_[a-z0-9_]+$"
	extensions: [...=~"^\\.[A-Za-z0-9._+-]+$"] & [_, ...]
}

#Kind: {
	source_kind: =~"^[A-Z][A-Za-z0-9]*$"
	canonical:   #Canonical
	container?:  bool
	callable?:   bool
	// qualify_as_target: an extension/impl block qualifies its members
	// under the type it extends, not under itself.
	qualify_as_target?: bool
}

#OracleName: "ctags" | "scip" | "tool" | "tags-cli"

#ImportStrategy: "relative-path" | "dotted-module" | "namespace-dir" | "include-path" |
	"crate-mod" | "swift-module" | "file-symbol" | "go-package"

// #Canonical is §2.3's closed list: language kinds, then tool-format kinds.
#Canonical: #LanguageCanonical | #ToolCanonical

#LanguageCanonical: "module" | "namespace" | "package" | "class" | "struct" |
	"interface" | "trait" | "enum" | "enum_member" | "union" | "type_alias" |
	"function" | "method" | "constructor" | "field" | "property" | "constant" |
	"variable" | "macro" | "import" | "test"

#ToolCanonical: "target" | "job" | "stage" | "task" | "step" | "resource" |
	"data_source" | "output" | "variable_decl" | "rule" | "policy" | "service" |
	"rpc" | "message" | "table" | "view" | "column" | "selector" | "mixin" |
	"element" | "section" | "label" | "route" | "key"

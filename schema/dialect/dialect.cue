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

	// references (LB-F74) is the file resolver's own query: @ref.* names a
	// file references outside a call and @export.* the modules it re-exports.
	// locals (LB-F254) is the locals rule's query (@local.scope,
	// @local.definition) for the tags patterns set `(#set! is-not "local")`.
	queries?: {
		tags?:       =~"\\.scm$"
		imports?:    =~"\\.scm$"
		injections?: =~"\\.scm$"
		references?: =~"\\.scm$"
		locals?:     =~"\\.scm$"
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
		// cut_at: keep the text before its first occurrence (a Conan
		// reference `zlib/1.3.1` names the symbol `zlib`).
		cut_at?: string & !=""
		// pascal_case: camelize and capitalize the name as Vue's
		// resolveComponent resolves a component tag (el-input: ElInput).
		pascal_case?: bool
	}

	// package.strategy sets each definition's package: "dir" the file's stem
	// (the directory rule), "file" its base name WITH the extension (LB-F519:
	// X.psd1 beside X.psm1, Button.vue), "capture" @scope.package (§3.5),
	// "none" no package.
	package: {
		strategy: "dir" | "file" | "capture" | "none"
	} | *{strategy: "dir"}

	// preprocess names a pre-parse step (extractor.tsPreprocessors): a file
	// whose tree has a syntax error is parsed as each text the step gives,
	// and the first tree with none is read. "csharp-conditionals" blanks
	// each #if group's directive lines, then also all but its first branch
	// (O148). "cpp-macros" blanks C++ decorator macros by where they are
	// written and reads try and catch macros as the keywords, at equal byte
	// length, then also blanks the conditional directive lines (O455).
	preprocess?: "csharp-conditionals" | "cpp-macros"

	// imports.strategy is from §6's closed set; the parameters are the
	// closed set extractor.ImportParams decodes (LB-F11) — each strategy
	// reads the ones internal/fusion/resolvestrategy.go documents for it.
	imports?: {
		strategy: #ImportStrategy
		source_roots?: [...string]
		suffixes?: [...string]
		index_files?: [...string]
		// partial_prefix "_": relative-path also tries a file name with Sass's
		// partial prefix (`@use "x"` loads `_x.scss`, LB-F465).
		partial_prefix?: "_"
		separator?:     string & !=""
		// dir_vars: variables naming the importing file's own directory
		// (PowerShell's $PSScriptRoot); a file-symbol spec opening with one
		// resolves against that directory (LB-Q-powershell, wave lf6).
		dir_vars?: [...=~"^\\$[A-Za-z_][A-Za-z0-9_]*$"]
		relative?:      bool
		case?:          "snake"
		labels?:        "bazel"
		type_wildcard?: "members"
		// package_scope "nested": a namespace's code sees its own and every
		// enclosing namespace's names unqualified (C#); a namespace-dir dialect
		// whose package is a clause (package.strategy "capture") sees its own
		// package's without it (LB-F74).
		package_scope?: "nested"
		// constants "lexical": a constant reference is looked up through the
		// lexical scopes around it, innermost first (Ruby, LB-F76).
		constants?: "lexical"
		// module_roots "buf": a module path is also looked up under the roots
		// buf.yaml and buf.work.yaml name (protobuf, LB-F77).
		module_roots?: "buf"
		// type_paths "package-dir": a dotted type reference is package-
		// qualified, its package naming the defining file's directory
		// (protobuf, LB-F77).
		type_paths?: "package-dir"
	}

	signature: {
		max_chars: int & >0 & <=200 | *200
		// kinds: keys of kinds whose definitions are signed beside the
		// callables, each with its source up to its @body (a C# class's
		// header and base list, which symbolnav reads for a MonoBehaviour,
		// LB-F290).
		kinds?: [...or(_kindKeys)]
	} | *{max_chars: 200}
	// _kindKeys are the manifest's kind keys, named outside signature, where
	// `kinds` is the list's own field.
	let _kindKeys = [for k, _ in kinds {k}]

	// calls: how call sites are stored (LB-F65). qualify keeps a callee's
	// receiver or scope with the called name ("self.x", "Foo::bar"),
	// compacted; a callee that ends in no name is not stored.
	// owner "module" (wave lf5, design §1.8 W1): every call site is the
	// file's module symbol's, whatever definition encloses it (a markup
	// document's ID reference resolves against the document), so it needs
	// module_symbol. split "whitespace": a call site's text is split on ASCII
	// whitespace, one callee per token (an attribute holding a set of IDs).
	calls?: {
		qualify?: bool
		owner?:   "module"
		split?:   "whitespace"
	}
	if calls != _|_ if calls.owner != _|_ {
		_ownerNeedsModuleSymbol: true & (module_symbol != _|_)
	}

	// module_symbol (K18, ADR-048 amendment 2026-10-10) names the key of kinds
	// the tags engine mints once per file whose top level executes code: the
	// file's module symbol, named "<module>", from line 1 to its last line,
	// the definition its top-level calls and imports are made from and the
	// target of an import of the module. The kind is canonical "module",
	// neither a container nor a callable, and outside oracle_kinds (no oracle
	// tags a file as a definition). Its key is the file's (package strategy
	// "dir", the file's stem, or "file", its base name with the extension
	// for a dialect whose files share a stem, LB-F519: the package is the
	// file's own name). A pure namespace (a Go or
	// Java package, a C# namespace, a Rust `mod`) is file granularity and
	// names none.
	module_symbol?: or(_kindKeys)
	if module_symbol != _|_ {
		engine: "tags" | "tags+doc"
		package: strategy: "dir" | "file"
		kinds: (module_symbol): {canonical: "module", container?: false, callable?: false}
		oracle_kinds: [_]: [_]: !="module"
	}

	// injection_host (F2, wave lf7): the file is the module its injected
	// ranges (queries.injections) are part of. Each range's import specs,
	// references and re-exports become the host file's, at host lines,
	// resolved under the host's own imports strategy, and each range's
	// top-level calls (no enclosing definition of the range) are the host's
	// module symbol's: a Vue or Svelte component's <script>, an Astro page's
	// frontmatter. So a host needs the injections query, the module symbol
	// and the import strategy. Omitted: a range's imports and top-level calls
	// are dropped (HTML's <script>).
	injection_host?: bool
	if injection_host != _|_ if injection_host {
		_hostNeedsInjections:   true & (queries.injections != _|_)
		_hostNeedsModuleSymbol: true & (module_symbol != _|_)
		_hostNeedsImports:      true & (imports != _|_)
	}

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
// extensions (TypeScript's tsx). queries.tags (O732, LB-F385) is a tags
// query of the variant's own, compiled before the dialect's for the
// variant's extensions alone: the patterns only its grammar can satisfy
// (tsx's JSX), which the dialect's other extensions never run.
#Variant: {
	id:          #ID
	entry_point: =~"^tree_sitter_[a-z0-9_]+$"
	extensions: [...=~"^\\.[A-Za-z0-9._+-]+$"] & [_, ...]
	queries?: {
		tags?: =~"\\.scm$"
	}
}

#Kind: {
	source_kind: =~"^[A-Z][A-Za-z0-9]*$"
	canonical:   #Canonical
	container?:  bool
	callable?:   bool
	// qualify_as_target: an extension/impl block qualifies its members
	// under the type it extends, not under itself.
	qualify_as_target?: bool
	// titled: the definition is named by its string title, quoted, and one
	// inside a titled definition of the same kind by both titles (a jest
	// test block inside a describe block: "sum" "adds"); the quotes keep a
	// title from reading as a name a call resolves to.
	titled?: bool
}

#OracleName: "ctags" | "scip" | "tool" | "tags-cli"

#ImportStrategy: "relative-path" | "dotted-module" | "namespace-dir" | "include-path" |
	"crate-mod" | "swift-module" | "file-symbol" | "go-package" | "rego-package"

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

package dialect

// Well-formed manifests: design §2.2's worked Swift example (tags engine,
// L3), a doc-engine tool format, and a native engine leaning on defaults.
swift: #Dialect & {
	id:       "swift"
	display:  "Swift"
	kind:     "language"
	engine:   "tags"
	lang_tag: "swift"
	match: {
		extensions: [".swift"]
		basenames: []
		globs: []
		shebangs: []
		sniff: null
	}
	grammar: {blob: "swift", entry_point: "tree_sitter_swift"}
	variants: []
	queries: {tags: "tags.scm", imports: "imports.scm"}
	kinds: {
		class: {source_kind: "SwiftClass", canonical: "class", container: true}
		struct: {source_kind: "SwiftStruct", canonical: "struct", container: true}
		protocol: {source_kind: "SwiftProtocol", canonical: "interface", container: true}
		extension: {source_kind: "SwiftExtension", canonical: "class", container: true, qualify_as_target: true}
		enum: {source_kind: "SwiftEnum", canonical: "enum", container: true}
		function: {source_kind: "SwiftFunc", canonical: "function", callable: true}
		method: {source_kind: "SwiftMethod", canonical: "method", callable: true}
		property: {source_kind: "SwiftProperty", canonical: "property"}
	}
	required_kinds: ["class", "struct", "protocol", "enum", "function", "method"]
	oracle_kinds: {
		ctags: {class: "class", struct: "struct", protocol: "interface", enum: "enum", func: "function", method: "method", property: "property"}
		scip: {Type: "class", Method: "method", Term: "property"}
	}
	qualify: {chain_containers: true, container_sep: ".", method_sep: "."}
	package: {strategy: "dir"}
	imports: {strategy: "swift-module", suffixes: [".swift"]}
	signature: {max_chars: 200, kinds: ["class"]}
	calls: {qualify: true}
	target_level: "L3"
	queue_id:     "swift"
}

gitlabCI: #Dialect & {
	id:       "gitlab-ci"
	display:  "GitLab CI"
	kind:     "tooling"
	engine:   "doc"
	lang_tag: "gitlab-ci"
	match: {basenames: [".gitlab-ci.yml"], globs: [".gitlab/ci/**/*.yml"]}
	kinds: {
		job: {source_kind: "GitLabCIJob", canonical: "job"}
		template: {source_kind: "GitLabCITemplate", canonical: "job"}
	}
	required_kinds: ["job"]
	oracle_kinds: {tool: {job: "job"}}
	name_match: "identity"
	target_level: "L3"
	queue_id:     "gitlab-ci"
}

objcSniff: #Dialect & {
	id:       "objc"
	display:  "Objective-C"
	kind:     "language"
	engine:   "tags"
	lang_tag: "objc"
	match: {
		extensions: [".mm"]
		sniff: {extensions: [".m", ".h"], pattern: "@interface|@implementation|#import", priority: 0}
	}
	grammar: {blob: "objc", entry_point: "tree_sitter_objc"}
	kinds: {class: {source_kind: "ObjCClass", canonical: "class", container: true}}
	required_kinds: ["class"]
	target_level: "L2"
	queue_id:     "objc"
}

goNative: #Dialect & {
	id:       "go"
	display:  "Go"
	kind:     "language"
	engine:   "native"
	lang_tag: "go"
	match: {extensions: [".go"]}
	kinds: {function: {source_kind: "GoFunc", canonical: "function", callable: true}}
	required_kinds: ["function"]
	target_level: "L4"
	queue_id:     "go"
}

// A native language dialect resolving imports by the rego-package strategy
// (LB-Q-rego, LB-F132): `import data.x.y` to the files declaring package x.y.
regoNative: #Dialect & {
	id:       "rego"
	display:  "Rego / OPA policy"
	kind:     "language"
	engine:   "native"
	lang_tag: "rego"
	match: {extensions: [".rego"]}
	kinds: {
		package: {source_kind: "RegoPackage", canonical: "package"}
		rule: {source_kind: "RegoRule", canonical: "rule"}
		function: {source_kind: "RegoFunction", canonical: "function", callable: true}
	}
	required_kinds: ["package", "rule", "function"]
	oracle_kinds: {tool: {package: "package", rule: "rule", function: "function"}}
	package: {strategy: "capture"}
	imports: {strategy: "rego-package"}
	target_level: "L3"
	queue_id:     "rego"
}

// A tags+doc tool format (LB-Q-helm): documents on the doc engine, templates
// on the tags engine over a go-template grammar, and the manifest's `names`
// rules turning a quoted template name and a `.Values.` accessor into the
// symbol they name.
helm: #Dialect & {
	id:       "helm"
	display:  "Helm charts"
	kind:     "tooling"
	engine:   "tags+doc"
	lang_tag: "helm"
	match: {
		extensions: [".tpl"]
		basenames: ["Chart.yaml", "values.yaml"]
		globs: ["**/templates/*.yaml"]
	}
	grammar: {blob: "gotemplate", entry_point: "tree_sitter_helm"}
	queries: {tags: "tags.scm"}
	kinds: {
		chart: {source_kind: "HelmChart", canonical: "package"}
		template: {source_kind: "HelmTemplate", canonical: "macro", callable: true}
		value_ref: {source_kind: "HelmValueRef", canonical: "key"}
	}
	required_kinds: ["chart", "template"]
	name_match: "identity"
	names: {trim_quotes: true, strip_prefix: [".Values.", "$.Values."]}
	target_level: "L3"
	queue_id:     "helm"
}

// A doc-engine spec (#Doc, schema/dialect/doc.cue) with LB-Q-cloudformation's
// additions: the cloudformation yaml_tags scheme, a content sniff, the `..key`
// and `[N]` selector steps and a declared_only reference.
cfnDoc: #Doc & {
	decode:    "yaml"
	yaml_tags: "cloudformation"
	detect: {require_keys: ["Resources"], content_sniff: "(?m)^AWSTemplateFormatVersion\\s*:"}
	entities: [{select: "$.Resources.*", where: {is_map: true}, kind: "resource", source_kind: "CFNResource", name: "$key", signature: ".Type"}]
	references: [
		{from: "resource", select: "$..Ref", value: "$", to_kind: "resource", edge: "ref", declared_only: true},
		{from: "resource", select: "$..Fn::GetAtt[0]", value: "$", to_kind: "resource", edge: "getatt", declared_only: true},
	]
}

// A doc-engine spec reading a Clang compilation database (LB-Q-compile-commands): the
// compile-commands decoder, a unit per entry, the flags' include directories as references.
compdbDoc: #Doc & {
	decode: "compile-commands"
	detect: {basenames: ["compile_commands.json"]}
	entities: [{select: "$[*]", where: {is_map: true}, kind: "step", source_kind: "CompileUnit", name: ".file", signature: ".output"}]
	references: [{from: "step", select: "$.includes[*]", value: "$", to_kind: "step", edge: "include_dir"}]
}

// LB-Q-openapi's additions to #Doc: a `$ref` key in a selector, `$parent` and
// `@upper` / `@file` in a value, a `fmt:` template, and a reference whose
// `pointer` names each target kind (so it has no to_kind) from several kinds.
openapiDoc: #Doc & {
	decode: "yaml"
	detect: {content_sniff: "(?m)^openapi\\s*:"}
	entities: [{select: "$.paths.*.*", where: {is_map: true, key_in: ["get", "post"]}, kind: "route", source_kind: "OpenAPIOperation", name: "fmt:{$key @upper} {$parent}"},
		{select: "$.components.schemas.*", include_null: true, kind: "type_alias", source_kind: "OpenAPISchema", name: "$key"}]
	references: [
		{from: "OpenAPIOperation|OpenAPISchema", select: "$..$ref", value: "$", edge: "ref", declared_only: true,
			pointer: [{prefix: "#/components/schemas/", to_kind: "OpenAPISchema"}]},
	]
	cross_file: [{select: "$..$ref", value: "$ @file", resolve: "repo-path", edge: "ref"}]
}

// LB-F145's addition to #Doc (wave iac): a reference's and a cross-file
// reference's `pattern`, an RE2 with one capture group read out of every
// string the selector reaches (an ARM template's `[parameters('x')]`, its
// templateLink.uri relative to its own link).
armDoc: #Doc & {
	decode: "json"
	detect: {content_sniff: "deploymentTemplate\\.json"}
	entities: [
		{select: "$.parameters.*", where: {is_map: true}, kind: "variable_decl", source_kind: "ARMParameter", name: "$key"},
		{select: "$.resources[*]", where: {has_key: ["type"]}, kind: "resource", source_kind: "ARMResource", name: ".name"},
	]
	references: [
		{from: "ARMResource", select: "$", value: "$", pattern: "\\bparameters\\('([^']+)'\\)", to_kind: "ARMParameter", edge: "parameter", declared_only: true},
		{from: "ARMResource", select: "$.dependsOn[*]", value: "fmt:[{$}]", pattern: "^\\[resourceId\\('[^']*',\\s*(variables\\('[^']*'\\))\\)\\]$", to_kind: "ARMResource", edge: "depends_on", declared_only: true},
	]
	cross_file: [
		{from: "ARMResource", select: "$.properties.templateLink.uri", value: "$", pattern: "^\\[uri\\(deployment\\(\\)\\.properties\\.templateLink\\.uri,\\s*'([^']+)'\\)\\]$", resolve: "repo-path", edge: "linked_template"},
		{from: "ARMResource", select: "$.properties.templateLink.uri", value: "$", pattern: "^\\[variables\\('([^']+)'\\)\\]$",
			via: {kind: "ARMVariable", pattern: "\\buri\\([^,]*,\\s*'([^']+\\.json)'\\)"}, resolve: "repo-path", edge: "linked_template"},
	]
}

// A tags dialect whose bare-identifier calls the engine's locals rule decides
// (LB-F254): queries.locals names the locals query.
rubyLocals: #Dialect & {
	id:       "ruby"
	display:  "Ruby"
	kind:     "language"
	engine:   "tags"
	lang_tag: "ruby"
	match: {extensions: [".rb"]}
	grammar: {blob: "ruby", entry_point: "tree_sitter_ruby"}
	queries: {tags: "tags.scm", locals: "locals.scm"}
	kinds: {method: {source_kind: "RubyMethod", canonical: "method", callable: true}}
	required_kinds: ["method"]
	target_level: "L3"
	queue_id:     "ruby"
}

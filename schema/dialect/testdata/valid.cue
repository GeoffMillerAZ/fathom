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
	signature: {max_chars: 200}
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

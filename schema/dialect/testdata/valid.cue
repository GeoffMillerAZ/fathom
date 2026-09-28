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
	imports: {strategy: "swift-module"}
	signature: {max_chars: 200}
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

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
	// LB-F369: an identity reference, joined by fusion to the entities signed with it.
	cross_file: [{from: "CFNResource", select: "$.Properties", value: "fmt:aws.cloudwatch.metric={.Namespace}/{.MetricName}", resolve: "identity", edge: "metric"}]
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

// LB-F317's addition to #Doc (wave c1): a reference's and a route-host
// cross-file reference's `promql`, a PromQL expression read by the selector
// scanner (a recording rule used by another rule; an identifying label's
// equality matcher as a route value, with an optional label set).
promqlDoc: #Doc & {
	decode: "yaml"
	detect: {require_keys: ["groups"]}
	entities: [{select: "$.groups[*].rules[*]", where: {has_key: ["record"]}, kind: "rule", source_kind: "PromRecord", name: ".record"},
		{select: "$.groups[*].rules[*]", where: {has_key: ["alert"]}, kind: "rule", source_kind: "PromAlert", name: ".alert"}]
	references: [{from: "PromRecord|PromAlert", select: "$.expr", value: "$", promql: {}, to_kind: "PromRecord", edge: "uses_record", declared_only: true}]
	cross_file: [{from: "rule", select: "$.expr", value: "$", promql: {}, resolve: "route-host", edge: "route_ref", scope: "$.metadata.namespace"},
		{from: "rule", select: "$.expr", value: "$", promql: {labels: ["job", "namespace"]}, resolve: "route-host", edge: "route_ref"}]
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

// A tags dialect whose files' top level executes code (K18, wave lf2): the
// engine mints each file's module symbol from the kind module_symbol names,
// canonical "module" and outside oracle_kinds.
pythonModule: #Dialect & {
	id:       "python"
	display:  "Python"
	kind:     "language"
	engine:   "tags"
	lang_tag: "py"
	match: {extensions: [".py"]}
	grammar: {blob: "python", entry_point: "tree_sitter_python"}
	queries: {tags: "tags.scm"}
	kinds: {
		function: {source_kind: "PythonFunc", canonical: "function", callable: true}
		module: {source_kind: "PythonModule", canonical: "module"}
	}
	required_kinds: ["function"]
	oracle_kinds: {ctags: {function: "function"}}
	module_symbol: "module"
	target_level: "L4"
	queue_id:     "python"
}

// K20's additions to #Doc (wave c2): the key-absent selector, an anchored RE2
// value predicate, the k=v value transform, template refusal for an identity
// capture and one entity per name per file (a GitHub Actions pin; a Compose
// list-form OTEL_RESOURCE_ATTRIBUTES service.name).
predicatesDoc: #Doc & {
	decode: "yaml"
	detect: {globs: [".github/workflows/*.yml"]}
	entities: [
		{select: "$.jobs.*.steps[*]", where: {is_map: true, has_key: ["run"], not_has_key: ["id"]}, kind: "step", source_kind: "RunStep", name: "fmt:{$path}#{$index}"},
		{select: "$.jobs.*.steps[*]", where: {is_map: true, matches: {".uses": "[^./@$:][^/@$:]*/[^@$]*@[0-9a-f]{40}"}}, kind: "resource", source_kind: "PinSHA", name: "fmt:pin:{.uses}", signature: ".uses cut:@", unique: true},
		{select: "$.services.*.environment[*]", where: {is_scalar: true, matches: {"$": "OTEL_RESOURCE_ATTRIBUTES=.*"}}, kind: "key", source_kind: "TelemetryIdentity",
			name: "$ kv:service.name | $ kv:OTEL_RESOURCE_ATTRIBUTES kv:service.name", refuse_templates: true, unique: true},
	]
}

// Wave c3's additions to #Doc (LB-F361): `$ipath`, signature fields and keys,
// label matchers, facts, the entity-name and repo-tree strategies and a
// promql reference that reads metric names, Grafana's mode on.
obsDoc: #Doc & {
	decode: "yaml"
	detect: {content_sniff: "(?m)^route:"}
	entities: [
		{select: "$.route..routes[*]", where: {is_map: true}, kind: "route", source_kind: "Route", name: "$ipath", signature_fields: ["receiver", "matchers"]},
		{select: "$.receivers[*]", kind: "target", source_kind: "Receiver", name: ".name", signature_keys: ["slack_configs"]},
		{select: "$..panels[*]", kind: "element", source_kind: "Panel", name: "fmt:panel/{.id | $ipath}"},
		{select: "$.providers[*]", kind: "data_source", source_kind: "Provider", name: ".name"},
	]
	cross_file: [
		{from: "Panel", select: "$.targets[*]", value: ".expr", promql: {grafana: true}, resolve: "entity-name", to_kind: "PrometheusRecordingRule", edge: "uses_record"},
		{from: "Provider", select: "$.options.path", value: "$", resolve: "repo-tree", to_kind: "Dashboard", edge: "provides"},
	]
	matchers: [
		{from: "Route", select: "$.matchers[*]", syntax: "alertmanager", edge: "match"},
		{from: "Route", select: "$.match_re", op: "=~", edge: "match"},
	]
	facts: [
		{from: "Route", select: "$.receiver", key: "receiver"},
		{from: "Route", select: "$.labels", key: "labels", map: true},
	]
}

// A markup dialect whose same-document ID references are the document's
// (wave lf5, design §1.8 W1): calls.owner "module" gives every call site to the
// module symbol module_symbol names, and calls.split "whitespace" names each
// token of a set of IDs.
htmlDocument: #Dialect & {
	id:       "html"
	display:  "HTML"
	kind:     "language"
	engine:   "tags"
	lang_tag: "html"
	match: {extensions: [".html", ".htm"]}
	grammar: {blob: "html", entry_point: "tree_sitter_html"}
	queries: {tags: "tags.scm"}
	kinds: {
		label: {source_kind: "HTMLId", canonical: "label"}
		document: {source_kind: "HTMLDocument", canonical: "module"}
	}
	module_symbol: "document"
	calls: {owner: "module", split: "whitespace"}
	required_kinds: ["label"]
	oracle_kinds: {ctags: {id: "label"}}
	target_level: "L3"
	queue_id:     "html"
}

// SCSS (LB-F465, wave lf5): a tags dialect whose module symbol makes each
// stylesheet a module, whose names drop their sigils, and whose relative-path
// imports follow Sass's partial rule (partial_prefix "_").
scss: #Dialect & {
	id:       "scss"
	display:  "SCSS"
	kind:     "language"
	engine:   "tags"
	lang_tag: "scss"
	match: {
		extensions: [".scss"]
		basenames: []
		globs: []
		shebangs: []
		sniff: null
	}
	grammar: {blob: "scss", entry_point: "tree_sitter_scss"}
	variants: []
	queries: {tags: "tags.scm", imports: "imports.scm"}
	kinds: {
		mixin: {source_kind: "SCSSMixin", canonical: "function", callable: true}
		variable: {source_kind: "SCSSVariable", canonical: "variable"}
		class: {source_kind: "SCSSClass", canonical: "selector"}
		module: {source_kind: "SCSSModule", canonical: "module"}
	}
	required_kinds: ["mixin", "variable", "class"]
	oracle_kinds: {
		ctags: {mixin: "function", variable: "variable", class: "selector"}
		tool: {mixin: "function", variable: "variable", class: "selector"}
	}
	name_match: "identity"
	names: {strip_prefix: ["$", ".", "#", "%"]}
	module_symbol: "module"
	package: {strategy: "dir"}
	imports: {strategy: "relative-path", relative: true, suffixes: [".scss", ".css"], index_files: ["_index.scss", "index.scss"], partial_prefix: "_"}
	signature: {max_chars: 200}
	target_level: "L3"
	queue_id:     "scss"
}

// An injection host (F2, wave lf7): its injected ranges' import specs are the
// file's, resolved under its own imports strategy, and their top-level calls
// its module symbol's, so it names the injections query, the module symbol
// and the import strategy.
injectionHost: #Dialect & {
	id:       "component"
	display:  "Component"
	kind:     "language"
	engine:   "tags"
	lang_tag: "component"
	match: {extensions: [".component"]}
	grammar: {blob: "component", entry_point: "tree_sitter_component"}
	queries: {tags: "tags.scm", injections: "injections.scm"}
	kinds: {
		component: {source_kind: "ComponentFile", canonical: "module"}
		element: {source_kind: "ComponentElement", canonical: "element"}
	}
	module_symbol:  "component"
	injection_host: true
	calls: {owner: "module"}
	required_kinds: ["element"]
	package: {strategy: "dir"}
	imports: {strategy: "relative-path", suffixes: [".component", ".ts", ".js"]}
	target_level: "L3"
	queue_id:     "component"
}

package dialect

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/langbench/dialectschema_test.go) asserts each fails when
// unified with #Dialect (schema/AGENTS.md convention). Each is the minimal
// Go manifest below with one defect.
//
// base: {id: "go", display: "Go", kind: "language", engine: "native", lang_tag: "go",
//        match: {extensions: [".go"]},
//        kinds: {function: {source_kind: "GoFunc", canonical: "function", callable: true}},
//        required_kinds: ["function"], target_level: "L4", queue_id: "go"}
//
// 1. A claimed level:                 base & {level_met: "L4"}
// 2. An id outside [a-z0-9-]+:        id: "Go", queue_id: "Go"
// 3. A canonical kind off §2.3:       canonical: "procedure"
// 4. A required kind not in kinds:    required_kinds: ["function", "class"]
// 5. queue_id differs from id:        queue_id: "golang"
// 6. A tags engine without grammar:   engine: "tags"
// 7. A doc engine with a grammar:     engine: "doc", grammar: {blob: "go", entry_point: "tree_sitter_go"}
// 8. An import strategy off §6:       imports: {strategy: "stack-graphs"}
// 8a. A Rego strategy misspelt:       imports: {strategy: "rego-packages"}
// 9. A signature over 200 chars:      signature: {max_chars: 300}
// 9a. A signed kind not in kinds:     signature: {kinds: ["class"]}
// 10. No kinds at all:                kinds: {}
// 11. An extension without a dot:     match: {extensions: ["go"]}
// 12. A target level past L4:         target_level: "L5"
// 13. An import parameter no strategy reads: imports: {strategy: "file-symbol", roots: ["lib"]}
// 14. An import case off the list:    imports: {strategy: "dotted-module", case: "kebab"}
// 15. A type_wildcard off the list:   imports: {strategy: "namespace-dir", type_wildcard: "fields"}
// 16. A preprocess step off the list: preprocess: "c-conditionals"
// 16a. A calls rule off the list:     calls: {resolve: true}; calls: {qualify: "yes"}
// 16b. A locals query not a .scm file: queries: {locals: "locals.json"}
//      (asserted by internal/extractor/rubydialect_test.go, TestRubyLocalsManifest)
// 16c. module_symbol naming no kind:  module_symbol: "module"
// 16d. module_symbol off canonical module: module_symbol: "function"
// 16e. module_symbol on a native engine: kinds: {..., module: {source_kind: "GoModule", canonical: "module"}}, module_symbol: "module"
// 16f. module_symbol a container:     kinds: {..., module: {..., container: true}}
// 16g. module_symbol inside oracle_kinds: oracle_kinds: {ctags: {module: "module"}}
// 16h. module_symbol off the dir package rule: package: {strategy: "capture"}
//      (16c..16h asserted by internal/langbench/dialectschema_module_test.go, TestDialectModuleSymbolSchema;
//      the same rules are refused at load by internal/extractor checkModuleSymbol)
// 16j. calls.owner "module" with no module_symbol: calls: {owner: "module"}
// 16k. calls.owner off the list:     calls: {owner: "file"}, module_symbol: "module"
// 16l. calls.split off the list:     calls: {split: "comma"}
//      (16j..16l asserted by internal/langbench/dialectschema_calls_test.go, TestDialectCallsOwnerSchema;
//      the same rules are refused at load by internal/extractor checkCalls)
// 16w. injection_host with no injections query: injection_host: true, module_symbol: "document", imports: {strategy: "relative-path"}
// 16x. injection_host with no module_symbol:  injection_host: true, queries: {tags: "tags.scm", injections: "injections.scm"}, imports: {...}
// 16y. injection_host with no imports:        injection_host: true, queries: {..., injections: "injections.scm"}, module_symbol: "document"
// 16z. injection_host not a boolean:          injection_host: "yes"
//      (16w..16z asserted by internal/langbench/dialectschema_host_test.go, TestDialectInjectionHostSchema;
//      the same rules are refused at load by internal/extractor checkInjectionHost, F2 wave lf7)
// 16i. A variant's tags query not a .scm file: variants: [{id: "tsx", ..., queries: {tags: "tags.tsx.json"}}]
//      (asserted by internal/langbench/dialectschema_variant_test.go, TestDialectVariantQueriesSchema;
//      a variant query its directory does not hold is refused at load, LoadDialects)
// 16j. A partial prefix off Sass's:   imports: {strategy: "relative-path", partial_prefix: "-"}
//      (asserted by internal/langbench/dialectschema_scss_test.go, TestDialectPartialPrefixSchema)
//
// #Doc negatives (internal/langbench/docschema_test.go asserts each fails when
// unified with #Doc), each the minimal doc spec below with one defect.
//
// docBase: {decode: "yaml", detect: {},
//           entities: [{select: "$", kind: "package", source_kind: "X", name: "$file"}]}
//
// 17. A yaml_tags scheme off the list:  yaml_tags: "k8s"
// 18. yaml_tags on a JSON decode:       decode: "json", yaml_tags: "cloudformation"
// 19. declared_only not a boolean:      references: [{from: "package", select: "$..Ref", value: "$", to_kind: "package", edge: "ref", declared_only: "yes"}]
// 20. An empty content sniff:           detect: {content_sniff: ""}
// 21. A selector not rooted at $ or .:  entities: [{select: "Resources..Ref", ...}]
// 22. An empty pattern:                references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "ref", pattern: ""}]
// 23. A pattern with no group:         references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "ref", pattern: "parameters"}]
// 24. A pattern on a label-selector:   cross_file: [{select: "$.selector", value: "$", resolve: "label-selector", edge: "selects", pattern: "(a)"}]
// 25. A via without a pattern:        cross_file: [{select: "$", value: "$", resolve: "repo-path", edge: "e", via: {kind: "package", pattern: "(b)"}}]
// 26. promql off route-host:          cross_file: [{select: "$", value: "$", resolve: "repo-path", edge: "e", promql: {}}]
// 27. promql with a pattern:          cross_file: [{select: "$", value: "$", resolve: "route-host", edge: "e", promql: {}, pattern: "(a)"}]
// 28. An empty promql label:          cross_file: [{select: "$", value: "$", resolve: "route-host", edge: "e", promql: {labels: [""]}}]
// 29. promql labels on a reference:   references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "e", promql: {labels: ["job"]}}]
// 30. promql and scan on a reference: references: [{from: "package", select: "$", value: "$", to_kind: "package", edge: "e", promql: {}, scan: "(a)"}]
// 31. not_has_key not a list:        entities: [{select: "$.steps[*]", where: {not_has_key: "id"}, kind: "step", source_kind: "X", name: "$index"}]
// 32. An empty matches pattern:      entities: [{select: "$.steps[*]", where: {matches: {".uses": ""}}, kind: "step", source_kind: "X", name: ".uses"}]
// 33. A matches key not a selector:  entities: [{select: "$.steps[*]", where: {matches: {uses: "a"}}, kind: "step", source_kind: "X", name: ".uses"}]
// 34. refuse_templates not a boolean: entities: [{select: "$", kind: "key", source_kind: "X", name: "$file", refuse_templates: "yes"}]
// 35. unique not a boolean:          entities: [{select: "$", kind: "key", source_kind: "X", name: "$file", unique: 1}]
//     (an RE2 that does not compile, `matches: {"$": "v[0-9"}`, is CUE-valid
//     and refused at load: internal/extractor TestDocWhereMatchesRefusedAtLoad)
//
// Wave c3's #Doc negatives (asserted by internal/extractor/docengine_obs_schema_test.go,
// TestDocObsSchema), each the minimal doc spec above with one defect:
//
// 36. entity-name without to_kind:   cross_file: [{select: "$.a", value: "$", resolve: "entity-name", edge: "e"}]
// 37. to_kind on another strategy:   cross_file: [{select: "$.a", value: "$", resolve: "repo-path", to_kind: "X", edge: "e"}]
// 38. promql labels on entity-name:  cross_file: [{select: "$.a", value: "$", resolve: "entity-name", to_kind: "X", edge: "e", promql: {labels: ["job"]}}]
// 39. matchers with neither:          matchers: [{from: "package", select: "$.m", edge: "e"}]
// 40. matchers with both:             matchers: [{from: "package", select: "$.m", syntax: "alertmanager", op: "=", edge: "e"}]
// 41. a matcher op off the list:      matchers: [{from: "package", select: "$.m", op: "==", edge: "e"}]
// 42. a fact without a key:           facts: [{from: "package", select: "$.m"}]
// 43. an empty signature field:       entities: [{select: "$", kind: "package", source_kind: "X", name: "$file", signature_fields: [""]}]

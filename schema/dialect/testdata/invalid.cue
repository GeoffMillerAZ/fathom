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
// 9. A signature over 200 chars:      signature: {max_chars: 300}
// 10. No kinds at all:                kinds: {}
// 11. An extension without a dot:     match: {extensions: ["go"]}
// 12. A target level past L4:         target_level: "L5"
// 13. An import parameter no strategy reads: imports: {strategy: "file-symbol", roots: ["lib"]}
// 14. An import case off the list:    imports: {strategy: "dotted-module", case: "kebab"}
// 15. A type_wildcard off the list:   imports: {strategy: "namespace-dir", type_wildcard: "fields"}
// 16. A preprocess step off the list: preprocess: "c-conditionals"
// 16a. A calls rule off the list:     calls: {resolve: true}; calls: {qualify: "yes"}
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

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

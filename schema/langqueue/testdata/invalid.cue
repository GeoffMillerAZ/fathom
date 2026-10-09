package langqueue

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/langbench/queueschema_test.go) asserts each fails when
// unified with #QueueEntry (schema/AGENTS.md convention). Each is the minimal
// Go entry below with one defect.
//
// base: {id: "go", name: "Go", kind: "language", target_level: "L4",
//        grammar_source: "tree-sitter/tree-sitter-go", licence: "MIT",
//        oracle: ["ctags"], calibration_subjects: ["spf13/cobra"], depends_on: [],
//        wave: 1, engine: "tags", plan_row: "LB-Q-go", worker_model: "sonnet"}
//
// tier: "T8"                                  // off the T1-T7 list
// tier: "A"                                   // an oracle tier, not an excellence tier
// kind: "tooling", target_level: "L4"         // L4 is not-applicable for tool formats
// plan_row: "LB-Q-golang"                     // plan_row must be LB-Q-<id>
// stars: 5                                    // unknown field (closed struct)

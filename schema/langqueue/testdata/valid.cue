package langqueue

// Well-formed entries: a tiered language (T1) and an untiered tool format.
go: #QueueEntry & {
	id:             "go"
	name:           "Go (.go)"
	kind:           "language"
	target_level:   "L4"
	grammar_source: "tree-sitter/tree-sitter-go"
	licence:        "MIT"
	oracle: ["ctags", "scip-go"]
	calibration_subjects: ["spf13/cobra"]
	depends_on: ["LB-F1"]
	wave:         1
	engine:       "tags"
	plan_row:     "LB-Q-go"
	worker_model: "sonnet"
	tier:         "T1"
}

makefile: #QueueEntry & {
	id:             "makefile"
	name:           "Makefile"
	kind:           "tooling"
	target_level:   "L2"
	grammar_source: "tree-sitter-grammars/tree-sitter-make"
	licence:        "MIT"
	oracle: ["make -pn"]
	calibration_subjects: ["redis/redis"]
	depends_on: []
	wave:         2
	engine:       "doc"
	plan_row:     "LB-Q-makefile"
	worker_model: "haiku"
	notes:        "untiered: outside the excellence target set"
}

queue: #Queue & [go, makefile]

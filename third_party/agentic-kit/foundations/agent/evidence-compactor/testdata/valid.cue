// Positive fixture: a valid JournalRecord. Used by `cue vet` to confirm
// that the schema.cue contract accepts well-formed records.
package evidencecompactor

import "github.com/geoffmilleraz/agentic-kit/foundations/agent/evidence-compactor:evidencecompactor"

// A complete journal record with two dropped messages.
validCompleteRecord: evidencecompactor.#JournalRecord & {
	kind:             "compaction-dropped"
	at:               "2026-07-09T12:00:00Z"
	dropped_count:    2
	template_version: "v1.0.0"
	params_hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	messages: [
		{
			role: "user"
			content: [{type: "text", text: "What is the capital of France?"}]
		},
		{
			role: "assistant"
			content: [{type: "text", text: "The capital of France is Paris."}]
		},
	]
}

// A record with the optional prompt_hash set (post-LLM-call record).
validRecordWithPromptHash: evidencecompactor.#JournalRecord & {
	kind:             "compaction-dropped"
	at:               "2026-07-09T12:00:00Z"
	dropped_count:    1
	template_version: "v1.0.0"
	params_hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	prompt_hash:      "ba7816bf8f01cfea414140de5dae2ec73b00361bbef0469432f19ff33689573d"
	messages: [
		{
			role: "user"
			content: [{type: "text", text: "single message"}]
		},
	]
}

// A record with a tool_result block in the dropped messages.
validRecordWithToolResult: evidencecompactor.#JournalRecord & {
	kind:             "compaction-dropped"
	at:               "2026-07-09T13:00:00Z"
	dropped_count:    1
	template_version: "v1.0.0"
	params_hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	messages: [
		{
			role: "user"
			content: [{type: "tool_result", tool_use_id: "tu_1", content: "result data"}]
		},
	]
}

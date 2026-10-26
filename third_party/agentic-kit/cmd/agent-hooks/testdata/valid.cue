// Positive fixture: a consumer policy that maps both hosts onto four tiers.
// Codex model names are placeholders: real ones belong in the consumer's
// policy file, never in the kit (invariant I2).
package agenthooks

import "github.com/geoffmilleraz/agentic-kit/cmd/agent-hooks:agenthooks"

_base: {
	version: 1
	tiers: [
		{name: "fast", use_for: "locate, extract, classify", claude: ["haiku"], codex: ["cx-small"], effort: "low"},
		{name: "standard", use_for: "implement from a spec", claude: ["sonnet"], codex: ["cx-mid"], effort: "medium"},
		{name: "deep", use_for: "review and verification", claude: ["opus"], codex: ["cx-large"], effort: "high", marker: "[deep]"},
		{name: "frontier", use_for: "rare final judgment", claude: ["fable"], codex: [], effort: "high", marker: "[frontier]"},
	]
	classes: {
		critic_pattern: "(?i)\\b(review|verify|audit)\\b"
		worker_marker:  "[worker]"
		critic_marker:  "[critic]"
		worker: max_tier: "standard"
		critic: max_tier: "deep"
	}
	spawn: {require_model: true, require_effort: ["codex"], fork: "marker", fork_marker: "[fork]", deny_loop_limit: 6}
	advice: [{id: "ui", when: "(?i)\\bcss\\b", unless: "\\[no-ui\\]", classes: ["worker"], message: "Verify UI work in a real browser."}]
	ledger: {enabled: true, max_bytes: 16777216, keep_rotated: 3, record_tool_output: true}
}

validConsumerPolicy: agenthooks.#Policy & _base & {
	main_thread: {bash_max_chars: 10000, head_chars: 4000, tail_chars: 4000}
}

// Containment disabled is also valid.
validNoContainment: agenthooks.#Policy & _base & {
	main_thread: {bash_max_chars: 0, head_chars: 0, tail_chars: 0}
}

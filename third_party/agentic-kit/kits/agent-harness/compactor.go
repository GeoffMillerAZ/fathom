package agentharness

import (
	"errors"

	evidencecompactor "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/evidence-compactor"
)

// WithEvidenceCompactor returns cfg with ec installed as the conversation
// Compactor (Config.Conversation.Compactor = ec.Compact), so that every
// compaction the harness performs under Config.ContextBudget journals the
// dropped messages verbatim BEFORE any summary is generated and stamps the
// summary with its provenance marker — practices §1, the projection-not-
// replacement rule, enforced by running code rather than by a doc claiming
// the wiring exists.
//
// It fails, rather than silently installing a compactor that can never run,
// when ec is nil or cfg.ContextBudget is not > 0: the harness only windows
// (and so only compacts) when a budget is set. Set ContextBudget first, then
// call this. The compactor's own LLM client is the caller's choice — pass
// the same governed client the harness uses so summary calls are budgeted,
// instrumented and costed like every other call, or a cheaper one; its
// calls are NOT charged against Deps.Budget by the harness.
func WithEvidenceCompactor(cfg Config, ec *evidencecompactor.Compactor) (Config, error) {
	if ec == nil {
		return cfg, errors.New("agentharness: WithEvidenceCompactor: compactor must be non-nil")
	}
	if cfg.ContextBudget <= 0 {
		return cfg, errors.New("agentharness: WithEvidenceCompactor: Config.ContextBudget must be > 0 — the compactor only runs when windowing does, so installing one without a budget is the drift this option exists to remove")
	}
	cfg.Conversation.Compactor = ec.Compact
	return cfg, nil
}

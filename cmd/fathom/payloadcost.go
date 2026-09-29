package main

// payloadcost.go — instrument A, payload cost accounting
// (docs/design/production-tokenomics.md §3, law 12: "No saving is claimed
// without a counterfactual. A token number is a COST until something
// establishes what would have happened otherwise.").
//
// Every injection beat records: rendered bytes, an explicit token
// ESTIMATE (bytes/4 via internal/cost.Estimate — the same heuristic used
// everywhere else in this program), rule count, and phase. The field
// names and every report line built from them say "estimate," never
// "measured" — this is a COST ledger, not a savings ledger.
//
// Five channels (design §3 and EW-1): session brief (session-hook),
// post-edit (edit-hook default path), pre-edit guard (edit-hook --pre),
// module rules (the module-tagged subset of either edit-hook payload), and
// post-bash (edit-hook's Bash-write branch, which served its payload
// outside this instrument until EW-1 counted it).

import (
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/cost"
)

// emitPayloadCost records one injected payload's cost. A zero-length
// payload is a no-op (nothing was actually injected — "answered:false"
// paths already have their own beat; this would only double-count an
// empty string as a real cost line). sessionID, when non-empty, tags the
// beat with the SAME "session" meta key the discovery observer uses
// (discovery.go) — the join key instrument D's session ledger uses to
// attribute this session's A (payload cost) and B (discovery) activity
// (docs/design/production-tokenomics.md §6).
// extraMeta carries COMPONENT figures — a breakdown of the payload already
// counted by this beat, never an addition to it. The channel table sums
// `bytes`; a component belongs in meta, or the grand total double-counts.
func emitPayloadCost(channel, payload string, ruleCount int, phase, sessionID string, extraMeta map[string]string) {
	if payload == "" {
		return
	}
	meta := map[string]string{
		"bytes":           strconv.Itoa(len(payload)),
		"tokens_estimate": strconv.Itoa(cost.Estimate(payload)),
		"rules":           strconv.Itoa(ruleCount),
	}
	if phase != "" {
		meta["phase"] = phase
	}
	if sessionID != "" {
		meta["session"] = sessionID
	}
	for k, v := range extraMeta {
		meta[k] = v
	}
	emitBeat("payload-cost", channel, meta)
}

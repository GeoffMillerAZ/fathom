// learning.go — GL-14 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §6 "Rules that learn"):
// learningAnnouncements is applyLearning's (GL-13) other half — GL-X1
// refuses silent learning ("every state change is announced with its
// signals"), so a caller that persists applyLearning's decisions must
// also announce every one of them, here, in the SAME intent ledger the
// session attention line and `fathom intent audit` already read
// (intent.KindLearning — "extended, not doubled", intentattention.go's
// own doctrine) rather than a new, disconnected channel.
//
// applyLearning itself writes nothing and announces nothing (it only
// decides); this file is where a decision becomes durable, visible
// evidence.
package main

import (
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// learningAnnouncements renders one line per decision — ClaimID, the
// transition, and the Reason string verbatim (the signals that moved it,
// never re-derived) — and records each as an intent.KindLearning ledger
// row, deduped by (root, ClaimID, verdict) via intent.LearningRecorded so
// a caller that recomputes an already-persisted transition does not fill
// the ledger with repeats (applyLearning's own state machine means a real
// transition happens at most once per claim per direction). The returned
// lines are 1:1 with decisions regardless of the dedup outcome — GL-X1's
// "none silent" is about what is ANNOUNCED, never about what is written
// twice.
//
// A ledger write failure is swallowed (AppendLedger's own I9 fail-open
// contract, ledger.go): a hook or a CLI verb calling this must never fail
// because telemetry could not be written, and the returned line still
// carries the decision either way — a caller that also prints these lines
// has not lost the announcement even when the durable record did not
// land.
func learningAnnouncements(root string, decisions []semantic.LearningDecision, now time.Time) []string {
	if len(decisions) == 0 {
		return nil
	}
	existing := intent.ReadLedger()
	lines := make([]string, 0, len(decisions))
	for _, d := range decisions {
		lines = append(lines, fmt.Sprintf("learning: %s %s → %s (%s)", d.ClaimID, d.From, d.To, d.Reason))
		if intent.LearningRecorded(existing, root, d.ClaimID, string(d.To)) {
			continue
		}
		_ = intent.AppendLedger(intent.LedgerRow{
			Kind: intent.KindLearning, RepoRoot: root, ID: d.ClaimID,
			Verdict: string(d.To), Evidence: d.Reason,
			At: now.UTC().Format(time.RFC3339),
		})
	}
	return lines
}

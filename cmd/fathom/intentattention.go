package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// The session-brief attention surface (EP-19, ADR-026 §7).
//
// ONE line carries every "something here may no longer be true" signal — the
// custodian's violated concerns and the intent corpus's unproven claims. A
// second line would double the always-loaded rent for the same job, and the
// design forbids it: "extended, not doubled".
//
// The intent half reads LEDGER ROWS ONLY. It never runs an audit, never opens
// the index, never makes an RPC. Everything it prints was measured by a
// deliberate `fathom intent audit`; the brief is a reader of that record, not
// a second evaluator with a different answer.

const (
	// attentionLineBudget is the byte ceiling for the whole line. The primer
	// is always-loaded context in every session of every adopter; a signal
	// that cannot fit one terminal line is not a signal.
	attentionLineBudget = 160
	// debtDeltaWindow is how far back "the current session" reaches when
	// choosing the baseline for `+k`. Two audits minutes apart must not
	// produce a delta against each other.
	debtDeltaWindow = 24 * time.Hour
	// debtMaxAge is how stale a recorded audit may be and still be spoken.
	// Past this the counts are not evidence about the working tree, they are
	// evidence about a fortnight ago — and a stale nag trains a reader to
	// ignore the line, which costs more than the silence does.
	debtMaxAge = 14 * 24 * time.Hour
	// learningAttentionWindow (GL-14) is how far back the attention line
	// counts a promotion/retirement — a session should hear about LAST
	// week's learning, not a transition from months ago that every prior
	// session already saw and acted on.
	learningAttentionWindow = 7 * 24 * time.Hour
)

// intentAttentionFragment renders the intent half — `unproven M (+k) · fired F`
// — from the newest recorded audit for projDir, or "" when there is nothing
// honest to say: no ledger, no audit for this repo, an audit older than
// debtMaxAge, or an audit that found nothing.
//
// M counts every unproven claim EXCEPT the fired-undecided ones, and F counts
// those, so the two numbers are disjoint populations that add up. `(+k)` is
// rendered only when the count ROSE against a baseline row from before the
// current session; a fall or a missing baseline prints nothing rather than a
// congratulatory "+0".
//
// PE-25: only when projDir is a registered service root (roots, decided
// from the identity the session hook already resolved: no daemon request,
// no extra file read, no extra ledger scan).
func intentAttentionFragment(projDir string, now time.Time, roots ledgerroots.Roots) string {
	rows := attentionRows(intent.ReadLedger(), projDir, roots)
	if len(rows) == 0 {
		return ""
	}
	newest, ok := intent.NewestDebt(rows, projDir)
	if !ok {
		return ""
	}
	at, err := time.Parse(time.RFC3339, newest.At)
	if err != nil || now.Sub(at) > debtMaxAge {
		return ""
	}

	unproven, fired := splitDebt(newest.ByKind)
	if unproven+fired == 0 {
		return ""
	}

	var parts []string
	if unproven > 0 {
		part := fmt.Sprintf("unproven %d", unproven)
		if base, found := intent.DebtBefore(rows, projDir, now.Add(-debtDeltaWindow)); found {
			if wasUnproven, _ := splitDebt(base.ByKind); unproven > wasUnproven {
				part += fmt.Sprintf(" (+%d)", unproven-wasUnproven)
			}
		}
		parts = append(parts, part)
	}
	if fired > 0 {
		parts = append(parts, fmt.Sprintf("fired %d", fired))
	}
	return strings.Join(parts, " · ")
}

// splitDebt separates a debt row's by-kind counts into the fired-undecided
// conditions (U5) and everything else, so the attention line's two numbers
// never count the same claim twice.
func splitDebt(byKind map[string]int) (unproven, fired int) {
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		if k == "U5" {
			fired += byKind[k]
			continue
		}
		unproven += byKind[k]
	}
	return unproven, fired
}

// learningAttentionFragment renders the learning half — `learning N` — from
// the intent ledger's KindLearning rows within learningAttentionWindow
// (GL-14, design doc §6: "every promotion or retirement announced in the
// attention line ... with its signals"). The count alone fits the shared
// line's budget; the signals themselves are one `fathom intent audit` away,
// the same drill-down convention the config/intent halves already use.
// "" when nothing was recorded in the window — silence is correct when
// there is genuinely nothing to report.
func learningAttentionFragment(projDir string, now time.Time, roots ledgerroots.Roots) string {
	rows := intent.RecentLearning(attentionRows(intent.ReadLedger(), projDir, roots), projDir, now.Add(-learningAttentionWindow))
	if len(rows) == 0 {
		return ""
	}
	return fmt.Sprintf("learning %d", len(rows))
}

// attentionRows is the intent ledger's rows for projDir kept under roots
// (PE-25). The line reads only projDir's rows, so they are narrowed to it
// first and the registered-roots rule canonicalizes that one root, never
// every row's.
func attentionRows(rows []intent.LedgerRow, projDir string, roots ledgerroots.Roots) []intent.LedgerRow {
	mine := make([]intent.LedgerRow, 0, len(rows))
	for _, r := range rows {
		if r.RepoRoot == projDir {
			mine = append(mine, r)
		}
	}
	kept, _ := intent.RegisteredRows(mine, roots)
	return kept
}

// attentionLedgerRoots is the registered roots the attention line reads
// under, from the identity the session hook already resolved for projDir
// (PE-25's hook rule: no list_services call, no file read). The identity's
// service root is the only root the line can use: projDir is registered
// exactly when it is that root. No identity (a daemon that did not answer,
// a hook past its deadline) is an unread registry: nothing is excluded.
func attentionLedgerRoots(projDir string, id *identityResult) ledgerroots.Roots {
	if id == nil {
		return ledgerroots.Unread("no service identity was resolved for " + projDir + " this session")
	}
	if id.Service == "" || id.Path == "" {
		return ledgerroots.RegisteredRoots(nil) // the registry answered: projDir is not a service root
	}
	return ledgerroots.RegisteredRoots([]string{id.Path})
}

// attentionLine composes the one attention line for projDir: the custodian's
// violated-concern count and the intent fragment, joined by `·`, remedied by
// the ONE verb that covers both (`fathom doctor` carries the config row and
// the intent rows, and each names its own drill-down verb). Returns "" when
// every fragment is empty — silence is the correct output for a repo with
// nothing to report.
func attentionLine(projDir string, now time.Time, roots ledgerroots.Roots) string {
	var parts []string
	if n := violatedConcernCount(projDir); n > 0 {
		parts = append(parts, fmt.Sprintf("config %d", n))
	}
	// PC-30: a drifted hook, stanza, MCP entry or owned skill is the one
	// thing a session must hear before it trusts its own wiring.
	if frag := wiringDriftAttention(projDir); frag != "" {
		parts = append(parts, frag)
	}
	if frag := intentAttentionFragment(projDir, now, roots); frag != "" {
		parts = append(parts, frag)
	}
	// GL-14: extended, not doubled — one more fragment on the SAME shared
	// line, never a second line.
	if frag := learningAttentionFragment(projDir, now, roots); frag != "" {
		parts = append(parts, frag)
	}
	if len(parts) == 0 {
		return ""
	}
	line := "⚠️ attention: " + strings.Join(parts, " · ") + " — fathom doctor"
	if len(line) > attentionLineBudget {
		// Unreachable with the counts this repo can produce, but a budget
		// that is not enforced is a comment. Drop the intent half rather
		// than truncate mid-number into a different, wrong count.
		line = fmt.Sprintf("⚠️ attention: %s — fathom doctor", parts[0])
	}
	return line
}

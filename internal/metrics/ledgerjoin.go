package metrics

// ledgerjoin.go — SR-68 (docs/design/surface-reckoning.md §6): joins the
// verify ledger and the session ledger by session id — the N2 burn-up and
// N3 cost-per-proven-row views (docs/design/progress-and-economics.md §4
// "The views", §5 "Outcome economics") read this join, not either ledger
// alone; this row lands the join itself, not those consumer views (their
// own plan rows wire them).
//
// Pure aggregation, like every other file in this package (traffic.go,
// activityrecord.go): callers decode both ledgers — cmd/fathom's
// verifyLedgerRow (verifyledger.go) and sessionLedgerEntry
// (sessionledger.go), this package cannot import package main — and pass
// the rows in. A verify-ledger row's session id is not written by every
// origin today (verifyledger.go's Origin field: "cli" — the pre-push
// gate, run outside any live session — versus "stop"/"claim-moment",
// which run inside one and can carry a session id once a later row wires
// it into that writer): a verify run with no session id is UNJOINABLE and
// reported as such in Unjoined, never guessed from commit or timing
// proximity (law 3: unknown stays unknown, docs/design/progress-and-
// economics.md §2 decision 1 — every view here is read-only, computed,
// never inferred).

import "sort"

// VerifyLedgerRow is the minimal verify-ledger shape JoinLedgersBySession
// needs, decoupled from cmd/fathom's verifyLedgerRow the same way
// ActivityCallEntry (activityrecord.go) is decoupled from
// adoptionCallEntry. Session is "" for a row logged before a session id
// was attached to it, or by an origin (like "cli") that runs outside any
// live session.
type VerifyLedgerRow struct {
	Session  string
	Plan     string
	Commit   string
	At       string
	Total    int
	Claimed  int
	Verified int
	Failing  int
}

// SessionLedgerRow is the minimal session-ledger shape JoinLedgersBySession
// needs, decoupled from cmd/fathom's sessionLedgerEntry the same way.
type SessionLedgerRow struct {
	Session string
	Repo    string
	At      string
}

// JoinedSession is one session's ledger row plus every verify-ledger
// reading this join attributed to it, in the order given (callers that
// want newest-last or newest-first sort their own input; this join never
// reorders VerifyRuns relative to each other, only groups them).
type JoinedSession struct {
	Session    string            `json:"session"`
	Ledger     *SessionLedgerRow `json:"ledger,omitempty"`
	VerifyRuns []VerifyLedgerRow `json:"verify_runs,omitempty"`
}

// JoinLedgersBySession groups verifyRuns and sessions by their shared
// Session field. joined is sorted by session id for a deterministic order
// (internal/AGENTS.md's determinism contract); a session-ledger row with
// no verify runs still appears (Ledger set, VerifyRuns nil) — a session
// that never ran verify is a real, measured fact (I38: never a silent
// zero standing in for "not read"). unjoined is every verify-ledger row
// whose Session is "" — named and returned separately, never dropped and
// never folded into a fabricated session.
func JoinLedgersBySession(verifyRuns []VerifyLedgerRow, sessions []SessionLedgerRow) (joined []JoinedSession, unjoined []VerifyLedgerRow) {
	by := map[string]*JoinedSession{}
	var order []string
	get := func(session string) *JoinedSession {
		j, ok := by[session]
		if !ok {
			j = &JoinedSession{Session: session}
			by[session] = j
			order = append(order, session)
		}
		return j
	}

	for _, s := range sessions {
		if s.Session == "" {
			continue // a session-ledger row must carry its own id; one that does not cannot be joined to anything
		}
		row := s
		get(s.Session).Ledger = &row
	}
	for _, v := range verifyRuns {
		if v.Session == "" {
			unjoined = append(unjoined, v)
			continue
		}
		j := get(v.Session)
		j.VerifyRuns = append(j.VerifyRuns, v)
	}

	sort.Strings(order)
	joined = make([]JoinedSession, len(order))
	for i, session := range order {
		joined[i] = *by[session]
	}
	return joined, unjoined
}

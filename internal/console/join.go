package console

// join.go — PE-15 (docs/design/progress-and-economics.md §5, "The join"):
// a session's proven outcome is the set of rows that moved to
// claimed-proven either in a reading the session's own run produced
// (sessionFor's window rule, reused from claim honesty PE-10) or in a
// reading whose commit the session authored — the commit's author time,
// read from its working tree, falls inside the session's window. Every
// transition neither reaches is named in JoinResult.Unjoined, never
// dropped (I38 — blindness is never a healthy zero). Read-only over the
// proof history and the session ledger; nothing here writes a plan or a
// status (ADR-049 decision 1).

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// How a ProvenRow reached its session.
const (
	JoinedByWindow = "window" // the reading itself fell inside the session's run
	JoinedByCommit = "commit" // the reading's commit was authored inside it
)

// ProvenRow is one row's claimed-proven transition. By is empty in
// JoinResult.Unjoined (its slice already says so).
type ProvenRow struct {
	Service string `json:"service"`
	Plan    string `json:"plan"`
	ID      string `json:"id"`
	At      string `json:"at"`
	Commit  string `json:"commit,omitempty"`
	By      string `json:"by,omitempty"`
}

// SessionOutcome is one session's proven rows, oldest first. No output
// here is a per-person figure (I58): SessionID is an opaque session
// identifier, and every render above this joins it right back to
// Harness/Model only, the way ClaimHonesty already does.
type SessionOutcome struct {
	SessionID string      `json:"session_id"`
	Harness   string      `json:"harness"`
	Model     string      `json:"model"`
	Rows      []ProvenRow `json:"rows"`
}

// JoinResult is JoinSessionProof's output.
type JoinResult struct {
	Outcomes []SessionOutcome `json:"outcomes"`
	// Unjoined is every claimed-proven transition no session's window or
	// commit reached — including a row already proven at its plan's first
	// reading in the history (it predates what this history can see, so
	// it can never be joined) — named, never dropped.
	Unjoined []ProvenRow `json:"unjoined"`
}

// CommitAuthorAt resolves commit's author time (RFC 3339) in root's
// working tree via one read-only `git show` (internal/gitro: no write, no
// index refresh). ok is false when root or commit is empty, or the
// command fails (an unresolvable commit, a root git cannot read) — never
// a guessed time.
func CommitAuthorAt(ctx context.Context, root, commit string) (string, bool) {
	if root == "" || commit == "" {
		return "", false
	}
	out, err := gitro.Output(ctx, root, "show", "-s", "--format=%aI", commit)
	if err != nil {
		return "", false
	}
	at := strings.TrimSpace(string(out))
	if at == "" {
		return "", false
	}
	return at, true
}

// JoinSessionProof joins every claimed-proven transition in h (within f's
// filter) to the session that produced it. authorAt resolves a commit's
// author time — CommitAuthorAt in production (passed nil), a fake in
// tests — and is called at most once per distinct (root, commit) pair
// regardless of how many rows share it.
func JoinSessionProof(ctx context.Context, h History, sessions []AgentSession, f HistoryFilter, authorAt func(ctx context.Context, root, commit string) (string, bool)) JoinResult {
	if authorAt == nil {
		authorAt = CommitAuthorAt
	}
	root := rootIndex(h)
	type cacheKey struct{ root, commit string }
	type cacheVal struct {
		at string
		ok bool
	}
	cache := map[cacheKey]cacheVal{}
	cachedAuthorAt := func(rt, commit string) (string, bool) {
		key := cacheKey{rt, commit}
		if v, ok := cache[key]; ok {
			return v.at, v.ok
		}
		at, ok := authorAt(ctx, rt, commit)
		cache[key] = cacheVal{at, ok}
		return at, ok
	}

	outcomes := map[string]*SessionOutcome{}
	var res JoinResult
	for _, rh := range f.rows(h) {
		if first := rh.Readings[0]; first.State == HistProven && f.inRange(first.At) {
			res.Unjoined = append(res.Unjoined, ProvenRow{Service: rh.Service, Plan: rh.Plan, ID: rh.ID, At: first.At, Commit: first.Commit})
		}
		for i := 1; i < len(rh.Readings); i++ {
			prev, cur := rh.Readings[i-1], rh.Readings[i]
			if cur.State != HistProven || prev.State == HistProven || !f.inRange(cur.At) {
				continue
			}
			pr := ProvenRow{Service: rh.Service, Plan: rh.Plan, ID: rh.ID, At: cur.At, Commit: cur.Commit}
			rt := root[readingKey{rh.Service, rh.Plan, cur.At}]
			if s, ok := sessionFor(PlanReading{At: cur.At, Root: rt}, sessions); ok {
				pr.By = JoinedByWindow
				addProvenRow(outcomes, s, pr)
				continue
			}
			if at, ok := cachedAuthorAt(rt, cur.Commit); ok {
				if s, ok := sessionFor(PlanReading{At: at, Root: rt}, sessions); ok {
					pr.By = JoinedByCommit
					addProvenRow(outcomes, s, pr)
					continue
				}
			}
			res.Unjoined = append(res.Unjoined, pr)
		}
	}
	for _, so := range outcomes {
		sort.Slice(so.Rows, func(i, j int) bool { return provenRowLess(so.Rows[i], so.Rows[j]) })
		res.Outcomes = append(res.Outcomes, *so)
	}
	sort.Slice(res.Outcomes, func(i, j int) bool { return res.Outcomes[i].SessionID < res.Outcomes[j].SessionID })
	sort.Slice(res.Unjoined, func(i, j int) bool { return provenRowLess(res.Unjoined[i], res.Unjoined[j]) })
	return res
}

func provenRowLess(a, b ProvenRow) bool {
	if a.At != b.At {
		return a.At < b.At
	}
	return RowKey(a.Service, a.Plan, a.ID) < RowKey(b.Service, b.Plan, b.ID)
}

func addProvenRow(outcomes map[string]*SessionOutcome, s AgentSession, pr ProvenRow) {
	so := outcomes[s.ID]
	if so == nil {
		so = &SessionOutcome{SessionID: s.ID, Harness: s.Harness, Model: s.Model}
		outcomes[s.ID] = so
	}
	so.Rows = append(so.Rows, pr)
}

type readingKey struct{ service, plan, at string }

// rootIndex maps each (service, plan, at) reading to its repo root, from
// the plan-level readings on the same ledger lines the row transitions
// came from — every RowReading.At is one PlanReading.At on the same line.
func rootIndex(h History) map[readingKey]string {
	idx := make(map[readingKey]string, len(h.Readings))
	for _, r := range h.Readings {
		idx[readingKey{r.Service, r.Plan, r.At}] = r.Root
	}
	return idx
}

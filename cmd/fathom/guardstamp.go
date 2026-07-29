package main

// guardstamp.go — the pre/post dedup mechanism (docs/design/guarded-
// edits.md §2.3): "no double-billing" when a file's rules were already
// served by the --pre hook and the SAME edit's --post hook fires moments
// later.
//
// Mechanism decision (this row's DoD requires deciding AND documenting
// this): stamps live under os.TempDir()/fathom-guard-stamps/, keyed by
// (session, service, repo-relative file). os.TempDir() (never
// workspace/tmp) because a stamp is genuinely ephemeral bookkeeping about
// ONE edit within ONE session, not a repo artifact — os.TempDir() also
// keeps it outside any repo tree entirely, so it can never be accidentally
// committed or picked up by the index. "Must not leak across sessions":
// enforced two ways —
//  1. When the PreToolUse/PostToolUse hook JSON carries "session_id"
//     (Claude Code's own hook contract), it is folded into the stamp key,
//     so two different sessions editing the same file never share a stamp
//     at all.
//  2. Regardless of (1) — Codex has no session_id and even Claude Code's
//     field is read defensively, never assumed present — every stamp
//     additionally carries a write timestamp and is treated as absent once
//     older than guardStampTTL. This bounds the worst case (no session_id
//     available) to "two sessions on the same file within the TTL might
//     dedup once," never an unbounded cross-session leak.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// guardStampTTL bounds how long a stamp is honored — the "must not leak
// across sessions" backstop when no session_id is available to scope the
// key precisely (see package doc above). Chosen as a generous upper bound
// on a single coding session, not a tight cache TTL: the dedup is a token
// optimization, not a correctness mechanism, so erring toward "still
// dedups" within a plausible single session costs nothing but a few
// suppressed-marker bytes.
const guardStampTTL = 4 * time.Hour

// guardStampDir is os.TempDir()/fathom-guard-stamps — a package var (not a
// const) so tests can point it at a t.TempDir() without env-var plumbing.
var guardStampDir = filepath.Join(os.TempDir(), "fathom-guard-stamps")

// guardStamp is one pre-hook's rendered rule identity for a single
// (session, service, file) — read back by the post-hook to compute a
// delta instead of repeating the whole payload.
type guardStamp struct {
	// RuleKeys are ruleKey(r) for every rule the pre-hook rendered,
	// sorted — the post-hook's delta is "rules in the new answer whose
	// key is NOT in this set."
	RuleKeys []string `json:"rule_keys"`
	// IndexStale is the pre-hook's own index_stale flag — a staleness
	// flip between pre and post is always surfaced (design §2.3: "any
	// staleness change"), even when RuleKeys is otherwise identical.
	IndexStale bool `json:"index_stale"`
	// WrittenAt is RFC3339 — the TTL clock.
	WrittenAt string `json:"written_at"`
}

// ruleKey returns a stable identity for one served rule — Rule text plus
// its citation, since two different rules can coincidentally share rule
// text (rare, but Src disambiguates) while a module-tagged rule from a
// different module also carries a distinct Module value worth keying on.
func ruleKey(rule, src, module string) string {
	return module + "\x1f" + rule + "\x1f" + src
}

// guardStampPath returns the stamp file path for (sessionID, service,
// repoRelFile). sessionID may be "" (see package doc's leakage bound).
func guardStampPath(sessionID, service, repoRelFile string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + service + "\x1f" + repoRelFile))
	return filepath.Join(guardStampDir, hex.EncodeToString(h[:16])+".json")
}

// writeGuardStamp persists a guard stamp for the pre-hook's rendered rule
// set. Failures are swallowed (hook doctrine: instrumentation/bookkeeping
// must never be the reason a hook breaks its own silent-exit contract) —
// worst case, the post-hook simply finds no stamp and renders the full
// payload, which is correct, just not maximally token-efficient.
func writeGuardStamp(sessionID, service, repoRelFile string, ruleKeys []string, indexStale bool, now time.Time) {
	if err := os.MkdirAll(guardStampDir, 0o700); err != nil {
		return
	}
	sorted := append([]string(nil), ruleKeys...)
	sort.Strings(sorted)
	data, err := json.Marshal(guardStamp{RuleKeys: sorted, IndexStale: indexStale, WrittenAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	_ = os.WriteFile(guardStampPath(sessionID, service, repoRelFile), data, 0o600)
}

// readGuardStamp returns the stamp for (sessionID, service, repoRelFile),
// ok=false when absent, unreadable, malformed, or expired (guardStampTTL).
func readGuardStamp(sessionID, service, repoRelFile string, now time.Time) (guardStamp, bool) {
	data, err := os.ReadFile(guardStampPath(sessionID, service, repoRelFile))
	if err != nil {
		return guardStamp{}, false
	}
	var st guardStamp
	if err := json.Unmarshal(data, &st); err != nil {
		return guardStamp{}, false
	}
	writtenAt, err := time.Parse(time.RFC3339, st.WrittenAt)
	if err != nil || now.Sub(writtenAt) > guardStampTTL {
		return guardStamp{}, false
	}
	return st, true
}

// guardDedupResult is what the post-hook actually renders once a valid
// pre-hook stamp is found for this (session, service, file).
type guardDedupResult struct {
	// DeltaRules are the rules in the new answer NOT already covered by
	// the stamp — "rules newly matched by the edit's actual content"
	// (design §2.3).
	DeltaRules []semanticObligationLike
	// StaleChanged is true when index_stale flipped between pre and post.
	StaleChanged bool
	// SuppressedCount is how many of the new answer's rules WERE already
	// covered by the stamp (i.e. len(newRules) - len(DeltaRules)) — the
	// count the marker line names (design §2.3: "never silent: the marker
	// line always names the suppression").
	SuppressedCount int
}

// semanticObligationLike is the minimal shape dedupAgainstStamp needs from
// a semantic.Obligation — kept as a small local struct (rather than
// importing internal/semantic here) purely to keep this file's test
// surface independent of that package; edithook.go converts.
type semanticObligationLike struct {
	Rule, Src, Module string
}

// dedupAgainstStamp computes the post-hook's delta rendering against a
// pre-hook stamp (design §2.3). newRules is the FULL current answer's
// rules (unfiltered).
func dedupAgainstStamp(stamp guardStamp, newRules []semanticObligationLike, indexStale bool) guardDedupResult {
	stamped := make(map[string]bool, len(stamp.RuleKeys))
	for _, k := range stamp.RuleKeys {
		stamped[k] = true
	}
	var delta []semanticObligationLike
	suppressed := 0
	for _, r := range newRules {
		if stamped[ruleKey(r.Rule, r.Src, r.Module)] {
			suppressed++
			continue
		}
		delta = append(delta, r)
	}
	return guardDedupResult{
		DeltaRules:      delta,
		StaleChanged:    indexStale != stamp.IndexStale,
		SuppressedCount: suppressed,
	}
}

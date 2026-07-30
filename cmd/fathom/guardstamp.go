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
// dedup is SKIPPED ENTIRELY when no session_id is available (see
// writeGuardStamp/readGuardStamp) — critic finding 4 (phase-O merge):
// os.TempDir() is world-writable shared state (e.g. Linux/CI /tmp), and
// with no session_id the stamp key sha256(""+service+relfile) is fully
// predictable, letting anyone who can write to the temp dir forge a stamp
// and make the post-hook silently suppress rules that were never actually
// served pre-edit ("denial of guidance"). Since the dedup is a pure token
// optimization (losing it costs a few repeated bytes, never correctness —
// the full payload is still safe to render), the fix is simply: no
// session_id, no dedup, ever. When a session_id IS present, the stamp is
// additionally hardened against a planted file (O_EXCL|O_NOFOLLOW on
// write, ownership/permission/regular-file checks on read — see below)
// and guardStampTTL remains a sane backstop against an unbounded-lifetime
// stamp if the post-hook never consumes (and deletes, finding 9) it.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
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
// set. sessionID == "" is a no-op (critic finding 4: no session scoping
// available means no predictable-key forgery surface either — the dedup
// optimization is simply skipped, never a correctness risk). Failures are
// otherwise swallowed (hook doctrine: instrumentation/bookkeeping must
// never be the reason a hook breaks its own silent-exit contract) — worst
// case, the post-hook simply finds no stamp and renders the full payload,
// which is correct, just not maximally token-efficient.
func writeGuardStamp(sessionID, service, repoRelFile string, ruleKeys []string, indexStale bool, now time.Time) {
	if sessionID == "" {
		return
	}
	if err := os.MkdirAll(guardStampDir, 0o700); err != nil {
		return
	}
	sorted := append([]string(nil), ruleKeys...)
	sort.Strings(sorted)
	data, err := json.Marshal(guardStamp{RuleKeys: sorted, IndexStale: indexStale, WrittenAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	path := guardStampPath(sessionID, service, repoRelFile)
	// Critic finding 4: never write through a pre-existing path entry —
	// os.Remove does not follow symlinks (it unlinks the entry itself), so
	// this clears a stale stamp from a prior (undeleted, pre-finding-9)
	// run OR a planted symlink/file without ever reading through it.
	// O_EXCL then refuses to proceed if something reappears at path
	// between the Remove and this Open (POSIX: O_CREAT|O_EXCL fails with
	// EEXIST even when the existing entry is a symlink, never following
	// it); O_NOFOLLOW is defense in depth for the same case. Best-effort:
	// any failure here is silent, per this function's own doc comment.
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// readGuardStamp returns the stamp for (sessionID, service, repoRelFile),
// ok=false when absent, unreadable, malformed, expired (guardStampTTL), or
// — critic finding 4 — when sessionID is "" (dedup is skipped entirely
// without a session to scope the key to; see writeGuardStamp) or the file
// on disk fails an ownership/permission/regular-file check (a forged
// stamp: a planted symlink, a file this process doesn't own, or one
// writable by anyone other than its owner).
func readGuardStamp(sessionID, service, repoRelFile string, now time.Time) (guardStamp, bool) {
	if sessionID == "" {
		return guardStamp{}, false
	}
	path := guardStampPath(sessionID, service, repoRelFile)
	// O_NOFOLLOW: a symlink at path fails to open at all (ELOOP) rather
	// than transparently reading through it — closes the read-side of the
	// same forged-stamp surface writeGuardStamp guards on write.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return guardStamp{}, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return guardStamp{}, false
	}
	// Ownership + world/group-writability check on the OPEN FD (not a
	// separate Lstat call) so there is no TOCTOU gap between the check and
	// the read: an attacker-owned or group/world-writable file at this
	// path is refused rather than trusted, even though it passed the
	// O_NOFOLLOW open.
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return guardStamp{}, false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return guardStamp{}, false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
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

// deleteGuardStamp removes the stamp for (sessionID, service, repoRelFile)
// — critic finding 9: a stamp is single-use. Once the post-hook has
// consulted it for a render, it is deleted so it can never be replayed for
// a second, unrelated edit of the same file and does not linger past its
// usefulness. sessionID == "" is a no-op (nothing was ever written for an
// empty session — see writeGuardStamp). Best-effort: a failed remove is
// silent, exactly like every other stamp operation's failure mode.
func deleteGuardStamp(sessionID, service, repoRelFile string) {
	if sessionID == "" {
		return
	}
	_ = os.Remove(guardStampPath(sessionID, service, repoRelFile))
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

package main

// ruleoutcome_track.go — SR-61 (surface reckoning F-27/U4, design doc §6):
// the edit hook's per-claim served/complied/violated tracking, feeding
// GL-10's signal log (ruleoutcome.go's recordRuleOutcome) from the SAME
// "top governing rule this turn vs the file's prior resolved turn"
// comparison SR-48 already makes (askruleverdict.go) — generalized from
// Rule+Src TEXT identity to the claim's stable, content-addressed
// ClaimID, so a rule's wording changing across a recompile is never read
// as a different claim.
//
// Deliberately an INDEPENDENT memo, never sharing askruleverdict.go's own
// file: this tracking can change only what the signal log records, never
// what the agent sees, so it must not be able to disturb SR-48's
// rule-verdict-ask mechanism (or its tests) by construction.
//
// Deliberately scoped to the file's TOP served rule for the same reason
// SR-48 is (askruleverdict.go's own package doc): the full governing set
// can reorder or grow turn to turn for reasons that have nothing to do
// with any one rule's own served/complied/violated life cycle (a
// recompile adding a sibling rule, the cap admitting a different tier).
// The top rule is the one signal stable enough to read without a false
// "violated" or "complied" from churn elsewhere in the set. Widening to
// the full set is a later row's job, not this one's — the exact
// deliberate-scope precedent this package's own doc comment documents.
//
// Semantics:
//   - served:   this turn's top rule carries a claim id — recorded every
//     turn a rule tops the file's governing set, new or not.
//   - violated: the SAME claim id topped the file's governing set on the
//     file's prior resolved turn too — the edit that followed serving it
//     did not address it (askRuleVerdict's own trigger condition, read
//     here independently rather than reused, so this package never needs
//     to know askruleverdict.go's internal ask-string shape).
//   - complied: the prior turn's top rule carried a DIFFERENT claim id (or
//     none) than this turn's — the edit that followed addressed it.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ruleOutcomeMemoTTL mirrors ruleVerdictMemoTTL (askruleverdict.go)
// exactly — a UX/staleness nicety, never a correctness mechanism.
const ruleOutcomeMemoTTL = 4 * time.Hour

// ruleOutcomeMemoDir is a package var (not a const) so tests can point it
// at a t.TempDir() — the same reason guardStampDir and
// ruleVerdictMemoDir are vars.
var ruleOutcomeMemoDir = filepath.Join(os.TempDir(), "fathom-rule-outcome-memos")

// ruleOutcomeMemo is the prior turn's top served claim id for one
// (session, service, file).
type ruleOutcomeMemo struct {
	ClaimID   string `json:"claim_id"`
	WrittenAt string `json:"written_at"`
}

func ruleOutcomeMemoPath(sessionID, service, repoRelFile string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + service + "\x1f" + repoRelFile))
	return filepath.Join(ruleOutcomeMemoDir, hex.EncodeToString(h[:16])+".json")
}

// writeRuleOutcomeMemo persists claimID as the file's current top served
// claim. sessionID == "" is a no-op — a predictable key with no session
// scope would let anyone writing to the shared temp dir plant a memo
// (guardstamp.go's own leakage-bound reasoning). Failures are swallowed
// (hook doctrine): worst case, next turn simply finds no memo and skips
// the violated/complied signal, still recording "served".
func writeRuleOutcomeMemo(sessionID, service, repoRelFile, claimID string, now time.Time) {
	if sessionID == "" {
		return
	}
	if err := os.MkdirAll(ruleOutcomeMemoDir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(ruleOutcomeMemo{ClaimID: claimID, WrittenAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	path := ruleOutcomeMemoPath(sessionID, service, repoRelFile)
	_ = os.Remove(path) // never write through a pre-existing entry — guardstamp.go's own reasoning
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// readRuleOutcomeMemo returns the memo for (sessionID, service,
// repoRelFile), ok=false when absent, unreadable, malformed, expired, or
// sessionID is "" — mirrors readRuleVerdictMemo's hardening exactly
// (O_NOFOLLOW open, regular-file + ownership + not-group/world-writable
// checks on the open FD, no TOCTOU gap).
func readRuleOutcomeMemo(sessionID, service, repoRelFile string, now time.Time) (ruleOutcomeMemo, bool) {
	if sessionID == "" {
		return ruleOutcomeMemo{}, false
	}
	path := ruleOutcomeMemoPath(sessionID, service, repoRelFile)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return ruleOutcomeMemo{}, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return ruleOutcomeMemo{}, false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return ruleOutcomeMemo{}, false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return ruleOutcomeMemo{}, false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return ruleOutcomeMemo{}, false
	}
	var m ruleOutcomeMemo
	if err := json.Unmarshal(data, &m); err != nil {
		return ruleOutcomeMemo{}, false
	}
	writtenAt, err := time.Parse(time.RFC3339, m.WrittenAt)
	if err != nil || now.Sub(writtenAt) > ruleOutcomeMemoTTL {
		return ruleOutcomeMemo{}, false
	}
	return m, true
}

// trackRuleOutcome is trackRuleOutcomeAt with the real clock.
func trackRuleOutcome(sessionID, service, repoRelFile, claimID string) {
	trackRuleOutcomeAt(sessionID, service, repoRelFile, claimID, time.Now())
}

// trackRuleOutcomeAt is SR-61's anchor: reads the prior turn's memo,
// records served (and violated, when the same claim topped the set last
// turn too) for claimID, records complied for the prior turn's claim when
// it differs from this turn's, then rewrites the memo to match what THIS
// turn actually served (claimID=="" clears it) — the exact
// read-then-rewrite shape withRuleVerdictAskAt already uses
// (askruleverdict.go), kept in its own memo so the two mechanisms can
// never interfere with each other.
func trackRuleOutcomeAt(sessionID, service, repoRelFile, claimID string, now time.Time) {
	prior, found := readRuleOutcomeMemo(sessionID, service, repoRelFile, now)
	if claimID != "" {
		recordRuleOutcome(claimID, ruleOutcomeServed, sessionID, repoRelFile)
		if found && prior.ClaimID == claimID {
			recordRuleOutcome(claimID, ruleOutcomeViolated, sessionID, repoRelFile)
		}
	}
	if found && prior.ClaimID != "" && prior.ClaimID != claimID {
		recordRuleOutcome(prior.ClaimID, ruleOutcomeComplied, sessionID, repoRelFile)
	}
	if sessionID == "" {
		return
	}
	if claimID == "" {
		_ = os.Remove(ruleOutcomeMemoPath(sessionID, service, repoRelFile))
		return
	}
	writeRuleOutcomeMemo(sessionID, service, repoRelFile, claimID, now)
}

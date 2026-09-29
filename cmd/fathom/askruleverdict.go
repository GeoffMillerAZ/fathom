package main

// askruleverdict.go — SR-48 (docs/plans/surface-reckoning.md, design doc §5
// row SR-48, F-31): when a served governing rule is violated (still tops
// the same file's obligations two turns running — the edit that followed
// serving it did not address it), the NEXT edit-hook turn asks for a
// one-key verdict instead of silently re-serving the same advice forever:
// corroborate (the rule is right, the violation is real) or refute (the
// rule is wrong, or does not apply here) — printed with the exact `fathom
// claim corroborate|refute` invocation (cmd/fathom/claim.go's
// cmdClaimVerdict) that records it, so answering is one command away.
//
// Cross-turn memory mirrors guardstamp.go's own mechanism exactly (same
// os.TempDir() rationale: ephemeral, per-session bookkeeping, never a repo
// artifact; same sha256(session+service+file) keying; same O_EXCL/
// O_NOFOLLOW/ownership-checked file I/O; same "sessionID == "" ⇒ no-op"
// leakage bound, since a predictable key with no session scope would let
// anyone writing to the shared temp dir plant a memo and forge a verdict
// ask). Unlike a guard stamp, a rule-verdict memo is not single-use: it is
// REWRITTEN every resolved turn to the file's current top rule (or cleared
// when the file is clean), so it always reflects "what was served last
// time", not "what was served once".
//
// Known, accepted gap: renderPostEdit's own "file is fully clean" early
// return discards its resolved postEditMeta (returns the zero value), so a
// memo written while a rule was still being served is not proactively
// cleared the moment the file becomes clean — it simply sits until
// ruleVerdictMemoTTL expires. Harmless: a clean file renders nothing, so
// the "same rule served again" comparison this memo exists for can never
// fire a false positive from it; the cost is one leftover temp file for at
// most ruleVerdictMemoTTL, the exact same bookkeeping-must-never-break-the-
// hook tradeoff guardstamp.go already accepts for its own stamps.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ruleVerdictMemoTTL bounds how long a memo is honored — the same
// generous single-session upper bound guardStampTTL uses; this is a UX
// nicety (don't ask about a rule from three days ago), never a
// correctness mechanism.
const ruleVerdictMemoTTL = 4 * time.Hour

// ruleVerdictMemoDir is os.TempDir()/fathom-rule-verdict-memos — a package
// var (not a const) so tests can point it at a t.TempDir().
var ruleVerdictMemoDir = filepath.Join(os.TempDir(), "fathom-rule-verdict-memos")

// ruleVerdictMemo is the prior turn's top served governing rule for one
// (session, service, file).
type ruleVerdictMemo struct {
	Rule      string `json:"rule"`
	Src       string `json:"src"`
	WrittenAt string `json:"written_at"`
}

func ruleVerdictMemoPath(sessionID, service, repoRelFile string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + service + "\x1f" + repoRelFile))
	return filepath.Join(ruleVerdictMemoDir, hex.EncodeToString(h[:16])+".json")
}

// writeRuleVerdictMemo persists rule/src as the file's current top served
// rule. sessionID == "" is a no-op (see package doc's leakage bound).
// Failures are swallowed (hook doctrine, guardstamp.go's own precedent):
// worst case, next turn simply finds no memo and asks nothing.
func writeRuleVerdictMemo(sessionID, service, repoRelFile, rule, src string, now time.Time) {
	if sessionID == "" {
		return
	}
	if err := os.MkdirAll(ruleVerdictMemoDir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(ruleVerdictMemo{Rule: rule, Src: src, WrittenAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	path := ruleVerdictMemoPath(sessionID, service, repoRelFile)
	_ = os.Remove(path) // never write through a pre-existing entry — guardstamp.go's own reasoning
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// readRuleVerdictMemo returns the memo for (sessionID, service,
// repoRelFile), ok=false when absent, unreadable, malformed, expired, or
// sessionID is "" — mirrors readGuardStamp's hardening exactly (O_NOFOLLOW
// open, regular-file + ownership + not-group/world-writable checks on the
// open FD, no TOCTOU gap).
func readRuleVerdictMemo(sessionID, service, repoRelFile string, now time.Time) (ruleVerdictMemo, bool) {
	if sessionID == "" {
		return ruleVerdictMemo{}, false
	}
	path := ruleVerdictMemoPath(sessionID, service, repoRelFile)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return ruleVerdictMemo{}, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return ruleVerdictMemo{}, false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return ruleVerdictMemo{}, false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return ruleVerdictMemo{}, false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return ruleVerdictMemo{}, false
	}
	var m ruleVerdictMemo
	if err := json.Unmarshal(data, &m); err != nil {
		return ruleVerdictMemo{}, false
	}
	writtenAt, err := time.Parse(time.RFC3339, m.WrittenAt)
	if err != nil || now.Sub(writtenAt) > ruleVerdictMemoTTL {
		return ruleVerdictMemo{}, false
	}
	return m, true
}

func deleteRuleVerdictMemo(sessionID, service, repoRelFile string) {
	if sessionID == "" {
		return
	}
	_ = os.Remove(ruleVerdictMemoPath(sessionID, service, repoRelFile))
}

// askRuleVerdict is SR-48's anchor (F-31): prior is the memo read for the
// file's PRIOR resolved turn (nil when none, expired, or this is the
// file's first resolved turn this session); rule/src is what THIS turn
// actually serves as the file's top governing rule ("" when nothing
// governs it right now — the rule resolved). Returns the one-key verdict
// prompt when the SAME rule (Rule text + Src citation, exactly) is being
// served again — the edit that followed serving it did not address it —
// or "" when there is nothing to ask.
func askRuleVerdict(prior *ruleVerdictMemo, rule, src string) string {
	if prior == nil || rule == "" || prior.Rule != rule || prior.Src != src {
		return ""
	}
	return fmt.Sprintf(
		"🔁 served rule ignored — still applies after this edit: %s (%s)\n"+
			"  answer with one key: [c]orroborate — the rule is right, the violation is real — or [r]efute — this rule is wrong or does not apply here.\n"+
			"  `fathom claim corroborate|refute --claim <id> --note \"...\"` records it (`fathom claim list` finds <id> by this rule's text/citation).",
		rule, src)
}

// withRuleVerdictAsk composes askRuleVerdict's prompt onto the edit hook's
// existing payload — mirrors withClaimMoment's own composition exactly:
// text/ok unchanged when there is nothing to ask, so every pre-existing
// edit-hook golden holds byte for byte. Always rewrites the memo for next
// turn to match what THIS turn actually served (rule=="" clears it — see
// the package doc comment's accepted gap for the one case that does not
// reach this function at all).
func withRuleVerdictAsk(sessionID, service, repoRelFile, rule, src, text string, ok bool) (string, bool) {
	return withRuleVerdictAskAt(sessionID, service, repoRelFile, rule, src, text, ok, time.Now())
}

// withRuleVerdictAskAt is withRuleVerdictAsk with an injected clock (the
// determinism contract, internal/AGENTS.md) — tests pin ruleVerdictMemoTTL
// behavior through it.
func withRuleVerdictAskAt(sessionID, service, repoRelFile, rule, src, text string, ok bool, now time.Time) (string, bool) {
	prior, found := readRuleVerdictMemo(sessionID, service, repoRelFile, now)
	var ask string
	if found {
		ask = askRuleVerdict(&prior, rule, src)
	}
	if rule == "" {
		deleteRuleVerdictMemo(sessionID, service, repoRelFile)
	} else {
		writeRuleVerdictMemo(sessionID, service, repoRelFile, rule, src, now)
	}
	if ask == "" {
		return text, ok
	}
	if ok && text != "" {
		return ask + "\n\n" + text, true
	}
	return ask, true
}

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
//
// SR-48w (F-31): the ask names the served rule's claim id and is answered
// in one key. When the ask fires for a rule with a claim id, the session's
// pending-ask memo (pendingVerdictAsk, same directory, same hardening)
// records it; the UserPromptSubmit hook (prompthook.go) records the user's
// reply only when the whole prompt is exactly `c` or `r`, optionally with
// a note after it (parseVerdictReply — exact keys, never an inference from
// prose, GL-X4); and the next edit-hook turn writes the answered ask as a
// corroborate or refute through claim.go's verdict core
// (recordClaimVerdict), attributed to the session, then clears the memo.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
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
	Rule string `json:"rule"`
	Src  string `json:"src"`
	// ClaimID (SR-48w) is the served rule's claim id, "" for a rule whose
	// compiled claim predates ids or a raw-fallback row.
	ClaimID   string `json:"claim_id,omitempty"`
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
func writeRuleVerdictMemo(sessionID, service, repoRelFile, rule, src, claimID string, now time.Time) {
	if sessionID == "" {
		return
	}
	writeVerdictMemoFile(ruleVerdictMemoPath(sessionID, service, repoRelFile),
		ruleVerdictMemo{Rule: rule, Src: src, ClaimID: claimID, WrittenAt: now.UTC().Format(time.RFC3339)})
}

// writeVerdictMemoFile writes v as JSON at path, never through a
// pre-existing entry (guardstamp.go's own reasoning). Failures are
// swallowed: bookkeeping never breaks the hook.
func writeVerdictMemoFile(path string, v any) {
	if err := os.MkdirAll(ruleVerdictMemoDir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = os.Remove(path)
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
	var m ruleVerdictMemo
	if !readVerdictMemoFile(ruleVerdictMemoPath(sessionID, service, repoRelFile), &m) {
		return ruleVerdictMemo{}, false
	}
	writtenAt, err := time.Parse(time.RFC3339, m.WrittenAt)
	if err != nil || now.Sub(writtenAt) > ruleVerdictMemoTTL {
		return ruleVerdictMemo{}, false
	}
	return m, true
}

// readVerdictMemoFile decodes path into v with readGuardStamp's hardening
// (O_NOFOLLOW open, regular-file + ownership + not-group/world-writable
// checks on the open FD, no TOCTOU gap); false on any failure.
func readVerdictMemoFile(path string, v any) bool {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
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
func askRuleVerdict(prior *ruleVerdictMemo, rule, src, claimID string) string {
	if prior == nil || rule == "" || prior.Rule != rule || prior.Src != src {
		return ""
	}
	if claimID == "" {
		return fmt.Sprintf(
			"🔁 served rule ignored — still applies after this edit: %s (%s)\n"+
				"  answer with one key: [c]orroborate — the rule is right, the violation is real — or [r]efute — this rule is wrong or does not apply here.\n"+
				"  `fathom claim corroborate|refute --claim <id> --note \"...\"` records it (`fathom claim list` finds <id> by this rule's text/citation).",
			rule, src)
	}
	return fmt.Sprintf(
		"🔁 served rule ignored — still applies after this edit: %s (%s) [claim %s]\n"+
			"  ask the user for a one-key verdict as their next prompt: c — corroborate (the rule is right, the violation is real) — or r — refute (this rule is wrong or does not apply here); a note may follow the key after a colon (\"r: superseded by ADR-060\"). The next edit records it against claim %s.\n"+
			"  by hand: `fathom claim corroborate|refute --claim %s --note \"...\"`.",
		rule, src, claimID, claimID, claimID)
}

// withRuleVerdictAsk composes askRuleVerdict's prompt onto the edit hook's
// existing payload — mirrors withClaimMoment's own composition exactly:
// text/ok unchanged when there is nothing to ask, so every pre-existing
// edit-hook golden holds byte for byte. Always rewrites the memo for next
// turn to match what THIS turn actually served (rule=="" clears it — see
// the package doc comment's accepted gap for the one case that does not
// reach this function at all).
func withRuleVerdictAsk(url, sessionID, service, repoRelFile, rule, src, claimID, text string, ok bool) (string, bool) {
	return withRuleVerdictAskAt(sessionID, service, repoRelFile, rule, src, claimID, text, ok, time.Now(), storeVerdictRecorder(url))
}

// withRuleVerdictAskAt is withRuleVerdictAsk with an injected clock (the
// determinism contract, internal/AGENTS.md) and verdict writer — tests pin
// ruleVerdictMemoTTL behavior and the recorded verdict through it.
//
// SR-48w: before comparing this turn's rule, an answered ask pending for
// the session (the prompt hook recorded the user's one-key reply) is
// written through record and its outcome line leads the payload; an ask
// fired this turn for a rule with a claim id becomes the session's
// pending ask.
func withRuleVerdictAskAt(sessionID, service, repoRelFile, rule, src, claimID, text string, ok bool, now time.Time, record verdictRecorder) (string, bool) {
	answered := applyVerdictAnswer(sessionID, now, record)
	prior, found := readRuleVerdictMemo(sessionID, service, repoRelFile, now)
	var ask string
	if found {
		ask = askRuleVerdict(&prior, rule, src, claimID)
	}
	if rule == "" {
		deleteRuleVerdictMemo(sessionID, service, repoRelFile)
	} else {
		writeRuleVerdictMemo(sessionID, service, repoRelFile, rule, src, claimID, now)
	}
	if ask != "" && claimID != "" {
		writePendingVerdictAsk(sessionID, pendingVerdictAsk{
			ClaimID: claimID, Service: service, File: repoRelFile, Rule: rule, Src: src,
			AskedAt: now.UTC().Format(time.RFC3339),
		})
	}
	var parts []string
	for _, p := range []string{answered, ask} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return text, ok
	}
	if ok && text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n"), true
}

// pendingVerdictAsk is the session's one open verdict ask (SR-48w): the
// claim the last ask named, answered once the prompt hook records an
// exact one-key reply. Answer is "corroborated" or "refuted" ("" while
// unanswered).
type pendingVerdictAsk struct {
	ClaimID    string `json:"claim_id"`
	Service    string `json:"service"`
	File       string `json:"file"`
	Rule       string `json:"rule"`
	Src        string `json:"src"`
	AskedAt    string `json:"asked_at"`
	Answer     string `json:"answer,omitempty"`
	Note       string `json:"note,omitempty"`
	AnsweredAt string `json:"answered_at,omitempty"`
}

// pendingVerdictAskPath is keyed by the session alone: the prompt hook
// that reads the reply knows no file or service.
func pendingVerdictAskPath(sessionID string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1fpending-verdict-ask"))
	return filepath.Join(ruleVerdictMemoDir, hex.EncodeToString(h[:16])+".ask.json")
}

func writePendingVerdictAsk(sessionID string, a pendingVerdictAsk) {
	if sessionID == "" {
		return
	}
	writeVerdictMemoFile(pendingVerdictAskPath(sessionID), a)
}

// readPendingVerdictAsk returns the session's pending ask; false when
// absent, unreadable, expired (ruleVerdictMemoTTL from the ask), or
// sessionID is "".
func readPendingVerdictAsk(sessionID string, now time.Time) (pendingVerdictAsk, bool) {
	if sessionID == "" {
		return pendingVerdictAsk{}, false
	}
	var a pendingVerdictAsk
	if !readVerdictMemoFile(pendingVerdictAskPath(sessionID), &a) || a.ClaimID == "" {
		return pendingVerdictAsk{}, false
	}
	asked, err := time.Parse(time.RFC3339, a.AskedAt)
	if err != nil || now.Sub(asked) > ruleVerdictMemoTTL {
		return pendingVerdictAsk{}, false
	}
	return a, true
}

// parseVerdictReply reads an exact one-key reply: the whole prompt is `c`
// or `r` (either case), optionally followed by a colon and a note
// (`r: superseded by ADR-060`). Anything else — a word that starts with c
// or r, a key followed by prose with no colon ("r u sure?"), prose — is
// not a reply (GL-X4: never an inference from prose); a refute flips the
// claim's status, so chat shorthand must never read as one.
func parseVerdictReply(prompt string) (verdict, note string, ok bool) {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return "", "", false
	}
	switch p[0] {
	case 'c', 'C':
		verdict = ruleOutcomeCorroborated
	case 'r', 'R':
		verdict = ruleOutcomeRefuted
	default:
		return "", "", false
	}
	rest := strings.TrimSpace(p[1:])
	if rest == "" {
		return verdict, "", true
	}
	if rest[0] != ':' {
		return "", "", false
	}
	return verdict, strings.TrimSpace(rest[1:]), true
}

// verdictReplyNow is the prompt hook's clock for recordVerdictReply — a
// var so a test pins it to the clock its ask was written with.
var verdictReplyNow = time.Now

// recordVerdictReply is the prompt hook's half: when prompt is an exact
// one-key reply and the session has a pending ask, it records the answer
// on the ask for the next edit-hook turn to write. The latest reply wins.
func recordVerdictReply(sessionID, prompt string, now time.Time) (pendingVerdictAsk, bool) {
	verdict, note, ok := parseVerdictReply(prompt)
	if !ok {
		return pendingVerdictAsk{}, false
	}
	a, found := readPendingVerdictAsk(sessionID, now)
	if !found {
		return pendingVerdictAsk{}, false
	}
	a.Answer, a.Note, a.AnsweredAt = verdict, note, now.UTC().Format(time.RFC3339)
	writePendingVerdictAsk(sessionID, a)
	return a, true
}

// verdictReplyLine is what the prompt hook tells the agent after it
// recorded a reply.
func verdictReplyLine(a pendingVerdictAsk) string {
	return fmt.Sprintf("⚖️ fathom: this prompt is the user's one-key verdict on claim %s (%s, %s): %s. The next edit records it; nothing else in this prompt is a task.", a.ClaimID, a.Rule, a.Src, a.Answer)
}

// verdictRecorder writes one verdict (recordClaimVerdict against a store).
type verdictRecorder func(ctx context.Context, in claimVerdictInput) error

// storeVerdictRecorder writes through claim.go's verdict core into the
// store the daemon at url records (resolveKnowledgeStore: the daemon's
// record, then [daemon].store, then the workspace default).
func storeVerdictRecorder(url string) verdictRecorder {
	return func(ctx context.Context, in claimVerdictInput) error {
		return recordVerdictInStore(ctx, resolveKnowledgeStore("", url).Path, in)
	}
}

// recordVerdictInStore opens the store at path (refusing a path with no
// store: store.Open would create an empty one) and records in.
func recordVerdictInStore(ctx context.Context, path string, in claimVerdictInput) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no store at %s: %v", path, err)
	}
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("open store %s: %w", path, err)
	}
	defer st.Close()
	return recordClaimVerdict(ctx, st, in)
}

// applyVerdictAnswer writes the session's answered pending ask, if any,
// and clears it (recorded or not: a failed write is named once, with the
// command that records it by hand, never retried every turn). The
// verdict is attributed to the session (GL-13's distinct-session count).
func applyVerdictAnswer(sessionID string, now time.Time, record verdictRecorder) string {
	a, found := readPendingVerdictAsk(sessionID, now)
	if !found || a.Answer == "" || record == nil {
		return ""
	}
	_ = os.Remove(pendingVerdictAskPath(sessionID))
	in := claimVerdictInput{
		Service: a.Service, ClaimID: a.ClaimID, Verdict: a.Answer,
		Author: "one-key reply", Note: a.Note, Identity: sessionID, At: now.UTC(),
	}
	if in.Note == "" {
		in.Note = "one-key reply to the edit hook's verdict ask"
	}
	if a.Answer == ruleOutcomeRefuted {
		in.NewStatus = core.ClaimRefuted
	}
	verb := "corroborate"
	if a.Answer == ruleOutcomeRefuted {
		verb = "refute"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := record(ctx, in); err != nil {
		return fmt.Sprintf("⚠️ the user's one-key verdict (%s) on claim %s was not recorded: %v — `fathom claim %s --service %s --claim %s --note %q` records it by hand.",
			a.Answer, a.ClaimID, err, verb, a.Service, a.ClaimID, in.Note)
	}
	return fmt.Sprintf("✅ fathom recorded the user's one-key verdict: claim %s %s (%s, %s).", a.ClaimID, a.Answer, a.Rule, a.Src)
}

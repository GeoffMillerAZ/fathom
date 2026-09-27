package main

// sessionhook_events.go — the wave-3 advisory events (HP-26, HP-27,
// HP-29, HP-30; docs/plans/harness-port.md wave 3, ADR-029 §5.3). Each is
// a `fathom session-hook --event <name>` branch: decode through the
// codec, render one short payload, encode, exit 0. None of them can
// block, none of them reads more than the harness documents, and every
// one is SILENT when it has nothing true to say — a hook that speaks on
// every firing is a tax, not a channel.
//
// The payload functions are harness-free (the renderPostEdit/renderPreEdit
// discipline of HP-5/HP-6): they take a base URL and a directory and
// return a string, so a test pins the text without a hook envelope.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// subagentPrimerBudget caps the subagent primer (HP-26). A subagent's
// context is smaller and shorter-lived than a session's, and it exists to
// do ONE thing — so the primer is the identity line plus the opening
// moves and nothing else. 600 bytes is the whole budget, ~5× under the
// SessionStart primer's (sessionPrimerBudget).
const subagentPrimerBudget = 600

// subagentPrimerCut names the cut when a primer somehow exceeds its cap
// (law 3: never a silent truncation).
const subagentPrimerCut = " …[cut at the subagent primer's byte cap]"

// subagentPrimer renders the ≤600-byte index-first primer a subagent gets
// on the harness's subagent-start event: which service this root is, and
// the opening-moves table the stanza already teaches — the same words, so
// a subagent that never saw SessionStart or the stanza still opens with
// the index instead of grep.
//
// "" (total silence) whenever fathom has nothing TRUE to add: the daemon
// is down, enumeration failed, or this root is not an indexed service. A
// subagent is spawned for a narrow job; a warning it cannot act on is
// pure cost.
//
// parentSessionID (RC-28, docs/vision/28-receipts.md §4) is the harness's
// own session id on the SubagentStart event — the PARENT session's id
// (Claude Code's subagent-start hook fires inside the parent's session,
// never a subagent-minted one of its own), used to look up that session's
// recorded start (sessionStartFor) so the discoveries brief covers
// exactly what the parent had established, not an arbitrary window.
func subagentPrimer(baseURL, projDir, parentSessionID string) string {
	name, info, ok := subagentIdentity(baseURL, projDir, parentSessionID)
	if !ok {
		return ""
	}
	primer := fmt.Sprintf(
		"fathom: this repo is the indexed service %q (%s) — route by question shape: where/who-calls/what-breaks -> fathom MCP tools; what-governs -> obligations (no path); what-was-known -> staleness_report; a doc-vs-doc conflict is grep's job.\n%s",
		name, info.countText(), openingMovesTable,
	)
	// A subagent started inside a git worktree of the service gets the
	// worktree line here: WorktreeCreate is a creation hook, not an
	// advisory one (O34), so this is the only moment that can carry it.
	if isWorktreeDir(projDir) {
		if line := worktreeAdvice(baseURL, projDir); line != "" {
			primer = line + "\n" + primer
		}
	}
	// FL-22 (docs/vision/30-findings-loop.md §3.7): the outbox sync also
	// runs on SubagentStart — a long-running session's first subagent may
	// be the first hook fired since the outbox last changed. The digest
	// cursor makes this safe to call from both paths: whichever fires
	// first does the sync, the other sees an unchanged digest and no-ops.
	if line := syncFindingsOutbox(baseURL, projDir); line != "" {
		primer += "\n" + line
	}
	// RC-28: the parent's discoveries since its own recorded start, within
	// 1200 bytes TOTAL — the constant 600-byte primer above stays the cap
	// when there is nothing to append (subagentPrimerBudget).
	budget := subagentPrimerBudget
	if disc := subagentDiscoveries(baseURL, projDir, name, parentSessionID, time.Now()); disc != "" {
		primer += "\n" + disc
		budget = subagentDiscoveriesPrimerBudget
	}
	if len(primer) > budget {
		keep := budget - len(subagentPrimerCut)
		if keep < 0 {
			keep = 0
		}
		primer = primer[:keep] + subagentPrimerCut
	}
	return primer
}

// subagentDiscoveriesPrimerBudget is the SubagentStart primer's TOTAL cap
// once a discoveries brief is appended (RC-28) — 600 bytes for the base
// primer's own headroom, up to 1200 bytes total.
const subagentDiscoveriesPrimerBudget = 1200

// fathomToolName reports whether toolName is one of fathom's own MCP
// tools — under the plain server-scoped name (`mcp__fathom__find_symbol`)
// or the plugin-scoped one Claude Code uses when the server arrives
// through a plugin (`mcp__plugin_<plugin>_fathom__find_symbol`). Anyone
// else's failure is not fathom's business, and HP-27's whole contract is
// that the hook is silent for it.
func fathomToolName(toolName string) bool {
	if strings.HasPrefix(toolName, "mcp__fathom__") {
		return true
	}
	if !strings.HasPrefix(toolName, "mcp__plugin_") {
		return false
	}
	// mcp__plugin_<plugin>_<server>__<tool>: the server segment is the
	// text between the last "_" before "__" and the "__" itself.
	i := strings.Index(toolName[len("mcp__"):], "__")
	if i < 0 {
		return false
	}
	head := toolName[len("mcp__") : len("mcp__")+i]
	return strings.HasSuffix(head, "_fathom")
}

// toolFailureAdvice renders ONE remedy line for a failed fathom MCP tool
// call (HP-27), from the same facts doctor's daemon/staleness rows read:
// is the daemon answering, and does it know this repo as a service. "" —
// silence — for any tool that is not fathom's, so the hook costs nothing
// on the overwhelming majority of firings.
func toolFailureAdvice(baseURL, projDir, toolName, toolErr string) string {
	if !fathomToolName(toolName) {
		return ""
	}
	short := strings.TrimSpace(toolErr)
	if i := strings.IndexByte(short, '\n'); i >= 0 {
		short = short[:i]
	}
	if len(short) > 200 {
		short = short[:200] + "…"
	}
	lead := fmt.Sprintf("⚠️ the fathom tool %s failed", toolName)
	if short != "" {
		lead += fmt.Sprintf(" (%s)", short)
	}
	hc := &http.Client{Timeout: 2 * time.Second}
	if !daemonAnswering(hc, baseURL) {
		return fmt.Sprintf("%s — the fathom daemon (%s) is not responding, so every fathom tool will fail this session; Grep/Read discovery is the expected fallback. Restart it with: %s",
			lead, baseURL, daemonRestartHint(""))
	}
	name, info, ok := indexedServiceFor(hc, baseURL, projDir)
	if !ok {
		return fmt.Sprintf("%s — the daemon is up, but this repo is not resolvable as an indexed service, so a `service` argument naming it cannot be answered. Index it with the ingest tool ({\"service\":\"<name>\",\"path\":\"<absolute repo root>\"}), or fall back to Grep/Read.",
			lead)
	}
	return fmt.Sprintf("%s — the daemon is up and this repo IS the indexed service %q (%s), so the failure is in the call, not the wiring: check the arguments (service must be %q) and retry. `fathom doctor` gives the full diagnosis.",
		lead, name, info.countText(), name)
}

// worktreeAdvice renders the one line a freshly created git worktree gets
// (HP-29): which service the worktree's code belongs to, and the fact
// PC-33 made expensive to learn the hard way — an ingest run from inside a
// worktree does NOT re-root the service; it would repoint the index at a
// checkout that disappears. "" when this root resolves to no service.
func worktreeAdvice(baseURL, worktreePath string) string {
	hc := &http.Client{Timeout: 2 * time.Second}
	name, _, ok := indexedServiceFor(hc, baseURL, worktreePath)
	if !ok {
		return ""
	}
	return fmt.Sprintf("fathom: this worktree belongs to the indexed service %q. Its symbols are the MAIN checkout's — `ingest` will not re-root the service at this worktree, so index answers here describe the service's recorded corpus root, not this directory. Query the index as usual; read files from this worktree to edit them.", name)
}

// cwdChangedPrimer renders the identity line for a root the session just
// moved to or added (HP-30) — the SAME line SessionStart composes
// (buildSessionContext), because the honest answer to "where am I now" is
// exactly the answer to "where did I start": indexed service and its
// freshness, or the honest not-indexed / daemon-down line. No thread
// brief and no attention line: those belong to a session, not a directory.
func cwdChangedPrimer(baseURL, newRoot string) string {
	if newRoot == "" {
		return ""
	}
	return buildSessionContext(baseURL, newRoot)
}

// daemonAnswering is the healthz probe the advisory events share.
func daemonAnswering(hc *http.Client, baseURL string) bool {
	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// indexedServiceFor resolves dir to an indexed service through the same
// authoritative path the session hook uses (the daemon's recorded corpus
// roots first, then the basename candidates). ok=false for a down daemon,
// a failed enumeration, or a root no service records — never a guess.
func indexedServiceFor(hc *http.Client, baseURL, dir string) (string, svcInfo, bool) {
	if dir == "" {
		return "", svcInfo{}, false
	}
	// RT-133: one GET /identity (the same resolution, done where the
	// registry lives) behind the cached resolver, else the full list from
	// a daemon that predates it.
	if id, err := lookupIdentityCached(hc, baseURL, dir); !errors.Is(err, errIdentityRouteMissing) {
		if err != nil || id.Service == "" {
			return "", svcInfo{}, false
		}
		return id.Service, id.info(), true
	}
	svcs, err := fetchServices(hc, baseURL)
	if err != nil || len(svcs) == 0 {
		return "", svcInfo{}, false
	}
	cands := serviceCandidates(dir)
	if resolved, ok := resolveFromServices(svcs, dir); ok {
		cands = mergeCandidates(resolved, cands)
	}
	for _, cand := range cands {
		lookup := strings.ToLower(cand.name)
		if info, ok := svcs[lookup]; ok {
			return lookup, info, true
		}
	}
	return "", svcInfo{}, false
}

// ─── the session-hook --event branches ──────────────────────────────────

// runAdvisoryEvent is the shared body of every injecting wave-3 event:
// decode, render, beat, encode, exit 0. render is handed the decoded
// envelope and returns the payload ("" ⇒ inject nothing). Every failure
// path — nil stdin, unreadable stdin, a malformed envelope, a codec that
// cannot encode this event — records nothing and still exits 0.
func runAdvisoryEvent(codec harness.Codec, ev harness.Event, channel string, stdin io.Reader, stdout io.Writer, render func(harness.HookInput) string) int {
	if stdin == nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<16))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(ev, raw)
	if err != nil {
		return 0
	}
	payload := render(in)
	meta := map[string]string{"emitted": boolStr(payload != "")}
	if payload != "" {
		meta["bytes"] = fmt.Sprintf("%d", len(payload))
	}
	emitBeat(channel, string(ev), firingMeta(meta, in.SessionID, ""))
	if payload == "" {
		return 0
	}
	out, err := codec.EncodeOutput(ev, harness.HookOutput{Context: payload})
	if err != nil {
		return 0
	}
	if len(out) == 0 {
		// The codec declined stdout for this event (WorktreeCreate's
		// stdout is the worktree path, O34): the advisory still exists,
		// on stderr, where a verbose harness shows it and nothing parses it.
		fmt.Fprintln(os.Stderr, payload)
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// eventProjectDir is the root an advisory event is about: the event's own
// path field where it has one, else the harness's project dir, else the
// envelope's cwd.
func eventProjectDir(codec harness.Codec, in harness.HookInput) string {
	if in.Path != "" {
		return in.Path
	}
	if p := harnessProjectDir(codec.ID()); p != "" {
		return p
	}
	return in.Cwd
}

// isWorktreeDir reports whether dir is a linked git worktree: its `.git` is
// a file (gitdir pointer), not a directory. Cheap, no subprocess.
func isWorktreeDir(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && !st.IsDir()
}

// subagentIdentity is the service a SubagentStart primes for (RT-133,
// §11.3): the parent session's SessionStart resolution from its stamp when
// the subagent runs in the same directory — zero RPCs — else the cached
// resolver's lookup.
func subagentIdentity(baseURL, projDir, parentSessionID string) (string, svcInfo, bool) {
	if st := readSessionStamp(parentSessionID); st.Identity != nil && st.Identity.Service != "" && st.Cwd != "" &&
		strings.TrimRight(st.Cwd, "/") == strings.TrimRight(projDir, "/") {
		emitBeat("ident-cache", "parent-stamp", nil)
		return st.Identity.Service, st.Identity.info(), true
	}
	return indexedServiceFor(&http.Client{Timeout: 2 * time.Second}, baseURL, projDir)
}

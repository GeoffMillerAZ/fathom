package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/assess"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// stopHookVerifyTimeout bounds each verify_plan HTTP call the Stop hook
// makes. It is sized to verify_plan's own p95 latency SLO
// (docs/design/coding-agent-excellence.md §2: p95 ≤ 5s) rather than the
// blanket 2s convention other, cheap hook calls use (§5.3) — verify_plan is
// explicitly budgeted as the one expensive companion tool (B3,
// docs/plans/falsifiability.md). A call that exceeds this degrades to an
// advisory warning (fail-open, §5.3: hooks never block on infrastructure)
// instead of stalling session end.
const stopHookVerifyTimeout = 5 * time.Second

// cmdStopHook implements "stop-hook": a Claude Code Stop hook that checks
// whether any plan task was newly claimed ([x]) during this session without
// verifying against the index (ADR-018 §4 — done means proven, at session
// granularity).
//
// Output policy (mirrors edit-hook doctrine — hooks never block on
// operational failure):
//   - ANY error / daemon-down / no git / no plans → print nothing, exit 0.
//   - stop_hook_active true → exit 0 (no re-entry loops).
//   - Newly claimed tasks all verify (or are Unknown) → exit 0 silently.
//   - Newly claimed tasks FAIL verification → advisory additionalContext by
//     default; {"decision":"block"} with --block.
func cmdStopHook(args []string, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // FATHOM_CHANNELS=off (channels.go): total silence, before any other work
	}
	// NOT assistiveChannelsDisabled: under an OFF arm this hook must still
	// reach recordSessionLedgerEntry below. The ledger row IS the
	// counterfactual data — skipping it made the OFF arm collect nothing.
	rpcOrigin = "stop-hook"
	fs := newFlagSet("stop-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	block := fs.Bool("block", false, "block session end on disproven claims (advisory otherwise)")
	event := fs.String("event", "", "the stop event: stop (default), or subagent-stop, which runs the claim check alone")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 0 // silent (hook contract)
	}
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0 // I9: an unknown harness id is one stderr line, never a blocked stop
	}
	switch *event {
	case "", "stop":
	case "subagent-stop":
		// RO-38: the claim check alone under its own 2 s descriptor, never
		// Stop's verify, ledger or transcript work.
		return cmdSubagentStopHook(codec, *url, os.Stdin, stdout)
	default:
		return 0 // an unknown event is silence (I9), never a blocked stop
	}
	endGuard, silent := beginHookGuard("stop-hook", "fathom stop-hook", codec.ID(), *url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0
	}
	hook, err := codec.DecodeInput(harness.EventStop, raw)
	if err != nil {
		return 0
	}
	rpcSession = hook.SessionID // O1151: the daemon sees this hook session
	projDir := stopHookProjDir(codec, hook)
	if hook.ReEntrant {
		// RO-39 prints at most once per stop chain (RO-V1: Stop's context
		// re-opens the turn): a repeat is logged, never printed.
		checkStopClaims(claimCheckInputFor(projDir, hook, false))
		return 0
	}

	// The verify pass runs FIRST so the ledger row can carry its outcome
	// (IO-15: verify_catches — nil when verify never ran, which is
	// "unknown", never a fabricated zero). The ledger append itself still
	// swallows every failure internally, exactly like emitBeat, so a
	// ledger problem can never affect the verification flow.
	// Capability enforcement (capabilityenforce.go): `capabilities set
	// verify-gate off` skips the gate but STILL records the ledger row —
	// switching off an assistive gate is not a reason to stop measuring.
	// verifyCatches stays nil: the gate did not run, so its result is
	// unknown, never a fabricated zero (law 3).
	if capabilityOffAt(projDir, "verify-gate") {
		recordSessionLedgerEntry(codec.ID(), hook.SessionID, hook.TranscriptPath, projDir, time.Now(), nil)
		runStopHookRetro(projDir, hook.SessionID)
		return 0
	}

	if armOffActive() {
		// Control group: measure, do not assist. The ledger row is derived
		// entirely from the transcript, so it still carries this session's
		// raw-discovery counts — exactly the comparison data the arm
		// exists to collect — while verify (an assistive gate whose daemon
		// calls would also pollute the call log with fathom activity
		// during an unaided session) is skipped.
		//
		// verifyCatches is nil, not 0: verify did not run, so its result
		// is UNKNOWN. A fabricated zero here would read as "the gate found
		// nothing", which is a different and false claim (law 3).
		recordSessionLedgerEntry(codec.ID(), hook.SessionID, hook.TranscriptPath, projDir, time.Now(), nil)
		// IT-13: retro is counting/artifact work, not assistance — it
		// writes a local file and injects nothing into the session, so
		// the OFF arm (which silences assistance, never measurement) must
		// not silence it either. The env kill switch above already
		// returned before this line runs at all, which is the ONLY case
		// that skips it.
		runStopHookRetro(projDir, hook.SessionID)
		return 0
	}

	hc := newStopHookClient()
	advice := checkStopClaims(claimCheckInputFor(projDir, hook, false)) // RO-39: local reads only, before verify
	exit, catches := runStopHookWithAdvice(codec, projDir, hc, *url, *block, advice, stdout)
	recordSessionLedgerEntry(codec.ID(), hook.SessionID, hook.TranscriptPath, projDir, time.Now(), catches)
	runStopHookRetro(projDir, hook.SessionID)
	return exit
}

// stopHookProjDir is the project a stop event is about: the harness's
// project-dir variable, else the input's cwd, else the process's.
func stopHookProjDir(codec harness.Codec, in harness.HookInput) string {
	projDir := harnessProjectDir(codec.ID())
	if projDir == "" {
		projDir = in.Cwd
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}
	return projDir
}

// runStopHookRetro mechanically lands one retro artifact per session
// (IT-13, docs/design/intervention-outcomes.md IR-2): `fathom retro
// --write`, in-process (cmdRetro is a same-package function call, not a
// subprocess), scoped to THIS session id explicitly rather than retro's
// own "most recent stamped session" default — the hook already knows the
// exact session that just ended, so there is no ambiguity to resolve.
// LLM-free, zero tokens: this is disk + git only.
//
// Non-fatal by construction: any failure (nothing to retro, an unreadable
// beat log, a git error) is exactly what cmdRetro already reports as an
// advisory line on the writer it's given — swallowed here (io.Discard)
// because a failed retro must never break the Stop hook's own silent-exit
// contract, and cmdRetro's own return code is never propagated to the
// hook's exit code.
func runStopHookRetro(projDir, sessionID string) {
	if hookDeadlineSpent() {
		// RT-7 (§11.1): past the deadline a hook skips its local work rather
		// than run past its budget. The retro rewrites this session's one
		// artifact each Stop (a whole beat-log read), so the next Stop lands
		// it; the ledger row, which is measurement, is not skipped.
		return
	}
	args := []string{"--project", projDir, "--write"}
	if sessionID != "" {
		args = append(args, "--session", sessionID)
	}
	_ = cmdRetro(args, io.Discard, io.Discard)
}

// sessionLedgerBeatTailBytes bounds how much of the beat log
// recordSessionLedgerEntry reads — critic finding F7 (phase-P fix round):
// THIS session's beats (sumSessionBeats filters by session_id) are, by
// definition, among the most recently written ones, so a full-file
// heartbeat.Load on every session end is unbounded cost for a bounded
// need. 4MB comfortably covers a very active session's own beats.
const sessionLedgerBeatTailBytes = 4 << 20

// recordSessionLedgerEntry resolves a local (daemon-free) repo label,
// summarizes the session transcript (best-effort), sums this session's
// instrument A/B beats, and appends the resulting row. harnessID is the
// hook codec's id (WI-9w): the row records which harness ran the session.
func recordSessionLedgerEntry(harnessID, sessionID, transcriptPath, projDir string, now time.Time, verifyCatches *int) {
	repo := ""
	if cands := serviceCandidates(projDir); len(cands) > 0 {
		repo = strings.ToLower(cands[0].name)
	}
	// RT-132 (§11.3): only what the transcript gained since this session's
	// last Stop is read; the stamp carries the fold. A Codex session's
	// transcript_path is its rollout, folded on the harness port (PE-14).
	var ts transcriptSummary
	if harnessID == harness.Codex {
		ts = readCodexRolloutFromOffset(sessionID, transcriptPath)
	} else {
		ts = readTranscriptFromOffset(sessionID, transcriptPath)
	}
	beats, _ := readBeatLogTail(beatLogPath(), sessionLedgerBeatTailBytes)
	e := buildSessionLedgerEntry(sessionID, repo, ts, beats, now)
	e.Harness = harnessID
	e.VerifyCatches = verifyCatches
	// IR-4: a session running inside an assess judge inherits the spend
	// label; stamping it is what keeps judging spend out of every unit.
	e.SpendLabel = strings.TrimSpace(os.Getenv(assess.SpendLabelEnv))

	// The amended unit (production-tokenomics.md §5, 2026-08-12): a session
	// that began BEFORE the arm still contributes its arm-covered WINDOW,
	// measured from the same transcript with the same estimators but
	// restricted to lines after the arm's set_at. The owner's practice is
	// one session resumed across days; a unit that only fresh sessions
	// could satisfy produced a control arm that could never admit anything.
	if e.ArmCoverage == "partial" {
		if st, active := loadArmState(now); active {
			if since, perr := time.Parse(time.RFC3339, st.SetAt); perr == nil {
				if wsum, wtrunc, werr := windowSummaryFrom(transcriptPath, since, transcriptReadMaxBytes); werr == nil {
					e.ArmWindow = buildArmWindow(wsum, since, wtrunc)
				}
			}
		}
	}
	appendSessionLedger(e)
}

// runStopHook is the testable core of cmdStopHook: everything after stdin
// parsing and project-dir resolution.
// newStopHookClient builds the verify-sized HTTP client (stopHookVerifyTimeout,
// sized to verify_plan's own p95 SLO).
func newStopHookClient() *http.Client {
	return &http.Client{Timeout: stopHookVerifyTimeout}
}

// runStopHook returns its hook exit code and, when the verify pass actually
// ran against at least one plan, the number of proven-failing newly-claimed
// tasks it caught (nil = verify never ran — unknown, not zero; IO-15).
func runStopHook(projDir string, hc *http.Client, baseURL string, block bool, stdout io.Writer) (int, *int) {
	return runStopHookWith(harness.ClaudeCodec(), projDir, hc, baseURL, block, stdout)
}

// runStopHookWith is runStopHook with the codec that renders the verdict
// in the calling harness's envelope; runStopHook (the pre-seam signature
// every test drives) is the Claude Code instance.
func runStopHookWith(codec harness.Codec, projDir string, hc *http.Client, baseURL string, block bool, stdout io.Writer) (int, *int) {
	return runStopHookWithAdvice(codec, projDir, hc, baseURL, block, "", stdout)
}

// runStopHookWithAdvice is runStopHookWith with advice, RO-39's claim-check
// lines, appended to whatever the verify pass says, so Stop prints one
// envelope. advice never blocks: only a proven-failing claim may, behind
// --block.
func runStopHookWithAdvice(codec harness.Codec, projDir string, hc *http.Client, baseURL string, block bool, advice string, stdout io.Writer) (int, *int) {
	msg, proven, catches := stopHookVerify(projDir, hc, baseURL)
	text := msg
	if advice != "" {
		if text != "" {
			text += "\n\n"
		}
		text += advice
	}
	if text == "" {
		return 0, catches
	}
	// A timeout is operational, never evidence of a false claim — it must
	// stay advisory even when --block is set (§5.3: hooks never block on
	// infrastructure). Only a proven-failing claim may block.
	verdict := harness.HookOutput{Context: text}
	if block && proven {
		verdict = harness.HookOutput{Block: true, Reason: text}
	}
	out, err := codec.EncodeOutput(harness.EventStop, verdict)
	if err != nil {
		return 0, catches // silence over a malformed envelope (§5.3)
	}
	fmt.Fprintln(stdout, string(out))
	return 0, catches
}

// stopHookVerify is the Stop hook's verify of newly claimed plan rows: the
// message to inject ("" for silence), whether a claim is proven failing,
// and the verify catch count (nil when verify never ran).
func stopHookVerify(projDir string, hc *http.Client, baseURL string) (string, bool, *int) {
	plansDir := filepath.Join(projDir, "docs", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return "", false, nil // no plans → nothing to prove
	}

	// The local scan runs first, and the daemon is asked nothing until it
	// finds a plan that gained a claim: a Stop fires every turn and almost
	// none of them claim anything, so a session that claimed nothing costs
	// one git process and no RPC, however many plans the repository has
	// (O140: the per-plan `git show` and the identity round trip put the
	// hook's p99 past its deadline).
	claims := plansWithNewClaims(projDir, entries)
	if len(claims) == 0 {
		return "", false, nil
	}

	// Resolve the service identity: first candidate the daemon has indexed
	// wins — same discipline as the edit-hook.
	service := ""
	for _, cand := range candidatesWithDaemon(hc, baseURL, projDir) {
		name := strings.ToLower(cand.name)
		ok, probeErr := daemonServiceIndexed(hc, baseURL, name)
		if probeErr != nil {
			return "", false, nil // daemon down → silent
		}
		if ok {
			service = name
			break
		}
	}
	if service == "" {
		return "", false, nil // unindexed project → silent
	}

	var complaints, timedOutPlans, busyPlans []string
	verifiedAny := false
	for _, c := range claims {
		rel, newMD, newly := c.rel, c.md, c.newly
		localPlan := verify.ParseMarkdownPlan(service, newMD)
		rep, err := verifyViaHTTP(hc, baseURL, service, "", newMD, localPlan, goneTreeFor(projDir, localPlan))
		if err != nil {
			if isTimeoutErr(err) {
				// B3-1: a slow verify degrades to a warning for THIS plan —
				// fail open, never silent, never a stall — but other plan
				// files still get a chance to verify.
				timedOutPlans = append(timedOutPlans, rel)
				continue
			}
			if strings.Contains(err.Error(), core.ErrSourcesBusy.Error()) {
				// RT-146: the daemon skipped this hook's whole-tree read
				// (spec coverage needs it). Operational, never evidence —
				// named as an advisory, like a timeout.
				busyPlans = append(busyPlans, rel)
				continue
			}
			return "", false, nil // other operational errors (daemon down, bad response) → silent
		}
		verifiedAny = true
		// PC-22: every hook verify leaves a ledger row with its origin,
		// so the claim-moment reading (PC-26) and the stop-hook reading
		// are separable — best effort, never part of the verdict.
		appendVerifyLedgerFrom(verifyOriginStop, projDir, []string{rel}, service, []verify.Report{rep},
			verifyLedgerProvenance(hc, baseURL, "", service))
		// Unknown never fires the hook — fail-open honesty (claimRefusals,
		// shared with the claim-moment path in edithook_claim.go).
		complaints = append(complaints, claimRefusals(projDir, rel, newly, rep)...)
	}
	var catches *int
	if verifiedAny {
		n := len(complaints)
		catches = &n
	}
	if len(complaints) == 0 && len(timedOutPlans) == 0 && len(busyPlans) == 0 {
		return "", false, catches
	}
	sort.Strings(complaints)
	sort.Strings(timedOutPlans)
	sort.Strings(busyPlans)

	var sections []string
	if len(complaints) > 0 {
		sections = append(sections, "⛔ done-means-proven: task(s) claimed [x] this session fail verification:\n• "+
			strings.Join(complaints, "\n• ")+
			"\nProve the claim (fix the code / contract) or uncheck it before ending the session.")
	}
	if len(timedOutPlans) > 0 {
		sections = append(sections, fmt.Sprintf(
			"⚠️ done-means-proven: verify timed out (>%s) for %s — newly claimed task(s) there could not be confirmed this session; treat them as unconfirmed until verify runs clean.",
			stopHookVerifyTimeout, strings.Join(timedOutPlans, ", "),
		))
	}
	if len(busyPlans) > 0 {
		sections = append(sections, fmt.Sprintf(
			"⚠️ done-means-proven: %s for %s — newly claimed task(s) there could not be confirmed this session; treat them as unconfirmed until verify runs clean.",
			core.ErrSourcesBusy.Error(), strings.Join(busyPlans, ", "),
		))
	}
	return strings.Join(sections, "\n\n"), len(complaints) > 0, catches
}

// claimedPlan is a plan file that gained claimed tasks since HEAD: its path
// relative to the project, its working-tree text, and the ids that are new.
type claimedPlan struct {
	rel   string
	md    []byte
	newly []string
}

// plansWithNewClaims is the Stop hook's local scan: the plans under
// docs/plans whose working tree claims a task HEAD did not. It sends no RPC
// and spawns one git process for all of them (headFileVersions). A plan
// whose text has no `[x]` claims nothing and is not compared at all.
// Task ids do not depend on the service name, so the scan needs none.
func plansWithNewClaims(projDir string, entries []os.DirEntry) []claimedPlan {
	type candidate struct {
		rel string
		md  []byte
	}
	var cands []candidate
	var rels []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.EqualFold(e.Name(), "README.md") {
			continue
		}
		rel := filepath.Join("docs", "plans", e.Name())
		md, err := os.ReadFile(filepath.Join(projDir, rel))
		if err != nil {
			continue
		}
		if !bytes.Contains(md, []byte("[x]")) && !bytes.Contains(md, []byte("[X]")) {
			continue // no claimed task, so nothing newly claimed
		}
		cands = append(cands, candidate{rel, md})
		rels = append(rels, rel)
	}
	if len(cands) == 0 {
		return nil
	}
	heads := headFileVersions(projDir, rels) // nil per file: new file / no git
	var out []claimedPlan
	for _, c := range cands {
		if newly := newlyClaimedTasks(heads[c.rel], c.md, ""); len(newly) > 0 {
			out = append(out, claimedPlan{rel: c.rel, md: c.md, newly: newly})
		}
	}
	return out
}

// isTimeoutErr reports whether err (possibly wrapped, e.g. by verifyViaHTTP's
// fmt.Errorf("...: %w", err)) is a client-side timeout — either an
// *http.Client deadline (surfaced as a net.Error with Timeout() true,
// typically *url.Error wrapping context.DeadlineExceeded) or a bare
// context.DeadlineExceeded from a canceled request context.
func isTimeoutErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// newlyClaimedTasks returns the IDs of tasks that are claimed in newMD but
// were not claimed in oldMD (oldMD nil ⇒ every claimed task is new). Matching
// is by task ID: exact with stable `id:` keys, best-effort with positional
// IDs — acceptable for an advisory channel (ADR-018 §4).
func newlyClaimedTasks(oldMD, newMD []byte, service string) []string {
	oldClaimed := map[string]bool{}
	if oldMD != nil {
		for _, t := range verify.ParseMarkdownPlan(service, oldMD).Tasks {
			if t.Claimed {
				oldClaimed[t.ID] = true
			}
		}
	}
	var out []string
	for _, t := range verify.ParseMarkdownPlan(service, newMD).Tasks {
		if t.Claimed && !oldClaimed[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}

// headFileVersion returns rel's content at git HEAD, or nil when unavailable
// (new file, not a git repo, git missing).
func headFileVersion(repoDir, rel string) []byte {
	//fathom:tenancy-allow exec-noctx hook process bounded by its hookguard deadline, which kills the process tree; no request context to pass
	cmd := exec.Command("git", "-C", repoDir, "show", "HEAD:"+filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

// headFileVersions is headFileVersion for many files through one
// `git cat-file --batch` process: rel → its content at HEAD, nil when HEAD
// has no such file or git cannot answer (not a repository, git missing, the
// hook's deadline passed). One spawn however many files are asked for; the
// per-file `git show` it replaces cost a process each, which is what a
// loaded machine multiplies.
func headFileVersions(repoDir string, rels []string) map[string][]byte {
	out := make(map[string][]byte, len(rels))
	for _, rel := range rels {
		out[rel] = nil
	}
	if len(rels) == 0 {
		return out
	}
	var in bytes.Buffer
	for _, rel := range rels {
		fmt.Fprintf(&in, "HEAD:%s\n", filepath.ToSlash(rel))
	}
	ctx, cancel := testRunContext() // the hook's deadline, or none outside a hook
	defer cancel()
	cmd, cleanup, err := gitro.Command(ctx, repoDir, "cat-file", "--batch")
	if err != nil {
		cleanup()
		return out
	}
	cmd.Stdin = &in
	raw, err := cmd.Output()
	cleanup()
	if err != nil {
		return out
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	for _, rel := range rels {
		header, err := r.ReadString('\n')
		if err != nil {
			break
		}
		header = strings.TrimSuffix(header, "\n")
		// "<object> missing" / "<object> ambiguous": no body follows.
		if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
			continue
		}
		// "<sha> <type> <size>", then size bytes and a newline.
		f := strings.Fields(header)
		if len(f) < 3 {
			break
		}
		n, err := strconv.Atoi(f[2])
		if err != nil || n < 0 {
			break
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			break
		}
		_, _ = r.ReadByte() // the newline after the body
		if f[1] == "blob" {
			out[rel] = body
		}
	}
	return out
}

// stopHookRunner runs `fathom stop-hook` (registered below; EW-45).
type stopHookRunner struct{}

func (stopHookRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdStopHook(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "stop-hook", Group: "hooks", Order: 60, Run: stopHookRunner{},
		Summary: "Stop hook: flag tasks claimed [x] this session that fail verification"})
}

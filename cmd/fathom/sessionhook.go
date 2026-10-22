package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// cmdSessionHook implements the "session-hook" subcommand: a Claude Code
// SessionStart hook that primes each session to use fathom's index before
// falling back to Grep/Read.
//
// Telemetry across real fleet sessions showed fathom is dramatically
// under-used — agents reach for shell grep 10×+ more than fathom even where
// the index exists, because "index first" lives only in passive docs. This hook
// makes it active: at session start it pings the shared serve-http daemon, maps
// the project dir to a service, and emits a one-line reminder (or an honest
// "not available / not indexed" signal) into the session context.
//
// It prints the SessionStart hook JSON to stdout and always exits 0 (a context
// hook must never block session start). HTTP calls use a short timeout so a slow
// or down daemon can't stall startup.
func cmdSessionHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // FATHOM_CHANNELS=off (channels.go): total silence, before any other work
	}
	// NOT assistiveChannelsDisabled: under an OFF arm this hook must still
	// run, because it owns the loud arm announcement AND the session
	// decrement (see the arm branch below). Exiting here was the bug that
	// let an OFF arm run indefinitely without ever spending its budget.
	if ev := sessionHookEventArg(args); ev != "" {
		return cmdSessionHookEvent(ev, args, stdin, stdout, stderr) // PC-23/HP-26..HP-30: an event branch, never the primer and never the "fired" count (sessionhook_instructions.go)
	}
	rpcOrigin = "session-hook"
	// Stamp the harness BEFORE the first beat: the "fired" count is the
	// session denominator doctor and instrument read, and a beat written
	// before the codec is resolved would be the one beat of the session
	// that cannot say which harness produced it (hookharness.go).
	noteBeatHarness(args)
	emitBeat("session-hook", "fired", nil)
	fs := newFlagSet("session-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	// The codec is TOLD which harness called (ADR-029 rule 6: hooks never
	// sniff); absent ⇒ claude-code, the wired default. An unknown id is
	// I9 fail-open: one stderr line, exit 0.
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0
	}
	// session_id (docs/design/production-tokenomics.md §6): read
	// best-effort from the SessionStart hook JSON through the codec — the
	// same envelope edit-hook/stop-hook decode. Absent/unreadable/malformed
	// stdin degrades to sessionID="" (no session-scoped tagging), never a
	// failure of the session brief itself.
	var sessionID, startSource string
	if stdin != nil {
		raw, _ := io.ReadAll(io.LimitReader(stdin, 1<<16))
		if in, err := codec.DecodeInput(harness.EventSessionStart, raw); err == nil {
			sessionID = in.SessionID
			rpcSession = in.SessionID // O1151: the daemon sees this hook session
			startSource = in.Source
		}
	}

	// Context loss resets dedupe even when the daemon guard declines RPC.
	resetHookDedupe(sessionID, startSource, nowFunc())
	endGuard, silent := beginHookGuard("session-hook", "fathom session-hook", codec.ID(), *url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()

	projDir := *dir
	if projDir == "" {
		projDir = harnessProjectDir(codec.ID())
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}
	stdout, finishHealth := beginAdviceHealth(projDir, codec.ID(), harness.CapPrime, stdout)
	defer finishHealth()

	ctx, primerIdentity := sessionContext(*url, projDir)
	// RT-7 (§11.1): a hung daemon spends the whole deadline in the line
	// above. Past it, the outbox sync and the stamp's identity lookup (each
	// only feeds an RPC the guard now refuses) and the continuity brief are
	// skipped — the brief with a note naming where to get it — and the
	// hook writes its records and exits at its budget.
	spent := hookDeadlineSpent()

	// FL-22 (docs/vision/30-findings-loop.md §3.7): the outbox sync, gated
	// to the SAME startup/clear/"" sources as the continuity brief below —
	// a resumed or compacted session already has this session's context,
	// so re-syncing there would be the schema-tax mistake in a new
	// costume, not a real session start. syncFindingsOutbox itself is the
	// "never more than one attempt" gate (the digest cursor short-circuits
	// a repeat call once it has been written).
	if !spent && (startSource == "startup" || startSource == "clear" || startSource == "") {
		if line := syncFindingsOutbox(*url, projDir); line != "" {
			ctx += "\n" + line
		}
	}

	// Session windows (design point 2, docs/vision/28-receipts.md §4): a
	// mechanical {session_id, cwd, service, started} row under
	// logs/sessions.jsonl in the repo's state directory (statedir.Resolve
	// — outside the repo tree by default, O63 item 5) — SubagentStart
	// looks its parent's
	// start up here (subagentDiscoveries), and the session-end hook looks
	// its OWN start up here (sessionEndPointers). Best-effort: the service
	// resolution reuses the same daemon-backed lookup the advisory events
	// already make, so this costs one extra round trip only when a
	// session id exists to tag it with.
	var stampIdentity *identityResult
	if sessionID != "" {
		svc := ""
		if !spent { // past the deadline the lookup's RPC would be refused
			if name, info, ok := indexedServiceFor(&http.Client{Timeout: 2 * time.Second}, *url, projDir); ok {
				svc = name
				stampIdentity = &identityResult{Schema: 1, Service: name, Symbols: info.symbols, CountErr: info.countErr, Langs: info.langs, Path: info.path}
			}
		}
		recordSessionWindow(projDir, sessionWindowRow{
			SchemaVersion: 1, SessionID: sessionID, Cwd: projDir, Service: svc,
			Started: time.Now().UTC().Format(time.RFC3339),
		})
	}

	// Continuity (vision 21, TH-3): the thread brief joins the session
	// context ONLY where context was actually lost — startup and clear.
	// resume/compact keep their context; injecting there is the schema-tax
	// mistake in a new costume. The stamp records the cohort label and the
	// brief's measured cost either way, which is what makes the
	// pre-registered bet evaluable (fresh-with-brief vs resumed).
	briefBytes, attentionBytes, inactiveBytes := 0, 0, 0
	intentLine := false
	if startSource == "startup" || startSource == "clear" || startSource == "" {
		// CC-7 (docs/plans/custodian.md) + EP-19: the continuity brief and
		// the one attention line — present only while this repo has a
		// violated concern or a recorded unproven claim. Mechanical file
		// reads; adds nothing on clean or unconfigured repos. Same
		// startup/clear gate as the thread brief: resumed sessions kept
		// their context. The whole primer is capped here (EP-20).
		if spent {
			ctx += "\n" + primerDeadlineNote
		} else {
			var line string
			// PE-25: the attention line's rows under the registered roots,
			// decided from the identity this hook already resolved.
			if primerIdentity == nil {
				primerIdentity = stampIdentity
			}
			inactive := inactiveFeaturesLine(projDir, codec.ID(), nowFunc())
			ctx, briefBytes, line = composePrimer(ctx, projDir, nowFunc(), attentionLedgerRoots(projDir, primerIdentity), inactive)
			if inactive != "" && strings.Contains(line, inactive) && strings.Contains(ctx, line) {
				inactiveBytes = len(inactive)
			}
			attentionBytes = len(line)
			intentLine = line != ""
			if intentLine {
				emitBeat("custodian", "attention", firingMeta(nil, sessionID, ""))
			}
		}
	}
	writeSessionStamp(sessionStamp{
		SchemaVersion: 1,
		SessionID:     sessionID,
		Source:        startSource,
		BriefInjected: briefBytes > 0,
		BriefBytes:    briefBytes,
		IntentLine:    intentLine,
		At:            time.Now().UTC().Format(time.RFC3339),
		Cwd:           projDir,
		Identity:      stampIdentity,
	})

	// EP-24: the attention line's bytes as a COMPONENT of the primer, so
	// the rent model has a per-channel number without the channel table
	// double-counting bytes it already summed under session-brief.
	var component map[string]string
	if attentionBytes > 0 {
		component = map[string]string{"attention_bytes": strconv.Itoa(attentionBytes)}
		if inactiveBytes > 0 {
			component["inactive_features_bytes"] = strconv.Itoa(inactiveBytes)
		}
	}
	emitPayloadCost("session-brief", ctx, 0, "", sessionID, component)

	// The codec renders the harness's envelope; for Claude Code these are
	// the exact bytes json.NewEncoder(...).Encode produced before the seam.
	out, err := codec.EncodeOutput(harness.EventSessionStart, harness.HookOutput{Context: ctx})
	if err != nil {
		return 0 // a codec that cannot answer is silence, never a failed session start
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

// sessionPrimerBudget caps the WHOLE SessionStart primer (EP-20, ADR-026):
// identity + freshness line, thread brief, the repo-wide invariants line
// (IV-3c), attention line. Each part already
// carries its own budget, but three budgets do not compose into one — the
// only number an adopter can reason about is the total that lands in every
// session. I12 (docs/intent/invariants.md) is the invariant this enforces.
const sessionPrimerBudget = 3200

// primerDeadlineNote replaces the continuity brief and attention line when
// the hook's deadline was spent before they were composed (RT-7): named,
// with the command that prints them, never silently absent.
const primerDeadlineNote = "(thread brief and attention line skipped — this hook's deadline was spent waiting on the daemon; `fathom prime` prints them)"

// primerOverflowNote is appended when even the identity line alone exceeds
// the cap — the last-resort cut, never silent.
const primerOverflowNote = " …[session primer truncated at its byte cap]"

// composeContinuityAndCustodian renders the full SessionStart primer for
// projDir: the identity/freshness line it is handed, then the continuity
// thread brief, then the one attention line (EP-19). Shared by cmdSessionHook
// (gated to startup/clear/"" sources) and `fathom prime` (WF-8 — always-on
// pull) so neither duplicates the other's string-building or its cap.
//
// The result is capped at sessionPrimerBudget with the cut NAMED (law 3):
// the thread brief absorbs the overflow by whole lines from the end, because
// identity is why the hook exists and the attention line is the smallest,
// highest-signal part. Returns the brief's byte count as it was ACTUALLY
// injected (post-cut — the session stamp must not record bytes that never
// reached the model) and whether the attention line fired.
//
// roots are the registered service roots the attention line reads the
// intent ledger under (PE-25; attentionLedgerRoots).
func composePrimer(identity, projDir string, now time.Time, roots ledgerroots.Roots, featureLines ...string) (primer string, briefBytes int, attention string) {
	items := loadThreadItems(threadsPath())
	var mapLines []string
	if threadsHoldMaps() {
		// XM-12/XM-13: the brief names at most three maps, and re-surfaces the
		// ones an open note (or the pre-compact pointer) references.
		items = evaluateThreadConditions(items, projDir, now)
		mapLines = briefMapLines(items, projDir, now)
	}
	brief := renderThreadBrief(items, projDir, threadBriefBudgetBytes, now, mapLines...)
	line := attentionLine(projDir, now, roots, featureLines...)
	// RT-51: the tenancy line, only when the daemon is not normal (≤160 B).
	tenancyLine := primerTenancyLine(now)
	// IV-3c: repo-wide invariants, once, here and never at an edit (≤480 B).
	invariantsLine := repoWideInvariantLine(projDir)

	join := func(b string) string {
		out := identity
		if tenancyLine != "" {
			out += "\n" + tenancyLine
		}
		if b != "" {
			out += "\n" + b
		}
		if invariantsLine != "" {
			out += "\n" + invariantsLine
		}
		if line != "" {
			out += "\n" + line
		}
		return out
	}

	primer = join(brief)
	if len(primer) > sessionPrimerBudget && brief != "" {
		lines := strings.Split(brief, "\n")
		dropped := 0
		for len(lines) > 0 && len(primer) > sessionPrimerBudget {
			lines = lines[:len(lines)-1]
			dropped++
			trimmed := strings.Join(lines, "\n")
			trimmed = strings.TrimPrefix(trimmed+fmt.Sprintf("\n… %d brief line(s) cut to fit the %d B session primer — `fathom thread brief` for all of them", dropped, sessionPrimerBudget), "\n")
			brief = trimmed
			primer = join(brief)
		}
	}
	if len(primer) > sessionPrimerBudget {
		// Identity alone is over the cap. That is a bug in whoever produced
		// it, not a reason to inject an over-budget primer OR to truncate
		// silently — cut with the reason attached.
		keep := sessionPrimerBudget - len(primerOverflowNote)
		primer = primer[:keep] + primerOverflowNote
	}
	return primer, len(brief), line
}

// cmdPrime implements the "prime" subcommand (WF-8): prints the full
// session primer — the exact content the SessionStart hook composes into
// its additionalContext — as plain text to stdout, with no hook-JSON
// envelope. This is the pull-tier equivalent for a harness with no
// SessionStart hook (Codex): one command gets what Claude Code gets
// pushed. Always exits 0, even when the daemon is unreachable —
// buildSessionContext already degrades to an honest status line rather
// than an error, and priming must never fail a session open.
//
// `--routes` (SR-29, surface reckoning F-47, design §4) routes to
// cmdRoutes instead: `routes` is no longer its own top-level verb (0
// lifetime calls as one), so `fathom prime --routes [package|concern]` is
// the one entry point now. Detected by a plain scan BEFORE prime's own
// flag set parses args — cmdRoutes owns its own disjoint flag set
// (service/root/store/call-log/json), which prime's flags do not and must
// not shadow — so every existing `--url`/`--dir` prime caller, and every
// existing `routes` flag/arg, is unaffected.
func cmdPrime(args []string, stdout, stderr io.Writer) int {
	for i, a := range args {
		if a == "--routes" || a == "-routes" {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return cmdRoutes(rest, stdout, stderr)
		}
	}
	fs := newFlagSet("prime", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rpcOrigin = "prime" // after Parse: help runs flag parsing alone (O671)

	projDir := *dir
	if projDir == "" {
		// The pull tier serves every harness: whichever descriptor's
		// project-dir variable is set names the repo.
		projDir = anyHarnessProjectDir()
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	primerLine, id := sessionContext(*url, projDir)
	ctx, _, _ := composePrimer(primerLine, projDir, time.Now(), attentionLedgerRoots(projDir, id))
	fmt.Fprintln(stdout, ctx)
	return 0
}

func defaultFathomURL() string {
	if u := os.Getenv("FATHOM_HTTP"); u != "" {
		return u
	}
	// RT-17/RT-18 (§10.4): a test binary, or a process a test started
	// (FATHOM_TEST=1), never falls back to the operator's daemon.
	return testisolate.DaemonURLBackstop("http://localhost:7575")
}

type svcInfo struct {
	symbols  int
	countErr string
	langs    string
	path     string // recorded corpus root; "" when the daemon has none
}

// sessionContext returns the additionalContext line for the project at
// projDir, and the service identity it resolved (nil when none was: the
// daemon did not answer, or an OFF arm is active) — the attention line
// decides from it whether projDir is a registered root (PE-25). Service identity tries the dir basename first, then the enclosing
// git repo's basename (projects rooted in a subdirectory like <repo>/deploy).
func sessionContext(baseURL, projDir string) (string, *identityResult) {
	hc := &http.Client{Timeout: 2 * time.Second}

	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		// A busy daemon (503 + Retry-After, held by the hook breaker) IS
		// responding: saying otherwise would send the agent to Grep for
		// the whole session over a few seconds of load (§11.2).
		var skip *hookguard.SkipError
		if errors.As(err, &skip) && skip.Verdict == hookguard.Open {
			// The shared breaker is open: other hooks already saw this
			// daemon fail and are shielding it. §11.2 says hooks stay
			// silent then (I9, P18) — the old "not responding … Restart
			// it" line told every new session to restart a daemon the
			// breaker was giving room to recover. The skip is recorded as
			// a breaker_open beat.
			return "", nil
		}
		if errors.As(err, &skip) && skip.Verdict == hookguard.Busy {
			return fmt.Sprintf("fathom daemon (%s) is busy right now (hooks back off until %s) — its MCP tools may refuse briefly with a Retry-After; retry them before falling back to Grep/Read.", baseURL, skip.Until.Local().Format("15:04:05")), nil
		}
		// This line reaches EVERY session of EVERY adopter, which is why it
		// may not name a label that exists on one machine.
		return fmt.Sprintf("⚠️ fathom code-intelligence daemon (%s) is not responding — its MCP tools are unavailable this session, so Grep/Read discovery is expected. Restart it with: %s", baseURL, daemonDownHint()), nil
	}
	resp.Body.Close()

	if st, active := loadArmState(time.Now()); active {
		// An active OFF arm must be LOUD (the O15 lesson): the session
		// opens by naming it, its bound, and the undo.
		bound := st.Until
		if st.SessionsRemaining != nil {
			bound += fmt.Sprintf(" / %d session(s) left", *st.SessionsRemaining)
		}
		defer decrementArmSessions()
		return fmt.Sprintf("🔬 fathom OFF arm active (measurement, vision 14 §2): channels are silent this session — no obligations, no guards, no verify gate. Bound: %s. This is deliberate counterfactual data collection; end it early with `fathom arm on`.", bound), nil
	}

	// RT-133 (§11.3): the cached identity answers (SessionStart's stamp
	// lookup already made the one round trip); the full list is read only
	// when nothing matched (to name the indexed services) or the daemon
	// predates GET /identity.
	id, idErr := lookupIdentityCached(hc, baseURL, projDir)
	if idErr == nil && id.Service != "" {
		return sessionPrimerLine(hc, baseURL, projDir, id.Service, id.info()), &id
	}
	if idErr != nil && !errors.Is(idErr, errIdentityRouteMissing) {
		return fmt.Sprintf("⚠️ fathom daemon is healthy but service enumeration failed (%v) — index state is unknown this session; the MCP tools may still work, so try them before falling back to Grep/Read. If this repeats, check GET %s/statusz.", idErr, baseURL), nil
	}
	svcs, svcErr := fetchServices(hc, baseURL)
	if svcErr != nil {
		// O2: enumeration failure is NOT "not indexed". The daemon is healthy
		// (healthz just passed) but list_services failed — saying "this repo
		// is not an indexed service" here would be a confident false
		// instruction the agent has no reason to disprove. Degrade honestly.
		return fmt.Sprintf("⚠️ fathom daemon is healthy but service enumeration failed (%v) — index state is unknown this session; the MCP tools may still work, so try them before falling back to Grep/Read. If this repeats, check GET %s/statusz.", svcErr, baseURL), nil
	}
	cands := serviceCandidates(projDir)
	if resolved, ok := resolveFromServices(svcs, projDir); ok {
		cands = mergeCandidates(resolved, cands)
	}
	for _, cand := range cands {
		lookup := strings.ToLower(cand.name)
		info, ok := svcs[lookup]
		if !ok {
			continue
		}
		return sessionPrimerLine(hc, baseURL, projDir, lookup, info), &identityResult{Schema: 1, Service: lookup, Path: info.path}
	}

	repo := filepath.Base(strings.TrimRight(projDir, "/"))
	known := make([]string, 0, len(svcs))
	for k := range svcs {
		known = append(known, k)
	}
	sort.Strings(known)
	indexed := strings.Join(known, ", ")
	if len(known) == 0 {
		indexed = "none yet"
	}
	return fmt.Sprintf("ℹ️ fathom code-intelligence is live, but this repo (%q) is not an indexed service — its symbols aren't in the index, so Grep/Read discovery is expected here. Indexed services: %s. (Add this repo: run `fathom init` from its root.)", repo, indexed), &identityResult{Schema: 1}
}

// sessionPrimerLine is the live line for projDir, the indexed service
// lookup (with info).
func sessionPrimerLine(hc *http.Client, baseURL, projDir, lookup string, info svcInfo) string {
	langs := ""
	if info.langs != "" {
		langs = ", " + info.langs
	}
	ctxLine := fmt.Sprintf("✅ fathom code-intelligence is live — this repo is the indexed service %q (%s%s). Route by question shape: where a symbol is defined is find_symbol; who-calls it is symbol_context; what-breaks is blast_radius; a review verdict over a diff is review_diff; what-governs this repository is obligations with no path (the root conventions); is a plan's box proven is verify_plan. A bug report, stack trace, error string or failing test starts search -> symbol_context -> read_source. A literal string, a config value, a log line, or a document-versus-document contradiction is search's job (grep with a receipt). The staleness and what-was-known tools live on the /rpc/extended surface. Read files to *edit* them, not to discover them. The index refreshes itself when git HEAD drifts.", lookup, info.countText(), langs)
	// FR-4 (docs/plans/exploration-ledger.md): teach the FIRST query
	// with a LIVE symbol from this repo's own index — the first-query
	// experience decides adoption; one miss loses the session to grep.
	// Degraded-branch discipline holds: any failure omits the line.
	if ex := liveExampleLine(hc, baseURL, lookup); ex != "" {
		ctxLine += " " + ex
	}
	// The wiring tripwire (vision 14 §7 tier-1, IO-19): every drifted
	// session opens by telling its agent the exact remedy — detection
	// is free and universal; nothing mutates here.
	if d := analyzeWiring(projDir); len(d.Missing) > 0 || len(d.StaleCmds) > 0 {
		ctxLine += fmt.Sprintf(" ⚠️ wiring drift: %d missing hook template entr(ies), %d stale binary path(s) — run `fathom fleet reconcile --apply` (or /fathom:rewire) to reconcile in place.", len(d.Missing), len(d.StaleCmds))
	}
	// RC-30: routes named in ONE line only when at least one exists —
	// a cheap candidate count (no live-index resolution here; `fathom
	// routes` does that pass).
	if rl := routesSummaryLine(projDir, lookup, time.Now()); rl != "" {
		ctxLine += " " + rl
	}
	// FL-21 (docs/vision/30-findings-loop.md §3.4): one line naming
	// the repo-wide open finding-kind claim count, when any exist —
	// silent on a zero count or any fetch failure, matching this
	// hook's own degraded-branch discipline (never a reason to fail
	// or slow session start).
	if fl := findingsBriefLine(hc, baseURL, lookup); fl != "" {
		ctxLine += " " + fl
	}
	return ctxLine
}

// fetchServices calls the list_services MCP tool over HTTP and returns a map
// keyed by lower-cased service ID, plus an error when ENUMERATION failed —
// callers must distinguish "failed to enumerate" from "zero services" (O2:
// collapsing them once rendered a confident false not-indexed instruction
// for an indexed repo). One retry, on the caller's own client, and only for
// a refused or reset connection inside the hook's deadline (RT-8).
// inProcessRegistry is published by serve-http: a registry lookup aimed at
// the daemon's own URL is answered in process (the same
// mcp.ServiceSummaries list_services returns) instead of over loopback HTTP.
// It stays unset in every CLI process. One atomic pointer carries both
// fields: they are written once at startup and read from request and
// feeder goroutines, and a test that starts serve-http while an earlier
// test's goroutine still resolves a call log must not race (-race gate,
// 2026-09-26).
type inProcessRegistry struct {
	self     string
	services func(ctx context.Context) (map[string]svcInfo, error)
	// stale is the registry's stale disclosure ("" when fresh): a read
	// refused while shedding answers the last good projection (§3.4).
	stale func() string
	// statusz is the daemon's own /statusz report, built in process
	// (RT-115): a console page reads it without a loopback request.
	statusz func(ctx context.Context) StatuszReport
	// build is the daemon binary's stamp (nil: statuszBuildInfo, this
	// process's own — the value statusz reports; tests inject one).
	build func() StatuszBuildInfo
	// rpc answers one JSON-RPC message in process (the MCP server's
	// HandleRequest): the console's own tool calls — the agent surface's
	// tools/list, /learn/ask, plan verification — are admitted by the
	// server's gate like any call, without a loopback request (RT-115,
	// RT-116). The caller's context carries the surface and origin.
	rpc func(ctx context.Context, raw []byte) ([]byte, bool)
}

// inProcessDaemon is the in-process registry when baseURL names this
// daemon, else nil (a CLI process, or another daemon's URL).
func inProcessDaemon(baseURL string) *inProcessRegistry {
	if r := inProcessReg.Load(); r != nil && sameDaemonURL(baseURL, r.self) {
		return r
	}
	return nil
}

// inProcessCall runs one JSON-RPC message on this daemon's surface in
// process, as origin console-inprocess; ok is false when baseURL is not
// this daemon (the caller then asks over HTTP).
func inProcessCall(ctx context.Context, baseURL, surface string, raw []byte) (resp []byte, ok bool) {
	r := inProcessDaemon(baseURL)
	if r == nil || r.rpc == nil {
		return nil, false
	}
	ctx = mcp.WithOrigin(mcp.WithRequestScope(ctx, surface, true), mcp.OriginConsoleInProcess)
	resp, has := r.rpc(ctx, raw)
	if !has {
		return nil, true
	}
	return resp, true
}

var inProcessReg atomic.Pointer[inProcessRegistry]

// svcInfoFromSummaries converts list_services' identity rows into the
// registry map every caller of fetchServices reads, skipping malformed IDs
// exactly as the HTTP decode does.
func svcInfoFromSummaries(sums []mcp.ServiceSummary) map[string]svcInfo {
	out := make(map[string]svcInfo, len(sums))
	for _, s := range sums {
		if err := core.ValidServiceID(s.Service); err != nil {
			continue
		}
		out[strings.ToLower(s.Service)] = svcInfo{symbols: s.SymbolCount, countErr: s.CountErr, langs: joinLangs(s.Languages), path: s.Path}
	}
	return out
}

// sameDaemonURL reports whether baseURL names the daemon at self, ignoring a
// trailing slash and the localhost / 127.0.0.1 spelling.
func sameDaemonURL(baseURL, self string) bool {
	norm := func(u string) string {
		u = strings.TrimRight(strings.TrimSpace(u), "/")
		return strings.Replace(u, "://localhost:", "://127.0.0.1:", 1)
	}
	return self != "" && norm(baseURL) == norm(self)
}

func fetchServices(hc *http.Client, baseURL string) (map[string]svcInfo, error) {
	if r := inProcessReg.Load(); r != nil && r.services != nil && sameDaemonURL(baseURL, r.self) {
		return r.services(context.Background())
	}
	m, err := fetchServicesOnce(hc, baseURL)
	if err == nil || !hookguard.Retryable(err) {
		// RT-8 (§11.1): a timeout, a 5xx or a guard skip is never retried —
		// the old fresh 4 s retry client let a hook facing a hung daemon
		// spend its whole budget and then four seconds more.
		return m, err
	}
	// Refused or reset: the daemon may be mid-restart. One retry on the
	// SAME client, and only while the hook's deadline has time for it.
	if g := activeHookGuard(); g != nil && g.Deadline.Remaining() < fetchServicesMinRetry {
		return m, err
	}
	return fetchServicesOnce(hc, baseURL)
}

// fetchServicesMinRetry is the least deadline left that is worth a retry.
const fetchServicesMinRetry = 100 * time.Millisecond

func fetchServicesOnce(hc *http.Client, baseURL string) (map[string]svcInfo, error) {
	out := map[string]svcInfo{}
	// A nil client is a caller bug, but the caller is often a diagnostic
	// (doctor, flywheel) whose whole job is to run when things are broken —
	// so crashing here turns "one check could not resolve a path" into a
	// SIGSEGV that aborts the entire process. Worse in a test binary: a
	// panic aborts the WHOLE package run, so every test file sorting after
	// the panicking one never executes, and the gate reports on work it
	// never exercised. Default rather than dereference.
	if hc == nil {
		hc = &http.Client{Timeout: 4 * time.Second}
	}
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`)
	req, err := originHeader(baseURL, body)
	if err != nil {
		return out, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, fmt.Errorf("list_services: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	// Unwrap: {"result":{"content":[{"text":"<json>"}]}} or {"result":{services:...}}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Services []rawSvc `json:"services"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return out, fmt.Errorf("list_services: %w", err)
	}
	svcs := env.Result.Services
	if len(svcs) == 0 && len(env.Result.Content) > 0 {
		var inner struct {
			Services []rawSvc `json:"services"`
		}
		if err := json.Unmarshal([]byte(env.Result.Content[0].Text), &inner); err == nil {
			svcs = inner.Services
		}
	}
	for _, s := range svcs {
		if err := core.ValidServiceID(s.Service); err != nil {
			continue // skip malformed/path-like service IDs (e.g. "../app-kit")
		}
		out[strings.ToLower(s.Service)] = svcInfo{symbols: s.SymbolCount, countErr: s.SymbolCountError, langs: joinLangs(s.Languages), path: s.Path}
	}
	return out, nil
}

// liveExampleLine returns the "first query to try" teaching line (FR-4,
// docs/plans/exploration-ledger.md): one REAL symbol fetched from the
// service's own index over the agent surface, so the session's first
// fathom call cannot miss. "" on ANY failure — the hook must never break
// or slow session start, so this degrades silently (the same discipline
// as every other branch in this file), with its own 2s ceiling on top of
// whatever the caller's client allows.
func liveExampleLine(hc *http.Client, baseURL, service string) string {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_symbols","arguments":{"service":%q,"concern":"main","top":1}}}`, service)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/rpc/agent", strings.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := hc.Do(req.WithContext(ctx))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &env) != nil || len(env.Result.Content) == 0 {
		return ""
	}
	var inner struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	}
	if json.Unmarshal([]byte(env.Result.Content[0].Text), &inner) != nil || len(inner.Nodes) == 0 || inner.Nodes[0].Name == "" {
		return ""
	}
	return fmt.Sprintf("first query to try: find_symbol {\"service\": %q, \"name\": %q} — the index answers where/what/who-calls without reading files.", service, inner.Nodes[0].Name)
}

type rawSvc struct {
	Service          string   `json:"service"`
	SymbolCount      int      `json:"symbol_count"`
	SymbolCountError string   `json:"symbol_count_error"` // RT-41: the count was unreadable
	Languages        []string `json:"languages"`
	Path             string   `json:"path"` // recorded corpus root (authoritative repo↔service identity)
}

// countText is "N symbols", or the named reason the count could not be
// read (RT-41, I38: an unreadable count is never rendered 0).
func (s svcInfo) countText() string {
	if s.countErr != "" {
		return "symbol count unreadable: " + s.countErr
	}
	return fmt.Sprintf("%d symbols", s.symbols)
}

func joinLangs(l []string) string {
	if len(l) == 0 {
		return ""
	}
	return strings.Join(l, "/")
}

// sessionHookRunner runs `fathom session-hook` (registered below; EW-45).
type sessionHookRunner struct{}

func (sessionHookRunner) execVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cmdSessionHook(args, stdin, stdout, stderr)
}

// primeRunner runs `fathom prime` (registered below; EW-45).
type primeRunner struct{}

func (primeRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdPrime(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "session-hook", Group: "hooks", Order: 10, Run: sessionHookRunner{},
		Summary: "SessionStart hook: resolve this repo's service and emit a one-line reminder (or an honest not-indexed line); always exits 0"})
	// "routes" (SR-29, surface reckoning F-47) is no longer its own top-level
	// verb: cmdRoutes is reached only through prime's --routes flag.
	declareVerb(verb{Name: "prime", Group: "hooks", Order: 20, Run: primeRunner{},
		Summary: "Print the SessionStart context as plain text — the pull tier for a harness with no hook (Codex); always exits 0 | --routes [package|concern]: long-lived discovery paths (pointers established in 2+ sessions over 30 days), grouped by package"})
}

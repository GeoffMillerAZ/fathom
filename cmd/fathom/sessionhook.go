package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
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
			startSource = in.Source
		}
	}

	projDir := *dir
	if projDir == "" {
		projDir = harnessProjectDir(codec.ID())
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	ctx := buildSessionContext(*url, projDir)

	// FL-22 (docs/vision/30-findings-loop.md §3.7): the outbox sync, gated
	// to the SAME startup/clear/"" sources as the continuity brief below —
	// a resumed or compacted session already has this session's context,
	// so re-syncing there would be the schema-tax mistake in a new
	// costume, not a real session start. syncFindingsOutbox itself is the
	// "never more than one attempt" gate (the digest cursor short-circuits
	// a repeat call once it has been written).
	if startSource == "startup" || startSource == "clear" || startSource == "" {
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
	if sessionID != "" {
		svc := ""
		if name, _, ok := indexedServiceFor(&http.Client{Timeout: 2 * time.Second}, *url, projDir); ok {
			svc = name
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
	briefBytes, attentionBytes := 0, 0
	intentLine := false
	if startSource == "startup" || startSource == "clear" || startSource == "" {
		// CC-7 (docs/plans/custodian.md) + EP-19: the continuity brief and
		// the one attention line — present only while this repo has a
		// violated concern or a recorded unproven claim. Mechanical file
		// reads; adds nothing on clean or unconfigured repos. Same
		// startup/clear gate as the thread brief: resumed sessions kept
		// their context. The whole primer is capped here (EP-20).
		var line string
		ctx, briefBytes, line = composeContinuityAndCustodian(ctx, projDir, time.Now())
		attentionBytes = len(line)
		intentLine = line != ""
		if intentLine {
			emitBeat("custodian", "attention", firingMeta(nil, sessionID, ""))
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
	})

	// EP-24: the attention line's bytes as a COMPONENT of the primer, so
	// the rent model has a per-channel number without the channel table
	// double-counting bytes it already summed under session-brief.
	var component map[string]string
	if attentionBytes > 0 {
		component = map[string]string{"attention_bytes": strconv.Itoa(attentionBytes)}
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
// identity + freshness line, thread brief, attention line. Each part already
// carries its own budget, but three budgets do not compose into one — the
// only number an adopter can reason about is the total that lands in every
// session. I12 (docs/intent/invariants.md) is the invariant this enforces.
const sessionPrimerBudget = 3200

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
func composeContinuityAndCustodian(identity, projDir string, now time.Time) (primer string, briefBytes int, attention string) {
	brief := renderThreadBrief(loadThreadItems(threadsPath()), projDir, threadBriefBudgetBytes, now)
	line := attentionLine(projDir, now)

	join := func(b string) string {
		out := identity
		if b != "" {
			out += "\n" + b
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
			if trimmed != "" {
				trimmed += fmt.Sprintf("\n… %d brief line(s) cut to fit the %d B session primer — `fathom thread brief` for all of them", dropped, sessionPrimerBudget)
			}
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
func cmdPrime(args []string, stdout, stderr io.Writer) int {
	rpcOrigin = "prime"
	fs := newFlagSet("prime", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	projDir := *dir
	if projDir == "" {
		// The pull tier serves every harness: whichever descriptor's
		// project-dir variable is set names the repo.
		projDir = anyHarnessProjectDir()
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	ctx, _, _ := composeContinuityAndCustodian(buildSessionContext(*url, projDir), projDir, time.Now())
	fmt.Fprintln(stdout, ctx)
	return 0
}

func defaultFathomURL() string {
	if u := os.Getenv("FATHOM_HTTP"); u != "" {
		return u
	}
	return "http://localhost:7575"
}

type svcInfo struct {
	symbols int
	langs   string
	path    string // recorded corpus root; "" when the daemon has none
}

// buildSessionContext returns the additionalContext line for the project at
// projDir. Service identity tries the dir basename first, then the enclosing
// git repo's basename (projects rooted in a subdirectory like <repo>/deploy).
func buildSessionContext(baseURL, projDir string) string {
	hc := &http.Client{Timeout: 2 * time.Second}

	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		// This line reaches EVERY session of EVERY adopter, which is why it
		// may not name a label that exists on one machine.
		return fmt.Sprintf("⚠️ fathom code-intelligence daemon (%s) is not responding — its MCP tools are unavailable this session, so Grep/Read discovery is expected. Restart it with: %s", baseURL, daemonRestartHint(""))
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
		return fmt.Sprintf("🔬 fathom OFF arm active (measurement, vision 14 §2): channels are silent this session — no obligations, no guards, no verify gate. Bound: %s. This is deliberate counterfactual data collection; end it early with `fathom arm on`.", bound)
	}

	svcs, svcErr := fetchServices(hc, baseURL)
	if svcErr != nil {
		// O2: enumeration failure is NOT "not indexed". The daemon is healthy
		// (healthz just passed) but list_services failed — saying "this repo
		// is not an indexed service" here would be a confident false
		// instruction the agent has no reason to disprove. Degrade honestly.
		return fmt.Sprintf("⚠️ fathom daemon is healthy but service enumeration failed (%v) — index state is unknown this session; the MCP tools may still work, so try them before falling back to Grep/Read. If this repeats, check GET %s/statusz.", svcErr, baseURL)
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
		langs := ""
		if info.langs != "" {
			langs = ", " + info.langs
		}
		ctxLine := fmt.Sprintf("✅ fathom code-intelligence is live — this repo is the indexed service %q (%d symbols%s). Route by question shape: where / who-calls / what-breaks resolve via fathom: find_symbol, query_symbols, find_callers, outline, repo_map, references; what-governs this repository is obligations with no path (the root conventions); is this still true / what was known is staleness_report; a bug report, stack trace, error string or failing test is localize's job (one bounded, logged walk of the index — before any grep). A literal string, a config value, a log line, or a document-versus-document contradiction is grep's job. Read files to *edit* them, not to discover them. The index auto-refreshes on git-HEAD drift; force a refresh with the ingest tool ({\"service\":%q}).", lookup, info.symbols, langs, lookup)
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
	return fmt.Sprintf("ℹ️ fathom code-intelligence is live, but this repo (%q) is not an indexed service — its symbols aren't in the index, so Grep/Read discovery is expected here. Indexed services: %s. (Add this repo with the ingest tool: {\"service\":\"<name>\",\"path\":\"<absolute repo root>\"}.)", repo, indexed)
}

// fetchServices calls the list_services MCP tool over HTTP and returns a map
// keyed by lower-cased service ID, plus an error when ENUMERATION failed —
// callers must distinguish "failed to enumerate" from "zero services" (O2:
// collapsing them once rendered a confident false not-indexed instruction
// for an indexed repo). One retry with its own timeout, decoupled from the
// caller's healthz-sized client.
func fetchServices(hc *http.Client, baseURL string) (map[string]svcInfo, error) {
	m, err := fetchServicesOnce(hc, baseURL)
	if err == nil {
		return m, nil
	}
	retryClient := &http.Client{Timeout: 4 * time.Second}
	if hc != nil && hc.Transport != nil {
		retryClient.Transport = hc.Transport
	}
	return fetchServicesOnce(retryClient, baseURL)
}

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
		out[strings.ToLower(s.Service)] = svcInfo{symbols: s.SymbolCount, langs: joinLangs(s.Languages), path: s.Path}
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
	Service     string   `json:"service"`
	SymbolCount int      `json:"symbol_count"`
	Languages   []string `json:"languages"`
	Path        string   `json:"path"` // recorded corpus root (authoritative repo↔service identity)
}

func joinLangs(l []string) string {
	if len(l) == 0 {
		return ""
	}
	return strings.Join(l, "/")
}

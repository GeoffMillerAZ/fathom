package main

// prompthook.go — fathom prompt-hook: a Claude Code UserPromptSubmit hook
// (O57 items 4/5, the guards pilot's second reading). edit-hook can only
// answer "what governs this file" once something has already been
// written to it; a refusal-shaped task (ADR-025: no org-specific names)
// needs the governing rule BEFORE the first edit. Claude Code hands
// fathom the user's own prompt text on UserPromptSubmit — this verb pulls
// the paths, packages and bare symbol names the prompt itself mentions,
// resolves each through the index, and serves the union of rules that
// govern them: capped, deduped, and silent when nothing resolves.
//
// Same silence contract as every other hook (ADR-016 §5.3): any error,
// timeout, or an unindexed/empty prompt prints nothing and exits 0.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/stackframe"
)

const (
	// maxPromptCandidates bounds how many distinct path/package/symbol
	// tokens one prompt resolves — a long prompt pasting a stack trace or
	// a file listing must not turn one UserPromptSubmit into dozens of
	// RPCs (the same discipline maxSpecIDsPerEdit applies to edit-hook's
	// own spec-line injection).
	maxPromptCandidates = 5
	// maxPromptSymbolLookups additionally bounds the more expensive
	// find_symbol calls (one RPC per bare identifier token) separately
	// from path/package candidates, which cost one obligations call each
	// and no resolution round trip of their own.
	maxPromptSymbolLookups = 3
)

// promptHookDeadlineBudget is the ONE wall-clock budget every RPC this
// hook makes shares — symbol resolution, candidate-identity resolution,
// and one obligations call per resolved file all draw against it, rather
// than each getting its own fresh timeout. Up to nine sequential 3s
// calls against a 10s hook timeout was the actual defect (O57 review): a
// prompt naming several files could spend several multiples of a single
// call's own budget. 8s leaves headroom inside the 10s hook timeout for
// encode/decode and process startup. A var, not a const, so a test can
// shorten it against a deliberately slow fake daemon.
var promptHookDeadlineBudget = 8 * time.Second

// cmdPromptHook implements the "prompt-hook" subcommand.
func cmdPromptHook(args []string, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // raw kill switch: total silence, no beat, no ledger row
	}
	armOff := assistiveChannelsDisabled()
	rpcOrigin = "prompt-hook"
	fs := newFlagSet("prompt-hook", stderr)
	urlFlag := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 0 // hook contract: silent
	}
	url := *urlFlag
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0
	}
	endGuard, silent := beginHookGuard("prompt-hook", "fathom prompt-hook", codec.ID(), url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0
	}
	hook, err := codec.DecodeInput(harness.EventUserPromptSubmit, raw)
	if err != nil {
		return 0
	}
	sessionID := hook.SessionID
	rpcSession = sessionID // O1151: the daemon sees this hook session
	prompt := strings.TrimSpace(hook.Prompt)
	if prompt == "" {
		return 0
	}
	// SU-7 (docs/plans/substrate.md §A, ADR-041 decision 3): a harness- or
	// tool-injected system notification is not a user asking a question —
	// its body is prose ABOUT something (an article title, a URL, a task
	// summary) that extractPromptCandidates' lexical heuristics can and do
	// misread as repo paths/packages (O91-class: "publication/observation"
	// and "cursor.com/blog/secure-codebase-indexing" both reached
	// obligations as fabricated `__prompt__.go` candidates from exactly
	// this kind of text). No candidate extraction, no daemon call, no
	// output — the prompt hook's own silence contract, applied before
	// anything downstream ever sees this text.
	if isSystemNotification(prompt) {
		return 0
	}
	// EW-21 (O126): a message another agent relayed into this session
	// (a sub-agent, a teammate, a peer session, the workflow harness) is
	// not a user asking a question either. It names scratch paths and build
	// artifacts, and the hook answered one with six root rules and "178
	// more eligible". Same silence, same place, and no daemon call.
	if isAgentRelay(prompt) {
		return 0
	}
	// SR-48w (F-31): an exact one-key reply (`c` or `r`, optionally with a
	// note) to the edit hook's pending verdict ask is recorded on the ask
	// for the next edit-hook turn to write — only when the session has
	// one pending; otherwise the prompt is an ordinary prompt.
	if ask, ok := recordVerdictReply(sessionID, prompt, verdictReplyNow()); ok {
		if out, err := codec.EncodeOutput(harness.EventUserPromptSubmit, harness.HookOutput{Context: verdictReplyLine(ask)}); err == nil {
			fmt.Fprintln(stdout, string(out))
		}
		return 0
	}

	emitBeat("prompt-hook", "seen", firingMetaArm(map[string]string{}, sessionID, "", armOff))
	// GL-26 (O822): every prompt seen here is answered exactly once, and a
	// silent answer names its reason from the edit hook's closed vocabulary
	// (silenceReason, edithook_dest.go).
	answerSilent := func(meta map[string]string, reason silenceReason) int {
		emitBeat("prompt-hook", "answered", firingMetaArm(silentAnswerMeta(meta, reason), sessionID, "", armOff))
		return 0
	}
	if armOff {
		// Counting-only baseline stops here, matching edit-hook's own
		// OFF-arm gate (IT-3): no daemon call, no stdout.
		return answerSilent(nil, silenceArmOff)
	}

	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)
	deadline := time.Now().Add(promptHookDeadlineBudget)
	text, meta, ok := renderPromptObligations(url, projDir, prompt, sessionID, deadline)

	// H10 (ADR-076): behind [hooks] prompt_definitions, one line naming the
	// definitions of the identifiers resolved above. It adds no call, shares
	// the obligations cap, and yields to it when it does not fit.
	text, ok = withPromptDefinitions(projDir, sessionID, text, ok, meta.defs, meta.defsMissed)

	// DP-10 (ADR-042, docs/vision/32-decision-port.md §2 item 3): routing
	// draws on the SAME shared deadline as the obligations work above, and
	// asks AFTER it — obligations always gets first claim on the budget; a
	// route line is a bonus this hook can afford to skip, never a reason
	// to starve it. A route line can stand alone (a prompt naming nothing
	// obligations resolves, e.g. "why does X crash", is still routable).
	route := routeLineFor(&http.Client{Timeout: deadlineRemaining(deadline)}, url, projDir, prompt, deadline)
	text, ok = combineWithRouteLine(text, ok, route)
	text, ok = withPromptMapPointer(projDir, prompt, sessionID, text, ok) // XM-11: one line naming an exploration map this prompt's paths or symbols lie in, once per session, daemon-free (maphooks.go)
	if !ok {
		// Nothing resolved and no route line either — silence contract;
		// renderPromptObligations named why.
		return answerSilent(map[string]string{"candidates": fmt.Sprint(meta.candidates)}, meta.silence)
	}
	out, err := codec.EncodeOutput(harness.EventUserPromptSubmit, harness.HookOutput{Context: text})
	if err != nil {
		return answerSilent(nil, silenceEncodeFailed)
	}
	answeredMeta := map[string]string{"emitted": "true", "candidates": fmt.Sprint(meta.candidates), "files": fmt.Sprint(meta.files)}
	if meta.displaced > 0 {
		answeredMeta["displaced"] = fmt.Sprint(meta.displaced)
	}
	if meta.resolved < meta.files {
		answeredMeta["resolved"] = fmt.Sprint(meta.resolved)
	}
	emitBeat("prompt-hook", "answered", firingMetaArm(answeredMeta, sessionID, "", armOff))
	fmt.Fprintln(stdout, string(out))
	return 0
}

// isSystemNotification reports whether prompt is a harness/tool-injected
// system notification rather than user-authored text (SU-7): it begins
// with "[SYSTEM NOTIFICATION" (any casing of the bracket tag Claude Code
// and other harnesses use for injected system messages, e.g. task/queue
// completion notices) or contains a "<task-notification>" tag anywhere.
// prompt is matched as given — the caller already trimmed leading/
// trailing whitespace before this check runs.
func isSystemNotification(prompt string) bool {
	if strings.HasPrefix(prompt, "[SYSTEM NOTIFICATION") {
		return true
	}
	return strings.Contains(prompt, "<task-notification>")
}

// agentRelayRE matches a line that opens with the wrapper Claude Code puts
// around a message that another agent sent: `<cross-session-message
// from="…">` (a peer session), `<teammate-message …>` (an agent team), or the
// workflow harness's `[Workflow harness …]` framing of a computed task. The
// tags are the ones the harness documents to the model as "not from your
// user"; matching a line start (not the whole prompt's start) catches a
// relay queued behind typed text, while a user's own sentence that merely
// names a tag mid-line is not one.
var agentRelayRE = regexp.MustCompile(`(?m)^\s*(<cross-session-message\b|<teammate-message\b|\[Workflow harness\b)`)

// isAgentRelay reports whether prompt carries a cross-agent relay message
// (EW-21, O126) rather than text the user typed.
func isAgentRelay(prompt string) bool {
	return agentRelayRE.MatchString(prompt)
}

// promptObligationsMeta is what renderPromptObligations learned, for the
// answered beat.
type promptObligationsMeta struct {
	candidates int // tokens extracted from the prompt
	files      int // distinct files obligations were attempted for
	displaced  int
	resolved   int // of files, how many got a definite answer before the deadline
	// defs and defsMissed carry H10's definitions out of the resolve step
	// (ADR-076); empty unless a symbol candidate resolved.
	defs       []promptDefinition
	defsMissed bool
	// silence (GL-26) is why renderPromptObligations said nothing, set on
	// every ok=false return.
	silence silenceReason
}

// renderPromptObligations builds the UserPromptSubmit payload for prompt as
// harness-free text: extract candidate names, resolve each to a
// (service, file), fetch and merge obligations across every resolved
// file, cap and render. ok=false is the silence contract — nothing named,
// nothing resolved, nothing governs it, or the capability is off; the meta's
// silence names which (GL-26).
func renderPromptObligations(url, projDir, prompt, sessionID string, deadline time.Time) (string, promptObligationsMeta, bool) {
	if capabilityOffForFile(projDir, "", "obligations") {
		return "", promptObligationsMeta{silence: silenceCapabilityOff}, false
	}
	candidates := extractPromptCandidates(prompt, workingTreeNames(localCandidateRoots(projDir)))
	if len(candidates) == 0 {
		return "", promptObligationsMeta{silence: silenceNoCandidate}, false
	}
	hc := &http.Client{Timeout: deadlineRemaining(deadline)}
	resolved, defs, defsMissed := resolvePromptCandidates(hc, url, projDir, candidates, deadline)
	if len(resolved) == 0 {
		return "", promptObligationsMeta{candidates: len(candidates), defsMissed: defsMissed, silence: silenceNoCandidate}, false
	}
	svc := resolved[0].service // resolvePromptCandidates resolves ONE service per call
	files := make([]string, len(resolved))
	for i, r := range resolved {
		files[i] = r.file
	}
	m := mergeObligationsAcrossFiles(url, svc, files, deadline, func(f string) {
		emitBeat("prompt-hook", "error", firingMeta(map[string]string{"target": f}, sessionID, ""))
	})
	if len(m.rules) == 0 {
		// The caller writes the one answered beat (a route line may still
		// speak); every failed fetch already has its error beat.
		meta := promptObligationsMeta{candidates: len(candidates), files: m.total, resolved: m.resolved, defs: defs, defsMissed: defsMissed, silence: silenceNoRules}
		if m.resolved == 0 && m.total > 0 {
			meta.silence = silenceDaemonError
		}
		return "", meta, false
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚖️ fathom obligations — this prompt names %s:", promptNamesLabel(m.namedFiles))
	writeRuleBullets(&sb, m.rules, svc)
	if m.displaced > 0 {
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", m.displaced)
	}
	if m.stale {
		sb.WriteString("\n⚠️ index is behind your working tree for one of these paths")
	}
	if m.coverageSkipped != "" {
		sb.WriteString("\n⚠️ " + m.coverageSkipped) // RT-146 / I38
	}
	if m.resolved < m.total {
		fmt.Fprintf(&sb, "\n⚠️ partial: %d of %d resolved (time budget)", m.resolved, m.total)
	}
	// FL-21 (docs/vision/30-findings-loop.md §3.4): one line naming the
	// open finding count across the SAME resolved files, inside the SAME
	// shared deadline — skipped silently (the plan's own skip-note
	// mechanism: simply not rendered) when the deadline is already spent.
	if n, ok := countFindingsAcrossFiles(url, svc, files, deadline); ok && n > 0 {
		fmt.Fprintf(&sb, "\nopen findings: %d across the files this prompt names", n)
	}
	return sb.String(), promptObligationsMeta{candidates: len(candidates), files: m.total, displaced: m.displaced, resolved: m.resolved, defs: defs, defsMissed: defsMissed}, true
}

// deadlineRemaining is deadline's remaining wall-clock time, floored at
// one nanosecond — an http.Client Timeout of exactly ZERO means
// UNLIMITED, never "no time left", so a caller with nothing left in its
// budget still gets a real (if minuscule) Timeout that fails the call
// almost immediately, rather than one with none at all.
func deadlineRemaining(deadline time.Time) time.Duration {
	if d := time.Until(deadline); d > 0 {
		return d
	}
	return time.Nanosecond
}

// promptNamesLabel renders the resolved files a prompt's rules actually
// came from, capped for readability — the payload names WHAT governed,
// this names WHERE it came from.
func promptNamesLabel(files []string) string {
	if len(files) == 0 {
		return "what it names"
	}
	const maxNamed = 3
	if len(files) > maxNamed {
		return strings.Join(files[:maxNamed], ", ") + fmt.Sprintf(" (+%d more)", len(files)-maxNamed)
	}
	return strings.Join(files, ", ")
}

// promptResolved is one candidate resolved to a concrete (service, file)
// obligations can be queried for.
type promptResolved struct {
	service, file, kind string
}

// resolvePromptCandidates resolves candidates against projDir's service —
// the daemon-resolved identity when reachable, else the basename guess
// (candidatesWithDaemon, the same fallback ladder edit-hook uses). Path
// and package tokens resolve locally: extractPromptCandidates already kept
// only those that name a file or directory in the working tree (EW-21), as
// repo-relative text; symbol tokens cost one find_symbol
// RPC each, capped at maxPromptSymbolLookups AND stopped early once
// deadline arrives — every RPC here draws against the SAME shared budget
// the caller's later per-file obligations calls also draw against.
// It also returns the definitions those find_symbol answers carried (H10,
// ADR-076), in candidate order, and whether a symbol lookup was skipped or
// failed because the shared deadline had passed; the definitions add no call.
func resolvePromptCandidates(hc *http.Client, url, projDir string, candidates []promptCandidate, deadline time.Time) ([]promptResolved, []promptDefinition, bool) {
	cands := candidatesWithDaemon(hc, url, projDir)
	if len(cands) == 0 {
		return nil, nil, false
	}
	var defs []promptDefinition
	missed := false
	svc := strings.ToLower(cands[0].name)
	seen := map[string]bool{}
	var out []promptResolved
	add := func(file string) {
		file = strings.TrimPrefix(strings.TrimSpace(file), "./")
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		out = append(out, promptResolved{service: svc, file: file})
	}
	symbolLookups := 0
	for _, c := range candidates {
		switch c.kind {
		case candidatePath:
			add(c.text)
		case candidatePackage:
			// BP-32/scopeFloor-style package globs anchor at the
			// directory ("<dir>/**"); a synthetic filename inside it
			// lets a bare package mention match the SAME scope a real
			// file there would, with no new matching logic in
			// semantic.ObligationsFor.
			add(strings.TrimSuffix(c.text, "/") + "/__prompt__.go")
		case candidateSymbol:
			if symbolLookups >= maxPromptSymbolLookups {
				continue // cap reached
			}
			if !time.Now().Before(deadline) {
				missed = true // nothing left in the shared budget to spend on a lookup
				continue
			}
			symbolLookups++
			symHC := &http.Client{Timeout: deadlineRemaining(deadline)}
			if d, ok := fetchSymbolDef(symHC, url, svc, c.text, c.marked); ok {
				add(d.File)
				defs = append(defs, d)
			} else if !time.Now().Before(deadline) {
				missed = true
			}
		}
	}
	return out, defs, missed
}

// promptDefinition is one resolved identifier's definition as find_symbol
// answered it (H10): Count is how many definitions the exact tier returned.
type promptDefinition struct {
	Name, Kind, File string
	Line, Count      int
}

// maxPromptDefinitionNames bounds the definitions line (ADR-076).
const maxPromptDefinitionNames = 5

// promptDefinitionsLine renders the H10 line: for each resolved identifier
// its definition file:line and kind, at most maxPromptDefinitionNames; a
// name with several definitions reads "N definitions" and none is chosen.
// Empty when nothing resolved. missed names a deadline miss instead of
// reading as nothing found.
func promptDefinitionsLine(defs []promptDefinition, missed bool) string {
	if len(defs) == 0 {
		if missed {
			return "definitions not computed (deadline)"
		}
		return ""
	}
	var parts []string
	for _, d := range defs {
		if len(parts) == maxPromptDefinitionNames {
			break
		}
		switch {
		case d.Count > 1:
			parts = append(parts, fmt.Sprintf("%s (%d definitions)", d.Name, d.Count))
		case d.Line > 0:
			parts = append(parts, fmt.Sprintf("%s %s %s:%d", d.Name, d.Kind, d.File, d.Line))
		default:
			parts = append(parts, fmt.Sprintf("%s %s %s", d.Name, d.Kind, d.File))
		}
	}
	line := "defined: " + strings.Join(parts, ", ")
	if missed {
		line += " (more not computed (deadline))"
	}
	return line
}

// fetchSymbolDef resolves a bare symbol name to its definition via the
// find_symbol MCP tool — ok=false on any error, no match, or an ambiguous
// name the tool declines to pick for (did_you_mean without an exact hit).
// marked is true when the prompt wrote name as code (backticked or called).
// It carries the first match's file, line and kind and the number of
// definitions the exact tier returned (H10 names a count over one, never
// the first; obligations still reads the first file, O1378).
func fetchSymbolDef(hc *http.Client, baseURL, service, name string, marked bool) (promptDefinition, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "find_symbol",
			"arguments": map[string]any{"service": service, "name": name},
		},
	})
	if err != nil {
		return promptDefinition{}, false
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return promptDefinition{}, false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return promptDefinition{}, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return promptDefinition{}, false
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return promptDefinition{}, false
	}
	var payload struct {
		Symbols []struct {
			FilePath string `json:"file_path"`
			Kind     string `json:"kind"`
			Line     int    `json:"line"`
		} `json:"symbols"`
		MatchedVia string `json:"matched_via"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil || len(payload.Symbols) == 0 {
		return promptDefinition{}, false
	}
	// O1114: only the exact tier names what the prompt said. A
	// case-insensitive hit counts when the prompt wrote the token as code
	// (backticked or called); a substring or stem hit is a suggestion, never
	// a resolution.
	switch payload.MatchedVia {
	case "exact":
	case "case_insensitive":
		if !marked {
			return promptDefinition{}, false
		}
	default:
		return promptDefinition{}, false
	}
	if payload.Symbols[0].FilePath == "" {
		return promptDefinition{}, false
	}
	s0 := payload.Symbols[0]
	return promptDefinition{Name: name, Kind: s0.Kind, File: s0.FilePath, Line: s0.Line, Count: len(payload.Symbols)}, true
}

// combineWithRouteLine folds route (DP-10's rendered line, possibly "")
// into text/ok (renderPromptObligations' own result): route == "" changes
// nothing. Otherwise, text=="" (ok==false: nothing resolved) makes route
// stand alone as the whole output. Otherwise route is appended on its own
// line ONLY when the combined length still fits the hook's existing byte
// budget (harness.CodexAdditionalContextCap, codec_codex.go) — appending
// past it would risk that cap's own blind end-of-string truncation eating
// into obligations text a caller already earned, so the route line is
// dropped instead, never the obligations text. Pure function: no I/O, no
// clock, so a test can exercise every branch without a daemon.
func combineWithRouteLine(text string, ok bool, route string) (string, bool) {
	if route == "" {
		return text, ok
	}
	if !ok {
		return route, true
	}
	if len(text)+1+len(route) <= harness.CodexAdditionalContextCap {
		return text + "\n" + route, true
	}
	return text, true
}

// routePostResponse mirrors POST /route's JSON body (cmd/fathom/route.go's
// routeResponse) — a private mirror here rather than importing route.go's
// type directly, matching this file's existing pattern for every other
// plain (non-JSON-RPC) daemon endpoint a hook calls (refreshFileRequest/
// refreshFileResponse in edithook_pre.go/refreshfile.go).
type routePostResponse struct {
	Shape      string  `json:"shape"`
	Verb       string  `json:"verb"`
	Confidence float64 `json:"confidence"`
	Band       string  `json:"band"`
	Provider   string  `json:"provider"`
	Error      string  `json:"error,omitempty"`
}

// routeLine asks the daemon's POST /route (DP-9) which MCP tool's
// question-shape fits prompt and renders DP-10's one-line hint — verbs in
// the line are the MCP tool names verbatim (find_symbol, references,
// find_callers, obligations, staleness_report, search, localize), the
// exact spelling a reader can paste into a tool call.
//
// Amended 2026-09-24 (ADR-042, CV-M11): the route line is ALWAYS hedged,
// at any confidence — `docs/bench/readings/decision-port-calibration-
// 2026-09-24.md` measured route's proceed-band choice matching its own
// adjudicated gold tool only 45.7% of the time, and sweeping every
// confidence cutoff never reaches the 0.9-right-nine-times-in-ten bar
// this ADR requires before a firm line is warranted (the highest-
// confidence slice was measurably WORSE than the band below it — routing
// confidence and correctness are not positively correlated on this
// sample). A firm imperative ("open with X") overstates a suggestion this
// port has shown it cannot back at any confidence; hedging costs nothing
// and never overstates:
//
//   - band "proceed" or "confirm": one hedged line — same wording either
//     way, since band no longer changes the phrasing, only whether it
//     counts as "applied" for the decisions log (routeHandler's own
//     concern, unchanged).
//   - anything else — band "unsure", verb "" or "none", provider "none",
//     a deadline already spent, any transport/HTTP/decode error, or a
//     non-200 status — returns "": nothing is ever injected on a routing
//     failure (ADR-016 §5.3's fail-open contract; the hook must never
//     block, error, or print anything just because routing did not work),
//     and never below the confirm floor either.
func routeLine(hc *http.Client, baseURL, prompt string, deadline time.Time) string {
	if !time.Now().Before(deadline) {
		return "" // routing never starves obligations: nothing left to spend
	}
	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return ""
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/route", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return ""
	}
	var out routePostResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ""
	}
	if out.Error != "" || out.Provider == "none" || out.Verb == "" || out.Verb == "none" {
		return ""
	}
	switch out.Band {
	case "proceed", "confirm":
		return fmt.Sprintf("fathom route: this looks %s-shaped; %s is the likely opening move (confidence %.2f, unconfirmed).", out.Shape, out.Verb, out.Confidence)
	default:
		return "" // "unsure", or an unrecognized band string — say nothing
	}
}

// issueErrorLineRE matches an unambiguous error/panic/traceback header: a
// Go panic, a Python traceback header, a goroutine dump header, or a
// "SomeError:"/"SomeException:" message line (Java/JS/Python-shaped).
// Deliberately narrow (RC-24/SB-2 doctrine: a false positive here fires a
// deterministic, unhedged claim — precision matters more than recall) so
// ordinary prose that merely discusses "errors" never matches.
var issueErrorLineRE = regexp.MustCompile(`(?m)^\s*(panic:\s|Traceback \(most recent call last\):|goroutine \d+ \[|[A-Za-z_][A-Za-z0-9_.]*(Error|Exception):\s)`)

// issueFailingTestRE matches a go-test-shaped failure line naming a Test
// function ("--- FAIL: TestFoo" or "FAIL: TestFoo/case").
var issueFailingTestRE = regexp.MustCompile(`(?m)^\s*(---\s+)?\bFAIL(ED)?\b[:\s]+Test[A-Za-z0-9_]*(/[^\s()]+)?`)

// issueBareTestNameRE matches a prompt that IS, in its entirety (trimmed),
// a test name — the shape a caller gets from copy-pasting `go test`'s own
// "--- FAIL: TestFoo" line's name back out, or an IDE's "run failing
// test" action.
var issueBareTestNameRE = regexp.MustCompile(`^Test[A-Za-z0-9_]*(/[^\s()]+)?$`)

// routeIssueShapedPrompt is SR-44's deterministic half of DP-9 routing
// (F-25, design §5): a stack trace, a failing test name, or an error
// string is lexically unambiguous — unlike the probabilistic POST /route
// call (ADR-042, CV-M11: proceed-band matched its own adjudicated gold
// tool only 45.7% of the time on lca), these three shapes need no
// judgment call. ok is false for anything else, leaving routeLineFor to
// fall back to the HTTP decision port as before.
func routeIssueShapedPrompt(prompt string) (verb, shape string, ok bool) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", "", false
	}
	switch {
	case issueBareTestNameRE.MatchString(trimmed),
		issueFailingTestRE.MatchString(trimmed),
		issueErrorLineRE.MatchString(trimmed),
		// Two lines that are wholly a stack frame in any grammar localize
		// parses (Go/Rust file:line, JVM, V8, PHP, Python, Ruby):
		// internal/stackframe's own counter (TL-1), so localize and this
		// hook read frames through one parser and the hook's detector no
		// longer mirrors localize's by hand.
		stackframe.CountFrameLines(trimmed, 2) >= 2,
		// TL-1: a pasted error payload (Bugsnag, OTLP, Lambda, New
		// Relic) carrying a frame tuple is issue-shaped too; the payload
		// is only tested here, never echoed.
		stackframe.IsPayload(trimmed):
		return "localize", "issue", true
	default:
		return "", "", false
	}
}

// routeLineFor resolves the prompt hook's one-line routing hint. A
// deterministic issue-shape match (routeIssueShapedPrompt) short-circuits
// entirely — no HTTP call, no hedge, since the shape itself is the
// evidence — and names a tool the session's registered surface serves
// (O615: localize is on /rpc/extended only, ADR-053). Everything else still
// asks POST /route exactly as routeLine always has.
func routeLineFor(hc *http.Client, baseURL, projDir, prompt string, deadline time.Time) string {
	if verb, shape, ok := routeIssueShapedPrompt(prompt); ok {
		lead := fmt.Sprintf("fathom route: this prompt is %s-shaped (a stack trace, failing test, or error string); ", shape)
		const defaultMove = "search its most distinctive frame, test name or error text, then symbol_context on the top in-repo symbol."
		switch registeredPromptSurface(projDir, deadline) {
		case promptSurfaceLocalize:
			return lead + verb + " is the opening move."
		case promptSurfaceDefault:
			return lead + defaultMove
		default:
			return lead + defaultMove + " On /rpc/extended, " + verb + " does this in one call."
		}
	}
	return routeLine(hc, baseURL, prompt, deadline)
}

// promptSurface is what the session's fathom MCP registration serves, as
// far as a local read can tell.
type promptSurface int

const (
	promptSurfaceUnknown  promptSurface = iota // no registration found, or a read failed
	promptSurfaceDefault                       // /rpc/agent: the default eight, no localize
	promptSurfaceLocalize                      // /rpc/extended, /rpc/companion or /rpc: localize is served
)

// promptUserConfigCap bounds the read of Claude Code's user config file
// (and the project's .mcp.json): a larger file is not read, and the
// surface is unknown.
const promptUserConfigCap = 1 << 20

// registeredPromptSurface reads the session's "fathom" MCP registration:
// the project's .mcp.json first, then Claude Code's user config file
// (harness.ClaudeUserConfigFile: the local scope for projDir, then the user
// scope), each only under promptUserConfigCap. A spent deadline, a missing
// entry, or any read or parse error is unknown, never a failure.
func registeredPromptSurface(projDir string, deadline time.Time) promptSurface {
	if !time.Now().Before(deadline) {
		return promptSurfaceUnknown
	}
	type mcpEntry struct {
		URL  string   `json:"url"`
		Args []string `json:"args"`
	}
	type mcpScope struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}
	type mcpDoc struct {
		mcpScope
		Projects map[string]mcpScope `json:"projects"`
	}
	read := func(path string) (doc mcpDoc, ok bool) {
		if path == "" {
			return doc, false
		}
		f, err := os.Open(path)
		if err != nil {
			return doc, false
		}
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, promptUserConfigCap+1))
		if err != nil || len(raw) > promptUserConfigCap || json.Unmarshal(raw, &doc) != nil {
			return doc, false
		}
		return doc, true
	}
	classify := func(e mcpEntry) promptSurface {
		for _, u := range append([]string{e.URL}, e.Args...) {
			switch u = strings.TrimRight(u, "/"); {
			case strings.HasSuffix(u, "/rpc/agent"):
				return promptSurfaceDefault
			case strings.HasSuffix(u, "/rpc/extended"), strings.HasSuffix(u, "/rpc/companion"), strings.HasSuffix(u, "/rpc"):
				return promptSurfaceLocalize
			}
		}
		return promptSurfaceUnknown
	}
	if projDir != "" {
		if doc, ok := read(filepath.Join(projDir, ".mcp.json")); ok {
			if e, found := doc.MCPServers["fathom"]; found {
				return classify(e)
			}
		}
	}
	doc, ok := read(harness.ClaudeUserConfigFile(wiringDriftHome()))
	if !ok {
		return promptSurfaceUnknown
	}
	if e, found := doc.Projects[projDir].MCPServers["fathom"]; found && projDir != "" {
		return classify(e)
	}
	if e, found := doc.MCPServers["fathom"]; found {
		return classify(e)
	}
	return promptSurfaceUnknown
}

// promptHookRunner runs `fathom prompt-hook` (registered below; EW-45).
type promptHookRunner struct{}

func (promptHookRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdPromptHook(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "prompt-hook", Group: "hooks", Order: 50, Run: promptHookRunner{},
		Summary: "UserPromptSubmit hook: resolve the paths/packages/symbols the prompt names through the index and inject the rules that govern them, before the first edit"})
}

// promptDefinitionsChannel logs the definitions line's counts and outcome,
// never prompt text. Kinds match tests-to-run: line, missed or silent.
const promptDefinitionsChannel = "prompt-definitions"

// withPromptDefinitions uses the definitions already resolved by the prompt
// hook. Observe records the same computed row without adding context.
func withPromptDefinitions(projDir, sessionID, text string, ok bool, defs []promptDefinition, missed bool) (string, bool) {
	cfg, loaded := hooksConfigAt(projDir, hookOptInPromptDefinitions)
	if !loaded || !cfg.Hooks.PromptDefinitions {
		return text, ok
	}
	line := promptDefinitionsLine(defs, missed)
	meta := map[string]string{"definitions": fmt.Sprint(len(defs)), "emitted": "false"}
	if missed {
		meta["missed"] = "deadline"
	}
	if line != "" {
		joined, joinedOK := combineWithRouteLine(text, ok, line)
		if joined != text {
			if !cfg.Hooks.Observes(hookOptInPromptDefinitions) {
				text, ok, meta["emitted"] = joined, joinedOK, "true"
			}
		} else {
			meta["dropped"] = "budget"
		}
	}
	kind := "silent"
	if line != "" {
		kind = "line"
	}
	if missed {
		kind = "missed"
	}
	emitBeat(promptDefinitionsChannel, kind, firingMeta(meta, sessionID, ""))
	return text, ok
}

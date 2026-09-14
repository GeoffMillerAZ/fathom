// RunSessions (SB-13) executes one task's sessions in order, under the
// budget task.Sessions declares: mode "single" runs Count cold sessions of
// the same prompt, mode "planner-workers" runs one planning session and
// then one cold worker session per row the planner's plan file names. The
// prompt is identical across arms; the only thing that may differ by arm is
// the text RenderArm declared (wt.Arm.SystemAppend).
package scenariobench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RunOptions is what RunSessions needs beyond the task/arm/worktree: the
// trial number (for raw-artifact naming), the model to run, the base
// directory raw transcripts land under, and extra environment variables
// for the harness.
type RunOptions struct {
	Trial  int
	Model  string
	RawDir string
	Env    map[string]string
	// RunID keys the raw transcript directory (RawDir/<RunID>/…) so a rerun
	// never reads or overwrites another run's sessions (reuse-pilot-2 read
	// stale worker transcripts from the round before, 2026-09-13).
	RunID string
}

// scenarioContinuationPrompt is the fixed, documented prompt every session
// after the first gets in "single" mode: sessions are cold (no --resume),
// so the ONLY thing carrying state forward is what persists in the
// worktree — the repo, and (for an arm with NotesFile) NOTES.md.
const scenarioContinuationPrompt = "Continue the task from where the previous session left off. The repository is in whatever state that session left it; check your notes file if you kept one, and the current state of the tree, before doing anything else."

// PlanFirstInstruction is appended to a task's prompt whenever the driver
// wants a session to write its plan before touching anything: the
// planning session in "planner-workers" mode (so RunSessions can read the
// checklist back, readPlanRows), and — since single-session tasks used to
// give the M2/M3 readings (PlanCoverage, FalseDoneRate) nothing to read —
// the FIRST session of "single" mode too (runSingle). Identical text in
// every mode and arm: only a wired arm's own stanza may steer the agent
// toward docs/plans/<slug>.md instead of NOTES.md, never this sentence.
const PlanFirstInstruction = "\n\nBefore making any changes, write your plan as a markdown checklist (`- [ ] ...` per row) to NOTES.md at the repo root, or to a file under docs/plans/ if the repository already keeps plans there. Do not implement anything yet — later sessions will pick up individual rows."

// planGrammarInstruction is the wired arm's addendum to PlanFirstInstruction
// (SB-53): it names the repository's plan grammar and the verify verb so
// the plan carries rows the verify gate can adjudicate (the M2 reading).
// The control arm keeps the plain checklist — the grammar is part of what
// "wired" means, not a prompt advantage handed to one arm by the driver.
const planGrammarInstruction = " Because this repository is indexed by fathom, write the plan to docs/plans/<slug>.md in its plan grammar: one row per change, `- [ ] <what changes> {id: <SLUG>-<n>, symbol: <the function you will add or change>, package: <its package path>}`, anchoring every row to a function. A row is done only when `fathom verify --plan docs/plans/<slug>.md` proves it — never mark `[x]` on a claim."

// planFirstInstructionFor returns the plan-first instruction for one arm:
// the shared checklist text, plus the plan-grammar addendum on a wired arm.
func planFirstInstructionFor(arm Arm) string {
	if arm.Wired {
		return PlanFirstInstruction + planGrammarInstruction
	}
	return PlanFirstInstruction
}

// workerRowPromptFmt formats one worker's assignment in "planner-workers"
// mode: the row text the planner wrote, verbatim, plus the original task
// prompt for context.
const workerRowPromptFmt = "You are one worker in a team completing this task:\n\n%s\n\nYour assignment is this row from the plan:\n\n%s\n\nComplete it, and mark it done (`- [x]`) in the plan file when finished."

// RunSessions executes task's sessions for one (armName, arm) treatment in
// wt, via harness h. On a session that fails to even start, it returns the
// sessions collected so far and the error; a session that ran but reported
// trouble is still returned as data (SessionResult.Error) and the run
// continues, since a bad transcript on its own is not a reason to abandon
// a multi-session budget.
func RunSessions(ctx context.Context, h Harness, task Task, armName string, arm Arm, wt Worktree, opts RunOptions) ([]SessionResult, error) {
	switch task.Sessions.Mode {
	case ModeSingle:
		return runSingle(ctx, h, task, armName, arm, wt, opts)
	case ModePlannerWorkers:
		return runPlannerWorkers(ctx, h, task, armName, arm, wt, opts)
	default:
		return nil, fmt.Errorf("running sessions for task %s: unknown mode %q", task.ID, task.Sessions.Mode)
	}
}

// isChangeShaped reports whether task expects an edit at all: gold files,
// a hidden test patch, or a detector name the shape PlanFirstInstruction
// exists for — a multi-session task that needs a checklist to hand its
// work off across cold sessions. A pure lookup task (SB-32's
// lookups-vs.json, Oracle.AnswerAllOf with none of these) has nothing to
// plan: appending the instruction there only spent the turn budget on a
// plan the oracle never reads (O45 item 2). A localization task (VB-25,
// Oracle.Localization) is never change-shaped either, even though it
// carries Gold.Files: those name the files the answer must correctly
// NAME, not files this session is meant to edit — the diagnosis IS the
// answer, exactly like AnswerAllOf, so it is excluded here first.
func isChangeShaped(task Task) bool {
	if task.Oracle.Localization {
		return false
	}
	return len(task.Gold.Files) > 0 || task.Gold.TestPatch != "" || len(task.Oracle.Detectors) > 0
}

func runSingle(ctx context.Context, h Harness, task Task, armName string, arm Arm, wt Worktree, opts RunOptions) ([]SessionResult, error) {
	var results []SessionResult
	changeShaped := isChangeShaped(task)
	for i := 0; i < task.Sessions.Count; i++ {
		var prompt string
		switch {
		case i == 0 && changeShaped:
			// A single-session CHANGE task never asked for a plan before —
			// nothing for PlanCoverage/FalseDoneRate to read. Same
			// instruction planner-workers gives its planner, appended
			// here instead of replacing the task prompt.
			prompt = task.Prompt + planFirstInstructionFor(arm)
		case i == 0:
			// A pure-lookup task (isChangeShaped false): the prompt IS the
			// whole ask, verbatim — no plan to write, no checklist to hand
			// off.
			prompt = task.Prompt
		default:
			prompt = scenarioContinuationPrompt
		}
		req, err := newSessionRequest(task, armName, arm, wt, opts, i, "single", "", prompt)
		if err != nil {
			return results, err
		}
		res, err := runOneSession(h, req, wt.Arm.MCPServerNames)
		if err != nil {
			return append(results, res), fmt.Errorf("task %s arm %s session %d: %w", task.ID, armName, i, err)
		}
		results = append(results, res)
		if res.Error != "" {
			return results, nil
		}
	}
	return results, nil
}

func runPlannerWorkers(ctx context.Context, h Harness, task Task, armName string, arm Arm, wt Worktree, opts RunOptions) ([]SessionResult, error) {
	var results []SessionResult
	// RC-32 (docs/vision/28-receipts.md §4): the planner's own session
	// start, captured BEFORE it runs — workerDiscoveryBrief's `--since`
	// must cover what the PLANNER established, not just what happened
	// between the planner and each worker.
	sessionStart := time.Now()
	plannerReq, err := newSessionRequest(task, armName, arm, wt, opts, 0, "planner", "", task.Prompt+planFirstInstructionFor(arm))
	if err != nil {
		return results, err
	}
	plannerRes, err := runOneSession(h, plannerReq, wt.Arm.MCPServerNames)
	if err != nil {
		return append(results, plannerRes), fmt.Errorf("task %s arm %s planner session: %w", task.ID, armName, err)
	}
	results = append(results, plannerRes)
	if plannerRes.Error != "" {
		return results, nil
	}

	rows, err := readPlanRows(wt.Path)
	if err != nil {
		return results, fmt.Errorf("task %s arm %s: reading the planner's plan file: %w", task.ID, armName, err)
	}
	n := task.Sessions.Workers
	if len(rows) < n {
		n = len(rows)
	}
	// RC-32: on a wired arm, the same discoveries brief SubagentStart
	// renders (cmd/fathom's subagentDiscoveries) — run once, against the
	// cell's own scratch daemon, and prepended to every worker's prompt so
	// M5 measures the feature, not the prompt.
	brief, pointers := workerDiscoveryBrief(ctx, wt, arm, sessionStart)
	for i := 0; i < n; i++ {
		row := rows[i]
		rowID := row.id
		if rowID == "" {
			rowID = fmt.Sprintf("worker-%d", i+1)
		}
		prompt := fmt.Sprintf(workerRowPromptFmt, task.Prompt, row.text)
		if brief != "" {
			prompt = brief + "\n\n" + prompt
		}
		req, err := newSessionRequest(task, armName, arm, wt, opts, i+1, "worker", rowID, prompt)
		if err != nil {
			return results, err
		}
		res, err := runOneSession(h, req, wt.Arm.MCPServerNames)
		if err != nil {
			return append(results, res), fmt.Errorf("task %s arm %s worker %d: %w", task.ID, armName, i+1, err)
		}
		if len(pointers) > 0 && factsReusedHook != nil {
			factsReusedHook(&res, pointers)
		}
		results = append(results, res)
		if res.Error != "" {
			return results, nil
		}
	}
	return results, nil
}

// workerDiscoveryBrief runs `fathom discoveries` against a wired cell's
// scratch daemon for the pointers established since sessionStart, and
// returns the text to prepend to each worker's prompt plus the raw
// pointer list (for factsReusedHook). "" / nil whenever there is nothing
// honest to say: an unwired arm, no scratch daemon, no built binary for
// this cell, the command failing, or an empty answer — never a fabricated
// brief (RC-32, docs/vision/28-receipts.md §4).
//
// The subprocess env is the CELL's own, never the operator's: `fathom
// discoveries` reads FATHOM_THREADS (thread.go's threadsPath, default
// ~/.fathom/threads.jsonl) and — via threadScopeFor's own service
// auto-resolution — FATHOM_HTTP (default http://localhost:7575) when no
// override is set; runIn's bare os.Environ() inheritance would otherwise
// hand this subprocess the OPERATOR's real threads file and daemon. Both
// are pinned here (FATHOM_THREADS under wt.CellDir, FATHOM_HTTP to
// wt.DaemonURL), and -service is passed explicitly so the command never
// falls back to that same auto-resolution either.
//
// Note: the scratch daemon wireScratchDaemon (subject.go) starts with
// `-call-log ""` (disabled) for a wired cell, so this call reads an empty
// log until that is revisited — the function itself is correct and
// tested against a populated log; making the daemon actually log is
// subject.go's row, out of this row's file scope.
func workerDiscoveryBrief(ctx context.Context, wt Worktree, arm Arm, sessionStart time.Time) (brief string, pointers []string) {
	if !arm.Wired || wt.DaemonURL == "" {
		return "", nil
	}
	bin := filepath.Join(wt.CellDir, "fathom")
	if _, err := os.Stat(bin); err != nil {
		return "", nil
	}
	// The SAME per-cell FATHOM_THREADS a real session's own clean room
	// gets (cleanRoomEnv, cleanroom.go) — one threads.jsonl per cell, not
	// a second path this function invents on its own.
	env, err := cleanRoomEnv(wt.CellDir)
	if err != nil {
		return "", nil
	}
	env["FATHOM_HTTP"] = wt.DaemonURL
	out, err := runInEnv(ctx, wt.Path, env, bin, "discoveries",
		"-url", wt.DaemonURL, "-service", wt.Service, "-since", sessionStart.UTC().Format(time.RFC3339), "-budget", "1200", "-json")
	if err != nil {
		return "", nil
	}
	var payload struct {
		Rendered string `json:"rendered"`
		Pointers []struct {
			Pointer string `json:"pointer"`
		} `json:"pointers"`
	}
	if json.Unmarshal([]byte(out), &payload) != nil {
		return "", nil
	}
	rendered := strings.TrimSpace(payload.Rendered)
	if rendered == "" {
		return "", nil
	}
	ptrs := make([]string, len(payload.Pointers))
	for i, p := range payload.Pointers {
		ptrs[i] = p.Pointer
	}
	return rendered, ptrs
}

// factsReusedHook is RC-32's TODO seam: FactsReused(session, pointers)
// (docs/vision/28-receipts.md §4 / RC-13) is designed to live in
// readings_resume.go on a sibling branch and was absent from this
// worktree at RC-32 time. A small function-var interface rather than a
// stub — wiring it to the real FactsReused is that row's job, not this
// one's; nil here means "not counted yet", never a faked count.
//
// TODO(RC-13): set this to a thin adapter over FactsReused once it lands.
var factsReusedHook func(session *SessionResult, pointers []string)

func newSessionRequest(task Task, armName string, arm Arm, wt Worktree, opts RunOptions, index int, role, rowID, prompt string) (SessionRequest, error) {
	rawBase := opts.RawDir
	if opts.RunID != "" {
		rawBase = filepath.Join(opts.RawDir, opts.RunID)
	}
	rawDir := filepath.Join(rawBase, fmt.Sprintf("%s-%s-t%d", task.ID, armName, orOne(opts.Trial)))
	env, err := sessionEnv(wt, opts)
	if err != nil {
		return SessionRequest{}, fmt.Errorf("task %s arm %s session %d: %w", task.ID, armName, index, err)
	}
	return SessionRequest{
		Worktree:        wt.Path,
		Prompt:          prompt,
		SystemAppend:    wt.Arm.SystemAppend,
		Arm:             arm,
		MaxTurns:        task.Sessions.MaxTurns,
		Model:           opts.Model,
		RawDir:          rawDir,
		Index:           index,
		Role:            role,
		RowID:           rowID,
		Env:             env,
		MCPConfigPath:   wt.Arm.MCPConfigPath,
		SettingsPath:    wt.Arm.SettingsPath,
		AllowedTools:    wt.Arm.AllowedTools,
		DisallowedTools: wt.Arm.DisallowedTools,
		MCPServerNames:  wt.Arm.MCPServerNames,
	}, nil
}

// sessionEnv is opts.Env with the clean room's own FATHOM_THREADS merged
// underneath it (cleanRoomEnv), keyed on wt.CellDir being set — a
// hand-built Worktree with no CellDir (every existing test fixture that
// predates RC-6) simply gets opts.Env unchanged, same as before RC-6.
// opts.Env, when set, wins on a key collision.
func sessionEnv(wt Worktree, opts RunOptions) (map[string]string, error) {
	if wt.CellDir == "" {
		return opts.Env, nil
	}
	cr, err := cleanRoomEnv(wt.CellDir)
	if err != nil {
		return nil, err
	}
	if wt.DaemonURL != "" {
		// A wired cell's own scratch daemon (subject.go's wireScratchDaemon)
		// ingested ONLY this worktree — never the operator's fleet index
		// at FATHOM_HTTP's default localhost:7575 (O45 item 6). Every
		// `fathom` subprocess the session spawns — its own SessionStart/
		// PostToolUse hooks, any `fathom` CLI call — resolves this URL via
		// defaultFathomURL (cmd/fathom/sessionhook.go) and must reach THIS
		// daemon, or the primer reads "not an indexed service" and no
		// obligation is ever injected (primerInstrumentError catches the
		// residual case where it still does not). Service identity itself
		// needs no separate env var: resolution is by INGESTED PATH
		// (resolveFromServices, cmd/fathom/serviceident.go), and this
		// daemon ingested exactly wt.Path.
		cr["FATHOM_HTTP"] = wt.DaemonURL
	}
	merged := make(map[string]string, len(cr)+len(opts.Env))
	for k, v := range cr {
		merged[k] = v
	}
	for k, v := range opts.Env {
		merged[k] = v
	}
	return merged, nil
}

func orOne(trial int) int {
	if trial <= 0 {
		return 1
	}
	return trial
}

// planRow is one checklist item read from the planner's own plan file.
type planRow struct {
	text string
	id   string
	file string
}

// planRowRE matches a markdown task-list line: optional indent, "- [ ]" or
// "- [x]" (only unchecked rows are assignable to a worker), then the row
// text.
var planRowRE = regexp.MustCompile(`^\s*-\s*\[ \]\s*(.+)$`)

// planRowAnnotationRE pulls a plan-grammar-v2-shaped {id: ...} off a row,
// when present (docs/plans/README.md); a plain checklist has none, and
// readPlanRows falls back to a positional worker-N id in that case.
var planRowAnnotationRE = regexp.MustCompile(`\{\s*id:\s*([A-Za-z0-9_.-]+)`)

// readPlanRows finds the planner's checklist — the first docs/plans/*.md
// file in the worktree (sorted, so the choice is deterministic when more
// than one exists), else NOTES.md at the worktree root — and returns each
// UNCHECKED row in file order. No plan file at all is not an error: it
// returns (nil, nil), and the caller assigns no workers (an empty plan is
// itself a reading, not a failure of RunSessions).
func readPlanRows(worktree string) ([]planRow, error) {
	path, err := findPlanFile(worktree)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file %s: %w", path, err)
	}
	rel, err := filepath.Rel(worktree, path)
	if err != nil {
		rel = path
	}
	var rows []planRow
	for _, line := range strings.Split(string(raw), "\n") {
		m := planRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		row := planRow{text: strings.TrimSpace(m[1]), file: rel}
		if am := planRowAnnotationRE.FindStringSubmatch(m[1]); am != nil {
			row.id = am[1]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// findPlanFile returns the plan file readPlanRows should read: the
// lexicographically first docs/plans/*.md in worktree, else NOTES.md at
// its root, else "" (no plan file was written).
func findPlanFile(worktree string) (string, error) {
	plansDir := filepath.Join(worktree, "docs", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", plansDir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return filepath.Join(plansDir, names[0]), nil
	}
	notes := filepath.Join(worktree, "NOTES.md")
	if _, err := os.Stat(notes); err == nil {
		return notes, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("statting %s: %w", notes, err)
	}
	return "", nil
}

// runOneSession runs one session via h and then applies the
// instrument-truth gate: a wired session whose rendered MCP config
// declared a server (wantServers, wt.Arm.MCPServerNames) that the
// transcript's own init event never reported connected did not run with
// the tools it was measured as having — the defect this closes (72/72
// "full" sessions ran with mcp_servers: [] because RunSessions handed the
// harness the ORIGINAL arm, whose MCP map a wired cell's merged-in fathom
// entry never reached) — and, for a wired arm, that its transcript shows
// no SessionStart hook ever fired (hooksInstrumentError): the same defect
// shape as an absent MCP server, since a wired cell whose `--settings`
// file never actually ran fathom's hooks was not measured with what it
// claims either. Either mismatch is recorded on BOTH res.Error (so the
// existing "stop after an errored session" handling picks it up
// unchanged) and res.InstrumentError, which CellLost treats as
// disqualifying the whole cell regardless of how the other sessions in it
// went. err is non-nil only when the harness failed to even start the
// process.
func runOneSession(h Harness, req SessionRequest, wantServers []string) (SessionResult, error) {
	res, err := h.RunSession(req)
	if err != nil {
		return res, err
	}
	if res.Error == "" {
		msg := instrumentTruthError(wantServers, res)
		if msg == "" {
			msg = hooksInstrumentError(req.Arm.Wired, res)
		}
		if msg == "" {
			msg = primerInstrumentError(req.Arm.Wired, res)
		}
		if msg != "" {
			res.Error = msg
			res.InstrumentError = msg
		}
	}
	return res, nil
}

// primerInstrumentError (O45 item 7) reports whether a wired session's own
// SessionStart primer proves it actually reached the CELL's scratch
// daemon: cmd/fathom's buildSessionContext names the indexed service by
// its own resolved identity ("… this repo is the indexed service …") only
// when a live daemon at FATHOM_HTTP recognizes the worktree's path as one
// it ingested; a session whose FATHOM_HTTP still pointed at the
// operator's real daemon (or none) gets back "… this repo (…) is not an
// indexed service …" instead — the same worktree that never wires an
// obligation, since the PostToolUse hook resolves identity the same way.
// "" for an unwired arm (no primer is expected at all) or a wired session
// whose SessionStart hook never fired at all (hooksInstrumentError already
// gates that case, so this function is never reached with an empty
// primer for that reason).
func primerInstrumentError(wired bool, res SessionResult) string {
	if !wired {
		return ""
	}
	if res.HooksFired["SessionStart"] == 0 {
		return ""
	}
	switch {
	case strings.Contains(res.SessionStartContext, "not an indexed service"):
		return "instrument: SessionStart primer reports this worktree is not an indexed service"
	case !strings.Contains(res.SessionStartContext, "is the indexed service"):
		return "instrument: SessionStart primer named no indexed service"
	}
	return ""
}

// instrumentTruthError reports the first wantServers entry (in the given,
// deterministic order) that res.MCPServers does not show connected:
// "instrument: mcp server <name> absent" when the transcript's init event
// never mentioned it at all, "instrument: mcp server <name> <status>"
// when it did but not as connected. Empty wantServers (an arm with no MCP
// entries) always reports "".
func instrumentTruthError(wantServers []string, res SessionResult) string {
	for _, name := range wantServers {
		status, ok := res.MCPServers[name]
		switch {
		case !ok:
			return fmt.Sprintf("instrument: mcp server %s absent", name)
		case status != "connected":
			return fmt.Sprintf("instrument: mcp server %s %s", name, status)
		}
	}
	return ""
}

// hooksInstrumentError reports "instrument: no SessionStart hook fired"
// when wired is true and res.HooksFired shows no SessionStart entry — a
// wired session's `--settings` file is supposed to carry the real fathom
// hook stack (RenderArm/subject.go), and a session whose own transcript
// never shows that hook running was not measured with it, exactly like a
// declared MCP server that never connected. "" for an unwired arm, which
// declares no hook at all.
func hooksInstrumentError(wired bool, res SessionResult) string {
	if !wired {
		return ""
	}
	if res.HooksFired["SessionStart"] > 0 {
		return ""
	}
	return "instrument: no SessionStart hook fired"
}

// CellLost reports why a cell is not a reading: any session carrying an
// instrument error (instrumentTruthError/hooksInstrumentError, via
// runOneSession), every session errored some other way, or no session
// ever produced a turn. A single instrument-broken session voids the
// whole cell regardless of how the other sessions in it went — unlike a
// session that merely errored in-band, which only loses the cell when
// EVERY session did. A lost cell is never stored or scored.
func CellLost(sessions []SessionResult) string {
	if len(sessions) == 0 {
		return "no sessions ran"
	}
	for _, s := range sessions {
		if s.InstrumentError != "" {
			return "instrument error: " + s.InstrumentError
		}
	}
	errs := 0
	turns := 0
	for _, s := range sessions {
		if s.Error != "" {
			errs++
		}
		turns += s.Turns
	}
	if errs == len(sessions) {
		return "every session errored: " + sessions[0].Error
	}
	if turns == 0 {
		return "no session produced a turn"
	}
	return ""
}

// AnyEdits reports whether any session edited a file: the floor under
// "success" — an agent that did nothing cannot pass a change task by
// never violating a rule.
func AnyEdits(sessions []SessionResult) bool {
	for _, s := range sessions {
		if len(s.FilesEdited) > 0 {
			return true
		}
	}
	return false
}

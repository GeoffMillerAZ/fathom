// RunSessions (SB-13) executes one task's sessions in order, under the
// budget task.Sessions declares: mode "single" runs Count cold sessions of
// the same prompt, mode "planner-workers" runs one planning session and
// then one cold worker session per row the planner's plan file names. The
// prompt is identical across arms; the only thing that may differ by arm is
// the text RenderArm declared (wt.Arm.SystemAppend).
package scenariobench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

func runSingle(ctx context.Context, h Harness, task Task, armName string, arm Arm, wt Worktree, opts RunOptions) ([]SessionResult, error) {
	var results []SessionResult
	for i := 0; i < task.Sessions.Count; i++ {
		var prompt string
		switch {
		case i == 0:
			// A single-session task never asked for a plan before —
			// nothing for PlanCoverage/FalseDoneRate to read. Same
			// instruction planner-workers gives its planner, appended
			// here instead of replacing the task prompt.
			prompt = task.Prompt + PlanFirstInstruction
		default:
			prompt = scenarioContinuationPrompt
		}
		req := newSessionRequest(task, armName, arm, wt, opts, i, "single", "", prompt)
		res, err := h.RunSession(req)
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
	plannerReq := newSessionRequest(task, armName, arm, wt, opts, 0, "planner", "", task.Prompt+PlanFirstInstruction)
	plannerRes, err := h.RunSession(plannerReq)
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
	for i := 0; i < n; i++ {
		row := rows[i]
		rowID := row.id
		if rowID == "" {
			rowID = fmt.Sprintf("worker-%d", i+1)
		}
		prompt := fmt.Sprintf(workerRowPromptFmt, task.Prompt, row.text)
		req := newSessionRequest(task, armName, arm, wt, opts, i+1, "worker", rowID, prompt)
		res, err := h.RunSession(req)
		if err != nil {
			return append(results, res), fmt.Errorf("task %s arm %s worker %d: %w", task.ID, armName, i+1, err)
		}
		results = append(results, res)
		if res.Error != "" {
			return results, nil
		}
	}
	return results, nil
}

func newSessionRequest(task Task, armName string, arm Arm, wt Worktree, opts RunOptions, index int, role, rowID, prompt string) SessionRequest {
	rawDir := filepath.Join(opts.RawDir, fmt.Sprintf("%s-%s-t%d", task.ID, armName, orOne(opts.Trial)))
	return SessionRequest{
		Worktree:     wt.Path,
		Prompt:       prompt,
		SystemAppend: wt.Arm.SystemAppend,
		Arm:          arm,
		MaxTurns:     task.Sessions.MaxTurns,
		Model:        opts.Model,
		RawDir:       rawDir,
		Index:        index,
		Role:         role,
		RowID:        rowID,
		Env:          opts.Env,
	}
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

// CellLost reports why a cell is not a reading: every session errored, or
// no session ever produced a turn. A lost cell is never stored or scored.
func CellLost(sessions []SessionResult) string {
	if len(sessions) == 0 {
		return "no sessions ran"
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

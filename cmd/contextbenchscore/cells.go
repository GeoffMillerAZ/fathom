package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// The cell flags every subcommand that touches the cell store shares
// (ADR-032 §2). They are the invariants a cell is bought under; the runner
// computes each one once and passes the same values to `borrow`, `score`
// and `ledger`, so the three agree on what a cell key means.
type cellOpts struct {
	ModelID        *string
	HarnessVersion *string
	PromptSubject  *string
	ArmSurface     *string
}

func cellFlags(fs *flag.FlagSet) cellOpts {
	return cellOpts{
		ModelID:        fs.String("model-id", "", "model identity the run is pinned to (a dated id, else the alias)"),
		HarnessVersion: fs.String("harness-version", "", "`claude --version` string the runner captured"),
		PromptSubject:  fs.String("prompt-subject", "", "the runner's --prompt-subject"),
		ArmSurface:     fs.String("arm-surface", "", "the subject this arm queried: the fathom binary commit for the fathom arm, empty for grep and none"),
	}
}

// inputs builds the CellInputs for one (task, arm), hashing the corpus
// file itself so the key cannot drift from the questions it scored.
func (c cellOpts) inputs(tasksPath, taskID, arm, scoring string) contextbench.CellInputs {
	sha, err := contextbench.CorpusSHA256(tasksPath)
	if err != nil {
		// An unreadable corpus is caught by the caller's own load; leave
		// the hash empty so Validate refuses rather than keying on "".
		sha = ""
	}
	return contextbench.CellInputs{
		CorpusSHA256:   sha,
		TaskID:         taskID,
		Arm:            arm,
		ModelID:        *c.ModelID,
		HarnessVersion: *c.HarnessVersion,
		Scoring:        scoring,
		PromptSubject:  *c.PromptSubject,
		ArmSurface:     *c.ArmSurface,
	}
}

// cmdBorrow is the runner's pre-flight: if an earlier run already bought
// this exact cell — same corpus, task, arm, model, harness, scoring,
// prompt subject and arm surface — copy its reading into this run instead
// of paying for it again. Exit 3 (not 1) on a miss: a miss is the normal
// case, not a failure.
func cmdBorrow(args []string) int {
	fs := flag.NewFlagSet("borrow", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to the corpus JSON")
	taskID := fs.String("task-id", "", "task id")
	arm := fs.String("arm", "", "arm name")
	trial := fs.Int("trial", 1, "1-based trial index")
	scoring := fs.String("scoring", contextbench.ScoringSubstring, "substring or precision")
	cells := fs.String("cells", "", "content-addressed cell store")
	outPath := fs.String("out", "", "path to write the borrowed ArmResult JSON")
	cell := cellFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || *taskID == "" || *arm == "" || *cells == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "borrow: -tasks, -task-id, -arm, -cells and -out are all required")
		return 2
	}
	in := cell.inputs(*tasksPath, *taskID, *arm, *scoring)
	if err := in.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "borrow: %v\n", err)
		return 2
	}
	key := contextbench.CellKey(in)
	got, ok, err := contextbench.BorrowCell(*cells, key, *trial)
	if err != nil {
		fmt.Fprintf(os.Stderr, "borrow: %v\n", err)
		return 2
	}
	if !ok {
		return 3
	}
	got.Trial = *trial
	raw, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "borrow: %v\n", err)
		return 2
	}
	if err := os.WriteFile(*outPath, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "borrow: writing %s: %v\n", *outPath, err)
		return 2
	}
	fmt.Printf("BORROWED %s/%s/t%d from run %s (key %s…)\n", got.TaskID, got.Arm, *trial, got.SourceRunID, key[:12])
	return 0
}

// cmdSelect maps a diff to the tasks worth re-running (BP-7). It runs
// nothing and spends nothing: git plus the committed handler table.
func cmdSelect(args []string) int {
	fs := flag.NewFlagSet("select", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to the corpus JSON")
	diff := fs.String("diff", "", "git range whose changed files select tasks (e.g. origin/main...HEAD)")
	files := fs.String("files", "", "explicit comma/space-separated file list instead of -diff")
	repo := fs.String("repo", ".", "repository to run git in")
	asJSON := fs.Bool("json", false, "print the full selection as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || (*diff == "" && *files == "") {
		fmt.Fprintln(os.Stderr, "select: -tasks and one of -diff / -files are required")
		return 2
	}
	tasks, err := contextbench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "select: %v\n", err)
		return 1
	}
	var changed []string
	if *files != "" {
		changed = splitList(*files)
	} else {
		out, err := exec.Command("git", "-C", *repo, "diff", "--name-only", *diff).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "select: git diff --name-only %s: %v\n", *diff, err)
			return 2
		}
		for _, line := range strings.Split(string(out), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				changed = append(changed, line)
			}
		}
	}
	sel := contextbench.SelectTasks(changed, tasks)
	if *asJSON {
		raw, _ := json.MarshalIndent(sel, "", "  ")
		fmt.Println(string(raw))
		return 0
	}
	for _, id := range sel.TaskIDs {
		fmt.Println(id)
	}
	fmt.Fprintf(os.Stderr, "select: %d of %d task(s) from %d changed file(s); tools: %s\n",
		len(sel.TaskIDs), len(tasks), len(changed), strings.Join(sel.Tools, " "))
	if len(sel.Unmapped) > 0 {
		fmt.Fprintf(os.Stderr, "select: %d changed file(s) map to no tool — a targeted run over this diff cannot be a verdict: %s\n",
			len(sel.Unmapped), strings.Join(sel.Unmapped, " "))
	}
	if len(sel.Undeclared) > 0 {
		fmt.Fprintf(os.Stderr, "select: %d task(s) declare no exercises and can never be selected: %s\n",
			len(sel.Undeclared), strings.Join(sel.Undeclared, " "))
	}
	return 0
}

// applyTranscriptMetrics fills the H2-3 readings on a scored cell from its
// stream transcript (BP-9).
func applyTranscriptMetrics(ar *contextbench.ArmResult, transcript []byte, gold []string) {
	st := contextbench.TranscriptMetrics(transcript, gold)
	ar.PayloadBytes = st.PayloadBytes
	ar.ToolSearchTurns = st.ToolSearchTurns
	ar.TurnsExclToolSearch = st.TurnsExclToolSearch
	ar.GoldSourceTool = st.GoldSourceTool

	// RC-17 (F8): trusted_without_recheck, per fathom tool call in this cell.
	if twr := contextbench.TrustedWithoutRecheck(transcript); twr.Calls > 0 {
		ar.TrustedWithoutRecheck = &twr
	}
}

// applyPathChecks fills the VB-36 (O47) safety readings on a scored cell
// from its stream transcript: liveCheckout/sessionRoot are the runner's own
// absolute paths, and either may be "" when the runner has nothing to
// check against (a control run, or an operator run against a daemon it
// does not know the checkout path for) — in which case the corresponding
// reading is left at its zero value, "not checked", never a false "clean".
func applyPathChecks(ar *contextbench.ArmResult, transcript []byte, liveCheckout, sessionRoot string) {
	if liveCheckout != "" {
		leak := contextbench.DetectPathLeak(transcript, liveCheckout)
		ar.PathLeak = leak.Leaked
		ar.PathLeakPaths = leak.Paths
	}
	if sessionRoot != "" {
		reads := contextbench.DetectOutOfRootReads(transcript, sessionRoot)
		ar.OutOfRootReads = len(reads)
		for _, r := range reads {
			ar.OutOfRootPaths = append(ar.OutOfRootPaths, r.Tool+": "+r.Path)
		}
	}
}

// hasBorrowed reports whether any cell in results was inherited from an
// earlier run rather than paid for by this one.
func hasBorrowed(results []contextbench.ArmResult) bool {
	for _, r := range results {
		if r.Borrowed {
			return true
		}
	}
	return false
}

// fillTranscriptMetrics re-reads each cell's stream transcript from dir
// (the runner's raw/<task>-<arm>-t<n>.stream.jsonl) and fills the H2-3
// readings on cells that lack them — how an existing run is scored under a
// criterion registered after it was paid for (BP-11). A cell whose
// transcript is missing keeps whatever it recorded; a transcript that
// cannot be READ is an error, never a silent zero (ADR-027/U14).
func fillTranscriptMetrics(results []contextbench.ArmResult, tasks []contextbench.Task, dir string) ([]contextbench.ArmResult, error) {
	gold := map[string][]string{}
	for _, t := range tasks {
		gold[t.ID] = t.GroundTruth
	}
	out := make([]contextbench.ArmResult, 0, len(results))
	for _, r := range results {
		trial := r.Trial
		if trial < 1 {
			trial = 1
		}
		path := filepath.Join(dir, fmt.Sprintf("%s-%s-t%d.stream.jsonl", r.TaskID, r.Arm, trial))
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				out = append(out, r)
				continue
			}
			return nil, fmt.Errorf("reading transcript %s: %w", path, err)
		}
		applyTranscriptMetrics(&r, raw, gold[r.TaskID])
		out = append(out, r)
	}
	return out, nil
}

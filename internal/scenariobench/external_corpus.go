// external_corpus.go is VB-25's own harness adapter (vision 29 Wave G,
// docs/vision/29-verbs-not-lookups.md §3 row G; plan docs/plans/verbs.md
// VB-25): RunExternalCorpus drives a corpus fathom does not own (Long
// Code Arena's bug-localization track, lca.go) through this package's OWN
// cell/session/oracle/ledger machinery — PrepareSubject, RunSessions,
// RunOracle, StoreScenarioCell, AggregateRowReadings, AppendScenarioLedger
// — the same primitives cmd/scenariobench's run/ledger subcommands use,
// so an external-corpus cell is measured under the identical
// instrument-truth gate (CellLost, instrumentTruthError,
// hooksInstrumentError) every other cell is: server connected, hooks
// fired, surface calls — never trusted merely because an arm claims it is
// wired.
//
// Unlike cmd/scenariobench's own runOneCell, this adapter never captures
// the SDD readings (plan coverage, false-done, drift, resumption,
// rule-served): a Localization-shaped task is change-shaped=false (no
// plan-first — isChangeShaped) and produces no plan or diff for those
// readings to read.
package scenariobench

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
)

// RunExternalCorpusOptions is what RunExternalCorpus needs to run every
// (task, arm, trial) cell of an external corpus and file the resulting
// rows.
type RunExternalCorpusOptions struct {
	// Root is this repo's root (PrepareSubject's own `root`: where
	// `go build ./cmd/fathom` runs from for a wired arm).
	Root string
	// Corpus is the already-built corpus (BuildLCACorpus, or any other
	// Corpus a caller assembles by hand) — every non-dropped task runs
	// across every arm Corpus.Arms declares.
	Corpus Corpus
	// CorpusSHA256 identifies Corpus for the cell key: CanonicalCorpusSHA256
	// for a corpus built in memory, CorpusSHA256(path) for one loaded from
	// a committed file.
	CorpusSHA256 string
	Harness      Harness
	RunID        string
	Model        string
	RawDir       string
	CellsDir     string
	// Trials per (task, arm); less than 1 defaults to 1.
	Trials int
	// ModelID / HarnessVersion / Scoring / FathomCommit are the same
	// cell-key invariants cmd/scenariobench's cellFlags carries — see
	// CellInvariants (cell.go).
	ModelID        string
	HarnessVersion string
	Scoring        string
	FathomCommit   string
	// LedgerPath, when non-empty, appends every filed row to this JSONL
	// ledger (AppendScenarioLedger). Left empty, RunExternalCorpus still
	// returns every row — it is simply not filed anywhere.
	LedgerPath string
	// Commit is the repo HEAD this run measured, recorded verbatim on
	// every row (ScenarioLedgerRow.Commit) — resolving it (e.g. `git
	// rev-parse --short HEAD`) is the caller's own responsibility, exactly
	// as cmd/scenariobench's resolveFathomCommit is; RunExternalCorpus
	// never guesses at it.
	Commit string
}

// RunExternalCorpusResult is everything one RunExternalCorpus call
// produced: every cell it ran or borrowed, the ledger rows built from
// them (one per task/arm), and the cells the instrument-truth gate
// (CellLost) voided before they ever became a reading.
type RunExternalCorpusResult struct {
	Cells []CellResult
	Rows  []ScenarioLedgerRow
	// Lost names every (task, arm, trial) CellLost voided, with its own
	// reason — never stored, never scored, never folded into a row.
	Lost []LostCell
}

// LostCell is one cell the instrument-truth gate voided before it became
// a reading.
type LostCell struct {
	TaskID string
	Arm    string
	Trial  int
	Reason string
}

// harnessIdentitySetter is implemented by FakeHarness (SetIdentity):
// unlike cmd/scenariobench's own resolveHarness, which constructs a fresh
// FakeHarness per cell with TaskID/ArmName preset, RunExternalCorpus holds
// ONE shared Harness across every (task, arm) it runs — a real
// ClaudeHarness needs no per-cell identity at all, but a FakeHarness's own
// stream lookup does (SessionRequest itself carries no task/arm identity —
// harness_fake.go), so this is set just before each RunSessions call when
// the harness supports it.
type harnessIdentitySetter interface {
	SetIdentity(taskID, armName string)
}

// RunExternalCorpus runs every non-dropped task of opts.Corpus across
// every declared arm and opts.Trials trials: borrow a stored cell when
// one already matches this run's own invariants (BorrowScenarioCell),
// otherwise PrepareSubject, RunSessions, RunOracle (which scores
// ScoreLocalization for a Localization-shaped task), aggregate readings
// and gold-source, apply the instrument-truth gate (CellLost) — a voided
// cell is never stored, counted into result.Lost instead — then
// StoreScenarioCell. Every (task, arm)'s own cells are grouped into one
// ScenarioLedgerRow (GoldSourceRate, SurfaceUnused, the Localization row
// aggregates — AggregateRowReadings), appended to opts.LedgerPath when
// set, and returned alongside every cell.
func RunExternalCorpus(ctx context.Context, opts RunExternalCorpusOptions) (RunExternalCorpusResult, error) {
	if opts.Harness == nil {
		return RunExternalCorpusResult{}, fmt.Errorf("running external corpus: no harness")
	}
	if opts.RunID == "" {
		return RunExternalCorpusResult{}, fmt.Errorf("running external corpus: no run id")
	}
	if opts.CorpusSHA256 == "" {
		return RunExternalCorpusResult{}, fmt.Errorf("running external corpus: no corpus_sha256 — a cell key over an unknown corpus would let an unrelated cell be borrowed")
	}
	trials := opts.Trials
	if trials < 1 {
		trials = 1
	}
	root := opts.Root
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}

	armNames := make([]string, 0, len(opts.Corpus.Arms))
	for name := range opts.Corpus.Arms {
		armNames = append(armNames, name)
	}
	sort.Strings(armNames)

	type taskArmKey struct{ task, arm string }
	byTaskArm := map[taskArmKey][]CellResult{}
	var order []taskArmKey
	var result RunExternalCorpusResult

	for _, task := range opts.Corpus.Tasks {
		// Mirrors planCells (cmd/scenariobench/main.go): a task the
		// corpus marked dropped never becomes a cell at all.
		if task.Disposition == "dropped" {
			continue
		}
		for _, armName := range armNames {
			arm := opts.Corpus.Arms[armName]
			surface := ResolveArmSurface(arm, opts.FathomCommit)
			inv := CellInvariants{
				CorpusSHA256:   opts.CorpusSHA256,
				TaskID:         task.ID,
				Arm:            armName,
				ModelID:        opts.ModelID,
				HarnessVersion: opts.HarnessVersion,
				Scoring:        opts.Scoring,
				PromptSubject:  externalPromptSubjectFor(task),
				ArmSurface:     surface,
			}
			key := ScenarioCellKey(inv)
			tak := taskArmKey{task.ID, armName}
			if _, seen := byTaskArm[tak]; !seen {
				order = append(order, tak)
				byTaskArm[tak] = nil
			}

			for trial := 1; trial <= trials; trial++ {
				cell, borrowed, err := BorrowScenarioCell(opts.CellsDir, key, trial)
				if err != nil {
					return RunExternalCorpusResult{}, fmt.Errorf("checking cell store for %s/%s/t%d: %w", task.ID, armName, trial, err)
				}
				if !borrowed {
					cell, err = runExternalCell(ctx, root, task, armName, arm, opts.Harness, opts.RawDir, opts.Model, opts.RunID, trial, inv, key)
					if err != nil {
						return RunExternalCorpusResult{}, fmt.Errorf("%s/%s/t%d: %w", task.ID, armName, trial, err)
					}
					if reason := CellLost(cell.Sessions); reason != "" {
						result.Lost = append(result.Lost, LostCell{TaskID: task.ID, Arm: armName, Trial: trial, Reason: reason})
						continue
					}
					if err := StoreScenarioCell(opts.CellsDir, cell); err != nil {
						return RunExternalCorpusResult{}, fmt.Errorf("storing %s/%s/t%d: %w", task.ID, armName, trial, err)
					}
				}
				result.Cells = append(result.Cells, cell)
				byTaskArm[tak] = append(byTaskArm[tak], cell)
			}
		}
	}

	taskByID := make(map[string]Task, len(opts.Corpus.Tasks))
	for _, t := range opts.Corpus.Tasks {
		taskByID[t.ID] = t
	}
	for _, tak := range order {
		group := byTaskArm[tak]
		if len(group) == 0 {
			continue
		}
		row := buildExternalLedgerRow(opts, tak.task, tak.arm, group, taskByID[tak.task], opts.Corpus.Arms[tak.arm])
		result.Rows = append(result.Rows, row)
	}

	if opts.LedgerPath != "" {
		for _, row := range result.Rows {
			if err := AppendScenarioLedger(opts.LedgerPath, row); err != nil {
				return RunExternalCorpusResult{}, err
			}
		}
	}
	return result, nil
}

// externalPromptSubjectFor mirrors cmd/scenariobench's own
// promptSubjectFor (main.go) — kept as its own small copy here rather
// than an exported/shared symbol so a change to the CLI's own prompt
// text can never silently move a package-level cell key out from under
// it.
func externalPromptSubjectFor(task Task) string {
	svc := task.Subject.Service
	if svc == "" {
		svc = "the subject"
	}
	return "the " + svc + " codebase"
}

// runExternalCell runs one (task, arm, trial) cell: PrepareSubject,
// RunSessions, RunOracle, and the readings a non-change-shaped task
// supports — basic aggregates, surface calls, gold source, and (for a
// Localization task) OracleResult.Localization carried onto Readings.
// Never the SDD readings (plan coverage, false-done, drift, resumption,
// rule-served): those need a captured plan/diff a change-shaped task
// alone produces.
func runExternalCell(ctx context.Context, root string, task Task, armName string, arm Arm, harness Harness, rawDir, model, runID string, trial int, inv CellInvariants, key string) (CellResult, error) {
	if setter, ok := harness.(harnessIdentitySetter); ok {
		setter.SetIdentity(task.ID, armName)
	}
	wt, err := PrepareSubject(ctx, task, arm, root)
	if err != nil {
		return CellResult{}, fmt.Errorf("preparing subject: %w", err)
	}
	defer StopWorktree(&wt)

	sessions, err := RunSessions(ctx, harness, task, armName, arm, wt, RunOptions{
		RunID: runID, Trial: trial, Model: model, RawDir: rawDir,
	})
	if err != nil {
		return CellResult{}, fmt.Errorf("running sessions: %w", err)
	}

	oracle, err := RunOracle(ctx, wt, task, sessions, root)
	if err != nil {
		return CellResult{}, fmt.Errorf("running oracle: %w", err)
	}

	readings := AggregateReadings(sessions)
	readings.SurfaceCalls = SurfaceCallCount(readings.ToolCalls, arm.Surface)
	ApplyCompileCost(&readings, wt)
	readings.GoldSourceTool, readings.GoldSourceEvidence = GoldSourceTool(sessions, task)
	readings.Localization = oracle.Localization

	// Fix-first review: the any-edits floor is the ONE SuccessFor formula
	// (oracle.go) every Success-setting caller shares — this used to carry
	// its own copy, missing the RefusalAware exemption runOneCell's copy
	// had (O57 finding 2), scoring a correct refusal on an externally-run
	// cell FAIL for never editing anything, by design.
	success := SuccessFor(task, oracle, sessions)

	return CellResult{
		TaskID:         task.ID,
		Arm:            armName,
		Trial:          trial,
		Category:       task.Category,
		Sessions:       sessions,
		Oracle:         oracle,
		Readings:       readings,
		Success:        success,
		SelfSkip:       SelfSkipDetected(sessions),
		CellKey:        key,
		CorpusSHA256:   inv.CorpusSHA256,
		ModelID:        inv.ModelID,
		HarnessVersion: inv.HarnessVersion,
		ArmSurface:     inv.ArmSurface,
		Surface:        arm.Surface,
		Gold:           task.Gold,
		RunID:          runID,
		WorktreePath:   wt.Path,
		SubjectClone:   wt.SubjectClone,
		BaseCommit:     task.Subject.BaseCommit,
	}, nil
}

// buildExternalLedgerRow builds one ScenarioLedgerRow from group (every
// stored/borrowed cell for one (task, arm)) — mirrors cmd/scenariobench's
// own cmdLedger row construction (GoldSourceRate/GoldSourcePasses,
// Prediction, SurfaceUnused) so a row this adapter files reads exactly
// like one `scenariobench ledger` files by hand.
func buildExternalLedgerRow(opts RunExternalCorpusOptions, taskID, armName string, group []CellResult, task Task, arm Arm) ScenarioLedgerRow {
	row := ScenarioLedgerRow{
		RunID:        opts.RunID,
		Commit:       opts.Commit,
		CorpusSHA256: opts.CorpusSHA256,
		// CriterionID mirrors cmdLedger's own stamp — see
		// ScenarioLedgerRow.CriterionID.
		CriterionID:    opts.Corpus.Criterion.ID,
		TaskID:         taskID,
		Category:       task.Category,
		Arm:            armName,
		ModelID:        opts.ModelID,
		HarnessVersion: opts.HarnessVersion,
	}
	// AggregateRowReadings is the single source of truth for a row's own
	// Readings (every count summed, every ratio averaged with its own n,
	// VB-25's Localization aggregates included) plus the full per-trial
	// detail.
	row.Readings, row.PerTrial = AggregateRowReadings(group)

	goldCounts := map[string]int{}
	goldPassCounts := map[string]int{}
	for _, cell := range group {
		row.Trials++
		if cell.Success {
			row.Successes++
		}
		row.CostUSD += cell.Readings.TotalCostUSD
		tool := cell.Readings.GoldSourceTool
		if tool == "" {
			tool = GoldSourceNotMeasured
		}
		goldCounts[tool]++
		if cell.Success {
			goldPassCounts[tool]++
		}
	}
	if row.Trials > 0 {
		row.GoldSourceRate = make(map[string]float64, len(goldCounts))
		for tool, n := range goldCounts {
			row.GoldSourceRate[tool] = float64(n) / float64(row.Trials)
		}
	}
	if len(goldPassCounts) > 0 {
		row.GoldSourcePasses = goldPassCounts
	}
	row.Prediction = task.Predictions[armName]
	row.Readings.SurfaceCalls = SurfaceCallCount(row.Readings.ToolCalls, arm.Surface)
	if arm.Surface != "" && row.Readings.SurfaceCalls == 0 {
		row.SurfaceUnused = true
	}
	row.Borrowed = BorrowedScenarioCellsOf(group)
	return row
}

// SU-24 (docs/plans/substrate.md section F, ADR-041 decision 9): the pure
// cell-enumeration logic every committed corpus must pass through before a
// single cell is ever run. cmd/scenariobench's own `planCells`
// (cmd/scenariobench/main.go) is package main and cannot be imported from
// here (docs/bench/scenarios.md: "internal/scenariobench cannot import
// cmd/fathom, package main" — the same boundary applies to
// cmd/scenariobench), so PlanCells below is built from the SAME exported
// primitives planCells itself calls (CellInvariants, ScenarioCellKey,
// ResolveArmSurface) in the same shape (skip a dropped task, sorted arm
// names, one cell per (task, arm, trial)) — the part of `scenariobench
// plan` that never touches the network or a subject checkout at all: no
// clone, no worktree, no daemon. TestAllCorporaPlan (allcorpora_plan_test.go)
// is this row's own proof that every committed corpus plans cleanly through
// it.
package scenariobench

import "sort"

// PlanOptions is what PlanCells needs beyond the corpus itself — the same
// inputs cmd/scenariobench's own cellFlags resolve into a CellInvariants
// per cell.
type PlanOptions struct {
	FathomCommit   string
	ModelID        string
	HarnessVersion string
	Scoring        string
	Trials         int
}

// CellPlan is one (task, arm, trial) cell's identity — the same shape
// cmd/scenariobench's own package-local cellPlan carries, exported here so
// a test (or a future caller) outside cmd/scenariobench can enumerate a
// corpus's cells without a subject checkout.
type CellPlan struct {
	Task  Task
	Arm   string
	Trial int
	Inv   CellInvariants
	Key   string
}

// planPromptSubject mirrors cmd/scenariobench's own promptSubjectFor
// (main.go) verbatim, including its "the subject" fallback — kept as a
// private mirror rather than a shared export since PromptSubject is cosmetic
// (part of the cell key, but no caller other than the driver ever reads it
// back out), and a divergence here would not be a defect in either copy.
func planPromptSubject(task Task) string {
	svc := task.Subject.Service
	if svc == "" {
		svc = "the subject"
	}
	return "the " + svc + " codebase"
}

// PlanCells enumerates every (task, arm, trial) cell a corpus's own
// `scenariobench plan` would list, in the same order (task, then sorted arm
// name, then trial 1..Trials) — never running anything, never touching the
// network or a subject checkout. A task carrying Disposition == "dropped"
// (RC-15) contributes no cells, exactly as cmd/scenariobench's own
// planCells skips it. corpusSHA256 is the corpus's own content identity —
// CorpusSHA256(path) for a committed corpus file, CanonicalCorpusSHA256(c)
// for one built in memory (the LCA external corpus, lca.go).
func PlanCells(c Corpus, opts PlanOptions, corpusSHA256 string) []CellPlan {
	armNames := make([]string, 0, len(c.Arms))
	for name := range c.Arms {
		armNames = append(armNames, name)
	}
	sort.Strings(armNames)

	trials := opts.Trials
	if trials <= 0 {
		trials = 1
	}

	var out []CellPlan
	for _, task := range c.Tasks {
		if task.Disposition == "dropped" {
			continue
		}
		for _, armName := range armNames {
			arm := c.Arms[armName]
			surface := ResolveArmSurface(arm, opts.FathomCommit)
			for trial := 1; trial <= trials; trial++ {
				inv := CellInvariants{
					CorpusSHA256:   corpusSHA256,
					TaskID:         task.ID,
					Arm:            armName,
					ModelID:        opts.ModelID,
					HarnessVersion: opts.HarnessVersion,
					Scoring:        opts.Scoring,
					PromptSubject:  planPromptSubject(task),
					ArmSurface:     surface,
				}
				out = append(out, CellPlan{
					Task:  task,
					Arm:   armName,
					Trial: trial,
					Inv:   inv,
					Key:   ScenarioCellKey(inv),
				})
			}
		}
	}
	return out
}

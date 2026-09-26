package console

// proof.go — the verify ledger as the console reads it (vision 35 §3, Now
// and Fleet). Every `fathom verify` run appends one row per plan to the
// user-central verify ledger (cmd/fathom/verifyledger.go). The console never
// runs a verify to render a page — 45 plans is about 70 seconds — so a
// plan's proof state is the NEWEST run recorded for it at the project's
// registered root, named as such with its time and commit. Runs recorded in
// scratch clones, worktrees and bench subjects belong to other roots and are
// not the project's state.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

// VerifyRun is one verify-ledger row: one plan, one run.
type VerifyRun struct {
	At       string `json:"at"`
	RepoRoot string `json:"repo_root,omitempty"`
	Service  string `json:"service,omitempty"`
	Plan     string `json:"plan"`
	Commit   string `json:"commit,omitempty"`
	Total    int    `json:"total"`
	Claimed  int    `json:"claimed"`
	Verified int    `json:"verified"`
	Failing  int    `json:"failing"`
	Origin   string `json:"origin,omitempty"`
}

// Unproven is how many claimed rows the run could neither prove nor refute
// (no contract the gate can test, an unknown verdict): `verify --strict`
// refuses them, the default gate does not.
func (r VerifyRun) Unproven() int {
	if n := r.Claimed - r.Verified - r.Failing; n > 0 {
		return n
	}
	return 0
}

// ParseVerifyLedger reads ledger bytes into runs; malformed counts lines
// that did not parse (named by the caller, never silently dropped).
func ParseVerifyLedger(data []byte) (runs []VerifyRun, malformed int) {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r VerifyRun
		if err := json.Unmarshal(line, &r); err != nil || r.Plan == "" || r.At == "" {
			malformed++
			continue
		}
		runs = append(runs, r)
	}
	return runs, malformed
}

// NewestRunsAt returns the newest run of each plan recorded at root, with
// the plan named relative to root (a run may record its plan absolute), in
// plan order. sameRoot reports whether a run's recorded root is root (a
// symlinked spelling of the same directory counts); exists, when non-nil,
// drops plans no longer on disk — an archived plan's last run is history,
// not the project's state.
func NewestRunsAt(runs []VerifyRun, root string, sameRoot func(recorded string) bool, exists func(plan string) bool) []VerifyRun {
	if root == "" {
		return nil
	}
	newest := map[string]VerifyRun{}
	for _, r := range runs {
		if !sameRoot(r.RepoRoot) {
			continue
		}
		plan := RelPlan(root, r.Plan)
		r.Plan = plan
		if prev, ok := newest[plan]; !ok || r.At > prev.At {
			newest[plan] = r
		}
	}
	out := make([]VerifyRun, 0, len(newest))
	for plan, r := range newest {
		if exists != nil && !exists(plan) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Plan < out[j].Plan })
	return out
}

// RelPlan names a recorded plan path relative to root when it lies under
// it; anything else is returned cleaned.
func RelPlan(root, plan string) string {
	p := filepath.Clean(plan)
	if filepath.IsAbs(p) && root != "" {
		if rel, err := filepath.Rel(filepath.Clean(root), p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

// PlanProof totals a project's newest run per plan.
type PlanProof struct {
	Plans    int `json:"plans"`
	Rows     int `json:"rows"`
	Claimed  int `json:"claimed"`
	Verified int `json:"verified"`
	Failing  int `json:"failing"`
	Unproven int `json:"unproven"`
	// Unclaimed rows are open, in progress or skipped: the ledger does not
	// tell them apart, so neither does this total.
	Unclaimed int    `json:"unclaimed"`
	Oldest    string `json:"oldest_run"`
	Newest    string `json:"newest_run"`
}

// SumPlanProof folds runs (one per plan) into one project's proof.
func SumPlanProof(runs []VerifyRun) PlanProof {
	p := PlanProof{Plans: len(runs)}
	for _, r := range runs {
		p.Rows += r.Total
		p.Claimed += r.Claimed
		p.Verified += r.Verified
		p.Failing += r.Failing
		p.Unproven += r.Unproven()
		if u := r.Total - r.Claimed; u > 0 {
			p.Unclaimed += u
		}
		if p.Oldest == "" || r.At < p.Oldest {
			p.Oldest = r.At
		}
		if r.At > p.Newest {
			p.Newest = r.At
		}
	}
	return p
}

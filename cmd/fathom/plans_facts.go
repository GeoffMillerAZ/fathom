package main

// plans_facts.go — the join every plans verb computes (planRow: an authored
// task and its verdict) as the console projections read it
// (console.PlanRowFact), and the few plan-file reads the plans verbs and the
// Proof pages share: a plan's title, its git history, and the verify
// ledger as projection rows.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// planRowFacts converts computed rows for the projections. Every row of a
// computed plan carries its verdict (Verified).
func planRowFacts(rows []planRow) []console.PlanRowFact {
	out := make([]console.PlanRowFact, 0, len(rows))
	for _, r := range rows {
		out = append(out, planTaskFact(r.Task, &r.Verdict))
	}
	return out
}

// planTaskFact is one task as a fact; v nil means no verdict yet.
func planTaskFact(t verify.Task, v *verify.TaskVerdict) console.PlanRowFact {
	f := console.PlanRowFact{
		ID: t.ID, Title: t.Title, Line: t.Line,
		Claimed: t.Claimed, Skipped: t.Skipped, InProgress: t.InProgress,
		Op:          console.IsOpTitle(t.Title),
		Disposition: t.Disposition, Reason: t.Reason, Trigger: t.Trigger,
		Symbol: t.Expect.Symbol, Package: t.Expect.Package, ReachableFrom: t.Expect.ReachableFrom,
		SkipTest: t.Expect.SkipTest, SkipReason: t.Expect.SkipReason, CertifiedBy: t.Expect.CertifiedBy,
		Oracle: t.Oracle, OraclePath: t.OraclePath, OraclePattern: t.OraclePattern, Gone: t.Gone,
	}
	if v != nil {
		f.Verified = true
		f.Status, f.Label = string(v.Status), v.Label
		for _, c := range v.Criteria {
			f.Criteria = append(f.Criteria, console.CriterionFact{Name: c.Name, Status: string(c.Status), Detail: c.Detail})
		}
	}
	return f
}

// planTitle is a plan's first "# " heading, or its file name.
func planTitle(raw []byte, path string) string {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return filepath.Base(path)
}

// planRevisionText is one commit of a plan file and its text.
type planRevisionText struct {
	Commit string
	At     time.Time
	Text   []byte
}

// maxPlanRevisions bounds the history read for a burn-up: the newest are
// kept, and the count of older ones is reported.
const maxPlanRevisions = 400

// planGitHistory reads rel's commits in root (oldest first) with the file's
// text at each, through one `git log` and one `git cat-file --batch`. note
// says why there are none: not a git repository, or never committed.
func planGitHistory(root, rel string) (revs []planRevisionText, omitted int, note string) {
	if gitToplevel(root) == "" {
		return nil, 0, "not a git repository — the plan has no history to draw"
	}
	out, err := exec.Command("git", "-C", root, "log", "--format=%H %cI", "--", rel).Output()
	if err != nil {
		return nil, 0, "git log failed: " + err.Error()
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, 0, "the plan file has never been committed"
	}
	if len(lines) > maxPlanRevisions {
		omitted = len(lines) - maxPlanRevisions
		lines = lines[:maxPlanRevisions]
	}
	var in bytes.Buffer
	for i := len(lines) - 1; i >= 0; i-- {
		sha, at, ok := strings.Cut(lines[i], " ")
		if !ok {
			continue
		}
		when, _ := time.Parse(time.RFC3339, at)
		revs = append(revs, planRevisionText{Commit: sha, At: when})
		fmt.Fprintf(&in, "%s:%s\n", sha, filepath.ToSlash(rel))
	}
	cmd := exec.Command("git", "-C", root, "cat-file", "--batch")
	cmd.Stdin = &in
	raw, err := cmd.Output()
	if err != nil {
		return nil, 0, "git cat-file failed: " + err.Error()
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	for i := range revs {
		header, err := r.ReadString('\n')
		if err != nil {
			break
		}
		fields := strings.Fields(header)
		if len(fields) < 3 || fields[1] != "blob" {
			continue // "<sha>:<path> missing": the file was deleted in that commit
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			break
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			break
		}
		_, _ = r.ReadByte() // the trailing newline
		revs[i].Text = body
	}
	return revs, omitted, ""
}

// planRevisions parses each revision's scope for the burn-up.
func planRevisions(service string, revs []planRevisionText) []console.PlanRevision {
	out := make([]console.PlanRevision, 0, len(revs))
	for _, rv := range revs {
		pr := console.PlanRevision{Commit: rv.Commit, At: rv.At}
		for _, t := range verify.ParseMarkdownPlan(service, rv.Text).Tasks {
			pr.Total++
			switch {
			case t.Skipped:
				pr.Skipped++
			case t.Claimed:
				pr.Claimed++
			}
		}
		out = append(out, pr)
	}
	return out
}

// rowRevisions is one row's checkbox across the plan's revisions.
func rowRevisions(service, id string, revs []planRevisionText) []console.RowRevision {
	out := make([]console.RowRevision, 0, len(revs))
	for _, rv := range revs {
		rr := console.RowRevision{Commit: rv.Commit, At: rv.At}
		for _, t := range verify.ParseMarkdownPlan(service, rv.Text).Tasks {
			if t.ID == id {
				rr.Present, rr.Claimed, rr.Skipped = true, t.Claimed, t.Skipped
				break
			}
		}
		out = append(out, rr)
	}
	return out
}

// ledgerRuns reads the verify ledger as projection rows; readErr names a
// ledger that exists and could not be read (absence is zero rows).
func ledgerRuns() (runs []console.LedgerRun, path, readErr string) {
	path = verifyLedgerPath()
	rows, err := readVerifyLedgerErr()
	if err != "" {
		return nil, path, err
	}
	for _, r := range rows {
		at, perr := time.Parse(time.RFC3339, r.At)
		if perr != nil {
			continue
		}
		runs = append(runs, console.LedgerRun{
			At: at, Service: r.Service, Plan: r.Plan, Commit: r.Commit,
			Total: r.Total, Claimed: r.Claimed, Verified: r.Verified, Failing: r.Failing, Origin: r.Origin,
		})
	}
	return runs, path, ""
}

// readVerifyLedgerErr is readVerifyLedger that tells an absent ledger (no
// rows, no error — a machine that has never gated) from one that exists
// and could not be read (the error, named, so a trend is never drawn from
// a ledger it could not see).
func readVerifyLedgerErr() ([]verifyLedgerRow, string) {
	path := verifyLedgerPath()
	if path == "" {
		return nil, ""
	}
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, ""
	case err != nil:
		return nil, err.Error()
	}
	var out []verifyLedgerRow
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var r verifyLedgerRow
		if err := dec.Decode(&r); err != nil {
			break
		}
		out = append(out, r)
	}
	return out, ""
}

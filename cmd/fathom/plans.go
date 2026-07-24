package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cmdPlans implements the "plans" subcommand family — SDD computed progress
// (E3, docs/plans/new-use-cases.md Phase E §E3): state derived from the
// verify engine, never asserted (ADR-018's "SDD state is derived, never
// driven"). Sub-verbs:
//
//	plans status  — per-plan rollup {proven, claimed-unproven, open, deferred-with-trigger}
//	plans next    — the ordered, evidence-derived work frontier
//	plans archive — governed artifact lifecycle (E4-1)
//	plans restore — the archive/restore round-trip (E4-1)
func cmdPlans(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: plans requires a subcommand (status, next, archive, restore)")
		return 1
	}
	switch args[0] {
	case "status":
		return cmdPlansStatus(args[1:], stdout, stderr)
	case "next":
		return cmdPlansNext(args[1:], stdout, stderr)
	case "archive":
		return cmdPlansArchive(args[1:], stdout, stderr)
	case "restore":
		return cmdPlansRestore(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown plans subcommand %q (want status, next, archive, restore)\n", args[0])
		return 1
	}
}

// planRow is one authored task combined with its verify-engine verdict —
// the join every plans sub-verb computes from (E3's "computed, never
// asserted"). Task carries document order and the {symbol, package} anchors
// (verify.TaskVerdict does not carry Expect, so both halves are needed).
type planRow struct {
	PlanFile string
	Task     verify.Task
	Verdict  verify.TaskVerdict
}

// activePlanFiles returns the sorted list of active (non-archived) plan
// files under <root>/docs/plans/ — the same non-recursive scope as the
// pre-push/CI verify gate (E4-2): filepath.Glob's "*.md" never descends into
// docs/plans/archive/, and README.md is excluded (READMEs describe, plans
// claim).
func activePlanFiles(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "docs", "plans", "*.md"))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.EqualFold(filepath.Base(m), "README.md") {
			continue
		}
		out = append(out, m)
	}
	sort.Strings(out)
	return out, nil
}

// computePlanRows parses each plan file and runs the verify engine against
// it (direct --store mode, or the daemon over HTTP when storePath is
// empty), returning one []planRow per plan, in the SAME order as planFiles
// (document order preserved within each plan — verify.Report's own Tasks
// slice is sorted by TaskID, which is why this rebuilds rows from the
// locally-parsed Plan.Tasks and only looks up each one's verdict by ID).
func computePlanRows(planFiles []string, service, storePath, urlStr string) ([][]planRow, error) {
	localPlans := make([]verify.Plan, 0, len(planFiles))
	raws := make([][]byte, 0, len(planFiles))
	for _, pf := range planFiles {
		raw, err := os.ReadFile(pf)
		if err != nil {
			return nil, fmt.Errorf("reading plan %q: %w", pf, err)
		}
		localPlans = append(localPlans, verify.ParseMarkdownPlan(service, raw))
		raws = append(raws, raw)
	}

	var reports []verify.Report
	if storePath != "" {
		reps, err := verifyAllAgainstStore(localPlans, service, storePath, "")
		if err != nil {
			return nil, err
		}
		reports = reps
	} else {
		hc := &http.Client{Timeout: 10 * time.Second}
		indexed, err := daemonServiceIndexed(hc, urlStr, service)
		if err != nil {
			return nil, fmt.Errorf("probing daemon for service %q: %w", service, err)
		}
		if !indexed {
			return nil, fmt.Errorf("service %q is not indexed on the daemon at %s — ingest it and retry", service, urlStr)
		}
		for i, lp := range localPlans {
			rep, err := verifyViaHTTP(hc, urlStr, service, "", raws[i], lp)
			if err != nil {
				return nil, err
			}
			reports = append(reports, rep)
		}
	}

	out := make([][]planRow, len(planFiles))
	for i, pf := range planFiles {
		verdictByID := make(map[string]verify.TaskVerdict, len(reports[i].Tasks))
		for _, tv := range reports[i].Tasks {
			verdictByID[tv.TaskID] = tv
		}
		rows := make([]planRow, 0, len(localPlans[i].Tasks))
		for _, t := range localPlans[i].Tasks {
			rows = append(rows, planRow{PlanFile: pf, Task: t, Verdict: verdictByID[t.ID]})
		}
		out[i] = rows
	}
	return out, nil
}

// planFlagsOrDiscover resolves the plan file list for a plans sub-verb: the
// repeatable --plan flag when given, else every active plan under CWD's
// docs/plans/ (activePlanFiles) — so `fathom plans status` with no flags
// answers "where does MY repo's SDD program stand" out of the box.
func planFlagsOrDiscover(given planFlags) ([]string, error) {
	if len(given) > 0 {
		return append([]string(nil), given...), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolving cwd: %w", err)
	}
	return activePlanFiles(cwd)
}

// ─── plans status (E3-1) ────────────────────────────────────────────────────

// planRollup is one plan's computed rollup — the four named buckets from
// docs/plans/new-use-cases.md's E3 design, plus SkippedOther so every row is
// accounted for (a [-] task without disposition:deferred still counts
// somewhere; verify never silently drops state).
type planRollup struct {
	Plan                string `json:"plan"`
	Total               int    `json:"total"`
	Proven              int    `json:"proven"`
	ClaimedUnproven     int    `json:"claimed_unproven"`
	Open                int    `json:"open"`
	DeferredWithTrigger int    `json:"deferred_with_trigger"`
	SkippedOther        int    `json:"skipped_other"`
}

// rollupPlan computes one plan's rollup from its rows. Bucket precedence
// (each row lands in exactly one bucket):
//  1. Skipped + disposition:deferred → DeferredWithTrigger
//  2. Skipped (any other reason, including no disposition) → SkippedOther
//  3. Claimed + verified Pass → Proven
//  4. Claimed + not Pass (Fail or Unknown) → ClaimedUnproven
//  5. Neither claimed nor skipped ([ ] open or [~] in-progress) → Open
func rollupPlan(planFile string, rows []planRow) planRollup {
	r := planRollup{Plan: planFile, Total: len(rows)}
	for _, row := range rows {
		t := row.Task
		switch {
		case t.Skipped && t.Disposition == "deferred":
			r.DeferredWithTrigger++
		case t.Skipped:
			r.SkippedOther++
		case t.Claimed:
			if row.Verdict.Status == verify.Pass {
				r.Proven++
			} else {
				r.ClaimedUnproven++
			}
		default:
			r.Open++
		}
	}
	return r
}

func cmdPlansStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans status", stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "plan file (repeatable; defaults to every active docs/plans/*.md)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	files, err := planFlagsOrDiscover(plans)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintln(stderr, "error: no plan files given and none found under docs/plans/")
		return 1
	}

	rowSets, err := computePlanRows(files, *service, *storePath, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rollups := make([]planRollup, len(files))
	for i, pf := range files {
		rollups[i] = rollupPlan(pf, rowSets[i])
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rollups)
		return 0
	}

	fmt.Fprintf(stdout, "%-40s %6s %7s %17s %5s %9s %14s\n",
		"plan", "total", "proven", "claimed-unproven", "open", "deferred", "other-skipped")
	for _, r := range rollups {
		fmt.Fprintf(stdout, "%-40s %6d %7d %17d %5d %9d %14d\n",
			filepath.Base(r.Plan), r.Total, r.Proven, r.ClaimedUnproven, r.Open, r.DeferredWithTrigger, r.SkippedOther)
	}
	return 0
}

// ─── plans next (E3-2) ──────────────────────────────────────────────────────

// frontierRow is one unit of the work frontier `plans next` reports: an
// open/in-progress row not yet claimed, or a claimed row the engine cannot
// prove right now — verifying Fail ("claimed-failing") OR Unknown
// ("claimed-unknown", e.g. no-contract). Claimed-Unknown rows are frontier
// work because CI's `verify --strict` (scripts/ci.sh) blocks on them: a
// frontier that omitted them would answer "no open work" on a plan CI
// rejects — the frontier and the gate must never disagree about done
// (critic finding 4).
type frontierRow struct {
	Plan    string `json:"plan"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Symbol  string `json:"symbol,omitempty"`
	Package string `json:"package,omitempty"`
	State   string `json:"state"` // open | in-progress | claimed-failing | claimed-unknown
	Detail  string `json:"detail,omitempty"`
}

// frontierForPlan returns rows in DOCUMENT order (the order they were
// authored in the plan file) — never TaskID order, which an author-assigned
// `id:` need not sort lexically the same way (E3-2's ordering contract).
func frontierForPlan(rows []planRow) []frontierRow {
	var out []frontierRow
	for _, row := range rows {
		t, v := row.Task, row.Verdict
		if t.Skipped {
			continue
		}
		if !t.Claimed {
			state := "open"
			if t.InProgress {
				state = "in-progress"
			}
			out = append(out, frontierRow{
				Plan: row.PlanFile, ID: t.ID, Title: t.Title,
				Symbol: t.Expect.Symbol, Package: t.Expect.Package, State: state,
			})
			continue
		}
		switch v.Status {
		case verify.Fail:
			detail := v.Label
			for _, c := range v.Criteria {
				if c.Status == verify.Fail {
					detail = fmt.Sprintf("%s: %s", c.Name, c.Detail)
					break
				}
			}
			out = append(out, frontierRow{
				Plan: row.PlanFile, ID: t.ID, Title: t.Title,
				Symbol: t.Expect.Symbol, Package: t.Expect.Package,
				State: "claimed-failing", Detail: detail,
			})
		case verify.Unknown:
			out = append(out, frontierRow{
				Plan: row.PlanFile, ID: t.ID, Title: t.Title,
				Symbol: t.Expect.Symbol, Package: t.Expect.Package,
				State: "claimed-unknown", Detail: v.Label,
			})
		}
	}
	return out
}

func cmdPlansNext(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans next", stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "plan file (repeatable; defaults to every active docs/plans/*.md)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human list")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	files, err := planFlagsOrDiscover(plans)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintln(stderr, "error: no plan files given and none found under docs/plans/")
		return 1
	}

	rowSets, err := computePlanRows(files, *service, *storePath, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var frontier []frontierRow
	for _, rows := range rowSets {
		frontier = append(frontier, frontierForPlan(rows)...)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(frontier)
		return 0
	}

	if len(frontier) == 0 {
		fmt.Fprintln(stdout, "no open work — every row is claimed-and-verified or skipped")
		return 0
	}
	for _, r := range frontier {
		anchor := ""
		if r.Symbol != "" {
			anchor = fmt.Sprintf(" {symbol: %s", r.Symbol)
			if r.Package != "" {
				anchor += fmt.Sprintf(", package: %s", r.Package)
			}
			anchor += "}"
		}
		line := fmt.Sprintf("%s [%s] (%s) %s%s", r.Plan, r.ID, r.State, r.Title, anchor)
		if r.Detail != "" {
			line += " — " + r.Detail
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

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
	"sync"
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
	case "context":
		return cmdPlansContext(args[1:], stdout, stderr)
	case "lease":
		return cmdPlansLease(args[1:], stdout, stderr)
	case "leases":
		return cmdPlansLeases(args[1:], stdout, stderr)
	case "archive":
		return cmdPlansArchive(args[1:], stdout, stderr)
	case "restore":
		return cmdPlansRestore(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown plans subcommand %q (want status, next, context, lease, leases, archive, restore)\n", args[0])
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

	// Lexical oracles (ADR-030) read the working tree the plans came from.
	files := verify.DirFiles(verifyRepoRoot())

	var reports []verify.Report
	if storePath != "" {
		reps, err := verifyAllAgainstStore(localPlans, service, storePath, "", files, "", false)
		if err != nil {
			return nil, err
		}
		reports = reps
	} else {
		// 60s, not the old 10s: concurrent verifies contend on the daemon
		// (and the FIRST call may pay an auto-refresh), so the per-call
		// ceiling must absorb queueing that the serial loop never saw —
		// with 10s the fan-out timed out where slow-but-serial survived.
		hc := &http.Client{Timeout: 60 * time.Second}
		indexed, err := daemonServiceIndexed(hc, urlStr, service)
		if err != nil {
			return nil, fmt.Errorf("probing daemon for service %q: %w", service, err)
		}
		if !indexed {
			return nil, fmt.Errorf("service %q is not indexed on the daemon at %s — ingest it and retry", service, urlStr)
		}
		// O29 (docs/dogfood-observations.md): the per-plan verifies are
		// independent daemon calls; running them serially made this verb
		// scale at seconds x plans (2m35s measured over this fleet's ~40
		// active plans). Bounded fan-out; order rebuilt by index below.
		reports = make([]verify.Report, len(localPlans))
		verifyErrs := make([]error, len(localPlans))
		sem := make(chan struct{}, 4)
		var wg sync.WaitGroup
		for i := range localPlans {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				reports[i], verifyErrs[i] = verifyViaHTTP(hc, urlStr, service, "", raws[i], localPlans[i])
				if verifyErrs[i] == nil {
					verify.OverlayLocal(&reports[i], localPlans[i], files)
				}
			}(i)
		}
		wg.Wait()
		for _, e := range verifyErrs {
			if e != nil {
				return nil, e
			}
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

// discoverPlansForService resolves plan files for a --service (FR-1,
// docs/plans/exploration-ledger.md): explicit --plan flags win; otherwise
// discovery runs in the SERVICE's registered corpus path — never blindly in
// the cwd, which silently verified one repo's plans against another repo's
// index (the resume_as_code field report, 2026-08-08). The note names the
// directory actually scanned, so a wrong resolution is loud. Discovery
// scope stays activePlanFiles' (non-recursive, README excluded — READMEs
// describe, plans claim).
func discoverPlansForService(explicit planFlags, service, baseURL string) (files []string, note string, err error) {
	if len(explicit) > 0 {
		return explicit, "", nil
	}
	root := ""
	hc := &http.Client{Timeout: 4 * time.Second}
	if svcs, ferr := fetchServices(hc, baseURL); ferr == nil {
		if info, ok := svcs[strings.ToLower(service)]; ok {
			root = info.path
		}
	}
	if root == "" {
		if cwd, cerr := os.Getwd(); cerr == nil {
			root = cwd
			note = fmt.Sprintf("plans: service %q has no registered path reachable from here — scanning the current directory's docs/plans", service)
		}
	}
	matches, gerr := activePlanFiles(root)
	if gerr != nil {
		return nil, "", gerr
	}
	if note == "" {
		note = fmt.Sprintf("plans: scanning %s", filepath.Join(root, "docs", "plans"))
	}
	return matches, note, nil
}

// ─── plans status (E3-1) ────────────────────────────────────────────────────

// planRollup is one plan's computed rollup — the four named buckets from
// docs/plans/new-use-cases.md's E3 design, plus SkippedOther so every row is
// accounted for (a [-] task without disposition:deferred still counts
// somewhere; verify never silently drops state).
type planRollup struct {
	Plan            string `json:"plan"`
	Total           int    `json:"total"`
	Proven          int    `json:"proven"`
	ClaimedUnproven int    `json:"claimed_unproven"`
	// MeasuredStale (PC-5): claimed rows the daemon could not prove while
	// the daemon's binary was stale — counted here, never as unproven.
	MeasuredStale       int `json:"measured_stale,omitempty"`
	Open                int `json:"open"`
	DeferredWithTrigger int `json:"deferred_with_trigger"`
	SkippedOther        int `json:"skipped_other"`
}

// rollupPlan computes one plan's rollup from its rows. Bucket precedence
// (each row lands in exactly one bucket):
//  1. Skipped + disposition:deferred → DeferredWithTrigger
//  2. Skipped (any other reason, including no disposition) → SkippedOther
//  3. Claimed + verified Pass → Proven
//  4. Claimed + not Pass (Fail or Unknown) → ClaimedUnproven
//  5. Neither claimed nor skipped ([ ] open or [~] in-progress) → Open
func rollupPlan(planFile string, rows []planRow) planRollup {
	return rollupPlanStale(planFile, rows, false)
}

// rollupPlanStale is rollupPlan with the PC-5 switch: when the daemon that
// answered was measured-stale, a claimed row it did not prove is a
// coverage gap (MeasuredStale), not ClaimedUnproven.
func rollupPlanStale(planFile string, rows []planRow, daemonStale bool) planRollup {
	r := planRollup{Plan: planFile, Total: len(rows)}
	for _, row := range rows {
		t := row.Task
		switch {
		case t.Skipped && t.Disposition == "deferred":
			r.DeferredWithTrigger++
		case t.Skipped:
			r.SkippedOther++
		case t.Claimed:
			switch {
			case row.Verdict.Status == verify.Pass:
				r.Proven++
			case daemonStale:
				r.MeasuredStale++
			default:
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
	files, scanNote, err := discoverPlansForService(plans, *service, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if scanNote != "" {
		fmt.Fprintln(stderr, scanNote)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(stderr, "error: no plan files given and none found under docs/plans/ (searched service %q's registered repo, else the cwd)\n", *service)
		return 1
	}

	rowSets, err := computePlanRows(files, *service, *storePath, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	daemonStale, staleWhy := false, ""
	if *storePath == "" {
		daemonStale, staleWhy = liveDaemonStale(&http.Client{Timeout: 5 * time.Second}, *url, verifyRepoRoot())
	}
	rollups := make([]planRollup, len(files))
	for i, pf := range files {
		rollups[i] = rollupPlanStale(pf, rowSets[i], daemonStale)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rollups)
		return 0
	}

	fmt.Fprintf(stdout, "%-40s %6s %7s %17s %14s %5s %9s %14s\n",
		"plan", "total", "proven", "claimed-unproven", "measured-stale", "open", "deferred", "other-skipped")
	for _, r := range rollups {
		fmt.Fprintf(stdout, "%-40s %6d %7d %17d %14d %5d %9d %14d\n",
			filepath.Base(r.Plan), r.Total, r.Proven, r.ClaimedUnproven, r.MeasuredStale, r.Open, r.DeferredWithTrigger, r.SkippedOther)
	}
	if daemonStale {
		fmt.Fprintf(stdout, "\nmeasured-stale: %s — claimed rows the daemon did not prove are coverage gaps, not failures (ADR-027); task deploy or restart the daemon, then re-run\n", staleWhy)
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

	// Eligible/BlockedOn (WF-4, vision 24 §3): pure derived eligibility —
	// a function of (plan, evidence) computed at query time, nothing
	// stored. A row with no needs: is eligible by definition; a row whose
	// every needs-target is claimed-and-verified or skipped-with-
	// disposition is eligible too.
	Eligible  bool         `json:"eligible"`
	BlockedOn []blockedDep `json:"blocked_on,omitempty"`

	// Leased/LeaseHolder/LeaseExpires (WF-6, vision 24 §4): decoration
	// from a fresh (non-released, unexpired) lease on this row, derived
	// from .fathom/leases.jsonl at render time — never stored on the row,
	// and an expired or released lease makes the row reappear undecorated.
	Leased       bool   `json:"leased,omitempty"`
	LeaseHolder  string `json:"lease_holder,omitempty"`
	LeaseExpires string `json:"lease_expires,omitempty"`
}

// blockedDep is one unmet needs-target on a frontier row: its id and its
// current state (mirrors frontierRow.State's vocabulary, plus "missing"
// for a needs: id that resolves to no row in the plan).
type blockedDep struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// rowByID indexes rows by their id — the lookup table needs: targets
// resolve against, scoped to the SAME plan's rows (needs: ids are
// authored per-plan, mirroring LintPlan's own unknown-id/cycle scope).
func rowByID(rows []planRow) map[string]planRow {
	m := make(map[string]planRow, len(rows))
	for _, r := range rows {
		m[r.Task.ID] = r
	}
	return m
}

// depSatisfied reports whether row counts as a satisfied needs-target:
// claimed-and-verified, or skipped-with-disposition (WF-4's exact
// eligibility predicate).
func depSatisfied(row planRow) bool {
	t := row.Task
	if t.Skipped {
		return t.Disposition != ""
	}
	return t.Claimed && row.Verdict.Status == verify.Pass
}

// depState names an unmet needs-target's current state for blocked_on,
// reusing frontierRow's own state vocabulary.
func depState(row planRow) string {
	t := row.Task
	switch {
	case t.Skipped:
		return "skipped-other" // skipped with no disposition — still unmet
	case t.Claimed:
		if row.Verdict.Status == verify.Fail {
			return "claimed-failing"
		}
		return "claimed-unknown"
	case t.InProgress:
		return "in-progress"
	default:
		return "open"
	}
}

// rowEligibility computes eligible + blocked_on for one task's needs:
// list against byID (WF-4, vision 24 §3). A cross-plan target
// ("<sibling>.md#<id>", PC-10) is read from planDir: its AUTHORED state
// decides — claimed or skipped-with-disposition satisfies, anything else
// blocks — because the sibling's verdict is not recomputed here; the
// blocked state says which plan it lives in.
func rowEligibility(t verify.Task, byID map[string]planRow, planDir string) (bool, []blockedDep) {
	if len(t.Needs) == 0 {
		return true, nil
	}
	var blocked []blockedDep
	for _, id := range t.Needs {
		if _, cross := verify.ParseCrossPlanNeed(id); cross {
			sib, err := verify.ResolveCrossPlanNeed(planDir, id)
			switch {
			case err != nil:
				blocked = append(blocked, blockedDep{ID: id, State: "missing"})
			case sib.Skipped && sib.Disposition != "", sib.Claimed:
				// satisfied by authored state (sibling verdict not recomputed)
			default:
				blocked = append(blocked, blockedDep{ID: id, State: depState(planRow{Task: sib}) + " (sibling plan)"})
			}
			continue
		}
		dep, ok := byID[id]
		if !ok {
			blocked = append(blocked, blockedDep{ID: id, State: "missing"})
			continue
		}
		if !depSatisfied(dep) {
			blocked = append(blocked, blockedDep{ID: id, State: depState(dep)})
		}
	}
	return len(blocked) == 0, blocked
}

// freshLeasesByRow indexes leases' currently-fresh events by
// filepath.Base(plan)+"|"+row_id — basename-keyed because a lease's --plan
// string and a frontier row's discovered PlanFile need not share the same
// relative/absolute form; the filename is the stable join key (docs/plans/
// is a flat, non-recursive scope, so basenames don't collide).
func freshLeasesByRow(leases []leaseRow, now time.Time) map[string][]leaseRow {
	out := map[string][]leaseRow{}
	for _, l := range activeLeases(leases, now) {
		key := filepath.Base(l.Plan) + "|" + l.RowID
		out[key] = append(out[key], l)
	}
	return out
}

// pickDisplayLease chooses which fresh lease to surface when more than
// one holder is simultaneously fresh on a row (an overlap — WF-7 is what
// counts those; decoration just needs one deterministic answer): the
// earliest-filed claim.
func pickDisplayLease(fresh []leaseRow) leaseRow {
	best := fresh[0]
	for _, l := range fresh[1:] {
		if l.Filed.Before(best.Filed) {
			best = l
		}
	}
	return best
}

// frontierForPlan returns rows in DOCUMENT order (the order they were
// authored in the plan file) — never TaskID order, which an author-assigned
// `id:` need not sort lexically the same way (E3-2's ordering contract).
// leases decorates rows holding a fresh lease as of now (WF-6); pass nil
// leases for no decoration.
func frontierForPlan(rows []planRow, leases []leaseRow, now time.Time) []frontierRow {
	byID := rowByID(rows)
	freshByRow := freshLeasesByRow(leases, now)

	var out []frontierRow
	for _, row := range rows {
		t, v := row.Task, row.Verdict
		if t.Skipped {
			continue
		}
		var fr frontierRow
		if !t.Claimed {
			state := "open"
			if t.InProgress {
				state = "in-progress"
			}
			fr = frontierRow{
				Plan: row.PlanFile, ID: t.ID, Title: t.Title,
				Symbol: t.Expect.Symbol, Package: t.Expect.Package, State: state,
			}
		} else {
			switch v.Status {
			case verify.Fail:
				detail := v.Label
				for _, c := range v.Criteria {
					if c.Status == verify.Fail {
						detail = fmt.Sprintf("%s: %s", c.Name, c.Detail)
						break
					}
				}
				fr = frontierRow{
					Plan: row.PlanFile, ID: t.ID, Title: t.Title,
					Symbol: t.Expect.Symbol, Package: t.Expect.Package,
					State: "claimed-failing", Detail: detail,
				}
			case verify.Unknown:
				fr = frontierRow{
					Plan: row.PlanFile, ID: t.ID, Title: t.Title,
					Symbol: t.Expect.Symbol, Package: t.Expect.Package,
					State: "claimed-unknown", Detail: v.Label,
				}
			default:
				continue // Pass + Claimed + not Skipped never reaches the frontier
			}
		}

		fr.Eligible, fr.BlockedOn = rowEligibility(t, byID, filepath.Dir(row.PlanFile))

		if fresh := freshByRow[filepath.Base(row.PlanFile)+"|"+t.ID]; len(fresh) > 0 {
			lease := pickDisplayLease(fresh)
			fr.Leased = true
			fr.LeaseHolder = lease.Holder
			fr.LeaseExpires = lease.Expires.UTC().Format(time.RFC3339)
		}

		out = append(out, fr)
	}
	return out
}

// formatRemaining renders a duration the way `plans next`'s human output
// names a lease's remaining time ("43m", "1h5m"). Negative/zero renders
// "0m" — frontierForPlan never surfaces an expired lease, so this only
// ever formats a genuinely-fresh remainder, but stays defined for 0.
func formatRemaining(d time.Duration) string {
	d = d.Round(time.Minute)
	if d <= 0 {
		return "0m"
	}
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func cmdPlansNext(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans next", stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "plan file (repeatable; defaults to every active docs/plans/*.md)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	root := fs.String("root", ".", "project root — .fathom/leases.jsonl lives under here")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human list")
	eligibleOnly := fs.Bool("eligible-only", false, "hide rows blocked on an unmet needs: target (WF-4)")
	free := fs.Bool("free", false, "hide rows holding a fresh lease (WF-6)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	files, scanNote, err := discoverPlansForService(plans, *service, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if scanNote != "" {
		fmt.Fprintln(stderr, scanNote)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(stderr, "error: no plan files given and none found under docs/plans/ (searched service %q's registered repo, else the cwd)\n", *service)
		return 1
	}

	rowSets, err := computePlanRows(files, *service, *storePath, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// Lease decoration is best-effort and never fatal to the frontier: a
	// missing/unreadable leases.jsonl just means no rows decorate.
	leases, _ := readLeases(*root)
	now := nowFunc()

	var frontier []frontierRow
	for _, rows := range rowSets {
		frontier = append(frontier, frontierForPlan(rows, leases, now)...)
	}
	if *eligibleOnly {
		frontier = filterFrontier(frontier, func(r frontierRow) bool { return r.Eligible })
	}
	if *free {
		frontier = filterFrontier(frontier, func(r frontierRow) bool { return !r.Leased })
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
		if !r.Eligible && len(r.BlockedOn) > 0 {
			ids := make([]string, len(r.BlockedOn))
			for i, b := range r.BlockedOn {
				ids[i] = b.ID
			}
			line += fmt.Sprintf(" (blocked on %s)", strings.Join(ids, ", "))
		}
		if r.Leased {
			remaining := "0m"
			if exp, err := time.Parse(time.RFC3339, r.LeaseExpires); err == nil {
				remaining = formatRemaining(exp.Sub(now))
			}
			line += fmt.Sprintf(" — leased (%s, %s left)", r.LeaseHolder, remaining)
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// filterFrontier returns the rows of frontier for which keep reports true,
// preserving order.
func filterFrontier(frontier []frontierRow, keep func(frontierRow) bool) []frontierRow {
	out := make([]frontierRow, 0, len(frontier))
	for _, r := range frontier {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

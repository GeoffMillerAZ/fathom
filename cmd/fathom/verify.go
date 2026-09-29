package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/specdialect"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// mutationReportEnvVar is the env-var fallback for `fathom verify
// --mutation` (VB-34): a CI pipeline that already sets it once for every
// gate invocation needn't repeat the flag on every `fathom verify` call.
// The flag, when given, always wins.
const mutationReportEnvVar = "FATHOM_MUTATION_REPORT"

// planFlags is a repeatable --plan flag value.
type planFlags []string

func (p *planFlags) String() string     { return strings.Join(*p, ",") }
func (p *planFlags) Set(v string) error { *p = append(*p, v); return nil }

// cmdVerify implements the "verify" subcommand: runs the "done means proven" gate
// against one or more Markdown plan files. It queries fathom (via the daemon in
// HTTP mode, or a local store with --store) and computes the gate locally, so the
// daemon binary's staleness cannot affect gate correctness.
//
// Exit codes (per §5.3 of docs/design/coding-agent-excellence.md):
//
//	0 — gate pass
//	1 — gate fail (a claimed [x] task failed verification)
//	2 — operational error (unreadable plan, daemon unreachable, malformed response)
func cmdVerify(args []string, stdout, stderr io.Writer) int {
	if rpcOrigin == "" {
		rpcOrigin = "verify-cli"
	}
	fs := newFlagSet("verify", stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "plan file (repeatable; ≥1 required): a Markdown checklist in plan grammar v2, or a foreign spec file read in place (Spec Kit specs/<feature>/tasks.md, OpenSpec openspec/changes/<id>/tasks.md, Kiro .kiro/specs/<name>/tasks.md, Task Master .taskmaster/tasks/tasks.json, or a GitHub issue/PR task list exported to a file — pass --dialect gh-tasklist, since it carries no path fathom can detect)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	strict := fs.Bool("strict", false, "treat claimed+unknown tasks as gate failures (recommended for CI)")
	requireAll := fs.Bool("require-all", false, "wave gate: additionally fail on open/in-progress tasks (ADR-018 §2)")
	concern := fs.String("concern", "", "optional concern for contradiction scoping")
	jsonOut := fs.Bool("json", false, "emit JSON report + gate result instead of human text")
	format := fs.String("format", "", "output format: text (default) | json | sarif | check-run — SARIF 2.1.0 of the refusals, one rule per criterion, located at the plan line (PC-21); check-run is a GitHub check-run JSON body (name, conclusion, output.summary/annotations, capped at 50 annotations) written here, never posted (GL-21)")
	quietPass := fs.Bool("quiet-pass", false, "suppress output when gate passes (exit 0)")
	quiet := fs.Bool("quiet", false, "print only failing (and, with --strict, unknown) task rows plus the summary; composes with --quiet-pass")
	mutationPath := fs.String("mutation", "", "VB-34: mutation-testing report (go-mutesting log output, or fathom's own JSON shape); a claimed row whose symbol has mutation score 0 gets a covered-not-asserted note (falls back to $"+mutationReportEnvVar+"; direct/--store mode only)")
	strictMutation := fs.Bool("strict-mutation", false, "gate-fail a covered-not-asserted finding instead of noting it as advisory")
	rowID := fs.String("row", "", "narrow the report and the gate to one row id (the console's row evidence renders `verify --row ID --json`); every plan is still verified and ledgered in full")
	dialectID := fs.String("dialect", "", "spec dialect every --plan file is read with (fathom-v2 | speckit | openspec | kiro | taskmaster | gh-tasklist); default: detected per file by its path and content shape (ADR-049) — gh-tasklist is never auto-detected and always needs this flag")
	converge := fs.Bool("converge", false, "also print each plan's converge report (ADR-049, spec-interop design §5): claimed-unproven tasks with the failing criterion, unanchored tasks, and code changed on the spec's branch range that no task anchors (the last needs --store); exit codes are the gate's, unchanged")
	convergeBase := fs.String("base", "", "--converge: the branch range's base ref (default: the commit that added the spec file)")
	writeFixups := fs.Bool("write-fixups", false, "--converge: append a fix-up task per claimed-unproven and unclaimed item inside the spec's <!-- fathom:begin converge --> block, in the spec's own checkbox shape; text outside the markers never changes (law 15)")
	since := fs.String("since", "", "git ref to compare each --plan file against (ADR-044 §1.4, I45): a claimed row's skip/disposition/retires/surface_change/raised-max gained since <ref> is reported self-declared and never excuses a loss; rows present at <ref> and missing now are reported deleted. Applies only inside a git work tree; a ref the plan file did not exist at counts as every current row being self-declared.")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mutationPath == "" {
		*mutationPath = os.Getenv(mutationReportEnvVar)
	}
	if *jsonOut && *format == "" {
		*format = "json"
	}
	switch *format {
	case "", "text", "json", "sarif", "check-run":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want text, json, sarif or check-run\n", *format)
		return 2
	}
	if (*writeFixups || *convergeBase != "") && !*converge {
		fmt.Fprintln(stderr, "error: --write-fixups and --base apply only with --converge")
		return 2
	}
	if len(plans) == 0 {
		fmt.Fprintln(stderr, "error: at least one --plan file is required")
		fs.Usage()
		return 2
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 2
	}

	// root is the tree every local, git-aware check reads against: the
	// lexical oracles (ADR-030) AND the self-skip comparison (O56) — both
	// read the SAME working tree the plan file itself came from.
	root := verifyRepoRoot()

	// Read and parse all plan files locally — claimed/skipped state is always
	// derived from the local file, never from the daemon.
	localPlans := make([]verify.Plan, 0, len(plans))
	planBytes := make([][]byte, 0, len(plans))
	// foreign marks a plan read through a foreign dialect (ADR-049): its
	// verdict lines point into the author's own file, and the daemon is
	// sent the plan synthesized into the native grammar, never the bytes.
	foreign := make([]bool, 0, len(plans))
	dialectIDs := make([]string, 0, len(plans))
	dialects := specdialect.Default()
	for _, pf := range plans {
		raw, err := os.ReadFile(pf)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading plan %q: %v\n", pf, err)
			return 2
		}
		d, err := dialectForPlan(dialects, pf, raw, *dialectID)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		lp, err := parsePlanWith(d, *service, pf, raw)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		parse := planParserFor(d, pf)
		// O56: mark any claimed row whose skip_test/skip was added in the
		// working tree or in the commit that flipped its box — self-skipped,
		// never proven by the agent's own hand. Inapplicable (never flagged)
		// outside a git repo, or when pf resolves outside root.
		if absPF, aerr := filepath.Abs(pf); aerr == nil {
			if rel, ok := relativeToRoot(root, absPF); ok {
				lp = verify.AnnotateSelfSkip(lp, priorSelfSkipPlanWith(root, rel, *service, raw, parse))
				// ADR-044 §1.4 (I45, CV-14): --since widens the same
				// question from "the one edit/commit that claimed this
				// row" to "anywhere across base..head" — a declaration
				// gained at ANY point in the range excuses nothing.
				if *since != "" {
					lp = verify.AnnotateSelfDeclared(lp, sinceRefPlanWith(root, rel, *since, *service, parse))
				}
			}
		}
		for _, w := range verify.LintPlan(lp) {
			fmt.Fprintf(stderr, "lint: %s: %s\n", pf, w)
		}
		// PC-10: a needs: target in a sibling plan is resolved here (LintPlan
		// is pure and checks only the shape); a missing plan or row is a
		// lint line naming which half failed.
		for _, t := range lp.Tasks {
			for _, dep := range t.Needs {
				if _, ok := verify.ParseCrossPlanNeed(dep); !ok {
					continue
				}
				if _, err := verify.ResolveCrossPlanNeed(filepath.Dir(pf), dep); err != nil {
					fmt.Fprintf(stderr, "lint: %s: %s: needs %s — %v\n", pf, t.ID, dep, err)
				}
			}
		}
		localPlans = append(localPlans, lp)
		planBytes = append(planBytes, raw)
		foreign = append(foreign, d.ID() != specdialect.NativeID)
		dialectIDs = append(dialectIDs, d.ID())
	}

	// The working tree lexical oracles (ADR-030) read: the tree the plan
	// files themselves were read from, in both modes.
	files := verify.DirFiles(root)

	// Gather verdicts — either in-process (--store) or via the daemon (HTTP).
	var reports []verify.Report
	var prov verifyProvenance
	if *storePath != "" {
		prov = verifyLedgerProvenance(nil, "", *storePath, *service)
		reps, err := verifyAllAgainstStore(localPlans, *service, *storePath, *concern, files, *mutationPath, *strictMutation)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		reports = reps
	} else {
		if *mutationPath != "" {
			fmt.Fprintln(stderr, "error: --mutation requires --store (direct mode) — HTTP/daemon mode has no local symbol join for it yet")
			return 2
		}
		// verify_plan measured 10-15 s on the fleet daemon on 2026-09-13
		// (4.5 s on 2026-09-09; O33). A client deadline under that turns
		// the pre-push verify gate into exit 2 — operational, fail-open —
		// which proves nothing while looking green. The budget is the
		// slowest measured call with headroom, not a guess.
		hc := &http.Client{Timeout: 60 * time.Second}
		// Probe first: a service with no symbols in the index would make every
		// claim read as "not found" — evidence-shaped output for an operational
		// problem. That must be exit 2 (fail-open), never a blocking gate fail
		// (§5.3: a gate that blocks pushes for infra reasons gets uninstalled).
		indexed, err := daemonServiceIndexed(hc, *url, *service)
		if err != nil {
			fmt.Fprintf(stderr, "error: probing daemon for service %q: %v\n", *service, err)
			return 2
		}
		if !indexed {
			fmt.Fprintf(stderr, "error: service %q is not indexed on the daemon at %s — claims cannot be verified (operational, fail-open). Ingest it (MCP ingest tool or scripts/reingest-fleet.sh) and retry.\n", *service, *url)
			return 2
		}
		// PC-2: which daemon binary is about to answer, and at which
		// store watermark — recorded on every ledger row and spoken once
		// (below) when it is not this binary.
		prov = verifyLedgerProvenance(hc, *url, "", *service)
		for i, lp := range localPlans {
			// A foreign plan reaches the daemon synthesized into the
			// native grammar (the daemon parses plan_markdown natively);
			// the overlays below restore its foreign lines from lp.
			wireMD, wirePlan := planBytes[i], lp
			if foreign[i] {
				wireMD = specdialect.Synthesize(lp)
				wirePlan = verify.ParseMarkdownPlan(*service, wireMD)
			}
			rep, err := verifyViaHTTP(hc, *url, *service, *concern, wireMD, wirePlan)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			// The daemon has the index but not this working tree: its
			// lexical verdicts are Unknown-with-reason, replaced here from
			// the tree the plan came from; every verdict gains its line.
			verify.OverlayLocal(&rep, lp, files)
			// O56: the daemon parsed plan_markdown with no git access, so a
			// self-skipped row it verdicted from lp's skip: reason alone may
			// read Pass — force it back to Fail/self-skipped from the LOCAL
			// AnnotateSelfSkip result already folded into lp above.
			verify.OverlaySelfSkip(&rep, lp)
			// CV-14: same reasoning, widened to --since's whole range —
			// the daemon never saw lp's AnnotateSelfDeclared result either.
			verify.OverlaySelfDeclared(&rep, lp)
			reports = append(reports, rep)
		}
	}

	// SD-1 (docs/plans/custodian.md): every verify run leaves a ledger
	// row per plan — the history that lets a trend be a measurement
	// instead of a guess, and the signal the flywheel's verify-gate loop
	// reads (O30: lefthook pre-push runs were invisible to it). Best
	// effort, never part of the gate.
	appendVerifyLedger(plans, *service, reports, prov)
	// PC-3: one line, stderr, only when the daemon's binary is not this one
	// (or could not be learned) — never a comparison to the cwd's HEAD.
	if notice := provenanceNotice(prov); notice != "" {
		fmt.Fprintln(stderr, notice)
	}

	// --converge (ADR-049, design §5): one report per plan over the full
	// verdicts (before any --row narrowing); --write-fixups then writes
	// only inside each spec's converge block.
	var converged []specdialect.ConvergeReport
	if *converge {
		for i, pf := range plans {
			r := buildConverge(pf, dialectIDs[i], localPlans[i], reports[i], *service, *storePath, *convergeBase)
			converged = append(converged, r)
			if *writeFixups {
				if err := writeFixupsFile(pf, dialectIDs[i], planBytes[i], localPlans[i], r); err != nil {
					fmt.Fprintf(stderr, "error: --write-fixups: %v\n", err)
					return 2
				}
				fmt.Fprintf(stderr, "converge: %s: %d fix-up task(s) in its converge block\n", pf, len(r.ClaimedUnproven)+len(r.Unclaimed))
			}
		}
	}

	// --row narrows what is reported and gated to one row (the ledger
	// above already recorded every plan in full).
	if *rowID != "" {
		narrowed, found := narrowReportsToRow(reports, *rowID)
		if !found {
			fmt.Fprintf(stderr, "error: no row %q in %s\n", *rowID, strings.Join(plans, ", "))
			return 2
		}
		reports = narrowed
	}

	// Compute gate locally over all reports.
	allFail := verifyGateFailures(reports, *requireAll, *strict)
	gatePass := len(allFail) == 0

	if *format == "sarif" {
		_, _ = stdout.Write(verify.RenderSARIF("fathom verify", verify.GateSARIF(plans, reports, *strict)))
	} else if *format == "check-run" {
		// GL-21 (design doc §8 "Check payload"): the same refusals GateSARIF
		// renders, as a GitHub check-run body instead — fathom writes it
		// here and never posts it (the caller's CI, or a forge app, does).
		total, claimed, passed, failed := 0, 0, 0, 0
		for _, rep := range reports {
			for _, tv := range rep.Tasks {
				total++
				if tv.Claimed && !tv.Skipped {
					claimed++
					switch {
					case tv.Status == verify.Pass:
						passed++
					case tv.Status == verify.Fail:
						failed++
					case *strict && tv.Status == verify.Unknown:
						failed++
					}
				}
			}
		}
		conclusion := "success"
		if !gatePass {
			conclusion = "failure"
		}
		summary := fmt.Sprintf("%d tasks total; %d claimed: %d verified, %d failing", total, claimed, passed, failed)
		_, _ = stdout.Write(verify.RenderCheckRun("fathom verify", conclusion, summary, verify.GateCheckRunAnnotations(plans, reports, *strict)))
	} else if *format == "json" {
		out := verifyJSONDoc(reports, allFail)
		if *converge {
			out["converge"] = converged
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else if !gatePass || !*quietPass {
		for ri, rep := range reports {
			// A foreign plan's rows name their line in the author's file
			// (ADR-049); --row narrowing drops the index link, so only an
			// un-narrowed run can say which file a report came from.
			where := ""
			if *rowID == "" && ri < len(foreign) && foreign[ri] {
				where = plans[ri]
			}
			for _, tv := range rep.Tasks {
				icon := verifyIcon(tv)
				// --quiet: print only failing rows (✗), plus unknown rows (?)
				// when --strict is also set — a gate run over many plans reads
				// as its exceptions, not a wall of checkmarks (VS-8 / O19 / G14).
				if *quiet && icon != "✗" && !(*strict && icon == "?") {
					continue
				}
				if where != "" && tv.Line > 0 {
					fmt.Fprintf(stdout, "%s [%s] %s (%s:%d)\n", icon, tv.TaskID, tv.Title, where, tv.Line)
				} else {
					fmt.Fprintf(stdout, "%s [%s] %s\n", icon, tv.TaskID, tv.Title)
				}
				// CV-14 (ADR-044 §1.4, I45): --since's per-row rendering —
				// printed for every row that carries one, claimed or not,
				// failing or not, since a self-declared retirement/ratchet
				// is information for the reviewer regardless of the row's
				// own verdict.
				for _, c := range tv.Criteria {
					if c.Name == "self_declared" {
						fmt.Fprintf(stdout, "    %s\n", c.Detail)
					}
				}
				// Only expand failing criteria for claimed tasks that gate.
				if tv.Claimed && !tv.Skipped &&
					(tv.Status == verify.Fail || (*strict && tv.Status == verify.Unknown)) {
					for _, c := range tv.Criteria {
						if c.Status == verify.Fail || (*strict && c.Status == verify.Unknown) {
							fmt.Fprintf(stdout, "    %s: %s\n", c.Name, c.Detail)
						}
					}
				}
			}
		}
		// Summary line.
		total, claimed, passed, failed := 0, 0, 0, 0
		for _, rep := range reports {
			for _, tv := range rep.Tasks {
				total++
				if tv.Claimed && !tv.Skipped {
					claimed++
					switch {
					case tv.Status == verify.Pass:
						passed++
					case tv.Status == verify.Fail:
						failed++
					case *strict && tv.Status == verify.Unknown:
						failed++
					}
				}
			}
		}
		fmt.Fprintf(stdout, "\n%d tasks total; %d claimed: %d verified, %d failing\n",
			total, claimed, passed, failed)
		// CV-14: rows present at --since's ref and missing from this plan
		// file now — I45's "rows deleted since base." One line per plan
		// file that lost rows; a plan with none prints nothing.
		if *since != "" {
			for i, lp := range localPlans {
				if len(lp.DeletedRows) == 0 {
					continue
				}
				fmt.Fprintf(stdout, "deleted since %s (%s): %s\n", *since, plans[i], strings.Join(lp.DeletedRows, ", "))
			}
		}
	}
	if *converge && *format != "json" && *format != "sarif" && *format != "check-run" {
		for _, r := range converged {
			specdialect.RenderConverge(stdout, r)
		}
	}

	if !gatePass {
		return 1
	}
	return 0
}

// verifyAllAgainstStore opens the store once and runs VerifyPlan for each plan
// in sequence, sharing the call graph and contradiction index. mutationPath,
// when non-empty (VB-34), is parsed and joined against nodes once and
// overlaid onto every report via verify.OverlayMutation; strictMutation is
// passed straight through.
func verifyAllAgainstStore(plans []verify.Plan, service, storePath, concern string, files verify.FileEvidence, mutationPath string, strictMutation bool) ([]verify.Report, error) {
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		return nil, fmt.Errorf("opening store %q: %w", storePath, err)
	}
	defer c.Close()

	ctx := context.Background()

	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("listing symbols for %q: %w", service, err)
	}
	// An empty index means "cannot verify", not "all claims false" — surface
	// it as an operational error (exit 2) rather than gate-failing every claim.
	if len(nodes) == 0 {
		return nil, fmt.Errorf("service %q has no symbols in store %q — ingest it before verifying", service, storePath)
	}
	// Typed Go call edges (ADR-021 §3), additive on top of the syntax graph —
	// the SAME graph the daemon's verify_plan builds (BuildBoth). Without them
	// the CLI gate ran on syntax-only edges, which resolve a bare method call
	// ("st.EnsureIndexes") through a name-suffix fallback that returns nothing
	// whenever two receivers share a method name (SQLiteStore and MemoryStore
	// both implement Store.EnsureIndexes) and never see a method value or an
	// interface dispatch at all — the O39/O40 false "does not reach" on
	// SU-1/SU-2, and INC-21's rows that verified under the daemon and failed
	// under `fathom verify --store` on the identical tree. Best-effort, as on
	// the daemon: a non-Go service or a store with no typed pass yet simply
	// contributes nothing.
	typed, _ := c.TypedEdges(ctx, service)
	graph := callgraph.BuildBoth(nodes, typed)

	var contras []core.Contradiction
	if res, qErr := c.QuerySymbols(ctx, core.Query{
		ServiceID: service,
		ConcernID: concern,
		TopN:      500,
	}); qErr == nil {
		contras = res.Contradictions
	}

	// Build spec matrix only if any task references spec IDs.
	var haveMat bool
	for _, p := range plans {
		for _, t := range p.Tasks {
			if len(t.SpecIDs) > 0 {
				haveMat = true
				break
			}
		}
		if haveMat {
			break
		}
	}
	matrix := buildCLISpecMatrix(ctx, c, service, nodes)

	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}

	ev := &cliEvidence{
		ctx:     ctx,
		c:       c,
		service: service,
		graph:   graph,
		contras: contras,
		matrix:  matrix,
		haveMat: haveMat,
		files:   files,
		root:    verifyRepoRoot(),
		byKey:   byKey,
		product: callgraph.ProductEntryReach(graph),
	}

	var mutationScores []verify.MutationScore
	if mutationPath != "" {
		mf, ferr := os.Open(mutationPath) //nolint:gosec — an operator-supplied CLI flag / env var
		if ferr != nil {
			return nil, fmt.Errorf("open --mutation %q: %w", mutationPath, ferr)
		}
		mrep, perr := coverage.ParseMutationReport(mf)
		mf.Close()
		if perr != nil {
			return nil, fmt.Errorf("parse --mutation %q: %w", mutationPath, perr)
		}
		for _, m := range coverage.JoinMutationSymbols(mrep, nodes) {
			mutationScores = append(mutationScores, verify.MutationScore{
				File: m.File, Line: m.Line, Mutants: m.Mutants, Killed: m.Killed, Survived: m.Survived, Score: m.Score,
			})
		}
	}

	reports := make([]verify.Report, 0, len(plans))
	for _, plan := range plans {
		rep := verify.VerifyPlan(plan, ev)
		if len(mutationScores) > 0 {
			verify.OverlayMutation(&rep, mutationScores, strictMutation)
		}
		reports = append(reports, rep)
	}
	return reports, nil
}

// httpVerifyResult is the JSON shape returned by verify_plan via the MCP server.
type httpVerifyResult struct {
	Service string `json:"service"`
	Summary struct {
		Total   int `json:"total"`
		Pass    int `json:"pass"`
		Fail    int `json:"fail"`
		Unknown int `json:"unknown"`
	} `json:"summary"`
	Tasks []httpVerifyTask `json:"tasks"`
	// Elided is the dispatch budget's disclosure (SU-3, budgeted_handler.go):
	// a long plan's tasks array is cut to what fits, and the rest of the
	// rows arrive in follow-up calls (verifyViaHTTP).
	Elided *struct {
		Count int `json:"count"`
		Of    int `json:"of"`
	} `json:"elided,omitempty"`
	// Partial names why the daemon answered short of every row: its call
	// graph for this index state was still building when the call's
	// deadline neared (EW-22, O130), or its spec scan was a lower bound.
	Partial string `json:"partial,omitempty"`
}

type httpVerifyTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Label    string `json:"label"`
	Criteria []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"criteria"`
}

// maxVerifyFollowUps bounds the follow-up verify_plan calls one plan may
// take when the dispatch budget cuts its answer.
const maxVerifyFollowUps = 64

// verifyViaHTTP calls verify_plan on the daemon and merges the verdicts with
// the locally-parsed claimed/skipped state. Gate is always computed locally.
//
// The daemon's dispatch budget (SU-3) cuts a long plan's tasks array to what
// fits and says so in an elided block; the rows it cut are verified in
// follow-up calls that carry only those rows (every other row replaced by a
// placeholder, so positional md-NNN ids stay put). A row whose verdict still
// never arrives is reported Unknown with the reason — never left out, which
// would let the gate pass a claim it never checked.
func verifyViaHTTP(hc *http.Client, baseURL, service, concern string, planMD []byte, localPlan verify.Plan) (verify.Report, error) {
	return verifyWithCaller(func(md []byte) (httpVerifyResult, error) {
		return callVerifyPlan(hc, baseURL, service, concern, md)
	}, planMD, localPlan)
}

// verifyWithCaller is verifyViaHTTP over any one-call transport: HTTP, or
// the daemon's own handler in process (verifyPlanInProcess).
func verifyWithCaller(call func(planMD []byte) (httpVerifyResult, error), planMD []byte, localPlan verify.Plan) (verify.Report, error) {
	hvr, err := call(planMD)
	if err != nil {
		return verify.Report{}, err
	}
	tasks := hvr.Tasks
	got := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		got[t.ID] = true
	}
	for i := 0; i < maxVerifyFollowUps && hvr.Elided != nil && hvr.Elided.Count > 0; i++ {
		sub, ok := verifyFollowUpPlan(planMD, localPlan, got)
		if !ok {
			break
		}
		next, err := call(sub)
		if err != nil {
			return verify.Report{}, err
		}
		added := 0
		for _, t := range next.Tasks {
			if !got[t.ID] && localPlanHas(localPlan, t.ID) {
				tasks = append(tasks, t)
				got[t.ID] = true
				added++
			}
		}
		hvr.Elided, hvr.Partial = next.Elided, next.Partial
		if added == 0 {
			break
		}
	}

	// Build verify.Report with locally-known claimed/skipped (gate is local).
	claimedByID := map[string]bool{}
	skippedByID := map[string]bool{}
	inProgressByID := map[string]bool{}
	weightByID := map[string]int{}
	for _, t := range localPlan.Tasks {
		claimedByID[t.ID] = t.Claimed
		skippedByID[t.ID] = t.Skipped
		inProgressByID[t.ID] = t.InProgress
		weightByID[t.ID] = t.Weight
	}

	verdicts := make([]verify.TaskVerdict, 0, len(localPlan.Tasks))
	for _, ht := range tasks {
		var criteria []verify.Criterion
		for _, hc := range ht.Criteria {
			criteria = append(criteria, verify.Criterion{
				Name:   hc.Name,
				Status: verify.CriterionStatus(hc.Status),
				Detail: hc.Detail,
			})
		}
		verdicts = append(verdicts, verify.TaskVerdict{
			TaskID:     ht.ID,
			Title:      ht.Title,
			Status:     verify.CriterionStatus(ht.Status),
			Label:      ht.Label,
			Criteria:   criteria,
			Claimed:    claimedByID[ht.ID],
			Skipped:    skippedByID[ht.ID],
			InProgress: inProgressByID[ht.ID],
			Weight:     weightByID[ht.ID],
		})
	}
	// A row the daemon never answered for: Unknown, with the reason.
	for _, t := range localPlan.Tasks {
		if got[t.ID] {
			continue
		}
		got[t.ID] = true
		detail := "the daemon's verify_plan answer was cut by its response budget and this row's verdict never arrived — verify with --store, or narrow the plan"
		if hvr.Partial != "" && len(tasks) == 0 {
			detail = "the daemon answered before any row could be verified (" + hvr.Partial + ") — re-run verify"
		}
		verdicts = append(verdicts, verify.TaskVerdict{
			TaskID: t.ID, Title: t.Title, Status: verify.Unknown, Label: "verdict-elided",
			Criteria: []verify.Criterion{{Name: "verdict", Status: verify.Unknown,
				Detail: detail}},
			Claimed: t.Claimed, Skipped: t.Skipped, InProgress: t.InProgress, Weight: t.Weight,
		})
	}
	sort.SliceStable(verdicts, func(i, j int) bool { return verdicts[i].TaskID < verdicts[j].TaskID })
	var sum verify.Summary
	if len(tasks) == len(hvr.Tasks) && hvr.Elided == nil {
		// Nothing was cut: the daemon's own summary stands.
		sum = verify.Summary{Total: hvr.Summary.Total, Pass: hvr.Summary.Pass, Fail: hvr.Summary.Fail, Unknown: hvr.Summary.Unknown}
	} else {
		sum.Total = len(verdicts)
		for _, v := range verdicts {
			switch v.Status {
			case verify.Pass:
				sum.Pass++
			case verify.Fail:
				sum.Fail++
			default:
				sum.Unknown++
			}
		}
	}
	return verify.Report{
		Service: hvr.Service,
		Tasks:   verdicts,
		Summary: sum,
	}, nil
}

// callVerifyPlan is one verify_plan call.
func callVerifyPlan(hc *http.Client, baseURL, service, concern string, planMD []byte) (httpVerifyResult, error) {
	reqBody, err := verifyPlanRequest(service, concern, planMD)
	if err != nil {
		return httpVerifyResult{}, err
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return httpVerifyResult{}, fmt.Errorf("build request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return httpVerifyResult{}, fmt.Errorf("POST %s/rpc: %w", baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return httpVerifyResult{}, fmt.Errorf("reading response: %w", err)
	}
	return parseVerifyPlanResponse(raw)
}

// verifyPlanRequest is one verify_plan tools/call message.
func verifyPlanRequest(service, concern string, planMD []byte) ([]byte, error) {
	args := map[string]any{
		"service":       service,
		"plan_markdown": string(planMD),
	}
	if concern != "" {
		args["concern"] = concern
	}
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "verify_plan",
			"arguments": args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	return reqBody, nil
}

// parseVerifyPlanResponse unwraps a verify_plan JSON-RPC response.
func parseVerifyPlanResponse(raw []byte) (httpVerifyResult, error) {
	// Unwrap MCP content envelope: {"result":{"content":[{"text":"<json>"}]}}
	innerJSON, err := unwrapRPCResult(raw)
	if err != nil {
		return httpVerifyResult{}, fmt.Errorf("unwrap verify_plan response: %w", err)
	}
	var hvr httpVerifyResult
	if err := json.Unmarshal(innerJSON, &hvr); err != nil {
		return httpVerifyResult{}, fmt.Errorf("parse verify_plan result: %w", err)
	}
	return hvr, nil
}

// verifyFollowUpPlan is the plan markdown for a follow-up call: the rows
// whose verdicts have not arrived kept verbatim, every row that has one
// replaced by a placeholder row, so the parser's positional md-NNN ids are
// unchanged. A placeholder's own id starts with "~", which sorts after every
// other id, so the budget's cut (a prefix in id order) never spends itself
// on placeholders before a missing row. ok is false when no row is missing.
func verifyFollowUpPlan(planMD []byte, plan verify.Plan, got map[string]bool) ([]byte, bool) {
	lines := strings.Split(string(planMD), "\n")
	keep := map[int]bool{}
	missing := 0
	for _, t := range plan.Tasks {
		if !got[t.ID] {
			keep[t.Line] = true
			missing++
		}
	}
	if missing == 0 {
		return nil, false
	}
	var b strings.Builder
	for i, t := range plan.Tasks {
		if keep[t.Line] && t.Line >= 1 && t.Line <= len(lines) {
			// A wrapped row (G2-5) keeps every line the parser read for it,
			// or the follow-up would verify it without its annotation.
			end := t.EndLine
			if end < t.Line || end > len(lines) {
				end = t.Line
			}
			b.WriteString(strings.Join(lines[t.Line-1:end], "\n"))
		} else {
			fmt.Fprintf(&b, "- [-] . {id: ~%d}", i+1)
		}
		b.WriteByte('\n')
	}
	return []byte(b.String()), true
}

// localPlanHas reports whether the plan has a row with id.
func localPlanHas(plan verify.Plan, id string) bool {
	for _, t := range plan.Tasks {
		if t.ID == id {
			return true
		}
	}
	return false
}

// daemonServiceIndexed reports whether the daemon has indexed service.
// false-with-nil-error means the daemon answered and the service is
// genuinely absent, never indexed, or indexed with zero symbols (the
// exit-2 fail-open below depends on that last case). It asks GET /indexed
// (RT-80: a meta row plus the cached symbol-count aggregate); only a daemon that predates that route is asked through
// list_services, the probe it has always served.
func daemonServiceIndexed(hc *http.Client, baseURL, service string) (bool, error) {
	indexed, err := probeIndexed(hc, baseURL, service)
	if !errors.Is(err, errIndexedRouteMissing) {
		return indexed, err
	}
	return daemonServiceIndexedViaListServices(hc, baseURL, service)
}

// daemonServiceIndexedViaListServices is the pre-RT-80 probe: whether
// list_services reports symbols for service.
func daemonServiceIndexedViaListServices(hc *http.Client, baseURL, service string) (bool, error) {
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`
	req, err := originHeader(baseURL, strings.NewReader(reqBody))
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("POST %s/rpc: %w", baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("reading list_services response: %w", err)
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return false, fmt.Errorf("unwrap list_services response: %w", err)
	}
	var payload struct {
		Services []struct {
			Service          string `json:"service"`
			SymbolCount      int    `json:"symbol_count"`
			SymbolCountError string `json:"symbol_count_error"`
		} `json:"services"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return false, fmt.Errorf("parse list_services result: %w", err)
	}
	for _, s := range payload.Services {
		if !strings.EqualFold(s.Service, service) {
			continue
		}
		// RT-41 (I38): an unreadable count is an error (verify's exit 2,
		// fail-open), never "zero symbols, not indexed".
		if s.SymbolCountError != "" {
			return false, fmt.Errorf("list_services: %s symbol count unreadable: %s", s.Service, s.SymbolCountError)
		}
		if s.SymbolCount > 0 {
			return true, nil
		}
	}
	return false, nil
}

// unwrapRPCResult extracts the inner JSON text from an MCP JSON-RPC envelope:
// {"result":{"content":[{"type":"text","text":"<inner>"}]}}
func unwrapRPCResult(raw []byte) ([]byte, error) {
	var outer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parse JSON-RPC envelope: %w", err)
	}
	if outer.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", outer.Error.Message)
	}
	if len(outer.Result.Content) == 0 {
		return nil, fmt.Errorf("RPC result has no content")
	}
	return []byte(outer.Result.Content[0].Text), nil
}

// verifyIcon returns the human-readable status icon for a task verdict.
func verifyIcon(tv verify.TaskVerdict) string {
	if tv.Skipped {
		return "–" // en-dash: [-] skipped/abandoned
	}
	if tv.InProgress {
		return "~" // [~] in progress
	}
	switch tv.Status {
	case verify.Pass:
		return "✓"
	case verify.Fail:
		return "✗"
	default:
		return "?"
	}
}

// reportsToMaps renders a slice of verify.Report as JSON-friendly maps.
func reportsToMaps(reports []verify.Report) []any {
	out := make([]any, 0, len(reports))
	for _, rep := range reports {
		tasks := make([]any, 0, len(rep.Tasks))
		for _, tv := range rep.Tasks {
			crit := make([]any, 0, len(tv.Criteria))
			for _, c := range tv.Criteria {
				crit = append(crit, map[string]any{
					"name":   c.Name,
					"status": string(c.Status),
					"detail": c.Detail,
				})
			}
			tm := map[string]any{
				"id":       tv.TaskID,
				"title":    tv.Title,
				"status":   string(tv.Status),
				"label":    tv.Label,
				"criteria": crit,
			}
			if tv.Claimed {
				tm["claimed"] = true
			}
			if tv.Skipped {
				tm["skipped"] = true
			}
			if tv.Line > 0 {
				tm["line"] = tv.Line
			}
			tasks = append(tasks, tm)
		}
		out = append(out, map[string]any{
			"service": rep.Service,
			"summary": map[string]any{
				"total":   rep.Summary.Total,
				"pass":    rep.Summary.Pass,
				"fail":    rep.Summary.Fail,
				"unknown": rep.Summary.Unknown,
			},
			"tasks": tasks,
		})
	}
	return out
}

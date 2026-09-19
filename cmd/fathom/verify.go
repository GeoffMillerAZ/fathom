package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/core"
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
	fs.Var(&plans, "plan", "plan file (repeatable; ≥1 required; .md checklist)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	strict := fs.Bool("strict", false, "treat claimed+unknown tasks as gate failures (recommended for CI)")
	requireAll := fs.Bool("require-all", false, "wave gate: additionally fail on open/in-progress tasks (ADR-018 §2)")
	concern := fs.String("concern", "", "optional concern for contradiction scoping")
	jsonOut := fs.Bool("json", false, "emit JSON report + gate result instead of human text")
	format := fs.String("format", "", "output format: text (default) | json | sarif — SARIF 2.1.0 of the refusals, one rule per criterion, located at the plan line (PC-21)")
	quietPass := fs.Bool("quiet-pass", false, "suppress output when gate passes (exit 0)")
	quiet := fs.Bool("quiet", false, "print only failing (and, with --strict, unknown) task rows plus the summary; composes with --quiet-pass")
	mutationPath := fs.String("mutation", "", "VB-34: mutation-testing report (go-mutesting log output, or fathom's own JSON shape); a claimed row whose symbol has mutation score 0 gets a covered-not-asserted note (falls back to $"+mutationReportEnvVar+"; direct/--store mode only)")
	strictMutation := fs.Bool("strict-mutation", false, "gate-fail a covered-not-asserted finding instead of noting it as advisory")

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
	case "", "text", "json", "sarif":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want text, json or sarif\n", *format)
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
	for _, pf := range plans {
		raw, err := os.ReadFile(pf)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading plan %q: %v\n", pf, err)
			return 2
		}
		if !strings.HasSuffix(pf, ".md") {
			fmt.Fprintf(stderr, "error: plan %q: only .md checklist plans are supported by fathom verify\n", pf)
			return 2
		}
		lp := verify.ParseMarkdownPlan(*service, raw)
		// O56: mark any claimed row whose skip_test/skip was added in the
		// working tree or in the commit that flipped its box — self-skipped,
		// never proven by the agent's own hand. Inapplicable (never flagged)
		// outside a git repo, or when pf resolves outside root.
		if absPF, aerr := filepath.Abs(pf); aerr == nil {
			if rel, ok := relativeToRoot(root, absPF); ok {
				lp = verify.AnnotateSelfSkip(lp, priorSelfSkipPlan(root, rel, *service, raw))
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
			rep, err := verifyViaHTTP(hc, *url, *service, *concern, planBytes[i], lp)
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

	// Compute gate locally over all reports.
	var allFail []string
	for _, rep := range reports {
		var gr verify.GateResult
		if *requireAll {
			gr = verify.GateRequireAll(rep, *strict)
		} else {
			gr = verify.Gate(rep, *strict)
		}
		allFail = append(allFail, gr.Failures...)
	}
	gatePass := len(allFail) == 0

	if *format == "sarif" {
		_, _ = stdout.Write(verify.RenderSARIF("fathom verify", verify.GateSARIF(plans, reports, *strict)))
	} else if *format == "json" {
		out := map[string]any{
			"gate": map[string]any{
				"pass":     gatePass,
				"failures": allFail,
			},
			"reports": reportsToMaps(reports),
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else if !gatePass || !*quietPass {
		for _, rep := range reports {
			for _, tv := range rep.Tasks {
				icon := verifyIcon(tv)
				// --quiet: print only failing rows (✗), plus unknown rows (?)
				// when --strict is also set — a gate run over many plans reads
				// as its exceptions, not a wall of checkmarks (VS-8 / O19 / G14).
				if *quiet && icon != "✗" && !(*strict && icon == "?") {
					continue
				}
				fmt.Fprintf(stdout, "%s [%s] %s\n", icon, tv.TaskID, tv.Title)
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
	graph := callgraph.Build(nodes)
	// Typed Go call edges (ADR-021 §3), additive on top of the syntax graph —
	// the SAME graph the daemon's verify_plan builds (BuildBoth). INC-21: this
	// path built the syntax-only graph, so a plan row whose reachability runs
	// through a method value or an interface dispatch verified under the
	// daemon and failed under `fathom verify --store` on the identical tree.
	// Best-effort: a non-Go service or a store with no typed edges yet simply
	// contributes nothing.
	if typed, typedErr := c.TypedEdges(ctx, service); typedErr == nil {
		callgraph.AddTypedEdges(graph, typed)
	}

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

	ev := &cliEvidence{
		ctx:     ctx,
		c:       c,
		service: service,
		graph:   graph,
		contras: contras,
		matrix:  matrix,
		haveMat: haveMat,
		files:   files,
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
	Tasks []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Label    string `json:"label"`
		Criteria []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"criteria"`
	} `json:"tasks"`
}

// verifyViaHTTP calls verify_plan on the daemon and merges the verdicts with
// the locally-parsed claimed/skipped state. Gate is always computed locally.
func verifyViaHTTP(hc *http.Client, baseURL, service, concern string, planMD []byte, localPlan verify.Plan) (verify.Report, error) {
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
		return verify.Report{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return verify.Report{}, fmt.Errorf("build request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return verify.Report{}, fmt.Errorf("POST %s/rpc: %w", baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return verify.Report{}, fmt.Errorf("reading response: %w", err)
	}

	// Unwrap MCP content envelope: {"result":{"content":[{"text":"<json>"}]}}
	innerJSON, err := unwrapRPCResult(raw)
	if err != nil {
		return verify.Report{}, fmt.Errorf("unwrap verify_plan response: %w", err)
	}

	var hvr httpVerifyResult
	if err := json.Unmarshal(innerJSON, &hvr); err != nil {
		return verify.Report{}, fmt.Errorf("parse verify_plan result: %w", err)
	}

	// Build verify.Report with locally-known claimed/skipped (gate is local).
	claimedByID := map[string]bool{}
	skippedByID := map[string]bool{}
	inProgressByID := map[string]bool{}
	for _, t := range localPlan.Tasks {
		claimedByID[t.ID] = t.Claimed
		skippedByID[t.ID] = t.Skipped
		inProgressByID[t.ID] = t.InProgress
	}

	verdicts := make([]verify.TaskVerdict, 0, len(hvr.Tasks))
	for _, ht := range hvr.Tasks {
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
		})
	}
	return verify.Report{
		Service: hvr.Service,
		Tasks:   verdicts,
		Summary: verify.Summary{
			Total:   hvr.Summary.Total,
			Pass:    hvr.Summary.Pass,
			Fail:    hvr.Summary.Fail,
			Unknown: hvr.Summary.Unknown,
		},
	}, nil
}

// daemonServiceIndexed reports whether the daemon's index has symbols for
// service (via the list_services tool). false-with-nil-error means the daemon
// answered and the service is genuinely absent/empty.
func daemonServiceIndexed(hc *http.Client, baseURL, service string) (bool, error) {
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
			Service     string `json:"service"`
			SymbolCount int    `json:"symbol_count"`
		} `json:"services"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return false, fmt.Errorf("parse list_services result: %w", err)
	}
	for _, s := range payload.Services {
		if strings.EqualFold(s.Service, service) && s.SymbolCount > 0 {
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

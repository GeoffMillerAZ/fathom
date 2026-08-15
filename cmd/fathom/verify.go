package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

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
	quietPass := fs.Bool("quiet-pass", false, "suppress output when gate passes (exit 0)")

	if err := fs.Parse(args); err != nil {
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
		for _, w := range verify.LintPlan(lp) {
			fmt.Fprintf(stderr, "lint: %s: %s\n", pf, w)
		}
		localPlans = append(localPlans, lp)
		planBytes = append(planBytes, raw)
	}

	// Gather verdicts — either in-process (--store) or via the daemon (HTTP).
	var reports []verify.Report
	if *storePath != "" {
		reps, err := verifyAllAgainstStore(localPlans, *service, *storePath, *concern)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		reports = reps
	} else {
		hc := &http.Client{Timeout: 10 * time.Second}
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
		for i, lp := range localPlans {
			rep, err := verifyViaHTTP(hc, *url, *service, *concern, planBytes[i], lp)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
			reports = append(reports, rep)
		}
	}

	// SD-1 (docs/plans/custodian.md): every verify run leaves a ledger
	// row per plan — the history that lets a trend be a measurement
	// instead of a guess, and the signal the flywheel's verify-gate loop
	// reads (O30: lefthook pre-push runs were invisible to it). Best
	// effort, never part of the gate.
	appendVerifyLedger(plans, *service, reports)

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

	if *jsonOut {
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
// in sequence, sharing the call graph and contradiction index.
func verifyAllAgainstStore(plans []verify.Plan, service, storePath, concern string) ([]verify.Report, error) {
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
	}

	reports := make([]verify.Report, 0, len(plans))
	for _, plan := range plans {
		reports = append(reports, verify.VerifyPlan(plan, ev))
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

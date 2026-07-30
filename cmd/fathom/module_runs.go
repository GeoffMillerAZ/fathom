package main

// module_runs.go — `fathom module runs`: compile-run history (dogfood gap
// (c), phase-K studio bug hunt). Before this verb existed, only the
// just-triggered `module compile` invocation's own result was visible to a
// verb-JSON-only consumer (law 10, fathom-studio may only consume verb
// JSON, never reach into the store directly) — every EARLIER compile_runs
// row for the module's layer was invisible outside a direct SQLite query.
//
// This reuses store.MountReadOnly + Store.CompileRuns exactly as
// module_doctor.go's loadStoreState already does (no new store method — the
// read already existed) and CompileRuns' own query already orders
// newest-first, deterministically (ORDER BY ran_at DESC, run_id — see
// internal/store/sqlite.go), so no re-sorting is needed here.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// CompileRunSummary is one compile_runs row, reshaped for verb JSON —
// id/started/model/lane/watermark/claims/cost, per the dogfood gap's exact
// ask. Lane is derived from Model the same way classifyCompileLane derives
// a store-wide lane from a claim's Model field (module_modellane.go): empty
// is the deterministic/oracle-class lane, mockModelName is the scripted dev
// lane, anything else is a real model name.
type CompileRunSummary struct {
	ID            string  `json:"id"`
	StartedAt     string  `json:"started_at"`
	Model         string  `json:"model"`
	Lane          string  `json:"lane"`
	Watermark     string  `json:"watermark"`
	ClaimsEmitted int     `json:"claims_emitted"`
	CostUSD       float64 `json:"cost_usd"`
	// DryRun/Refused/BudgetStopped surface an otherwise-invisible run
	// shape (a refused or budget-stopped run has no/partial claims, which
	// would otherwise read as an unremarkable low number) — omitted (false
	// is the common case) rather than always present, so a normal run's
	// JSON stays exactly the six fields above.
	DryRun        bool `json:"dry_run,omitempty"`
	Refused       bool `json:"refused,omitempty"`
	BudgetStopped bool `json:"budget_stopped,omitempty"`
}

// RunsReport is `module runs`'s full output.
type RunsReport struct {
	Module  string              `json:"module"`
	Service string              `json:"service"`
	Layer   string              `json:"layer"`
	Runs    []CompileRunSummary `json:"runs"`
}

// runLane classifies a single compile_runs row's Model field the same way
// classifyCompileLane classifies a whole claim set's — but per-run rather
// than per-store-snapshot, since each row already carries its own Model.
func runLane(model string) string {
	switch model {
	case "":
		return "deterministic"
	case mockModelName:
		return "mock"
	default:
		return "real"
	}
}

func cmdModuleRuns(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module runs", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID in --store (default: manifest name)")
	limit := fs.Int("limit", 20, "maximum runs to return (0 = no limit); CompileRuns is already newest-first, so this simply truncates")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" || *storePath == "" {
		fmt.Fprintln(stderr, "error: --manifest and --store are required")
		fs.Usage()
		return 1
	}

	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: load manifest %s: %v\n", *manifestPath, err)
		return 1
	}
	svc := *service
	if svc == "" {
		svc = m.Name
	}

	mounted, mErr := store.MountReadOnly(*storePath, store.LevelL0)
	if mErr != nil {
		fmt.Fprintf(stderr, "error: mount store %s: %v\n", *storePath, mErr)
		return 1
	}
	defer mounted.Close()

	runs, rErr := mounted.CompileRuns(context.Background(), svc, m.Name)
	if rErr != nil {
		fmt.Fprintf(stderr, "error: read compile runs: %v\n", rErr)
		return 1
	}
	if *limit > 0 && len(runs) > *limit {
		runs = runs[:*limit]
	}

	report := RunsReport{Module: m.Name, Service: svc, Layer: m.Name, Runs: make([]CompileRunSummary, 0, len(runs))}
	for _, r := range runs {
		report.Runs = append(report.Runs, CompileRunSummary{
			ID:            r.RunID,
			StartedAt:     r.RanAt.UTC().Format(time.RFC3339),
			Model:         r.Model,
			Lane:          runLane(r.Model),
			Watermark:     r.CorpusWatermark,
			ClaimsEmitted: r.ClaimsEmitted,
			CostUSD:       r.CostUSD,
			DryRun:        r.DryRun,
			Refused:       r.Refused,
			BudgetStopped: r.BudgetStopped,
		})
	}

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "module:  %s\n", report.Module)
	fmt.Fprintf(stdout, "service: %s\n\n", report.Service)
	if len(report.Runs) == 0 {
		fmt.Fprintln(stdout, "no compile runs recorded for this module's layer")
		return 0
	}
	for _, r := range report.Runs {
		note := ""
		switch {
		case r.Refused:
			note = " REFUSED"
		case r.BudgetStopped:
			note = " BUDGET-STOPPED"
		case r.DryRun:
			note = " DRY-RUN"
		}
		fmt.Fprintf(stdout, "%s  %-24s  model=%-16s lane=%-13s claims=%-4d cost=$%.4f watermark=%s%s\n",
			r.ID, r.StartedAt, r.Model, r.Lane, r.ClaimsEmitted, r.CostUSD, r.Watermark, note)
	}
	return 0
}

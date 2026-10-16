// Command eval-runner is the `task eval` entrypoint (H5 Stage 1, first
// subscriber per uplift D9: the kit's own agent-driven development process).
//
// It is the composition root for a REPLAY-mode run of kits/eval-runner over
// the committed corpus: it loads the scenario bank (through the runner's own
// LoadBank path), the caller-supplied outputs, the model catalog, the price
// table, and the DLP-scrubbed judge cassette from kits/eval-runner/testdata,
// builds the budget guard from Config.Budget, and runs with a fixed injected
// clock. No provider keys are read and no network is touched —
// `env -i PATH=$PATH HOME=$HOME task eval` must exit 0.
//
// Exit status is non-zero on: construction failure (independence violations
// and a production-captured bank source are construction-fatal),
// ErrEmptyBank (a run over nothing can never exit 0 — the count assertion
// lives in the runner, not here), any other run error including the typed
// budget stop, a non-completed stop cause, and a regression-gate failure
// against the committed baseline. On success it prints the scenario count
// (informational — the empty-bank refusal is the real check).
//
// All fixture data under testdata/ is EXAMPLE data (models, families,
// prices), never authoritative — see kits/eval-runner/testdata/fixtures.cue.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	costtracker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/cost/cost-tracker"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	modelcatalog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/model-catalog"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
	evalrunner "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/eval-runner"
)

// fixedNow is the injected clock for replay-mode runs. Replay must be
// byte-stable (TestReplayRunByteStable), so the wall clock never leaks in:
// the same instant is used here, in the cassette/baseline generator
// (kits/eval-runner/gen-cassettes), and in the integration tests.
var fixedNow = func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }

// healthyPinger satisfies llmclient.Pinger for replay lanes. Probing a
// cassette proves nothing about a live endpoint — replay mode's startup
// probe exercises the runner's probe wiring, not provider reachability
// (replay-green ≠ live-green, the kit's own stated non-guarantee).
type healthyPinger struct{}

func (healthyPinger) Ping(ctx context.Context) error { return ctx.Err() }

func main() {
	dir := flag.String("dir", "kits/eval-runner/testdata", "corpus directory (scenarios, outputs, catalog, prices, cassette, baseline)")
	flag.Parse()

	if err := run(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "eval-runner: %v\n", err)
		os.Exit(1)
	}
}

// config builds the replay-mode Config over the corpus directory. Mirrored
// by corpusConfig in kits/eval-runner/eval_integration_test.go and by the
// generator — keep the three in sync or byte-stability breaks loudly.
func config(dir string) evalrunner.Config {
	return evalrunner.Config{
		RunID: "eval",
		JudgeLanes: []evalrunner.JudgeLane{
			{Name: "judge-a", Model: "example-judge-1", Family: "beta"},
		},
		Producer: evalrunner.Producer{
			Model:      "example-producer-1",
			PromptHash: "prompthash-fixture-0001",
			ParamsHash: "paramshash-fixture-0001",
		},
		Mode:             evalrunner.ModeReplay,
		ScenarioBankPath: filepath.Join(dir, "scenarios.json"),
		BaselinePath:     filepath.Join(dir, "baseline.jsonl"),
		PriceTableSource: filepath.Join(dir, "pricetable.json"),
		Budget: evalrunner.BudgetLimits{
			MaxReqPerMin: 1000,
			MaxReqPerDay: 1000,
			MaxUSDPerDay: 10,
		},
		DeterminismK: 3,
	}
}

// outputWire is the JSON shape of testdata/outputs.json.
type outputWire struct {
	ScenarioID string `json:"scenario_id"`
	Text       string `json:"text"`
	Producer   struct {
		Model      string `json:"model"`
		PromptHash string `json:"prompt_hash"`
		ParamsHash string `json:"params_hash"`
	} `json:"producer"`
}

func loadOutputs(path string) ([]evalrunner.Output, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wire []outputWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}
	outputs := make([]evalrunner.Output, 0, len(wire))
	for _, w := range wire {
		outputs = append(outputs, evalrunner.Output{
			ScenarioID: w.ScenarioID,
			Text:       w.Text,
			Producer: evalrunner.Producer{
				Model:      w.Producer.Model,
				PromptHash: w.Producer.PromptHash,
				ParamsHash: w.Producer.ParamsHash,
			},
		})
	}
	return outputs, nil
}

func run(dir string) error {
	cfg := config(dir)

	// Scenario bank — through the runner's own bank-loading path, so the
	// provenance gate (production-captured refusal) is kit wiring, not prose.
	bank, err := evalrunner.LoadBank(cfg.ScenarioBankPath)
	if err != nil {
		return fmt.Errorf("load scenario bank: %w", err)
	}

	// Caller-supplied, pre-produced outputs (Stage 1: the runner drives no
	// producer lane).
	outputs, err := loadOutputs(filepath.Join(dir, "outputs.json"))
	if err != nil {
		return fmt.Errorf("load outputs: %w", err)
	}

	// Model catalog — caller data, EXAMPLE rows (never shipped as defaults).
	rawCatalog, err := os.ReadFile(filepath.Join(dir, "modelcatalog.json"))
	if err != nil {
		return fmt.Errorf("read model catalog: %w", err)
	}
	catalog, err := modelcatalog.Load(rawCatalog)
	if err != nil {
		return fmt.Errorf("load model catalog: %w", err)
	}

	// Price table — fixture prices (explicitly example data) so offline
	// budget accounting is defined, not skipped.
	rawPrices, err := os.ReadFile(cfg.PriceTableSource)
	if err != nil {
		return fmt.Errorf("read price table: %w", err)
	}
	var prices costtracker.PriceTable
	if err := json.Unmarshal(rawPrices, &prices); err != nil {
		return fmt.Errorf("decode price table: %w", err)
	}

	// Judge lane — replay client over the committed DLP-scrubbed cassette,
	// strict hash matching on.
	entries, err := llmreplay.LoadFile(filepath.Join(dir, "cassettes", "judge.jsonl"))
	if err != nil {
		return fmt.Errorf("load judge cassette: %w", err)
	}
	judgeClient := llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash())

	var journal jsonlBuffer
	deps := evalrunner.Deps{
		JudgeClients: map[string]llmclient.LLMClient{"judge-a": judgeClient},
		Pingers:      map[string]llmclient.Pinger{"judge-a": healthyPinger{}},
		Catalog:      catalog,
		Guard: budgetguard.New(budgetguard.Config{
			MaxReqPerMin: cfg.Budget.MaxReqPerMin,
			MaxReqPerDay: cfg.Budget.MaxReqPerDay,
			MaxUSDPerDay: cfg.Budget.MaxUSDPerDay,
		}, fixedNow),
		PriceTable: prices,
		Journal:    jsonllog.NewWriter(&journal),
		Now:        fixedNow,
	}

	runner, err := evalrunner.New(cfg, deps)
	if err != nil {
		return fmt.Errorf("construct runner: %w", err)
	}

	res, err := runner.Run(context.Background(), bank, outputs)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if res.Stop != evalrunner.StopCompleted {
		return fmt.Errorf("run stopped with cause %q (want %q) after %d judge calls — a non-completed run is never green", res.Stop, evalrunner.StopCompleted, res.JudgeCalls)
	}

	// Regression gate against the committed baseline.
	baseline, err := os.Open(cfg.BaselinePath)
	if err != nil {
		return fmt.Errorf("open baseline: %w", err)
	}
	defer baseline.Close()
	reg, err := runner.Regression(baseline, res.Records)
	if err != nil {
		return fmt.Errorf("regression gate: %w", err)
	}
	if !reg.Passed {
		return fmt.Errorf("regression gate failed: %d regression(s), %d verdict flip(s) vs baseline %s",
			reg.Delta.Regressions, len(reg.Delta.VerdictFlips), cfg.BaselinePath)
	}

	stability := "unprobed"
	if res.Probe != nil {
		stability = string(res.Probe.Stability)
	}
	fmt.Printf("eval: %d scenarios judged in replay mode (records=%d, stable=%t, probe=%s, regressions=%d)\n",
		res.ScenarioCount, len(res.Records), res.Stable, stability, reg.Delta.Regressions)
	return nil
}

// jsonlBuffer is a minimal in-memory io.Writer for the journal: `task eval`
// gates on the run and the regression result; the journal bytes themselves
// are proven byte-stable by TestReplayRunByteStable.
type jsonlBuffer struct{ buf []byte }

func (b *jsonlBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

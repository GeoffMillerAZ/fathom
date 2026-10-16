// Command gen-cassettes regenerates the committed replay corpus for
// kits/eval-runner: the DLP-scrubbed judge cassette
// (testdata/cassettes/judge.jsonl) and the regression baseline journal
// (testdata/baseline.jsonl).
//
// Recording is KEY-GATED and never runs in tests: the command refuses to run
// unless EVALRUNNER_RECORD=1 is set (`task eval-record` sets it). It is
// nevertheless fully OFFLINE and deterministic — the "live" judge lane it
// records is llmclient.MockClient scripted with structured
// {verdict, score, rationale} JSON, wrapped in llmreplay.NewRecordingClient
// with the default scrubber (the repo's cassette scrub discipline), driven
// through the runner's own New/Run path with the shared fixed clock. Because
// the cassette is recorded THROUGH evalrunner.LLMJudge, the recorded request
// bytes always match what replay-mode runs send, so
// llmreplay.MatchByRequestHash stays strict.
//
// Entry layout contract (pinned by TestCassetteProbeLayout): replay consumes
// ordinally, so the cassette records the DeterminismK probe entries first,
// in scheduling order (K identical requests against the first scenario),
// then one judge entry per scenario in bank order. Probe calls are
// budget-charged but NOT journaled (they would pollute the diff baseline),
// so baseline.jsonl carries exactly one record per scenario.
//
// Regenerate whenever the corpus, the judge prompt shape, or
// foundations/llmclient request types change:
//
//	task eval-record
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

// fixedNow must match cmd/eval-runner and the integration tests: replay-mode
// byte-stability is only meaningful when the baseline and every replay run
// share one injected instant.
var fixedNow = func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }

// scriptedJudgeVerdict is the structured judge output every recorded call
// returns. Identical responses across the probe's K calls make the committed
// corpus probe Stable by construction; the variance machinery is exercised
// separately in tests with divergent mock scripts.
const scriptedJudgeVerdict = `{"verdict":"pass","score":0.9,"rationale":"Output satisfies the scenario rubric: citations, labeling, and grounding rules are followed."}`

type healthyPinger struct{}

func (healthyPinger) Ping(ctx context.Context) error { return ctx.Err() }

func main() {
	dir := flag.String("dir", "kits/eval-runner/testdata", "corpus directory to (re)generate cassette + baseline into")
	flag.Parse()

	if os.Getenv("EVALRUNNER_RECORD") != "1" {
		fmt.Fprintln(os.Stderr, "gen-cassettes: refusing to record — set EVALRUNNER_RECORD=1 explicitly (recording overwrites the committed cassette and baseline); use `task eval-record`")
		os.Exit(1)
	}
	if err := run(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "gen-cassettes: %v\n", err)
		os.Exit(1)
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

func run(dir string) error {
	cfg := evalrunner.Config{
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
		BaselinePath:     "", // this run RECORDS the baseline; nothing to diff against
		PriceTableSource: filepath.Join(dir, "pricetable.json"),
		Budget: evalrunner.BudgetLimits{
			MaxReqPerMin: 1000,
			MaxReqPerDay: 1000,
			MaxUSDPerDay: 10,
		},
		DeterminismK: 3,
	}

	bank, err := evalrunner.LoadBank(cfg.ScenarioBankPath)
	if err != nil {
		return fmt.Errorf("load scenario bank: %w", err)
	}

	rawOutputs, err := os.ReadFile(filepath.Join(dir, "outputs.json"))
	if err != nil {
		return fmt.Errorf("read outputs: %w", err)
	}
	var wire []outputWire
	if err := json.Unmarshal(rawOutputs, &wire); err != nil {
		return fmt.Errorf("decode outputs: %w", err)
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

	rawCatalog, err := os.ReadFile(filepath.Join(dir, "modelcatalog.json"))
	if err != nil {
		return fmt.Errorf("read model catalog: %w", err)
	}
	catalog, err := modelcatalog.Load(rawCatalog)
	if err != nil {
		return fmt.Errorf("load model catalog: %w", err)
	}

	rawPrices, err := os.ReadFile(cfg.PriceTableSource)
	if err != nil {
		return fmt.Errorf("read price table: %w", err)
	}
	var prices costtracker.PriceTable
	if err := json.Unmarshal(rawPrices, &prices); err != nil {
		return fmt.Errorf("decode price table: %w", err)
	}

	// Scripted "live" judge: enough identical entries for the K probe calls
	// plus one call per scenario (extra entries are never consumed — the
	// recording client persists only calls that actually happen).
	entries := make([]llmclient.MockEntry, 0, 64)
	for i := 0; i < 64; i++ {
		entries = append(entries, llmclient.MockEntryOK(scriptedJudgeVerdict))
	}
	mock := llmclient.NewMockClient(entries...)

	if err := os.MkdirAll(filepath.Join(dir, "cassettes"), 0o755); err != nil {
		return fmt.Errorf("mkdir cassettes: %w", err)
	}
	cassettePath := filepath.Join(dir, "cassettes", "judge.jsonl")
	cassette, err := os.Create(cassettePath)
	if err != nil {
		return fmt.Errorf("create cassette: %w", err)
	}
	defer cassette.Close()
	recording := llmreplay.NewRecordingClient(mock, cassette) // DefaultScrubber stays on

	baselinePath := filepath.Join(dir, "baseline.jsonl")
	baseline, err := os.Create(baselinePath)
	if err != nil {
		return fmt.Errorf("create baseline: %w", err)
	}
	defer baseline.Close()

	deps := evalrunner.Deps{
		JudgeClients: map[string]llmclient.LLMClient{"judge-a": recording},
		Pingers:      map[string]llmclient.Pinger{"judge-a": healthyPinger{}},
		Catalog:      catalog,
		Guard: budgetguard.New(budgetguard.Config{
			MaxReqPerMin: cfg.Budget.MaxReqPerMin,
			MaxReqPerDay: cfg.Budget.MaxReqPerDay,
			MaxUSDPerDay: cfg.Budget.MaxUSDPerDay,
		}, fixedNow),
		PriceTable: prices,
		Journal:    jsonllog.NewWriter(baseline),
		Now:        fixedNow,
	}

	runner, err := evalrunner.New(cfg, deps)
	if err != nil {
		return fmt.Errorf("construct runner: %w", err)
	}
	res, err := runner.Run(context.Background(), bank, outputs)
	if err != nil {
		return fmt.Errorf("recording run: %w", err)
	}
	if res.Stop != evalrunner.StopCompleted {
		return fmt.Errorf("recording run stopped with cause %q (want %q) — refusing to commit a partial cassette", res.Stop, evalrunner.StopCompleted)
	}

	if err := cassette.Sync(); err != nil {
		return fmt.Errorf("sync cassette: %w", err)
	}
	if err := baseline.Sync(); err != nil {
		return fmt.Errorf("sync baseline: %w", err)
	}

	fmt.Printf("gen-cassettes: recorded %d judge calls (K=%d probes + %d scenarios) → %s; baseline %d records → %s\n",
		mock.CallCount(), cfg.DeterminismK, res.ScenarioCount, cassettePath, len(res.Records), baselinePath)
	return nil
}

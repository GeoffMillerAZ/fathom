// H5 Stage-1 set-B integration tests: they drive the evalrunner surface
// end-to-end in replay/mock mode over the COMMITTED corpus (testdata/), with
// no network and no provider keys. Unit-level behavior is pinned by the
// in-package tests (evalrunner_test.go); this file pins the committed-corpus
// contracts instead: the D6 corpus shape, the provenance gate through the
// runner's own bank-loading/config path, corpus-wide budget/decisive
// semantics, the LoopConfig fitness canaries with dependencies genuinely
// absent, the regression gate over real runs, the replay cassette's ordinal
// layout, and replay byte-stability against the committed baseline.
package evalrunner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
	modelcatalog "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/model-catalog"
	llmreplay "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/replay"
	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
	evaljournal "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-journal"
	fitnessfunctionregistry "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry"
	scenariobank "github.com/geoffmilleraz/agentic-kit/foundations/quality/scenario-bank"
	evalrunner "github.com/geoffmilleraz/agentic-kit/kits/eval-runner"
)

// corpusFixedNow matches cmd/eval-runner and gen-cassettes exactly —
// replay-mode byte-stability depends on every party sharing one injected
// instant.
func corpusFixedNow() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }

const (
	corpusScenarioCount = 6
	corpusDeterminismK  = 3
)

type corpusOKPinger struct{}

func (corpusOKPinger) Ping(ctx context.Context) error { return ctx.Err() }

// corpusVerdictJSON renders the structured judge output the LLMJudge schema
// expects.
func corpusVerdictJSON(t *testing.T, verdict string, score float64, rationale string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"verdict":   verdict,
		"score":     score,
		"rationale": rationale,
	})
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}
	return string(b)
}

// corpusPassEntry is one scripted pass response with a non-empty rationale.
func corpusPassEntry(t *testing.T) llmclient.MockEntry {
	t.Helper()
	return llmclient.MockEntryOK(corpusVerdictJSON(t, "pass", 0.9,
		"Output satisfies the scenario rubric: citations, labeling, and grounding rules are followed."))
}

// corpusMockJudge scripts n identical pass responses.
func corpusMockJudge(t *testing.T, n int) *llmclient.MockClient {
	t.Helper()
	entries := make([]llmclient.MockEntry, 0, n)
	for i := 0; i < n; i++ {
		entries = append(entries, corpusPassEntry(t))
	}
	return llmclient.NewMockClient(entries...)
}

func corpusCatalog(t *testing.T) modelcatalog.Catalog {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "modelcatalog.json"))
	if err != nil {
		t.Fatalf("read fixture catalog: %v", err)
	}
	c, err := modelcatalog.Load(raw)
	if err != nil {
		t.Fatalf("load fixture catalog: %v", err)
	}
	return c
}

func corpusPrices(t *testing.T) costtracker.PriceTable {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "pricetable.json"))
	if err != nil {
		t.Fatalf("read fixture price table: %v", err)
	}
	var p costtracker.PriceTable
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode fixture price table: %v", err)
	}
	return p
}

// corpusBank loads the committed corpus through the runner's own
// bank-loading path.
func corpusBank(t *testing.T) scenariobank.Bank {
	t.Helper()
	bank, err := evalrunner.LoadBank(filepath.Join("testdata", "scenarios.json"))
	if err != nil {
		t.Fatalf("LoadBank(testdata/scenarios.json): %v", err)
	}
	return bank
}

func corpusOutputs(t *testing.T) []evalrunner.Output {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "outputs.json"))
	if err != nil {
		t.Fatalf("read outputs fixture: %v", err)
	}
	var wire []struct {
		ScenarioID string `json:"scenario_id"`
		Text       string `json:"text"`
		Producer   struct {
			Model      string `json:"model"`
			PromptHash string `json:"prompt_hash"`
			ParamsHash string `json:"params_hash"`
		} `json:"producer"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("decode outputs fixture: %v", err)
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
	return outputs
}

// corpusConfig is the independence-passing replay-shaped config the
// committed corpus runs with (mirrors cmd/eval-runner). BaselinePath is left
// empty; replayRun sets it explicitly so mock-judge tests do not gate
// against the committed replay baseline.
func corpusConfig() evalrunner.Config {
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
		ScenarioBankPath: filepath.Join("testdata", "scenarios.json"),
		PriceTableSource: filepath.Join("testdata", "pricetable.json"),
		Budget: evalrunner.BudgetLimits{
			MaxReqPerMin: 1000,
			MaxReqPerDay: 1000,
			MaxUSDPerDay: 10,
		},
		DeterminismK: corpusDeterminismK,
	}
}

// corpusDeps builds Deps for the given judge client and journal, with the
// guard constructed from cfg.Budget (the composition-root duty).
func corpusDeps(t *testing.T, cfg evalrunner.Config, judge llmclient.LLMClient, journal *jsonllog.Writer) evalrunner.Deps {
	t.Helper()
	return evalrunner.Deps{
		JudgeClients: map[string]llmclient.LLMClient{"judge-a": judge},
		Pingers:      map[string]llmclient.Pinger{"judge-a": corpusOKPinger{}},
		Catalog:      corpusCatalog(t),
		Guard: budgetguard.New(budgetguard.Config{
			MaxReqPerMin: cfg.Budget.MaxReqPerMin,
			MaxReqPerDay: cfg.Budget.MaxReqPerDay,
			MaxUSDPerDay: cfg.Budget.MaxUSDPerDay,
		}, corpusFixedNow),
		PriceTable: corpusPrices(t),
		Journal:    journal,
		Now:        corpusFixedNow,
	}
}

// recordsByScenario indexes journal records by scenario ID (last write wins,
// matching diff-reporter semantics).
func recordsByScenario(recs []evaljournal.Record) map[string]evaljournal.Record {
	out := make(map[string]evaljournal.Record, len(recs))
	for _, r := range recs {
		out[r.ScenarioID] = r
	}
	return out
}

// TestCorpusLoadsAndValidates pins the committed D6 corpus: it loads through
// the runner's own bank-loading path, carries the expected scenario set with
// the expected provenance mix, and every scenario has a matching
// caller-supplied output from the configured producer.
func TestCorpusLoadsAndValidates(t *testing.T) {
	bank := corpusBank(t)
	all := bank.All()
	if len(all) != corpusScenarioCount {
		t.Fatalf("corpus has %d scenarios, want %d", len(all), corpusScenarioCount)
	}

	wantIDs := []string{
		"tl-chat-citation-discipline",
		"tl-chat-reasoning-label",
		"tl-chat-insufficient-evidence",
		"tl-chat-injection-fence",
		"agentcli-word-count-answer",
		"agentcli-tool-loop-order",
	}
	for i, id := range wantIDs {
		if all[i].ID != id {
			t.Errorf("scenario[%d].ID = %q, want %q (bank order is a corpus contract — the cassette layout depends on it)", i, all[i].ID, id)
		}
	}

	redacted := bank.Filter(scenariobank.FilterOptions{Provenance: scenariobank.ProductionDerivedRedacted})
	if len(redacted) != 3 {
		t.Errorf("production-derived-redacted scenarios = %d, want 3 (the throughline-derived set)", len(redacted))
	}
	for _, s := range bank.Filter(scenariobank.FilterOptions{Provenance: scenariobank.SyntheticAdversarial}) {
		if s.DecisiveEligible {
			t.Errorf("synthetic-adversarial scenario %q is decisive-eligible — canaries are never ground truth", s.ID)
		}
	}

	outputs := corpusOutputs(t)
	if len(outputs) != len(all) {
		t.Fatalf("outputs fixture has %d entries, want %d (one per scenario)", len(outputs), len(all))
	}
	for i, s := range all {
		if outputs[i].ScenarioID != s.ID {
			t.Errorf("outputs[%d].ScenarioID = %q, want %q", i, outputs[i].ScenarioID, s.ID)
		}
		if outputs[i].Text == "" {
			t.Errorf("outputs[%d] (%s) has empty text", i, s.ID)
		}
		if outputs[i].Producer.Model != "example-producer-1" {
			t.Errorf("outputs[%d].Producer.Model = %q, want example-producer-1", i, outputs[i].Producer.Model)
		}
	}
}

// TestBankPathRefusesProductionCaptured is the provenance-gate canary: a
// production-captured scenario fed through the RUNNER'S bank-loading path
// (LoadBank) AND through its config path (New with Config.ScenarioBankPath)
// must be refused. Calling scenariobank.Scenario.Validate directly proves
// nothing about kit wiring — that check is unconditional in the foundation.
func TestBankPathRefusesProductionCaptured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenarios.json")
	captured := `[
	  {
	    "id": "leaked-prod-scenario",
	    "version": 1,
	    "provenance": "production-captured",
	    "decisive_eligible": false,
	    "input": {"question": "raw production content"}
	  }
	]`
	if err := os.WriteFile(path, []byte(captured), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := evalrunner.LoadBank(path); err == nil {
		t.Fatal("LoadBank accepted a production-captured scenario; the runner's bank-loading path must refuse it (PII hazard)")
	}

	cfg := corpusConfig()
	cfg.ScenarioBankPath = path
	var journal bytes.Buffer
	if _, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal))); err == nil {
		t.Fatal("New accepted a config whose scenario bank source contains a production-captured scenario")
	}
}

// TestBudgetExhaustionJournalsErrorVerdicts (corpus-wide): when the budget
// runs out mid-run, EVERY scenario still journals a verdict:"error" record —
// never a missing record — the run reports the typed StopBudget cause, and
// the returned error satisfies errors.Is(_, llminstrument.ErrBudgetExceeded)
// so a truncated run can never read as green.
func TestBudgetExhaustionJournalsErrorVerdicts(t *testing.T) {
	cfg := corpusConfig()
	// The K determinism-probe calls fit the daily request budget exactly;
	// every main-loop judge call is denied by the guard.
	cfg.Budget.MaxReqPerDay = corpusDeterminismK

	var journal bytes.Buffer
	deps := corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal))
	runner, err := evalrunner.New(cfg, deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t))
	if err == nil {
		t.Fatal("budget-exhausted run returned nil error; want a run-level typed stop")
	}
	if !errors.Is(err, llminstrument.ErrBudgetExceeded) {
		t.Fatalf("budget-exhausted run error = %v, want errors.Is llminstrument.ErrBudgetExceeded", err)
	}
	if res.Stop != evalrunner.StopBudget {
		t.Fatalf("Result.Stop = %q, want %q", res.Stop, evalrunner.StopBudget)
	}

	recs, err := evaljournal.Load(bytes.NewReader(journal.Bytes()))
	if err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if len(recs) != corpusScenarioCount {
		t.Fatalf("journal has %d records, want %d — budget exhaustion must journal error records, not drop them", len(recs), corpusScenarioCount)
	}
	for _, r := range recs {
		if r.Verdict != evaljournal.VerdictError {
			t.Errorf("scenario %s journaled verdict %q under budget exhaustion, want %q", r.ScenarioID, r.Verdict, evaljournal.VerdictError)
		}
		if r.Decisive {
			t.Errorf("scenario %s: budget-error record marked decisive", r.ScenarioID)
		}
	}
}

// TestEmptyRationaleNeverDecisive: the evaljournal invariant (empty
// rationale ⇒ Decisive=false) holds through the runner over the committed
// corpus — and a synthetic-adversarial scenario is never decisive even with
// a rationale.
func TestEmptyRationaleNeverDecisive(t *testing.T) {
	entries := make([]llmclient.MockEntry, 0, corpusDeterminismK+corpusScenarioCount)
	for i := 0; i < corpusDeterminismK; i++ { // determinism-probe calls
		entries = append(entries, corpusPassEntry(t))
	}
	// Main-loop calls in bank order:
	entries = append(
		entries,
		corpusPassEntry(t), // tl-chat-citation-discipline: rationale, decisive-eligible → decisive
		llmclient.MockEntryOK(corpusVerdictJSON(t, "pass", 0.9, "")), // tl-chat-reasoning-label: EMPTY rationale
		corpusPassEntry(t), // tl-chat-insufficient-evidence
		corpusPassEntry(t), // tl-chat-injection-fence (synthetic-adversarial)
		corpusPassEntry(t), // agentcli-word-count-answer
		corpusPassEntry(t), // agentcli-tool-loop-order
	)

	cfg := corpusConfig()
	var journal bytes.Buffer
	deps := corpusDeps(t, cfg, llmclient.NewMockClient(entries...), jsonllog.NewWriter(&journal))
	runner, err := evalrunner.New(cfg, deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != evalrunner.StopCompleted {
		t.Fatalf("Result.Stop = %q, want %q", res.Stop, evalrunner.StopCompleted)
	}

	byID := recordsByScenario(res.Records)
	if r := byID["tl-chat-citation-discipline"]; !r.Decisive {
		t.Error("decisive-eligible scenario with rationale and validated independence journaled Decisive=false")
	}
	if r := byID["tl-chat-reasoning-label"]; r.Decisive {
		t.Error("empty-rationale verdict journaled Decisive=true — the evaljournal invariant was bypassed")
	}
	if r := byID["tl-chat-injection-fence"]; r.Decisive {
		t.Error("synthetic-adversarial scenario journaled Decisive=true")
	}
}

// fitnessByName runs the runner's Fitness (NewRegistry + Register +
// ExecuteAll over the live LoopConfig — never a hand-built literal) and
// indexes the results by function name.
func fitnessByName(t *testing.T, runner *evalrunner.Runner) map[string]fitnessfunctionregistry.ExecutionResult {
	t.Helper()
	results, err := runner.Fitness("loop-config-canary", corpusFixedNow())
	if err != nil {
		t.Fatalf("Fitness: %v", err)
	}
	out := make(map[string]fitnessfunctionregistry.ExecutionResult, len(results))
	for _, res := range results {
		out[res.FunctionName] = res
	}
	return out
}

// TestLoopConfigFitnessCanaries proves each EvalLoopConfig field is computed
// from actual wiring state: the runner is built via New with the dependency
// genuinely absent, and the corresponding eval.* fitness function must fail
// in ExecuteAll output. A fully wired runner is the positive control, so a
// fitness function failing for the wrong reason is caught.
func TestLoopConfigFitnessCanaries(t *testing.T) {
	t.Run("fully wired and run runner passes all four", func(t *testing.T) {
		cfg := corpusConfig()
		var journal bytes.Buffer
		runner, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal)))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// ProvenanceGatesOn and DeterminismProbed are run-observed events:
		// the positive control must actually run the loop.
		if _, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t)); err != nil {
			t.Fatalf("Run: %v", err)
		}
		for name, res := range fitnessByName(t, runner) {
			if !res.Passed {
				t.Errorf("%s failed on a fully wired, fully run runner (failures: %v)", name, res.Failures)
			}
		}
	})

	t.Run("nil journal writer fails eval.journal-attached", func(t *testing.T) {
		cfg := corpusConfig()
		deps := corpusDeps(t, cfg, corpusMockJudge(t, 16), nil) // journal genuinely absent
		runner, err := evalrunner.New(cfg, deps)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t)); !errors.Is(err, evalrunner.ErrNoJournal) {
			t.Errorf("Run without a journal returned %v, want errors.Is ErrNoJournal", err)
		}
		if res := fitnessByName(t, runner)["eval.journal-attached"]; res.Passed {
			t.Fatal("eval.journal-attached passed with no journal writer wired — LoopConfig is lying about the wiring")
		}
	})

	t.Run("K below 3 fails eval.determinism-probed", func(t *testing.T) {
		cfg := corpusConfig()
		cfg.DeterminismK = 1 // probe genuinely absent (K<3 never probes)
		var journal bytes.Buffer
		runner, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal)))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		res, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.Probe != nil {
			t.Error("Result.Probe is non-nil with K=1 — K<3 must skip the probe entirely")
		}
		if res.Stable {
			t.Error("Result.Stable = true without a K>=3 probe — unprobed is not stable")
		}
		if fres := fitnessByName(t, runner)["eval.determinism-probed"]; fres.Passed {
			t.Fatal("eval.determinism-probed passed with K=1 — LoopConfig is lying about the wiring")
		}
	})

	t.Run("violating catalog is construction-fatal", func(t *testing.T) {
		cfg := corpusConfig()
		cfg.JudgeLanes = []evalrunner.JudgeLane{
			{Name: "judge-a", Model: "example-judge-same-family", Family: "alpha"},
		}
		var journal bytes.Buffer
		runner, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal)))
		if !errors.Is(err, evalrunner.ErrIndependenceViolated) {
			t.Fatalf("New with a shared-family judge/producer pair returned %v, want errors.Is ErrIndependenceViolated", err)
		}
		if runner != nil {
			// Construction-fatal means no runner exists whose LoopConfig
			// could claim independence; a returned runner must not claim it.
			if fres := fitnessByName(t, runner)["eval.independence-validated"]; fres.Passed {
				t.Fatal("eval.independence-validated passed despite a shared_family violation")
			}
		}
	})

	t.Run("absent bank source fails eval.provenance-gates-on", func(t *testing.T) {
		cfg := corpusConfig()
		cfg.ScenarioBankPath = "" // no validated bank source declared
		var journal bytes.Buffer
		runner, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&journal)))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// No Run either: no bank has passed the runner's provenance
		// validation, so the gate cannot honestly be reported on.
		if res := fitnessByName(t, runner)["eval.provenance-gates-on"]; res.Passed {
			t.Fatal("eval.provenance-gates-on passed before any bank was validated through the runner's path")
		}
	})
}

// TestRegressionGateFailsOnVerdictFlip: a verdict flip against a baseline
// journal produced by a real run is always a regression, and the typed
// RegressionResult reports the failure (this is what makes `task eval` exit
// non-zero on regressions).
func TestRegressionGateFailsOnVerdictFlip(t *testing.T) {
	// Baseline run: everything passes.
	cfg := corpusConfig()
	var baseline bytes.Buffer
	baseRunner, err := evalrunner.New(cfg, corpusDeps(t, cfg, corpusMockJudge(t, 16), jsonllog.NewWriter(&baseline)))
	if err != nil {
		t.Fatalf("New (baseline run): %v", err)
	}
	if _, err := baseRunner.Run(context.Background(), corpusBank(t), corpusOutputs(t)); err != nil {
		t.Fatalf("baseline Run: %v", err)
	}

	// Candidate run: the first scenario's verdict flips to fail.
	entries := make([]llmclient.MockEntry, 0, corpusDeterminismK+corpusScenarioCount)
	for i := 0; i < corpusDeterminismK; i++ {
		entries = append(entries, corpusPassEntry(t))
	}
	entries = append(entries, llmclient.MockEntryOK(corpusVerdictJSON(t, "fail", 0.1,
		"Output omits the required [span:ID] citation.")))
	for i := 1; i < corpusScenarioCount; i++ {
		entries = append(entries, corpusPassEntry(t))
	}

	var journal bytes.Buffer
	candRunner, err := evalrunner.New(cfg, corpusDeps(t, cfg, llmclient.NewMockClient(entries...), jsonllog.NewWriter(&journal)))
	if err != nil {
		t.Fatalf("New (candidate run): %v", err)
	}
	res, err := candRunner.Run(context.Background(), corpusBank(t), corpusOutputs(t))
	if err != nil {
		t.Fatalf("candidate Run: %v", err)
	}

	reg, err := candRunner.Regression(bytes.NewReader(baseline.Bytes()), res.Records)
	if err != nil {
		t.Fatalf("Regression: %v", err)
	}
	if reg.Passed {
		t.Fatal("regression gate passed despite a verdict flip against the baseline")
	}
	if reg.Delta.Regressions == 0 {
		t.Error("Delta.Regressions == 0 despite a verdict flip (flips are always regressions)")
	}
	if len(reg.Delta.VerdictFlips) == 0 {
		t.Error("Delta.VerdictFlips is empty despite a pass→fail flip")
	}
}

// replayRun runs the runner once over the committed cassette + corpus with
// the fixed clock, asserts the run completes with the regression gate green
// against the committed baseline, and returns the journal bytes.
func replayRun(t *testing.T) []byte {
	t.Helper()
	entries, err := llmreplay.LoadFile(filepath.Join("testdata", "cassettes", "judge.jsonl"))
	if err != nil {
		t.Fatalf("load committed cassette (regenerate with `task eval-record`): %v", err)
	}
	client := llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash())

	cfg := corpusConfig()
	cfg.BaselinePath = filepath.Join("testdata", "baseline.jsonl")
	var journal bytes.Buffer
	runner, err := evalrunner.New(cfg, corpusDeps(t, cfg, client, jsonllog.NewWriter(&journal)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := runner.Run(context.Background(), corpusBank(t), corpusOutputs(t))
	if err != nil {
		t.Fatalf("replay Run: %v", err)
	}
	if res.Stop != evalrunner.StopCompleted {
		t.Fatalf("replay Result.Stop = %q, want %q", res.Stop, evalrunner.StopCompleted)
	}
	if !res.Stable {
		t.Error("replay run is not Stable — the committed cassette's probe entries should classify stable")
	}

	baseline, err := os.Open(cfg.BaselinePath)
	if err != nil {
		t.Fatalf("open committed baseline (regenerate with `task eval-record`): %v", err)
	}
	defer baseline.Close()
	reg, err := runner.Regression(baseline, res.Records)
	if err != nil {
		t.Fatalf("Regression: %v", err)
	}
	if !reg.Passed {
		t.Fatalf("replay run regressed against its own committed baseline: %+v", reg.Delta)
	}
	return journal.Bytes()
}

// TestReplayRunByteStable is the H5 conformance check: two Run invocations
// over the committed corpus with the same fixed clock produce byte-identical
// journals — and both equal the committed baseline, which was recorded
// through the same path with the same clock. Replay-green ≠ live-green: this
// proves reproducibility, never judge correctness.
func TestReplayRunByteStable(t *testing.T) {
	first := replayRun(t)
	second := replayRun(t)
	if len(first) == 0 {
		t.Fatal("replay run journaled zero bytes")
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("two replay runs over the same cassette + clock differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	baseline, err := os.ReadFile(filepath.Join("testdata", "baseline.jsonl"))
	if err != nil {
		t.Fatalf("read committed baseline: %v", err)
	}
	if !bytes.Equal(first, baseline) {
		t.Error("replay journal differs from the committed baseline — regenerate both with `task eval-record`")
	}
}

// TestCassetteProbeLayout pins the committed cassette's ordinal layout: the
// DeterminismK probe entries come first (one identical request repeated K
// times, in scheduling order), followed by one judge entry per scenario in
// bank order, each with its own distinct request. Replay consumes entries
// ordinally, so this layout IS the replay contract.
func TestCassetteProbeLayout(t *testing.T) {
	entries, err := llmreplay.LoadFile(filepath.Join("testdata", "cassettes", "judge.jsonl"))
	if err != nil {
		t.Fatalf("load committed cassette (regenerate with `task eval-record`): %v", err)
	}
	want := corpusDeterminismK + corpusScenarioCount
	if len(entries) != want {
		t.Fatalf("cassette has %d entries, want %d (K=%d probes + %d scenarios)", len(entries), want, corpusDeterminismK, corpusScenarioCount)
	}

	hashes := make([]string, len(entries))
	for i, e := range entries {
		h, err := llmreplay.RequestHash(e.Request)
		if err != nil {
			t.Fatalf("hash entry %d: %v", i, err)
		}
		hashes[i] = h
	}

	probeHash := hashes[0]
	for i := 1; i < corpusDeterminismK; i++ {
		if hashes[i] != probeHash {
			t.Errorf("probe entry %d hash %s differs from entry 0 %s — the K probe calls must repeat one request", i, hashes[i], probeHash)
		}
	}
	seen := map[string]int{}
	for i := corpusDeterminismK; i < len(entries); i++ {
		if hashes[i] == probeHash {
			t.Errorf("scenario entry %d repeats the probe request — probe calls must precede, not interleave with, scenario judging", i)
		}
		if j, dup := seen[hashes[i]]; dup {
			t.Errorf("scenario entries %d and %d share a request hash — each scenario must be judged with its own request", j, i)
		}
		seen[hashes[i]] = i
	}
}

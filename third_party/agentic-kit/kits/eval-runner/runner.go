package evalrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	costtracker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/instrument"
	modelcatalog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/model-catalog"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
	determinismgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/determinism-gate"
	diffreporter "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/diff-reporter"
	evalgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
	evaljournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-journal"
	fitnessfunctionregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	qaharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
	reliabilitystats "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
	scenariobank "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/scenario-bank"
)

// Typed refusals and stops, matched with errors.Is.
var (
	// ErrEmptyBank: the bank holds zero scenarios. A run over nothing can
	// never exit 0 by construction.
	ErrEmptyBank = errors.New("evalrunner: scenario bank is empty")

	// ErrNoJournal: Deps.Journal is nil. Verdicts that cannot be journaled
	// are not rendered at all.
	ErrNoJournal = errors.New("evalrunner: no eval-journal writer attached")

	// ErrJudgeStackUnavailable: the startup judge-stack probe failed for at
	// least one lane; the run aborts before the first scenario.
	ErrJudgeStackUnavailable = errors.New("evalrunner: judge stack unavailable")

	// ErrIndependenceViolated: modelcatalog.ValidateIndependence reported
	// violations and Config.NonDecisive is false — construction-fatal.
	ErrIndependenceViolated = errors.New("evalrunner: judge/producer independence violated")

	// ErrMissingOutput: a bank scenario has no matching Output.
	ErrMissingOutput = errors.New("evalrunner: scenario has no matching output")

	// ErrUnexpectedOutput: an Output has an empty ScenarioID or names a
	// scenario outside the supplied bank.
	ErrUnexpectedOutput = errors.New("evalrunner: output does not match a scenario in the bank")

	// ErrDuplicateOutput: more than one Output names the same bank scenario.
	ErrDuplicateOutput = errors.New("evalrunner: multiple outputs match the same scenario")

	// ErrProducerMismatch: an Output's Producer differs from Config.Producer.
	// Outputs must match the configured model, prompt and parameter identities;
	// construction-time family/session checks alone do not bind generated data.
	ErrProducerMismatch = errors.New("evalrunner: output producer does not match configured producer")
)

// lane is one wired judge lane: its config plus the budget-wrapped client
// and optional pinger.
type lane struct {
	cfg    JudgeLane
	client llmclient.LLMClient // budget-wrapped at New
	pinger llmclient.Pinger    // nil = not probed (stated non-guarantee)
}

// Runner is the batch eval runner. Construct with New; the zero value is
// not ready for use.
//
// Concurrency: safe for concurrent use — construction state is immutable
// and the observed-wiring flag is mutex-guarded. Each Run is internally
// sequential (cassette-deterministic ordering); see the package doc.
type Runner struct {
	cfg     Config
	lanes   []lane
	journal *jsonllog.Writer
	now     func() time.Time

	independenceOK bool
	violations     []modelcatalog.Violation

	mu                  sync.Mutex // guards the observed-wiring flags below
	provenanceValidated bool       // set when New or Run has validated a full bank
	determinismProbed   bool       // set when a K>=3 probe completed in Run
}

// New constructs a Runner from data-shaped cfg and behavioral deps.
//
// Construction-time honesty checks, in order:
//
//  1. cfg.Validate() — the CUE-mirrored data invariants.
//  2. Deps completeness — a client per lane, a non-nil budget guard.
//     Deps.Journal MAY be nil at construction (Run refuses with
//     ErrNoJournal; the eval.journal-attached fitness function observes
//     the absence) so wiring gaps are visible rather than unconstructable.
//     When Config.ScenarioBankPath is non-empty, New loads and validates
//     the bank through LoadBank — a bad or production-captured source is
//     construction-fatal, and success arms eval.provenance-gates-on.
//  3. Family drift — each lane's configured Family must match the
//     catalog's entry for its Model (when the catalog knows the model).
//  4. Independence — modelcatalog.ValidateIndependence for every
//     (judge lane, producer) pair. Any violation is fatal
//     (ErrIndependenceViolated) unless cfg.NonDecisive is true, in which
//     case the runner constructs, records the violations, and never marks
//     any record decisive.
//
// Every lane client is wrapped with llminstrument.WithBudget(deps.Guard,
// deps.PriceTable, deps.Estimator, defaultModel): the pre-charge is
// fail-closed — a denied charge means the inner client is never called.
func New(cfg Config, deps Deps) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if deps.Guard == nil {
		return nil, fmt.Errorf("evalrunner: New: Deps.Guard must not be nil (budget-charged judging is not optional)")
	}

	// Provenance gate at construction: a declared bank source must load
	// and validate through the runner's own bank path.
	provenanceValidated := false
	if cfg.ScenarioBankPath != "" {
		if _, err := LoadBank(cfg.ScenarioBankPath); err != nil {
			return nil, fmt.Errorf("evalrunner: New: scenario bank source: %w", err)
		}
		provenanceValidated = true
	}

	now := deps.Now
	if now == nil {
		now = time.Now // documented default for this seam
	}
	est := costtracker.EstimatorOrDefault(deps.Estimator)

	var violations []modelcatalog.Violation
	lanes := make([]lane, 0, len(cfg.JudgeLanes))
	for _, jl := range cfg.JudgeLanes {
		base, ok := deps.JudgeClients[jl.Name]
		if !ok || base == nil {
			return nil, fmt.Errorf("evalrunner: New: Deps.JudgeClients has no client for lane %q", jl.Name)
		}

		// Family drift: the config's family claim must agree with the
		// catalog when the catalog knows the model.
		if entry, known := deps.Catalog.Entry(jl.Model); known && entry.Family != jl.Family {
			return nil, fmt.Errorf(
				"evalrunner: New: lane %q declares family %q but the catalog records %q for model %q",
				jl.Name, jl.Family, entry.Family, jl.Model,
			)
		}

		vs := modelcatalog.ValidateIndependence(jl.Model, cfg.Producer.Model, deps.Catalog, modelcatalog.IndependenceOpts{
			SameSession:   cfg.Independence.SameSession,
			SharedHistory: cfg.Independence.SharedHistory,
		})
		violations = append(violations, vs...)

		var p llmclient.Pinger
		if deps.Pingers != nil {
			p = deps.Pingers[jl.Name]
		}
		if p == nil {
			if bp, isPinger := base.(llmclient.Pinger); isPinger {
				p = bp
			}
		}

		defaultModel := deps.DefaultModel
		if defaultModel == "" {
			defaultModel = jl.Model
		}
		lanes = append(lanes, lane{
			cfg:    jl,
			client: llminstrument.WithBudget(base, deps.Guard, deps.PriceTable, est, defaultModel),
			pinger: p,
		})
	}

	if len(violations) > 0 && !cfg.NonDecisive {
		details := make([]string, len(violations))
		for i, v := range violations {
			details[i] = v.Kind + ": " + v.Detail
		}
		return nil, fmt.Errorf("%w: %s (set Config.NonDecisive to construct a never-decisive runner)",
			ErrIndependenceViolated, strings.Join(details, "; "))
	}

	return &Runner{
		cfg:                 cfg,
		lanes:               lanes,
		journal:             deps.Journal,
		now:                 now,
		independenceOK:      len(violations) == 0,
		violations:          violations,
		provenanceValidated: provenanceValidated,
	}, nil
}

// IndependenceViolations returns the violations recorded at construction
// (non-empty only for NonDecisive runners). The slice is a defensive copy.
func (r *Runner) IndependenceViolations() []modelcatalog.Violation {
	out := make([]modelcatalog.Violation, len(r.violations))
	copy(out, r.violations)
	return out
}

// LoadBank is the runner's bank-loading path: it reads the JSON file at
// path and validates every scenario via scenariobank.Load. Provenance
// gating rides that foundation — a production-captured scenario is refused
// here, before any Runner exists. New routes Config.ScenarioBankPath
// through this same function.
func LoadBank(path string) (scenariobank.Bank, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return scenariobank.Bank{}, fmt.Errorf("evalrunner: LoadBank: %w", err)
	}
	return scenariobank.Load(data)
}

// Run judges every (scenario, judge lane) pair: scenarios in bank order,
// lanes in config order, strictly sequentially (ordinal cassette replay
// depends on this). outputs are the caller-supplied, pre-produced outputs
// under judgment — exactly one per scenario.
//
// Order of operations: journal presence → bank non-empty → scenario
// re-validation (provenance gates) → exact output binding and producer match →
// judge-stack probe (llmclient.ProbeAll) → determinism probe
// (determinismgate.Probe, when Config.DeterminismK >= 3; budget-charged
// but NOT journaled, so cassettes carry the K probe entries first) → the
// main judging loop. The regression gate is a separate call — see
// Regression.
//
// Every main-loop judge call is appended to the journal as an
// evaljournal.Record — including failed calls, as verdict:"error" records.
// After a budget denial the remaining pairs are journaled as error records
// without calling the judge, Result.Stop is StopBudget, and Run returns a
// run-level typed error satisfying
// errors.Is(err, llminstrument.ErrBudgetExceeded) — never a silent
// truncation, never missing records. Decisive is true only when
// independence validated AND !Config.NonDecisive AND
// Scenario.DecisiveEligible AND the verdict is pass/fail/absent_proven
// with a non-empty rationale.
func (r *Runner) Run(ctx context.Context, bank scenariobank.Bank, outputs []Output) (Result, error) {
	res := Result{Stop: StopError}

	if r.journal == nil {
		return res, ErrNoJournal
	}

	scenarios := bank.All()
	if len(scenarios) == 0 {
		return res, ErrEmptyBank
	}
	res.ScenarioCount = len(scenarios)

	// Provenance gates: re-validate every scenario on the run path, so a
	// bank that dodged validation still cannot run.
	for i, sc := range scenarios {
		if err := sc.Validate(); err != nil {
			return res, fmt.Errorf("evalrunner: Run: scenario[%d]: %w", i, err)
		}
	}
	r.mu.Lock()
	r.provenanceValidated = true
	r.mu.Unlock()

	// Output binding is a side-effect-free preflight: every output must name
	// exactly one bank scenario, every bank scenario needs exactly one output,
	// and the producer must equal the producer whose independence was checked at
	// construction. Do this before pinging, judging, or appending a journal
	// record, so malformed caller input cannot consume a cassette entry or look
	// like an attempted evaluation.
	expected := make(map[string]struct{}, len(scenarios))
	for _, sc := range scenarios {
		expected[sc.ID] = struct{}{}
	}
	byScenario := make(map[string]Output, len(outputs))
	for i, o := range outputs {
		if o.ScenarioID == "" {
			return res, fmt.Errorf("%w: outputs[%d] has an empty ScenarioID", ErrUnexpectedOutput, i)
		}
		if _, ok := expected[o.ScenarioID]; !ok {
			return res, fmt.Errorf("%w: outputs[%d] names scenario %q", ErrUnexpectedOutput, i, o.ScenarioID)
		}
		if _, dup := byScenario[o.ScenarioID]; dup {
			return res, fmt.Errorf("%w: outputs[%d] names scenario %q", ErrDuplicateOutput, i, o.ScenarioID)
		}
		byScenario[o.ScenarioID] = o
	}
	for _, sc := range scenarios {
		if _, ok := byScenario[sc.ID]; !ok {
			return res, fmt.Errorf("%w: scenario %q", ErrMissingOutput, sc.ID)
		}
	}
	for i, o := range outputs {
		if o.Producer != r.cfg.Producer {
			return res, fmt.Errorf("%w: outputs[%d] producer %+v, config declares %+v",
				ErrProducerMismatch, i, o.Producer, r.cfg.Producer)
		}
	}

	// Startup judge-stack probe: every lane with a pinger, before the
	// first scenario. Any failure aborts the run.
	pingers := make(map[string]llmclient.Pinger)
	for _, ln := range r.lanes {
		if ln.pinger != nil {
			pingers[ln.cfg.Name] = ln.pinger
		}
	}
	if failures := llmclient.ProbeAll(ctx, pingers); len(failures) > 0 {
		names := make([]string, 0, len(failures))
		for name := range failures {
			names = append(names, name)
		}
		sort.Strings(names)
		details := make([]string, len(names))
		for i, name := range names {
			details[i] = fmt.Sprintf("%s: %v", name, failures[name])
		}
		return res, fmt.Errorf("%w: %s", ErrJudgeStackUnavailable, strings.Join(details, "; "))
	}

	rec := &recorder{runner: r, scenarios: scenarios, outputs: byScenario}

	// Determinism probe: K calls on the FIRST lane against the FIRST
	// scenario, budget-charged (the wrapped client) but journal-silent
	// (nil sink) so probe calls never pollute the regression baseline.
	if r.cfg.DeterminismK >= 3 {
		probeJudge, err := NewLLMJudge(r.lanes[0].client, LLMJudgeOptions{
			Model:     r.lanes[0].cfg.Model,
			System:    r.cfg.JudgeSystem,
			MaxTokens: r.cfg.JudgeMaxTokens,
		})
		if err != nil {
			return res, fmt.Errorf("evalrunner: Run: determinism probe judge: %w", err)
		}
		pr, err := determinismgate.Probe(ctx, probeJudge, scenarios[0], r.cfg.DeterminismK,
			determinismgate.ProbeOptions{StabilityBand: r.cfg.StabilityBand})
		if err != nil {
			return res, fmt.Errorf("evalrunner: Run: determinism probe: %w", err)
		}
		res.Probe = &pr
		res.Stable = pr.Stability == determinismgate.StabilityStable
		r.mu.Lock()
		r.determinismProbed = true
		r.mu.Unlock()
	}

	// Per-lane judges wired to the journaling sink.
	judges := make([]*LLMJudge, len(r.lanes))
	for i, ln := range r.lanes {
		laneName := ln.cfg.Name
		j, err := NewLLMJudge(ln.client, LLMJudgeOptions{
			Model:     ln.cfg.Model,
			System:    r.cfg.JudgeSystem,
			MaxTokens: r.cfg.JudgeMaxTokens,
			Sink:      func(v Verdict) { rec.record(laneName, v) },
		})
		if err != nil {
			return res, fmt.Errorf("evalrunner: Run: lane %q judge: %w", laneName, err)
		}
		judges[i] = j
	}

	// Main loop: sequential by design (see package doc).
	var budgetErr error
	for _, sc := range scenarios {
		out := byScenario[sc.ID]
		item := qaharness.Item{
			ID:     sc.ID,
			Output: out.Text,
			Metadata: map[string]string{
				"scenario_input": string(sc.Input),
				"expected":       string(sc.Expected),
			},
		}
		for li, ln := range r.lanes {
			if ctx.Err() != nil {
				res.Stop = StopCanceled
				res.Records = rec.snapshot()
				return res, nil
			}
			if budgetErr != nil {
				// Never a missing record: journal the skip as an error
				// verdict without calling (and re-charging) the judge.
				rec.record(ln.cfg.Name, Verdict{
					ScenarioID: sc.ID,
					JudgeModel: ln.cfg.Model,
					Verdict:    evaljournal.VerdictError,
					Err:        "budget exceeded: judge call skipped",
				})
				continue
			}

			res.JudgeCalls++
			_, err := judges[li].Score(ctx, item)
			if err != nil && errors.Is(err, llminstrument.ErrBudgetExceeded) {
				budgetErr = err
			}
			// Non-budget errors were journaled as error verdicts by the
			// sink inside Score; the run continues to the next pair.

			if aerr := rec.appendErr(); aerr != nil {
				res.Records = rec.snapshot()
				return res, fmt.Errorf("evalrunner: Run: journal append: %w", aerr)
			}
		}
	}

	res.Records = rec.snapshot()
	if budgetErr != nil {
		// Run-level typed stop: every remaining pair was journaled as an
		// error record above; the error still surfaces so no caller can
		// mistake a truncated run for a green one.
		res.Stop = StopBudget
		return res, fmt.Errorf("evalrunner: Run: %w", budgetErr)
	}
	res.Stop = StopCompleted
	return res, nil
}

// recorder builds, journals, and collects evaljournal.Records from sink
// verdicts. Safe for concurrent use via sync.Mutex (the journal Writer has
// its own lock; this one guards the collected slice and first-error).
type recorder struct {
	runner    *Runner
	scenarios []scenariobank.Scenario
	outputs   map[string]Output

	mu      sync.Mutex
	records []evaljournal.Record
	err     error // first journal append failure
}

// record converts v into an evaljournal.Record, appends it to the journal,
// and collects it. Called from LLMJudge sinks (synchronously inside Score)
// and directly for budget-skipped pairs.
func (c *recorder) record(laneName string, v Verdict) {
	var decisiveEligible bool
	for _, sc := range c.scenarios {
		if sc.ID == v.ScenarioID {
			decisiveEligible = sc.DecisiveEligible
			break
		}
	}
	out := c.outputs[v.ScenarioID]

	r := c.runner
	decisive := v.Err == "" &&
		v.Rationale != "" &&
		decisiveEligible &&
		r.independenceOK &&
		!r.cfg.NonDecisive &&
		(v.Verdict == evaljournal.VerdictPass ||
			v.Verdict == evaljournal.VerdictFail ||
			v.Verdict == evaljournal.VerdictAbsentProven)

	rec := evaljournal.Record{
		RunID:        r.cfg.RunID + "/" + laneName,
		ScenarioID:   v.ScenarioID,
		Producer:     out.Producer.journal(),
		Judge:        evaljournal.Judge{Model: v.JudgeModel, Family: r.laneFamily(laneName)},
		Verdict:      v.Verdict,
		Decisive:     decisive,
		Score:        v.Score,
		Rationale:    v.Rationale,
		EvidenceRefs: []string{},
		OccurredAt:   r.now(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := evaljournal.Append(r.journal, rec); err != nil {
		if c.err == nil {
			c.err = err
		}
		return
	}
	c.records = append(c.records, rec)
}

// appendErr returns the first journal append failure, if any.
func (c *recorder) appendErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// snapshot returns a copy of the collected records.
func (c *recorder) snapshot() []evaljournal.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]evaljournal.Record, len(c.records))
	copy(out, c.records)
	return out
}

// laneFamily returns the configured family for a lane name.
func (r *Runner) laneFamily(laneName string) string {
	for _, ln := range r.lanes {
		if ln.cfg.Name == laneName {
			return ln.cfg.Family
		}
	}
	return ""
}

// LoopConfig reports the eval loop's honesty configuration, COMPUTED from
// observed wiring events — never from literal booleans:
//
//   - IndependenceValidated: ValidateIndependence ran at New and returned
//     zero violations across every (lane, producer) pair.
//   - ProvenanceGatesOn: a full bank has passed scenario validation — at
//     New via Config.ScenarioBankPath, or on the run path during a Run
//     (false when no bank source was declared and no Run has validated
//     one).
//   - JournalAttached: a journal writer is wired.
//   - DeterminismProbed: a Probe with K >= 3 completed in a Run (false
//     until then, and forever when Config.DeterminismK < 3).
//
// Feed the result to determinismgate.FitnessFunctions (or call Fitness,
// which does exactly that).
func (r *Runner) LoopConfig() determinismgate.EvalLoopConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return determinismgate.EvalLoopConfig{
		IndependenceValidated: r.independenceOK,
		ProvenanceGatesOn:     r.provenanceValidated,
		JournalAttached:       r.journal != nil,
		DeterminismProbed:     r.determinismProbed,
	}
}

// Fitness registers the four eval.* loop-honesty fitness functions
// (compiled from LoopConfig) in a fresh fitnessfunctionregistry.Registry
// and executes them. runID and at are passed through for a reproducible
// batch; results are sorted by function name.
func (r *Runner) Fitness(runID string, at time.Time) ([]fitnessfunctionregistry.ExecutionResult, error) {
	reg := fitnessfunctionregistry.NewRegistry()
	for _, ff := range determinismgate.FitnessFunctions(r.LoopConfig()) {
		if err := reg.Register(ff); err != nil {
			return nil, fmt.Errorf("evalrunner: Fitness: %w", err)
		}
	}
	return reg.ExecuteAll(runID, at), nil
}

// Regression runs the regression gate: load the baseline journal from
// baseline (JSONL, as written by Run), diff it against current with
// diffreporter.Diff, and evaluate one eval-gate check per compared
// (scenario, concern) pair — a check fails on a verdict flip or, when
// Config.ScoreDropThreshold > 0, a score drop of at least that fraction.
//
// The overall Passed is Gate.Passed against Config.GateThreshold (zero
// defaults to 1.0 — any regression fails). When there are no comparable
// pairs at all, Passed is true: nothing to regress (stated explicitly
// because eval-gate alone fails an empty check set).
//
// A torn baseline tail (jsonllog.ErrPartialTrailingLine) is treated as
// fatal — a regression gate must not silently compare against a truncated
// baseline.
func (r *Runner) Regression(baseline io.Reader, current []evaljournal.Record) (RegressionResult, error) {
	base, err := evaljournal.Load(baseline)
	if err != nil {
		return RegressionResult{}, fmt.Errorf("evalrunner: Regression: baseline: %w", err)
	}

	delta := diffreporter.Diff(base, current, diffreporter.DiffOptions{
		ScoreDropThreshold: r.cfg.ScoreDropThreshold,
	})

	type key struct{ scenario, concern string }
	flipped := make(map[key]bool, len(delta.VerdictFlips))
	for _, f := range delta.VerdictFlips {
		flipped[key{f.ScenarioID, f.Concern}] = true
	}
	dropped := make(map[key]bool)
	if r.cfg.ScoreDropThreshold > 0 {
		for _, d := range delta.ScoreDeltas {
			if d.Delta != nil && *d.Delta <= -r.cfg.ScoreDropThreshold {
				dropped[key{d.ScenarioID, d.Concern}] = true
			}
		}
	}

	// One check per compared pair: union of score-delta keys and flip keys,
	// in deterministic (scenario, concern) order.
	seen := make(map[key]bool)
	var keys []key
	for _, d := range delta.ScoreDeltas {
		k := key{d.ScenarioID, d.Concern}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, f := range delta.VerdictFlips {
		k := key{f.ScenarioID, f.Concern}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].scenario != keys[j].scenario {
			return keys[i].scenario < keys[j].scenario
		}
		return keys[i].concern < keys[j].concern
	})

	checks := make([]evalgate.Check, 0, len(keys))
	for _, k := range keys {
		checks = append(checks, evalgate.Check{
			Name:   k.scenario + "/" + k.concern,
			Passed: !flipped[k] && !dropped[k],
			Weight: 1,
		})
	}

	gres := evalgate.Gate{Threshold: r.cfg.GateThreshold}.Evaluate(checks)
	passed := gres.Passed
	if len(checks) == 0 {
		passed = true // nothing to regress; documented above
	}
	return RegressionResult{
		Delta:    delta,
		Gate:     gres,
		Passed:   passed,
		PassPowK: regressionPassPowK(current, regressionPassK),
	}, nil
}

// regressionPassK is the k in the per-scenario pass^k Regression reports:
// three, the smallest k at which "passes every time" starts to separate
// a reliable scenario from a lucky one (τ-bench's pass^k, reported at
// k = 3 by this kit's benches).
const regressionPassK = 3

// regressionPassPowK folds current into one ScenarioPassPowK per scenario
// in ScenarioID order: pass and absent_proven count as successes, fail as
// a failure, abstain and error as excluded (counted, not hidden). It is
// reported next to the gate and never feeds it.
func regressionPassPowK(current []evaljournal.Record, k int) []ScenarioPassPowK {
	byID := make(map[string]*ScenarioPassPowK)
	for _, rec := range current {
		s := byID[rec.ScenarioID]
		if s == nil {
			s = &ScenarioPassPowK{ScenarioID: rec.ScenarioID, K: k}
			byID[rec.ScenarioID] = s
		}
		switch rec.Verdict {
		case evaljournal.VerdictPass, evaljournal.VerdictAbsentProven:
			s.Trials++
			s.Passes++
		case evaljournal.VerdictFail:
			s.Trials++
		default: // abstain, error, or an unknown verdict: judged nothing
			s.Excluded++
		}
	}
	out := make([]ScenarioPassPowK, 0, len(byID))
	for _, s := range byID {
		if s.Trials == 0 {
			s.PassPowK = math.NaN()
		} else {
			s.PassPowK = reliabilitystats.PassPowK(float64(s.Passes)/float64(s.Trials), k)
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScenarioID < out[j].ScenarioID })
	return out
}

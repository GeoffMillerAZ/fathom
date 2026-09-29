package evalrunner

import (
	"fmt"
	"time"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	modelcatalog "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/model-catalog"
	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
	determinismgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/determinism-gate"
	diffreporter "github.com/geoffmilleraz/agentic-kit/foundations/quality/diff-reporter"
	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
	evaljournal "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-journal"
)

// Mode selects how judge lanes are backed.
//
// CUE mirror: #Mode in schema.cue.
type Mode string

const (
	// ModeReplay backs judge lanes with replay cassettes — fully offline,
	// the CI mode. A green replay run proves the code path, not current
	// live-model behavior.
	ModeReplay Mode = "replay"

	// ModeLive backs judge lanes with real provider clients. Spends tokens.
	ModeLive Mode = "live"
)

// JudgeLane names one judge model lane. Model and Family are caller data;
// New refuses construction when Family disagrees with the caller-supplied
// model catalog's entry for Model.
//
// CUE mirror: #JudgeLane in schema.cue.
type JudgeLane struct {
	// Name is the lane identifier, unique within the config. It keys
	// Deps.JudgeClients/Deps.Pingers and appears in journal RunID concern
	// labels.
	Name string

	// Model is the judge model ID as used on the wire. Caller data — never
	// a Go constant.
	Model string

	// Family is the judge model's family tag (org-defined, opaque). Feeds
	// evaljournal.Judge.Family.
	Family string
}

// Producer describes the component that generated the outputs under
// judgment. It maps 1:1 onto evaljournal.Producer; the runner copies it
// field-for-field into every journal record.
//
// CUE mirror: #Producer in schema.cue.
type Producer struct {
	// Model is the producer model ID. Required.
	Model string `json:"model"`

	// PromptHash, when non-empty, is a hash of the prompt template used.
	PromptHash string `json:"prompt_hash,omitempty"`

	// ParamsHash, when non-empty, is a hash of the inference parameters used.
	ParamsHash string `json:"params_hash,omitempty"`
}

// journal converts p to its eval-journal wire twin.
func (p Producer) journal() evaljournal.Producer {
	return evaljournal.Producer{Model: p.Model, PromptHash: p.PromptHash, ParamsHash: p.ParamsHash}
}

// Output is one caller-supplied, pre-produced output under judgment. The
// runner does NOT drive a producer lane: outputs are produced elsewhere
// (a harness run, a recorded session) and handed to Run. Committed replay
// cassettes therefore record judge calls only.
type Output struct {
	// ScenarioID names the scenario this output answers. Required; every
	// scenario in the bank must have exactly one matching Output.
	ScenarioID string `json:"scenario_id"`

	// Text is the output under judgment.
	Text string `json:"text"`

	// Producer describes what produced Text. It must exactly equal
	// Config.Producer — independence was validated against that producer
	// identity and no other (Run refuses mismatches with
	// ErrProducerMismatch).
	Producer Producer `json:"producer"`
}

// Independence carries caller assertions forwarded verbatim to
// modelcatalog.ValidateIndependence.
//
// CUE mirror: #Independence in schema.cue.
type Independence struct {
	// SameSession asserts the judge and producer share a session context.
	// Always a violation when true.
	SameSession bool

	// SharedHistory asserts the judge can see the producer's generation
	// history. Always a violation when true.
	SharedHistory bool
}

// BudgetLimits is the data half of the budget seam: the composition root
// builds the budgetguard.Guard from these numbers and injects it via
// Deps.Guard. The kit never constructs a guard itself.
//
// CUE mirror: #BudgetLimits in schema.cue.
type BudgetLimits struct {
	// MaxReqPerMin is the per-rolling-minute judge-call cap.
	MaxReqPerMin int

	// MaxReqPerDay is the per-rolling-day judge-call cap.
	MaxReqPerDay int

	// MaxUSDPerDay is the per-rolling-day estimated USD spend cap.
	MaxUSDPerDay float64
}

// Config holds the data-shaped settings of a Runner. All fields are plain
// data; behavioral collaborators live in Deps.
//
// CUE mirror: #EvalConfig in schema.cue.
type Config struct {
	// RunID labels every journal record from this runner. diff-reporter
	// keys records by (ScenarioID, run_id) — keep RunID stable across runs
	// of the same eval concern so regression diffs line up. The runner
	// appends "/<lane name>" per judge lane so multi-lane records do not
	// collide under diff-reporter's last-record-wins rule.
	RunID string

	// JudgeLanes lists the judge lanes. At least one is required.
	JudgeLanes []JudgeLane

	// Producer describes the (single) producer of all outputs under
	// judgment in this run.
	Producer Producer

	// Mode selects replay vs live judge backing. Required — the choice is
	// always explicit config, never an ambient default.
	Mode Mode

	// NonDecisive, when true, constructs the runner even when independence
	// validation reports violations — and in exchange NO record is ever
	// marked decisive. This is the only way an all-same-family
	// judge/producer config constructs. Default false: any violation is
	// construction-fatal (ErrIndependenceViolated).
	NonDecisive bool

	// Independence carries session/history assertions for
	// modelcatalog.ValidateIndependence.
	Independence Independence

	// ScenarioBankPath is the path to the scenario-bank JSON. When
	// non-empty, New loads and validates it through LoadBank — that is
	// what arms the eval.provenance-gates-on fitness function before the
	// first Run (a bad or production-captured source is
	// construction-fatal). Empty means no bank source is declared at
	// construction: ProvenanceGatesOn stays false until a Run validates a
	// bank on the run path.
	ScenarioBankPath string

	// BaselinePath is the path to the baseline journal JSONL for the
	// regression gate. Data for the composition root, which opens it and
	// calls Regression over the run's records.
	BaselinePath string

	// PriceTableSource is the path to the price-table data used to build
	// Deps.PriceTable. Data for the composition root. Committed fixture
	// tables are EXAMPLE data, never authoritative prices.
	PriceTableSource string

	// Budget is the data half of the budget-guard seam: the composition
	// root builds Deps.Guard from these numbers.
	Budget BudgetLimits

	// DeterminismK is the K for the determinism-gate variance probe run
	// before judging. K >= 3 is required for any run to be reported
	// Stable; K < 3 skips the probe entirely — the run can never be
	// Stable and the eval.determinism-probed fitness function fails.
	// 0 disables probing. Must not be negative.
	DeterminismK int

	// StabilityBand is the minimum probe agreement rate to classify
	// Stable, in [0,1]. Zero defers to determinism-gate's default (1.0).
	StabilityBand float64

	// ScoreDropThreshold is diff-reporter's minimum score decrease to
	// count as a regression, in [0,1] (fraction of score). Zero = only
	// verdict flips count as regressions.
	ScoreDropThreshold float64

	// GateThreshold is the eval-gate pass threshold for the regression
	// gate, in [0,1]. Zero defaults to 1.0 (any regression fails).
	GateThreshold float64

	// JudgeMaxTokens caps each judge response; 0 = model default.
	JudgeMaxTokens int

	// JudgeSystem overrides the default judge system prompt.
	JudgeSystem string
}

// Validate checks the data invariants that mirror schema.cue. New calls it;
// callers constructing Configs programmatically may call it directly.
func (c Config) Validate() error {
	if c.RunID == "" {
		return fmt.Errorf("evalrunner: Config.RunID must not be empty")
	}
	if len(c.JudgeLanes) == 0 {
		return fmt.Errorf("evalrunner: Config.JudgeLanes must have at least one lane")
	}
	seen := make(map[string]struct{}, len(c.JudgeLanes))
	for i, l := range c.JudgeLanes {
		if l.Name == "" {
			return fmt.Errorf("evalrunner: Config.JudgeLanes[%d].Name must not be empty", i)
		}
		if l.Model == "" {
			return fmt.Errorf("evalrunner: Config.JudgeLanes[%d].Model must not be empty", i)
		}
		if l.Family == "" {
			return fmt.Errorf("evalrunner: Config.JudgeLanes[%d].Family must not be empty", i)
		}
		if _, dup := seen[l.Name]; dup {
			return fmt.Errorf("evalrunner: Config.JudgeLanes[%d].Name %q is duplicated", i, l.Name)
		}
		seen[l.Name] = struct{}{}
	}
	if c.Producer.Model == "" {
		return fmt.Errorf("evalrunner: Config.Producer.Model must not be empty")
	}
	if c.Mode != ModeReplay && c.Mode != ModeLive {
		return fmt.Errorf("evalrunner: Config.Mode %q is not one of replay|live", c.Mode)
	}
	if c.DeterminismK < 0 {
		return fmt.Errorf("evalrunner: Config.DeterminismK must be >= 0 (got %d)", c.DeterminismK)
	}
	if c.StabilityBand < 0 || c.StabilityBand > 1 {
		return fmt.Errorf("evalrunner: Config.StabilityBand %g is outside [0, 1]", c.StabilityBand)
	}
	if c.ScoreDropThreshold < 0 || c.ScoreDropThreshold > 1 {
		return fmt.Errorf("evalrunner: Config.ScoreDropThreshold %g is outside [0, 1]", c.ScoreDropThreshold)
	}
	if c.GateThreshold < 0 || c.GateThreshold > 1 {
		return fmt.Errorf("evalrunner: Config.GateThreshold %g is outside [0, 1]", c.GateThreshold)
	}
	if c.JudgeMaxTokens < 0 {
		return fmt.Errorf("evalrunner: Config.JudgeMaxTokens must be >= 0 (got %d)", c.JudgeMaxTokens)
	}
	return nil
}

// Deps carries the behavioral collaborators of a Runner — the full
// composition surface. Data-shaped settings live in Config.
type Deps struct {
	// JudgeClients maps lane name → the lane's LLM client (live provider,
	// replay client, or mock). Required: one entry per Config.JudgeLanes
	// lane. New wraps each client with llminstrument.WithBudget; inject
	// any other instrumentation (retry, telemetry, cost) BEFORE handing
	// the client over, per that package's composition-order doc.
	JudgeClients map[string]llmclient.LLMClient

	// Pingers maps lane name → the lane's startup probe. Optional per
	// lane: when absent, the lane's JudgeClients client is used if it
	// implements llmclient.Pinger; otherwise the lane is NOT probed (a
	// stated non-guarantee, typical for replay clients).
	Pingers map[string]llmclient.Pinger

	// Catalog is the caller-supplied model catalog used for independence
	// validation. The kit ships no rows; an empty catalog fails every
	// lane with an unknown_model violation — fail-closed by design.
	Catalog modelcatalog.Catalog

	// Guard is the budget-guard every judge call pre-charges through.
	// Required — New refuses a nil guard.
	Guard *budgetguard.Guard

	// PriceTable is the price table for pre-flight estimates and
	// settlement. Caller data (committed fixtures are examples, never
	// authoritative). A nil/empty table prices every call at $0: the
	// guard's request-count dimensions still govern, its USD dimension
	// does not.
	PriceTable costtracker.PriceTable

	// Estimator is the optional token-count estimator; nil uses
	// costtracker.EstimateTokens.
	Estimator costtracker.Estimator

	// DefaultModel is llminstrument.WithBudget's pricing fallback for
	// requests that carry an empty Model — one of the real WithBudget
	// arguments. LLMJudge always sets the lane model on its requests, so
	// this is inert in normal operation. Empty falls back to each lane's
	// own Model.
	DefaultModel string

	// Journal is the eval-journal sink. New permits nil (so the
	// eval.journal-attached fitness function can observe the absence),
	// but Run refuses to judge without it (ErrNoJournal).
	Journal *jsonllog.Writer

	// Now is the injected clock stamping OccurredAt on every record.
	// Nil defaults to time.Now (the documented default for this seam);
	// inject a fixed clock for byte-stable journals.
	Now func() time.Time
}

// StopCause reports why a Run ended. Refusals are typed errors; budget
// exhaustion carries BOTH a StopCause and a typed error, so a truncated run
// can never read as green. Callers must treat anything other than
// StopCompleted as a non-green run.
type StopCause string

const (
	// StopCompleted: every (scenario, lane) pair was judged.
	StopCompleted StopCause = "completed"

	// StopBudget: the budget guard denied a judge-call pre-charge. Every
	// remaining pair was journaled as a verdict:"error" record — never
	// missing records — and Run returned an error satisfying
	// errors.Is(err, llminstrument.ErrBudgetExceeded).
	StopBudget StopCause = "budget"

	// StopCanceled: ctx was cancelled between judge calls. Records
	// journaled so far are kept; remaining pairs are NOT journaled
	// (cancellation is caller-chosen, not an eval outcome).
	StopCanceled StopCause = "canceled"

	// StopError: Run is returning a non-nil error (empty bank, missing
	// journal, probe failure, output-set mismatch, journal write failure).
	StopError StopCause = "error"
)

// Result is the outcome of one Run.
type Result struct {
	// Stop reports why the run ended. Callers must treat anything other
	// than StopCompleted as a non-green run.
	Stop StopCause

	// Records holds every journal record this run appended, in append
	// order — the input to Regression.
	Records []evaljournal.Record

	// Probe is the determinism-gate probe result, nil when
	// Config.DeterminismK < 3 (no probe ran).
	Probe *determinismgate.ProbeResult

	// Stable is true only when a probe with K >= 3 ran AND classified the
	// judge stable within the configured band. Without a probe a run is
	// never Stable — unprobed is not stable.
	Stable bool

	// ScenarioCount is the number of scenarios judged (bank size).
	ScenarioCount int

	// JudgeCalls is the number of judge Score invocations made in the
	// main loop (excludes probe calls; includes calls that returned
	// errors). Pairs skipped after budget exhaustion are journaled but
	// not called, so they are not counted here.
	JudgeCalls int
}

// RegressionResult is the typed outcome of the regression gate: the raw
// diff-reporter delta, the eval-gate evaluation over per-pair checks, and
// the overall verdict.
type RegressionResult struct {
	// Delta is the diff-reporter comparison of baseline vs current.
	Delta diffreporter.Delta

	// Gate is the eval-gate result over one check per compared
	// (scenario, concern) pair — a pair's check fails on a verdict flip
	// or a score drop >= Config.ScoreDropThreshold (when > 0).
	Gate evalgate.Result

	// Passed is the overall verdict: Gate.Passed, or true when there were
	// no comparable pairs at all (nothing to regress — documented
	// explicitly because eval-gate alone would fail an empty check set).
	Passed bool

	// PassPowK is the per-scenario pass^3 over the current records, one
	// entry per ScenarioID in sorted order (FB-13). Reported, never gated:
	// Passed does not depend on it.
	PassPowK []ScenarioPassPowK
}

// ScenarioPassPowK is one scenario's pass^k reliability estimate over the
// current records, from reliabilitystats.PassPowK(Passes/Trials, K).
// Trials counts judged outcomes (pass, fail, absent_proven), Passes the
// successes (pass, absent_proven), Excluded the records that judged
// nothing (abstain, error) — reported so a budget-starved run cannot look
// reliable by omission. PassPowK is NaN when Trials == 0, and a Trials
// below K is a point on too little evidence: consumers floor it (fathom's
// bench requires n >= 3), this kit reports it.
type ScenarioPassPowK struct {
	// ScenarioID names the scenario.
	ScenarioID string

	// Trials is the number of judged (pass/fail/absent_proven) records.
	Trials int

	// Passes is the number of pass or absent_proven records.
	Passes int

	// Excluded is the number of abstain or error records left out of Trials.
	Excluded int

	// K is the exponent: regressionPassK (3).
	K int

	// PassPowK is (Passes/Trials)^K; NaN when Trials == 0.
	PassPowK float64
}

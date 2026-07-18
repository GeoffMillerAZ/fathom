// Package evalrunner defines the schema for eval-runner configuration.
// These constraints mirror the data-shaped Go Config type (types.go) and are
// evaluated with `cue vet ./...`. Behavioral dependencies (judge clients,
// the budget guard, the price table, the journal writer, the model catalog,
// the clock) live in the Go Deps struct and have no CUE shape — CUE owns
// data, Go owns behavior.
//
// ORG-AGNOSTIC BY CONSTRUCTION: model IDs, family tags, prices, and catalog
// rows are ALWAYS caller data. Nothing in this schema names a real provider,
// model, or price; the values in testdata/valid.cue are illustrative
// examples only, never defaults.
package evalrunner

// #Mode selects how judge lanes are backed. "replay" = cassette-backed
// clients (offline, CI); "live" = real provider clients (spends tokens).
// A green replay run proves the code path, NOT current live-model behavior.
#Mode: "replay" | "live"

// #JudgeLane names one judge model lane. Model and Family are caller data;
// Family must agree with the caller-supplied model catalog entry for Model
// (construction verifies and refuses drift).
#JudgeLane: {
	// Name is the lane identifier, unique within the config. It keys
	// Deps.JudgeClients/Deps.Pingers and appears in journal RunID concern
	// labels.
	Name: string & !=""

	// Model is the judge model ID as used on the wire. Caller data.
	Model: string & !=""

	// Family is the judge model's family tag (org-defined, opaque). It
	// feeds evaljournal.Judge.Family and is cross-checked against the
	// model catalog at construction.
	Family: string & !=""
}

// #Producer describes the component that generated the outputs under
// judgment. It maps 1:1 onto eval-journal's producer descriptor and
// validates Output.Producer at run time: every caller-supplied Output must
// carry this Model or the run refuses (independence was validated against
// THIS producer, no other).
#Producer: {
	// Model is the producer model ID. Required.
	Model: string & !=""

	// PromptHash, when set, is a hash of the prompt template used.
	PromptHash?: string

	// ParamsHash, when set, is a hash of the inference parameters used.
	ParamsHash?: string
}

// #Independence carries caller assertions forwarded verbatim to
// modelcatalog.ValidateIndependence. Either flag being true is always a
// violation (construction-fatal in decisive mode).
#Independence: {
	// SameSession asserts the judge and producer share a session context.
	SameSession?: bool

	// SharedHistory asserts the judge can see the producer's generation
	// history.
	SharedHistory?: bool
}

// #BudgetLimits is the data half of the budget seam: the composition root
// builds the budgetguard.Guard from these numbers and injects it via
// Deps.Guard. The kit never constructs a guard itself.
#BudgetLimits: {
	// MaxReqPerMin is the per-rolling-minute judge-call cap.
	MaxReqPerMin: int & >=1

	// MaxReqPerDay is the per-rolling-day judge-call cap.
	MaxReqPerDay: int & >=1

	// MaxUSDPerDay is the per-rolling-day estimated USD spend cap.
	MaxUSDPerDay: number & >0
}

// #EvalConfig holds the data-shaped settings of a Runner.
#EvalConfig: {
	// RunID labels every journal record from this runner. diff-reporter
	// keys records by (ScenarioID, run_id) — keep RunID stable across runs
	// of the same eval concern so regression diffs line up. The runner
	// appends "/<lane name>" per judge lane.
	RunID: string & !=""

	// JudgeLanes lists the judge lanes. At least one is required.
	JudgeLanes: [#JudgeLane, ...#JudgeLane]

	// Producer describes the (single) producer of all outputs under
	// judgment in this run.
	Producer: #Producer

	// Mode selects replay vs live judge backing. Required so the choice is
	// always explicit config, never an ambient default.
	Mode: #Mode

	// NonDecisive, when true, constructs the runner even when independence
	// validation reports violations — and in exchange NO record is ever
	// marked decisive. This is the only way an all-same-family
	// judge/producer config constructs. Default false: any violation is
	// construction-fatal.
	NonDecisive?: bool

	// Independence carries session/history assertions for
	// modelcatalog.ValidateIndependence.
	Independence?: #Independence

	// ScenarioBankPath is the path to the scenario-bank JSON. When
	// non-empty, New loads and validates it through LoadBank at
	// construction (arming the eval.provenance-gates-on fitness function
	// before the first Run).
	ScenarioBankPath?: string

	// BaselinePath is the path to the baseline journal JSONL for the
	// regression gate. Data for the composition root.
	BaselinePath?: string

	// PriceTableSource is the path to the price-table JSON/CUE used to
	// build Deps.PriceTable. Data for the composition root. Committed fixture
	// tables are EXAMPLE data, never authoritative prices.
	PriceTableSource?: string

	// Budget is the data half of the budget-guard seam (see #BudgetLimits).
	Budget?: #BudgetLimits

	// DeterminismK is the K for the determinism-gate variance probe run
	// before judging. K >= 3 is required for any run to be reported Stable;
	// K < 3 skips the probe and the run can never be Stable (and the
	// eval.determinism-probed fitness function fails). 0 disables probing.
	DeterminismK?: int & >=0

	// StabilityBand is the minimum probe agreement rate to classify Stable,
	// in [0,1]. Zero defers to determinism-gate's default (1.0).
	StabilityBand?: number & >=0 & <=1

	// ScoreDropThreshold is diff-reporter's minimum score decrease to count
	// as a regression, in [0,1]. Zero = only verdict flips count.
	ScoreDropThreshold?: number & >=0 & <=1

	// GateThreshold is the eval-gate pass threshold for the regression
	// gate, in [0,1]. Zero defaults to 1.0 (any regression fails).
	GateThreshold?: number & >=0 & <=1

	// JudgeMaxTokens caps each judge response; 0 = model default.
	JudgeMaxTokens?: int & >=0

	// JudgeSystem overrides the default judge system prompt.
	JudgeSystem?: string
}

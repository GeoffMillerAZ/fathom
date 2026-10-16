package evalrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	evaljournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-journal"
	qaharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
)

// judgeOutputSchema constrains the judge's response to the structured
// verdict shape via Request.OutputSchema. "error" is deliberately absent
// from the enum: error verdicts describe infrastructure failures and are
// minted by this package, never by the model.
const judgeOutputSchema = `{
	"type": "object",
	"properties": {
		"verdict": {"type": "string", "enum": ["pass", "fail", "absent_proven", "abstain"]},
		"score": {"type": "number", "minimum": 0, "maximum": 1},
		"rationale": {"type": "string"}
	},
	"required": ["verdict", "score", "rationale"],
	"additionalProperties": false
}`

// defaultJudgeSystem is the org-agnostic default system prompt for judge
// calls. Override via LLMJudgeOptions.System (Config.JudgeSystem in the
// runner).
const defaultJudgeSystem = "You are an impartial evaluation judge. Judge only the output you are " +
	"given against the scenario input and any expected answer. Respond with a " +
	"single JSON object matching the required schema: verdict is one of " +
	"pass|fail|absent_proven|abstain, score is a number in [0,1], and " +
	"rationale explains the verdict. Abstain when you cannot judge."

// Verdict is the structured outcome of one LLMJudge call, delivered to the
// per-call sink synchronously inside Score. Exactly one of two shapes:
// a parsed model verdict (Err == ""), or an infrastructure failure
// (Err != "", Verdict == evaljournal.VerdictError, Score nil).
type Verdict struct {
	// ScenarioID is the judged Item's ID.
	ScenarioID string

	// JudgeModel is the judge model ID that produced (or failed to
	// produce) this verdict.
	JudgeModel string

	// Verdict is the outcome. evaljournal.VerdictError when the call or
	// the parse failed.
	Verdict evaljournal.Verdict

	// Score is the model's score in [0,1]; nil on failure.
	Score *float64

	// Rationale is the model's explanation; empty on failure (an empty
	// rationale can never support a decisive record).
	Rationale string

	// Err is the failure message when the call or parse failed; empty on
	// success.
	Err string
}

// LLMJudgeOptions configures NewLLMJudge. Client and Model are required.
type LLMJudgeOptions struct {
	// Model is the judge model ID sent on every request. Caller data.
	Model string

	// System overrides the default judge system prompt when non-empty.
	System string

	// MaxTokens caps each judge response; 0 = model default.
	MaxTokens int

	// Sink, when non-nil, receives every Verdict synchronously inside
	// Score before Score returns — including error verdicts for failed
	// calls. Invocations are serialized under the judge's mutex, so a
	// non-reentrant sink is safe under concurrent Score calls.
	Sink func(Verdict)
}

// LLMJudge is the model-backed qaharness.Judge: one llmclient call per
// scenario, structured via Request.OutputSchema, with a dual result
// channel — Score's float64 for the qa-harness contract, and the richer
// Verdict (verdict/score/rationale, or the failure) through the sink.
//
// Concurrency: safe for concurrent Score calls. The sink is invoked under
// a mutex (one invocation at a time, synchronously inside Score).
type LLMJudge struct {
	client    llmclient.LLMClient
	model     string
	system    string
	maxTokens int

	mu   sync.Mutex // serializes sink invocations
	sink func(Verdict)
}

var _ qaharness.Judge = (*LLMJudge)(nil)

// NewLLMJudge constructs a model-backed judge over client. client must be
// non-nil and opts.Model non-empty.
func NewLLMJudge(client llmclient.LLMClient, opts LLMJudgeOptions) (*LLMJudge, error) {
	if client == nil {
		return nil, fmt.Errorf("evalrunner: NewLLMJudge: client must not be nil")
	}
	if opts.Model == "" {
		return nil, fmt.Errorf("evalrunner: NewLLMJudge: opts.Model must not be empty")
	}
	system := opts.System
	if system == "" {
		system = defaultJudgeSystem
	}
	return &LLMJudge{
		client:    client,
		model:     opts.Model,
		system:    system,
		maxTokens: opts.MaxTokens,
		sink:      opts.Sink,
	}, nil
}

// judgeWire is the JSON shape the model is constrained to emit.
type judgeWire struct {
	Verdict   string   `json:"verdict"`
	Score     *float64 `json:"score"`
	Rationale string   `json:"rationale"`
}

// modelVerdicts is the set of verdicts the model may emit ("error" is
// reserved for infrastructure failures minted by this package).
var modelVerdicts = map[evaljournal.Verdict]struct{}{
	evaljournal.VerdictPass:         {},
	evaljournal.VerdictFail:         {},
	evaljournal.VerdictAbsentProven: {},
	evaljournal.VerdictAbstain:      {},
}

// Score implements qaharness.Judge: it makes one Complete call, parses the
// structured verdict, delivers the full Verdict to the sink (synchronously,
// before returning), and returns the numeric score.
//
// Failure semantics: a client error or an unparseable/non-conforming
// response delivers an error Verdict to the sink and returns a non-nil
// error — the sink ALWAYS observes exactly one Verdict per Score call, so
// a journaling sink records error verdicts rather than missing records.
// Item context: Metadata["scenario_input"] and Metadata["expected"], when
// present, are included in the judge prompt alongside Item.Output.
func (j *LLMJudge) Score(ctx context.Context, item qaharness.Item) (float64, error) {
	req := llmclient.Request{
		Model:        j.model,
		System:       j.system,
		Prompt:       j.buildPrompt(item),
		MaxTokens:    j.maxTokens,
		OutputSchema: json.RawMessage(judgeOutputSchema),
	}

	resp, err := j.client.Complete(ctx, req)
	if err != nil {
		j.emit(Verdict{
			ScenarioID: item.ID,
			JudgeModel: j.model,
			Verdict:    evaljournal.VerdictError,
			Err:        err.Error(),
		})
		return 0, fmt.Errorf("evalrunner: LLMJudge %q: %w", j.model, err)
	}

	var wire judgeWire
	if perr := json.Unmarshal([]byte(resp.Text), &wire); perr != nil {
		return 0, j.emitParseFailure(item.ID, fmt.Sprintf("unparseable judge response: %v", perr))
	}
	v := evaljournal.Verdict(wire.Verdict)
	if _, ok := modelVerdicts[v]; !ok {
		return 0, j.emitParseFailure(item.ID, fmt.Sprintf("judge verdict %q is not one of pass|fail|absent_proven|abstain", wire.Verdict))
	}
	if wire.Score == nil {
		return 0, j.emitParseFailure(item.ID, "judge response is missing score")
	}
	if *wire.Score < 0 || *wire.Score > 1 {
		return 0, j.emitParseFailure(item.ID, fmt.Sprintf("judge score %g is outside [0, 1]", *wire.Score))
	}

	score := *wire.Score
	j.emit(Verdict{
		ScenarioID: item.ID,
		JudgeModel: j.model,
		Verdict:    v,
		Score:      &score,
		Rationale:  wire.Rationale,
	})
	return score, nil
}

// emitParseFailure delivers an error Verdict for a structurally invalid
// judge response and returns the typed error Score propagates.
func (j *LLMJudge) emitParseFailure(scenarioID, msg string) error {
	j.emit(Verdict{
		ScenarioID: scenarioID,
		JudgeModel: j.model,
		Verdict:    evaljournal.VerdictError,
		Err:        msg,
	})
	return fmt.Errorf("evalrunner: LLMJudge %q: %s", j.model, msg)
}

// emit delivers v to the sink under the judge's mutex. Nil sink = no-op.
func (j *LLMJudge) emit(v Verdict) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.sink != nil {
		j.sink(v)
	}
}

// buildPrompt renders the judge prompt from the Item: scenario input and
// expected answer (when carried in Metadata) plus the output under
// judgment.
func (j *LLMJudge) buildPrompt(item qaharness.Item) string {
	var b strings.Builder
	b.WriteString("Judge the following output.\n")
	if in := item.Metadata["scenario_input"]; in != "" {
		b.WriteString("\n## Scenario input\n")
		b.WriteString(in)
		b.WriteString("\n")
	}
	if exp := item.Metadata["expected"]; exp != "" {
		b.WriteString("\n## Expected answer\n")
		b.WriteString(exp)
		b.WriteString("\n")
	}
	b.WriteString("\n## Output under judgment\n")
	b.WriteString(item.Output)
	b.WriteString("\n")
	return b.String()
}

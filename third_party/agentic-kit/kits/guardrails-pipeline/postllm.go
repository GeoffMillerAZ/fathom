package guardrailspipeline

import (
	"context"
	"fmt"

	honestygate "github.com/geoffmilleraz/agentic-kit/foundations/quality/honesty-gate"
	integrityevaluator "github.com/geoffmilleraz/agentic-kit/foundations/quality/integrity-evaluator"
)

// defaultPostLLMStages is the canonical stage order when PostLLMConfig.Stages
// is empty.
var defaultPostLLMStages = []string{"integrity", "honesty", "dlp", "disclaimer"}

// defaultDLPThreshold is used when PostLLMConfig.DLPThreshold is zero.
const defaultDLPThreshold = 0.67

// PostLLMChain is a configurable pipeline of post-LLM safety gates.
// Create one with NewPostLLMChain; the zero value is not usable.
// Safe for concurrent use after construction.
type PostLLMChain struct {
	cfg    PostLLMConfig
	deps   PostLLMDeps
	stages []string
}

// NewPostLLMChain returns a PostLLMChain configured with cfg and deps.
// Defaults are applied for a zero DLPThreshold and an empty stage list.
func NewPostLLMChain(cfg PostLLMConfig, deps PostLLMDeps) *PostLLMChain {
	if cfg.DLPThreshold == 0 {
		cfg.DLPThreshold = defaultDLPThreshold
	}
	stages := cfg.Stages
	if len(stages) == 0 {
		stages = defaultPostLLMStages
	}
	return &PostLLMChain{cfg: cfg, deps: deps, stages: stages}
}

// Run executes the post-LLM chain against text and returns a Decision.
// Stages run in the configured order. The chain is fail-closed: any stage
// error produces Decision{Allowed:false, Err:err} immediately.
func (c *PostLLMChain) Run(ctx context.Context, text string) Decision {
	transformed := text
	var stageResults []StageResult

	for _, stage := range c.stages {
		var sr StageResult
		var err error

		switch stage {
		case "integrity":
			sr, err = c.runIntegrity(text)
		case "honesty":
			sr, err = c.runHonesty(text)
		case "dlp":
			sr, err = c.runDLP(text)
		case "disclaimer":
			sr, transformed = c.runDisclaimer(transformed)
		default:
			sr = StageResult{Stage: stage, Allowed: true, Action: "skipped"}
		}

		if err != nil {
			d := Decision{
				Allowed:     false,
				Stages:      append(stageResults, StageResult{Stage: stage, Allowed: false, Action: "error"}),
				Transformed: transformed,
				Err:         err,
			}
			if c.deps.Sink != nil {
				c.deps.Sink.Emit(ctx, "post-llm", d)
			}
			return d
		}

		stageResults = append(stageResults, sr)
		if !sr.Allowed {
			d := Decision{
				Allowed:     false,
				Stages:      stageResults,
				Transformed: transformed,
			}
			if c.deps.Sink != nil {
				c.deps.Sink.Emit(ctx, "post-llm", d)
			}
			return d
		}
	}

	d := Decision{
		Allowed:     true,
		Stages:      stageResults,
		Transformed: transformed,
	}
	if c.deps.Sink != nil {
		c.deps.Sink.Emit(ctx, "post-llm", d)
	}
	return d
}

func (c *PostLLMChain) runIntegrity(text string) (StageResult, error) {
	const stageName = "integrity"
	knownRefs := c.deps.IntegrityKnownRefs
	if knownRefs == nil {
		knownRefs = map[string]string{}
	}
	report := integrityevaluator.Evaluate(text, knownRefs)
	if report.Passed {
		return StageResult{Stage: stageName, Allowed: true, Action: "allowed"}, nil
	}
	var findings []string
	for _, v := range report.Violations {
		findings = append(findings, fmt.Sprintf("%s(%s): %s", v.Kind, v.Ref, v.Detail))
	}
	return StageResult{
		Stage:    stageName,
		Allowed:  false,
		Action:   "denied",
		Findings: findings,
	}, nil
}

func (c *PostLLMChain) runHonesty(text string) (StageResult, error) {
	const stageName = "honesty"
	if c.deps.HonestyGate == nil || c.deps.ClaimProvider == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, nil
	}
	claim := c.deps.ClaimProvider.ClaimFor(text)
	decision := c.deps.HonestyGate.Evaluate(claim)
	if decision.Verdict == honestygate.VerdictBlock {
		return StageResult{
			Stage:    stageName,
			Allowed:  false,
			Action:   "denied",
			Findings: []string{decision.Reason},
			Severity: string(decision.Verdict),
		}, nil
	}
	return StageResult{
		Stage:    stageName,
		Allowed:  true,
		Action:   "allowed",
		Findings: []string{decision.Reason},
		Severity: string(decision.Verdict),
	}, nil
}

func (c *PostLLMChain) runDLP(text string) (StageResult, error) {
	const stageName = "dlp"
	if c.deps.DLPEvaluator == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, nil
	}
	report := c.deps.DLPEvaluator.Evaluate(text)
	var findings []string
	for _, f := range report.Findings {
		findings = append(findings, fmt.Sprintf("%s(%s): %s", f.RuleID, string(f.RiskBand), f.Excerpt))
	}
	if report.Score >= c.cfg.DLPThreshold {
		return StageResult{
			Stage:    stageName,
			Allowed:  false,
			Action:   "denied",
			Findings: findings,
			Severity: string(report.MaxRisk),
		}, nil
	}
	return StageResult{
		Stage:    stageName,
		Allowed:  true,
		Action:   "allowed",
		Findings: findings,
		Severity: string(report.MaxRisk),
	}, nil
}

func (c *PostLLMChain) runDisclaimer(text string) (StageResult, string) {
	const stageName = "disclaimer"
	if c.cfg.Disclaimer == "" {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, text
	}
	return StageResult{
		Stage:   stageName,
		Allowed: true,
		Action:  "transformed",
	}, text + c.cfg.Disclaimer
}

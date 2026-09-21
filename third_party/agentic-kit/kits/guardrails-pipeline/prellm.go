package guardrailspipeline

import (
	"context"
	"fmt"

	rulecore "github.com/geoffmilleraz/agentic-kit/foundations/security/rule-core"
)

// defaultPreLLMStages is the canonical stage order when PreLLMConfig.Stages
// is empty.
var defaultPreLLMStages = []string{"wrap", "inject-detect", "classify", "redact"}

// sensitivityLevels defines the ordered sensitivity levels from lowest to
// highest, matching data-sensitivity-types.SensitivityLevel constants.
var sensitivityLevels = []string{"public", "internal", "confidential", "restricted"}

// severityLevels defines the ordered injection severity levels from lowest to
// highest, matching injectiondetector.Severity constants.
var severityLevels = []string{"low", "medium", "high", "critical"}

// PreLLMChain is a configurable pipeline of pre-LLM safety gates.
// Create one with NewPreLLMChain; the zero value is not usable.
// Safe for concurrent use after construction.
type PreLLMChain struct {
	cfg      PreLLMConfig
	deps     PreLLMDeps
	sevOrd   rulecore.Ordinal
	classOrd rulecore.Ordinal
	stages   []string
}

// NewPreLLMChain returns a PreLLMChain configured with cfg and deps.
// Defaults are applied for empty threshold strings and empty stage lists.
func NewPreLLMChain(cfg PreLLMConfig, deps PreLLMDeps) *PreLLMChain {
	if cfg.InjectDetectThreshold == "" {
		cfg.InjectDetectThreshold = "high"
	}
	if cfg.ClassifyThreshold == "" {
		cfg.ClassifyThreshold = "confidential"
	}
	stages := cfg.Stages
	if len(stages) == 0 {
		stages = defaultPreLLMStages
	}
	return &PreLLMChain{
		cfg:      cfg,
		deps:     deps,
		sevOrd:   rulecore.NewOrdinal(severityLevels),
		classOrd: rulecore.NewOrdinal(sensitivityLevels),
		stages:   stages,
	}
}

// Run executes the pre-LLM chain against text and returns a Decision.
// Stages run in the configured order. The chain is fail-closed: any stage
// error produces Decision{Allowed:false, Err:err} immediately.
func (c *PreLLMChain) Run(ctx context.Context, text string) Decision {
	transformed := text
	var stageResults []StageResult

	for _, stage := range c.stages {
		var sr StageResult
		var err error

		switch stage {
		case "wrap":
			sr, transformed, err = c.runWrap(transformed)
		case "inject-detect":
			sr, err = c.runInjectDetect(transformed)
		case "classify":
			sr, err = c.runClassify(transformed)
		case "redact":
			sr, transformed, err = c.runRedact(transformed)
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
				c.deps.Sink.Emit(ctx, "pre-llm", d)
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
				c.deps.Sink.Emit(ctx, "pre-llm", d)
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
		c.deps.Sink.Emit(ctx, "pre-llm", d)
	}
	return d
}

func (c *PreLLMChain) runWrap(text string) (StageResult, string, error) {
	const stageName = "wrap"
	if c.deps.Wrapper == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, text, nil
	}
	wrapped, err := c.deps.Wrapper.Wrap(text)
	if err != nil {
		return StageResult{}, text, fmt.Errorf("guardrailspipeline: wrap stage: %w", err)
	}
	sr := StageResult{
		Stage:   stageName,
		Allowed: true,
		Action:  "transformed",
	}
	return sr, wrapped.Text, nil
}

func (c *PreLLMChain) runInjectDetect(text string) (StageResult, error) {
	const stageName = "inject-detect"
	if c.deps.Detector == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, nil
	}
	result := c.deps.Detector.Scan(text)
	if !result.Flagged {
		return StageResult{Stage: stageName, Allowed: true, Action: "allowed"}, nil
	}

	var findings []string
	for _, m := range result.Matches {
		findings = append(findings, fmt.Sprintf("%s(%s): %s", m.RuleID, string(m.Severity), m.Excerpt))
	}

	maxSev := string(result.MaxSeverity)
	if c.sevOrd.GTE(maxSev, c.cfg.InjectDetectThreshold) {
		return StageResult{
			Stage:    stageName,
			Allowed:  false,
			Action:   "denied",
			Findings: findings,
			Severity: maxSev,
		}, nil
	}

	// Flagged but below threshold: allow with findings recorded.
	return StageResult{
		Stage:    stageName,
		Allowed:  true,
		Action:   "allowed",
		Findings: findings,
		Severity: maxSev,
	}, nil
}

func (c *PreLLMChain) runClassify(text string) (StageResult, error) {
	const stageName = "classify"
	if c.deps.Classifier == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, nil
	}
	class := c.deps.Classifier.Classify(text)
	level := string(class.Level)

	if c.classOrd.GTE(level, c.cfg.ClassifyThreshold) {
		return StageResult{
			Stage:    stageName,
			Allowed:  false,
			Action:   "denied",
			Findings: []string{class.Reason},
			Severity: level,
		}, nil
	}

	return StageResult{
		Stage:    stageName,
		Allowed:  true,
		Action:   "allowed",
		Findings: []string{class.Reason},
		Severity: level,
	}, nil
}

func (c *PreLLMChain) runRedact(text string) (StageResult, string, error) {
	const stageName = "redact"
	if c.deps.Gate == nil {
		return StageResult{Stage: stageName, Allowed: true, Action: "skipped"}, text, nil
	}
	redacted, gateFindings := c.deps.Gate.Scan(text)
	var findings []string
	for _, f := range gateFindings {
		findings = append(findings, fmt.Sprintf("%s → %s: %s", f.RuleID, f.Label, f.Excerpt))
	}
	action := "allowed"
	if len(gateFindings) > 0 {
		action = "transformed"
	}
	sr := StageResult{
		Stage:    stageName,
		Allowed:  true,
		Action:   action,
		Findings: findings,
	}
	return sr, redacted, nil
}

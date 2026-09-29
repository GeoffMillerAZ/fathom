package guardrailspipeline

import (
	"fmt"
	"math"
	"slices"
)

// Validate rejects unknown stages, repeated stages, thresholds and actions.
// Zero values select documented defaults. It does not claim dependencies exist.
func (c PreLLMConfig) Validate() error {
	if err := validateStages(c.Stages, defaultPreLLMStages); err != nil {
		return err
	}
	if c.InjectDetectThreshold != "" && !slices.Contains(severityLevels, c.InjectDetectThreshold) {
		return fmt.Errorf("guardrailspipeline: invalid injection threshold %q", c.InjectDetectThreshold)
	}
	if c.ClassifyThreshold != "" && !slices.Contains(sensitivityLevels, c.ClassifyThreshold) {
		return fmt.Errorf("guardrailspipeline: invalid classification threshold %q", c.ClassifyThreshold)
	}
	if c.InjectDetectAction != "" && c.InjectDetectAction != "block" && c.InjectDetectAction != "annotate" {
		return fmt.Errorf("guardrailspipeline: invalid injection action %q", c.InjectDetectAction)
	}
	return nil
}

// Validate rejects unknown/repeated stages and non-finite or out-of-range scores.
func (c PostLLMConfig) Validate() error {
	if err := validateStages(c.Stages, defaultPostLLMStages); err != nil {
		return err
	}
	if math.IsNaN(c.DLPThreshold) || math.IsInf(c.DLPThreshold, 0) || c.DLPThreshold < 0 || c.DLPThreshold > 1 {
		return fmt.Errorf("guardrailspipeline: DLP threshold must be finite and within [0,1]")
	}
	return nil
}

func validateStages(stages, allowed []string) error {
	seen := make(map[string]bool, len(stages))
	for _, stage := range stages {
		if !slices.Contains(allowed, stage) || seen[stage] {
			return fmt.Errorf("guardrailspipeline: invalid or repeated stage %q", stage)
		}
		seen[stage] = true
	}
	return nil
}

func firstError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

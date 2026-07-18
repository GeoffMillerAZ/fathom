package expresslane

// Assess deterministically evaluates task against the two-gate express-lane
// check (size check then spec-challenge gate) and returns a Decision.
//
// The function is pure and stateless: given the same task and cfg, it always
// returns the same Decision.
func Assess(task Task, cfg Config) Decision {
	if cfg.MaxPayloadBytes <= 0 {
		cfg = DefaultConfig()
	}

	// Gate 1: size check.
	if len(task.Payload) > cfg.MaxPayloadBytes {
		return Decision{
			TaskID: task.ID,
			Lane:   LaneFull,
			Reason: ReasonSizeExceeded,
		}
	}

	// Gate 2: spec-challenge gate.
	// A task fails spec challenge if its description is empty or its estimated
	// complexity is unspecified (0 or negative).
	if task.Spec.Description == "" || task.Spec.EstimatedComplexity <= 0 {
		return Decision{
			TaskID: task.ID,
			Lane:   LaneFull,
			Reason: ReasonSpecIncomplete,
		}
	}

	return Decision{
		TaskID: task.ID,
		Lane:   LaneFast,
		Reason: ReasonEligible,
	}
}

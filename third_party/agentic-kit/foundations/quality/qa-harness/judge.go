package qaharness

import "context"

// Score implements Judge for RubricJudge. It iterates over Criteria in order,
// evaluates each Check, and computes the weighted score. The result is
// deterministic given identical Item inputs and Criteria.
func (r RubricJudge) Score(_ context.Context, item Item) (float64, error) {
	var total, passing float64
	for _, c := range r.Criteria {
		if c.Weight <= 0 {
			continue
		}
		total += c.Weight
		if c.Check(item) {
			passing += c.Weight
		}
	}
	if total == 0 {
		return 0.0, nil
	}
	return passing / total, nil
}

// Score implements Judge for MockJudge. It returns FixedScore and Err
// unconditionally.
func (m MockJudge) Score(_ context.Context, _ Item) (float64, error) {
	return m.FixedScore, m.Err
}

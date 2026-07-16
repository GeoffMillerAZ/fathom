package verify

import "fmt"

// LintPlan reports plan-grammar-v2 convention violations (ADR-018 §1). Lint
// findings are warnings: they never change gate exit codes in v0, but they
// surface the honesty conventions — stable IDs, reasoned dispositions,
// justified skips — that keep gated plans machine-legible. Deterministic:
// findings come out in task order, one pass, no I/O.
func LintPlan(p Plan) []string {
	var out []string
	seen := map[string]string{}
	for _, t := range p.Tasks {
		if prev, dup := seen[t.ID]; dup {
			out = append(out, fmt.Sprintf("%s: duplicate task id (first used by %q)", t.ID, prev))
		} else {
			seen[t.ID] = t.Title
		}
		if t.Disposition != "" {
			if t.Disposition != "refused" && t.Disposition != "deferred" {
				out = append(out, fmt.Sprintf("%s: unknown disposition %q (want refused|deferred)", t.ID, t.Disposition))
			}
			if !t.Skipped {
				out = append(out, fmt.Sprintf("%s: disposition %q requires a [-] checkbox", t.ID, t.Disposition))
			}
			if t.Reason == "" {
				out = append(out, fmt.Sprintf("%s: disposition %q requires a reason", t.ID, t.Disposition))
			}
		}
		if t.Trigger != "" && t.Disposition != "deferred" {
			out = append(out, fmt.Sprintf("%s: trigger only applies to disposition: deferred", t.ID))
		}
		if t.Expect.SkipTest && t.Expect.SkipReason == "" {
			out = append(out, fmt.Sprintf("%s: skip_test without a skip: justification", t.ID))
		}
	}
	return out
}

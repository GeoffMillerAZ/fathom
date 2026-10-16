package redactionexecutor

import (
	"fmt"

	rulecore "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/rule-core"
)

// MaxPasses is the hard cap on full rule sweeps a single Redact call will
// execute. It exists purely as a guard against pathological policies whose
// replacements re-trigger one another indefinitely (rules are data, so the
// executor cannot assume they are sane — e.g. rule A's replacement containing
// rule B's trigger and vice versa). Realistic policies converge within a few
// sweeps; hitting the cap without convergence is surfaced via
// Report.Converged == false on RedactReport.
const MaxPasses = 16

// Executor applies a RedactionPolicy to text. Create one with NewExecutor;
// the zero value is not usable. An Executor is immutable after construction
// and safe for concurrent use by multiple goroutines.
type Executor struct {
	policy   RedactionPolicy
	cs       *rulecore.CompiledSet   // all rules compiled together (for ReplaceFixpoint)
	singleCS []*rulecore.CompiledSet // one per rule, for per-rule occurrence counting
}

// NewExecutor compiles each rule's Pattern and returns a ready Executor.
// An error is returned if any pattern fails to compile.
func NewExecutor(policy RedactionPolicy) (*Executor, error) {
	specs := make([]rulecore.PatternSpec, len(policy.Rules))
	for i, r := range policy.Rules {
		specs[i] = rulecore.PatternSpec{ID: r.ID, Pattern: r.Pattern}
	}
	cs, err := rulecore.Compile(specs)
	if err != nil {
		msg := err.Error()
		const pfx = "rulecore: "
		if len(msg) > len(pfx) && msg[:len(pfx)] == pfx {
			msg = "redactionexecutor: " + msg[len(pfx):]
		}
		return nil, fmt.Errorf("%s", msg) //nolint:goerr113
	}

	// Build one single-rule CompiledSet per rule for occurrence counting.
	singleCS := make([]*rulecore.CompiledSet, len(policy.Rules))
	for i, r := range policy.Rules {
		sc, err2 := rulecore.Compile([]rulecore.PatternSpec{{ID: r.ID, Pattern: r.Pattern}})
		if err2 != nil {
			// Cannot happen: same patterns already compiled successfully above.
			panic("redactionexecutor: unexpected compile error on already-validated pattern: " + err2.Error())
		}
		singleCS[i] = sc
	}

	return &Executor{policy: policy, cs: cs, singleCS: singleCS}, nil
}

// Redact applies the policy to text and returns the redacted string together
// with a slice of Applied records — one entry per rule that fired at least
// once, in policy order, with occurrence counts totalled across all sweeps.
//
// Redact sweeps the rule list repeatedly until the output stabilizes (a
// textual fixpoint). A single sweep is NOT sufficient for completeness: one
// rule's replacement can create word boundaries that expose matches an
// earlier rule could not see in the same sweep. Iterating internally restores
// idempotency: for any policy that converges within MaxPasses sweeps,
// Redact(Redact(x)) == Redact(x).
//
// If the MaxPasses cap is reached without convergence (only possible with
// pathological, mutually re-triggering rules), Redact returns the final
// sweep's output as-is. Callers that need to detect that case should use
// RedactReport, which carries an explicit Converged indicator; Redact keeps
// its original two-value signature for source compatibility.
//
// Redact is deterministic: given the same text and the same policy, the
// output and Applied records are always identical.
func (e *Executor) Redact(text string) (string, []Applied) {
	result, report := e.RedactReport(text)
	return result, report.Applied
}

// RedactReport is Redact with full run provenance: it returns the redacted
// string and a Report describing how many sweeps ran, whether the output
// reached a fixpoint within MaxPasses, and which rules fired.
//
// Converged == false means the policy is pathological (its replacements keep
// re-triggering rules indefinitely); the returned text is the final sweep's
// output and may still contain rule matches.
func (e *Executor) RedactReport(text string) (string, Report) {
	result := text
	occurrences := make([]int, len(e.policy.Rules))
	var report Report

	for pass := 1; pass <= MaxPasses; pass++ {
		report.Passes = pass
		next := result

		for i, r := range e.policy.Rules {
			sc := e.singleCS[i]
			repl := map[string]string{r.ID: r.Replacement}

			// Count all non-overlapping matches in next before replacing.
			count := countMatches(sc, r.ID, next)
			if count == 0 {
				continue
			}

			// Replace all occurrences (single pass, literal replacement).
			replaced, _ := sc.ReplaceFixpoint(next, repl, 1)
			next = replaced
			occurrences[i] += count
		}

		if next == result {
			report.Converged = true
			break
		}
		result = next
	}

	for i, n := range occurrences {
		if n == 0 {
			continue
		}
		report.Applied = append(report.Applied, Applied{
			RuleID:      e.policy.Rules[i].ID,
			Occurrences: n,
		})
	}
	return result, report
}

// countMatches returns the number of non-overlapping matches of the single
// rule in sc against text. It advances past each match to find subsequent
// ones without allocating a FindAll result slice.
func countMatches(sc *rulecore.CompiledSet, _ string, text string) int {
	count := 0
	remaining := text
	for {
		findings := sc.ScanFirst(remaining)
		if len(findings) == 0 {
			break
		}
		count++
		end := findings[0].End
		start := findings[0].Start
		if end <= start {
			// Zero-length match: advance one byte to avoid infinite loop.
			end = start + 1
		}
		if end > len(remaining) {
			break
		}
		remaining = remaining[end:]
	}
	return count
}

package dlpevaluator

import (
	rulecore "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/rule-core"
)

// Evaluator scans text against a configured set of DLP rules and produces a
// leakage-risk Report. Create one with NewEvaluator; the zero value is not
// usable.
type Evaluator struct {
	rules []DLPRule
	cs    *rulecore.CompiledSet
}

// NewEvaluator compiles each rule's Pattern into a regexp and returns a ready
// Evaluator. An error is returned if any pattern fails to compile.
func NewEvaluator(rules []DLPRule) (*Evaluator, error) {
	specs := make([]rulecore.PatternSpec, len(rules))
	for i, r := range rules {
		specs[i] = rulecore.PatternSpec{ID: r.ID, Pattern: r.Pattern}
	}
	cs, err := rulecore.Compile(specs)
	if err != nil {
		return nil, reformatErr("dlpevaluator", err)
	}
	return &Evaluator{rules: rules, cs: cs}, nil
}

// reformatErr re-wraps a rulecore compile error, substituting the calling
// package name for the "rulecore:" prefix.
func reformatErr(pkg string, err error) error {
	msg := err.Error()
	const prefix = "rulecore: "
	if len(msg) > len(prefix) && msg[:len(prefix)] == prefix {
		msg = pkg + ": " + msg[len(prefix):]
	}
	return &pkgError{msg: msg, cause: err}
}

type pkgError struct {
	msg   string
	cause error
}

func (e *pkgError) Error() string { return e.msg }
func (e *pkgError) Unwrap() error { return e.cause }

// Evaluate tests text against every rule and returns a Report. The result is
// deterministic for a given (text, rules) pair.
func (e *Evaluator) Evaluate(text string) Report {
	findings := e.cs.ScanFirst(text)

	var dlpFindings []Finding
	maxRisk := RiskNone

	for _, f := range findings {
		var band RiskBand
		for _, r := range e.rules {
			if r.ID == f.RuleID {
				band = r.RiskBand
				break
			}
		}
		dlpFindings = append(dlpFindings, Finding{
			RuleID:   f.RuleID,
			RiskBand: band,
			Excerpt:  f.Excerpt,
		})
		if riskRank[band] > riskRank[maxRisk] {
			maxRisk = band
		}
	}

	return Report{
		Score:    riskScore(maxRisk),
		Findings: dlpFindings,
		MaxRisk:  maxRisk,
	}
}

// Evaluate is a package-level convenience function. It builds a temporary
// Evaluator from the provided rules and evaluates text. Returns an error if
// any rule pattern fails to compile.
func Evaluate(text string, rules []DLPRule) (Report, error) {
	ev, err := NewEvaluator(rules)
	if err != nil {
		return Report{}, err
	}
	return ev.Evaluate(text), nil
}

// riskScore converts a RiskBand to a normalised score in [0.0, 1.0].
// none→0.0, low→0.33, medium→0.67, high→1.0.
func riskScore(band RiskBand) float64 {
	switch band {
	case RiskLow:
		return 0.33
	case RiskMedium:
		return 0.67
	case RiskHigh:
		return 1.0
	default:
		return 0.0
	}
}

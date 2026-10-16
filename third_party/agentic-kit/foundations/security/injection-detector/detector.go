package injectiondetector

import (
	rulecore "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/rule-core"
)

// Detector scans untrusted content against a configured set of rules.
// Create one with NewDetector; the zero value is not usable.
type Detector struct {
	rules []Rule
	cs    *rulecore.CompiledSet
}

// NewDetector compiles each rule's Pattern into a regexp and returns a ready
// Detector. An error is returned if any pattern fails to compile.
func NewDetector(rules []Rule) (*Detector, error) {
	specs := make([]rulecore.PatternSpec, len(rules))
	for i, r := range rules {
		specs[i] = rulecore.PatternSpec{ID: r.ID, Pattern: r.Pattern}
	}
	cs, err := rulecore.Compile(specs)
	if err != nil {
		// Reformat to match the original error convention for this package.
		return nil, reformatErr("injectiondetector", err)
	}
	return &Detector{rules: rules, cs: cs}, nil
}

// reformatErr re-wraps a rulecore compile error to preserve the
// "injectiondetector: rule ..." message prefix expected by callers.
// rulecore already includes the rule ID and pattern so we just adjust
// the leading package name.
func reformatErr(pkg string, err error) error {
	// rulecore error format: "rulecore: rule X has invalid pattern Y: ..."
	// We re-emit the original error's text, replacing "rulecore:" with pkg.
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

// Scan tests content against every rule and returns a Result.
// The content is assumed to be untrusted external input.
func (d *Detector) Scan(content string) Result {
	findings := d.cs.ScanFirst(content)

	var matches []Match
	maxSev := Severity("")

	for _, f := range findings {
		// Look up the rule to get its Severity.
		var sev Severity
		for _, r := range d.rules {
			if r.ID == f.RuleID {
				sev = r.Severity
				break
			}
		}
		m := Match{
			RuleID:   f.RuleID,
			Severity: sev,
			Excerpt:  f.Excerpt,
		}
		matches = append(matches, m)
		if maxSev == "" || severityRank[sev] > severityRank[maxSev] {
			maxSev = sev
		}
	}

	return Result{
		Flagged:     len(matches) > 0,
		Matches:     matches,
		MaxSeverity: maxSev,
	}
}

// DefaultRules returns a small, generic built-in rule set suitable for
// detecting common prompt-injection and instruction-override patterns.
// These rules are intentionally generic — no org-specific literals.
// Callers may replace or extend them by supplying their own []Rule to
// NewDetector, e.g. loaded from a CUE-validated rule set.
func DefaultRules() []Rule {
	return []Rule{
		{
			ID:          "INJ-001",
			Pattern:     `(?i)(ignor(e|es|ed|ing)|disregard(s|ed|ing)?|forget(s|ting)?|bypass(es|ed|ing)?)\s+(?:\w+\s+){0,3}(previous|prior|above|earlier|preceding|all|any|your)\s+(?:\w+\s+){0,2}(instructions?|prompts?|rules?|directives?|guidelines?|context)`,
			Severity:    SeverityCritical,
			Description: "Attempts to nullify prior instructions or guidelines; tolerates up to three intervening words (e.g. \"ignore all of your previous instructions\"). Known FP class: benign asks to relax formatting rules (\"ignore your previous formatting rules\") — treat as a review signal.",
		},
		{
			ID:          "INJ-002",
			Pattern:     `(?i)disregard\s+(the\s+)?(above|previous|prior|all|everything)`,
			Severity:    SeverityCritical,
			Description: "Tells the model to disregard existing context.",
		},
		{
			ID:          "INJ-003",
			Pattern:     `(?i)you\s+are\s+now\s+(a|an|the)\s+[^\w\s]{0,3}\w+`,
			Severity:    SeverityHigh,
			Description: "Role-override attempt: reassigns the model's persona. Tolerates quotes/punctuation before the persona noun (e.g. \"you are now the 'developer mode' version\").",
		},
		{
			ID:          "INJ-004",
			Pattern:     `(?i)(print|output|reveal|show|display|repeat|write)\s+(your|the)\s+(system\s+prompt|instructions?|prompt|directives?|context)`,
			Severity:    SeverityHigh,
			Description: "Exfiltration attempt: requests the system prompt or internal instructions.",
		},
		{
			ID:          "INJ-005",
			Pattern:     `(?i)act\s+as\s+(if\s+)?(you\s+(have\s+no|are\s+without)\s+(restrictions?|limits?|guidelines?|rules?|constraints?)|a\s+jailbroken)`,
			Severity:    SeverityCritical,
			Description: "Jailbreak attempt: instructs the model to behave without restrictions.",
		},
		{
			ID:          "INJ-006",
			Pattern:     `(?i)forget\s+(everything|all|prior|previous|your\s+(previous|prior|earlier))\s*(you\s+were\s+(told|given|instructed))?`,
			Severity:    SeverityHigh,
			Description: "Instructs the model to forget prior context or training.",
		},
		{
			ID:          "INJ-007",
			Pattern:     `(?i)(new|updated|override)\s+(system\s+)?(prompt|instructions?|directives?)\s*:`,
			Severity:    SeverityCritical,
			Description: "Attempts to inject a replacement system prompt inline.",
		},
		{
			ID:          "INJ-008",
			Pattern:     `(?i)your\s+(real|true|actual)\s+(purpose|goal|objective|mission|instructions?)\s+is`,
			Severity:    SeverityMedium,
			Description: "Attempts to reframe the model's purpose.",
		},
		{
			ID:          "INJ-009",
			Pattern:     `(?i)(translate|repeat|echo)\s+.*\s+in\s+(plain\s+text|base64|hex|rot13)`,
			Severity:    SeverityLow,
			Description: "Encoding-exfiltration probe.",
		},
		{
			ID:          "INJ-010",
			Pattern:     `(?i)do\s+not\s+(follow|obey|respect|adhere\s+to)\s+(your\s+)?(previous|prior|above|safety|content)\s*(instructions?|guidelines?|rules?|policy|policies)?`,
			Severity:    SeverityCritical,
			Description: "Explicit instruction to violate safety or operational guidelines.",
		},
		{
			ID:          "INJ-011",
			Pattern:     `(?i)(use|using|with|via)\s+your\s+(?:[\w-]+\s+){0,2}tools?\b`,
			Severity:    SeverityHigh,
			Description: "Untrusted content directing the agent's own tools (e.g. \"use your browser tool to fetch …\"). Known FP class: idioms like \"use your favorite tool\" — treat as a review signal, not a block.",
		},
		{
			ID:          "INJ-012",
			Pattern:     `(?i)(send|email|post|upload|forward|transmit|exfiltrate)\s+(?:\w+\s+){0,4}(conversation|chat\s+history|message\s+history|credentials?|secrets?|passwords?|api\s+keys?|system\s+prompt)`,
			Severity:    SeverityCritical,
			Description: "Tool-mediated exfiltration ask: send the conversation, credentials, or the system prompt to an external party. Known FP class: legitimate asks to share a transcript.",
		},
		{
			ID:          "INJ-013",
			Pattern:     `(?i)\bdecode\s+(?:\w+\s+){0,3}(base\s*64|hex|rot13|binary)\b[^.?!\n]{0,80}\b(follow|execute|run|obey|comply|act\s+on)`,
			Severity:    SeverityHigh,
			Description: "Encoded-payload smuggling: decode base64/hex/rot13 content and then follow or execute it. Plain decode asks without a follow/execute clause do not fire.",
		},
		{
			ID:          "INJ-014",
			Pattern:     `(?i)(repeat|print|output|paste|echo|show|display|write)\s+(?:the\s+|your\s+|all\s+)?(text|words?|messages?|contents?|conversation|chat|prompt|everything)\s+(above|written\s+above|before\s+this|previously|so\s+far|earlier)`,
			Severity:    SeverityHigh,
			Description: "Indirect context exfiltration: repeat/print the text or conversation above. Known FP class: UI/layout prose like \"display the text above the button\".",
		},
		{
			ID:          "INJ-015",
			Pattern:     `(?i)(what\s+is|what'?s|tell\s+me|give\s+me|share)\s+your\s+(system\s+prompt|initial\s+prompt|hidden\s+prompt|original\s+instructions?|system\s+message)`,
			Severity:    SeverityHigh,
			Description: "Direct system-prompt exfiltration phrased as a question (\"what is your system prompt?\").",
		},
		{
			ID:          "INJ-016",
			Pattern:     `(?i)(pretend|imagine|role-?play|act\s+as)\b[^.?!\n]{0,60}\b(no|without|free\s+of)\s+(content\s+polic(y|ies)|restrictions?|filters?|filtering|guidelines?|limits?|limitations?|rules?|guardrails?|safety)`,
			Severity:    SeverityCritical,
			Description: "Role-play jailbreak: pretend/imagine a persona with no content policy, filters, or restrictions. Known FP class: creative-writing prompts (\"imagine a world with no rules\").",
		},
		{
			ID:          "INJ-017",
			Pattern:     `(?i)(ignore|bypass|disable|override|circumvent)\s+(?:\w+\s+){0,2}(tool|sandbox|permission|safety|security)\w*\s+(restrictions?|limits?|limitations?|checks?|polic(y|ies)|settings?|rules?|guardrails?)`,
			Severity:    SeverityCritical,
			Description: "Attempts to disable tool, sandbox, or safety restrictions. Known FP class: ops/dev prose like \"disable the security checks for local dev\" — treat as a review signal.",
		},
	}
}

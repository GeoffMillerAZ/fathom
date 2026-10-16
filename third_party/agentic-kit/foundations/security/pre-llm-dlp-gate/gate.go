package prellmdlpgate

import (
	"fmt"

	rulecore "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/rule-core"
)

// maxScanPasses is the hard cap on full rule sweeps a single Scan call will
// execute. It guards against pathological policies whose labels re-trigger a
// rule indefinitely (rules are caller data, so the gate cannot assume sanity).
// Realistic policies converge in one or two passes; hitting the cap without
// convergence is possible only with mutually re-triggering rules and is
// surfaced by the fact that the output may still contain rule matches.
const maxScanPasses = 16

// Gate scans and redacts text using a compiled RedactPolicy.
// Create one with NewGate or MustDefaultGate; the zero value is not usable.
// A Gate is immutable after construction and safe for concurrent use.
type Gate struct {
	policy   RedactPolicy
	singleCS []*rulecore.CompiledSet // one per rule, for per-rule scan + replace
}

// NewGate compiles each rule's Pattern and returns a ready Gate.
// Returns an error if any pattern fails to compile.
func NewGate(policy RedactPolicy) (*Gate, error) {
	singleCS := make([]*rulecore.CompiledSet, len(policy.Rules))
	for i, r := range policy.Rules {
		if r.Label == "" {
			return nil, fmt.Errorf("prellmdlpgate: rule %q has empty Label", r.ID)
		}
		sc, err := rulecore.Compile([]rulecore.PatternSpec{{ID: r.ID, Pattern: r.Pattern}})
		if err != nil {
			msg := err.Error()
			const pfx = "rulecore: "
			if len(msg) > len(pfx) && msg[:len(pfx)] == pfx {
				msg = "prellmdlpgate: " + msg[len(pfx):]
			}
			return nil, fmt.Errorf("%s", msg) //nolint:goerr113
		}
		singleCS[i] = sc
	}
	return &Gate{policy: policy, singleCS: singleCS}, nil
}

// Scan applies every rule in the policy to text in policy order, replacing
// matches with their configured Labels. It returns the redacted string and a
// slice of Findings (one per rule that matched at least once across all
// passes). Results are deterministic for a given input, and Scan is safe to
// call concurrently.
//
// Scan iterates to a fixpoint: after a full sweep it re-scans from the first
// rule until the output no longer changes, or until maxScanPasses is reached.
// This guarantees that one rule's label cannot expose a match that an earlier
// rule would have caught (overlapping or nested secrets that become visible
// only after a sibling rule fires). ReplaceAllLiteralString is used so label
// strings containing "$" are never expanded as backreference templates.
//
// Scan is best-effort secret-SHAPE redaction, not a guarantee: it only
// removes text that matches a rule pattern. Secrets that are encoded, split
// across segments, or simply shaped unlike any rule pass through untouched.
func (g *Gate) Scan(text string) (string, []Finding) {
	// fired[i] tracks whether rule i has been seen at least once (across all
	// passes) and records the first excerpt seen for it.
	type firstFinding struct {
		excerpt string
		seen    bool
	}
	fired := make([]firstFinding, len(g.policy.Rules))

	redacted := text
	for range maxScanPasses {
		next := redacted
		for i, r := range g.policy.Rules {
			sc := g.singleCS[i]
			repl := map[string]string{r.ID: r.Label}

			findings := sc.ScanFirst(next)
			if len(findings) == 0 {
				continue
			}
			if !fired[i].seen {
				fired[i] = firstFinding{excerpt: findings[0].Excerpt, seen: true}
			}
			// ReplaceAllLiteralString semantics via ReplaceFixpoint (1 pass, literal).
			replaced, _ := sc.ReplaceFixpoint(next, repl, 1)
			next = replaced
		}
		if next == redacted {
			redacted = next
			break
		}
		redacted = next
	}

	// Collect findings in policy order.
	var findings []Finding
	for i, ff := range fired {
		if !ff.seen {
			continue
		}
		findings = append(findings, Finding{
			RuleID:  g.policy.Rules[i].ID,
			Label:   g.policy.Rules[i].Label,
			Excerpt: ff.excerpt,
		})
	}

	return redacted, findings
}

// MustDefaultGate returns a Gate compiled from DefaultPolicy. It is the
// canonical infallible constructor for the shipped policy: the default rule
// set is fixed package data whose patterns are exercised by this package's
// tests, so compilation can only fail if the package itself is broken.
// MustDefaultGate panics in that impossible case — never on caller input —
// so consumers do not need to wrap NewGate(DefaultPolicy()) in error- or
// panic-handling boilerplate.
func MustDefaultGate() *Gate {
	g, err := NewGate(DefaultPolicy())
	if err != nil {
		panic("prellmdlpgate: internal error: shipped DefaultPolicy failed to compile: " + err.Error())
	}
	return g
}

// DefaultPolicy returns a small generic redaction policy suitable for common
// credential and identifier patterns. No org-specific literals are included.
// Callers may replace or extend it by supplying their own RedactPolicy to
// NewGate; see ExtendedPolicy for a larger shipped catalog. Like everything
// in this package, the policy is best-effort shape matching, not a guarantee.
func DefaultPolicy() RedactPolicy {
	return RedactPolicy{
		Name:    "generic-v1",
		Version: "1.0.0",
		Rules: []RedactRule{
			{
				ID:          "DLP-001",
				Pattern:     `(?i)(authorization\s*:\s*Bearer\s+)\S+`,
				Label:       "[REDACTED:BEARER]",
				Description: "Authorization Bearer token.",
			},
			{
				ID:          "DLP-002",
				Pattern:     `(?i)(api[_-]?key|apikey)\s*[=:]\s*\S+`,
				Label:       "[REDACTED:API_KEY]",
				Description: "API key assignment.",
			},
			{
				ID:          "DLP-003",
				Pattern:     `(?i)(password|passwd)\s*[=:]\s*\S+`,
				Label:       "[REDACTED:PASSWORD]",
				Description: "Password assignment.",
			},
			{
				ID:          "DLP-004",
				Pattern:     `\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`,
				Label:       "[REDACTED:CARD]",
				Description: "16-digit payment card number.",
			},
		},
	}
}

// ExtendedPolicy returns DefaultPolicy plus high-precision secret-shape rules
// (DLP-101…DLP-106) for well-known credential formats: PEM private-key
// blocks, AWS access key IDs, GitHub tokens, Slack tokens, JWTs, and Google
// API keys.
//
// The extended rules are deliberately false-positive-averse: each one anchors
// on a vendor-fixed prefix or framing plus strict length/alphabet
// constraints, so prose that merely mentions a prefix (a lone "AKIA", an
// "eyJ…"-prefixed word without dot-separated segments) does not trigger
// redaction. Broad-shape identifiers such as SSNs and email addresses are
// intentionally excluded — they redact far too much innocent text.
//
// The flip side of that precision is recall: truncated, re-encoded, or
// novel-format secrets will NOT match. This catalog reduces accidental
// leakage of recognizable credentials; it does not make text safe.
func ExtendedPolicy() RedactPolicy {
	p := DefaultPolicy()
	p.Name = "generic-extended-v1"
	p.Rules = append(p.Rules, []RedactRule{
		{
			ID: "DLP-101",
			Pattern: `(?s)-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY(?: BLOCK)?-----` +
				`.*?-----END (?:[A-Z0-9]+ )*PRIVATE KEY(?: BLOCK)?-----`,
			Label: "[REDACTED:PEM_PRIVATE_KEY]",
			Description: "PEM private-key block (RSA/EC/OPENSSH/PGP/PKCS#8 etc.). " +
				"Both BEGIN and END markers are required: matching a lone BEGIN " +
				"header would consume arbitrary trailing prose, so truncated blocks " +
				"are deliberately not matched.",
		},
		{
			ID:          "DLP-102",
			Pattern:     `\bAKIA[A-Z0-9]{16}\b`,
			Label:       "[REDACTED:AWS_ACCESS_KEY_ID]",
			Description: "AWS access key ID: AKIA followed by exactly 16 uppercase alphanumerics.",
		},
		{
			ID:          "DLP-103",
			Pattern:     `\b(?:gh[opsu]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})\b`,
			Label:       "[REDACTED:GITHUB_TOKEN]",
			Description: "GitHub token: classic ghp_/gho_/ghs_/ghu_ with a 36+ char body, or fine-grained github_pat_ with a 22+ char body.",
		},
		{
			ID:          "DLP-104",
			Pattern:     `\bxox[abprs]-[A-Za-z0-9-]{20,}\b`,
			Label:       "[REDACTED:SLACK_TOKEN]",
			Description: "Slack token: xoxb-/xoxa-/xoxp-/xoxr-/xoxs- with a 20+ char body.",
		},
		{
			ID:      "DLP-105",
			Pattern: `\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`,
			Label:   "[REDACTED:JWT]",
			Description: "JWT: three dot-separated base64url segments where header and " +
				"payload both decode to JSON objects (eyJ prefix). Unsigned alg=none " +
				"tokens (empty signature) are deliberately not matched.",
		},
		{
			ID:          "DLP-106",
			Pattern:     `\bAIza[0-9A-Za-z_-]{35}\b`,
			Label:       "[REDACTED:GOOGLE_API_KEY]",
			Description: "Google API key: AIza followed by exactly 35 base64url characters.",
		},
	}...)
	return p
}

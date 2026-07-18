// Package intentclassifier provides a configurable keyword/rule intent
// classifier. Intents are data — callers supply []Rule (from CUE or
// programmatically) rather than relying on hard-coded domain concepts.
// Classification is fully deterministic: no randomness, no ML calls.
//
// Concurrency: Classifier is safe for concurrent use once constructed; its rules are never mutated after NewClassifier returns.
package intentclassifier

import "strings"

// Intent is the label assigned to a query by the classifier.
// Values are defined by the caller via Rule.Intent; the only built-in
// sentinel is "unknown", returned when no rule matches.
type Intent string

// Unknown is the sentinel returned when no rule matches the query.
const Unknown Intent = "unknown"

// Rule maps an intent label to a list of keywords. A rule matches when
// the lower-cased query contains at least one keyword (exact substring).
// If multiple rules match, the first one in the slice wins (stable, first-match
// semantics). Rules are supplied by the caller — never hard-coded inside this
// package.
type Rule struct {
	// Intent is the label to return when this rule matches.
	Intent Intent

	// Keywords is the list of match terms. At least one keyword must be present.
	// Matching is case-insensitive exact substring.
	Keywords []string
}

// Classifier applies an ordered list of Rules to classify queries.
// Create one with NewClassifier; the zero value is not usable.
type Classifier struct {
	rules []Rule
}

// NewClassifier returns a Classifier that applies rules in order.
// Rules with an empty Keywords slice are silently skipped.
func NewClassifier(rules []Rule) *Classifier {
	filtered := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if len(r.Keywords) > 0 {
			filtered = append(filtered, r)
		}
	}
	return &Classifier{rules: filtered}
}

// Classify returns the Intent of the first Rule whose keyword appears as
// a case-insensitive substring of query. If no rule matches, it returns
// Unknown. The result is fully deterministic.
func (c *Classifier) Classify(query string) Intent {
	lower := strings.ToLower(query)
	for _, r := range c.rules {
		for _, kw := range r.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return r.Intent
			}
		}
	}
	return Unknown
}

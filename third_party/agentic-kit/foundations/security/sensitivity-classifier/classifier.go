package sensitivityclassifier

import (
	"fmt"
	"regexp"

	dst "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/data-sensitivity-types"
)

// Classifier scans text against a compiled Catalog of ClassRules.
// Create one with NewClassifier; the zero value is not usable.
type Classifier struct {
	catalog  Catalog
	compiled []*regexp.Regexp
}

// NewClassifier compiles each rule's Pattern and returns a ready Classifier.
// Returns an error if any pattern fails to compile.
func NewClassifier(catalog Catalog) (*Classifier, error) {
	compiled := make([]*regexp.Regexp, len(catalog.Rules))
	for i, r := range catalog.Rules {
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("sensitivityclassifier: rule %q has invalid pattern %q: %w", r.ID, r.Pattern, err)
		}
		compiled[i] = re
	}
	return &Classifier{catalog: catalog, compiled: compiled}, nil
}

// Classify scans text against every rule in the catalog and returns the
// Classification with the highest matched level. If no rule matches,
// the result is SensitivityPublic with a reason of "no rules matched".
// The result is deterministic for a given (text, catalog) pair.
func (c *Classifier) Classify(text string) dst.Classification {
	best := dst.SensitivityPublic
	reason := "no rules matched"

	for i, re := range c.compiled {
		if re.MatchString(text) {
			rule := c.catalog.Rules[i]
			if dst.LevelGTE(rule.Level, best) {
				best = rule.Level
				reason = fmt.Sprintf("rule %s: %s", rule.ID, rule.Description)
			}
		}
	}

	return dst.Classification{
		Level:  best,
		Reason: reason,
	}
}

// Classify is a package-level convenience function. It builds a temporary
// Classifier from the provided catalog and classifies text.
// Returns an error if any catalog pattern fails to compile.
func Classify(text string, catalog Catalog) (dst.Classification, error) {
	c, err := NewClassifier(catalog)
	if err != nil {
		return dst.Classification{}, err
	}
	return c.Classify(text), nil
}

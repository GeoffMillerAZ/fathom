// Package boundaries evaluates declarative rules over direct import edges
// from the dependency reader.
package boundaries

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrRules means a rule document did not meet the version 1 contract.
var ErrRules = errors.New("invalid boundary rules")

const maxRuleBytes = 1 << 20

// Rules is a validated, immutable ruleset. Its zero value has no rules.
type Rules struct{ rules []rule }

type rule struct {
	id         string
	from       []glob
	targets    []glob
	allowOnly  bool
	exceptions []exception
}

type exception struct{ from, to []glob }

type ruleDocument struct {
	SchemaVersion int        `json:"schema_version"`
	Rules         []ruleSpec `json:"rules"`
}

type ruleSpec struct {
	ID        string          `json:"id"`
	From      []string        `json:"from"`
	Deny      []string        `json:"deny"`
	AllowOnly []string        `json:"allow_only"`
	Except    []exceptionSpec `json:"except"`
	Reason    string          `json:"reason"`
}

type exceptionSpec struct {
	From []string `json:"from"`
	To   []string `json:"to"`
}

var (
	ruleID    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	ruleField = regexp.MustCompile(`^[a-z][a-z_]*$`)
)

// ParseRules reads a closed version 1 JSON document of at most 1 MiB.
// Diagnostic errors identify fields without echoing their values.
func ParseRules(src io.Reader) (Rules, error) {
	data, err := io.ReadAll(io.LimitReader(src, maxRuleBytes+1))
	if err != nil {
		return Rules{}, errors.New("cannot read boundary rules")
	}
	if len(data) > maxRuleBytes {
		return Rules{}, fmt.Errorf("%w: document exceeds 1 MiB", ErrRules)
	}
	if !utf8.Valid(data) || !closedJSON(json.NewDecoder(bytes.NewReader(data)), 0) {
		return Rules{}, fmt.Errorf("%w: expected one JSON object with unique, lowercase ASCII fields and non-null values", ErrRules)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc ruleDocument
	if err := dec.Decode(&doc); err != nil {
		return Rules{}, fmt.Errorf("%w: unexpected field or value type", ErrRules)
	}
	if doc.SchemaVersion != 1 || len(doc.Rules) == 0 {
		return Rules{}, fmt.Errorf("%w: expected schema_version 1 and at least one rule", ErrRules)
	}
	ids := make(map[string]bool, len(doc.Rules))
	var out Rules
	for i, spec := range doc.Rules {
		r, err := compileRule(spec)
		if err != nil {
			return Rules{}, fmt.Errorf("%w: rule %d: %v", ErrRules, i+1, err)
		}
		if ids[r.id] {
			return Rules{}, fmt.Errorf("%w: rule %d: duplicate id", ErrRules, i+1)
		}
		ids[r.id] = true
		out.rules = append(out.rules, r)
	}
	slices.SortFunc(out.rules, func(a, b rule) int { return cmp.Compare(a.id, b.id) })
	return out, nil
}

// encoding/json otherwise accepts duplicate keys, case aliases and null.
func closedJSON(dec *json.Decoder, depth int) bool {
	if depth > 8 {
		return false
	}
	token, err := dec.Token()
	if err != nil || token == nil {
		return false
	}
	switch token {
	case json.Delim('{'):
		fields := make(map[string]bool)
		for dec.More() {
			token, err := dec.Token()
			key, ok := token.(string)
			if err != nil || !ok || !ruleField.MatchString(key) || fields[key] || !closedJSON(dec, depth+1) {
				return false
			}
			fields[key] = true
		}
	case json.Delim('['):
		for dec.More() {
			if !closedJSON(dec, depth+1) {
				return false
			}
		}
	default:
		return depth != 0
	}
	if _, err := dec.Token(); err != nil {
		return false
	}
	if depth != 0 {
		return true
	}
	_, err = dec.Token()
	return token == json.Delim('{') && err == io.EOF
}

func compileRule(spec ruleSpec) (rule, error) {
	var r rule
	if !ruleID.MatchString(spec.ID) {
		return r, errors.New("id must start with a lowercase letter and use lowercase letters, digits and hyphens")
	}
	if strings.TrimSpace(spec.Reason) == "" || strings.ContainsFunc(spec.Reason, func(ch rune) bool {
		return unicode.IsControl(ch) || ch == '\u2028' || ch == '\u2029'
	}) {
		return r, errors.New("reason must be nonblank and on one line")
	}
	if (spec.Deny == nil) == (spec.AllowOnly == nil) {
		return r, errors.New("expected exactly one of deny or allow_only")
	}
	r.id = spec.ID
	var err error
	r.from, err = compileGlobs(spec.From, false)
	if err != nil {
		return r, fmt.Errorf("from: %w", err)
	}
	r.allowOnly = spec.AllowOnly != nil
	targets, field := spec.Deny, "deny"
	if r.allowOnly {
		targets, field = spec.AllowOnly, "allow_only"
	}
	r.targets, err = compileGlobs(targets, true)
	if err != nil {
		return r, fmt.Errorf("%s: %w", field, err)
	}
	for i, spec := range spec.Except {
		from, err := compileGlobs(spec.From, false)
		if err != nil {
			return r, fmt.Errorf("except entry %d from: %w", i+1, err)
		}
		to, err := compileGlobs(spec.To, true)
		if err != nil {
			return r, fmt.Errorf("except entry %d to: %w", i+1, err)
		}
		r.exceptions = append(r.exceptions, exception{from, to})
	}
	return r, nil
}

// Governs reports whether a logical repository file is selected by any rule.
func (r Rules) Governs(file string) bool {
	return validDependencyPath(file) && slices.ContainsFunc(r.rules, func(r rule) bool { return matches(r.from, "", file) })
}

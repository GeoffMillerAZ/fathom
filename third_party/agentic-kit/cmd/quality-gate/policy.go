package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxPolicyBytes = 1 << 20

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
var calibrationSitePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/(pre-commit|pre-push|ci|pre-publish)/[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// decodePolicy rejects ambiguous JSON before decoding into the typed policy.
// Validation is identical for file and committed-blob callers.
func decodePolicy(data []byte) (policy, error) {
	if len(data) == 0 || len(data) > maxPolicyBytes || !utf8.Valid(data) {
		return policy{}, errors.New("invalid policy size or encoding")
	}
	keys := json.NewDecoder(bytes.NewReader(data))
	if err := rejectAmbiguousJSON(keys, 0); err != nil {
		return policy{}, err
	}
	if _, err := keys.Token(); !errors.Is(err, io.EOF) {
		return policy{}, errors.New("trailing policy content")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p policy
	if err := dec.Decode(&p); err != nil {
		return policy{}, fmt.Errorf("invalid policy JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return policy{}, errors.New("trailing policy content")
	}
	if err := rejectWrongRuleOptions(data, p.Rules); err != nil {
		return policy{}, err
	}
	if err := validatePolicy(p); err != nil {
		return policy{}, err
	}
	return p, nil
}

func rejectAmbiguousJSON(dec *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("policy JSON nesting exceeds limit")
	}
	tok, err := dec.Token()
	if err != nil {
		return errors.New("invalid policy JSON")
	}
	if tok == nil {
		return errors.New("null is not allowed in policy")
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return errors.New("invalid policy object key")
			}
			name, ok := key.(string)
			if !ok || !canonicalPolicyKey(name) || seen[name] {
				return errors.New("duplicate or invalid policy key")
			}
			seen[name] = true
			if err := rejectAmbiguousJSON(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectAmbiguousJSON(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid policy JSON delimiter")
	}
	want := json.Delim('}')
	if delim == '[' {
		want = ']'
	}
	end, err := dec.Token()
	if err != nil || end != want {
		return errors.New("unterminated policy JSON")
	}
	return nil
}

func canonicalPolicyKey(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// Typed zero values cannot distinguish an omitted field from an explicit zero.
// Preserve the rule-option presence check before validating the typed value.
func rejectWrongRuleOptions(data []byte, rules []rule) error {
	var raw struct {
		Rules *[]map[string]json.RawMessage `json:"rules"`
		Model struct {
			Sites      *[]map[string]json.RawMessage `json:"sites"`
			MaxBatches *int                          `json:"max_batches"`
		} `json:"model"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.Rules == nil || raw.Model.Sites == nil || len(*raw.Rules) != len(rules) {
		return errors.New("policy rules and sites must be explicit arrays")
	}
	if raw.Model.MaxBatches != nil && (*raw.Model.MaxBatches < 1 || *raw.Model.MaxBatches > 32) {
		return errors.New("max_batches must be 1 through 32 when present")
	}
	for i, fields := range *raw.Rules {
		if _, present := fields["max_bytes"]; present && rules[i].Kind != "max-bytes" {
			return errors.New("max_bytes belongs only to max-bytes rules")
		}
		if _, present := fields["companion"]; present && rules[i].Kind != "require-companion" {
			return errors.New("companion belongs only to require-companion rules")
		}
	}
	for _, site := range *raw.Model.Sites {
		if calibrationJSON, present := site["calibration"]; present {
			var calibrationFields map[string]json.RawMessage
			if err := json.Unmarshal(calibrationJSON, &calibrationFields); err != nil {
				return errors.New("invalid calibration encoding")
			}
			if _, present := calibrationFields["cases"]; !present {
				return errors.New("calibration cases must be explicit")
			}
		}
	}
	return nil
}

func validatePolicy(p policy) error {
	if p.Version != 1 || !safeID(p.ID) {
		return errors.New("unsupported policy version or ID")
	}
	if p.Limits.MaxFiles < 1 || p.Limits.MaxFiles > 10000 ||
		p.Limits.MaxFileBytes < 1 || p.Limits.MaxFileBytes > 16<<20 ||
		p.Limits.MaxTotalBytes < 1 || p.Limits.MaxTotalBytes > 16<<20 ||
		p.Limits.TimeoutSeconds < 1 || p.Limits.TimeoutSeconds > 300 {
		return errors.New("policy limits out of range")
	}
	if len(p.Rules) > 1000 || len(p.Model.Sites) > 100 || len(p.Rules)+len(p.Model.Sites) == 0 {
		return errors.New("policy needs bounded rules or sites")
	}
	seen := map[string]bool{}
	for _, r := range p.Rules {
		if !safeID(r.ID) || seen[r.ID] {
			return errors.New("invalid or duplicate rule ID")
		}
		seen[r.ID] = true
		if err := validateBoundaries(r.Boundaries); err != nil {
			return err
		}
		if err := validateSelector(r.Select); err != nil {
			return err
		}
		switch r.Kind {
		case "conflict-markers", "forbidden-path", "json-valid":
			if r.MaxBytes != 0 || r.Companion != nil {
				return errors.New("rule has unsupported options")
			}
		case "max-bytes":
			if r.MaxBytes < 1 || r.MaxBytes > 16<<20 || r.Companion != nil {
				return errors.New("invalid max-bytes rule")
			}
		case "require-companion":
			if r.MaxBytes != 0 || r.Companion == nil {
				return errors.New("invalid companion rule")
			}
			if err := validateSelector(*r.Companion); err != nil {
				return err
			}
		default:
			return errors.New("unsupported rule kind")
		}
	}
	m := p.Model
	if m.Provider != "none" && m.Provider != "openai" && m.Provider != "typesafe" && m.Provider != "openrouter" {
		return errors.New("unsupported model provider")
	}
	if m.MaxInputBytes < 1 || m.MaxInputBytes > 1<<20 || m.TimeoutSeconds < 1 || m.TimeoutSeconds > 120 ||
		m.CacheTTLSeconds < 0 || m.CacheTTLSeconds > 86400 || m.MaxBatches < 0 || m.MaxBatches > 32 {
		return errors.New("model limits out of range")
	}
	for _, name := range []string{m.ModelEnv, m.ExpectedModelEnv, m.APIKeyEnv} {
		if name != "" && !envNamePattern.MatchString(name) {
			return errors.New("invalid model environment name")
		}
	}
	if m.Provider != "none" && (m.ModelEnv == "" || m.ExpectedModelEnv == "" || m.APIKeyEnv == "") {
		return errors.New("remote provider needs environment names")
	}
	for _, s := range m.Sites {
		if !safeID(s.ID) || seen[s.ID] {
			return errors.New("invalid or duplicate site ID")
		}
		seen[s.ID] = true
		if !safeID(s.Version) || !plainText(s.Question, 1, 16384) ||
			(s.Mode != "shadow" && s.Mode != "hold") || !positiveUnit(s.MinConfidence) {
			return errors.New("invalid model site")
		}
		if err := validateBoundaries(s.Boundaries); err != nil {
			return err
		}
		if err := validateSelector(s.Select); err != nil {
			return err
		}
		if s.Calibration != nil {
			if err := validateCalibration(s.Calibration, m.Provider); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCalibration(c *calibration, provider string) error {
	if !calibrationSitePattern.MatchString(c.Site) || !hex64(c.QuestionSHA256) ||
		c.Provider != expectedProvider(provider) || !singleLineText(c.ResolvedModel, 1, 256) ||
		c.ValidUntil.IsZero() || c.MinPerClass < 1 || c.MinPerClass > 10000 ||
		c.MinN < 1 || c.MinN > 10000 || !positiveUnit(c.MinAccuracy) || !positiveUnit(c.MinWilsonLo) ||
		len(c.Cases) > 10000 {
		return errors.New("invalid calibration")
	}
	seen := map[string]bool{}
	for _, v := range c.Cases {
		if !safeID(v.ID) || seen[v.ID] || !modelChoice(v.Expected) || !modelChoice(v.Actual) ||
			(v.Provenance != "human" && v.Provenance != "computed") || !unit(v.Confidence) {
			return errors.New("invalid calibration case")
		}
		seen[v.ID] = true
	}
	return nil
}

func validateBoundaries(values []string) error {
	if len(values) == 0 || len(values) > 4 {
		return errors.New("invalid boundaries")
	}
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] || (v != "pre-commit" && v != "pre-push" && v != "ci" && v != "pre-publish") {
			return errors.New("invalid or duplicate boundary")
		}
		seen[v] = true
	}
	return nil
}

func validateSelector(s selector) error {
	if len(s.Prefixes) == 0 || len(s.Prefixes) > 1000 || len(s.Suffixes) > 1000 {
		return errors.New("selector needs bounded prefixes")
	}
	for groupIndex, group := range [][]string{s.Prefixes, s.Suffixes} {
		seen := map[string]bool{}
		for _, value := range group {
			if seen[value] || !safeSelectorPart(value, groupIndex == 0) {
				return errors.New("invalid or duplicate selector path")
			}
			seen[value] = true
		}
	}
	return nil
}

func safeSelectorPart(s string, allowEmpty bool) bool {
	if s == "" {
		return allowEmpty
	}
	if len(s) > 1024 || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\:\x00") ||
		strings.Contains(s, "//") || strings.Contains(s, "@{") {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

func safeID(s string) bool { return safeIDPattern.MatchString(s) }

func plainText(s string, min, max int) bool {
	if len(s) < min || len(s) > max || !utf8.ValidString(s) || strings.TrimSpace(s) == "" {
		return false
	}
	for _, c := range s {
		if c == 0 || c == 0x7f || (c < 0x20 && c != '\n' && c != '\t') {
			return false
		}
	}
	return true
}

func singleLineText(s string, min, max int) bool {
	if len(s) < min || len(s) > max || !utf8.ValidString(s) || strings.TrimSpace(s) == "" {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

func hex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func unit(v float64) bool         { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func positiveUnit(v float64) bool { return unit(v) && v > 0 }

func includes(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func matches(s selector, path string) bool {
	prefix := false
	for _, p := range s.Prefixes {
		if strings.HasPrefix(path, p) {
			prefix = true
			break
		}
	}
	if !prefix {
		return false
	}
	if len(s.Suffixes) == 0 {
		return true
	}
	for _, suffix := range s.Suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var safeModelRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+@-]{0,255}$`)
var secretSignal = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|bearer|private[_-]?key|AKIA[0-9A-Z]{16})`)
var kinds = []string{"read-range", "search-literal", "git-status", "git-diff", "run-check", "write-file", "shell"}

func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func id(s string) bool { return safeID.MatchString(s) }

func validateAssess(s Scope, p Policy, r Operation) error {
	if s.Version != "command-lab.scope/1" || p.Version != "command-lab.policy/1" || r.Version != "command-lab.request/1" || !id(r.ID) || !contains(kinds, r.Kind) || r.MaxBytes < 1 || r.MaxResults < 1 {
		return errors.New("invalid_control")
	}
	if len(s.RequiredLayerIDs) < 1 || len(s.RequiredLayerIDs) > 32 || len(p.Layers) < 1 || len(p.Layers) > 32 {
		return errors.New("invalid_layers")
	}
	req := map[string]bool{}
	for _, x := range s.RequiredLayerIDs {
		if !id(x) || req[x] {
			return errors.New("invalid_scope")
		}
		req[x] = true
	}
	seen := map[string]bool{}
	for _, l := range p.Layers {
		if !id(l.ID) || seen[l.ID] || !req[l.ID] || !contains([]string{"organization", "team", "global", "project", "personal"}, l.Scope) || l.MaxBytes < 1 || l.MaxResults < 1 || len(l.AllowedOperations) < 1 {
			return errors.New("invalid_policy")
		}
		seen[l.ID] = true
		ops := map[string]bool{}
		for _, k := range l.AllowedOperations {
			if !contains(kinds, k) || ops[k] {
				return errors.New("invalid_policy")
			}
			ops[k] = true
		}
	}
	if len(seen) != len(req) {
		return errors.New("layer_set_mismatch")
	}
	if p.Classifier != nil && (!id(p.Classifier.Provider) || !safeModelRef.MatchString(p.Classifier.ResolvedModel)) {
		return errors.New("invalid_classifier")
	}
	return nil
}
func pathCode(s string) string {
	if s == "" {
		return "path_absent"
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") || len(s) > 1 && s[1] == ':' {
		return "path_absolute"
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "path_control"
		}
	}
	if strings.Contains(s, "\\") || path.Clean(s) != s || s == "." || strings.HasPrefix(s, "../") || s == ".." || strings.Contains(s, "/../") || strings.HasSuffix(s, "/..") {
		return "path_traversal"
	}
	return "path_relative"
}
func boundCategory(n int) string {
	switch {
	case n <= 1024:
		return "small"
	case n <= 1<<20:
		return "medium"
	default:
		return "large"
	}
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func projectedRequest(s Scope, p Policy, r Operation, signals []string) decision.Request {
	if signals == nil {
		signals = []string{}
	}
	ids := append([]string(nil), s.RequiredLayerIDs...)
	sort.Strings(ids)
	layers := append([]Layer(nil), p.Layers...)
	sort.Slice(layers, func(i, j int) bool { return layers[i].ID < layers[j].ID })
	for i := range layers {
		layers[i].AllowedOperations = append([]string(nil), layers[i].AllowedOperations...)
		sort.Strings(layers[i].AllowedOperations)
	}
	state := map[string]any{"binding": hash(struct {
		ScopeIDs   []string
		Layers     []Layer
		Classifier *Classifier
		RequestID  string
	}{ids, layers, p.Classifier, r.ID}), "kind": r.Kind, "pathCategory": pathCode(r.Path), "bytesCategory": boundCategory(r.MaxBytes), "resultsCategory": boundCategory(r.MaxResults), "signals": signals}
	b, _ := json.Marshal(state)
	return decision.Request{State: string(b), Questions: []decision.Question{{ID: "command-concern", Kind: decision.KindChoice, Instructions: "Does the sanitized proposed operation raise a concern? Choose unknown when evidence is insufficient.", Options: []decision.Option{{Name: "concern"}, {Name: "no-concern"}, {Name: "unknown"}}}}}
}
func assess(s Scope, p Policy, r Operation, cap *Capture, dec decision.Decider) (AssessReport, error) {
	out := AssessReport{Version: "command-lab.assess/1", Status: "incomplete", NoActionAuthority: true, EffectiveAllowed: []string{}, Codes: []string{}, ClassifierStatus: "none"}
	if err := validateAssess(s, p, r); err != nil {
		return out, err
	}
	allowed := map[string]bool{}
	for _, k := range kinds {
		allowed[k] = true
	}
	out.EffectiveMaxBytes = p.Layers[0].MaxBytes
	out.EffectiveMaxResults = p.Layers[0].MaxResults
	for _, l := range p.Layers {
		if l.MaxBytes < out.EffectiveMaxBytes {
			out.EffectiveMaxBytes = l.MaxBytes
		}
		if l.MaxResults < out.EffectiveMaxResults {
			out.EffectiveMaxResults = l.MaxResults
		}
		for k := range allowed {
			if !contains(l.AllowedOperations, k) {
				delete(allowed, k)
			}
		}
	}
	for _, k := range kinds {
		if allowed[k] {
			out.EffectiveAllowed = append(out.EffectiveAllowed, k)
		}
	}
	if !allowed[r.Kind] {
		out.Codes = append(out.Codes, "operation_disallowed")
	}
	if r.MaxBytes > out.EffectiveMaxBytes {
		out.Codes = append(out.Codes, "bytes_exceeded")
	}
	if r.MaxResults > out.EffectiveMaxResults {
		out.Codes = append(out.Codes, "results_exceeded")
	}
	pc := pathCode(r.Path)
	if pc == "path_absolute" || pc == "path_control" || pc == "path_traversal" {
		out.Codes = append(out.Codes, pc)
	}
	if secretSignal.MatchString(r.Path) || secretSignal.MatchString(r.Command) {
		out.Codes = append(out.Codes, "credential_signal")
	}
	if contains([]string{"shell", "write-file", "run-check"}, r.Kind) {
		out.Codes = append(out.Codes, "unsupported_effect")
	}
	if (r.Kind == "read-range" || r.Kind == "search-literal") && r.Path == "" {
		out.Codes = append(out.Codes, "path_required")
	}
	if r.Kind == "search-literal" && r.Command == "" {
		out.Codes = append(out.Codes, "literal_required")
	}
	if r.Kind != "search-literal" && r.Kind != "shell" && r.Kind != "run-check" && r.Command != "" {
		out.Codes = append(out.Codes, "unexpected_command")
	}
	sort.Strings(out.Codes)
	if len(out.Codes) > 0 {
		out.Status = "blocked"
		return out, nil
	}
	if p.Classifier != nil || cap != nil {
		signals := []string{}
		if contains(out.Codes, "credential_signal") {
			signals = append(signals, "credential")
		}
		if contains(out.Codes, "path_absolute") || contains(out.Codes, "path_traversal") || contains(out.Codes, "path_control") {
			signals = append(signals, "unsafe-path")
		}
		req := projectedRequest(s, p, r, signals)
		if err := decision.Validate(req); err != nil {
			return out, errors.New("classifier_request_invalid")
		}
		out.RequestKey = decision.RequestKey(req)
		if cap != nil {
			if p.Classifier == nil {
				return out, errors.New("capture_without_pin")
			}
			choice, confidence, status, err := checkCapture(*cap, *p.Classifier, req)
			if err != nil {
				return out, err
			}
			out.Choice = choice
			out.Confidence = confidence
			out.ClassifierStatus = status
		} else if dec != nil {
			resp, err := dec.Decide(context.Background(), req)
			if err != nil {
				out.ClassifierStatus = "unknown"
			} else {
				choice, confidence, status, e := checkResponse(resp, p.Classifier, req)
				if e != nil {
					return out, e
				}
				out.Choice = choice
				out.Confidence = confidence
				out.ClassifierStatus = status
			}
		}
	}
	out.Status = "advisory"
	return out, nil
}
func checkCapture(c Capture, p Classifier, req decision.Request) (string, float64, string, error) {
	if c.Version != "command-lab.capture/1" || c.RequestKey != decision.RequestKey(req) {
		return "", 0, "", errors.New("capture_binding")
	}
	if c.Cause != "" {
		if c.Response != nil || !contains([]string{"no-provider", "timeout", "transport-error", "refused"}, c.Cause) {
			return "", 0, "", errors.New("capture_shape")
		}
		return "", 0, "unknown", nil
	}
	if c.Response == nil {
		return "", 0, "", errors.New("capture_missing")
	}
	resp := decision.Response{Provider: c.Response.Provider, Model: c.Response.Model, Answers: c.Response.Answers, Invalid: c.Response.Invalid, Declined: c.Response.Declined, Usage: c.Response.Usage}
	if c.Response.Latency < 0 {
		return "", 0, "", errors.New("capture_metadata")
	}
	return checkResponse(resp, &p, req)
}
func checkResponse(resp decision.Response, p *Classifier, req decision.Request) (string, float64, string, error) {
	if p == nil || resp.Provider != p.Provider || resp.Model != p.ResolvedModel || resp.Usage.InputTokens < 0 || resp.Usage.OutputTokens < 0 {
		return "", 0, "", errors.New("capture_metadata")
	}
	id := "command-concern"
	for k := range resp.Answers {
		if k != id {
			return "", 0, "", errors.New("capture_foreign")
		}
	}
	for k := range resp.Invalid {
		if k != id {
			return "", 0, "", errors.New("capture_foreign")
		}
	}
	for k := range resp.Declined {
		if k != id {
			return "", 0, "", errors.New("capture_foreign")
		}
	}
	a, hasA := resp.Answers[id]
	invalid, hasI := resp.Invalid[id]
	declined, hasD := resp.Declined[id]
	n := 0
	for _, b := range []bool{hasA, hasI, hasD} {
		if b {
			n++
		}
	}
	if n != 1 {
		return "", 0, "", errors.New("capture_outcomes")
	}
	if hasI {
		if strings.TrimSpace(invalid) == "" {
			return "", 0, "", errors.New("capture_reason")
		}
		return "", 0, "unknown", nil
	}
	if hasD {
		if strings.TrimSpace(declined) == "" {
			return "", 0, "", errors.New("capture_reason")
		}
		return "", 0, "unknown", nil
	}
	if a.ID != id || a.Kind != decision.KindChoice || a.Score != 0 || a.Noul != 0 || !contains([]string{"concern", "no-concern", "unknown"}, a.Choice) || len(a.Probabilities) != 3 {
		return "", 0, "", errors.New("capture_answer")
	}
	sum := 0.0
	max := -1.0
	for _, k := range []string{"concern", "no-concern", "unknown"} {
		v, ok := a.Probabilities[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return "", 0, "", errors.New("capture_probability")
		}
		sum += v
		if v > max {
			max = v
		}
	}
	if math.Abs(sum-1) > 0.01 || a.Probabilities[a.Choice] < max || math.IsNaN(a.Confidence) || math.IsInf(a.Confidence, 0) || math.Abs(a.Confidence-decision.Confidence(a.Probabilities)) > 1e-12 {
		return "", 0, "", errors.New("capture_confidence")
	}
	if a.Choice == "unknown" {
		return "unknown", a.Confidence, "unknown", nil
	}
	return a.Choice, a.Confidence, "answered", nil
}

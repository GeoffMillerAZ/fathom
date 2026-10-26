package main

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func validID(s string) bool { return idPattern.MatchString(s) }

func (p Pack) validate() error {
	if p.Version != "decision-lab.pack/1" {
		return errors.New("unsupported pack version")
	}
	if p.ConfidenceRecipe != "decision.Confidence/1" || p.ReadingRecipe != "decision.NewReading/1" {
		return errors.New("unsupported decision recipe")
	}
	for _, s := range []string{p.ID, p.TemplateVersion, p.Provider, p.ResolvedModel, p.PolicyEpoch, p.Owner} {
		if !validID(s) {
			return errors.New("invalid pack identity")
		}
	}
	if strings.TrimSpace(p.Fallback) == "" || strings.TrimSpace(p.Baseline) == "" || strings.TrimSpace(p.KillCriterion) == "" {
		return errors.New("fallback, baseline and kill criterion required")
	}
	if len(p.Questions) < 1 || len(p.Questions) > 32 {
		return errors.New("pack requires 1..32 questions")
	}
	seen := map[string]bool{}
	for _, q := range p.Questions {
		if !validID(q.ID) || seen[q.ID] || strings.TrimSpace(q.Instructions) == "" {
			return errors.New("invalid or duplicate question")
		}
		seen[q.ID] = true
		if len(q.Options) < 2 || len(q.Options) > 16 {
			return errors.New("question requires 2..16 options")
		}
		options := map[string]bool{}
		for _, o := range q.Options {
			if !validID(o.Name) || options[o.Name] || strings.TrimSpace(o.Criteria) == "" {
				return errors.New("invalid or duplicate option")
			}
			options[o.Name] = true
		}
		if !options["unknown"] {
			return errors.New("question needs unknown option")
		}
		if len(q.FavorableOptions) == 0 {
			return errors.New("favorable options required")
		}
		favor := map[string]bool{}
		for _, s := range q.FavorableOptions {
			if !options[s] || s == "unknown" || favor[s] {
				return errors.New("invalid favorable option")
			}
			favor[s] = true
		}
	}
	return nil
}

func (in Input) validate() error {
	if in.Version != "decision-lab.input/1" {
		return errors.New("unsupported input version")
	}
	if !validID(in.CaseID) || !validID(in.SubjectID) || !validID(in.Revision) || strings.TrimSpace(in.CapturedState) == "" {
		return errors.New("invalid input identity or empty captured state")
	}
	return nil
}

func request(p Pack, in Input) decision.Request {
	qs := make([]decision.Question, 0, len(p.Questions))
	for _, q := range p.Questions {
		opts := make([]decision.Option, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, decision.Option{Name: o.Name, Criteria: o.Criteria})
		}
		qs = append(qs, decision.Question{ID: q.ID, Kind: decision.KindChoice, Instructions: q.Instructions, Options: opts, Levels: []string{}})
	}
	return decision.Request{State: in.CapturedState, Questions: qs}
}

func prepared(p Pack, in Input) (Prepared, error) {
	if err := p.validate(); err != nil {
		return Prepared{}, err
	}
	if err := in.validate(); err != nil {
		return Prepared{}, err
	}
	r := request(p, in)
	if err := decision.Validate(r); err != nil {
		return Prepared{}, err
	}
	d := map[string]string{}
	for _, q := range r.Questions {
		d[q.ID] = decision.TemplateDigest(q, p.TemplateVersion)
	}
	return Prepared{Version: "decision-lab.prepared/1", CaseID: in.CaseID, PackDigest: digest(canonical(p)), InputDigest: digest(canonical(in)), RequestKey: decision.RequestKey(r), TemplateDigests: d, ConfidenceRecipe: p.ConfidenceRecipe, NoActionAuthority: true, Request: r}, nil
}

type CheckResult struct {
	Version           string                    `json:"version"`
	Status            string                    `json:"status"`
	Reason            string                    `json:"reason"`
	CaseID            string                    `json:"caseId"`
	PackDigest        string                    `json:"packDigest"`
	InputDigest       string                    `json:"inputDigest"`
	RequestKey        string                    `json:"requestKey"`
	Outcomes          map[string]string         `json:"outcomes"`
	ObservedAnswers   map[string]ObservedAnswer `json:"observedAnswers"`
	AttemptCount      int                       `json:"attemptCount"`
	AnswerCacheHits   []string                  `json:"answerCacheHits"`
	NoActionAuthority bool                      `json:"noActionAuthority"`
}

type ObservedAnswer struct {
	Choice         string  `json:"choice"`
	Confidence     float64 `json:"confidence"`
	AnswerCacheHit bool    `json:"answerCacheHit"`
}

func check(p Pack, in Input, c Capture) (CheckResult, error) {
	pr, err := prepared(p, in)
	if err != nil {
		return CheckResult{}, err
	}
	r := CheckResult{Version: "decision-lab.check/1", Status: "unusable", Reason: "capture_unusable", CaseID: in.CaseID, PackDigest: pr.PackDigest, InputDigest: pr.InputDigest, RequestKey: pr.RequestKey, Outcomes: map[string]string{}, ObservedAnswers: map[string]ObservedAnswer{}, AttemptCount: c.AttemptCount, AnswerCacheHits: c.AnswerCacheHits, NoActionAuthority: true}
	if c.Version != "decision-lab.capture/1" || c.CaseID != in.CaseID || c.PackDigest != pr.PackDigest || c.InputDigest != pr.InputDigest || c.RequestKey != pr.RequestKey {
		return r, errors.New("capture binding mismatch")
	}
	if c.AttemptCount < 0 || c.AttemptCount > 32 {
		return r, errors.New("invalid declared attempt count")
	}
	if c.Cause != "" {
		if c.Response != nil {
			return r, errors.New("cause and response are exclusive")
		}
		causes := map[string]bool{"no-provider": true, "timeout": true, "not-authorized": true, "budget-exhausted": true, "transport-error": true}
		if !causes[c.Cause] {
			return r, errors.New("invalid captured cause")
		}
		if len(c.AnswerCacheHits) > 0 {
			return r, errors.New("failed capture cannot reuse answers")
		}
		if (c.Cause == "no-provider" || c.Cause == "not-authorized" || c.Cause == "budget-exhausted") && c.AttemptCount != 0 {
			return r, errors.New("preflight failure cannot have attempts")
		}
		if (c.Cause == "timeout" || c.Cause == "transport-error") && c.AttemptCount < 1 {
			return r, errors.New("attempted failure needs attempt count")
		}
		r.Reason = c.Cause
		for _, q := range p.Questions {
			r.Outcomes[q.ID] = "failed"
		}
		return r, nil
	}
	if c.Response == nil {
		return r, errors.New("capture needs response or cause")
	}
	resp := c.Response
	if resp.Provider != p.Provider || resp.Model != p.ResolvedModel || resp.Usage.InputTokens < 0 || resp.Usage.OutputTokens < 0 || resp.Latency < 0 {
		return r, errors.New("response metadata mismatch")
	}
	ids := map[string]bool{}
	for _, q := range p.Questions {
		ids[q.ID] = true
	}
	for id := range resp.Answers {
		if !ids[id] {
			return r, errors.New("foreign answer ID")
		}
	}
	for id := range resp.Invalid {
		if !ids[id] {
			return r, errors.New("foreign invalid ID")
		}
	}
	for id := range resp.Declined {
		if !ids[id] {
			return r, errors.New("foreign declined ID")
		}
	}
	hits := map[string]bool{}
	for _, id := range c.AnswerCacheHits {
		if !ids[id] || hits[id] {
			return r, errors.New("invalid or duplicate answer cache ID")
		}
		if _, ok := resp.Answers[id]; !ok {
			return r, errors.New("answer cache ID must name a valid answer")
		}
		hits[id] = true
	}
	r.AnswerCacheHits = append([]string{}, c.AnswerCacheHits...)
	sort.Strings(r.AnswerCacheHits)
	if len(hits) < len(p.Questions) && c.AttemptCount < 1 {
		return r, errors.New("fresh response outcome requires an attempt")
	}
	allAnswered := true
	anyAnswered := false
	for _, q := range p.Questions {
		ans, a := resp.Answers[q.ID]
		invalid, i := resp.Invalid[q.ID]
		declined, d := resp.Declined[q.ID]
		count := 0
		if a {
			count++
		}
		if i {
			count++
		}
		if d {
			count++
		}
		if count != 1 {
			return r, fmt.Errorf("question %s has missing or overlapping outcomes", q.ID)
		}
		if i {
			if strings.TrimSpace(invalid) == "" {
				return r, errors.New("empty invalid reason")
			}
			r.Outcomes[q.ID] = "invalid"
			allAnswered = false
			continue
		}
		if d {
			if strings.TrimSpace(declined) == "" {
				return r, errors.New("empty refusal reason")
			}
			r.Outcomes[q.ID] = "declined"
			allAnswered = false
			continue
		}
		if err := checkAnswer(q, ans); err != nil {
			return r, err
		}
		r.ObservedAnswers[q.ID] = ObservedAnswer{Choice: ans.Choice, Confidence: decision.Confidence(ans.Probabilities), AnswerCacheHit: hits[q.ID]}
		if ans.Choice == "unknown" {
			r.Outcomes[q.ID] = "unknown"
			allAnswered = false
		} else {
			r.Outcomes[q.ID] = "answered"
			anyAnswered = true
		}
	}
	if allAnswered {
		r.Status = "usable"
		r.Reason = "ok"
	} else if anyAnswered {
		r.Reason = "partial"
	} else {
		r.Reason = "abstained"
	}
	return r, nil
}

func checkAnswer(q PackQuestion, a decision.Answer) error {
	if a.ID != q.ID || a.Kind != decision.KindChoice || a.Score != 0 || a.Noul != 0 {
		return errors.New("answer identity or kind mismatch")
	}
	if len(a.Probabilities) != len(q.Options) {
		return errors.New("incomplete probabilities")
	}
	validChoice := false
	for _, o := range q.Options {
		if a.Choice == o.Name {
			validChoice = true
		}
	}
	if !validChoice {
		return errors.New("unknown answer choice")
	}
	sum := 0.0
	max := -1.0
	for _, o := range q.Options {
		v, ok := a.Probabilities[o.Name]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return errors.New("invalid probability")
		}
		sum += v
		if v > max {
			max = v
		}
	}
	if math.Abs(sum-1) > 0.01 {
		return errors.New("probability sum outside tolerance")
	}
	if a.Probabilities[a.Choice] < max {
		return errors.New("choice is not maximum")
	}
	if math.IsNaN(a.Confidence) || math.IsInf(a.Confidence, 0) || a.Confidence < 0 || a.Confidence > 1 || math.Abs(a.Confidence-decision.Confidence(a.Probabilities)) > 1e-12 {
		return errors.New("confidence mismatch")
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

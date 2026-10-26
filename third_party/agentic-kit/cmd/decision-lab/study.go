package main

import (
	"errors"
	"sort"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

type ClassReport struct {
	Expected  int              `json:"expected"`
	Predicted int              `json:"predicted"`
	Confusion map[string]int   `json:"confusion"`
	Reading   decision.Reading `json:"reading"`
}

type StudyReport struct {
	Version                    string                 `json:"version"`
	Status                     string                 `json:"status"`
	Reason                     string                 `json:"reason"`
	DatasetKind                string                 `json:"datasetKind"`
	PackDigest                 string                 `json:"packDigest"`
	CohortDigest               string                 `json:"cohortDigest"`
	RawFileDigests             map[string]string      `json:"rawFileDigests"`
	ExpectedCases              int                    `json:"expectedCases"`
	CapturedCases              int                    `json:"capturedCases"`
	ExpectedAnswers            int                    `json:"expectedAnswers"`
	MissingCaseIDs             []string               `json:"missingCaseIds,omitempty"`
	DuplicateExposureCaseIDs   []string               `json:"duplicateExposureCaseIds,omitempty"`
	Answered                   int                    `json:"answered"`
	Unknown                    int                    `json:"unknown"`
	Declined                   int                    `json:"declined"`
	Invalid                    int                    `json:"invalid"`
	Failed                     int                    `json:"failed"`
	AnswerCacheHits            int                    `json:"answerCacheHits"`
	DeclaredAttempts           int                    `json:"declaredAttempts"`
	FailedAttempts             int                    `json:"failedAttempts"`
	UsageUnreportedCaptures    int                    `json:"usageUnreportedCaptures"`
	CauseCounts                map[string]int         `json:"causeCounts"`
	FalseFavorable             int                    `json:"falseFavorable"`
	FalseUnfavorable           int                    `json:"falseUnfavorable"`
	AbstainedExpectedFavorable int                    `json:"abstainedExpectedFavorable"`
	Classes                    map[string]ClassReport `json:"classes"`
	NoActionAuthority          bool                   `json:"noActionAuthority"`
}

func study(p Pack, m Manifest, captures []Capture) (StudyReport, error) {
	if err := p.validate(); err != nil {
		return StudyReport{}, err
	}
	pd := digest(canonical(p))
	cohort := digest(canonical(m))
	r := StudyReport{Version: "decision-lab.study/1", Status: "incomplete", Reason: "incomplete_coverage", DatasetKind: m.DatasetKind, PackDigest: pd, CohortDigest: cohort, ExpectedCases: len(m.Cases), CapturedCases: len(captures), ExpectedAnswers: len(m.Cases) * len(p.Questions), Classes: map[string]ClassReport{}, CauseCounts: map[string]int{"no-provider": 0, "timeout": 0, "not-authorized": 0, "budget-exhausted": 0, "transport-error": 0}, NoActionAuthority: true}
	if m.Version != "decision-lab.manifest/1" || m.PackDigest != pd || (m.DatasetKind != "synthetic" && m.DatasetKind != "observed") || len(m.Cases) < 1 || len(m.Cases) > 64 {
		return r, errors.New("invalid manifest identity or case count")
	}
	caseMap := map[string]ExpectedCase{}
	exposures := map[string]string{}
	units := map[string]bool{}
	optionMap := map[string]map[string]bool{}
	for _, q := range p.Questions {
		opts := map[string]bool{}
		for _, o := range q.Options {
			opts[o.Name] = true
		}
		optionMap[q.ID] = opts
		for _, o := range q.Options {
			r.Classes[q.ID+"/"+o.Name] = ClassReport{Confusion: map[string]int{}}
		}
	}
	for _, e := range m.Cases {
		if err := e.Input.validate(); err != nil {
			return r, err
		}
		if _, ok := caseMap[e.Input.CaseID]; ok {
			return r, errors.New("duplicate expected case")
		}
		if !validID(e.IndependentUnitID) || units[e.IndependentUnitID] {
			return r, errors.New("invalid or duplicate independent unit")
		}
		units[e.IndependentUnitID] = true
		if len(e.Labels) != len(p.Questions) {
			return r, errors.New("expected labels do not cover questions")
		}
		for _, q := range p.Questions {
			l, ok := e.Labels[q.ID]
			if !ok || !optionMap[q.ID][l.Choice] || !validProvenance(l.Provenance) {
				return r, errors.New("invalid expected label")
			}
		}
		exposureKey := decision.RequestKey(request(p, e.Input))
		if prior, ok := exposures[exposureKey]; ok {
			r.Reason = "duplicate_exposure"
			r.DuplicateExposureCaseIDs = []string{prior, e.Input.CaseID}
			sort.Strings(r.DuplicateExposureCaseIDs)
			return r, errors.New("duplicate exposure")
		}
		exposures[exposureKey] = e.Input.CaseID
		caseMap[e.Input.CaseID] = e
	}
	capturedIDs := map[string]bool{}
	for _, c := range captures {
		if _, ok := caseMap[c.CaseID]; !ok {
			r.Reason = "foreign_case"
			return r, errors.New("foreign captured case")
		}
		if capturedIDs[c.CaseID] {
			r.Reason = "duplicate_case"
			return r, errors.New("duplicate captured case")
		}
		capturedIDs[c.CaseID] = true
	}
	for id := range caseMap {
		if !capturedIDs[id] {
			r.MissingCaseIDs = append(r.MissingCaseIDs, id)
		}
	}
	sort.Strings(r.MissingCaseIDs)
	if len(r.MissingCaseIDs) > 0 {
		r.Reason = "missing_cases"
		return r, errors.New("missing captured cases")
	}
	if len(captures) != len(m.Cases) {
		r.Reason = "excess_captures"
		return r, errors.New("excess captured cases")
	}
	seen := map[string]bool{}
	samples := map[string][]decision.LabeledSample{}
	for _, c := range captures {
		e, ok := caseMap[c.CaseID]
		if !ok {
			return r, errors.New("foreign captured case")
		}
		if seen[c.CaseID] {
			return r, errors.New("duplicate captured case")
		}
		seen[c.CaseID] = true
		cr, err := check(p, e.Input, c)
		if err != nil {
			return r, errors.New("invalid capture")
		}
		r.AnswerCacheHits += len(c.AnswerCacheHits)
		r.DeclaredAttempts += c.AttemptCount
		if c.AttemptCount > 0 && (c.Response == nil || (c.Response.Usage.InputTokens == 0 && c.Response.Usage.OutputTokens == 0)) {
			r.UsageUnreportedCaptures++
		}
		if c.Cause != "" {
			r.CauseCounts[c.Cause]++
			r.FailedAttempts += c.AttemptCount
		}
		hits := map[string]bool{}
		for _, id := range c.AnswerCacheHits {
			hits[id] = true
		}
		for _, q := range p.Questions {
			actual := e.Labels[q.ID]
			ac := r.Classes[q.ID+"/"+actual.Choice]
			ac.Expected++
			r.Classes[q.ID+"/"+actual.Choice] = ac
			out := cr.Outcomes[q.ID]
			switch out {
			case "failed":
				r.Failed++
			case "declined":
				r.Declined++
			case "invalid":
				r.Invalid++
			case "unknown":
				r.Unknown++
			case "answered":
				r.Answered++
			}
			if out != "answered" && out != "unknown" {
				continue
			}
			predicted := c.Response.Answers[q.ID].Choice
			pc := r.Classes[q.ID+"/"+predicted]
			pc.Predicted++
			pc.Confusion[actual.Choice]++
			r.Classes[q.ID+"/"+predicted] = pc
			fav := map[string]bool{}
			for _, v := range q.FavorableOptions {
				fav[v] = true
			}
			if fav[predicted] && !fav[actual.Choice] {
				r.FalseFavorable++
			}
			if predicted == "unknown" && fav[actual.Choice] {
				r.AbstainedExpectedFavorable++
			}
			if predicted != "unknown" && !fav[predicted] && fav[actual.Choice] {
				r.FalseUnfavorable++
			}
			if !hits[q.ID] && predicted != "unknown" {
				ans := c.Response.Answers[q.ID]
				samples[q.ID+"/"+predicted] = append(samples[q.ID+"/"+predicted], decision.LabeledSample{Confidence: ans.Confidence, Correct: predicted == actual.Choice, Provenance: actual.Provenance})
			}
		}
	}
	for cls, v := range r.Classes {
		key := decision.ReadingKey{Site: p.ID + ":" + pd + ":" + cohort + ":" + cls, TemplateVersion: p.TemplateVersion + ":" + p.ConfidenceRecipe + ":" + p.ReadingRecipe, Provider: p.Provider, ResolvedModel: p.ResolvedModel}
		reading, err := decision.NewReading(key, samples[cls])
		if err != nil {
			return r, err
		}
		v.Reading = reading
		r.Classes[cls] = v
	}
	r.Status = "complete"
	r.Reason = "ok"
	return r, nil
}

func validProvenance(p decision.Provenance) bool {
	return p == decision.ProvenanceHuman || p == decision.ProvenanceComputed || p == decision.ProvenanceFrontier
}

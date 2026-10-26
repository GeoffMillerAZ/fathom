// Command decision-lab is an offline decision-site authoring aid without application effects.
// It has no shared mutable state and is safe for concurrent independent invocations.
package main

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"

type Pack struct {
	Version          string         `json:"version"`
	ID               string         `json:"id"`
	TemplateVersion  string         `json:"templateVersion"`
	ConfidenceRecipe string         `json:"confidenceRecipe"`
	ReadingRecipe    string         `json:"readingRecipe"`
	Provider         string         `json:"provider"`
	ResolvedModel    string         `json:"resolvedModel"`
	PolicyEpoch      string         `json:"policyEpoch"`
	Owner            string         `json:"owner"`
	Fallback         string         `json:"fallback"`
	Baseline         string         `json:"baseline"`
	KillCriterion    string         `json:"killCriterion"`
	Questions        []PackQuestion `json:"questions"`
}

type PackQuestion struct {
	ID               string       `json:"id"`
	Instructions     string       `json:"instructions"`
	Options          []PackOption `json:"options"`
	FavorableOptions []string     `json:"favorableOptions"`
}

type PackOption struct {
	Name     string `json:"name"`
	Criteria string `json:"criteria"`
}

type Input struct {
	Version       string `json:"version"`
	CaseID        string `json:"caseId"`
	SubjectID     string `json:"subjectId"`
	Revision      string `json:"revision"`
	CapturedState string `json:"capturedState"`
}

type Prepared struct {
	Version           string            `json:"version"`
	CaseID            string            `json:"caseId"`
	PackFileDigest    string            `json:"packFileDigest,omitempty"`
	InputFileDigest   string            `json:"inputFileDigest,omitempty"`
	PackDigest        string            `json:"packDigest"`
	InputDigest       string            `json:"inputDigest"`
	RequestKey        string            `json:"requestKey"`
	TemplateDigests   map[string]string `json:"templateDigests"`
	ConfidenceRecipe  string            `json:"confidenceRecipe"`
	NoActionAuthority bool              `json:"noActionAuthority"`
	Request           decision.Request  `json:"request"`
}

type Capture struct {
	Version         string        `json:"version"`
	CaseID          string        `json:"caseId"`
	PackDigest      string        `json:"packDigest"`
	InputDigest     string        `json:"inputDigest"`
	RequestKey      string        `json:"requestKey"`
	AttemptCount    int           `json:"attemptCount"`
	AnswerCacheHits []string      `json:"answerCacheHits"`
	Cause           string        `json:"cause,omitempty"`
	Response        *PortResponse `json:"response,omitempty"`
}

// PortResponse spells the decision port fields exactly as Go/CUE do. It is
// deliberately distinct from a provider's native wire representation.
type PortResponse struct {
	Provider string                     `json:"Provider"`
	Model    string                     `json:"Model"`
	Answers  map[string]decision.Answer `json:"Answers"`
	Invalid  map[string]string          `json:"Invalid"`
	Declined map[string]string          `json:"Declined,omitempty"`
	Usage    decision.Usage             `json:"Usage"`
	Latency  int64                      `json:"Latency"`
}

type Manifest struct {
	Version     string         `json:"version"`
	DatasetKind string         `json:"datasetKind"`
	PackDigest  string         `json:"packDigest"`
	Cases       []ExpectedCase `json:"cases"`
}

type ExpectedCase struct {
	Input             Input                    `json:"input"`
	IndependentUnitID string                   `json:"independentUnitId"`
	Labels            map[string]ExpectedLabel `json:"labels"`
}

type ExpectedLabel struct {
	Choice     string              `json:"choice"`
	Provenance decision.Provenance `json:"provenance"`
}

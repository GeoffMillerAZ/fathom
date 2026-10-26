// Command command-lab is an offline, advisory command governance reference.
// Independent invocations share no mutable state and are safe for concurrent use.
package main

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"

type SourceManifest struct {
	Version           string   `json:"version"`
	ExpectedSourceIDs []string `json:"expectedSourceIds"`
	Sources           []Source `json:"sources"`
}
type Source struct {
	ID     string `json:"id"`
	Format string `json:"format"`
	Path   string `json:"path"`
}
type Scope struct {
	Version          string   `json:"version"`
	RequiredLayerIDs []string `json:"requiredLayerIds"`
}
type Policy struct {
	Version    string      `json:"version"`
	Layers     []Layer     `json:"layers"`
	Classifier *Classifier `json:"classifier,omitempty"`
}
type Layer struct {
	ID                string   `json:"id"`
	Scope             string   `json:"scope"`
	AllowedOperations []string `json:"allowedOperations"`
	MaxBytes          int      `json:"maxBytes"`
	MaxResults        int      `json:"maxResults"`
}
type Classifier struct {
	Provider      string `json:"provider"`
	ResolvedModel string `json:"resolvedModel"`
}
type Operation struct {
	Version    string `json:"version"`
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Path       string `json:"path,omitempty"`
	Command    string `json:"command,omitempty"`
	MaxBytes   int    `json:"maxBytes"`
	MaxResults int    `json:"maxResults"`
}
type Capture struct {
	Version    string        `json:"version"`
	RequestKey string        `json:"requestKey"`
	Response   *PortResponse `json:"response,omitempty"`
	Cause      string        `json:"cause,omitempty"`
}
type PortResponse struct {
	Provider string                     `json:"Provider"`
	Model    string                     `json:"Model"`
	Answers  map[string]decision.Answer `json:"Answers"`
	Invalid  map[string]string          `json:"Invalid"`
	Declined map[string]string          `json:"Declined,omitempty"`
	Usage    decision.Usage             `json:"Usage"`
	Latency  int64                      `json:"Latency"`
}

type AuditReport struct {
	Version           string         `json:"version"`
	Status            string         `json:"status"`
	NoActionAuthority bool           `json:"noActionAuthority"`
	ExpectedSources   int            `json:"expectedSources"`
	LoadedSources     int            `json:"loadedSources"`
	Bytes             int64          `json:"bytes"`
	Records           int            `json:"records"`
	Proposals         map[string]int `json:"proposals"`
	Candidates        map[string]int `json:"candidates"`
	GitOperations     map[string]int `json:"gitOperations"`
	Failures          map[string]int `json:"failures"`
	Sources           []SourceReport `json:"sources"`
}
type SourceReport struct {
	Ordinal int    `json:"ordinal"`
	Bytes   int64  `json:"bytes"`
	Records int    `json:"records"`
	Status  string `json:"status"`
}
type AssessReport struct {
	Version             string   `json:"version"`
	Status              string   `json:"status"`
	NoActionAuthority   bool     `json:"noActionAuthority"`
	EffectiveAllowed    []string `json:"effectiveAllowed"`
	EffectiveMaxBytes   int      `json:"effectiveMaxBytes"`
	EffectiveMaxResults int      `json:"effectiveMaxResults"`
	Codes               []string `json:"codes"`
	ClassifierStatus    string   `json:"classifierStatus"`
	Choice              string   `json:"choice,omitempty"`
	Confidence          float64  `json:"confidence,omitempty"`
	RequestKey          string   `json:"requestKey,omitempty"`
}

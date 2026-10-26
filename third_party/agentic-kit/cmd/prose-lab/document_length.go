package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

const documentCountRule = "markdown-whitespace-fields/v1"
const documentRunnerProtocolV2 = "prose-lab.document-runner/3"

type documentLengthGoal struct {
	CriterionID string `json:"criterion_id"`
	Class       string `json:"class"`
	MinWords    int    `json:"min_words"`
	MaxWords    int    `json:"max_words"`
	Reason      string `json:"reason"`
}
type documentScopeExpectation struct {
	DocumentID              string   `json:"document_id"`
	RequiredSemanticGoalIDs []string `json:"required_semantic_goal_ids"`
	LengthCriterionID       string   `json:"length_criterion_id"`
	LengthClass             string   `json:"length_class"`
	MinWords                int      `json:"min_words"`
	MaxWords                int      `json:"max_words"`
	ReasonDigest            string   `json:"reason_digest"`
}
type documentLongWitness struct {
	DocumentID  string `json:"document_id"`
	CriterionID string `json:"criterion_id"`
	MinWords    int    `json:"min_words"`
}
type documentGoalScope struct {
	Version      string                     `json:"version"`
	ID           string                     `json:"id"`
	CorpusDigest string                     `json:"corpus_digest"`
	Documents    []documentScopeExpectation `json:"documents"`
	LongArtifact documentLongWitness        `json:"long_artifact"`
}

func loadDocumentGoalScope(path, pin string, c documentCorpus) (documentGoalScope, error) {
	var s documentGoalScope
	if !documentDigestPattern.MatchString(pin) {
		return s, errors.New("invalid goal scope pin")
	}
	b, err := readLabBytes(path, 1<<20)
	if err != nil || hashBytes(b) != pin {
		return s, errors.New("goal scope digest mismatch")
	}
	if err = exactObject(b, "version", "id", "corpus_digest", "documents", "long_artifact"); err != nil {
		return s, err
	}
	m, _ := jsonwire.Object(b)
	if err = exactObject(m["long_artifact"], "document_id", "criterion_id", "min_words"); err != nil {
		return s, err
	}
	var docs []json.RawMessage
	if err = json.Unmarshal(m["documents"], &docs); err != nil || len(docs) != 6 {
		return s, errors.New("goal scope requires six documents")
	}
	for _, raw := range docs {
		if err = exactObject(raw, "document_id", "required_semantic_goal_ids", "length_criterion_id", "length_class", "min_words", "max_words", "reason_digest"); err != nil {
			return s, err
		}
	}
	if err = decodeLabJSON(b, &s); err != nil {
		return s, err
	}
	if s.Version != "prose-lab.goal-scope/1" || !validLabID(s.ID) || s.CorpusDigest != c.Digest {
		return s, errors.New("goal scope identity mismatch")
	}
	seen := map[string]bool{}
	for _, x := range s.Documents {
		d, ok := c.Fixtures[x.DocumentID]
		if !ok || seen[x.DocumentID] || d.LengthGoal == nil {
			return s, errors.New("foreign or duplicate goal scope document")
		}
		seen[x.DocumentID] = true
		g := d.LengthGoal
		if x.LengthCriterionID != g.CriterionID || x.LengthClass != g.Class || x.MinWords != g.MinWords || x.MaxWords != g.MaxWords || x.ReasonDigest != hashBytes([]byte(g.Reason)) {
			return s, errors.New("goal scope length mismatch")
		}
		if len(x.RequiredSemanticGoalIDs) != len(d.RequiredGoals) {
			return s, errors.New("goal scope semantic roster mismatch")
		}
		ids := map[string]bool{}
		for _, id := range x.RequiredSemanticGoalIDs {
			if !validLabID(id) || ids[id] {
				return s, errors.New("duplicate semantic scope id")
			}
			ids[id] = true
		}
		for _, goal := range d.RequiredGoals {
			if !ids[goal.ID] {
				return s, errors.New("goal scope semantic mismatch")
			}
		}
	}
	w := s.LongArtifact
	d, ok := c.Fixtures[w.DocumentID]
	if !ok || !seen[w.DocumentID] || d.LengthGoal == nil || w.MinWords < 1200 || d.LengthBand != "long" || d.LengthGoal.Class != "required" || d.LengthGoal.CriterionID != w.CriterionID || d.LengthGoal.MinWords < w.MinWords {
		return s, errors.New("long artifact witness mismatch")
	}
	return s, nil
}

type documentGoalAssessment struct {
	Class        string                `json:"class"`
	Profile      fitness.Profile       `json:"profile"`
	Observations []fitness.Observation `json:"observations"`
	Result       fitness.Assessment    `json:"result"`
}
type documentGoalsSidecar struct {
	Version     string                   `json:"version"`
	CellID      string                   `json:"cell_id"`
	ScopeDigest string                   `json:"scope_digest"`
	CountRule   string                   `json:"count_rule"`
	WordCount   int                      `json:"word_count"`
	Goals       []documentGoalAssessment `json:"goals"`
}

func documentLengthAssess(d documentFixture, v prose.DocumentVersion, supplied prose.RenderedDocument, scopeDigest string, now time.Time) (documentGoalsSidecar, string, int, error) {
	var side documentGoalsSidecar
	now = now.UTC().Round(0)
	if d.LengthGoal == nil {
		return side, "", 0, errors.New("missing length goal")
	}
	canonical, err := prose.RenderMarkdown(v)
	if err != nil || canonical.Markdown != supplied.Markdown || canonical.MarkdownDigest != supplied.MarkdownDigest || canonical.DocumentDigest != supplied.DocumentDigest || !bytes.Equal(jsonBytes(canonical), jsonBytes(supplied)) {
		return side, "", 0, errors.New("rendered artifact substitution")
	}
	count := len(strings.Fields(canonical.Markdown))
	g := d.LengthGoal
	deviation := 0
	if count < g.MinWords {
		deviation = count - g.MinWords
	} else if count > g.MaxWords {
		deviation = count - g.MaxWords
	}
	subject := hashBytes(jsonBytes(struct{ VersionDigest, MarkdownDigest, CountRule, ScopeDigest string }{v.Receipt.DocumentDigest, canonical.MarkdownDigest, documentCountRule, scopeDigest}))
	side = documentGoalsSidecar{Version: "prose-lab.document-goals/1", CellID: v.Target.Session.ID, ScopeDigest: scopeDigest, CountRule: documentCountRule, WordCount: count, Goals: make([]documentGoalAssessment, 0, len(d.RequiredGoals)+1)}
	for _, semantic := range d.RequiredGoals {
		definition := hashBytes(jsonBytes(semantic))
		p := fitness.Profile{Version: fitness.ProfileVersion, ID: "prose-lab/" + semantic.ID, Domain: "prose-lab", Criteria: []fitness.Criterion{{ID: semantic.ID, DefinitionDigest: definition, Kind: fitness.CriterionRequired, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodHuman}}}}
		r, err := fitness.AssessProfile(p, subject, nil, now)
		if err != nil {
			return side, "", 0, err
		}
		side.Goals = append(side.Goals, documentGoalAssessment{Class: "required", Profile: p, Observations: []fitness.Observation{}, Result: r})
	}
	definition := hashBytes(jsonBytes(struct {
		Goal        documentLengthGoal `json:"goal"`
		CountRule   string             `json:"count_rule"`
		ScopeDigest string             `json:"scope_digest"`
	}{*g, documentCountRule, scopeDigest}))
	p := fitness.Profile{Version: fitness.ProfileVersion, ID: "prose-lab/" + g.CriterionID, Domain: "prose-lab", Criteria: []fitness.Criterion{{ID: g.CriterionID, DefinitionDigest: definition, Kind: fitness.CriterionRequired, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodDeterministic}}}}
	outcome := fitness.FindingPass
	if deviation != 0 {
		outcome = fitness.FindingFail
	}
	obs := fitness.Observation{CriterionID: g.CriterionID, DefinitionDigest: definition, SubjectDigest: subject, Outcome: outcome, Method: fitness.EvidenceMethodDeterministic, Reason: fmt.Sprintf("%d words against %d..%d; deviation %+d", count, g.MinWords, g.MaxWords, deviation), ArtifactDigests: []string{v.Receipt.DocumentDigest, canonical.MarkdownDigest, scopeDigest}, ObservedAt: now.UTC().Round(0), ValidUntil: now.UTC().Round(0).Add(time.Hour)}
	r, err := fitness.AssessProfile(p, subject, []fitness.Observation{obs}, now.UTC().Round(0))
	if err != nil {
		return side, "", 0, err
	}
	side.Goals = append(side.Goals, documentGoalAssessment{Class: g.Class, Profile: p, Observations: []fitness.Observation{obs}, Result: r})
	status := "pass"
	if deviation != 0 {
		if g.Class == "required" {
			status = "hold"
		} else {
			status = "warning"
		}
	}
	return side, status, deviation, nil
}

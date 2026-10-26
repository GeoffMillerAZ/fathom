package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

const pairedCandidateProtocol = "prose-lab.support-review/2"
const pairedStudyVersion = "prose-lab.paired-review-study/1"
const pairedScopePin = "f8d86a5366e2317f2a5dd6bc90fae074452a5d3c2176dccf9b1e589043ac6a49"
const pairedRegressionSourcePin = "7cd0aff9d9c75359859579637f86a581a51dea226b5c3b6b79e29355ee0f109c"

type pairedVariant struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
}
type pairedStudy struct {
	Version                   string          `json:"version"`
	ID                        string          `json:"id"`
	DatasetKind               string          `json:"dataset_kind"`
	BudgetMicroUSD            int64           `json:"budget_micro_usd"`
	PriorDirectMicroUSD       int64           `json:"prior_direct_micro_usd"`
	DirectStageBudgetMicroUSD int64           `json:"direct_stage_budget_micro_usd"`
	Review                    lane            `json:"review"`
	Variants                  []pairedVariant `json:"variants"`
	ExpectedExecutionIDs      []string        `json:"expected_execution_ids"`
}
type pairedProvenance struct {
	SourceDigest string `json:"source_digest"`
	Method       string `json:"method"`
	Reason       string `json:"reason"`
}
type pairedExtension struct {
	ID         string           `json:"id"`
	DocumentID string           `json:"document_id"`
	Kind       string           `json:"kind"`
	Role       string           `json:"role"`
	FactIDs    []string         `json:"fact_ids"`
	Assertion  string           `json:"assertion"`
	Expected   string           `json:"expected"`
	Provenance pairedProvenance `json:"provenance"`
}
type pairedScope struct {
	Version         string            `json:"version"`
	ID              string            `json:"id"`
	CorpusDigest    string            `json:"corpus_digest"`
	GoalScopeDigest string            `json:"goal_scope_digest"`
	BaseScopeDigest string            `json:"base_scope_digest"`
	Extensions      []pairedExtension `json:"extensions"`
}
type pairedRegressionSource struct {
	Version            string                 `json:"version"`
	Provenance         string                 `json:"provenance"`
	FixtureDigest      string                 `json:"fixture_digest"`
	Cases              []pairedRegressionCase `json:"cases"`
	HumanLabeling      bool                   `json:"human_labeling"`
	ChangesPilotStatus bool                   `json:"changes_pilot_status"`
	SitePromotion      bool                   `json:"site_promotion"`
}
type pairedRegressionCase struct {
	ID                    string         `json:"id"`
	Assertion             string         `json:"assertion"`
	FactIDs               []string       `json:"fact_ids"`
	Facts                 []documentFact `json:"facts"`
	ObservedSupportStatus string         `json:"observed_support_status"`
	ReferenceJudgment     string         `json:"reference_judgment"`
	Method                string         `json:"method"`
	Basis                 string         `json:"basis"`
}
type pairedInputs struct {
	study                                 pairedStudy
	scope                                 pairedScope
	corpus                                documentCorpus
	cases                                 map[string]reviewCase
	ordered                               []string
	rawStudy, boundStudy                  []byte
	studyDigest, goalDigest, reviewDigest string
}

func pairedExecutionID(variant, id string) string { return variant + "--" + id }
func pairedCaseID(executionID string) (variant, id string, ok bool) {
	for _, v := range []string{"baseline", "candidate"} {
		if strings.HasPrefix(executionID, v+"--") {
			return v, strings.TrimPrefix(executionID, v+"--"), true
		}
	}
	return "", "", false
}
func loadPairedInputs(studyPath, corpusPath, goalPath, goalPin, basePath, scopePath, scopePin, sourcePath string) (pairedInputs, error) {
	var in pairedInputs
	bad := func() (pairedInputs, error) { return in, errors.New("paired input refused") }
	raw, e := readLabBytes(studyPath, 1<<20)
	if e != nil || exactObject(raw, "version", "id", "dataset_kind", "budget_micro_usd", "prior_direct_micro_usd", "direct_stage_budget_micro_usd", "review", "variants", "expected_execution_ids") != nil {
		return bad()
	}
	obj, _ := jsonwire.Object(raw)
	laneFields, e := jsonwire.Object(obj["review"])
	if e != nil {
		return bad()
	}
	if resolved, ok := laneFields["resolved_model"]; ok && string(resolved) == "null" {
		return bad()
	}
	delete(laneFields, "resolved_model")
	laneRequired, _ := json.Marshal(laneFields)
	if exactObject(laneRequired, "provider", "model", "pricing_source", "max_input_tokens", "max_output_tokens", "input_rate", "output_rate", "cache_write_rate", "cache_read_rate", "reserve_input_rate", "reserve_output_rate", "tier_threshold", "high_input_rate", "high_output_rate", "high_cache_write_rate", "high_cache_read_rate") != nil {
		return bad()
	}
	if exactArrayObjects(obj["variants"], "id", "protocol") != nil || decodeLabJSON(raw, &in.study) != nil {
		return bad()
	}
	s := in.study
	if s.Version != pairedStudyVersion || !validLabID(s.ID) || s.DatasetKind != "synthetic" || s.BudgetMicroUSD < 1 || s.BudgetMicroUSD > 5_000_000 || s.PriorDirectMicroUSD < 0 || s.DirectStageBudgetMicroUSD < 1 || s.DirectStageBudgetMicroUSD > 10_000_000 || s.PriorDirectMicroUSD > s.DirectStageBudgetMicroUSD-s.BudgetMicroUSD || s.Review.validate() != nil || s.Review.Provider != "anthropic" || s.Review.MaxOutputTokens < 1 || s.Review.MaxOutputTokens > 8192 || s.Review.reserveCost() > s.BudgetMicroUSD || len(s.Variants) != 2 || s.Variants[0] != (pairedVariant{"baseline", reviewProtocol}) || s.Variants[1] != (pairedVariant{"candidate", pairedCandidateProtocol}) {
		return bad()
	}
	in.rawStudy = raw
	in.corpus, e = loadDocumentCorpus(corpusPath)
	if e != nil || in.corpus.Manifest.Version != 2 {
		return bad()
	}
	if !documentDigestPattern.MatchString(goalPin) {
		return bad()
	}
	if _, e = loadDocumentGoalScope(goalPath, goalPin, in.corpus); e != nil {
		return bad()
	}
	in.goalDigest = goalPin
	if scopePin != pairedScopePin {
		return bad()
	}
	b, e := readLabBytes(scopePath, 1<<20)
	if e != nil || hashBytes(b) != scopePin || exactObject(b, "version", "id", "corpus_digest", "goal_scope_digest", "base_scope_digest", "extensions") != nil {
		return bad()
	}
	o, _ := jsonwire.Object(b)
	if exactArrayObjects(o["extensions"], "id", "document_id", "kind", "role", "fact_ids", "assertion", "expected", "provenance") != nil {
		return bad()
	}
	var rawExt []json.RawMessage
	if json.Unmarshal(o["extensions"], &rawExt) != nil {
		return bad()
	}
	for _, x := range rawExt {
		xo, _ := jsonwire.Object(x)
		if exactObject(xo["provenance"], "source_digest", "method", "reason") != nil {
			return bad()
		}
	}
	if decodeLabJSON(b, &in.scope) != nil {
		return bad()
	}
	in.reviewDigest = scopePin
	if in.scope.Version != "prose-lab.review-scope/2" || !validLabID(in.scope.ID) || in.scope.CorpusDigest != in.corpus.Digest || in.scope.GoalScopeDigest != goalPin || !documentDigestPattern.MatchString(in.scope.BaseScopeDigest) || len(in.scope.Extensions) != 3 {
		return bad()
	}
	sourceRaw, e := readLabBytes(sourcePath, 1<<20)
	if e != nil || hashBytes(sourceRaw) != pairedRegressionSourcePin || exactObject(sourceRaw, "version", "provenance", "fixture_digest", "cases", "human_labeling", "changes_pilot_status", "site_promotion") != nil {
		return bad()
	}
	so, _ := jsonwire.Object(sourceRaw)
	if exactArrayObjects(so["cases"], "id", "assertion", "fact_ids", "facts", "observed_support_status", "reference_judgment", "method", "basis") != nil {
		return bad()
	}
	var source pairedRegressionSource
	if decodeLabJSON(sourceRaw, &source) != nil || source.Version != "prose-lab.observed-review-regressions/1" || len(source.Cases) != 2 || source.HumanLabeling || source.ChangesPilotStatus || source.SitePromotion || !documentDigestPattern.MatchString(source.FixtureDigest) || !validPairedText(source.Provenance, 512) {
		return bad()
	}
	fixtureRaw, e := readLabBytes(filepath.Join(corpusPath, "technical-design.json"), 8<<20)
	if e != nil || hashBytes(fixtureRaw) != source.FixtureDigest {
		return bad()
	}
	for _, sc := range source.Cases {
		if !validLabID(sc.ID) || !validPairedText(sc.Assertion, 4096) || !validPairedText(sc.Method, 128) || !validPairedText(sc.Basis, 1024) || (sc.ObservedSupportStatus != "unsupported" && sc.ObservedSupportStatus != "not_reached") || len(sc.FactIDs) == 0 || len(sc.Facts) == 0 {
			return bad()
		}
		for _, sf := range sc.Facts {
			found := false
			for _, cf := range in.corpus.Fixtures["technical-design"].Facts {
				if reflect.DeepEqual(sf, cf) {
					found = true
					break
				}
			}
			if !found {
				return bad()
			}
		}
	}
	_, in.cases, e = readReviewScope(basePath, in.scope.BaseScopeDigest, in.corpus, goalPin)
	if e != nil {
		return bad()
	}
	roles := map[string]int{}
	kinds := map[string]int{}
	expected := map[string]string{"rhetorical-framing": "supported", "document-scope-overclaim": "unsupported", "material-scope-exclusion": "unsupported"}
	for _, x := range in.scope.Extensions {
		d, ok := in.corpus.Fixtures[x.DocumentID]
		if !ok || !validLabID(x.ID) || !strings.HasPrefix(x.ID, x.DocumentID+"--") || in.cases[x.ID].ID != "" || expected[x.Role] != x.Expected || !validPairedText(x.Assertion, 4096) || len(x.FactIDs) < 1 || len(x.FactIDs) > 8 || !documentDigestPattern.MatchString(x.Provenance.SourceDigest) || !validPairedText(x.Provenance.Method, 128) || !validPairedText(x.Provenance.Reason, 1024) || (x.Kind != "observed-development" && x.Kind != "authored-contrast") {
			return bad()
		}
		if x.Kind == "observed-development" {
			matched := false
			for _, sc := range source.Cases {
				if x.ID == "technical-design--"+sc.ID && x.DocumentID == "technical-design" && x.Assertion == sc.Assertion && reflect.DeepEqual(x.FactIDs, sc.FactIDs) && x.Expected == sc.ReferenceJudgment && x.Provenance.SourceDigest == hashBytes(sourceRaw) && x.Provenance.Method == sc.Method && x.Provenance.Reason == sc.Basis {
					matched = true
					break
				}
			}
			if !matched {
				return bad()
			}
		} else if x.Provenance.SourceDigest != source.FixtureDigest || x.Provenance.Method != "fixture-author" {
			return bad()
		}
		facts := map[string]string{}
		for _, f := range d.Facts {
			facts[f.ID] = f.Text
		}
		cited := map[string]bool{}
		for _, id := range x.FactIDs {
			if !validLabID(id) || facts[id] == "" || cited[id] {
				return bad()
			}
			cited[id] = true
		}
		in.cases[x.ID] = reviewCase{ID: x.ID, DocumentID: x.DocumentID, Kind: x.Kind, Expected: x.Expected, Assertion: x.Assertion, FactIDs: x.FactIDs, Facts: facts}
		roles[x.Role]++
		kinds[x.Kind]++
	}
	if len(in.cases) != 70 || roles["rhetorical-framing"] != 1 || roles["document-scope-overclaim"] != 1 || roles["material-scope-exclusion"] != 1 || kinds["observed-development"] != 2 || kinds["authored-contrast"] != 1 {
		return bad()
	}
	ids := make([]string, 0, 70)
	for id := range in.cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		in.ordered = append(in.ordered, pairedExecutionID("baseline", id), pairedExecutionID("candidate", id))
	}
	if len(s.ExpectedExecutionIDs) != 140 {
		return bad()
	}
	for i, id := range in.ordered {
		if !validLabID(id) || s.ExpectedExecutionIDs[i] != id {
			return bad()
		}
	}
	// Both exact prompt byte strings are included through their real native request digests.
	wires := make([]string, 0, 140)
	for _, executionID := range in.ordered {
		v, id, _ := pairedCaseID(executionID)
		_, wire, err := pairedRequest(context.Background(), in.cases[id], s.Review, v)
		if err != nil {
			return bad()
		}
		wires = append(wires, hashBytes(wire))
	}
	in.boundStudy = jsonBytes(struct {
		Raw              string   `json:"raw"`
		Corpus           string   `json:"corpus"`
		Goal             string   `json:"goal"`
		Base             string   `json:"base"`
		Scope            string   `json:"scope"`
		RegressionSource string   `json:"regression_source"`
		ExecutionIDs     []string `json:"execution_ids"`
		NativeRequests   []string `json:"native_requests"`
	}{string(raw), in.corpus.Digest, goalPin, in.scope.BaseScopeDigest, scopePin, hashBytes(sourceRaw), in.ordered, wires})
	in.studyDigest = hashBytes(in.boundStudy)
	return in, nil
}
func validPairedText(s string, max int) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && len(s) <= max
}
func pairedReviewer(c reviewCase, l lane, variant string, client llmclient.LLMClient) *documentReviewer {
	r := reviewReviewer(c, l.Model, int(l.MaxOutputTokens), client)
	r.pairedCandidate = variant == "candidate"
	return r
}
func pairedRequest(ctx context.Context, c reviewCase, l lane, variant string) (llmclient.Request, []byte, error) {
	if variant == "baseline" {
		return reviewRequest(ctx, c, l)
	}
	if variant != "candidate" {
		return llmclient.Request{}, nil, errors.New("variant")
	}
	collector := &collectReviewRequest{}
	_, _ = pairedReviewer(c, l, variant, collector).Review(ctx, reviewClaim(c))
	if !collector.called {
		return llmclient.Request{}, nil, errors.New("request unavailable")
	}
	t := &reviewTransport{}
	a, e := anthropic.New(anthropic.Config{APIKey: "offline-review-dummy", HTTPClient: &http.Client{Transport: t}, DefaultModel: l.Model})
	if e != nil {
		return llmclient.Request{}, nil, e
	}
	_, _ = a.Complete(ctx, collector.request)
	if !t.called || len(t.expected) == 0 || int64(len(t.expected))+4096 > l.MaxInputTokens || len(t.expected) > 64<<10 {
		return llmclient.Request{}, nil, errors.New("native request unavailable")
	}
	return collector.request, t.expected, nil
}
func pairedReplay(ctx context.Context, c reviewCase, l lane, variant string, wire, response []byte) (prose.SupportDecision, error) {
	if variant == "baseline" {
		return reviewReplay(ctx, c, l, wire, response)
	}
	t := &reviewTransport{expected: wire, response: response}
	a, e := anthropic.New(anthropic.Config{APIKey: "offline-review-dummy", HTTPClient: &http.Client{Transport: t}, DefaultModel: l.Model})
	if e != nil {
		return prose.SupportDecision{}, e
	}
	d, e := pairedReviewer(c, l, variant, &documentTextClient{inner: a}).Review(ctx, reviewClaim(c))
	if e != nil || !t.called || t.mismatch {
		return d, errors.New("native replay refused")
	}
	return d, nil
}
func pairedPreparedDigest(in pairedInputs, executionID string, wire []byte) string {
	return hashBytes(jsonBytes(map[string]string{"study_digest": in.studyDigest, "execution_id": executionID, "native_request_digest": hashBytes(wire)}))
}

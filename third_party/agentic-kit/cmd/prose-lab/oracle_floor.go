package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const floorCriterionV2ID = "cited-entailment/2"
const floorCriterionV2Pin = "ecdecdb37fac788f2d95919c540c397d547c14f353d76d9df55ae4fac2ed4e2d"

//go:embed oracle-criteria/cited-entailment-v2.txt
var floorCriterionV2 []byte

func floorCriterionProfile(id, pin string) bool {
	if id == oracleCriterionID && pin == oracleCriterionPin {
		return true // oracleBuild verifies the embedded v1 bytes.
	}
	return id == floorCriterionV2ID && pin == floorCriterionV2Pin && hashBytes(floorCriterionV2) == floorCriterionV2Pin
}

// Floor inputs and output are local to each call. This command is safe for concurrent use.
type floorFact struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	OccurredAt string `json:"occurred_at"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required"`
}
type floorReview struct {
	ReviewerRef     string   `json:"reviewer_ref"`
	CriterionID     string   `json:"criterion_id"`
	CriterionSHA256 string   `json:"criterion_sha256"`
	CaseDigest      string   `json:"case_digest"`
	Status          string   `json:"status"`
	Expected        string   `json:"expected"`
	PremiseFactIDs  []string `json:"premise_fact_ids"`
	CounterFactIDs  []string `json:"counter_fact_ids"`
	Basis           string   `json:"basis"`
}
type floorCase struct {
	CaseID       string        `json:"case_id"`
	DocumentID   string        `json:"document_id"`
	Assertion    string        `json:"assertion"`
	CitedFactIDs []string      `json:"cited_fact_ids"`
	Facts        []floorFact   `json:"facts"`
	Expected     string        `json:"expected"`
	Role         string        `json:"role"`
	OldCaseIDs   []string      `json:"old_case_ids"`
	Basis        string        `json:"basis"`
	Reviews      []floorReview `json:"reviews"`
}
type floorCanonicalCase struct {
	CriterionID     string      `json:"criterion_id"`
	CriterionSHA256 string      `json:"criterion_sha256"`
	CaseID          string      `json:"case_id"`
	DocumentID      string      `json:"document_id"`
	Assertion       string      `json:"assertion"`
	CitedFactIDs    []string    `json:"cited_fact_ids"`
	Facts           []floorFact `json:"facts"`
	Expected        string      `json:"expected"`
	Role            string      `json:"role"`
	OldCaseIDs      []string    `json:"old_case_ids"`
}

func (c floorCase) digest(criterionID, criterionSHA256 string) string {
	return hashBytes(jsonBytes(floorCanonicalCase{criterionID, criterionSHA256, c.CaseID, c.DocumentID, c.Assertion, c.CitedFactIDs, c.Facts, c.Expected, c.Role, c.OldCaseIDs}))
}

type floorFixture struct {
	Version string      `json:"version"`
	ID      string      `json:"id"`
	Cases   []floorCase `json:"cases"`
}
type floorFixturePin struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}
type floorScope struct {
	Version         string            `json:"version"`
	ScopeID         string            `json:"scope_id"`
	CriterionID     string            `json:"criterion_id"`
	CriterionSHA256 string            `json:"criterion_sha256"`
	ExpectedCaseIDs []string          `json:"expected_case_ids"`
	Fixtures        []floorFixturePin `json:"fixtures"`
}
type floorReviewer struct {
	Ref                    string `json:"ref"`
	Kind                   string `json:"kind"`
	Role                   string `json:"role"`
	Exposure               string `json:"exposure"`
	AssessmentsDigest      string `json:"assessments_digest"`
	AssessmentReportDigest string `json:"assessment_report_digest"`
}
type floorDisposition struct {
	CaseID         string   `json:"case_id"`
	CaseDigest     string   `json:"case_digest"`
	Disposition    string   `json:"disposition"`
	ProspectiveIDs []string `json:"prospective_ids"`
	ChangedFields  []string `json:"changed_fields"`
	Basis          string   `json:"basis"`
	ReviewerRefs   []string `json:"reviewer_refs"`
}
type floorCaseRef struct {
	CaseID     string `json:"case_id"`
	CaseDigest string `json:"case_digest"`
}
type floorContrast struct {
	ID           string         `json:"id"`
	Cases        []floorCaseRef `json:"cases"`
	ReviewerRefs []string       `json:"reviewer_refs"`
}
type floorAmbiguity struct {
	CaseID string `json:"case_id"`
	Basis  string `json:"basis"`
}
type floorReceipt struct {
	Version                string             `json:"version"`
	PacketDigest           string             `json:"packet_digest"`
	CriterionID            string             `json:"criterion_id"`
	CriterionSHA256        string             `json:"criterion_sha256"`
	AssessmentsDigest      string             `json:"assessments_digest"`
	AssessmentReportDigest string             `json:"assessment_report_digest"`
	FutureScopeDigest      string             `json:"future_scope_digest"`
	Reviewers              []floorReviewer    `json:"reviewers"`
	OldCaseDispositions    []floorDisposition `json:"old_case_dispositions"`
	Contrasts              []floorContrast    `json:"contrasts"`
	RemainingAmbiguities   []floorAmbiguity   `json:"remaining_ambiguities"`
	Verdict                string             `json:"verdict"`
	ProviderQualified      bool               `json:"provider_qualified"`
	IdentityAuthenticated  bool               `json:"identity_authenticated"`
	PublicationAuthority   bool               `json:"publication_authority"`
	NewStudyAdmission      bool               `json:"new_study_admission"`
}
type floorCounts struct {
	OldCases             int `json:"old_cases"`
	ProspectiveCases     int `json:"prospective_cases"`
	Contrasts            int `json:"contrasts"`
	Reviewers            int `json:"reviewers"`
	RemainingAmbiguities int `json:"remaining_ambiguities"`
}
type floorContent struct {
	Scope    floorScope     `json:"scope"`
	Receipt  floorReceipt   `json:"receipt"`
	Fixtures []floorFixture `json:"fixtures"`
}
type floorReport struct {
	Version                string        `json:"version"`
	PacketDigest           string        `json:"packet_digest"`
	AssessmentsDigest      string        `json:"assessments_digest"`
	AssessmentReportDigest string        `json:"assessment_report_digest"`
	FutureScopeDigest      string        `json:"future_scope_digest"`
	FloorReviewDigest      string        `json:"floor_review_digest"`
	Counts                 floorCounts   `json:"counts"`
	DeclaredFloorReady     bool          `json:"declared_floor_ready"`
	ProviderQualified      bool          `json:"provider_qualified"`
	IdentityAuthenticated  bool          `json:"identity_authenticated"`
	PublicationAuthority   bool          `json:"publication_authority"`
	NewStudyAdmission      bool          `json:"new_study_admission"`
	Content                *floorContent `json:"content,omitempty"`
}

var floorBad = errors.New("invalid oracle floor receipt")

func floorObject(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	if exactObject(raw, keys...) != nil {
		return nil, floorBad
	}
	o, e := jsonwire.Object(raw)
	if e != nil {
		return nil, floorBad
	}
	return o, nil
}
func floorArray(raw json.RawMessage, keys ...string) ([]json.RawMessage, error) {
	if exactArrayObjects(raw, keys...) != nil {
		return nil, floorBad
	}
	var a []json.RawMessage
	if json.Unmarshal(raw, &a) != nil {
		return nil, floorBad
	}
	return a, nil
}
func floorIDs(ids []string, max int) bool {
	if ids == nil || len(ids) > max {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validLabID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func floorDigest(s string) bool { return documentDigestPattern.MatchString(s) }
func floorExpected(s string) bool {
	return s == "supported" || s == "unsupported" || s == "inconclusive"
}
func floorRead(path string, total *int) ([]byte, error) {
	b, e := readLabBytes(path, 1<<20)
	if e != nil {
		return nil, floorBad
	}
	*total += len(b)
	if *total > 8<<20 {
		return nil, floorBad
	}
	return b, nil
}
func floorParseScope(raw []byte) (floorScope, error) {
	var s floorScope
	o, e := floorObject(raw, "version", "scope_id", "criterion_id", "criterion_sha256", "expected_case_ids", "fixtures")
	if e != nil {
		return s, e
	}
	if _, e = floorArray(o["fixtures"], "id", "sha256"); e != nil {
		return s, e
	}
	if decodeLabJSON(raw, &s) != nil || s.Version != "prose-lab.oracle-floor-scope/1" || !validLabID(s.ScopeID) || !floorCriterionProfile(s.CriterionID, s.CriterionSHA256) || !floorIDs(s.ExpectedCaseIDs, 128) || len(s.ExpectedCaseIDs) == 0 || len(s.Fixtures) == 0 || len(s.Fixtures) > 32 {
		return s, floorBad
	}
	seen := map[string]bool{}
	for _, f := range s.Fixtures {
		if !validLabID(f.ID) || seen[f.ID] || !floorDigest(f.SHA256) {
			return s, floorBad
		}
		seen[f.ID] = true
	}
	return s, nil
}
func floorParseFixture(raw []byte, id string) (floorFixture, error) {
	var f floorFixture
	o, e := floorObject(raw, "version", "id", "cases")
	if e != nil {
		return f, e
	}
	items, e := floorArray(o["cases"], "case_id", "document_id", "assertion", "cited_fact_ids", "facts", "expected", "role", "old_case_ids", "basis", "reviews")
	if e != nil {
		return f, e
	}
	for _, item := range items {
		co, _ := jsonwire.Object(item)
		if _, e = floorArray(co["facts"], "id", "text", "occurred_at", "kind", "required"); e != nil {
			return f, e
		}
		if _, e = floorArray(co["reviews"], "reviewer_ref", "criterion_id", "criterion_sha256", "case_digest", "status", "expected", "premise_fact_ids", "counter_fact_ids", "basis"); e != nil {
			return f, e
		}
	}
	if decodeLabJSON(raw, &f) != nil || f.Version != "prose-lab.oracle-proposed-fixture/1" || f.ID != id || len(f.Cases) == 0 || len(f.Cases) > 128 {
		return f, floorBad
	}
	return f, nil
}
func floorValidateCase(c floorCase, scope floorScope, old map[string]oracleCase, reviewers map[string]floorReviewer) error {
	if !validLabID(c.CaseID) || !validLabID(c.DocumentID) || !validPairedText(c.Assertion, 4096) || !floorExpected(c.Expected) || !validPairedText(c.Basis, 4096) || !floorIDs(c.CitedFactIDs, 8) || len(c.CitedFactIDs) == 0 || !floorIDs(c.OldCaseIDs, 70) || len(c.Facts) == 0 || len(c.Facts) > 256 || len(c.Reviews) < 2 || len(c.Reviews) > 8 {
		return floorBad
	}
	if c.Role != "ordinary" && c.Role != "uncertainty-control" && c.Role != "modal-control" {
		return floorBad
	}
	if c.Role == "uncertainty-control" && c.Expected != "unsupported" {
		return floorBad
	}
	facts := map[string]bool{}
	cited := map[string]bool{}
	for _, x := range c.Facts {
		if !validLabID(x.ID) || facts[x.ID] || !validPairedText(x.Text, 4096) || !validPairedText(x.OccurredAt, 256) || (x.Kind != "observed" && x.Kind != "proposed" && x.Kind != "assumed" && x.Kind != "definition") {
			return floorBad
		}
		if at, e := parseDocumentOccurredAt(x.OccurredAt); e != nil || at.IsZero() {
			return floorBad
		}
		facts[x.ID] = true
	}
	for _, id := range c.CitedFactIDs {
		if !facts[id] {
			return floorBad
		}
		cited[id] = true
	}
	for _, id := range c.OldCaseIDs {
		if _, ok := old[id]; !ok {
			return floorBad
		}
	}
	seen := map[string]bool{}
	for _, r := range c.Reviews {
		if !validLabID(r.ReviewerRef) || seen[r.ReviewerRef] || !floorExpected(r.Expected) || r.CriterionID != scope.CriterionID || r.CriterionSHA256 != scope.CriterionSHA256 || r.CaseDigest != c.digest(scope.CriterionID, scope.CriterionSHA256) || (r.Status != "determinate" && r.Status != "unresolved") || !validPairedText(r.Basis, 4096) || !floorIDs(r.PremiseFactIDs, 8) || !floorIDs(r.CounterFactIDs, 256) || len(r.PremiseFactIDs)+len(r.CounterFactIDs) == 0 || (r.Expected == "supported" && len(r.PremiseFactIDs) == 0) {
			return floorBad
		}
		if _, ok := reviewers[r.ReviewerRef]; !ok {
			return floorBad
		}
		for _, id := range r.PremiseFactIDs {
			if !cited[id] {
				return floorBad
			}
		}
		for _, id := range r.CounterFactIDs {
			if !facts[id] {
				return floorBad
			}
		}
		seen[r.ReviewerRef] = true
	}
	return nil
}
func floorOldFields(o oracleCase, c floorCase, scope floorScope) map[string]bool {
	diff := map[string]bool{}
	if scope.CriterionID != oracleCriterionID || scope.CriterionSHA256 != oracleCriterionPin {
		diff["criterion"] = true
	}
	if o.DocumentID != c.DocumentID {
		diff["source"] = true
	}
	if o.Assertion != c.Assertion {
		diff["assertion"] = true
	}
	if o.HistoricalExpected != c.Expected {
		diff["label"] = true
	}
	if !reflect.DeepEqual(o.CitedFactIDs, c.CitedFactIDs) {
		diff["citation"] = true
	}
	oldFacts := make([]floorFact, 0, len(o.ContextFacts))
	for _, f := range o.ContextFacts {
		oldFacts = append(oldFacts, floorFact{f.ID, f.Text, f.OccurredAt, f.Kind, f.Required})
	}
	if !reflect.DeepEqual(oldFacts, c.Facts) {
		diff["source"] = true
	}
	return diff
}
func floorRefs(ids []string, reviewers map[string]floorReviewer) bool {
	if !floorIDs(ids, 8) || len(ids) < 2 {
		return false
	}
	for _, id := range ids {
		if _, ok := reviewers[id]; !ok {
			return false
		}
	}
	return true
}
func floorParseReceipt(raw []byte) (floorReceipt, error) {
	var r floorReceipt
	o, e := floorObject(raw, "version", "packet_digest", "criterion_id", "criterion_sha256", "assessments_digest", "assessment_report_digest", "future_scope_digest", "reviewers", "old_case_dispositions", "contrasts", "remaining_ambiguities", "verdict", "provider_qualified", "identity_authenticated", "publication_authority", "new_study_admission")
	if e != nil {
		return r, e
	}
	if _, e = floorArray(o["reviewers"], "ref", "kind", "role", "exposure", "assessments_digest", "assessment_report_digest"); e != nil {
		return r, e
	}
	disps, e := floorArray(o["old_case_dispositions"], "case_id", "case_digest", "disposition", "prospective_ids", "changed_fields", "basis", "reviewer_refs")
	if e != nil {
		return r, e
	}
	contrasts, e := floorArray(o["contrasts"], "id", "cases", "reviewer_refs")
	if e != nil {
		return r, e
	}
	for _, x := range contrasts {
		xo, _ := jsonwire.Object(x)
		if _, e = floorArray(xo["cases"], "case_id", "case_digest"); e != nil {
			return r, e
		}
	}
	if _, e = floorArray(o["remaining_ambiguities"], "case_id", "basis"); e != nil {
		return r, e
	}
	_ = disps
	if decodeLabJSON(raw, &r) != nil || r.Version != "prose-lab.oracle-floor-review/1" || r.CriterionID != oracleCriterionID || r.CriterionSHA256 != oracleCriterionPin || !floorDigest(r.PacketDigest) || !floorDigest(r.AssessmentsDigest) || !floorDigest(r.AssessmentReportDigest) || !floorDigest(r.FutureScopeDigest) || r.ProviderQualified || r.IdentityAuthenticated || r.PublicationAuthority || r.NewStudyAdmission || len(r.Reviewers) < 2 || len(r.Reviewers) > 8 || len(r.OldCaseDispositions) != 70 || len(r.Contrasts) != 7 || len(r.RemainingAmbiguities) > 128 || (r.Verdict != "held" && r.Verdict != "ready-for-new-study-design") {
		return r, floorBad
	}
	return r, nil
}

func floorValidate(packet oraclePacket, old map[string]oracleCase, scope floorScope, future map[string]floorCase, receipt floorReceipt) (bool, error) {
	if receipt.PacketDigest != packet.PacketDigest || receipt.FutureScopeDigest == "" || !floorCriterionProfile(scope.CriterionID, scope.CriterionSHA256) || len(scope.ExpectedCaseIDs) != len(future) || !floorIDs(scope.ExpectedCaseIDs, 128) {
		return false, floorBad
	}
	for _, id := range scope.ExpectedCaseIDs {
		if _, ok := future[id]; !ok {
			return false, floorBad
		}
	}
	reviewers := map[string]floorReviewer{}
	for _, r := range receipt.Reviewers {
		if !validLabID(r.Ref) || reviewers[r.Ref].Ref != "" || (r.Kind != "human" && r.Kind != "agent") || r.Role != "independent-source-reviewer" || !validPairedText(r.Exposure, 4096) || !floorDigest(r.AssessmentsDigest) || !floorDigest(r.AssessmentReportDigest) {
			return false, floorBad
		}
		reviewers[r.Ref] = r
	}
	agree := true
	for _, c := range future {
		if e := floorValidateCase(c, scope, old, reviewers); e != nil {
			return false, e
		}
		for _, r := range c.Reviews {
			if r.Status != "determinate" || r.Expected != c.Expected {
				agree = false
			}
		}
	}
	seenOld := map[string]bool{}
	for _, d := range receipt.OldCaseDispositions {
		o, ok := old[d.CaseID]
		if !ok || seenOld[d.CaseID] || d.CaseDigest != hashBytes(jsonBytes(o)) || !validPairedText(d.Basis, 4096) || !floorRefs(d.ReviewerRefs, reviewers) || !floorIDs(d.ProspectiveIDs, 4) || len(d.ProspectiveIDs) == 0 || !floorChanged(d.ChangedFields) {
			return false, floorBad
		}
		if d.Disposition != "retain" && d.Disposition != "uncertainty" && d.Disposition != "replace" {
			return false, floorBad
		}
		if d.Disposition == "retain" && (len(d.ProspectiveIDs) != 1 || len(d.ChangedFields) != 0) {
			return false, floorBad
		}
		if d.Disposition != "retain" && len(d.ChangedFields) == 0 {
			return false, floorBad
		}
		actual := map[string]bool{}
		unsupported := false
		uncertainty := false
		for _, id := range d.ProspectiveIDs {
			c, ok := future[id]
			if !ok || !containsFloor(c.OldCaseIDs, d.CaseID) {
				return false, floorBad
			}
			for _, reviewer := range d.ReviewerRefs {
				if !floorCaseReviewedBy(c, reviewer) {
					return false, floorBad
				}
			}
			if c.Expected == "unsupported" {
				unsupported = true
			}
			if c.Role == "uncertainty-control" {
				uncertainty = true
			}
			for field := range floorOldFields(o, c, scope) {
				actual[field] = true
			}
		}
		if d.Disposition == "retain" && len(actual) != 0 || d.Disposition == "uncertainty" && !uncertainty || o.HistoricalExpected == "unsupported" && !unsupported {
			return false, floorBad
		}
		if len(actual) != len(d.ChangedFields) {
			return false, floorBad
		}
		for field := range actual {
			if !containsFloor(d.ChangedFields, field) {
				return false, floorBad
			}
		}
		seenOld[d.CaseID] = true
	}
	if len(seenOld) != 70 {
		return false, floorBad
	}
	for id, c := range future {
		for _, oldID := range c.OldCaseIDs {
			found := false
			for _, d := range receipt.OldCaseDispositions {
				if d.CaseID == oldID && containsFloor(d.ProspectiveIDs, id) {
					found = true
					break
				}
			}
			if !found {
				return false, floorBad
			}
		}
	}
	seenContrast := map[string]bool{}
	for _, x := range receipt.Contrasts {
		if len(x.ID) != 6 || !strings.HasPrefix(x.ID, "OQ-C0") || x.ID[5] < '1' || x.ID[5] > '7' || seenContrast[x.ID] || !floorRefs(x.ReviewerRefs, reviewers) || len(x.Cases) < 2 || len(x.Cases) > 4 {
			return false, floorBad
		}
		seen := map[string]bool{}
		supported := false
		unsupported := false
		originalRunbook := false
		for _, ref := range x.Cases {
			c, ok := future[ref.CaseID]
			if !ok || seen[ref.CaseID] || ref.CaseDigest != c.digest(scope.CriterionID, scope.CriterionSHA256) || c.Role != "modal-control" && !(x.ID == "OQ-C07" && c.Role == "uncertainty-control") {
				return false, floorBad
			}
			seen[ref.CaseID] = true
			if containsFloor(c.OldCaseIDs, "operational-runbook--p-03") {
				originalRunbook = true
			}
			for _, reviewer := range x.ReviewerRefs {
				if !floorCaseReviewedBy(c, reviewer) {
					return false, floorBad
				}
			}
			if c.Expected == "supported" {
				supported = true
			} else if c.Expected == "unsupported" {
				unsupported = true
			}
		}
		if !supported || !unsupported || x.ID == "OQ-C07" && !originalRunbook {
			return false, floorBad
		}
		seenContrast[x.ID] = true
	}
	if len(seenContrast) != 7 {
		return false, floorBad
	}
	amb := map[string]bool{}
	for _, a := range receipt.RemainingAmbiguities {
		if !validLabID(a.CaseID) || amb[a.CaseID] || !validPairedText(a.Basis, 4096) {
			return false, floorBad
		}
		if _, ok := old[a.CaseID]; !ok {
			if _, ok = future[a.CaseID]; !ok {
				return false, floorBad
			}
		}
		amb[a.CaseID] = true
	}
	ready := len(amb) == 0 && agree
	if receipt.Verdict == "ready-for-new-study-design" && !ready {
		return false, floorBad
	}
	return receipt.Verdict == "ready-for-new-study-design", nil
}
func floorChanged(ids []string) bool {
	if !floorIDs(ids, 5) {
		return false
	}
	for _, x := range ids {
		if x != "label" && x != "citation" && x != "criterion" && x != "source" && x != "assertion" {
			return false
		}
	}
	return true
}
func containsFloor(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
func floorCaseReviewedBy(c floorCase, ref string) bool {
	for _, r := range c.Reviews {
		if r.ReviewerRef == ref {
			return true
		}
	}
	return false
}

// Under the unchanged criterion, an old positive with any recorded HP-19
// ambiguity must remain visible as the same semantic case's uncertainty control.
func floorCheckPriorAmbiguity(scope floorScope, old map[string]oracleCase, future map[string]floorCase, receipt floorReceipt, sources ...map[string]oracleAssessment) error {
	if scope.CriterionID != oracleCriterionID || scope.CriterionSHA256 != oracleCriterionPin {
		return nil
	}
	byOld := make(map[string]floorDisposition, len(receipt.OldCaseDispositions))
	for _, d := range receipt.OldCaseDispositions {
		byOld[d.CaseID] = d
	}
	for _, source := range sources {
		for id, assessment := range source {
			o, ok := old[id]
			if !ok || o.HistoricalExpected != "supported" || assessment.Disposition != "ambiguous" {
				continue
			}
			d, ok := byOld[id]
			if !ok || d.Disposition != "uncertainty" {
				return floorBad
			}
			matched := false
			for _, prospectiveID := range d.ProspectiveIDs {
				c, exists := future[prospectiveID]
				if !exists || c.Role != "uncertainty-control" || c.Expected != "unsupported" || !containsFloor(c.OldCaseIDs, id) {
					continue
				}
				diff := floorOldFields(o, c, scope)
				if len(diff) == 1 && diff["label"] {
					matched = true
					break
				}
			}
			if !matched {
				return floorBad
			}
		}
	}
	return nil
}

func runReviewFloor(ctx context.Context, args []string, out, errOut io.Writer) int {
	const cmd = "review-floor-report"
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		return pairedWrite(out, []byte("Usage: prose-lab "+cmd+" "+oracleCommonUsage+" --packet-digest SHA256 --assessments PATH --floor-review PATH --future-scope PATH --future-scope-digest SHA256 --fixtures DIR --review-dir DIR [--include-content]\n"), 0)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	study := fs.String("study", "", "")
	corpus := fs.String("corpus", "", "")
	goal := fs.String("goal-scope", "", "")
	goalPin := fs.String("goal-scope-digest", "", "")
	base := fs.String("base-review-scope", "", "")
	scope := fs.String("review-scope", "", "")
	scopePin := fs.String("review-scope-digest", "", "")
	source := fs.String("regression-source", "", "")
	packetPin := fs.String("packet-digest", "", "")
	assessPath := fs.String("assessments", "", "")
	reviewPath := fs.String("floor-review", "", "")
	futurePath := fs.String("future-scope", "", "")
	futurePin := fs.String("future-scope-digest", "", "")
	fixturesDir := fs.String("fixtures", "", "")
	reviewDir := fs.String("review-dir", "", "")
	include := fs.Bool("include-content", false, "")
	if fs.Parse(args) != nil || fs.NArg() != 0 {
		return diagnostic(errOut, "invalid_floor_flags", 2)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, key := range []string{"study", "corpus", "goal-scope", "goal-scope-digest", "base-review-scope", "review-scope", "review-scope-digest", "regression-source", "packet-digest", "assessments", "floor-review", "future-scope", "future-scope-digest", "fixtures", "review-dir"} {
		if !set[key] {
			return diagnostic(errOut, "floor_input_required", 2)
		}
	}
	if !floorDigest(*packetPin) || !floorDigest(*futurePin) || *assessPath == "" || *reviewPath == "" || *futurePath == "" || *fixturesDir == "" || *reviewDir == "" {
		return diagnostic(errOut, "invalid_floor_flags", 2)
	}
	if ctx.Err() != nil {
		return diagnostic(errOut, "floor_cancelled", 2)
	}
	in, e := loadPairedInputs(*study, *corpus, *goal, *goalPin, *base, *scope, *scopePin, *source)
	if e != nil {
		return diagnostic(errOut, "floor_inputs_refused", 2)
	}
	packet, old, e := oracleBuild(in, *corpus, *base, *scope, *source, false)
	if e != nil || packet.PacketDigest != *packetPin {
		return diagnostic(errOut, "floor_packet_refused", 2)
	}
	total := 0
	assessRaw, e := floorRead(*assessPath, &total)
	if e != nil {
		return diagnostic(errOut, "floor_assessments_refused", 2)
	}
	assess, e := oracleReadAssessments(assessRaw, packet, old)
	if e != nil || len(assess) != 70 {
		return diagnostic(errOut, "floor_assessments_refused", 2)
	}
	assessDigest := hashBytes(assessRaw)
	reportDigest := hashBytes(jsonBytes(oracleMakeReport(packet, assessRaw, assess)))
	scopeRaw, e := floorRead(*futurePath, &total)
	if e != nil || hashBytes(scopeRaw) != *futurePin {
		return diagnostic(errOut, "floor_scope_refused", 2)
	}
	futureScope, e := floorParseScope(scopeRaw)
	if e != nil {
		return diagnostic(errOut, "floor_scope_refused", 2)
	}
	receiptRaw, e := floorRead(*reviewPath, &total)
	if e != nil {
		return diagnostic(errOut, "floor_review_refused", 2)
	}
	receipt, e := floorParseReceipt(receiptRaw)
	if e != nil || receipt.AssessmentsDigest != assessDigest || receipt.AssessmentReportDigest != reportDigest || receipt.FutureScopeDigest != *futurePin {
		return diagnostic(errOut, "floor_review_refused", 2)
	}
	reviewers := map[string]floorReviewer{}
	priorSources := []map[string]oracleAssessment{assess}
	for _, r := range receipt.Reviewers {
		if !validLabID(r.Ref) || reviewers[r.Ref].Ref != "" {
			return diagnostic(errOut, "floor_review_refused", 2)
		}
		reviewers[r.Ref] = r
	}
	for _, r := range receipt.Reviewers {
		b, e := floorRead(filepath.Join(*reviewDir, r.Ref+".json"), &total)
		if e != nil || hashBytes(b) != r.AssessmentsDigest {
			return diagnostic(errOut, "floor_reviewer_refused", 2)
		}
		a, e := oracleReadAssessments(b, packet, old)
		if e != nil || len(a) != 70 || hashBytes(jsonBytes(oracleMakeReport(packet, b, a))) != r.AssessmentReportDigest {
			return diagnostic(errOut, "floor_reviewer_refused", 2)
		}
		for _, row := range a {
			if row.AssessorRef != r.Ref || row.AssessorKind != r.Kind {
				return diagnostic(errOut, "floor_reviewer_refused", 2)
			}
		}
		priorSources = append(priorSources, a)
	}
	future := map[string]floorCase{}
	fixtures := make([]floorFixture, 0, len(futureScope.Fixtures))
	for _, pin := range futureScope.Fixtures {
		b, e := floorRead(filepath.Join(*fixturesDir, pin.ID+".json"), &total)
		if e != nil || hashBytes(b) != pin.SHA256 {
			return diagnostic(errOut, "floor_fixture_refused", 2)
		}
		f, e := floorParseFixture(b, pin.ID)
		if e != nil {
			return diagnostic(errOut, "floor_fixture_refused", 2)
		}
		for _, c := range f.Cases {
			if _, ok := future[c.CaseID]; ok {
				return diagnostic(errOut, "floor_fixture_refused", 2)
			}
			future[c.CaseID] = c
		}
		fixtures = append(fixtures, f)
	}
	if len(future) != len(futureScope.ExpectedCaseIDs) {
		return diagnostic(errOut, "floor_coverage_refused", 2)
	}
	ids := append([]string(nil), futureScope.ExpectedCaseIDs...)
	sort.Strings(ids)
	for _, id := range ids {
		if _, ok := future[id]; !ok {
			return diagnostic(errOut, "floor_coverage_refused", 2)
		}
	}
	if ctx.Err() != nil {
		return diagnostic(errOut, "floor_cancelled", 2)
	}
	if floorCheckPriorAmbiguity(futureScope, old, future, receipt, priorSources...) != nil {
		return diagnostic(errOut, "floor_review_refused", 2)
	}
	ready, e := floorValidate(packet, old, futureScope, future, receipt)
	if e != nil {
		return diagnostic(errOut, "floor_review_refused", 2)
	}
	r := floorReport{Version: "prose-lab.oracle-floor-report/1", PacketDigest: packet.PacketDigest, AssessmentsDigest: assessDigest, AssessmentReportDigest: reportDigest, FutureScopeDigest: *futurePin, FloorReviewDigest: hashBytes(receiptRaw), Counts: floorCounts{70, len(future), 7, len(receipt.Reviewers), len(receipt.RemainingAmbiguities)}, DeclaredFloorReady: ready}
	if *include {
		r.Content = &floorContent{futureScope, receipt, fixtures}
	}
	code := 1
	if ready {
		code = 0
	}
	return oracleWrite(ctx, out, r, code)
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

const reviewProtocol = "prose-lab.support-review/1"

type reviewControl struct {
	ID         string   `json:"id"`
	DocumentID string   `json:"document_id"`
	Kind       string   `json:"kind"`
	FactIDs    []string `json:"fact_ids"`
	Assertion  string   `json:"assertion"`
}
type reviewScope struct {
	Version          string          `json:"version"`
	ID               string          `json:"id"`
	CorpusDigest     string          `json:"corpus_digest"`
	GoalScopeDigest  string          `json:"goal_scope_digest"`
	NegativeCaseIDs  []string        `json:"negative_case_ids"`
	PositiveControls []reviewControl `json:"positive_controls"`
}
type reviewCase struct {
	ID, DocumentID, Kind, Expected, Assertion string
	FactIDs                                   []string
	Facts                                     map[string]string
}
type reviewInputs struct {
	study                                         canaryStudy
	rawStudy, boundStudy                          []byte
	corpus                                        documentCorpus
	scope                                         reviewScope
	goalDigest, reviewDigest, studyDigest, laneID string
	selected                                      lane
	cases                                         map[string]reviewCase
}

type collectReviewRequest struct {
	request llmclient.Request
	called  bool
}

func (c *collectReviewRequest) Complete(_ context.Context, r llmclient.Request) (llmclient.Response, error) {
	c.request = r
	c.called = true
	return llmclient.Response{}, errors.New("request collected")
}
func (*collectReviewRequest) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("stream refused")
}

type reviewTransport struct {
	expected []byte
	response []byte
	called   bool
	mismatch bool
}

func (t *reviewTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.called || req.Method != "POST" || req.URL.String() != "https://api.anthropic.com/v1/messages" {
		return nil, errors.New("offline review transport refused")
	}
	t.called = true
	b, e := io.ReadAll(io.LimitReader(req.Body, 64<<10+1))
	if e != nil || len(b) > 64<<10 {
		return nil, errors.New("review wire bound")
	}
	if t.expected != nil && !bytes.Equal(b, t.expected) {
		t.mismatch = true
		return nil, errors.New("review wire mismatch")
	}
	t.expected = append([]byte(nil), b...)
	if t.response == nil {
		return nil, errors.New("review wire collected")
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(t.response)), Request: req}, nil
}
func reviewReviewer(c reviewCase, model string, max int, client llmclient.LLMClient) *documentReviewer {
	return &documentReviewer{client: client, model: model, facts: c.Facts, reviewer: "configured-model-heuristic", maxTokens: max}
}
func reviewClaim(c reviewCase) prose.SupportReview {
	return prose.SupportReview{Claim: &prose.ProseClaim{ID: c.ID, Assertion: c.Assertion, FactIDs: append([]string(nil), c.FactIDs...)}}
}
func reviewRequest(ctx context.Context, c reviewCase, l lane) (llmclient.Request, []byte, error) {
	collector := &collectReviewRequest{}
	_, _ = reviewReviewer(c, l.Model, int(l.MaxOutputTokens), collector).Review(ctx, reviewClaim(c))
	if !collector.called {
		return llmclient.Request{}, nil, errors.New("review request unavailable")
	}
	t := &reviewTransport{}
	a, e := anthropic.New(anthropic.Config{APIKey: "offline-review-dummy", HTTPClient: &http.Client{Transport: t}, DefaultModel: l.Model})
	if e != nil {
		return llmclient.Request{}, nil, e
	}
	_, _ = a.Complete(ctx, collector.request)
	if !t.called || len(t.expected) == 0 {
		return llmclient.Request{}, nil, errors.New("native request unavailable")
	}
	if int64(len(t.expected))+4096 > l.MaxInputTokens || len(t.expected) > 64<<10 {
		return llmclient.Request{}, nil, errors.New("review request exceeds reservation")
	}
	return collector.request, t.expected, nil
}
func reviewReplay(ctx context.Context, c reviewCase, l lane, wire, response []byte) (prose.SupportDecision, error) {
	t := &reviewTransport{expected: wire, response: response}
	a, e := anthropic.New(anthropic.Config{APIKey: "offline-review-dummy", HTTPClient: &http.Client{Transport: t}, DefaultModel: l.Model})
	if e != nil {
		return prose.SupportDecision{}, e
	}
	d, e := reviewReviewer(c, l.Model, int(l.MaxOutputTokens), &documentTextClient{inner: a}).Review(ctx, reviewClaim(c))
	if e != nil {
		return d, e
	}
	if !t.called || t.mismatch {
		return d, errors.New("native request not replayed")
	}
	return d, nil
}
func readReviewScope(path, pin string, c documentCorpus, goalDigest string) (reviewScope, map[string]reviewCase, error) {
	var s reviewScope
	cases := map[string]reviewCase{}
	if !documentDigestPattern.MatchString(pin) {
		return s, nil, errors.New("review pin required")
	}
	b, e := readLabBytes(path, 1<<20)
	if e != nil || hashBytes(b) != pin {
		return s, nil, errors.New("review scope pin mismatch")
	}
	if e = exactObject(b, "version", "id", "corpus_digest", "goal_scope_digest", "negative_case_ids", "positive_controls"); e != nil {
		return s, nil, e
	}
	obj, _ := jsonwire.Object(b)
	if e = exactArrayObjects(obj["positive_controls"], "id", "document_id", "kind", "fact_ids", "assertion"); e != nil {
		return s, nil, e
	}
	if e = decodeLabJSON(b, &s); e != nil {
		return s, nil, e
	}
	if s.Version != "prose-lab.review-scope/1" || !validLabID(s.ID) || s.CorpusDigest != c.Digest || s.GoalScopeDigest != goalDigest || len(s.NegativeCaseIDs) != 43 || len(s.PositiveControls) != 24 {
		return s, nil, errors.New("review scope shape or binding")
	}
	negatives := map[string]reviewCase{}
	for id, d := range c.Fixtures {
		facts := map[string]string{}
		for _, f := range d.Facts {
			facts[f.ID] = f.Text
		}
		for _, n := range d.NegativeCases {
			caseID := id + "--" + n.ID
			negatives[caseID] = reviewCase{ID: caseID, DocumentID: id, Kind: "negative", Expected: "unsupported", Assertion: n.MutatedAssertion, FactIDs: []string{n.TargetFactID}, Facts: facts}
		}
	}
	if len(negatives) != 43 {
		return s, nil, errors.New("fixture negative roster changed")
	}
	for _, id := range s.NegativeCaseIDs {
		v, ok := negatives[id]
		if !ok || cases[id].ID != "" {
			return s, nil, errors.New("duplicate or foreign negative")
		}
		cases[id] = v
	}
	if len(cases) != len(negatives) {
		return s, nil, errors.New("negative roster omitted")
	}
	counts := map[string]map[string]int{}
	for _, p := range s.PositiveControls {
		d, ok := c.Fixtures[p.DocumentID]
		if !ok || !validLabID(p.ID) || !strings.HasPrefix(p.ID, p.DocumentID+"--p-") || cases[p.ID].ID != "" || strings.TrimSpace(p.Assertion) == "" || len(p.Assertion) > 4096 || len(p.FactIDs) < 1 || len(p.FactIDs) > 8 {
			return s, nil, errors.New("invalid positive control")
		}
		if p.Kind != "exact" && p.Kind != "paraphrase" && p.Kind != "synthesis" {
			return s, nil, errors.New("invalid positive kind")
		}
		facts := map[string]string{}
		for _, f := range d.Facts {
			facts[f.ID] = f.Text
		}
		cited := map[string]bool{}
		for _, id := range p.FactIDs {
			if facts[id] == "" || cited[id] {
				return s, nil, errors.New("invalid cited fact")
			}
			cited[id] = true
		}
		if p.Kind == "exact" && (len(p.FactIDs) != 1 || p.Assertion != facts[p.FactIDs[0]]) {
			return s, nil, errors.New("exact control mismatch")
		}
		if p.Kind == "synthesis" && len(p.FactIDs) < 2 {
			return s, nil, errors.New("synthesis requires multiple facts")
		}
		if counts[p.DocumentID] == nil {
			counts[p.DocumentID] = map[string]int{}
		}
		counts[p.DocumentID][p.Kind]++
		cases[p.ID] = reviewCase{ID: p.ID, DocumentID: p.DocumentID, Kind: p.Kind, Expected: "supported", Assertion: p.Assertion, FactIDs: p.FactIDs, Facts: facts}
	}
	for id := range c.Fixtures {
		m := counts[id]
		if m["exact"] != 2 || m["paraphrase"] != 1 || m["synthesis"] != 1 {
			return s, nil, errors.New("positive roster mismatch")
		}
	}
	if len(cases) != 67 {
		return s, nil, errors.New("review roster must be 67")
	}
	return s, cases, nil
}
func loadReviewInputs(studyPath, corpusPath, goalPath, goalPin, reviewPath, reviewPin, laneID string) (reviewInputs, error) {
	var in reviewInputs
	in.goalDigest = goalPin
	in.reviewDigest = reviewPin
	in.laneID = laneID
	raw, e := readStudyJSON(studyPath, &in.study)
	if e != nil || in.study.validate() != nil {
		return in, errors.New("invalid study")
	}
	in.rawStudy = raw
	in.corpus, e = loadDocumentCorpus(corpusPath)
	if e != nil || in.corpus.Manifest.Version != 2 {
		return in, errors.New("invalid v2 corpus")
	}
	if !documentDigestPattern.MatchString(goalPin) {
		return in, errors.New("goal pin required")
	}
	if _, e = loadDocumentGoalScope(goalPath, goalPin, in.corpus); e != nil {
		return in, errors.New("goal scope invalid")
	}
	in.scope, in.cases, e = readReviewScope(reviewPath, reviewPin, in.corpus, goalPin)
	if e != nil {
		return in, e
	}
	l, ok := in.study.Lanes[laneID]
	if !ok || len(in.study.Lanes) != 1 || l.Provider != "anthropic" || l.MaxOutputTokens < 1 || l.MaxOutputTokens > 8192 {
		return in, errors.New("one selected Anthropic review lane required")
	}
	in.selected = l
	idmap := map[string]string{"protocol": reviewProtocol, "corpus_digest": in.corpus.Digest, "goal_scope_digest": goalPin, "review_scope_digest": reviewPin, "lane_id": laneID}
	requiredID := "review-" + hashBytes(jsonBytes(idmap))
	if in.study.ID != requiredID || len(in.study.ExpectedCaseIDs) != 67 {
		return in, errors.New("study ID or roster mismatch")
	}
	seen := map[string]bool{}
	for _, id := range in.study.ExpectedCaseIDs {
		if _, ok := in.cases[id]; !ok || seen[id] {
			return in, errors.New("study roster mismatch")
		}
		seen[id] = true
	}
	in.boundStudy = append(append([]byte{}, raw...), []byte("\nprotocol:"+reviewProtocol+"\ncorpus-sha256:"+in.corpus.Digest+"\ngoal-scope-sha256:"+goalPin+"\nreview-scope-sha256:"+reviewPin+"\nlane:"+laneID+"\n")...)
	in.studyDigest = hashBytes(in.boundStudy)
	return in, nil
}
func reviewPreparedDigest(in reviewInputs, id string, wire []byte) string {
	return hashBytes(jsonBytes(map[string]string{"protocol": reviewProtocol, "study_digest": in.studyDigest, "case_id": id, "lane_id": in.laneID, "native_request_digest": hashBytes(wire)}))
}

// A live cohort cannot advance past a mismatch, incomplete result, or replay.
// This replays the native private history while the writer owns the lock.
func reviewPriorAttempts(ctx context.Context, j *journal, artifacts *os.Root, in reviewInputs, selectedCase string) error {
	reader := &journal{root: j.root, header: j.header, next: 1}
	attempts, err := reader.readHistory(false)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var cumulative int64
	for _, a := range attempts {
		r, o := a.Reservation, a.Outcome
		c, ok := in.cases[r.CaseID]
		if !ok || seen[r.CaseID] || r.CaseID == selectedCase || r.Provider != "anthropic" || r.Model != in.selected.Model || r.Pricing != in.selected || o == nil || o.Status != "completed" || o.HTTPStatus < 200 || o.HTTPStatus >= 300 {
			return errors.New("prior cohort attempt requires review")
		}
		seen[r.CaseID] = true
		_, wire, err := reviewRequest(ctx, c, in.selected)
		if err != nil || r.RequestHash != hashBytes(wire) || r.RequestJSON != string(wire) {
			return errors.New("prior native request mismatch")
		}
		d, err := reviewReplay(ctx, c, in.selected, wire, []byte(o.Response))
		if err != nil || d.Status != c.Expected {
			return errors.New("prior semantic mismatch")
		}
		caseCharge := r.MicroUSD
		if o.ActualMicroUSD != nil {
			caseCharge = *o.ActualMicroUSD
		}
		cumulative += caseCharge
		if err := reviewArtifactValidate(artifacts, in, c, r, d, caseCharge, cumulative); err != nil {
			return err
		}
	}
	return nil
}

type reviewReceipt struct {
	Version                   string `json:"version"`
	CaseID                    string `json:"case_id"`
	StudyDigest               string `json:"study_digest"`
	PreparedDigest            string `json:"prepared_digest"`
	CorpusDigest              string `json:"corpus_digest"`
	GoalScopeDigest           string `json:"goal_scope_digest"`
	ReviewScopeDigest         string `json:"review_scope_digest"`
	ReviewLane                string `json:"review_lane"`
	RequestedReviewModel      string `json:"requested_review_model"`
	NativeRequestDigest       string `json:"native_request_digest"`
	Status                    string `json:"status"`
	Expected                  string `json:"expected"`
	Matched                   bool   `json:"matched_fixture_expectation"`
	ChargeBasis               string `json:"charge_basis"`
	CumulativeChargedMicroUSD *int64 `json:"cumulative_charged_micro_usd,omitempty"`
	CaseChargedMicroUSD       *int64 `json:"case_charged_micro_usd,omitempty"`
	IndependentAcceptance     bool   `json:"independent_acceptance"`
	PublicationAuthority      bool   `json:"publication_authority"`
	ErrorClass                string `json:"error_class,omitempty"`
}
type reviewCaseReport struct {
	CaseID   string `json:"case_id"`
	Expected string `json:"expected"`
	Status   string `json:"status"`
	Matched  bool   `json:"matched"`
}
type reviewReport struct {
	Version                    string             `json:"version"`
	StudyDigest                string             `json:"study_digest"`
	CorpusDigest               string             `json:"corpus_digest"`
	GoalScopeDigest            string             `json:"goal_scope_digest"`
	ReviewScopeDigest          string             `json:"review_scope_digest"`
	ReviewLane                 string             `json:"review_lane"`
	Planned                    int                `json:"planned"`
	Completed                  int                `json:"completed"`
	Missing                    int                `json:"missing"`
	FalseSupport               int                `json:"false_support"`
	FalseHold                  int                `json:"false_hold"`
	Inconclusive               int                `json:"inconclusive"`
	Pending                    int                `json:"pending"`
	Failed                     int                `json:"failed"`
	Unknown                    int                `json:"unknown"`
	InvalidCount               int                `json:"invalid_count"`
	TotalChargedMicroUSD       int64              `json:"total_charged_micro_usd"`
	ChargeBasis                string             `json:"charge_basis"`
	MatchedFixtureExpectations bool               `json:"matched_fixture_expectations"`
	Cases                      []reviewCaseReport `json:"cases"`
	IndependentAcceptance      bool               `json:"independent_acceptance"`
	PublicationAuthority       bool               `json:"publication_authority"`
}

// reviewDeps is a command-local test seam. Nil fields retain the live path.
type reviewDeps struct {
	now         func() time.Time
	captureHTTP func() (*http.Client, *captureTransport)
	apiKey      string
}

func runReview(ctx context.Context, args []string, out, errOut io.Writer) int {
	return runReviewWith(ctx, args, out, errOut, reviewDeps{})
}

func runReviewWith(ctx context.Context, args []string, out, errOut io.Writer, deps reviewDeps) int {
	cmd := args[0]
	usage := func() int {
		fmt.Fprintln(errOut, "Usage: prose-lab review-prepare|review-capture|review-report --study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --review-scope PATH --review-scope-digest SHA256 --review-lane ID [--case ID] [--prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live]; report requires --journal and --artifacts")
		return 2
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	study := fs.String("study", "", "")
	corpus := fs.String("corpus", "", "")
	goal := fs.String("goal-scope", "", "")
	goalPin := fs.String("goal-scope-digest", "", "")
	scope := fs.String("review-scope", "", "")
	scopePin := fs.String("review-scope-digest", "", "")
	laneID := fs.String("review-lane", "", "")
	caseID := fs.String("case", "", "")
	preparedPin := fs.String("prepared-digest", "", "")
	journalPath := fs.String("journal", "", "")
	artifactsPath := fs.String("artifacts", "", "")
	live := fs.Bool("live", false, "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return usage()
	}
	if cmd == "review-prepare" && (*caseID == "" || *preparedPin != "" || *journalPath != "" || *artifactsPath != "" || *live) {
		return diagnostic(errOut, "prepare_has_no_live_effects", 2)
	}
	if cmd == "review-report" && (*caseID != "" || *preparedPin != "" || *live || *journalPath == "" || *artifactsPath == "" || filepath.Clean(*journalPath) == filepath.Clean(*artifactsPath)) {
		return diagnostic(errOut, "invalid_report_flags", 2)
	}
	if cmd == "review-capture" && (*caseID == "" || !documentDigestPattern.MatchString(*preparedPin) || !*live || *journalPath == "" || *artifactsPath == "" || filepath.Clean(*journalPath) == filepath.Clean(*artifactsPath)) {
		return diagnostic(errOut, "explicit_capture_binding_required", 2)
	}
	in, e := loadReviewInputs(*study, *corpus, *goal, *goalPin, *scope, *scopePin, *laneID)
	if e != nil {
		return diagnostic(errOut, "review_inputs_refused", 2)
	}
	if cmd == "review-report" {
		return runReviewReport(ctx, in, *journalPath, *artifactsPath, out, errOut)
	}
	c, ok := in.cases[*caseID]
	if !ok {
		return diagnostic(errOut, "foreign_case", 2)
	}
	req, wire, e := reviewRequest(ctx, c, in.selected)
	if e != nil {
		return diagnostic(errOut, "review_request_refused", 2)
	}
	selected := reviewPreparedDigest(in, *caseID, wire)
	if cmd == "review-prepare" {
		p := map[string]any{"version": "prose-lab.review-prepared/1", "case_id": *caseID, "case_kind": c.Kind, "expected": c.Expected, "study_digest": in.studyDigest, "corpus_digest": in.corpus.Digest, "goal_scope_digest": in.goalDigest, "review_scope_digest": in.reviewDigest, "review_lane": in.laneID, "requested_review_model": in.selected.Model, "native_request_digest": hashBytes(wire), "prepared_digest": selected, "reservation_micro_usd": in.selected.reserveCost(), "planned": len(in.cases), "independent_acceptance": false, "publication_authority": false, "live": false}
		if json.NewEncoder(out).Encode(p) != nil {
			return 2
		}
		return 0
	}
	if selected != *preparedPin {
		return diagnostic(errOut, "prepared_digest_mismatch", 2)
	}
	root, e := documentArtifactRoot(*artifactsPath)
	if e != nil {
		return diagnostic(errOut, "artifacts_refused", 2)
	}
	defer root.Close()
	marker := map[string]string{"case_id": *caseID, "prepared_digest": selected, "study_digest": in.studyDigest}
	if e = writeExclusive(root, *caseID+".started.json", marker); e != nil {
		return diagnostic(errOut, "case_already_attempted", 2)
	}
	receipt := reviewReceipt{Version: "prose-lab.review-capture/1", CaseID: *caseID, StudyDigest: in.studyDigest, PreparedDigest: selected, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, ReviewScopeDigest: in.reviewDigest, ReviewLane: in.laneID, RequestedReviewModel: in.selected.Model, NativeRequestDigest: hashBytes(wire), Status: "held", Expected: c.Expected, ChargeBasis: "unavailable"}
	persist := func(code string, status int) int {
		receipt.Status = "held"
		receipt.Matched = false
		receipt.ErrorClass = code
		if e := writeExclusive(root, *caseID+".receipt.json", receipt); e != nil {
			return diagnostic(errOut, "artifact_write_failed", 2)
		}
		if json.NewEncoder(out).Encode(receipt) != nil {
			return 2
		}
		return status
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.captureHTTP == nil {
		deps.captureHTTP = func() (*http.Client, *captureTransport) { return studyHTTP("anthropic") }
	}
	j, e := openJournal(*journalPath, in.boundStudy, in.study.BudgetMicroUSD, deps.now)
	if e != nil {
		return persist("journal_refused", 2)
	}
	defer j.close()
	if e = reviewPriorAttempts(ctx, j, root, in, *caseID); e != nil {
		return persist("prior_cohort_held", 2)
	}
	start := j.charged
	charge := func() {
		v := j.charged
		delta := v - start
		receipt.CumulativeChargedMicroUSD = &v
		receipt.CaseChargedMicroUSD = &delta
		receipt.ChargeBasis = "journal-usage-priced-or-reserved-local-accounting"
	}
	hc, cap := deps.captureHTTP()
	if hc == nil || cap == nil {
		charge()
		return persist("review_client_unavailable", 2)
	}
	inner, e := anthropic.New(anthropic.Config{APIKey: deps.apiKey, HTTPClient: hc, DefaultModel: in.selected.Model})
	if e != nil {
		charge()
		return persist("review_client_unavailable", 2)
	}
	client := &documentTextClient{inner: &labClient{inner: inner, attempt: &attemptRunner{journal: j, lane: in.selected, capture: cap, now: deps.now, caseID: *caseID, expectedRequestDigest: hashBytes(wire)}}}
	// The request was prepared by the same reviewer; its prompt/wire is checked against the captured reservation.
	d, e := reviewReviewer(c, in.selected.Model, int(in.selected.MaxOutputTokens), client).Review(ctx, reviewClaim(c))
	charge()
	if e != nil {
		return persist("review_incomplete", 2)
	}
	// A changed native encoder or request is an incomplete attempt, never qualified.
	if req.Model != in.selected.Model || len(j.pending) != 0 {
		return persist("review_binding_refused", 2)
	}
	var last reservation
	for seq := 1; seq < j.next; seq++ {
		var r reservation
		if readJournalJSON(j.root, fmt.Sprintf("reserve-%06d.json", seq), &r) == nil && r.CaseID == *caseID {
			last = r
		}
	}
	if last.RequestHash != hashBytes(wire) {
		return persist("native_request_mismatch", 2)
	}
	receipt.Status = d.Status
	receipt.Matched = d.Status == c.Expected
	if e = writeExclusive(root, *caseID+".reason.json", map[string]string{"reason": d.Reason}); e != nil {
		return persist("reason_write_failed", 2)
	}
	if e = writeExclusive(root, *caseID+".receipt.json", receipt); e != nil {
		return diagnostic(errOut, "artifact_write_failed", 2)
	}
	if json.NewEncoder(out).Encode(receipt) != nil {
		return 2
	}
	if receipt.Matched {
		return 0
	}
	return 1
}
func runReviewReport(ctx context.Context, in reviewInputs, path, artifactPath string, out, errOut io.Writer) int {
	attempts, charged, e := auditJournal(path, in.boundStudy, in.study.BudgetMicroUSD)
	if e != nil {
		return diagnostic(errOut, "journal_audit_unavailable", 2)
	}
	artifacts, artifactErr := existingReviewArtifacts(artifactPath)
	if artifacts != nil {
		defer artifacts.Close()
	}
	report := reviewReport{Version: "prose-lab.review-report/1", StudyDigest: in.studyDigest, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, ReviewScopeDigest: in.reviewDigest, ReviewLane: in.laneID, Planned: len(in.cases), TotalChargedMicroUSD: charged, ChargeBasis: "journal-usage-priced-or-reserved-local-accounting"}
	seen := map[string]bool{}
	results := map[string]reviewCaseReport{}
	var cumulative int64
	markInvalid := func(id string, c reviewCase) {
		report.InvalidCount++
		if c.ID != "" {
			results[id] = reviewCaseReport{CaseID: id, Expected: c.Expected, Status: "invalid"}
		}
	}
	for _, a := range attempts {
		r, o := a.Reservation, a.Outcome
		caseCharge := r.MicroUSD
		if o != nil && o.ActualMicroUSD != nil {
			caseCharge = *o.ActualMicroUSD
		}
		cumulative += caseCharge
		c, ok := in.cases[r.CaseID]
		if !ok {
			markInvalid("", reviewCase{})
			continue
		}
		if seen[r.CaseID] {
			markInvalid(r.CaseID, c)
			continue
		}
		seen[r.CaseID] = true
		if r.Provider != "anthropic" || r.Model != in.selected.Model || r.Pricing != in.selected || r.MicroUSD != in.selected.reserveCost() {
			markInvalid(r.CaseID, c)
			continue
		}
		_, wire, err := reviewRequest(ctx, c, in.selected)
		if err != nil || r.RequestHash != hashBytes(wire) || r.RequestJSON != string(wire) {
			markInvalid(r.CaseID, c)
			continue
		}
		if o == nil {
			results[c.ID] = reviewCaseReport{CaseID: c.ID, Expected: c.Expected, Status: "pending"}
			continue
		}
		if o.Status == "failed" || o.Status == "unknown" {
			results[c.ID] = reviewCaseReport{CaseID: c.ID, Expected: c.Expected, Status: o.Status}
			continue
		}
		if o.Status != "completed" || o.HTTPStatus < 200 || o.HTTPStatus >= 300 || !o.UsagePresent || o.ActualMicroUSD == nil {
			markInvalid(r.CaseID, c)
			continue
		}
		d, err := reviewReplay(ctx, c, in.selected, wire, []byte(o.Response))
		if err != nil || reviewArtifactValidate(artifacts, in, c, r, d, caseCharge, cumulative) != nil {
			markInvalid(r.CaseID, c)
			continue
		}
		results[c.ID] = reviewCaseReport{CaseID: c.ID, Expected: c.Expected, Status: d.Status, Matched: d.Status == c.Expected}
	}
	ids := make([]string, 0, len(in.cases))
	for id := range in.cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v, ok := results[id]
		if !ok {
			v = reviewCaseReport{CaseID: id, Expected: in.cases[id].Expected, Status: "missing"}
		}
		switch v.Status {
		case "missing":
			report.Missing++
		case "pending":
			report.Pending++
		case "failed":
			report.Failed++
		case "unknown":
			report.Unknown++
		case "supported", "unsupported", "inconclusive":
			report.Completed++
			if v.Status == "inconclusive" {
				report.Inconclusive++
			}
			if v.Expected == "unsupported" && v.Status == "supported" {
				report.FalseSupport++
			}
			if v.Expected == "supported" && v.Status != "supported" {
				report.FalseHold++
			}
		}
		report.Cases = append(report.Cases, v)
	}
	report.MatchedFixtureExpectations = artifactErr == nil && report.Completed == report.Planned && report.Missing == 0 && report.Pending == 0 && report.Failed == 0 && report.Unknown == 0 && report.InvalidCount == 0 && report.FalseSupport == 0 && report.FalseHold == 0 && report.Inconclusive == 0
	if json.NewEncoder(out).Encode(report) != nil {
		return 2
	}
	if artifactErr != nil || report.InvalidCount > 0 || report.Missing > 0 || report.Pending > 0 || report.Failed > 0 || report.Unknown > 0 {
		return 2
	}
	if !report.MatchedFixtureExpectations {
		return 1
	}
	return 0
}

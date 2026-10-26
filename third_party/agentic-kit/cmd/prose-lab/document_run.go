package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

type documentDeps struct {
	generation llmclient.LLMClient
	review     llmclient.LLMClient
	now        func() time.Time
}
type documentRunReceipt struct {
	Version                   string `json:"version"`
	CellID                    string `json:"cell_id"`
	CorpusDigest              string `json:"corpus_digest"`
	StudyDigest               string `json:"study_digest"`
	PreparedDigest            string `json:"prepared_digest"`
	GenerationLane            string `json:"generation_lane"`
	ReviewLane                string `json:"review_lane"`
	RequestedGenerationModel  string `json:"requested_generation_model"`
	RequestedReviewModel      string `json:"requested_review_model"`
	TextProjection            string `json:"text_projection"`
	Status                    string `json:"status"`
	DocumentDigest            string `json:"document_digest,omitempty"`
	MarkdownDigest            string `json:"markdown_digest,omitempty"`
	ChargeBasis               string `json:"charge_basis"`
	CumulativeChargedMicroUSD *int64 `json:"cumulative_charged_micro_usd,omitempty"`
	CellChargedMicroUSD       *int64 `json:"cell_charged_micro_usd,omitempty"`
	NegativeCasesEvaluated    bool   `json:"negative_cases_evaluated"`
	IndependentAcceptance     bool   `json:"independent_acceptance"`
	PublicationAuthority      bool   `json:"publication_authority"`
	ErrorClass                string `json:"error_class,omitempty"`
	GoalScopeDigest           string `json:"goal_scope_digest,omitempty"`
	LengthCriterionID         string `json:"length_criterion_id,omitempty"`
	LengthClass               string `json:"length_class,omitempty"`
	LengthMinWords            int    `json:"length_min_words,omitempty"`
	LengthMaxWords            int    `json:"length_max_words,omitempty"`
	LengthWordCount           int    `json:"length_word_count,omitempty"`
	LengthDeviation           *int   `json:"length_deviation,omitempty"`
	LengthStatus              string `json:"length_status,omitempty"`
	CountRule                 string `json:"count_rule,omitempty"`
	SemanticQuality           string `json:"semantic_quality,omitempty"`
	GoalsDigest               string `json:"goals_digest,omitempty"`
}

const documentRunnerProtocol = "prose-lab.document-runner/2"
const documentGenerationTemplate = "document-wire/v1"
const documentReviewTemplate = "configured-model-heuristic/v1"

var documentDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func documentStudyBytes(raw []byte, digest string) []byte {
	return append(append([]byte{}, raw...), []byte("\nprotocol:"+documentRunnerProtocol+"\ngeneration-template:"+documentGenerationTemplate+"\nreview-template:"+documentReviewTemplate+"\ntext-projection:"+documentTextProjection+"\ncorpus-sha256:"+digest+"\n")...)
}
func documentPreparedDigest(studyDigest, cellID, generationLane, reviewLane string) string {
	return hashBytes(jsonBytes(struct {
		Protocol       string `json:"protocol"`
		StudyDigest    string `json:"study_digest"`
		CellID         string `json:"cell_id"`
		GenerationLane string `json:"generation_lane"`
		ReviewLane     string `json:"review_lane"`
	}{documentRunnerProtocol, studyDigest, cellID, generationLane, reviewLane}))
}
func documentStudyBytesV2(raw []byte, corpusDigest, scopeDigest string) []byte {
	return append(append([]byte{}, raw...), []byte("\nprotocol:"+documentRunnerProtocolV2+"\ngeneration-template:"+documentGenerationTemplate+"\nreview-template:"+documentReviewTemplate+"\ntext-projection:"+documentTextProjection+"\ncorpus-sha256:"+corpusDigest+"\ngoal-scope-sha256:"+scopeDigest+"\n")...)
}
func documentPreparedDigestV2(studyDigest, cellID, generationLane, reviewLane, scopeDigest string) string {
	return hashBytes(jsonBytes(struct {
		Protocol       string `json:"protocol"`
		StudyDigest    string `json:"study_digest"`
		CellID         string `json:"cell_id"`
		GenerationLane string `json:"generation_lane"`
		ReviewLane     string `json:"review_lane"`
		ScopeDigest    string `json:"goal_scope_digest"`
	}{documentRunnerProtocolV2, studyDigest, cellID, generationLane, reviewLane, scopeDigest}))
}
func documentCharges(j *journal, start int64) (cumulative, cell int64) {
	return j.charged, j.charged - start
}
func documentModelSetsDisjoint(a, b lane) bool {
	am := map[string]bool{a.Model: true}
	if a.ResolvedModel != "" {
		am[a.ResolvedModel] = true
	}
	for _, id := range []string{b.Model, b.ResolvedModel} {
		if id != "" && am[id] {
			return false
		}
	}
	return true
}
func documentArtifactRoot(path string) (*os.Root, error) {
	if path == "" {
		return nil, errors.New("missing artifacts root")
	}
	if e := os.Mkdir(path, 0700); e != nil && !errors.Is(e, os.ErrExist) {
		return nil, e
	}
	st, e := os.Lstat(path)
	if e != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("artifacts require private non-symlink directory")
	}
	return os.OpenRoot(path)
}
func artifactWriteBytes(r *os.Root, name string, b []byte) error {
	if len(b) > 4<<20 {
		return errors.New("artifact byte bound")
	}
	f, e := r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	return errors.Join(e, f.Close())
}
func documentPersist(root *os.Root, cell string, v *prose.DocumentVersion, rendered *prose.RenderedDocument, receipt documentRunReceipt) error {
	if v != nil {
		if e := writeExclusive(root, cell+".version.json", v); e != nil {
			return e
		}
	}
	if rendered != nil {
		if e := artifactWriteBytes(root, cell+".md", []byte(rendered.Markdown)); e != nil {
			return e
		}
	}
	return writeExclusive(root, cell+".receipt.json", receipt)
}
func documentPersistV2(root *os.Root, cell string, v *prose.DocumentVersion, rendered *prose.RenderedDocument, goals *documentGoalsSidecar, receipt documentRunReceipt) error {
	if v != nil {
		if e := writeExclusive(root, cell+".version.json", v); e != nil {
			return e
		}
	}
	if rendered != nil {
		if e := artifactWriteBytes(root, cell+".md", []byte(rendered.Markdown)); e != nil {
			return e
		}
	}
	if goals != nil {
		if e := writeExclusive(root, cell+".goals.json", goals); e != nil {
			return e
		}
	}
	return writeExclusive(root, cell+".receipt.json", receipt)
}
func runDocument(ctx context.Context, args []string, out, errOut io.Writer) int {
	return runDocumentWith(ctx, args, out, errOut, documentDeps{})
}
func runDocumentWith(ctx context.Context, args []string, out, errOut io.Writer, deps documentDeps) int {
	usage := func() int {
		fmt.Fprintln(errOut, "Usage: prose-lab document-prepare|document-capture --study PATH --corpus DIR --cell ID --generation-lane ID --review-lane ID [--goal-scope PATH --goal-scope-digest SHA256] [--prepared-digest SHA256 --journal PRIVATE_DIR --artifacts PRIVATE_DIR --live]")
		return 2
	}
	if len(args) < 1 || (args[0] != "document-prepare" && args[0] != "document-capture") {
		return usage()
	}
	fs := flag.NewFlagSet("prose-lab-document", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	studyPath := fs.String("study", "", "study JSON")
	corpusPath := fs.String("corpus", "", "corpus dir")
	cellID := fs.String("cell", "", "manifest cell")
	genID := fs.String("generation-lane", "", "Anthropic generation lane")
	revID := fs.String("review-lane", "", "Anthropic review lane")
	journalPath := fs.String("journal", "", "private journal")
	artifactsPath := fs.String("artifacts", "", "private artifacts")
	live := fs.Bool("live", false, "explicit live capture")
	preparedDigest := fs.String("prepared-digest", "", "exact selected cell and lanes digest")
	goalScopePath := fs.String("goal-scope", "", "independent v2 goal scope")
	goalScopeDigest := fs.String("goal-scope-digest", "", "raw SHA256 of goal scope")
	if e := fs.Parse(args[1:]); e != nil || fs.NArg() != 0 {
		return usage()
	}
	scopeFlagsSeen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "goal-scope" || f.Name == "goal-scope-digest" {
			scopeFlagsSeen = true
		}
	})
	var study canaryStudy
	raw, e := readStudyJSON(*studyPath, &study)
	if e != nil || study.validate() != nil {
		return diagnostic(errOut, "invalid_study", 2)
	}
	corpus, e := loadDocumentCorpus(*corpusPath)
	if e != nil {
		return diagnostic(errOut, "invalid_corpus", 2)
	}
	var scope documentGoalScope
	if corpus.Manifest.Version == 1 {
		if scopeFlagsSeen {
			return diagnostic(errOut, "v1_rejects_goal_scope", 2)
		}
	} else {
		if *goalScopePath == "" || *goalScopeDigest == "" {
			return diagnostic(errOut, "goal_scope_required", 2)
		}
		scope, e = loadDocumentGoalScope(*goalScopePath, *goalScopeDigest, corpus)
		if e != nil {
			return diagnostic(errOut, "goal_scope_refused", 2)
		}
	}
	cell, ok := corpus.cell(*cellID)
	if !ok {
		return diagnostic(errOut, "foreign_cell", 2)
	}
	expected := map[string]bool{}
	for _, id := range study.ExpectedCaseIDs {
		expected[id] = true
	}
	if len(expected) != 36 {
		return diagnostic(errOut, "study_requires_exact_36_cells", 2)
	}
	for _, id := range corpus.sortedIDs() {
		if !expected[id] {
			return diagnostic(errOut, "study_roster_mismatch", 2)
		}
	}
	gl, gok := study.Lanes[*genID]
	rl, rok := study.Lanes[*revID]
	if !gok || !rok || *genID == *revID || gl.Provider != "anthropic" || rl.Provider != "anthropic" || !documentModelSetsDisjoint(gl, rl) || gl.MaxOutputTokens < 4096 || rl.MaxOutputTokens < 1 || rl.MaxOutputTokens > 8192 {
		return diagnostic(errOut, "distinct_anthropic_lanes_required", 2)
	}
	bound := documentStudyBytes(raw, corpus.Digest)
	if corpus.Manifest.Version == 2 {
		bound = documentStudyBytesV2(raw, corpus.Digest, *goalScopeDigest)
	}
	studyDigest := hashBytes(bound)
	selectedDigest := documentPreparedDigest(studyDigest, *cellID, *genID, *revID)
	if corpus.Manifest.Version == 2 {
		selectedDigest = documentPreparedDigestV2(studyDigest, *cellID, *genID, *revID, *goalScopeDigest)
	}
	if args[0] == "document-prepare" {
		if *live || *journalPath != "" || *artifactsPath != "" || *preparedDigest != "" {
			return diagnostic(errOut, "prepare_has_no_live_effects", 2)
		}
		prepared := map[string]any{"version": "prose-lab.document-prepared/1", "study_digest": studyDigest, "prepared_digest": selectedDigest, "cell_id": *cellID, "generation_lane": *genID, "review_lane": *revID, "requested_generation_model": gl.Model, "requested_review_model": rl.Model, "text_projection": documentTextProjection, "corpus_digest": corpus.Digest, "documents": len(corpus.Fixtures), "cells": len(corpus.Manifest.Cells), "available_negative_cases": documentNegativeCount(corpus), "negative_cases_evaluated": false, "reservation_per_generation_micro_usd": gl.reserveCost(), "reservation_per_review_micro_usd": rl.reserveCost(), "independent_acceptance": false, "live": false}
		if corpus.Manifest.Version == 2 {
			metadata := make([]map[string]any, 0, len(scope.Documents))
			for _, x := range scope.Documents {
				metadata = append(metadata, map[string]any{"document_id": x.DocumentID, "criterion_id": x.LengthCriterionID, "class": x.LengthClass, "min_words": x.MinWords, "max_words": x.MaxWords, "reason_digest": x.ReasonDigest})
			}
			prepared["version"] = "prose-lab.document-prepared/2"
			prepared["goal_scope_digest"] = *goalScopeDigest
			prepared["length_goals"] = metadata
			prepared["long_artifact"] = scope.LongArtifact
			prepared["count_rule"] = documentCountRule
			prepared["semantic_quality"] = "unassessed"
		}
		if json.NewEncoder(out).Encode(prepared) != nil {
			return 2
		}
		return 0
	}
	if !*live || *journalPath == "" || *artifactsPath == "" || filepath.Clean(*journalPath) == filepath.Clean(*artifactsPath) {
		return diagnostic(errOut, "explicit_live_private_paths_required", 2)
	}
	if !documentDigestPattern.MatchString(*preparedDigest) || *preparedDigest != selectedDigest {
		return diagnostic(errOut, "prepared_digest_mismatch", 2)
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	root, e := documentArtifactRoot(*artifactsPath)
	if e != nil {
		return diagnostic(errOut, "artifacts_refused", 2)
	}
	defer root.Close()
	// This marker is exclusive and is never removed. It prevents replay, including after a failed attempt.
	marker := *cellID + ".started.json"
	markerBody := map[string]any{"cell_id": *cellID, "study_digest": studyDigest, "prepared_digest": selectedDigest, "corpus_digest": corpus.Digest, "at_unix_nano": deps.now().UnixNano()}
	if corpus.Manifest.Version == 2 {
		markerBody["goal_scope_digest"] = *goalScopeDigest
	}
	if e = writeExclusive(root, marker, markerBody); e != nil {
		return diagnostic(errOut, "cell_already_attempted", 2)
	}
	receipt := documentRunReceipt{Version: "prose-lab.document-run/1", CellID: *cellID, StudyDigest: studyDigest, PreparedDigest: selectedDigest, CorpusDigest: corpus.Digest, GenerationLane: *genID, ReviewLane: *revID, RequestedGenerationModel: gl.Model, RequestedReviewModel: rl.Model, TextProjection: documentTextProjection, Status: "held", ChargeBasis: "unavailable", NegativeCasesEvaluated: false, IndependentAcceptance: false, PublicationAuthority: false}
	if corpus.Manifest.Version == 2 {
		g := corpus.Fixtures[cell.DocumentID].LengthGoal
		receipt.Version = "prose-lab.document-run/2"
		receipt.GoalScopeDigest = *goalScopeDigest
		receipt.LengthCriterionID = g.CriterionID
		receipt.LengthClass = g.Class
		receipt.LengthMinWords = g.MinWords
		receipt.LengthMaxWords = g.MaxWords
		receipt.CountRule = documentCountRule
		receipt.SemanticQuality = "unassessed"
	}
	var j *journal
	var chargeStart int64
	updateCharge := func() {
		if j != nil {
			cumulative, cellCharge := documentCharges(j, chargeStart)
			receipt.CumulativeChargedMicroUSD = &cumulative
			receipt.CellChargedMicroUSD = &cellCharge
			receipt.ChargeBasis = "journal-usage-priced-or-reserved-local-accounting"
		}
	}
	hold := func(class string, v *prose.DocumentVersion, rendered *prose.RenderedDocument) int {
		receipt.ErrorClass = class
		updateCharge()
		var persist error
		if corpus.Manifest.Version == 2 {
			persist = documentPersistV2(root, *cellID, v, rendered, nil, receipt)
		} else {
			persist = documentPersist(root, *cellID, v, rendered, receipt)
		}
		if err := persist; err != nil {
			return diagnostic(errOut, "artifact_write_failed", 2)
		}
		return diagnostic(errOut, class, 2)
	}
	j, e = openJournal(*journalPath, bound, study.BudgetMicroUSD, deps.now)
	if e != nil {
		return hold("journal_refused", nil, nil)
	}
	chargeStart = j.charged
	defer j.close()
	makeClient := func(l lane) (llmclient.LLMClient, error) {
		httpClient, capture := studyHTTP("anthropic")
		inner, e := anthropic.New(anthropic.Config{HTTPClient: httpClient, DefaultModel: l.Model})
		if e != nil {
			return nil, e
		}
		return &labClient{inner: inner, attempt: &attemptRunner{journal: j, lane: l, capture: capture, now: deps.now, caseID: *cellID}}, nil
	}
	gen, rev := deps.generation, deps.review
	if gen == nil {
		gen, e = makeClient(gl)
		if e != nil {
			return hold("generation_client_unavailable", nil, nil)
		}
	}
	if rev == nil {
		rev, e = makeClient(rl)
		if e != nil {
			return hold("review_client_unavailable", nil, nil)
		}
	}
	engine, target, e := documentEngine(&documentTextClient{inner: gen}, &documentTextClient{inner: rev}, gl.Model, rl.Model, int(rl.MaxOutputTokens), corpus.Fixtures[cell.DocumentID], cell, deps.now)
	if e != nil {
		return hold("document_target_invalid", nil, nil)
	}
	v, e := engine.Generate(ctx, target)
	if e != nil {
		return hold("generation_or_support_held", nil, nil)
	}
	rendered, e := engine.Render(ctx, v)
	if e != nil {
		return hold("render_held", &v, nil)
	}
	if corpus.Manifest.Version == 2 {
		d := corpus.Fixtures[cell.DocumentID]
		if e = documentStructuralCoverage(d, v); e != nil {
			return hold("coverage_held", &v, &rendered)
		}
		goals, status, deviation, assessErr := documentLengthAssess(d, v, rendered, *goalScopeDigest, deps.now())
		if assessErr != nil {
			return hold("length_assessment_refused", &v, &rendered)
		}
		receipt.LengthWordCount = goals.WordCount
		receipt.LengthDeviation = &deviation
		receipt.LengthStatus = status
		receipt.GoalsDigest = hashBytes(jsonBytes(goals))
		receipt.DocumentDigest = v.Receipt.DocumentDigest
		receipt.MarkdownDigest = rendered.MarkdownDigest
		if status == "hold" {
			receipt.ErrorClass = "required_length_held"
		} else {
			receipt.Status = "candidate_checked"
		}
		updateCharge()
		if e = documentPersistV2(root, *cellID, &v, &rendered, &goals, receipt); e != nil {
			return diagnostic(errOut, "artifact_write_failed", 2)
		}
		if status == "hold" {
			return diagnostic(errOut, "required_length_held", 2)
		}
		if e = json.NewEncoder(out).Encode(receipt); e != nil {
			return 2
		}
		return 0
	}
	if e = documentCoverage(corpus.Fixtures[cell.DocumentID], v, rendered); e != nil {
		return hold("coverage_or_length_held", &v, &rendered)
	}
	receipt.Status = "candidate_checked"
	receipt.DocumentDigest = v.Receipt.DocumentDigest
	receipt.MarkdownDigest = rendered.MarkdownDigest
	updateCharge()
	if e = documentPersist(root, *cellID, &v, &rendered, receipt); e != nil {
		return diagnostic(errOut, "artifact_write_failed", 2)
	}
	if e = json.NewEncoder(out).Encode(receipt); e != nil {
		return 2
	}
	return 0
}
func documentNegativeCount(c documentCorpus) int {
	n := 0
	for _, f := range c.Fixtures {
		n += len(f.NegativeCases)
	}
	return n
}

// Structural coverage is a necessary floor. Semantic acceptance remains external.
func documentCoverage(d documentFixture, v prose.DocumentVersion, r prose.RenderedDocument) error {
	if n := len(strings.Fields(r.Markdown)); n < d.MinWords || n > d.MaxWords {
		return errors.New("rendered word count outside fixed range")
	}
	return documentStructuralCoverage(d, v)
}
func documentStructuralCoverage(d documentFixture, v prose.DocumentVersion) error {
	nodes := map[string]prose.DocNode{}
	for _, n := range v.Nodes {
		nodes[n.ID] = n
	}
	for _, s := range d.Sections {
		body, ok := nodes[s.ID+"-body"]
		if !ok {
			return errors.New("missing section body")
		}
		cited := map[string]bool{}
		for _, c := range body.Claims {
			for _, id := range c.FactIDs {
				cited[id] = true
			}
		}
		for _, id := range s.RequiredFactIDs {
			if !cited[id] {
				return errors.New("required fact not cited in section")
			}
		}
	}
	return nil
}

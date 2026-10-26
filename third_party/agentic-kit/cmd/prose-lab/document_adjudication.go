package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

const documentReviewPacketVersion = "prose-lab.document-review-packet/1"

const documentReviewHelp = "Usage: prose-lab document-review-prepare|document-review-report --study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --cell ID --generation-lane ID --review-lane ID --artifacts PRIVATE_DIR --receipt-digest SHA256 --at CANONICAL_UTC [--include-content]\nReport additionally requires --packet-digest SHA256 and optionally paired --adjudication PATH --adjudication-digest SHA256.\nRead-only caller-declared review. Default output excludes document content. Identity verification and publication authority remain separate. Report exits: 0 declared criteria pass, 1 required failure, 2 unknown or refused.\n"

type documentReviewLength struct {
	CriterionID      string `json:"criterion_id"`
	Class            string `json:"class"`
	MinWords         int    `json:"min_words"`
	MaxWords         int    `json:"max_words"`
	CountRule        string `json:"count_rule"`
	WordCount        int    `json:"word_count"`
	Deviation        int    `json:"deviation"`
	Status           string `json:"status"`
	DefinitionDigest string `json:"definition_digest"`
	ProfileDigest    string `json:"profile_digest"`
}
type documentReviewGoal struct {
	CriterionID      string `json:"criterion_id"`
	DefinitionDigest string `json:"definition_digest"`
	ProfileDigest    string `json:"profile_digest"`
}
type documentReviewIdentity struct {
	Version            string               `json:"version"`
	CellID             string               `json:"cell_id"`
	DocumentID         string               `json:"document_id"`
	CorpusDigest       string               `json:"corpus_digest"`
	GoalScopeDigest    string               `json:"goal_scope_digest"`
	StudyDigest        string               `json:"study_digest"`
	PreparedDigest     string               `json:"prepared_digest"`
	ReceiptDigest      string               `json:"receipt_digest"`
	VersionFileDigest  string               `json:"version_file_digest"`
	MarkdownFileDigest string               `json:"markdown_file_digest"`
	GoalsFileDigest    string               `json:"goals_file_digest"`
	DocumentDigest     string               `json:"document_digest"`
	MarkdownDigest     string               `json:"markdown_digest"`
	SubjectDigest      string               `json:"subject_digest"`
	CandidateStatus    string               `json:"candidate_status"`
	Length             documentReviewLength `json:"length"`
	SemanticGoals      []documentReviewGoal `json:"semantic_goals"`
}
type documentReviewTrust struct {
	Trust                       string `json:"trust"`
	IdentityVerification        string `json:"identity_verification"`
	SourceRevalidation          string `json:"source_revalidation"`
	ProviderHistoryVerification string `json:"provider_history_verification"`
	IndependentAcceptance       bool   `json:"independent_acceptance"`
	PublicationAuthority        bool   `json:"publication_authority"`
}

var documentDeclaredTrust = documentReviewTrust{"caller-declared", "not_performed", "not_performed", "not_performed", false, false}

type documentReviewContent struct {
	Purpose        string                `json:"purpose"`
	Audience       string                `json:"audience"`
	Voice          string                `json:"voice"`
	TechnicalLevel string                `json:"technical_level"`
	Markdown       string                `json:"markdown"`
	SpanMap        []prose.SpanMapEntry  `json:"span_map"`
	Facts          []documentFact        `json:"facts"`
	Sections       []documentSection     `json:"sections"`
	RequiredGoals  []documentGoal        `json:"required_goals"`
	Claims         []documentReviewClaim `json:"claims"`
}
type documentReviewClaim struct {
	NodeID          string   `json:"node_id"`
	ClaimID         string   `json:"claim_id"`
	CoordinateBasis string   `json:"coordinate_basis"`
	Start           int      `json:"start"`
	End             int      `json:"end"`
	Assertion       string   `json:"assertion"`
	FactIDs         []string `json:"fact_ids"`
	Contradictions  []string `json:"contradictions"`
}
type documentReviewPacket struct {
	documentReviewIdentity
	PacketDigest string `json:"packet_digest"`
	documentReviewTrust
	Content *documentReviewContent `json:"content,omitempty"`
}
type documentReviewFinding struct {
	CriterionID     string                 `json:"criterion_id"`
	Outcome         fitness.FindingOutcome `json:"outcome"`
	Rule            fitness.AssessmentRule `json:"rule"`
	Eligible        bool                   `json:"eligible"`
	EligibilityRule string                 `json:"eligibility_rule"`
	Reason          string                 `json:"reason,omitempty"`
}
type documentReviewReport struct {
	Version                string                  `json:"version"`
	PacketDigest           string                  `json:"packet_digest"`
	AdjudicationDigest     string                  `json:"adjudication_digest,omitempty"`
	DeclaredGoalsStatus    fitness.RequiredStatus  `json:"declared_goals_status"`
	DeclaredCriteriaStatus fitness.RequiredStatus  `json:"declared_criteria_status"`
	SemanticGoals          []documentReviewFinding `json:"semantic_goals"`
	Length                 documentReviewLength    `json:"length"`
	documentReviewTrust
	Content *documentReviewReportContent `json:"content,omitempty"`
}
type documentReviewReportContent struct {
	Reviewer *documentAdjudication     `json:"adjudication,omitempty"`
	Traces   []fitness.AssessmentTrace `json:"traces"`
	Packet   *documentReviewContent    `json:"packet"`
}

type documentReviewInputs struct {
	study          canaryStudy
	studyDigest    string
	corpus         documentCorpus
	cell           documentCell
	fixture        documentFixture
	scopeDigest    string
	preparedDigest string
	gen            lane
	rev            lane
	genLaneID      string
	revLaneID      string
	rootPath       string
	when           time.Time
}

func runDocumentReview(ctx context.Context, args []string, out, errOut io.Writer) int {
	refuse := func() int { return diagnostic(errOut, "document_review_refused", 2) }
	if len(args) == 0 {
		return refuse()
	}
	mode := args[0]
	if mode != "document-review-prepare" && mode != "document-review-report" {
		return refuse()
	}
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h" || args[1] == "help") {
		n, err := io.WriteString(out, documentReviewHelp)
		if err != nil || n != len(documentReviewHelp) {
			return 2
		}
		return 0
	}
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	studyPath := fs.String("study", "", "")
	corpusPath := fs.String("corpus", "", "")
	scopePath := fs.String("goal-scope", "", "")
	scopeDigest := fs.String("goal-scope-digest", "", "")
	cellID := fs.String("cell", "", "")
	genID := fs.String("generation-lane", "", "")
	revID := fs.String("review-lane", "", "")
	rootPath := fs.String("artifacts", "", "")
	receiptDigest := fs.String("receipt-digest", "", "")
	at := fs.String("at", "", "")
	include := fs.Bool("include-content", false, "")
	packetPin := fs.String("packet-digest", "", "")
	adjudicationPath := fs.String("adjudication", "", "")
	adjudicationPin := fs.String("adjudication-digest", "", "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return refuse()
	}
	reportOnlySeen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "packet-digest" || f.Name == "adjudication" || f.Name == "adjudication-digest" {
			reportOnlySeen = true
		}
	})
	if mode == "document-review-prepare" && reportOnlySeen {
		return refuse()
	}
	if mode == "document-review-report" && !documentDigestPattern.MatchString(*packetPin) {
		return refuse()
	}
	if (*adjudicationPath == "") != (*adjudicationPin == "") || (mode == "document-review-report" && *adjudicationPin != "" && !documentDigestPattern.MatchString(*adjudicationPin)) {
		return refuse()
	}
	when, err := reviewAt(*at)
	if err != nil || !documentDigestPattern.MatchString(*receiptDigest) || !documentDigestPattern.MatchString(*scopeDigest) {
		return refuse()
	}
	if ctx.Err() != nil {
		return refuse()
	}
	in, err := loadDocumentReviewInputs(*studyPath, *corpusPath, *scopePath, *scopeDigest, *cellID, *genID, *revID, *rootPath, when)
	if err != nil {
		return refuse()
	}
	packet, side, err := loadDocumentReviewPacket(in, *receiptDigest, *include)
	if err != nil || ctx.Err() != nil {
		return refuse()
	}
	if mode == "document-review-prepare" {
		return documentReviewWrite(out, errOut, packet, 0)
	}
	if packet.PacketDigest != *packetPin {
		return refuse()
	}
	var adjudication *documentAdjudication
	if *adjudicationPath != "" {
		a, e := loadDocumentAdjudication(*adjudicationPath, *adjudicationPin, *packetPin)
		if e != nil {
			return refuse()
		}
		adjudication = &a
	}
	report, code, err := assessDocumentReview(packet, side, in.fixture, adjudication, *adjudicationPin, when, *include)
	if err != nil || ctx.Err() != nil {
		return refuse()
	}
	return documentReviewWrite(out, errOut, report, code)
}

func documentReviewWrite(out, errOut io.Writer, value any, code int) int {
	b := append(jsonBytes(value), '\n')
	if len(b) > 8<<20 {
		return diagnostic(errOut, "document_review_output_refused", 2)
	}
	n, err := out.Write(b)
	if err != nil || n != len(b) {
		return diagnostic(errOut, "document_review_output_failed", 2)
	}
	return code
}

func loadDocumentReviewInputs(studyPath, corpusPath, scopePath, scopeDigest, cellID, genID, revID, rootPath string, when time.Time) (documentReviewInputs, error) {
	var in documentReviewInputs
	raw, err := readStudyJSON(studyPath, &in.study)
	if err != nil || in.study.validate() != nil {
		return in, errors.New("invalid study")
	}
	in.corpus, err = loadDocumentCorpus(corpusPath)
	if err != nil || in.corpus.Manifest.Version != 2 {
		return in, errors.New("invalid corpus")
	}
	if _, err = loadDocumentGoalScope(scopePath, scopeDigest, in.corpus); err != nil {
		return in, err
	}
	in.cell, _ = in.corpus.cell(cellID)
	if in.cell.ID == "" {
		return in, errors.New("foreign cell")
	}
	ids := map[string]bool{}
	for _, id := range in.study.ExpectedCaseIDs {
		ids[id] = true
	}
	if len(ids) != 36 {
		return in, errors.New("study roster size")
	}
	for _, id := range in.corpus.sortedIDs() {
		if !ids[id] {
			return in, errors.New("study roster mismatch")
		}
	}
	var ok1, ok2 bool
	in.gen, ok1 = in.study.Lanes[genID]
	in.rev, ok2 = in.study.Lanes[revID]
	if !ok1 || !ok2 || genID == revID || in.gen.Provider != "anthropic" || in.rev.Provider != "anthropic" || !documentModelSetsDisjoint(in.gen, in.rev) || in.gen.MaxOutputTokens < 4096 || in.rev.MaxOutputTokens < 1 || in.rev.MaxOutputTokens > 8192 {
		return in, errors.New("invalid lanes")
	}
	in.studyDigest = hashBytes(documentStudyBytesV2(raw, in.corpus.Digest, scopeDigest))
	in.preparedDigest = documentPreparedDigestV2(in.studyDigest, cellID, genID, revID, scopeDigest)
	in.scopeDigest, in.fixture, in.rootPath, in.when, in.genLaneID, in.revLaneID = scopeDigest, in.corpus.Fixtures[in.cell.DocumentID], rootPath, when, genID, revID
	return in, nil
}

func loadDocumentReviewPacket(in documentReviewInputs, receiptPin string, include bool) (documentReviewPacket, documentGoalsSidecar, error) {
	var packet documentReviewPacket
	var side documentGoalsSidecar
	root, err := existingReviewArtifacts(in.rootPath)
	if err != nil {
		return packet, side, err
	}
	defer root.Close()
	st, err := root.Stat(".")
	if err != nil || !st.IsDir() || (st.Mode().Perm() != 0500 && st.Mode().Perm() != 0700) {
		return packet, side, errors.New("artifact root permissions")
	}
	pathSt, err := os.Lstat(in.rootPath)
	if err != nil || !pathSt.IsDir() || !os.SameFile(pathSt, st) {
		return packet, side, errors.New("artifact root changed")
	}
	names := []string{in.cell.ID + ".receipt.json", in.cell.ID + ".version.json", in.cell.ID + ".md", in.cell.ID + ".goals.json"}
	var raw [4][]byte
	total := 0
	for i, name := range names {
		raw[i], err = reviewPrivateBytes(root, name, 4<<20)
		if err != nil {
			return packet, side, err
		}
		total += len(raw[i])
	}
	if total > 16<<20 || hashBytes(raw[0]) != receiptPin {
		return packet, side, errors.New("receipt pin mismatch")
	}
	var receipt documentRunReceipt
	var version prose.DocumentVersion
	if err = decodeLabJSON(raw[0], &receipt); err != nil {
		return packet, side, errors.New("invalid receipt")
	}
	if err = decodeDocumentReviewArtifact(raw[1], &version); err != nil {
		return packet, side, fmt.Errorf("invalid version: %w", err)
	}
	if err = decodeDocumentReviewArtifact(raw[3], &side); err != nil {
		return packet, side, errors.New("invalid goals")
	}
	if receipt.Version != "prose-lab.document-run/2" || receipt.CellID != in.cell.ID || receipt.CorpusDigest != in.corpus.Digest || receipt.GoalScopeDigest != in.scopeDigest || receipt.StudyDigest != in.studyDigest || receipt.PreparedDigest != in.preparedDigest || receipt.GenerationLane != in.genLaneID || receipt.ReviewLane != in.revLaneID || receipt.RequestedGenerationModel != in.gen.Model || receipt.RequestedReviewModel != in.rev.Model || receipt.TextProjection != documentTextProjection || receipt.SemanticQuality != "unassessed" || receipt.IndependentAcceptance || receipt.PublicationAuthority || receipt.NegativeCasesEvaluated {
		return packet, side, errors.New("receipt identity mismatch")
	}
	if receipt.ChargeBasis != "journal-usage-priced-or-reserved-local-accounting" || receipt.CumulativeChargedMicroUSD == nil || receipt.CellChargedMicroUSD == nil || *receipt.CumulativeChargedMicroUSD < 0 || *receipt.CellChargedMicroUSD < 0 || *receipt.CellChargedMicroUSD > *receipt.CumulativeChargedMicroUSD {
		return packet, side, errors.New("receipt accounting shape")
	}
	if !(receipt.Status == "candidate_checked" && receipt.ErrorClass == "" || receipt.Status == "held" && receipt.ErrorClass == "required_length_held") {
		return packet, side, errors.New("incomplete candidate")
	}
	if prose.VerifyDocument(version) != nil || version.Target.Session.ID != in.cell.ID || version.Receipt.DocumentDigest != receipt.DocumentDigest {
		return packet, side, errors.New("document mismatch")
	}
	_, timeErr := reviewAt(version.Receipt.CheckedAt.Format(time.RFC3339Nano))
	if timeErr != nil || version.Receipt.CheckedAt.After(in.when) {
		return packet, side, errors.New("document checked in future")
	}
	if len(version.Target.Input.Facts) != len(in.fixture.Facts) || len(version.Target.Input.Facts) == 0 {
		return packet, side, errors.New("fact cut mismatch")
	}
	observed := version.Target.Input.Facts[0].Span.ObservedAt
	_, timeErr = reviewAt(observed.Format(time.RFC3339Nano))
	if observed.After(in.when) || timeErr != nil {
		return packet, side, errors.New("fact acquisition time")
	}
	for _, f := range version.Target.Input.Facts {
		if !f.Span.ObservedAt.Equal(observed) {
			return packet, side, errors.New("mixed fact acquisition")
		}
	}
	expected, _, err := documentTarget(in.fixture, in.cell, observed)
	if err != nil || !bytes.Equal(jsonBytes(expected), jsonBytes(version.Target)) {
		return packet, side, errors.New("target mismatch")
	}
	if documentStructuralCoverage(in.fixture, version) != nil {
		return packet, side, errors.New("coverage mismatch")
	}
	rendered, err := prose.RenderMarkdown(version)
	if err != nil || rendered.MarkdownDigest != receipt.MarkdownDigest || !bytes.Equal([]byte(rendered.Markdown), raw[2]) {
		return packet, side, errors.New("markdown mismatch")
	}
	if hashBytes(jsonBytes(side)) != receipt.GoalsDigest {
		return packet, side, errors.New("goals digest mismatch")
	}
	if len(side.Goals) != len(in.fixture.RequiredGoals)+1 {
		return packet, side, errors.New("goals count")
	}
	originalAt := side.Goals[0].Result.EvaluatedAt
	_, timeErr = reviewAt(originalAt.Format(time.RFC3339Nano))
	if originalAt.After(in.when) || timeErr != nil {
		return packet, side, errors.New("goals time")
	}
	replayed, oldStatus, oldDeviation, err := documentLengthAssess(in.fixture, version, rendered, in.scopeDigest, originalAt)
	if err != nil || !bytes.Equal(jsonBytes(side), jsonBytes(replayed)) {
		return packet, side, errors.New("goals replay mismatch")
	}
	current, status, deviation, err := documentLengthAssess(in.fixture, version, rendered, in.scopeDigest, in.when)
	if err != nil || receipt.LengthDeviation == nil || oldStatus != receipt.LengthStatus || oldDeviation != *receipt.LengthDeviation || side.WordCount != receipt.LengthWordCount || side.CountRule != receipt.CountRule || receipt.LengthCriterionID != in.fixture.LengthGoal.CriterionID || receipt.LengthClass != in.fixture.LengthGoal.Class || receipt.LengthMinWords != in.fixture.LengthGoal.MinWords || receipt.LengthMaxWords != in.fixture.LengthGoal.MaxWords || (receipt.Status == "held") != (oldStatus == "hold") {
		return packet, side, errors.New("receipt length mismatch")
	}
	lengthProfile := current.Goals[len(current.Goals)-1].Profile
	lengthDigest, err := lengthProfile.Fingerprint()
	if err != nil {
		return packet, side, err
	}
	identity := documentReviewIdentity{Version: documentReviewPacketVersion, CellID: in.cell.ID, DocumentID: in.cell.DocumentID, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.scopeDigest, StudyDigest: in.studyDigest, PreparedDigest: in.preparedDigest, ReceiptDigest: receiptPin, VersionFileDigest: hashBytes(raw[1]), MarkdownFileDigest: hashBytes(raw[2]), GoalsFileDigest: hashBytes(raw[3]), DocumentDigest: receipt.DocumentDigest, MarkdownDigest: receipt.MarkdownDigest, SubjectDigest: current.Goals[0].Result.SubjectDigest, CandidateStatus: receipt.Status, Length: documentReviewLength{in.fixture.LengthGoal.CriterionID, in.fixture.LengthGoal.Class, in.fixture.LengthGoal.MinWords, in.fixture.LengthGoal.MaxWords, documentCountRule, current.WordCount, deviation, status, lengthProfile.Criteria[0].DefinitionDigest, lengthDigest}, SemanticGoals: make([]documentReviewGoal, 0, len(in.fixture.RequiredGoals))}
	for i := range in.fixture.RequiredGoals {
		p := current.Goals[i].Profile
		d, e := p.Fingerprint()
		if e != nil {
			return packet, side, e
		}
		identity.SemanticGoals = append(identity.SemanticGoals, documentReviewGoal{p.Criteria[0].ID, p.Criteria[0].DefinitionDigest, d})
	}
	packet = documentReviewPacket{documentReviewIdentity: identity, PacketDigest: hashBytes(jsonBytes(identity)), documentReviewTrust: documentDeclaredTrust}
	if include {
		packet.Content = documentReviewSelectedContent(in.fixture, version, rendered)
	}
	return packet, current, nil
}

func decodeDocumentReviewArtifact(raw []byte, target any) error {
	if _, err := jsonwire.Object(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

func documentReviewSelectedContent(f documentFixture, v prose.DocumentVersion, r prose.RenderedDocument) *documentReviewContent {
	c := &documentReviewContent{Purpose: f.Purpose, Audience: v.Target.Session.Presentation.Audience, Voice: v.Target.Session.Presentation.Voice, TechnicalLevel: v.Target.Session.Presentation.TechnicalLevel, Markdown: r.Markdown, SpanMap: r.SpanMap, Facts: f.Facts, Sections: f.Sections, RequiredGoals: f.RequiredGoals, Claims: []documentReviewClaim{}}
	for _, n := range v.Nodes {
		for _, claim := range n.Claims {
			for _, span := range n.Ranges {
				if span.ClaimID == claim.ID {
					c.Claims = append(c.Claims, documentReviewClaim{n.ID, claim.ID, "node-text-utf8-byte-range-half-open/v1", span.Start, span.End, claim.Assertion, claim.FactIDs, claim.Contradictions})
				}
			}
		}
	}
	return c
}

func assessDocumentReview(packet documentReviewPacket, side documentGoalsSidecar, fixture documentFixture, adjudication *documentAdjudication, inputDigest string, at time.Time, include bool) (documentReviewReport, int, error) {
	var report documentReviewReport
	obsByID := map[string]fitness.Observation{}
	known := map[string]bool{}
	for _, g := range packet.SemanticGoals {
		known[g.CriterionID] = true
	}
	if adjudication != nil {
		for _, obs := range adjudication.Observations {
			if !known[obs.CriterionID] || obs.CriterionID == packet.Length.CriterionID || obsByID[obs.CriterionID].CriterionID != "" {
				return report, 2, errors.New("foreign or duplicate observation")
			}
			hasDoc, hasMarkdown := false, false
			for _, digest := range obs.ArtifactDigests {
				hasDoc = hasDoc || digest == packet.DocumentDigest
				hasMarkdown = hasMarkdown || digest == packet.MarkdownDigest
			}
			if !hasDoc || !hasMarkdown {
				return report, 2, errors.New("unbound artifact reference")
			}
			obsByID[obs.CriterionID] = obs
		}
	}
	semanticProfile := fitness.Profile{Version: fitness.ProfileVersion, ID: "prose-lab/document-review-semantic", Domain: "prose-lab", Criteria: make([]fitness.Criterion, 0, len(packet.SemanticGoals))}
	eligible := make([]fitness.Observation, 0, len(obsByID))
	findings := make([]documentReviewFinding, 0, len(packet.SemanticGoals))
	for i, g := range packet.SemanticGoals {
		p := side.Goals[i].Profile
		if len(p.Criteria) != 1 || p.Criteria[0].ID != g.CriterionID {
			return report, 2, errors.New("profile mismatch")
		}
		semanticProfile.Criteria = append(semanticProfile.Criteria, p.Criteria[0])
		obs, present := obsByID[g.CriterionID]
		use := present && (adjudication == nil || adjudication.Reviewer.Relationship == "independent")
		var rows []fitness.Observation
		if use {
			rows = []fitness.Observation{obs}
			eligible = append(eligible, obs)
		}
		assessment, err := fitness.AssessProfile(p, packet.SubjectDigest, rows, at)
		if err != nil {
			return report, 2, err
		}
		trace := assessment.Trace[0]
		admitted := trace.Rule == fitness.RuleObserved || trace.Rule == fitness.RuleObservedUnknown || trace.Rule == fitness.RuleObservedNotApplicable
		eligibilityRule := string(trace.Rule)
		if present && !use {
			eligibilityRule = "independence-not-declared"
		}
		findings = append(findings, documentReviewFinding{CriterionID: g.CriterionID, Outcome: trace.Outcome, Rule: trace.Rule, Eligible: use && admitted, EligibilityRule: eligibilityRule})
		if include {
			findings[len(findings)-1].Reason = trace.Reason
		}
	}
	goals, err := fitness.AssessProfile(semanticProfile, packet.SubjectDigest, eligible, at)
	if err != nil {
		return report, 2, err
	}
	selectedProfile := semanticProfile
	selectedObs := append([]fitness.Observation{}, eligible...)
	if fixture.LengthGoal.Class == "required" {
		selectedProfile.ID = "prose-lab/document-review-selected"
		selectedProfile.Criteria = append(append([]fitness.Criterion{}, semanticProfile.Criteria...), side.Goals[len(side.Goals)-1].Profile.Criteria[0])
		selectedObs = append(selectedObs, side.Goals[len(side.Goals)-1].Observations...)
	}
	selected, err := fitness.AssessProfile(selectedProfile, packet.SubjectDigest, selectedObs, at)
	if err != nil {
		return report, 2, err
	}
	report = documentReviewReport{Version: "prose-lab.document-review-report/1", PacketDigest: packet.PacketDigest, AdjudicationDigest: inputDigest, DeclaredGoalsStatus: goals.RequiredStatus, DeclaredCriteriaStatus: selected.RequiredStatus, SemanticGoals: findings, Length: packet.Length, documentReviewTrust: documentDeclaredTrust}
	if include {
		report.Content = &documentReviewReportContent{Reviewer: adjudication, Traces: selected.Trace, Packet: packet.Content}
	}
	code := 2
	switch selected.RequiredStatus {
	case fitness.RequiredPass:
		code = 0
	case fitness.RequiredFail:
		code = 1
	}
	return report, code, nil
}

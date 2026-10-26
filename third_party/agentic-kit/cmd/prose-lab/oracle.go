package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

//go:embed oracle-criteria/cited-entailment-v1.txt
var oracleCriterionFS embed.FS

const oracleCriterionID = "cited-entailment/1"
const oracleCriterionPin = "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
const oracleCommonUsage = "--study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256 --base-review-scope PATH --review-scope PATH --review-scope-digest SHA256 --regression-source PATH"

// Oracle input and output are immutable within a call; concurrent calls share no state.
type oracleLineage struct {
	ArtifactKind         string         `json:"artifact_kind"`
	ArtifactDigest       string         `json:"artifact_digest"`
	JSONPointer          string         `json:"json_pointer"`
	MappingRule          string         `json:"mapping_rule,omitempty"`
	SourceDigest         string         `json:"source_digest,omitempty"`
	Method               string         `json:"method,omitempty"`
	Reason               string         `json:"reason,omitempty"`
	RegressionSource     *oracleLineage `json:"regression_source,omitempty"`
	FixtureExpected      *oracleLineage `json:"fixture_expected,omitempty"`
	FixtureExpectedValue string         `json:"fixture_expected_value,omitempty"`
	TargetFactID         string         `json:"target_fact_id,omitempty"`
}
type oracleFact struct {
	ID         string        `json:"id"`
	Text       string        `json:"text"`
	OccurredAt string        `json:"occurred_at"`
	Kind       string        `json:"kind"`
	Required   bool          `json:"required"`
	Lineage    oracleLineage `json:"lineage"`
}
type oracleCase struct {
	CaseID             string        `json:"case_id"`
	DocumentID         string        `json:"document_id"`
	HistoricalExpected string        `json:"historical_expected"`
	Assertion          string        `json:"assertion"`
	CitedFactIDs       []string      `json:"cited_fact_ids"`
	CitedFacts         []oracleFact  `json:"cited_facts"`
	ContextFacts       []oracleFact  `json:"context_facts"`
	LabelLineage       oracleLineage `json:"label_lineage"`
	AssertionLineage   oracleLineage `json:"assertion_lineage"`
}
type oracleCanonicalPacket struct {
	Version                string       `json:"version"`
	CriterionID            string       `json:"criterion_id"`
	CriterionSHA256        string       `json:"criterion_sha256"`
	CorpusDigest           string       `json:"corpus_digest"`
	GoalScopeDigest        string       `json:"goal_scope_digest"`
	BaseReviewScopeDigest  string       `json:"base_review_scope_digest"`
	ReviewScopeDigest      string       `json:"review_scope_digest"`
	RegressionSourceDigest string       `json:"regression_source_digest"`
	Rows                   []oracleCase `json:"rows"`
}
type oracleBinding struct {
	CaseID     string      `json:"case_id"`
	DocumentID string      `json:"document_id"`
	CaseDigest string      `json:"case_digest"`
	Content    *oracleCase `json:"content,omitempty"`
}
type oraclePacket struct {
	Version                    string          `json:"version"`
	PacketDigest               string          `json:"packet_digest"`
	StudyDigest                string          `json:"study_digest"`
	CorpusDigest               string          `json:"corpus_digest"`
	GoalScopeDigest            string          `json:"goal_scope_digest"`
	BaseReviewScopeDigest      string          `json:"base_review_scope_digest"`
	ReviewScopeDigest          string          `json:"review_scope_digest"`
	RegressionSourceDigest     string          `json:"regression_source_digest"`
	CriterionID                string          `json:"criterion_id"`
	CriterionSHA256            string          `json:"criterion_sha256"`
	Planned                    int             `json:"planned"`
	Rows                       []oracleBinding `json:"rows"`
	HumanIdentityAuthenticated bool            `json:"human_identity_authenticated"`
	SourceAuthenticated        bool            `json:"source_authenticated"`
	ModelSiteAdoption          bool            `json:"model_site_adoption"`
	IndependentAcceptance      bool            `json:"independent_acceptance"`
	PublicationAuthority       bool            `json:"publication_authority"`
	NewStudyAdmission          bool            `json:"new_study_admission"`
	CriterionText              string          `json:"criterion_text,omitempty"`
}
type oracleAssessment struct {
	CaseID          string   `json:"case_id"`
	CaseDigest      string   `json:"case_digest"`
	Disposition     string   `json:"disposition"`
	Interpretation  string   `json:"interpretation"`
	ReasonCode      string   `json:"reason_code"`
	PremiseFactIDs  []string `json:"premise_fact_ids"`
	CounterFactIDs  []string `json:"counter_fact_ids"`
	Basis           string   `json:"basis"`
	AssessorKind    string   `json:"assessor_kind"`
	AssessorRef     string   `json:"assessor_ref"`
	ProposedFactIDs []string `json:"proposed_fact_ids"`
}
type oracleAssessments struct {
	Version         string             `json:"version"`
	PacketDigest    string             `json:"packet_digest"`
	CriterionID     string             `json:"criterion_id"`
	CriterionSHA256 string             `json:"criterion_sha256"`
	Rows            []oracleAssessment `json:"rows"`
}
type oracleReportContent struct {
	Case       oracleCase        `json:"case"`
	Assessment *oracleAssessment `json:"assessment,omitempty"`
}
type oracleReportRow struct {
	CaseID     string               `json:"case_id"`
	DocumentID string               `json:"document_id"`
	CaseDigest string               `json:"case_digest"`
	State      string               `json:"state"`
	Content    *oracleReportContent `json:"content,omitempty"`
}
type oracleReport struct {
	Version                     string            `json:"version"`
	PacketDigest                string            `json:"packet_digest"`
	StudyDigest                 string            `json:"study_digest"`
	AssessmentsDigest           string            `json:"assessments_digest"`
	CorpusDigest                string            `json:"corpus_digest"`
	GoalScopeDigest             string            `json:"goal_scope_digest"`
	BaseReviewScopeDigest       string            `json:"base_review_scope_digest"`
	ReviewScopeDigest           string            `json:"review_scope_digest"`
	RegressionSourceDigest      string            `json:"regression_source_digest"`
	CriterionID                 string            `json:"criterion_id"`
	CriterionSHA256             string            `json:"criterion_sha256"`
	Planned                     int               `json:"planned"`
	Assessed                    int               `json:"assessed"`
	Unreviewed                  int               `json:"unreviewed"`
	Affirmed                    int               `json:"affirmed"`
	Ambiguous                   int               `json:"ambiguous"`
	Revise                      int               `json:"revise"`
	Rows                        []oracleReportRow `json:"rows"`
	DeclaredAssessmentsComplete bool              `json:"declared_assessments_complete"`
	FloorDeclaredReady          bool              `json:"floor_declared_ready"`
	HumanIdentityAuthenticated  bool              `json:"human_identity_authenticated"`
	SourceAuthenticated         bool              `json:"source_authenticated"`
	ModelSiteAdoption           bool              `json:"model_site_adoption"`
	IndependentAcceptance       bool              `json:"independent_acceptance"`
	PublicationAuthority        bool              `json:"publication_authority"`
	NewStudyAdmission           bool              `json:"new_study_admission"`
	CriterionText               string            `json:"criterion_text,omitempty"`
}

func oracleBuild(in pairedInputs, corpusPath, basePath, scopePath, sourcePath string, include bool) (oraclePacket, map[string]oracleCase, error) {
	var out oraclePacket
	criterion, err := oracleCriterionFS.ReadFile("oracle-criteria/cited-entailment-v1.txt")
	if err != nil || hashBytes(criterion) != oracleCriterionPin || !utf8.Valid(criterion) || len(criterion) == 0 || criterion[len(criterion)-1] != '\n' {
		return out, nil, errors.New("criterion mismatch")
	}
	baseRaw, err := readLabBytes(basePath, 1<<20)
	if err != nil || hashBytes(baseRaw) != in.scope.BaseScopeDigest {
		return out, nil, errors.New("base scope mismatch")
	}
	scopeRaw, err := readLabBytes(scopePath, 1<<20)
	if err != nil || hashBytes(scopeRaw) != in.reviewDigest {
		return out, nil, errors.New("scope mismatch")
	}
	sourceRaw, err := readLabBytes(sourcePath, 1<<20)
	if err != nil || hashBytes(sourceRaw) != pairedRegressionSourcePin {
		return out, nil, errors.New("regression mismatch")
	}
	var base reviewScope
	if decodeLabJSON(baseRaw, &base) != nil {
		return out, nil, errors.New("base scope invalid")
	}
	positive := map[string]int{}
	negative := map[string]int{}
	extension := map[string]int{}
	for i, x := range base.PositiveControls {
		positive[x.ID] = i
	}
	for i, id := range base.NegativeCaseIDs {
		negative[id] = i
	}
	for i, x := range in.scope.Extensions {
		extension[x.ID] = i
	}
	fixtureRaw := map[string]string{}
	negativeIndex := map[string]int{}
	factIndex := map[string]map[string]int{}
	for doc, d := range in.corpus.Fixtures {
		raw, e := readLabBytes(filepath.Join(corpusPath, doc+".json"), 8<<20)
		if e != nil {
			return out, nil, e
		}
		var reread documentFixture
		if decodeLabJSON(raw, &reread) != nil || !reflect.DeepEqual(reread, d) {
			return out, nil, errors.New("fixture changed during packet build")
		}
		fixtureRaw[doc] = hashBytes(raw)
		factIndex[doc] = map[string]int{}
		for i, f := range d.Facts {
			factIndex[doc][f.ID] = i
		}
		for i, n := range d.NegativeCases {
			negativeIndex[doc+"--"+n.ID] = i
		}
	}
	fresh, err := loadDocumentCorpus(corpusPath)
	if err != nil || fresh.Digest != in.corpus.Digest {
		return out, nil, errors.New("corpus changed during packet build")
	}
	ids := make([]string, 0, len(in.cases))
	for id := range in.cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	canonical := oracleCanonicalPacket{Version: "prose-lab.oracle-canonical-packet/1", CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, BaseReviewScopeDigest: in.scope.BaseScopeDigest, ReviewScopeDigest: in.reviewDigest, RegressionSourceDigest: pairedRegressionSourcePin, Rows: make([]oracleCase, 0, 70)}
	byID := map[string]oracleCase{}
	bindings := make([]oracleBinding, 0, 70)
	for _, id := range ids {
		c := in.cases[id]
		doc := in.corpus.Fixtures[c.DocumentID]
		rawDigest := fixtureRaw[c.DocumentID]
		row := oracleCase{CaseID: id, DocumentID: c.DocumentID, HistoricalExpected: c.Expected, Assertion: c.Assertion, CitedFactIDs: append([]string(nil), c.FactIDs...), CitedFacts: []oracleFact{}, ContextFacts: []oracleFact{}}
		if i, ok := positive[id]; ok {
			row.LabelLineage = oracleLineage{ArtifactKind: "base-review-scope", ArtifactDigest: in.scope.BaseScopeDigest, JSONPointer: "/positive_controls/" + itoa(i), MappingRule: "positive-control-means-supported/1"}
			row.AssertionLineage = oracleLineage{ArtifactKind: "base-review-scope", ArtifactDigest: in.scope.BaseScopeDigest, JSONPointer: "/positive_controls/" + itoa(i) + "/assertion"}
		} else if i, ok := negative[id]; ok {
			j, found := negativeIndex[id]
			if !found {
				return out, nil, errors.New("negative lineage missing")
			}
			row.LabelLineage = oracleLineage{ArtifactKind: "base-review-scope", ArtifactDigest: in.scope.BaseScopeDigest, JSONPointer: "/negative_case_ids/" + itoa(i), MappingRule: "negative-control-means-unsupported/1", TargetFactID: doc.NegativeCases[j].TargetFactID, FixtureExpectedValue: doc.NegativeCases[j].Expected, FixtureExpected: &oracleLineage{ArtifactKind: "corpus-fixture", ArtifactDigest: rawDigest, JSONPointer: "/negative_cases/" + itoa(j) + "/expected"}}
			row.AssertionLineage = oracleLineage{ArtifactKind: "corpus-fixture", ArtifactDigest: rawDigest, JSONPointer: "/negative_cases/" + itoa(j) + "/mutated_assertion"}
		} else if i, ok := extension[id]; ok {
			x := in.scope.Extensions[i]
			row.LabelLineage = oracleLineage{ArtifactKind: "paired-review-scope", ArtifactDigest: in.reviewDigest, JSONPointer: "/extensions/" + itoa(i) + "/expected", SourceDigest: x.Provenance.SourceDigest, Method: x.Provenance.Method, Reason: x.Provenance.Reason}
			row.AssertionLineage = oracleLineage{ArtifactKind: "paired-review-scope", ArtifactDigest: in.reviewDigest, JSONPointer: "/extensions/" + itoa(i) + "/assertion"}
			if x.Kind == "observed-development" {
				row.AssertionLineage.RegressionSource = &oracleLineage{ArtifactKind: "observed-regression-source", ArtifactDigest: pairedRegressionSourcePin, JSONPointer: oracleRegressionPointer(sourceRaw, id)}
			}
		} else {
			return out, nil, errors.New("case lineage missing")
		}
		for _, f := range doc.Facts {
			of := oracleFact{ID: f.ID, Text: f.Text, OccurredAt: f.OccurredAt, Kind: f.Kind, Required: f.Required, Lineage: oracleLineage{ArtifactKind: "corpus-fixture", ArtifactDigest: rawDigest, JSONPointer: "/facts/" + itoa(factIndex[c.DocumentID][f.ID])}}
			row.ContextFacts = append(row.ContextFacts, of)
			for _, cid := range c.FactIDs {
				if cid == f.ID {
					row.CitedFacts = append(row.CitedFacts, of)
					break
				}
			}
		}
		sort.Slice(row.ContextFacts, func(i, j int) bool { return row.ContextFacts[i].ID < row.ContextFacts[j].ID })
		sort.Slice(row.CitedFacts, func(i, j int) bool { return row.CitedFacts[i].ID < row.CitedFacts[j].ID })
		canonical.Rows = append(canonical.Rows, row)
		byID[id] = row
		bindings = append(bindings, oracleBinding{CaseID: id, DocumentID: c.DocumentID, CaseDigest: hashBytes(jsonBytes(row))})
	}
	if len(bindings) != 70 {
		return out, nil, errors.New("roster count")
	}
	out = oraclePacket{Version: "prose-lab.oracle-packet/1", PacketDigest: hashBytes(jsonBytes(canonical)), StudyDigest: in.studyDigest, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, BaseReviewScopeDigest: in.scope.BaseScopeDigest, ReviewScopeDigest: in.reviewDigest, RegressionSourceDigest: pairedRegressionSourcePin, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Planned: 70, Rows: bindings}
	if include {
		out.CriterionText = string(criterion)
		for i := range out.Rows {
			row := byID[out.Rows[i].CaseID]
			out.Rows[i].Content = &row
		}
	}
	return out, byID, nil
}

func oracleRegressionPointer(raw []byte, id string) string {
	var source pairedRegressionSource
	if decodeLabJSON(raw, &source) != nil {
		return ""
	}
	for i, c := range source.Cases {
		if id == "technical-design--"+c.ID {
			return "/cases/" + itoa(i)
		}
	}
	return ""
}
func itoa(i int) string { return strconv.Itoa(i) }

func runReviewOracle(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return diagnostic(errOut, "oracle_command_required", 2)
	}
	cmd := args[0]
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h" || args[1] == "help") {
		usage := "Usage: prose-lab " + cmd + " " + oracleCommonUsage
		if cmd == "review-oracle-report" {
			usage += " --packet-digest SHA256 --assessments PATH"
		}
		return pairedWrite(out, []byte(usage+" [--include-content]\n"), 0)
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
	assessmentsPath := fs.String("assessments", "", "")
	content := fs.Bool("include-content", false, "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return diagnostic(errOut, "invalid_oracle_flags", 2)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, key := range []string{"study", "corpus", "goal-scope", "goal-scope-digest", "base-review-scope", "review-scope", "review-scope-digest", "regression-source"} {
		if !set[key] {
			return diagnostic(errOut, "oracle_input_required", 2)
		}
	}
	if cmd == "review-oracle-prepare" && (set["packet-digest"] || set["assessments"]) {
		return diagnostic(errOut, "invalid_oracle_prepare_flags", 2)
	}
	if cmd == "review-oracle-report" && (!set["packet-digest"] || !set["assessments"] || !documentDigestPattern.MatchString(*packetPin) || *assessmentsPath == "") {
		return diagnostic(errOut, "invalid_oracle_report_flags", 2)
	}
	if ctx.Err() != nil {
		return diagnostic(errOut, "oracle_cancelled", 2)
	}
	in, e := loadPairedInputs(*study, *corpus, *goal, *goalPin, *base, *scope, *scopePin, *source)
	if e != nil {
		return diagnostic(errOut, "oracle_inputs_refused", 2)
	}
	packet, byID, e := oracleBuild(in, *corpus, *base, *scope, *source, *content)
	if e != nil {
		return diagnostic(errOut, "oracle_packet_refused", 2)
	}
	if ctx.Err() != nil {
		return diagnostic(errOut, "oracle_cancelled", 2)
	}
	if cmd == "review-oracle-prepare" {
		return oracleWrite(ctx, out, packet, 0)
	}
	if packet.PacketDigest != *packetPin {
		return diagnostic(errOut, "oracle_packet_mismatch", 2)
	}
	raw, e := readLabBytes(*assessmentsPath, 1<<20)
	if e != nil {
		return diagnostic(errOut, "oracle_assessments_refused", 2)
	}
	assessments, e := oracleReadAssessments(raw, packet, byID)
	if e != nil {
		return diagnostic(errOut, "oracle_assessments_refused", 2)
	}
	report := oracleMakeReport(packet, raw, assessments)
	code := 2
	if report.Ambiguous+report.Revise > 0 {
		code = 1
	} else if report.FloorDeclaredReady {
		code = 0
	}
	return oracleWrite(ctx, out, report, code)
}
func oracleWrite(ctx context.Context, out io.Writer, v any, code int) int {
	if ctx.Err() != nil {
		return 2
	}
	b, e := json.Marshal(v)
	if e != nil || len(b)+1 > 8<<20 || ctx.Err() != nil {
		return 2
	}
	return pairedWrite(out, append(b, '\n'), code)
}
func oracleReadAssessments(raw []byte, packet oraclePacket, byID map[string]oracleCase) (map[string]oracleAssessment, error) {
	bad := errors.New("invalid oracle assessments")
	if exactObject(raw, "version", "packet_digest", "criterion_id", "criterion_sha256", "rows") != nil {
		return nil, bad
	}
	obj, e := jsonwire.Object(raw)
	if e != nil || exactArrayObjects(obj["rows"], "case_id", "case_digest", "disposition", "interpretation", "reason_code", "premise_fact_ids", "counter_fact_ids", "basis", "assessor_kind", "assessor_ref", "proposed_fact_ids") != nil {
		return nil, bad
	}
	var a oracleAssessments
	if decodeLabJSON(raw, &a) != nil || a.Version != "prose-lab.oracle-assessments/1" || a.PacketDigest != packet.PacketDigest || a.CriterionID != oracleCriterionID || a.CriterionSHA256 != oracleCriterionPin || len(a.Rows) > 70 {
		return nil, bad
	}
	binding := map[string]oracleBinding{}
	for _, b := range packet.Rows {
		binding[b.CaseID] = b
	}
	found := map[string]oracleAssessment{}
	for _, x := range a.Rows {
		b, ok := binding[x.CaseID]
		if !ok || found[x.CaseID].CaseID != "" || x.CaseDigest != b.CaseDigest || !validPairedText(x.Basis, 4096) || !validPairedText(x.AssessorRef, 256) || (x.AssessorKind != "human" && x.AssessorKind != "agent") {
			return nil, bad
		}
		row := byID[x.CaseID]
		cited := map[string]bool{}
		facts := map[string]bool{}
		for _, f := range row.ContextFacts {
			facts[f.ID] = true
		}
		for _, id := range row.CitedFactIDs {
			cited[id] = true
		}
		if !oracleIDs(x.PremiseFactIDs, facts) || !oracleIDs(x.CounterFactIDs, facts) || !oracleIDs(x.ProposedFactIDs, facts) || len(x.PremiseFactIDs)+len(x.CounterFactIDs) == 0 || (x.ReasonCode == "contradiction" && len(x.CounterFactIDs) == 0) {
			return nil, bad
		}
		for _, id := range x.PremiseFactIDs {
			if !cited[id] {
				return nil, bad
			}
		}
		resolved := x.Interpretation == "literal" || x.Interpretation == "faithful-paraphrase" || x.Interpretation == "normative-direction"
		switch x.Disposition {
		case "affirmed":
			if !resolved || len(x.ProposedFactIDs) != 0 {
				return nil, bad
			}
			if row.HistoricalExpected == "supported" {
				if x.ReasonCode != "sufficient-premises" || len(x.PremiseFactIDs) == 0 {
					return nil, bad
				}
			} else if x.ReasonCode != "contradiction" && x.ReasonCode != "citation-mismatch" && x.ReasonCode != "missing-premise" {
				return nil, bad
			} else if len(x.PremiseFactIDs)+len(x.CounterFactIDs) == 0 {
				return nil, bad
			}
		case "ambiguous":
			if x.Interpretation != "unresolved" || (x.ReasonCode != "scope-ambiguity" && x.ReasonCode != "modality-ambiguity" && x.ReasonCode != "missing-premise") {
				return nil, bad
			}
		case "revise":
			if x.ReasonCode != "contradiction" && x.ReasonCode != "citation-mismatch" && x.ReasonCode != "missing-premise" {
				return nil, bad
			}
			if x.Interpretation != "unresolved" && !resolved {
				return nil, bad
			}
			if x.ReasonCode == "citation-mismatch" && len(x.ProposedFactIDs) == 0 {
				return nil, bad
			}
			if len(x.PremiseFactIDs)+len(x.CounterFactIDs)+len(x.ProposedFactIDs) == 0 {
				return nil, bad
			}
		default:
			return nil, bad
		}
		found[x.CaseID] = x
	}
	return found, nil
}
func oracleIDs(ids []string, allowed map[string]bool) bool {
	if ids == nil || len(ids) > len(allowed) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validLabID(id) || !allowed[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func oracleMakeReport(packet oraclePacket, raw []byte, assessments map[string]oracleAssessment) oracleReport {
	r := oracleReport{CriterionText: packet.CriterionText, Version: "prose-lab.oracle-report/1", PacketDigest: packet.PacketDigest, StudyDigest: packet.StudyDigest, AssessmentsDigest: hashBytes(raw), CorpusDigest: packet.CorpusDigest, GoalScopeDigest: packet.GoalScopeDigest, BaseReviewScopeDigest: packet.BaseReviewScopeDigest, ReviewScopeDigest: packet.ReviewScopeDigest, RegressionSourceDigest: packet.RegressionSourceDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Planned: 70, Rows: make([]oracleReportRow, 0, 70)}
	for _, b := range packet.Rows {
		row := oracleReportRow{CaseID: b.CaseID, DocumentID: b.DocumentID, CaseDigest: b.CaseDigest, State: "unreviewed"}
		if x, ok := assessments[b.CaseID]; ok {
			row.State = x.Disposition
			r.Assessed++
			switch x.Disposition {
			case "affirmed":
				r.Affirmed++
			case "ambiguous":
				r.Ambiguous++
			case "revise":
				r.Revise++
			}
			if b.Content != nil {
				xc := x
				row.Content = &oracleReportContent{Case: *b.Content, Assessment: &xc}
			}
		} else {
			r.Unreviewed++
			if b.Content != nil {
				row.Content = &oracleReportContent{Case: *b.Content}
			}
		}
		r.Rows = append(r.Rows, row)
	}
	r.DeclaredAssessmentsComplete = r.Assessed == 70
	r.FloorDeclaredReady = r.DeclaredAssessmentsComplete && r.Affirmed == 70
	return r
}

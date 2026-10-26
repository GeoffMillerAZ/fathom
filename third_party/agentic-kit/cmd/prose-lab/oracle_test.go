package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func oracleFixture(t *testing.T) (oraclePacket, []string) {
	t.Helper()
	_, _, base := testPairedInputs(t)
	var out, errOut bytes.Buffer
	args := append([]string{"review-oracle-prepare"}, base...)
	if code := runLab(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("prepare %d %s", code, errOut.String())
	}
	var p oraclePacket
	if json.Unmarshal(out.Bytes(), &p) != nil {
		t.Fatal("packet JSON")
	}
	if p.Planned != 70 || len(p.Rows) != 70 || p.PacketDigest == "" {
		t.Fatal("packet roster")
	}
	vetPairedOutput(t, "#OraclePacket", out.Bytes())
	return p, base
}
func oracleAssessmentFor(p oraclePacket, b oracleBinding) oracleAssessment {
	return oracleAssessment{CaseID: b.CaseID, CaseDigest: b.CaseDigest, Disposition: "ambiguous", Interpretation: "unresolved", ReasonCode: "missing-premise", PremiseFactIDs: []string{}, CounterFactIDs: []string{b.Content.CitedFactIDs[0]}, Basis: "The cited material does not resolve this interpretation.", AssessorKind: "agent", AssessorRef: "declared-reviewer", ProposedFactIDs: []string{}}
}
func oracleReportCall(t *testing.T, p oraclePacket, base []string, a oracleAssessments) (int, oracleReport) {
	t.Helper()
	raw := jsonBytes(a)
	file := filepath.Join(t.TempDir(), "assessments.json")
	if e := os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	args := append(append([]string{"review-oracle-report"}, base...), "--packet-digest", p.PacketDigest, "--assessments", file)
	var out, errOut bytes.Buffer
	code := runLab(context.Background(), args, &out, &errOut)
	var r oracleReport
	if out.Len() > 0 {
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		vetPairedOutput(t, "#OracleReport", out.Bytes())
	}
	return code, r
}
func TestOracleEntrypointsAndCoverage(t *testing.T) {
	p, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	args := append(append([]string{"review-oracle-prepare"}, base...), "--include-content")
	if c := runLab(context.Background(), args, &out, &errOut); c != 0 {
		t.Fatalf("content %d %s", c, errOut.String())
	}
	var full oraclePacket
	if json.Unmarshal(out.Bytes(), &full) != nil || full.PacketDigest != p.PacketDigest || full.Rows[0].Content == nil {
		t.Fatal("projection binding")
	}
	vetPairedOutput(t, "#OraclePacket", out.Bytes())
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{oracleAssessmentFor(p, full.Rows[0])}}
	code, r := oracleReportCall(t, p, base, a)
	if code != 1 || r.Planned != 70 || r.Assessed != 1 || r.Unreviewed != 69 || r.Ambiguous != 1 || r.FloorDeclaredReady {
		t.Fatalf("partial ambiguity %d %+v", code, r)
	}
	a.Rows = []oracleAssessment{}
	code, r = oracleReportCall(t, p, base, a)
	if code != 2 || r.Unreviewed != 70 {
		t.Fatalf("empty %d %+v", code, r)
	}
	a.Rows = []oracleAssessment{oracleAssessmentFor(p, full.Rows[0]), oracleAssessmentFor(p, full.Rows[0])}
	code, _ = oracleReportCall(t, p, base, a)
	if code != 2 {
		t.Fatal("duplicate accepted")
	}
	a.Rows = []oracleAssessment{oracleAssessmentFor(p, full.Rows[0])}
	a.Rows[0].CounterFactIDs = []string{"foreign"}
	code, _ = oracleReportCall(t, p, base, a)
	if code != 2 {
		t.Fatal("foreign fact accepted")
	}
}
func TestOracleHelpFlagsAndCancellation(t *testing.T) {
	_, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), []string{"review-oracle-prepare", "--help"}, &out, &errOut); c != 0 || out.Len() == 0 {
		t.Fatal("help")
	}
	for _, flag := range []string{"--live=false", "--case=", "--variant=", "--journal=", "--artifacts=", "--prepared-digest=", "--criterion="} {
		out.Reset()
		errOut.Reset()
		args := append(append([]string{"review-oracle-prepare"}, base...), flag)
		if c := runLab(context.Background(), args, &out, &errOut); c != 2 || out.Len() != 0 {
			t.Fatalf("flag %s code %d", flag, c)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if c := runLab(ctx, append([]string{"review-oracle-prepare"}, base...), &out, &errOut); c != 2 || out.Len() != 0 {
		t.Fatal("cancellation")
	}
}

type oracleShortWriter struct{}

func (oracleShortWriter) Write(p []byte) (int, error) { return 0, nil }

func TestOracleCompleteStructuralAndRefusals(t *testing.T) {
	p, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), append(append([]string{"review-oracle-prepare"}, base...), "--include-content"), &out, &errOut); c != 0 {
		t.Fatal(c)
	}
	var full oraclePacket
	if json.Unmarshal(out.Bytes(), &full) != nil {
		t.Fatal("content")
	}
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: make([]oracleAssessment, 0, 70)}
	for _, b := range full.Rows {
		row := oracleAssessmentFor(p, b)
		row.Disposition = "affirmed"
		row.Interpretation = "literal"
		row.CounterFactIDs = []string{}
		row.PremiseFactIDs = []string{b.Content.CitedFactIDs[0]}
		if b.Content.HistoricalExpected == "supported" {
			row.ReasonCode = "sufficient-premises"
		} else {
			row.ReasonCode = "missing-premise"
		}
		a.Rows = append(a.Rows, row)
	}
	code, r := oracleReportCall(t, p, base, a)
	if code != 0 || !r.FloorDeclaredReady || !r.DeclaredAssessmentsComplete || r.Affirmed != 70 || r.Unreviewed != 0 {
		t.Fatalf("complete code=%d %+v", code, r)
	}
	// A declaration can be structurally complete, but this test does not adjudicate semantic truth.
	a.Rows[0].Disposition = "revise"
	a.Rows[0].ReasonCode = "citation-mismatch"
	a.Rows[0].ProposedFactIDs = []string{full.Rows[0].Content.ContextFacts[0].ID}
	code, r = oracleReportCall(t, p, base, a)
	if code != 1 || r.Revise != 1 || r.FloorDeclaredReady {
		t.Fatal("revision not held")
	}
	a.Rows[0].ProposedFactIDs = []string{}
	code, _ = oracleReportCall(t, p, base, a)
	if code != 2 {
		t.Fatal("revision without proposed citation accepted")
	}
	a.Rows[0].Disposition = "affirmed"
	a.Rows[0].ReasonCode = "sufficient-premises"
	a.Rows[0].ProposedFactIDs = []string{}
	var uncited string
	cited := map[string]bool{}
	for _, id := range full.Rows[0].Content.CitedFactIDs {
		cited[id] = true
	}
	for _, f := range full.Rows[0].Content.ContextFacts {
		if !cited[f.ID] {
			uncited = f.ID
			break
		}
	}
	if full.Rows[0].Content.HistoricalExpected == "supported" && uncited != "" {
		a.Rows[0].PremiseFactIDs = []string{uncited}
		code, _ = oracleReportCall(t, p, base, a)
		if code != 2 {
			t.Fatal("uncited positive premise accepted")
		}
	}
}
func TestOracleIOPrivacyAndBounds(t *testing.T) {
	p, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	args := append([]string{"review-oracle-prepare"}, base...)
	if c := runLab(context.Background(), args, oracleShortWriter{}, &errOut); c != 2 {
		t.Fatal("short writer")
	}
	for _, b := range []string{"assertion", "historical_expected", "criterion_text", "basis", "assessor_ref", "fact_ids"} {
		if bytes.Contains(jsonBytes(p), []byte(b)) {
			t.Fatalf("default leaked %s", b)
		}
	}
	if c := runLab(context.Background(), args, &out, &errOut); c != 0 {
		t.Fatal("prepare")
	}
	raw := jsonBytes(oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{}})
	dir := t.TempDir()
	f := filepath.Join(dir, "assessment.json")
	if e := os.WriteFile(f, raw, 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(dir, "assessment-link.json")
	if e := os.Symlink(f, link); e != nil {
		t.Fatal(e)
	}
	report := append(append([]string{"review-oracle-report"}, base...), "--packet-digest", p.PacketDigest, "--assessments", link)
	out.Reset()
	if c := runLab(context.Background(), report, &out, &errOut); c != 2 || out.Len() != 0 {
		t.Fatal("symlink")
	}
	if e := os.WriteFile(f, bytes.Repeat([]byte("x"), 1<<20+1), 0600); e != nil {
		t.Fatal(e)
	}
	report[len(report)-1] = f
	out.Reset()
	if c := runLab(context.Background(), report, &out, &errOut); c != 2 || out.Len() != 0 {
		t.Fatal("bound")
	}
}

func TestOracleAssessmentCUEParityAndUTF8(t *testing.T) {
	p, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), append(append([]string{"review-oracle-prepare"}, base...), "--include-content"), &out, &errOut); c != 0 {
		t.Fatal(c)
	}
	var full oraclePacket
	if json.Unmarshal(out.Bytes(), &full) != nil {
		t.Fatal("packet")
	}
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{oracleAssessmentFor(p, full.Rows[0])}}
	vetPairedOutput(t, "#OracleAssessments", jsonBytes(a))
	checks := []struct {
		name string
		edit func(*oracleAssessments)
	}{
		{"bad disposition", func(v *oracleAssessments) { v.Rows[0].Disposition = "unknown" }},
		{"missing basis", func(v *oracleAssessments) { v.Rows[0].Basis = "" }},
		{"assessor bytes", func(v *oracleAssessments) { v.Rows[0].AssessorRef = string(bytes.Repeat([]byte("é"), 129)) }},
		{"basis bytes", func(v *oracleAssessments) { v.Rows[0].Basis = string(bytes.Repeat([]byte("é"), 2049)) }},
		{"contradiction no counter", func(v *oracleAssessments) {
			v.Rows[0].Disposition = "revise"
			v.Rows[0].ReasonCode = "contradiction"
			v.Rows[0].CounterFactIDs = []string{}
			v.Rows[0].PremiseFactIDs = []string{full.Rows[0].Content.CitedFactIDs[0]}
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			v := a
			v.Rows = append([]oracleAssessment(nil), a.Rows...)
			tc.edit(&v)
			rejectPairedCUE(t, "#OracleAssessments", jsonBytes(v))
			if c, _ := oracleReportCall(t, p, base, v); c != 2 {
				t.Fatal("Go accepted invalid assessment")
			}
		})
	}
	// Invalid UTF-8 is refused by the strict JSON decoder before coverage is graded.
	raw := []byte(`{"version":"prose-lab.oracle-assessments/1","packet_digest":"` + p.PacketDigest + `","criterion_id":"cited-entailment/1","criterion_sha256":"` + oracleCriterionPin + `","rows":[]}`)
	raw = append(raw, 0xff)
	f := filepath.Join(t.TempDir(), "bad-utf8.json")
	if e := os.WriteFile(f, raw, 0600); e != nil {
		t.Fatal(e)
	}
	args := append(append([]string{"review-oracle-report"}, base...), "--packet-digest", p.PacketDigest, "--assessments", f)
	out.Reset()
	if c := runLab(context.Background(), args, &out, &errOut); c != 2 || out.Len() != 0 {
		t.Fatal("invalid UTF-8")
	}
}

func TestOracleSemanticBindingSurvivesStudyChange(t *testing.T) {
	p, base := oracleFixture(t)
	studyFile := base[1]
	raw, e := os.ReadFile(studyFile)
	if e != nil {
		t.Fatal(e)
	}
	var study pairedStudy
	if json.Unmarshal(raw, &study) != nil {
		t.Fatal("study")
	}
	study.Review.Model = "another-synthetic-model"
	if e := os.WriteFile(studyFile, jsonBytes(study), 0600); e != nil {
		t.Fatal(e)
	}
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), append([]string{"review-oracle-prepare"}, base...), &out, &errOut); c != 0 {
		t.Fatalf("changed study %d %s", c, errOut.String())
	}
	var changed oraclePacket
	if json.Unmarshal(out.Bytes(), &changed) != nil {
		t.Fatal("packet")
	}
	if changed.PacketDigest != p.PacketDigest || changed.StudyDigest == p.StudyDigest {
		t.Fatal("semantic identity includes study settings")
	}
	// A changed source pin cannot reuse a packet even if a caller supplies the old digest.
	altered := append([]string(nil), base...)
	for i := range altered {
		if altered[i] == "--review-scope-digest" {
			altered[i+1] = hashBytes([]byte("different scope"))
			break
		}
	}
	out.Reset()
	errOut.Reset()
	if c := runLab(context.Background(), append([]string{"review-oracle-prepare"}, altered...), &out, &errOut); c != 2 || out.Len() != 0 {
		t.Fatal("changed scope pin accepted")
	}
}

func TestOracleReportCUECounterRefusal(t *testing.T) {
	p, base := oracleFixture(t)
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{}}
	code, r := oracleReportCall(t, p, base, a)
	if code != 2 || len(r.Rows) != 70 {
		t.Fatal("report")
	}
	r.Unreviewed = 69
	rejectPairedCUE(t, "#OracleReport", jsonBytes(r))
	r.Unreviewed = 70
	r.PublicationAuthority = true
	rejectPairedCUE(t, "#OracleReport", jsonBytes(r))
	r.PublicationAuthority = false
	r.Rows[0].State = "affirmed"
	rejectPairedCUE(t, "#OracleReport", jsonBytes(r))
}

func TestOracleReportContentProjection(t *testing.T) {
	p, base := oracleFixture(t)
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{}}
	file := filepath.Join(t.TempDir(), "assessments.json")
	if e := os.WriteFile(file, jsonBytes(a), 0600); e != nil {
		t.Fatal(e)
	}
	args := append(append([]string{"review-oracle-report"}, base...), "--packet-digest", p.PacketDigest, "--assessments", file, "--include-content")
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), args, &out, &errOut); c != 2 || out.Len() == 0 {
		t.Fatalf("report %d %s", c, errOut.String())
	}
	var report oracleReport
	if e := json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if report.CriterionText == "" || report.Rows[0].Content == nil || report.Rows[0].Content.Assessment != nil || report.Rows[0].Content.Case.CaseID != report.Rows[0].CaseID {
		t.Fatal("content projection")
	}
	vetPairedOutput(t, "#OracleReport", out.Bytes())
}

func TestOracleCUEForgedRosterAndContent(t *testing.T) {
	p, base := oracleFixture(t)
	var out, errOut bytes.Buffer
	if c := runLab(context.Background(), append(append([]string{"review-oracle-prepare"}, base...), "--include-content"), &out, &errOut); c != 0 {
		t.Fatal(c)
	}
	var full oraclePacket
	if json.Unmarshal(out.Bytes(), &full) != nil {
		t.Fatal("packet")
	}
	duplicatePacket := full
	duplicatePacket.Rows = append([]oracleBinding(nil), full.Rows...)
	duplicatePacket.Rows[69] = full.Rows[0]
	rejectPairedCUE(t, "#OraclePacket", jsonBytes(duplicatePacket))
	swapped := full
	swapped.Rows = append([]oracleBinding(nil), full.Rows...)
	swapped.Rows[0].Content = full.Rows[1].Content
	rejectPairedCUE(t, "#OraclePacket", jsonBytes(swapped))
	a := oracleAssessments{Version: "prose-lab.oracle-assessments/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, Rows: []oracleAssessment{oracleAssessmentFor(p, full.Rows[0])}}
	dupA := a
	dupA.Rows = append([]oracleAssessment(nil), a.Rows...)
	changed := a.Rows[0]
	changed.Basis = "different but same case ID"
	dupA.Rows = append(dupA.Rows, changed)
	rejectPairedCUE(t, "#OracleAssessments", jsonBytes(dupA))
	file := filepath.Join(t.TempDir(), "a.json")
	if e := os.WriteFile(file, jsonBytes(a), 0600); e != nil {
		t.Fatal(e)
	}
	args := append(append([]string{"review-oracle-report"}, base...), "--packet-digest", p.PacketDigest, "--assessments", file, "--include-content")
	out.Reset()
	errOut.Reset()
	if c := runLab(context.Background(), args, &out, &errOut); c != 1 {
		t.Fatalf("report %d %s", c, errOut.String())
	}
	var report oracleReport
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatal("report JSON")
	}
	vetPairedOutput(t, "#OracleReport", out.Bytes())
	dupe := report
	dupe.Rows = append([]oracleReportRow(nil), report.Rows...)
	dupe.Rows[69] = report.Rows[0]
	dupe.Ambiguous = 2
	dupe.Unreviewed = 68
	dupe.Assessed = 2
	rejectPairedCUE(t, "#OracleReport", jsonBytes(dupe))
	allAffirmed := report
	allAffirmed.Rows = append([]oracleReportRow(nil), report.Rows...)
	for i := range allAffirmed.Rows {
		allAffirmed.Rows[i].State = "affirmed"
		allAffirmed.Rows[i].Content = nil
	}
	allAffirmed.Rows[69] = allAffirmed.Rows[0]
	allAffirmed.Assessed, allAffirmed.Affirmed, allAffirmed.Unreviewed, allAffirmed.Ambiguous = 70, 70, 0, 0
	allAffirmed.DeclaredAssessmentsComplete, allAffirmed.FloorDeclaredReady = true, true
	rejectPairedCUE(t, "#OracleReport", jsonBytes(allAffirmed))
	wrongCase := report
	wrongCase.Rows = append([]oracleReportRow(nil), report.Rows...)
	row := report.Rows[0]
	caseCopy := row.Content.Case
	caseCopy.CaseID = "forged"
	contentCopy := *row.Content
	contentCopy.Case = caseCopy
	row.Content = &contentCopy
	wrongCase.Rows[0] = row
	rejectPairedCUE(t, "#OracleReport", jsonBytes(wrongCase))
	wrongAssessment := report
	wrongAssessment.Rows = append([]oracleReportRow(nil), report.Rows...)
	row = report.Rows[0]
	contentCopy = *row.Content
	assessmentCopy := *contentCopy.Assessment
	assessmentCopy.Disposition = "revise"
	contentCopy.Assessment = &assessmentCopy
	row.Content = &contentCopy
	wrongAssessment.Rows[0] = row
	rejectPairedCUE(t, "#OracleReport", jsonBytes(wrongAssessment))
	unreviewed := report
	unreviewed.Rows = append([]oracleReportRow(nil), report.Rows...)
	row = report.Rows[1]
	contentCopy = *row.Content
	assessmentCopy = a.Rows[0]
	contentCopy.Assessment = &assessmentCopy
	row.Content = &contentCopy
	unreviewed.Rows[1] = row
	rejectPairedCUE(t, "#OracleReport", jsonBytes(unreviewed))
}

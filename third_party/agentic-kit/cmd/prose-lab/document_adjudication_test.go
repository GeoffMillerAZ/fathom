package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

func TestDocumentReviewEntrypointPinnedOffline(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	scope := "testdata/document-scopes/v2-length-goals.json"
	scopeBytes, err := os.ReadFile(scope)
	if err != nil {
		t.Fatal(err)
	}
	scopeDigest := hashBytes(scopeBytes)
	studyPath := documentTestStudy(t, corpus)
	studyBytes, err := os.ReadFile(studyPath)
	if err != nil {
		t.Fatal(err)
	}
	studyDigest := hashBytes(documentStudyBytesV2(studyBytes, corpus.Digest, scopeDigest))
	cell, _ := corpus.cell("architecture-decision-technical-r1")
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	engine, target, err := documentEngine(&documentGenerator{}, &documentSupport{}, "synthetic-generator", "synthetic-reviewer", 256, corpus.Fixtures[cell.DocumentID], cell, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	v, err := engine.Generate(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := engine.Render(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	side, status, deviation, err := documentLengthAssess(corpus.Fixtures[cell.DocumentID], v, rendered, scopeDigest, at)
	if err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(t.TempDir(), "artifacts")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	charge := int64(1)
	g := corpus.Fixtures[cell.DocumentID].LengthGoal
	receipt := documentRunReceipt{Version: "prose-lab.document-run/2", CellID: cell.ID, CorpusDigest: corpus.Digest, StudyDigest: studyDigest, PreparedDigest: documentPreparedDigestV2(studyDigest, cell.ID, "generation", "review", scopeDigest), GenerationLane: "generation", ReviewLane: "review", RequestedGenerationModel: "synthetic-generator", RequestedReviewModel: "synthetic-reviewer", TextProjection: documentTextProjection, Status: "candidate_checked", DocumentDigest: v.Receipt.DocumentDigest, MarkdownDigest: rendered.MarkdownDigest, GoalsDigest: hashBytes(jsonBytes(side)), GoalScopeDigest: scopeDigest, LengthCriterionID: g.CriterionID, LengthClass: g.Class, LengthMinWords: g.MinWords, LengthMaxWords: g.MaxWords, LengthWordCount: side.WordCount, LengthDeviation: &deviation, LengthStatus: status, CountRule: documentCountRule, SemanticQuality: "unassessed", ChargeBasis: "journal-usage-priced-or-reserved-local-accounting", CumulativeChargedMicroUSD: &charge, CellChargedMicroUSD: &charge}
	if status == "hold" {
		receipt.Status, receipt.ErrorClass = "held", "required_length_held"
	}
	write := func(name string, b []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(rootPath, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(cell.ID+".receipt.json", append(jsonBytes(receipt), '\n'))
	write(cell.ID+".version.json", append(jsonBytes(v), '\n'))
	write(cell.ID+".md", []byte(rendered.Markdown))
	write(cell.ID+".goals.json", append(jsonBytes(side), '\n'))
	receiptBytes, err := os.ReadFile(filepath.Join(rootPath, cell.ID+".receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"document-review-prepare", "--study", studyPath, "--corpus", "testdata/document-corpus-v2", "--goal-scope", scope, "--goal-scope-digest", scopeDigest, "--cell", cell.ID, "--generation-lane", "generation", "--review-lane", "review", "--artifacts", rootPath, "--receipt-digest", hashBytes(receiptBytes), "--at", at.Format(time.RFC3339Nano)}
	in, err := loadDocumentReviewInputs(studyPath, "testdata/document-corpus-v2", scope, scopeDigest, cell.ID, "generation", "review", rootPath, at)
	if err != nil {
		t.Fatalf("inputs: %v", err)
	}
	loaded, currentSide, err := loadDocumentReviewPacket(in, hashBytes(receiptBytes), false)
	if err != nil {
		t.Fatalf("packet load: %v", err)
	}
	var out, diag bytes.Buffer
	if code := runLab(context.Background(), args, &out, &diag); code != 0 {
		t.Fatalf("prepare %d: %s", code, diag.String())
	}
	var packet documentReviewPacket
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil || packet.PacketDigest == "" || packet.Content != nil {
		t.Fatalf("packet: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte(corpus.Fixtures[cell.DocumentID].Purpose)) {
		t.Fatal("private content leaked")
	}
	out.Reset()
	diag.Reset()
	withContent := append(append([]string{}, args...), "--include-content")
	if code := runLab(context.Background(), withContent, &out, &diag); code != 0 {
		t.Fatalf("content packet %d: %s", code, diag.String())
	}
	var contentPacket documentReviewPacket
	if err := json.Unmarshal(out.Bytes(), &contentPacket); err != nil || contentPacket.PacketDigest != packet.PacketDigest || contentPacket.Content == nil || len(contentPacket.Content.Claims) == 0 || contentPacket.Content.Claims[0].CoordinateBasis != "node-text-utf8-byte-range-half-open/v1" {
		t.Fatalf("content packet: %v", err)
	}
	if cueBin, err := exec.LookPath("cue"); err == nil {
		contentFile := filepath.Join(t.TempDir(), "packet.json")
		if err := os.WriteFile(contentFile, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(cueBin, "vet", "schema.cue", contentFile, "-d", "#DocumentReviewPacket", "-c").CombinedOutput(); err != nil {
			t.Fatalf("actual content packet violates CUE: %v %s", err, output)
		}
	}
	out.Reset()
	diag.Reset()
	reportArgs := append(append([]string{}, args...), "--packet-digest", packet.PacketDigest)
	reportArgs[0] = "document-review-report"
	code := runLab(context.Background(), reportArgs, &out, &diag)
	if code != 2 || !bytes.Contains(out.Bytes(), []byte(`"declared_goals_status":"unknown"`)) {
		t.Fatalf("unknown report %d: %s %s", code, out.String(), diag.String())
	}
	var a documentAdjudication
	a.Version, a.PacketDigest = "prose-lab.document-adjudication/1", loaded.PacketDigest
	a.Reviewer.ID, a.Reviewer.Kind, a.Reviewer.Relationship = "declared-reviewer", "human", "independent"
	a.Observations = []fitness.Observation{}
	for i, g := range loaded.SemanticGoals {
		a.Observations = append(a.Observations, fitness.Observation{CriterionID: g.CriterionID, DefinitionDigest: currentSide.Goals[i].Profile.Criteria[0].DefinitionDigest, SubjectDigest: loaded.SubjectDigest, Outcome: fitness.FindingPass, Method: fitness.EvidenceMethodHuman, Reason: "reviewed", ArtifactDigests: []string{loaded.DocumentDigest, loaded.MarkdownDigest}, ObservedAt: at, ValidUntil: at.Add(time.Hour)})
	}
	aBytes := jsonBytes(a)
	aPath := filepath.Join(t.TempDir(), "adjudication.json")
	if err := os.WriteFile(aPath, aBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDocumentAdjudication(aPath, hashBytes(aBytes), loaded.PacketDigest); err != nil {
		t.Fatalf("adjudication load: %v", err)
	}
	passArgs := append(append([]string{}, reportArgs...), "--adjudication", aPath, "--adjudication-digest", hashBytes(aBytes))
	out.Reset()
	diag.Reset()
	if code := runLab(context.Background(), passArgs, &out, &diag); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"declared_criteria_status":"pass"`)) {
		t.Fatalf("declared pass %d: %s %s", code, out.String(), diag.String())
	}
	if cueBin, err := exec.LookPath("cue"); err == nil {
		vet := func(raw []byte, definition string) {
			t.Helper()
			file := filepath.Join(t.TempDir(), "review.json")
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(cueBin, "vet", "schema.cue", file, "-d", definition, "-c").CombinedOutput(); err != nil {
				t.Fatalf("actual %s violates CUE: %v %s", definition, err, output)
			}
		}
		vet(aBytes, "#DocumentAdjudication")
		vet(out.Bytes(), "#DocumentReviewReport")
		reject := func(raw []byte, definition string) {
			t.Helper()
			file := filepath.Join(t.TempDir(), "contradiction.json")
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(cueBin, "vet", "schema.cue", file, "-d", definition, "-c").CombinedOutput(); err == nil {
				t.Fatalf("CUE accepted contradictory %s: %s", definition, output)
			}
		}
		var agent map[string]any
		if err := json.Unmarshal(aBytes, &agent); err != nil {
			t.Fatal(err)
		}
		agent["reviewer"].(map[string]any)["kind"] = "agent"
		reject(jsonBytes(agent), "#DocumentAdjudication")
		mutatedReport := func(goals, criteria, lengthStatus, lengthClass string) []byte {
			t.Helper()
			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				t.Fatal(err)
			}
			m["declared_goals_status"], m["declared_criteria_status"] = goals, criteria
			m["length"].(map[string]any)["status"] = lengthStatus
			m["length"].(map[string]any)["class"] = lengthClass
			return jsonBytes(m)
		}
		for _, row := range []struct{ goals, criteria, status, class string }{
			{"pass", "pass", "hold", "required"},
			{"fail", "pass", "warning", "advisory"},
			{"unknown", "pass", "warning", "advisory"},
			{"pass", "unknown", "warning", "advisory"},
		} {
			reject(mutatedReport(row.goals, row.criteria, row.status, row.class), "#DocumentReviewReport")
		}
		for _, row := range []struct{ goals, criteria, status, class string }{
			{"fail", "fail", "warning", "advisory"},
			{"unknown", "unknown", "warning", "advisory"},
			{"unknown", "fail", "hold", "required"},
		} {
			vet(mutatedReport(row.goals, row.criteria, row.status, row.class), "#DocumentReviewReport")
		}
	}
	if err := os.Chmod(rootPath, 0500); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diag.Reset()
	if code := runLab(context.Background(), args, &out, &diag); code != 0 {
		t.Fatalf("read-only root %d: %s", code, diag.String())
	}
	if err := os.Chmod(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(rootPath, cell.ID+".receipt.json"), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diag.Reset()
	if code := runLab(context.Background(), args, &out, &diag); code != 2 || out.Len() != 0 {
		t.Fatalf("public receipt accepted %d", code)
	}
}

func TestDocumentReviewAssessmentOffline(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	cell, _ := corpus.cell("technical-design-technical-r1")
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	engine, target, err := documentEngine(&documentGenerator{}, &documentSupport{}, "gen-model", "rev-model", 256, corpus.Fixtures[cell.DocumentID], cell, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	v, err := engine.Generate(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := engine.Render(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	side, status, deviation, err := documentLengthAssess(corpus.Fixtures[cell.DocumentID], v, rendered, hashBytes([]byte("scope")), at)
	if err != nil || status != "hold" {
		t.Fatalf("length: %s %v", status, err)
	}
	packet := documentReviewPacket{documentReviewIdentity: documentReviewIdentity{DocumentDigest: v.Receipt.DocumentDigest, MarkdownDigest: rendered.MarkdownDigest, SubjectDigest: side.Goals[0].Result.SubjectDigest, Length: documentReviewLength{CriterionID: corpus.Fixtures[cell.DocumentID].LengthGoal.CriterionID, Class: "required", WordCount: side.WordCount, Deviation: deviation, Status: status}, SemanticGoals: []documentReviewGoal{}}, PacketDigest: hashBytes([]byte("packet"))}
	for _, g := range side.Goals[:len(side.Goals)-1] {
		packet.SemanticGoals = append(packet.SemanticGoals, documentReviewGoal{CriterionID: g.Profile.Criteria[0].ID})
	}
	rows := make([]fitness.Observation, 0, len(packet.SemanticGoals))
	for i, g := range packet.SemanticGoals {
		rows = append(rows, fitness.Observation{CriterionID: g.CriterionID, DefinitionDigest: side.Goals[i].Profile.Criteria[0].DefinitionDigest, SubjectDigest: packet.SubjectDigest, Outcome: fitness.FindingPass, Method: fitness.EvidenceMethodHuman, Reason: "checked", ArtifactDigests: []string{packet.DocumentDigest, packet.MarkdownDigest}, ObservedAt: at, ValidUntil: at.Add(time.Hour)})
	}
	a := &documentAdjudication{Observations: rows}
	a.Reviewer.Relationship = "independent"
	report, code, err := assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || code != 1 || report.DeclaredGoalsStatus != fitness.RequiredPass || report.DeclaredCriteriaStatus != fitness.RequiredFail {
		t.Fatalf("required hold: code=%d report=%+v err=%v", code, report, err)
	}
	a.Reviewer.Relationship = "producer"
	report, code, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || code != 1 || report.DeclaredGoalsStatus != fitness.RequiredUnknown {
		t.Fatalf("producer: code=%d status=%s err=%v", code, report.DeclaredGoalsStatus, err)
	}
	if report.SemanticGoals[0].Eligible || report.SemanticGoals[0].EligibilityRule != "independence-not-declared" {
		t.Fatal("producer eligibility mislabeled")
	}
	a.Reviewer.Relationship = "independent"
	a.Observations[0].Outcome = fitness.FindingFail
	report, code, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || code != 1 || report.DeclaredGoalsStatus != fitness.RequiredFail {
		t.Fatalf("known failure: code=%d status=%s err=%v", code, report.DeclaredGoalsStatus, err)
	}
	a.Observations = append([]fitness.Observation{}, rows[1:]...)
	report, _, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || report.SemanticGoals[0].Eligible || report.SemanticGoals[0].EligibilityRule != string(fitness.RuleMissingObservation) {
		t.Fatal("missing eligibility mislabeled")
	}
	a.Observations = append([]fitness.Observation{}, rows...)
	a.Observations[0].Method = fitness.EvidenceMethodHeuristic
	report, _, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || report.SemanticGoals[0].Eligible || report.SemanticGoals[0].EligibilityRule != string(fitness.RuleMethodDisallowed) {
		t.Fatal("heuristic eligibility mislabeled")
	}
	a.Observations[0].Method = fitness.EvidenceMethodHuman
	a.Observations[0].ObservedAt, a.Observations[0].ValidUntil = at.Add(-2*time.Hour), at.Add(-time.Hour)
	report, _, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || report.SemanticGoals[0].Eligible || report.SemanticGoals[0].EligibilityRule != string(fitness.RuleObservationStale) {
		t.Fatal("stale eligibility mislabeled")
	}
	a.Observations[0].ObservedAt, a.Observations[0].ValidUntil = at, at.Add(time.Hour)
	a.Observations[0].Outcome = fitness.FindingUnknown
	report, _, err = assessDocumentReview(packet, side, corpus.Fixtures[cell.DocumentID], a, hashBytes([]byte("input")), at, false)
	if err != nil || !report.SemanticGoals[0].Eligible || report.SemanticGoals[0].EligibilityRule != string(fitness.RuleObservedUnknown) {
		t.Fatal("admitted unknown mislabeled")
	}
}

func TestDocumentReviewPrivateAndClock(t *testing.T) {
	for _, raw := range []string{"", "2026-10-24T12:00:00+00:00", "2026-10-24T12:00:00.0Z", "2026-10-24T12:00:00Zx"} {
		if _, err := reviewAt(raw); err == nil {
			t.Fatalf("accepted noncanonical %q", raw)
		}
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "receipt")
	if err := os.WriteFile(file, []byte("private"), 0400); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, err := reviewPrivateBytes(root, "receipt", 32)
	if err != nil || !bytes.Equal(got, []byte("private")) {
		t.Fatalf("read-only refused: %v", err)
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewPrivateBytes(root, "receipt", 32); err == nil {
		t.Fatal("world-readable file accepted")
	}
	if err := os.Symlink(file, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewPrivateBytes(root, "link", 32); err == nil {
		t.Fatal("symlink accepted")
	}
}

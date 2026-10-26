package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type ownerOracleRow struct {
	CaseID     string          `json:"case_id"`
	DocumentID string          `json:"document_id"`
	CaseDigest string          `json:"case_digest"`
	Content    json.RawMessage `json:"content"`
}
type ownerOraclePacket struct {
	Version          string           `json:"version"`
	PacketDigest     string           `json:"packet_digest"`
	StudyDigest      string           `json:"study_digest"`
	CriterionID      string           `json:"criterion_id"`
	CriterionSHA     string           `json:"criterion_sha256"`
	CriterionText    string           `json:"criterion_text"`
	CorpusDigest     string           `json:"corpus_digest"`
	GoalDigest       string           `json:"goal_scope_digest"`
	BaseDigest       string           `json:"base_review_scope_digest"`
	ReviewDigest     string           `json:"review_scope_digest"`
	RegressionDigest string           `json:"regression_source_digest"`
	Planned          int              `json:"planned"`
	Rows             []ownerOracleRow `json:"rows"`
}

func ownerOracleArgs(command string) []string {
	return []string{command,
		"--study", "testdata/paired-review-study.json",
		"--corpus", "testdata/document-corpus-v2",
		"--goal-scope", "testdata/document-scopes/v2-length-goals.json",
		"--goal-scope-digest", "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403",
		"--base-review-scope", "testdata/document-scopes/v2-support-review.json",
		"--review-scope", "testdata/document-scopes/v2-paired-support-review.json",
		"--review-scope-digest", "f8d86a5366e2317f2a5dd6bc90fae074452a5d3c2176dccf9b1e589043ac6a49",
		"--regression-source", "../../docs/evidence/prose-headless/2026-10-25-long-pilot/regressions.json"}
}
func ownerOracleRun(ctx context.Context, args []string) (int, []byte, []byte) {
	var out, errOut bytes.Buffer
	code := runLab(ctx, args, &out, &errOut)
	return code, out.Bytes(), errOut.Bytes()
}
func ownerOracleJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, b)
	}
	return v
}
func ownerOracleHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ownerOraclePrepare(t *testing.T) (ownerOraclePacket, []byte) {
	t.Helper()
	code, b, errOut := ownerOracleRun(context.Background(), append(ownerOracleArgs("review-oracle-prepare"), "--include-content"))
	if code != 0 {
		t.Fatalf("prepare=%d %s", code, errOut)
	}
	var p ownerOraclePacket
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p, b
}
func ownerOracleDeclarations(t *testing.T, p ownerOraclePacket) map[string]any {
	t.Helper()
	rows := make([]any, 0, len(p.Rows))
	for _, r := range p.Rows {
		c := ownerOracleJSON(t, r.Content)
		reason := "missing-premise"
		if c["historical_expected"] == "supported" {
			reason = "sufficient-premises"
		}
		rows = append(rows, map[string]any{"case_id": r.CaseID, "case_digest": r.CaseDigest,
			"disposition": "affirmed", "interpretation": "literal", "reason_code": reason,
			"premise_fact_ids": c["cited_fact_ids"], "counter_fact_ids": []string{},
			"basis":         "Synthetic structural test declaration, not an actual semantic adjudication.",
			"assessor_kind": "agent", "assessor_ref": "owner-private-marker", "proposed_fact_ids": []string{}})
	}
	return map[string]any{"version": "prose-lab.oracle-assessments/1", "packet_digest": p.PacketDigest,
		"criterion_id": p.CriterionID, "criterion_sha256": p.CriterionSHA, "rows": rows}
}
func ownerOracleWrite(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "assessments.json")
	if err = os.WriteFile(f, b, 0600); err != nil {
		t.Fatal(err)
	}
	return f
}
func ownerOracleReport(t *testing.T, p ownerOraclePacket, v any, content bool) (int, []byte, []byte) {
	t.Helper()
	args := append(ownerOracleArgs("review-oracle-report"), "--packet-digest", p.PacketDigest, "--assessments", ownerOracleWrite(t, v))
	if content {
		args = append(args, "--include-content")
	}
	return ownerOracleRun(context.Background(), args)
}
func ownerOracleFalseAuthority(t *testing.T, v map[string]any) {
	t.Helper()
	for _, k := range []string{"human_identity_authenticated", "source_authenticated", "model_site_adoption", "independent_acceptance", "publication_authority", "new_study_admission"} {
		if x, ok := v[k].(bool); !ok || x {
			t.Fatalf("%s=%v", k, v[k])
		}
	}
}

func TestOwnerOracleCanonicalProjectionAndReuse(t *testing.T) {
	p, full := ownerOraclePrepare(t)
	if p.Planned != 70 || len(p.Rows) != 70 {
		t.Fatalf("roster %d/%d", p.Planned, len(p.Rows))
	}
	criterion, err := os.ReadFile("oracle-criteria/cited-entailment-v1.txt")
	if err != nil {
		t.Fatal(err)
	}
	if p.CriterionText != string(criterion) || p.CriterionSHA != ownerOracleHash(criterion) {
		t.Fatal("criterion projection/binding")
	}
	rawCases := make([]json.RawMessage, 0, 70)
	for i, r := range p.Rows {
		if i > 0 && p.Rows[i-1].CaseID >= r.CaseID {
			t.Fatal("nonunique/nonlexical roster")
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, r.Content); err != nil {
			t.Fatal(err)
		}
		if ownerOracleHash(compact.Bytes()) != r.CaseDigest {
			t.Fatalf("canonical case %s", r.CaseID)
		}
		rawCases = append(rawCases, json.RawMessage(compact.Bytes()))
	}
	canonical := struct {
		Version      string            `json:"version"`
		CriterionID  string            `json:"criterion_id"`
		CriterionSHA string            `json:"criterion_sha256"`
		Corpus       string            `json:"corpus_digest"`
		Goal         string            `json:"goal_scope_digest"`
		Base         string            `json:"base_review_scope_digest"`
		Scope        string            `json:"review_scope_digest"`
		Regression   string            `json:"regression_source_digest"`
		Rows         []json.RawMessage `json:"rows"`
	}{"prose-lab.oracle-canonical-packet/1", p.CriterionID, p.CriterionSHA, p.CorpusDigest, p.GoalDigest, p.BaseDigest, p.ReviewDigest, p.RegressionDigest, rawCases}
	canonicalBytes, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if ownerOracleHash(canonicalBytes) != p.PacketDigest {
		t.Fatal("canonical packet hash differs from independently assembled projection")
	}
	code, meta, errOut := ownerOracleRun(context.Background(), ownerOracleArgs("review-oracle-prepare"))
	if code != 0 {
		t.Fatalf("meta %d %s", code, errOut)
	}
	m := ownerOracleJSON(t, meta)
	ownerOracleFalseAuthority(t, m)
	for _, secret := range []string{"assertion", "historical_expected", "criterion_text", "assessor_ref", "basis", "content"} {
		if bytes.Contains(meta, []byte(fmt.Sprintf("%q:", secret))) {
			t.Fatalf("metadata exposed %s", secret)
		}
	}
	var q ownerOraclePacket
	if json.Unmarshal(meta, &q) != nil || q.PacketDigest != p.PacketDigest {
		t.Fatal("projection changed identity")
	}
	studyRaw, err := os.ReadFile("testdata/paired-review-study.json")
	if err != nil {
		t.Fatal(err)
	}
	study := ownerOracleJSON(t, studyRaw)
	study["id"] = "owner-oracle-trace"
	study["budget_micro_usd"] = float64(4900000)
	lane := study["review"].(map[string]any)
	lane["model"] = "synthetic-owner-alternate"
	lane["input_rate"] = float64(3000000)
	f := ownerOracleWrite(t, study)
	args := ownerOracleArgs("review-oracle-prepare")
	args[2] = f
	code, changed, errOut := ownerOracleRun(context.Background(), args)
	if code != 0 {
		t.Fatalf("changed trace %d %s", code, errOut)
	}
	var alt ownerOraclePacket
	if json.Unmarshal(changed, &alt) != nil || alt.PacketDigest != p.PacketDigest || alt.StudyDigest == p.StudyDigest {
		t.Fatal("trace contaminated semantic identity or was not recorded")
	}
	for i, r := range alt.Rows {
		if r.CaseDigest != p.Rows[i].CaseDigest {
			t.Fatal("study change invalidated unchanged case")
		}
	}
	ownerOracleFalseAuthority(t, ownerOracleJSON(t, full))
}

func TestOwnerOracleActualSourceLineage(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	scopeRaw, err := os.ReadFile("testdata/document-scopes/v2-support-review.json")
	if err != nil {
		t.Fatal(err)
	}
	pairedRaw, err := os.ReadFile("testdata/document-scopes/v2-paired-support-review.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range p.Rows {
		c := ownerOracleJSON(t, r.Content)
		l := c["assertion_lineage"].(map[string]any)
		var source map[string]any
		switch l["artifact_kind"] {
		case "base-review-scope":
			if l["artifact_digest"] != ownerOracleHash(scopeRaw) {
				t.Fatal("positive digest")
			}
			source = ownerOracleJSON(t, scopeRaw)
		case "paired-review-scope":
			if l["artifact_digest"] != ownerOracleHash(pairedRaw) {
				t.Fatal("extension digest")
			}
			source = ownerOracleJSON(t, pairedRaw)
		case "corpus-fixture":
			raw, e := os.ReadFile(filepath.Join("testdata/document-corpus-v2", r.DocumentID+".json"))
			if e != nil {
				t.Fatal(e)
			}
			if l["artifact_digest"] != ownerOracleHash(raw) {
				t.Fatal("negative digest")
			}
			source = ownerOracleJSON(t, raw)
		default:
			t.Fatalf("lineage owner %v", l)
		}
		var value any = source
		for _, segment := range strings.Split(strings.TrimPrefix(l["json_pointer"].(string), "/"), "/") {
			switch x := value.(type) {
			case map[string]any:
				value = x[segment]
			case []any:
				var i int
				if _, e := fmt.Sscan(segment, &i); e != nil || i < 0 || i >= len(x) {
					t.Fatal("pointer index")
				}
				value = x[i]
			default:
				t.Fatalf("bad pointer in %s", r.CaseID)
			}
		}
		if value != c["assertion"] {
			t.Fatalf("assertion lineage %s", r.CaseID)
		}
		contextFacts := c["context_facts"].([]any)
		sourceRaw, e := os.ReadFile(filepath.Join("testdata/document-corpus-v2", r.DocumentID+".json"))
		if e != nil {
			t.Fatal(e)
		}
		original := ownerOracleJSON(t, sourceRaw)["facts"].([]any)
		if len(contextFacts) != len(original) {
			t.Fatalf("context shrunk %s", r.CaseID)
		}
		for _, fact := range contextFacts {
			f := fact.(map[string]any)
			fl := f["lineage"].(map[string]any)
			if fl["artifact_digest"] != ownerOracleHash(sourceRaw) {
				t.Fatal("fact digest")
			}
			var index int
			if _, e := fmt.Sscanf(fl["json_pointer"].(string), "/facts/%d", &index); e != nil {
				t.Fatal("fact pointer")
			}
			rawFact := original[index].(map[string]any)
			for _, k := range []string{"id", "text", "kind", "occurred_at", "required"} {
				if !reflect.DeepEqual(f[k], rawFact[k]) {
					t.Fatalf("fact content %s/%s", r.CaseID, k)
				}
			}
		}
	}
}

func TestOwnerOracleDeclaredCoverageAndHolds(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	a := ownerOracleDeclarations(t, p)
	code, raw, errOut := ownerOracleReport(t, p, a, false)
	if code != 0 {
		t.Fatalf("complete %d %s", code, errOut)
	}
	report := ownerOracleJSON(t, raw)
	if report["assessed"] != float64(70) || report["affirmed"] != float64(70) || report["floor_declared_ready"] != true || report["declared_assessments_complete"] != true {
		t.Fatal("structural coverage")
	}
	ownerOracleFalseAuthority(t, report)
	if bytes.Contains(raw, []byte("owner-private-marker")) {
		t.Fatal("assessor leaked")
	}
	rows := a["rows"].([]any)
	for _, r := range rows {
		v := r.(map[string]any)
		if v["case_id"] == "operational-runbook--p-03" {
			v["disposition"] = "ambiguous"
			v["interpretation"] = "unresolved"
			v["reason_code"] = "modality-ambiguity"
			v["basis"] = "Synthetic missing-premise test: permission after does not unambiguously establish only after."
			v["assessor_kind"] = "human"
			v["assessor_ref"] = "claimed-human-not-authenticated"
		}
	}
	code, raw, errOut = ownerOracleReport(t, p, a, true)
	if code != 1 {
		t.Fatalf("ambiguity %d %s", code, errOut)
	}
	report = ownerOracleJSON(t, raw)
	ownerOracleFalseAuthority(t, report)
	if report["ambiguous"] != float64(1) || report["unreviewed"] != float64(0) || report["declared_assessments_complete"] != true || report["floor_declared_ready"] != false {
		t.Fatal("ambiguity promoted or coverage lost")
	}
	// Known source issue outranks later missing coverage but never untrusted input.
	a["rows"] = []any{rows[0]}
	v := rows[0].(map[string]any)
	v["disposition"] = "revise"
	v["interpretation"] = "faithful-paraphrase"
	v["reason_code"] = "citation-mismatch"
	v["proposed_fact_ids"] = []string{"f-02"}
	code, raw, _ = ownerOracleReport(t, p, a, false)
	report = ownerOracleJSON(t, raw)
	if code != 1 || report["revise"] != float64(1) || report["unreviewed"] != float64(69) || report["floor_declared_ready"] != false {
		t.Fatal("prospective revision changed coverage or acceptance")
	}
	a = ownerOracleDeclarations(t, p)
	a["rows"] = a["rows"].([]any)[:69]
	code, raw, _ = ownerOracleReport(t, p, a, false)
	report = ownerOracleJSON(t, raw)
	if code != 2 || report["unreviewed"] != float64(1) || len(report["rows"].([]any)) != 70 {
		t.Fatal("missing row was accepted or omitted")
	}
}

func TestOwnerOracleInputRefusals(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	cases := map[string]func(map[string]any){
		"packet":          func(a map[string]any) { a["packet_digest"] = strings.Repeat("0", 64) },
		"criterion":       func(a map[string]any) { a["criterion_sha256"] = strings.Repeat("0", 64) },
		"unknown":         func(a map[string]any) { a["publication_authority"] = true },
		"duplicate":       func(a map[string]any) { rows := a["rows"].([]any); a["rows"] = append(rows, rows[0]) },
		"null-list":       func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["counter_fact_ids"] = nil },
		"blank-unicode":   func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["basis"] = "\u00a0\u2003" },
		"utf8-byte-limit": func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["basis"] = strings.Repeat("界", 1366) },
		"case-digest":     func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["case_digest"] = strings.Repeat("0", 64) },
		"foreign-fact":    func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["counter_fact_ids"] = []string{"f-99"} },
		"missing-source-basis": func(a map[string]any) {
			r := a["rows"].([]any)[0].(map[string]any)
			r["premise_fact_ids"] = []string{}
			r["counter_fact_ids"] = []string{}
		},
		"uncited-positive-premise": func(a map[string]any) {
			for _, r := range a["rows"].([]any) {
				v := r.(map[string]any)
				if v["case_id"] == "architecture-decision--p-01" {
					v["premise_fact_ids"] = []string{"f-02"}
				}
			}
		},
		"proposed-as-affirmed": func(a map[string]any) { a["rows"].([]any)[0].(map[string]any)["proposed_fact_ids"] = []string{"f-02"} },
		"partial-known-issue-with-foreign-tail": func(a map[string]any) {
			rows := a["rows"].([]any)
			v := rows[0].(map[string]any)
			v["disposition"] = "ambiguous"
			v["interpretation"] = "unresolved"
			v["reason_code"] = "scope-ambiguity"
			rows[1].(map[string]any)["case_id"] = "foreign-control"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := ownerOracleDeclarations(t, p)
			mutate(a)
			code, raw, _ := ownerOracleReport(t, p, a, false)
			if code != 2 || len(raw) != 0 {
				t.Fatalf("invalid input emitted decision: %d %s", code, raw)
			}
		})
	}
}

type ownerOracleBadWriter struct{ short bool }

func (w ownerOracleBadWriter) Write(p []byte) (int, error) {
	if w.short {
		return 1, nil
	}
	return 0, errors.New("owner writer failed")
}
func TestOwnerOracleHelpCancellationAndOutput(t *testing.T) {
	for _, cmd := range []string{"review-oracle-prepare", "review-oracle-report"} {
		for _, help := range []string{"--help", "-h", "help"} {
			code, b, _ := ownerOracleRun(context.Background(), []string{cmd, help})
			if code != 0 || !bytes.Contains(b, []byte("Usage:")) {
				t.Fatal("help not key free")
			}
		}
		code, b, _ := ownerOracleRun(context.Background(), []string{cmd, "--help", "--unknown"})
		if code != 2 || len(b) != 0 {
			t.Fatal("help masked invalid flag")
		}
	}
	for _, flag := range []string{"--live=false", "--case=", "--variant=", "--journal=", "--artifacts=", "--prepared-digest=", "--packet-digest=", "--assessments="} {
		code, b, _ := ownerOracleRun(context.Background(), append(ownerOracleArgs("review-oracle-prepare"), flag))
		if code != 2 || len(b) != 0 {
			t.Fatalf("effect or misplaced flag %s", flag)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, b, _ := ownerOracleRun(ctx, ownerOracleArgs("review-oracle-prepare"))
	if code != 2 || len(b) != 0 {
		t.Fatal("cancelled preparation emitted output")
	}
	for _, short := range []bool{true, false} {
		if code := runLab(context.Background(), ownerOracleArgs("review-oracle-prepare"), ownerOracleBadWriter{short: short}, io.Discard); code != 2 {
			t.Fatal("broken output success")
		}
	}
}

func TestOwnerOracleExaminedCounterAndAmbiguityProposal(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	a := ownerOracleDeclarations(t, p)
	for _, value := range a["rows"].([]any) {
		row := value.(map[string]any)
		if row["case_id"] == "architecture-decision--p-01" {
			row["counter_fact_ids"] = []string{"f-01"}
			row["basis"] = "Synthetic structure test: the same cited fact was examined in both roles; no semantic truth is established."
		}
	}
	code, raw, errOut := ownerOracleReport(t, p, a, false)
	if code != 0 {
		t.Fatalf("examined counter caused an undocumented semantic inference: %d %s", code, errOut)
	}
	ownerOracleFalseAuthority(t, ownerOracleJSON(t, raw))
	for _, value := range a["rows"].([]any) {
		row := value.(map[string]any)
		if row["case_id"] == "operational-runbook--p-03" {
			row["disposition"] = "ambiguous"
			row["interpretation"] = "unresolved"
			row["reason_code"] = "modality-ambiguity"
			row["proposed_fact_ids"] = []string{"f-07"}
			row["basis"] = "Synthetic declaration: a prospective citation may help investigate the ambiguity, but does not supply current positive support."
		}
	}
	code, raw, errOut = ownerOracleReport(t, p, a, true)
	if code != 1 {
		t.Fatalf("prospective proposal on unresolved case refused: %d %s", code, errOut)
	}
	r := ownerOracleJSON(t, raw)
	ownerOracleFalseAuthority(t, r)
	if r["ambiguous"] != float64(1) || r["floor_declared_ready"] != false {
		t.Fatal("proposal resolved ambiguity or promoted the floor")
	}
}

func TestOwnerOracleHeldExample(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	raw, err := os.ReadFile("testdata/oracle-assessments-held.json")
	if err != nil {
		t.Fatal(err)
	}
	input := ownerOracleJSON(t, raw)
	code, output, errOut := ownerOracleReport(t, p, input, false)
	if code != 1 {
		t.Fatalf("documented held example changed: %d %s", code, errOut)
	}
	report := ownerOracleJSON(t, output)
	ownerOracleFalseAuthority(t, report)
	if report["ambiguous"] != float64(1) || report["unreviewed"] != float64(69) || report["floor_declared_ready"] != false {
		t.Fatal("held example lost uncertainty or coverage")
	}
	vetPairedOutput(t, "#OracleAssessments", raw)
	vetPairedOutput(t, "#OracleReport", output)
}

func TestOwnerOracleCUEFactReferenceConsistency(t *testing.T) {
	_, original := ownerOraclePrepare(t)
	changes := map[string]func(map[string]any){
		"cited-record-id": func(c map[string]any) {
			c["cited_facts"].([]any)[0].(map[string]any)["id"] = "f-02"
		},
		"duplicate-context-id": func(c map[string]any) {
			facts := c["context_facts"].([]any)
			facts[1].(map[string]any)["id"] = facts[0].(map[string]any)["id"]
		},
		"cited-record-missing": func(c map[string]any) { c["cited_facts"] = []any{} },
		"cited-context-omitted": func(c map[string]any) {
			facts := []any{}
			for _, v := range c["context_facts"].([]any) {
				if v.(map[string]any)["id"] != "f-01" {
					facts = append(facts, v)
				}
			}
			c["context_facts"] = facts
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			p := ownerOracleJSON(t, original)
			for _, value := range p["rows"].([]any) {
				row := value.(map[string]any)
				if row["case_id"] == "architecture-decision--p-01" {
					change(row["content"].(map[string]any))
				}
			}
			file := ownerOracleWrite(t, p)
			cmd := exec.Command("cue", "vet", "-c", "schema.cue", file, "-d", "#OraclePacket")
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("CUE admitted contradictory fact references: %s", out)
			}
		})
	}
}

func TestOwnerOracleAssessmentStateReasonParity(t *testing.T) {
	p, _ := ownerOraclePrepare(t)
	var representatives []ownerOracleRow
	for _, row := range p.Rows {
		if row.CaseID == "architecture-decision--p-01" || row.CaseID == "architecture-decision--n-01" {
			representatives = append(representatives, row)
		}
	}
	if len(representatives) != 2 {
		t.Fatal("missing source representatives")
	}
	for _, disposition := range []string{"affirmed", "ambiguous", "revise"} {
		for _, interpretation := range []string{"literal", "faithful-paraphrase", "normative-direction", "unresolved"} {
			for _, reason := range []string{"sufficient-premises", "missing-premise", "scope-ambiguity", "modality-ambiguity", "contradiction", "citation-mismatch"} {
				t.Run(disposition+"/"+interpretation+"/"+reason, func(t *testing.T) {
					acceptedByGo := false
					var sample map[string]any
					for _, binding := range representatives {
						c := ownerOracleJSON(t, binding.Content)
						decl := ownerOracleDeclarations(t, p)
						var row map[string]any
						for _, value := range decl["rows"].([]any) {
							if value.(map[string]any)["case_id"] == binding.CaseID {
								row = value.(map[string]any)
								break
							}
						}
						row["disposition"], row["interpretation"], row["reason_code"] = disposition, interpretation, reason
						if reason == "contradiction" {
							row["counter_fact_ids"] = c["cited_fact_ids"]
						}
						if disposition == "revise" && reason == "citation-mismatch" {
							row["proposed_fact_ids"] = []string{"f-02"}
						}
						decl["rows"] = []any{row}
						_, out, _ := ownerOracleReport(t, p, decl, false)
						acceptedByGo = acceptedByGo || len(out) > 0
						if sample == nil {
							sample = row
						}
					}
					file := ownerOracleWrite(t, sample)
					err := exec.Command("cue", "vet", "-c", "schema.cue", file, "-d", "#OracleAssessment").Run()
					acceptedByCUE := err == nil
					if acceptedByCUE != acceptedByGo {
						t.Fatalf("intrinsic disposition/reason parity: CUE=%v Go(any source class)=%v", acceptedByCUE, acceptedByGo)
					}
				})
			}
		}
	}
}

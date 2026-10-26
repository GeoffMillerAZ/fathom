package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ownerReviewPin = "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8"

// Authored independently of the scope loader and report evaluator. These are
// synthetic wiring oracles, not observations about any real review model.
type ownerReviewScope struct {
	NegativeCaseIDs  []string `json:"negative_case_ids"`
	PositiveControls []struct {
		ID         string   `json:"id"`
		DocumentID string   `json:"document_id"`
		Kind       string   `json:"kind"`
		FactIDs    []string `json:"fact_ids"`
		Assertion  string   `json:"assertion"`
	} `json:"positive_controls"`
}
type ownerReviewCase struct {
	ID, Assertion, Expected string
	FactIDs                 []string
}

func ownerReviewSetup(t *testing.T, raw []byte) ([]string, []ownerReviewCase) {
	t.Helper()
	var s ownerReviewScope
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	var cases []ownerReviewCase
	for _, c := range s.PositiveControls {
		cases = append(cases, ownerReviewCase{c.ID, c.Assertion, "supported", c.FactIDs})
	}
	for _, id := range s.NegativeCaseIDs {
		doc, negID, ok := strings.Cut(id, "--")
		if !ok {
			t.Fatal("bad owner case ID")
		}
		var d struct {
			Negatives []struct {
				ID        string `json:"id"`
				Assertion string `json:"mutated_assertion"`
				Target    string `json:"target_fact_id"`
			} `json:"negative_cases"`
		}
		if err := json.Unmarshal(mustReadDocumentTest(t, filepath.Join("testdata/document-corpus-v2", doc+".json")), &d); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range d.Negatives {
			if n.ID == negID {
				cases = append(cases, ownerReviewCase{id, n.Assertion, "unsupported", []string{n.Target}})
				found = true
			}
		}
		if !found {
			t.Fatal("missing independently authored negative")
		}
	}
	root := t.TempDir()
	scope := filepath.Join(root, "scope.json")
	if err := os.WriteFile(scope, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pin := hashBytes(raw)
	bindings := map[string]string{"protocol": "prose-lab.support-review/1", "corpus_digest": frozenV2Corpus, "goal_scope_digest": frozenV2GoalScope, "review_scope_digest": pin, "lane_id": "review"}
	ids := make([]string, 0, len(cases))
	for _, c := range cases {
		ids = append(ids, c.ID)
	}
	l := lane{Provider: "anthropic", Model: "review-owner-model", PricingSource: "synthetic-fixture", MaxInputTokens: 1000000, MaxOutputTokens: 256, InputRate: 1000, OutputRate: 1000, ReserveInputRate: 1000, ReserveOutputRate: 1000}
	study := canaryStudy{Version: "prose-lab.canary/1", ID: "review-" + hashBytes(jsonBytes(bindings)), DatasetKind: "synthetic", BudgetMicroUSD: 1000000, Lanes: map[string]lane{"review": l}, ExpectedCaseIDs: ids}
	file := filepath.Join(root, "study.json")
	if err := os.WriteFile(file, jsonBytes(study), 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"--study", file, "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope, "--review-scope", scope, "--review-scope-digest", pin, "--review-lane", "review"}, cases
}

func ownerReviewPrepare(t *testing.T, base []string, id string) map[string]any {
	t.Helper()
	var out, errOut bytes.Buffer
	args := append(append([]string{"review-prepare"}, base...), "--case", id)
	if code := runReviewWith(context.Background(), args, &out, &errOut, reviewDeps{}); code != 0 {
		t.Fatalf("prepare %d: %s", code, errOut.String())
	}
	var p map[string]any
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

type ownerReviewTransport func(*http.Request) (*http.Response, error)

func (f ownerReviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOwnerReviewFullNativeCohort(t *testing.T) {
	raw := mustReadDocumentTest(t, "testdata/document-scopes/v2-support-review.json")
	if hashBytes(raw) != ownerReviewPin {
		t.Fatal("independently reviewed scope changed")
	}
	base, cases := ownerReviewSetup(t, raw)
	if len(cases) != 67 {
		t.Fatal("owner denominator changed")
	}
	byAssertion := map[string]ownerReviewCase{}
	for _, c := range cases {
		if _, exists := byAssertion[c.Assertion]; exists {
			t.Fatal("ambiguous owner fake response")
		}
		byAssertion[c.Assertion] = c
	}
	calls := 0
	deps := reviewDeps{apiKey: "offline-owner-fixture", now: func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) }, captureHTTP: func() (*http.Client, *captureTransport) {
		capture := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: ownerReviewTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			wire, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var body struct {
				Messages []struct{ Content json.RawMessage } `json:"messages"`
			}
			if err := json.Unmarshal(wire, &body); err != nil || len(body.Messages) != 1 {
				return nil, fmt.Errorf("unexpected native messages")
			}
			var text string
			if err := json.Unmarshal(body.Messages[0].Content, &text); err != nil {
				var blocks []struct{ Type, Text string }
				if err := json.Unmarshal(body.Messages[0].Content, &blocks); err != nil || len(blocks) != 1 || blocks[0].Type != "text" {
					return nil, fmt.Errorf("unexpected native content")
				}
				text = blocks[0].Text
			}
			var payload struct {
				Assertion string   `json:"assertion"`
				Cited     []string `json:"cited_fact_ids"`
			}
			if err := json.Unmarshal([]byte(text), &payload); err != nil {
				return nil, err
			}
			oracle, ok := byAssertion[payload.Assertion]
			if !ok || !bytes.Equal(jsonBytes(payload.Cited), jsonBytes(oracle.FactIDs)) {
				return nil, fmt.Errorf("native request diverged from independent citation oracle")
			}
			answer := string(jsonBytes(map[string]string{"status": oracle.Expected, "reason": "PRIVATE_FIXTURE_REASON"}))
			native := jsonBytes(map[string]any{"id": "msg-owner", "type": "message", "role": "assistant", "model": "review-owner-model", "content": []map[string]string{{"type": "text", "text": answer}}, "stop_reason": "end_turn", "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 10}})
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(native)), Request: req}, nil
		})}
		return &http.Client{Transport: capture}, capture
	}}
	root := t.TempDir()
	journal, artifacts := filepath.Join(root, "journal"), filepath.Join(root, "artifacts")
	for _, c := range cases {
		p := ownerReviewPrepare(t, base, c.ID)
		if p["planned"] != float64(67) || p["expected"] != c.Expected || p["publication_authority"] != false || p["independent_acceptance"] != false {
			t.Fatal("dishonest preparation metadata")
		}
		args := append(append([]string{"review-capture"}, base...), "--case", c.ID, "--prepared-digest", p["prepared_digest"].(string), "--journal", journal, "--artifacts", artifacts, "--live")
		var out, errOut bytes.Buffer
		if code := runReviewWith(context.Background(), args, &out, &errOut, deps); code != 0 {
			t.Fatalf("case %s capture%d: %s %s", c.ID, code, errOut.String(), out.String())
		}
		if strings.Contains(out.String(), "PRIVATE_FIXTURE_REASON") || strings.Contains(out.String(), c.Assertion) {
			t.Fatal("capture exposed private content")
		}
	}
	if calls != 67 {
		t.Fatalf("fake wire calls=%d", calls)
	}
	reportArgs := append(append([]string{"review-report"}, base...), "--journal", journal, "--artifacts", artifacts)
	var out, errOut bytes.Buffer
	if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != 0 {
		t.Fatalf("full report%d: %s %s", code, errOut.String(), out.String())
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["matched_fixture_expectations"] != true || report["completed"] != float64(67) || report["missing"] != float64(0) || report["total_charged_micro_usd"] != float64(134) || report["independent_acceptance"] != false || report["publication_authority"] != false || calls != 67 {
		t.Fatalf("native fixture report wrong: %s", out.String())
	}
	if strings.Contains(out.String(), "PRIVATE_FIXTURE_REASON") {
		t.Fatal("report exposed private rationale")
	}
	if cueBin, err := exec.LookPath("cue"); err == nil {
		file := filepath.Join(root, "report.json")
		if err := os.WriteFile(file, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if b, err := exec.Command(cueBin, "vet", "schema.cue", file, "-d", "#ReviewReport", "-c").CombinedOutput(); err != nil {
			t.Fatalf("actual report violates CUE: %v %s", err, b)
		}
	}
	checkReport := func(t *testing.T, codeWant int, counts map[string]float64) {
		t.Helper()
		var out, errOut bytes.Buffer
		if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != codeWant || calls != 67 {
			t.Fatalf("report code=%d want=%d calls=%d: %s %s", code, codeWant, calls, out.String(), errOut.String())
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["matched_fixture_expectations"] != false {
			t.Fatal("bad cohort qualified")
		}
		for key, value := range counts {
			if got[key] != value {
				t.Fatalf("%s=%v want%v", key, got[key], value)
			}
		}
	}
	changeObservation := func(t *testing.T, i int, status string) func() {
		t.Helper()
		outcomeFile := filepath.Join(journal, fmt.Sprintf("outcome-%06d.json", i+1))
		receiptFile := filepath.Join(artifacts, cases[i].ID+".receipt.json")
		originalOutcome, originalReceipt := mustReadDocumentTest(t, outcomeFile), mustReadDocumentTest(t, receiptFile)
		var event, native, receipt map[string]any
		if err := json.Unmarshal(originalOutcome, &event); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(event["response_json"].(string)), &native); err != nil {
			t.Fatal(err)
		}
		native["content"].([]any)[0].(map[string]any)["text"] = string(jsonBytes(map[string]string{"status": status, "reason": "PRIVATE_FIXTURE_REASON"}))
		event["response_json"] = string(jsonBytes(native))
		if err := json.Unmarshal(originalReceipt, &receipt); err != nil {
			t.Fatal(err)
		}
		receipt["status"], receipt["matched_fixture_expectation"] = status, status == cases[i].Expected
		for file, data := range map[string][]byte{outcomeFile: jsonBytes(event), receiptFile: jsonBytes(receipt)} {
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return func() {
			for file, data := range map[string][]byte{outcomeFile: originalOutcome, receiptFile: originalReceipt} {
				if err := os.WriteFile(file, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	t.Run("false-support", func(t *testing.T) {
		defer changeObservation(t, 24, "supported")()
		checkReport(t, 1, map[string]float64{"completed": 67, "false_support": 1, "false_hold": 0, "invalid_count": 0, "total_charged_micro_usd": 134})
	})
	t.Run("always-hold", func(t *testing.T) {
		for i := 0; i < 24; i++ {
			defer changeObservation(t, i, "unsupported")()
		}
		checkReport(t, 1, map[string]float64{"completed": 67, "false_support": 0, "false_hold": 24, "invalid_count": 0})
	})
	t.Run("inconclusive-negative", func(t *testing.T) {
		defer changeObservation(t, 24, "inconclusive")()
		checkReport(t, 1, map[string]float64{"completed": 67, "inconclusive": 1, "false_support": 0, "invalid_count": 0})
	})
	t.Run("pending-reservation-keeps-charge", func(t *testing.T) {
		file := filepath.Join(journal, "outcome-000067.json")
		original := mustReadDocumentTest(t, file)
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.WriteFile(file, original, 0600); err != nil {
				t.Fatal(err)
			}
		}()
		checkReport(t, 2, map[string]float64{"completed": 66, "pending": 1, "missing": 0, "total_charged_micro_usd": 1133})
	})
	// A forged typed artifact cannot replace the corresponding native decision.
	firstReceipt := filepath.Join(artifacts, cases[0].ID+".receipt.json")
	var receipt map[string]any
	if err := json.Unmarshal(mustReadDocumentTest(t, firstReceipt), &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["status"] = "unsupported"
	if err := os.WriteFile(firstReceipt, jsonBytes(receipt), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != 2 || calls != 67 {
		t.Fatalf("forged artifact admitted: %d %s", code, out.String())
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report["matched_fixture_expectations"] != false || report["invalid_count"] != float64(1) || report["missing"] != float64(0) || report["total_charged_micro_usd"] != float64(134) {
		t.Fatalf("invalid attempt hidden or charge lost: %s %v", out.String(), err)
	}
}

func TestOwnerReviewRefusesWeakerScopeBeforeEffects(t *testing.T) {
	original := mustReadDocumentTest(t, "testdata/document-scopes/v2-support-review.json")
	for name, mutate := range map[string]func(map[string]any){
		"omitted-negative": func(v map[string]any) { v["negative_case_ids"] = v["negative_case_ids"].([]any)[1:] },
		"omitted-synthesis": func(v map[string]any) {
			controls := v["positive_controls"].([]any)
			for i, value := range controls {
				if value.(map[string]any)["kind"] == "synthesis" {
					v["positive_controls"] = append(controls[:i], controls[i+1:]...)
					break
				}
			}
		},
		"downgraded-paraphrase": func(v map[string]any) {
			controls := v["positive_controls"].([]any)
			changed, exact := controls[2].(map[string]any), controls[0].(map[string]any)
			changed["kind"], changed["assertion"], changed["fact_ids"] = "exact", exact["assertion"], exact["fact_ids"]
		},
		"null-assertion": func(v map[string]any) { v["positive_controls"].([]any)[0].(map[string]any)["assertion"] = nil },
		"unicode-blank-assertion": func(v map[string]any) {
			v["positive_controls"].([]any)[2].(map[string]any)["assertion"] = "\u3000\u00a0"
		},
		"foreign-field": func(v map[string]any) { v["unknown"] = "never-print-this" },
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(original, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			base, _ := ownerReviewSetup(t, jsonBytes(value)) // New matching study ID/roster defeats stale-pin masking.
			calls := 0
			deps := reviewDeps{apiKey: "offline", captureHTTP: func() (*http.Client, *captureTransport) { calls++; return nil, nil }}
			args := append(append([]string{"review-prepare"}, base...), "--case", "freeform-brief--p-01")
			var out, errOut bytes.Buffer
			if code := runReviewWith(context.Background(), args, &out, &errOut, deps); code != 2 || calls != 0 || strings.Contains(out.String()+errOut.String(), "never-print-this") {
				t.Fatalf("weakened scope admitted or exposed: %d %s %s", code, out.String(), errOut.String())
			}
		})
	}
}

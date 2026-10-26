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
	"sort"
	"testing"
	"time"
)

func testPairedInputs(t *testing.T) (pairedInputs, string, []string) {
	t.Helper()
	corpusPath := "testdata/document-corpus-v2"
	goalPath := "testdata/document-scopes/v2-length-goals.json"
	basePath := "testdata/document-scopes/v2-support-review.json"
	scopePath := "testdata/document-scopes/v2-paired-support-review.json"
	sourcePath := "../../docs/evidence/prose-headless/2026-10-25-long-pilot/regressions.json"
	corpus, err := loadDocumentCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	goalRaw, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	baseRaw, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	scopeRaw, err := os.ReadFile(scopePath)
	if err != nil {
		t.Fatal(err)
	}
	_, old, err := readReviewScope(basePath, hashBytes(baseRaw), corpus, hashBytes(goalRaw))
	if err != nil {
		t.Fatal(err)
	}
	var scope pairedScope
	if err := json.Unmarshal(scopeRaw, &scope); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 70)
	for id := range old {
		ids = append(ids, id)
	}
	for _, x := range scope.Extensions {
		ids = append(ids, x.ID)
	}
	sort.Strings(ids)
	manifest := make([]string, 0, 140)
	for _, id := range ids {
		manifest = append(manifest, pairedExecutionID("baseline", id), pairedExecutionID("candidate", id))
	}
	study := pairedStudy{Version: pairedStudyVersion, ID: "paired-test", DatasetKind: "synthetic", BudgetMicroUSD: 5_000_000, PriorDirectMicroUSD: 0, DirectStageBudgetMicroUSD: 10_000_000, Review: lane{Provider: "anthropic", Model: "claude-test", PricingSource: "synthetic", MaxInputTokens: 100_000, MaxOutputTokens: 256, InputRate: 1, OutputRate: 1, ReserveInputRate: 1, ReserveOutputRate: 1}, Variants: []pairedVariant{{"baseline", reviewProtocol}, {"candidate", pairedCandidateProtocol}}, ExpectedExecutionIDs: manifest}
	b, _ := json.Marshal(study)
	vetPairedOutput(t, "#PairedStudy", b)
	p := filepath.Join(t.TempDir(), "study.json")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--study", p, "--corpus", corpusPath, "--goal-scope", goalPath, "--goal-scope-digest", hashBytes(goalRaw), "--base-review-scope", basePath, "--review-scope", scopePath, "--review-scope-digest", hashBytes(scopeRaw), "--regression-source", sourcePath}
	in, err := loadPairedInputs(p, corpusPath, goalPath, hashBytes(goalRaw), basePath, scopePath, hashBytes(scopeRaw), sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	return in, p, args
}

func TestPairedInputsAndVariantWire(t *testing.T) {
	in, studyPath, base := testPairedInputs(t)
	studyRaw, e := os.ReadFile(studyPath)
	if e != nil {
		t.Fatal(e)
	}
	var study pairedStudy
	if json.Unmarshal(studyRaw, &study) != nil {
		t.Fatal("study")
	}
	study.Review.Provider = "openrouter-jev"
	rejectPairedCUE(t, "#PairedStudy", jsonBytes(study))
	study.Review.Provider = "anthropic"
	study.Review.MaxOutputTokens = 8193
	rejectPairedCUE(t, "#PairedStudy", jsonBytes(study))
	if len(in.cases) != 70 || len(in.ordered) != 140 {
		t.Fatal("manifest shape")
	}
	c := in.cases["technical-design--lp-rhetorical-scope"]
	_, old, err := reviewRequest(context.Background(), c, in.study.Review)
	if err != nil {
		t.Fatal(err)
	}
	_, baseline, err := pairedRequest(context.Background(), c, in.study.Review, "baseline")
	if err != nil {
		t.Fatal(err)
	}
	_, candidate, err := pairedRequest(context.Background(), c, in.study.Review, "candidate")
	if err != nil {
		t.Fatal(err)
	}
	if hashBytes(old) != hashBytes(baseline) || hashBytes(candidate) == hashBytes(baseline) {
		t.Fatal("variant wire")
	}
	var out, errOut bytes.Buffer
	args := append(append([]string{"review-pair-prepare"}, base...), "--case", c.ID, "--variant", "candidate", "--include-content")
	if code := runReviewPairWith(context.Background(), args, &out, &errOut, reviewDeps{}); code != 0 {
		t.Fatalf("content prepare %d %s", code, errOut.String())
	}
	vetPairedOutput(t, "#PairedPrepared", out.Bytes())
}

func TestPairedFirstCaseNativeCaptureAndIncompleteReport(t *testing.T) {
	in, _, base := testPairedInputs(t)
	id := in.ordered[0]
	variant, cid, _ := pairedCaseID(id)
	c := in.cases[cid]
	root := t.TempDir()
	journalPath := filepath.Join(root, "journal")
	artifactPath := filepath.Join(root, "artifacts")
	prepare := append(append([]string{"review-pair-prepare"}, base...), "--case", cid, "--variant", variant)
	var out, errOut bytes.Buffer
	if code := runReviewPairWith(context.Background(), prepare, &out, &errOut, reviewDeps{}); code != 0 {
		t.Fatalf("prepare %d %s", code, errOut.String())
	}
	var p map[string]any
	if json.Unmarshal(out.Bytes(), &p) != nil {
		t.Fatal(out.String())
	}
	vetPairedOutput(t, "#PairedPrepared", out.Bytes())
	out.Reset()
	errOut.Reset()
	deps := reviewDeps{apiKey: "fake", now: func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) }, captureHTTP: func() (*http.Client, *captureTransport) {
		capture := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: ownerReviewTransport(func(req *http.Request) (*http.Response, error) {
			answer := string(jsonBytes(map[string]string{"status": c.Expected, "reason": "PRIVATE_REASON"}))
			native := jsonBytes(map[string]any{"id": "msg-test", "type": "message", "role": "assistant", "model": in.study.Review.Model, "content": []map[string]string{{"type": "text", "text": answer}}, "stop_reason": "end_turn", "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 10}})
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(native)), Request: req}, nil
		})}
		return &http.Client{Transport: capture}, capture
	}}
	capture := append(append([]string{"review-pair-capture"}, base...), "--case", cid, "--variant", variant, "--prepared-digest", p["prepared_digest"].(string), "--journal", journalPath, "--artifacts", artifactPath, "--live")
	if code := runReviewPairWith(context.Background(), capture, &out, &errOut, deps); code != 0 {
		t.Fatalf("capture %d %s %s", code, errOut.String(), out.String())
	}
	vetPairedOutput(t, "#PairedCapture", out.Bytes())
	if bytes.Contains(out.Bytes(), []byte("PRIVATE_REASON")) {
		t.Fatal("private content leaked")
	}
	out.Reset()
	errOut.Reset()
	report := append(append([]string{"review-pair-report"}, base...), "--journal", journalPath, "--artifacts", artifactPath)
	if code := runReviewPairWith(context.Background(), report, &out, &errOut, reviewDeps{}); code != 2 {
		t.Fatalf("report %d %s %s", code, errOut.String(), out.String())
	}
	var got pairedReport
	if json.Unmarshal(out.Bytes(), &got) != nil {
		t.Fatal(fmt.Sprintf("invalid report %s", out.String()))
	}
	vetPairedOutput(t, "#PairedReport", out.Bytes())
	var bad pairedReport
	if json.Unmarshal(out.Bytes(), &bad) != nil {
		t.Fatal("bad report")
	}
	bad.Qualified = true
	rejectPairedCUE(t, "#PairedReport", jsonBytes(bad))
	bad.Total.Complete, bad.Total.Missing = 140, 0
	bad.Baseline.Complete, bad.Baseline.Missing = 70, 0
	bad.Candidate.Complete, bad.Candidate.Missing = 70, 0
	for _, state := range []string{"missing", "invalid", "inconclusive"} {
		bad.Rows[1].State = state
		rejectPairedCUE(t, "#PairedReport", jsonBytes(bad))
	}
	if len(got.Rows) != 140 || got.Rows[0].State != c.Expected || got.Rows[1].State != "missing" {
		t.Fatal("report rows")
	}
	out.Reset()
	errOut.Reset()
	report = append(report, "--include-content")
	if code := runReviewPairWith(context.Background(), report, &out, &errOut, reviewDeps{}); code != 2 {
		t.Fatalf("content report %d %s", code, errOut.String())
	}
	vetPairedOutput(t, "#PairedReport", out.Bytes())
}
func TestPairedArtifactStateFileSets(t *testing.T) {
	rootPath := t.TempDir()
	root, e := os.OpenRoot(rootPath)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	id := "baseline--case-a"
	attempts := []journalAttempt{{Reservation: reservation{CaseID: id}}}
	if e = writeExclusive(root, id+".started.json", map[string]string{"test": "marker"}); e != nil {
		t.Fatal(e)
	}
	if e = pairedCheckArtifactNames(root, attempts); e != nil {
		t.Fatal(e)
	}
	if e = writeExclusive(root, id+".reason.json", map[string]string{"reason": "bogus"}); e != nil {
		t.Fatal(e)
	}
	if pairedCheckArtifactNames(root, attempts) == nil {
		t.Fatal("pending reason accepted")
	}
	if e = root.Remove(id + ".reason.json"); e != nil {
		t.Fatal(e)
	}
	attempts[0].Outcome = &outcome{Status: "unknown"}
	if e = writeExclusive(root, id+".receipt.json", map[string]string{"test": "receipt"}); e != nil {
		t.Fatal(e)
	}
	if e = pairedCheckArtifactNames(root, attempts); e != nil {
		t.Fatal(e)
	}
	if e = writeExclusive(root, id+".reason.json", map[string]string{"reason": "bogus"}); e != nil {
		t.Fatal(e)
	}
	if pairedCheckArtifactNames(root, attempts) == nil {
		t.Fatal("unknown reason accepted")
	}
	attempts[0].Outcome.Status = "completed"
	if e = pairedCheckArtifactNames(root, attempts); e != nil {
		t.Fatal(e)
	}
}
func TestPairedHelpForms(t *testing.T) {
	for _, cmd := range []string{"review-pair-prepare", "review-pair-capture", "review-pair-report"} {
		for _, form := range []string{"--help", "-h", "help"} {
			var out, errOut bytes.Buffer
			if code := runReviewPairWith(context.Background(), []string{cmd, form}, &out, &errOut, reviewDeps{}); code != 0 || out.Len() == 0 || errOut.Len() != 0 {
				t.Fatalf("help %s %s: %d %q %q", cmd, form, code, out.String(), errOut.String())
			}
		}
		var out, errOut bytes.Buffer
		if code := runReviewPairWith(context.Background(), []string{cmd, "--bogus", "--help"}, &out, &errOut, reviewDeps{}); code != 2 || out.Len() != 0 {
			t.Fatalf("unknown flag masked by help %s", cmd)
		}
	}
}
func vetPairedOutput(t *testing.T, definition string, b []byte) {
	t.Helper()
	cue, e := exec.LookPath("cue")
	if e != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "output.json")
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if output, e := exec.Command(cue, "vet", "-c", "-d", definition, "schema.cue", p).CombinedOutput(); e != nil {
		t.Fatalf("CUE %s: %v %s", definition, e, output)
	}
}
func rejectPairedCUE(t *testing.T, definition string, b []byte) {
	t.Helper()
	cue, e := exec.LookPath("cue")
	if e != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "bad.json")
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if output, e := exec.Command(cue, "vet", "-c", "-d", definition, "schema.cue", p).CombinedOutput(); e == nil {
		t.Fatalf("CUE accepted invalid %s: %s", definition, output)
	}
}

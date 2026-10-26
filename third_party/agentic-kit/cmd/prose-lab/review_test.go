package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestReviewAuditMissingAndPending(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if _, _, e := auditJournal(missing, []byte("study"), 100); e == nil {
		t.Fatal("missing journal accepted")
	}
	if _, e := os.Stat(missing); !os.IsNotExist(e) {
		t.Fatal("audit created directory")
	}
	j, path := testJournal(t, 100)
	if _, _, e := auditJournal(path, []byte("study"), 100); e == nil {
		t.Fatal("active writer lock ignored")
	}
	r, e := j.reserve("a", testLane(70), []byte("request"))
	if e != nil {
		t.Fatal(e)
	}
	j.close()
	if _, e = os.Stat(filepath.Join(path, ".lock")); e != nil {
		t.Fatal("crash lock lost")
	}
	if _, _, e = auditJournal(path, []byte("study"), 100); e == nil {
		t.Fatal("crash lock ignored")
	}
	if e = os.Remove(filepath.Join(path, ".lock")); e != nil {
		t.Fatal(e)
	}
	attempts, charged, e := auditJournal(path, []byte("study"), 100)
	if e != nil {
		t.Fatal(e)
	}
	if len(attempts) != 1 || attempts[0].Reservation != r || attempts[0].Outcome != nil || charged != 70 {
		t.Fatal("pending charge missing")
	}
	if _, e = openJournal(path, []byte("study"), 100, time.Now); e == nil {
		t.Fatal("writer resumed pending attempt")
	}
	if _, _, e = auditJournalAfterLock(path, []byte("study"), 100, func(root *os.Root) { _ = root.Remove("study.json") }); e == nil {
		t.Fatal("missing header under lock accepted")
	}
	if _, e = os.Stat(filepath.Join(path, "study.json")); !os.IsNotExist(e) {
		t.Fatal("header was repaired")
	}
}
func TestReviewScopeAndOfflineRequest(t *testing.T) {
	corpus, e := loadDocumentCorpus("testdata/document-corpus-v2")
	if e != nil {
		t.Fatal(e)
	}
	scope, cases, e := readReviewScope("testdata/document-scopes/v2-support-review.json", "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8", corpus, "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403")
	if e != nil {
		t.Fatal(e)
	}
	if scope.ID == "" || len(cases) != 67 {
		t.Fatal("scope roster")
	}
	var c reviewCase
	for _, v := range cases {
		c = v
		break
	}
	l := lane{Provider: "anthropic", Model: "model-x", PricingSource: "test", MaxInputTokens: 200000, MaxOutputTokens: 128, InputRate: 1, OutputRate: 1, ReserveInputRate: 1, ReserveOutputRate: 1}
	_, wire, e := reviewRequest(context.Background(), c, l)
	if e != nil || len(wire) == 0 {
		t.Fatal("offline native request", e)
	}
}

func TestReviewPositiveControlRejectsUnicodeWhitespace(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	path := "testdata/document-scopes/v2-support-review.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var scope reviewScope
	if err := json.Unmarshal(b, &scope); err != nil {
		t.Fatal(err)
	}
	scope.PositiveControls[1].Assertion = "\u00a0\u2003\n"
	bad := jsonBytes(scope)
	badPath := filepath.Join(t.TempDir(), "scope.json")
	if err := os.WriteFile(badPath, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readReviewScope(badPath, hashBytes(bad), corpus, scope.GoalScopeDigest); err == nil {
		t.Fatal("Unicode whitespace-only positive assertion accepted")
	}
}

func TestReviewPrepareAndReportNoEffects(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	goalPin := "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403"
	reviewPin := "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8"
	_, cases, err := readReviewScope("testdata/document-scopes/v2-support-review.json", reviewPin, corpus, goalPin)
	if err != nil {
		t.Fatal(err)
	}
	laneID := "reviewer"
	keys := map[string]string{"protocol": reviewProtocol, "corpus_digest": corpus.Digest, "goal_scope_digest": goalPin, "review_scope_digest": reviewPin, "lane_id": laneID}
	id := "review-" + hashBytes(jsonBytes(keys))
	ids := make([]string, 0, len(cases))
	for k := range cases {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	study := canaryStudy{Version: "prose-lab.canary/1", ID: id, DatasetKind: "synthetic", BudgetMicroUSD: 5000000, Lanes: map[string]lane{laneID: {Provider: "anthropic", Model: "model-x", PricingSource: "test", MaxInputTokens: 200000, MaxOutputTokens: 128, InputRate: 2, OutputRate: 10, ReserveInputRate: 2, ReserveOutputRate: 10}}, ExpectedCaseIDs: ids}
	path := filepath.Join(t.TempDir(), "study.json")
	if err = os.WriteFile(path, jsonBytes(study), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--study", path, "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", goalPin, "--review-scope", "testdata/document-scopes/v2-support-review.json", "--review-scope-digest", reviewPin, "--review-lane", laneID}
	var out, errOut bytes.Buffer
	if code := runLab(context.Background(), append([]string{"review-prepare"}, append(append([]string{}, base...), "--case", ids[0])...), &out, &errOut); code != 0 {
		t.Fatalf("prepare=%d %s", code, errOut.String())
	}
	var prepared map[string]any
	if err = json.Unmarshal(out.Bytes(), &prepared); err != nil {
		t.Fatal(err)
	}
	if prepared["native_request_digest"] == "" || prepared["prepared_digest"] == "" {
		t.Fatal("missing native pin")
	}
	missing := filepath.Join(t.TempDir(), "journal")
	out.Reset()
	errOut.Reset()
	if code := runLab(context.Background(), append([]string{"review-report"}, append(append([]string{}, base...), "--journal", missing)...), &out, &errOut); code != 2 {
		t.Fatalf("missing report=%d", code)
	}
	if _, err = os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("report created journal")
	}
}

func TestReviewReplayNativeResponse(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	_, cases, err := readReviewScope("testdata/document-scopes/v2-support-review.json", "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8", corpus, "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403")
	if err != nil {
		t.Fatal(err)
	}
	var c reviewCase
	for _, v := range cases {
		c = v
		break
	}
	l := lane{Provider: "anthropic", Model: "model-x", PricingSource: "test", MaxInputTokens: 200000, MaxOutputTokens: 128, InputRate: 2, OutputRate: 10, ReserveInputRate: 2, ReserveOutputRate: 10}
	_, wire, err := reviewRequest(context.Background(), c, l)
	if err != nil {
		t.Fatal(err)
	}
	native := []byte(`{"id":"msg_01","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":"{\"status\":\"unsupported\",\"reason\":\"contradiction\"}"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":20}}`)
	got, err := reviewReplay(context.Background(), c, l, wire, native)
	if err != nil || got.Status != "unsupported" {
		t.Fatalf("native replay: %#v %v", got, err)
	}
	wrong := append([]byte(nil), wire...)
	wrong = append(wrong, ' ')
	if _, err = reviewReplay(context.Background(), c, l, wrong, native); err == nil {
		t.Fatal("mismatched native request admitted")
	}
}

type reviewRoundTripFunc func(*http.Request) (*http.Response, error)

func (f reviewRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReviewCaptureNativeJournalOffline(t *testing.T) {
	corpus, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	goalPin := "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403"
	reviewPin := "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8"
	_, cases, err := readReviewScope("testdata/document-scopes/v2-support-review.json", reviewPin, corpus, goalPin)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(cases))
	for id := range cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	selected := "architecture-decision--n-01"
	if cases[selected].Expected != "unsupported" {
		t.Fatal("wrong test case")
	}
	laneID := "reviewer"
	study := canaryStudy{Version: "prose-lab.canary/1", ID: "review-" + hashBytes(jsonBytes(map[string]string{"protocol": reviewProtocol, "corpus_digest": corpus.Digest, "goal_scope_digest": goalPin, "review_scope_digest": reviewPin, "lane_id": laneID})), DatasetKind: "synthetic", BudgetMicroUSD: 5000000, Lanes: map[string]lane{laneID: {Provider: "anthropic", Model: "model-x", PricingSource: "test", MaxInputTokens: 200000, MaxOutputTokens: 128, InputRate: 2, OutputRate: 10, ReserveInputRate: 2, ReserveOutputRate: 10}}, ExpectedCaseIDs: ids}
	path := filepath.Join(t.TempDir(), "study.json")
	if err = os.WriteFile(path, jsonBytes(study), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--study", path, "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", goalPin, "--review-scope", "testdata/document-scopes/v2-support-review.json", "--review-scope-digest", reviewPin, "--review-lane", laneID, "--case", selected}
	var out, errOut bytes.Buffer
	if code := runReviewWith(context.Background(), append([]string{"review-prepare"}, base...), &out, &errOut, reviewDeps{}); code != 0 {
		t.Fatalf("prepare %d %s", code, errOut.String())
	}
	var prepared struct {
		Digest string `json:"prepared_digest"`
	}
	if err = json.Unmarshal(out.Bytes(), &prepared); err != nil || prepared.Digest == "" {
		t.Fatal("prepared digest", err)
	}
	native := []byte(`{"id":"msg_01","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":"{\"status\":\"unsupported\",\"reason\":\"contradiction\"}"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":20}}`)
	calls := 0
	deps := reviewDeps{now: func() time.Time { return time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) }, apiKey: "offline-dummy", captureHTTP: func() (*http.Client, *captureTransport) {
		cap := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: reviewRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(native)), Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}, nil
		})}
		return &http.Client{Transport: cap}, cap
	}}
	journal := filepath.Join(t.TempDir(), "journal")
	artifacts := filepath.Join(t.TempDir(), "artifacts")
	args := append(append([]string{"review-capture"}, base...), "--prepared-digest", prepared.Digest, "--journal", journal, "--artifacts", artifacts, "--live")
	out.Reset()
	errOut.Reset()
	if code := runReviewWith(context.Background(), args, &out, &errOut, deps); code != 0 {
		t.Fatalf("capture %d %s %s", code, errOut.String(), out.String())
	}
	if calls != 1 {
		t.Fatalf("native calls=%d", calls)
	}
	var receipt reviewReceipt
	if err = json.Unmarshal(out.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "unsupported" || !receipt.Matched || receipt.CumulativeChargedMicroUSD == nil || *receipt.CumulativeChargedMicroUSD != 2 {
		t.Fatalf("receipt=%+v", receipt)
	}
	in, err := loadReviewInputs(path, "testdata/document-corpus-v2", "testdata/document-scopes/v2-length-goals.json", goalPin, "testdata/document-scopes/v2-support-review.json", reviewPin, laneID)
	if err != nil {
		t.Fatal(err)
	}
	attempts, charged, err := auditJournal(journal, in.boundStudy, in.study.BudgetMicroUSD)
	if err != nil || len(attempts) != 1 || charged != 2 || attempts[0].Outcome == nil || attempts[0].Outcome.HTTPStatus != 200 {
		t.Fatalf("journal %d %d %v", len(attempts), charged, err)
	}
	out.Reset()
	errOut.Reset()
	if code := runReviewWith(context.Background(), args, &out, &errOut, deps); code != 2 || calls != 1 {
		t.Fatalf("replay code=%d calls=%d", code, calls)
	}
	// A wrapper changes valid JSON before the capture boundary sees it.
	mutatedJournal := filepath.Join(t.TempDir(), "journal")
	mutatedArtifacts := filepath.Join(t.TempDir(), "artifacts")
	innerCalls := 0
	mutatedDeps := reviewDeps{now: deps.now, apiKey: "offline-dummy", captureHTTP: func() (*http.Client, *captureTransport) {
		cap := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: reviewRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			innerCalls++
			return nil, nil
		})}
		wrapper := reviewRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			body = append(body, ' ')
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			return cap.RoundTrip(r)
		})
		return &http.Client{Transport: wrapper}, cap
	}}
	mutatedArgs := append(append([]string{"review-capture"}, base...), "--prepared-digest", prepared.Digest, "--journal", mutatedJournal, "--artifacts", mutatedArtifacts, "--live")
	out.Reset()
	errOut.Reset()
	if code := runReviewWith(context.Background(), mutatedArgs, &out, &errOut, mutatedDeps); code != 2 || innerCalls != 0 {
		t.Fatalf("mutated wire code=%d inner=%d %s", code, innerCalls, errOut.String())
	}
	var held reviewReceipt
	if err := json.Unmarshal(out.Bytes(), &held); err != nil || held.Status != "held" || held.Matched || held.ErrorClass != "review_incomplete" {
		t.Fatalf("mutated receipt=%+v err=%v", held, err)
	}
	mutatedAttempts, mutatedCharge, err := auditJournal(mutatedJournal, in.boundStudy, in.study.BudgetMicroUSD)
	if err != nil || len(mutatedAttempts) != 0 || mutatedCharge != 0 {
		t.Fatalf("mutated journal attempts=%d charge=%d err=%v", len(mutatedAttempts), mutatedCharge, err)
	}
	// A settled failure has an outcome. Only a missing outcome is pending.
	var original outcome
	outcomePath := filepath.Join(journal, "outcome-000001.json")
	if b, err := os.ReadFile(outcomePath); err != nil || json.Unmarshal(b, &original) != nil {
		t.Fatal("read outcome", err)
	}
	for _, status := range []string{"failed", "unknown"} {
		changed := original
		changed.Status = status
		changed.UsagePresent = false
		changed.ActualMicroUSD = nil
		changed.Response = ""
		if err := os.WriteFile(outcomePath, jsonBytes(changed), 0600); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		errOut.Reset()
		reportArgs := append(append([]string{"review-report"}, base[:len(base)-2]...), "--journal", journal, "--artifacts", artifacts)
		if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != 2 {
			t.Fatalf("%s report code=%d %s", status, code, errOut.String())
		}
		var report reviewReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Pending != 0 || report.Completed != 0 || report.Failed+report.Unknown != 1 || (status == "failed" && report.Failed != 1) || (status == "unknown" && report.Unknown != 1) || report.MatchedFixtureExpectations {
			t.Fatalf("%s report=%+v", status, report)
		}
	}
	if err := os.Remove(outcomePath); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	reportArgs := append(append([]string{"review-report"}, base[:len(base)-2]...), "--journal", journal, "--artifacts", artifacts)
	if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != 2 {
		t.Fatalf("pending report code=%d %s", code, errOut.String())
	}
	var pendingReport reviewReport
	if err := json.Unmarshal(out.Bytes(), &pendingReport); err != nil || pendingReport.Pending != 1 || pendingReport.Failed != 0 || pendingReport.Unknown != 0 {
		t.Fatalf("pending report=%+v err=%v", pendingReport, err)
	}
}

func TestReviewArtifactFailureStopsCohortAndReportsCharge(t *testing.T) {
	corpus, e := loadDocumentCorpus("testdata/document-corpus-v2")
	if e != nil {
		t.Fatal(e)
	}
	goalPin := "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403"
	reviewPin := "fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8"
	_, cases, e := readReviewScope("testdata/document-scopes/v2-support-review.json", reviewPin, corpus, goalPin)
	if e != nil {
		t.Fatal(e)
	}
	ids := make([]string, 0, len(cases))
	for id := range cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	first, second := "architecture-decision--n-01", "architecture-decision--n-02"
	laneID := "reviewer"
	study := canaryStudy{Version: "prose-lab.canary/1", ID: "review-" + hashBytes(jsonBytes(map[string]string{"protocol": reviewProtocol, "corpus_digest": corpus.Digest, "goal_scope_digest": goalPin, "review_scope_digest": reviewPin, "lane_id": laneID})), DatasetKind: "synthetic", BudgetMicroUSD: 5000000, Lanes: map[string]lane{laneID: {Provider: "anthropic", Model: "model-x", PricingSource: "test", MaxInputTokens: 200000, MaxOutputTokens: 128, InputRate: 2, OutputRate: 10, ReserveInputRate: 2, ReserveOutputRate: 10}}, ExpectedCaseIDs: ids}
	studyPath := filepath.Join(t.TempDir(), "study.json")
	if e = os.WriteFile(studyPath, jsonBytes(study), 0600); e != nil {
		t.Fatal(e)
	}
	base := []string{"--study", studyPath, "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", goalPin, "--review-scope", "testdata/document-scopes/v2-support-review.json", "--review-scope-digest", reviewPin, "--review-lane", laneID}
	prepare := func(id string) string {
		t.Helper()
		var out, errOut bytes.Buffer
		args := append(append([]string{"review-prepare"}, base...), "--case", id)
		if code := runReviewWith(context.Background(), args, &out, &errOut, reviewDeps{}); code != 0 {
			t.Fatalf("prepare %d %s", code, errOut.String())
		}
		var x struct {
			Digest string `json:"prepared_digest"`
		}
		if e := json.Unmarshal(out.Bytes(), &x); e != nil {
			t.Fatal(e)
		}
		return x.Digest
	}
	firstPin, secondPin := prepare(first), prepare(second)
	journal := filepath.Join(t.TempDir(), "journal")
	artifacts := filepath.Join(t.TempDir(), "artifacts")
	native := []byte(`{"id":"msg_01","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":"{\"status\":\"unsupported\",\"reason\":\"contradiction\"}"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":20}}`)
	calls := 0
	deps := reviewDeps{apiKey: "offline-dummy", now: func() time.Time { return time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) }, captureHTTP: func() (*http.Client, *captureTransport) {
		cap := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: reviewRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				if e := os.WriteFile(filepath.Join(artifacts, first+".reason.json"), []byte(`{"reason":"forged"}`), 0600); e != nil {
					t.Fatal(e)
				}
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(native)), Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}, nil
		})}
		return &http.Client{Transport: cap}, cap
	}}
	capture := func(id, pin string) int {
		t.Helper()
		var out, errOut bytes.Buffer
		args := append(append([]string{"review-capture"}, base...), "--case", id, "--prepared-digest", pin, "--journal", journal, "--artifacts", artifacts, "--live")
		return runReviewWith(context.Background(), args, &out, &errOut, deps)
	}
	if code := capture(first, firstPin); code != 2 || calls != 1 {
		t.Fatalf("first code=%d calls=%d", code, calls)
	}
	var held reviewReceipt
	b, e := os.ReadFile(filepath.Join(artifacts, first+".receipt.json"))
	if e == nil {
		e = json.Unmarshal(b, &held)
	}
	if e != nil || held.Status != "held" || held.Matched || held.ErrorClass != "reason_write_failed" {
		t.Fatalf("reason failure receipt=%+v err=%v", held, e)
	}
	if code := capture(second, secondPin); code != 2 || calls != 1 {
		t.Fatalf("second code=%d calls=%d", code, calls)
	}
	var out, errOut bytes.Buffer
	reportArgs := append(append([]string{"review-report"}, base...), "--journal", journal, "--artifacts", artifacts)
	if code := runReviewWith(context.Background(), reportArgs, &out, &errOut, reviewDeps{}); code != 2 {
		t.Fatalf("report code=%d %s", code, errOut.String())
	}
	var report reviewReport
	if e = json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if report.InvalidCount != 1 || report.TotalChargedMicroUSD != 2 || report.Cases[0].Status != "invalid" {
		t.Fatalf("report invalid=%d charge=%d first=%+v", report.InvalidCount, report.TotalChargedMicroUSD, report.Cases[0])
	}
}

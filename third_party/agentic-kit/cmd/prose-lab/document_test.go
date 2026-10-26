package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

func documentTestStudy(t *testing.T, c documentCorpus) string {
	t.Helper()
	l := testLane(100000)
	l.Provider = "anthropic"
	l.Model = "synthetic-generator"
	l.MaxOutputTokens = 4096
	l.OutputRate = 1000
	l.ReserveOutputRate = 1000
	r := l
	r.Model = "synthetic-reviewer"
	r.MaxOutputTokens = 256
	study := canaryStudy{Version: "prose-lab.canary/1", ID: "synthetic-document", DatasetKind: "synthetic", BudgetMicroUSD: 10_000_000, Lanes: map[string]lane{"generation": l, "review": r}, ExpectedCaseIDs: c.sortedIDs()}
	path := filepath.Join(t.TempDir(), "study.json")
	if e := os.WriteFile(path, jsonBytes(study), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func documentArgs(study, corpus, cell string) []string {
	return []string{"document-prepare", "--study", study, "--corpus", corpus, "--cell", cell, "--generation-lane", "generation", "--review-lane", "review"}
}
func TestDocumentCorpusAndPreflight(t *testing.T) {
	corpusPath := "testdata/document-corpus"
	c, e := loadDocumentCorpus(corpusPath)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Fixtures) != 6 || len(c.Manifest.Cells) != 36 || documentNegativeCount(c) < 36 {
		t.Fatal("corpus count")
	}
	study := documentTestStudy(t, c)
	args := documentArgs(study, corpusPath, c.Manifest.Cells[0].ID)
	var out, errOut bytes.Buffer
	if code := runDocument(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("prepare %d %s", code, errOut.String())
	}
	var prepared map[string]any
	if e = json.Unmarshal(out.Bytes(), &prepared); e != nil || prepared["corpus_digest"] != c.Digest {
		t.Fatal("missing digest")
	}
	if strings.Contains(out.String(), "fictional") || strings.Contains(out.String(), "Proposal:") {
		t.Fatal("source text printed")
	}
	// A changed fact changes the bound corpus digest before a provider or journal can be opened.
	temp := t.TempDir()
	for _, f := range append([]string{"manifest.json"}, []string{"architecture-decision.json", "draft-service-agreement.json", "freeform-brief.json", "incident-summary.json", "operational-runbook.json", "technical-design.json"}...) {
		b, e := os.ReadFile(filepath.Join(corpusPath, f))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(temp, f), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	changed, e := loadDocumentCorpus(temp)
	if e != nil {
		t.Fatal(e)
	}
	if changed.Digest != c.Digest {
		t.Fatal("copy digest changed")
	}
	f := filepath.Join(temp, "freeform-brief.json")
	b, _ := os.ReadFile(f)
	b = bytes.Replace(b, []byte("Orchard Reading Room"), []byte("Orchard Study Room"), 1)
	if e = os.WriteFile(f, b, 0600); e != nil {
		t.Fatal(e)
	}
	changed, e = loadDocumentCorpus(temp)
	if e != nil || changed.Digest == c.Digest {
		t.Fatal("fixture change did not alter binding")
	}
	root := t.TempDir()
	journal := filepath.Join(root, "journal")
	artifacts := filepath.Join(root, "artifacts")
	capture := documentArgs(study, temp, c.Manifest.Cells[0].ID)
	capture[0] = "document-capture"
	capture = append(capture, "--prepared-digest", documentPreparedDigest(hashBytes(documentStudyBytes(mustReadDocumentTest(t, study), c.Digest)), c.Manifest.Cells[0].ID, "generation", "review"), "--journal", journal, "--artifacts", artifacts, "--live")
	spy := &documentSpy{}
	if code := runDocumentWith(context.Background(), capture, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}); code == 0 || spy.calls != 0 {
		t.Fatal("changed corpus accepted under old pin")
	}
	for _, path := range []string{journal, artifacts} {
		if _, e := os.Stat(path); !os.IsNotExist(e) {
			t.Fatalf("changed corpus created %s before pin", path)
		}
	}
}
func TestDocumentStrictInputsAndNoLiveNoCalls(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	base := documentArgs(study, "testdata/document-corpus", c.Manifest.Cells[0].ID)
	spy := &documentSpy{}
	for _, args := range [][]string{append([]string{}, base[:len(base)-1]...), append([]string{"document-capture"}, base[1:]...)} {
		if code := runDocumentWith(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}); code == 0 {
			t.Fatal("invalid command passed")
		}
	}
	if spy.calls != 0 {
		t.Fatal("provider called without live")
	}
	d := c.Fixtures["freeform-brief"]
	raw := jsonBytes(d)
	raw = bytes.Replace(raw, []byte(`"purpose":`), []byte(`"unknown":1,"purpose":`), 1)
	if _, e = decodeDocumentFixture(raw); e == nil {
		t.Fatal("unknown fixture field passed")
	}
	raw = jsonBytes(d)
	raw = bytes.Replace(raw, []byte(`"required":true`), []byte(`"missing":true`), 1)
	if _, e = decodeDocumentFixture(raw); e == nil {
		t.Fatal("missing required nested field passed")
	}
}

type documentSpy struct {
	calls    int
	response llmclient.Response
}

func (s *documentSpy) Complete(_ context.Context, _ llmclient.Request) (llmclient.Response, error) {
	s.calls++
	return s.response, nil
}
func (s *documentSpy) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, nil
}
func TestDocumentReviewHoldsMalformedAndTruncated(t *testing.T) {
	d := documentFixture{Facts: []documentFact{{ID: "f-01", Text: "Observed detail", Kind: "observed"}}}
	cell := documentCell{ID: "test-executive-r1", DocumentID: "test", Audience: "executive", Repeat: 1}

	for _, r := range []llmclient.Response{{Text: `{"status":"supported"}`, StopReason: llmclient.StopEndTurn}, {Text: `{"status":"supported","reason":"ok"}`, StopReason: llmclient.StopReason("max_tokens")}} {
		spy := &documentSpy{response: r}
		judge := documentReviewer{client: spy, model: "review", facts: map[string]string{"f-01": "Observed detail"}, reviewer: "host", maxTokens: 256}
		_, e := judge.Review(context.Background(), prose.SupportReview{Claim: &prose.ProseClaim{Assertion: "Observed detail", FactIDs: []string{"f-01"}}, Facts: []prose.Fact{}})
		if e == nil || spy.calls != 1 {
			t.Fatalf("review failure not held: %v", e)
		}
	}
	_, _ = d, cell
}
func TestDocumentTargetBuildsSyntheticSources(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	cell, _ := c.cell("freeform-brief-executive-r1")
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	target, sources, e := documentTarget(c.Fixtures[cell.DocumentID], cell, now)
	if e != nil {
		t.Fatal(e)
	}
	if len(target.Input.Facts) != len(c.Fixtures[cell.DocumentID].Facts) || len(target.Spec.Nodes) != 1+2*len(c.Fixtures[cell.DocumentID].Sections) {
		t.Fatal("target shape")
	}
	for _, f := range target.Input.Facts {
		snapshot, e := sources.Resolve(context.Background(), target.Input.Principal, f.Source)
		if e != nil || snapshot.AcquiredAt != now || snapshot.Access != "public" {
			t.Fatal("source acquisition")
		}
	}
}

type documentGenerator struct{ calls int }

func (g *documentGenerator) Complete(_ context.Context, r llmclient.Request) (llmclient.Response, error) {
	g.calls++
	var p struct {
		Facts []struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"facts"`
	}
	if e := json.Unmarshal([]byte(r.Prompt), &p); e != nil {
		return llmclient.Response{}, e
	}
	segments := make([]map[string]any, 0, len(p.Facts))
	for _, f := range p.Facts {
		segments = append(segments, map[string]any{"kind": "assertion", "text": f.Value, "fact_ids": []string{f.ID}, "contradictions": []string{}, "reason": ""})
	}
	b, _ := json.Marshal(map[string]any{"segments": segments})
	return llmclient.Response{Text: string(b), StopReason: llmclient.StopEndTurn}, nil
}
func (g *documentGenerator) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, nil
}

type documentSupport struct {
	calls   int
	maxSeen int
}

func (s *documentSupport) Complete(_ context.Context, req llmclient.Request) (llmclient.Response, error) {
	s.calls++
	s.maxSeen = req.MaxTokens
	return llmclient.Response{Text: `{"status":"supported","reason":"Fixture assertion matches its cited fact."}`, StopReason: llmclient.StopEndTurn}, nil
}
func (s *documentSupport) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, nil
}
func TestDocumentEngineFakeCandidate(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	cell, _ := c.cell("freeform-brief-technical-r1")
	gen := &documentGenerator{}
	rev := &documentSupport{}
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	engine, target, e := documentEngine(gen, rev, "gen-model", "rev-model", 256, c.Fixtures[cell.DocumentID], cell, now)
	if e != nil {
		t.Fatal(e)
	}
	v, e := engine.Generate(context.Background(), target)
	if e != nil {
		t.Fatal(e)
	}
	rendered, e := engine.Render(context.Background(), v)
	if e != nil {
		t.Fatal(e)
	}
	if e = documentCoverage(c.Fixtures[cell.DocumentID], v, rendered); e != nil {
		t.Fatal(e)
	}
	if gen.calls != len(c.Fixtures[cell.DocumentID].Sections) || rev.calls < gen.calls {
		t.Fatalf("generation=%d review=%d", gen.calls, rev.calls)
	}
}

func TestDocumentPreparedDigestRequiredBeforeEffects(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	base := documentArgs(study, "testdata/document-corpus", c.Manifest.Cells[0].ID)
	base[0] = "document-capture"
	pin := documentPreparedDigest(hashBytes(documentStudyBytes(mustReadDocumentTest(t, study), c.Digest)), c.Manifest.Cells[0].ID, "generation", "review")
	spy := &documentSpy{}
	for _, test := range []struct{ name, pin string }{{"missing", ""}, {"changed", strings.Repeat("a", 64)}, {"malformed", "ABC"}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			journal := filepath.Join(root, "journal")
			artifacts := filepath.Join(root, "artifacts")
			args := append(append([]string{}, base...), "--journal", journal, "--artifacts", artifacts, "--live")
			if test.pin != "" {
				args = append(args, "--prepared-digest", test.pin)
			}
			if code := runDocumentWith(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}); code == 0 {
				t.Fatal("pin accepted")
			}
			for _, path := range []string{journal, artifacts} {
				if _, e := os.Stat(path); !os.IsNotExist(e) {
					t.Fatalf("created %s before pin", path)
				}
			}
		})
	}
	if spy.calls != 0 {
		t.Fatal("provider called")
	}
	if !documentDigestPattern.MatchString(pin) {
		t.Fatal("valid pin shape")
	}
}
func mustReadDocumentTest(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestDocumentModelAliasCollision(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	var s canaryStudy
	if _, e = readStudyJSON(study, &s); e != nil {
		t.Fatal(e)
	}
	g := s.Lanes["generation"]
	r := s.Lanes["review"]
	g.ResolvedModel = r.Model
	s.Lanes["generation"] = g
	collision := filepath.Join(t.TempDir(), "collision.json")
	if e = os.WriteFile(collision, jsonBytes(s), 0600); e != nil {
		t.Fatal(e)
	}
	args := documentArgs(collision, "testdata/document-corpus", c.Manifest.Cells[0].ID)
	if code := runDocument(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
		t.Fatal("alias collision accepted")
	}
	if documentModelSetsDisjoint(g, r) {
		t.Fatal("alias helper accepted collision")
	}
}
func TestDocumentRequiredNullsRefused(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	d := c.Fixtures["freeform-brief"]
	raw := jsonBytes(d)
	raw = bytes.Replace(raw, []byte(`"required":true`), []byte(`"required":null`), 1)
	if _, e = decodeDocumentFixture(raw); e == nil {
		t.Fatal("null required flag accepted")
	}
	raw = jsonBytes(d)
	var object map[string]any
	if e = json.Unmarshal(jsonBytes(d), &object); e != nil {
		t.Fatal(e)
	}
	object["sections"].([]any)[0].(map[string]any)["fact_ids"] = nil
	raw, _ = json.Marshal(object)
	if _, e = decodeDocumentFixture(raw); e == nil {
		t.Fatal("null fact array accepted")
	}
	raw = jsonBytes(c.Manifest)
	raw = bytes.Replace(raw, []byte(`"repeats":3`), []byte(`"repeats":null`), 1)
	if e = exactObject(raw, "version", "audiences", "repeats", "cells"); e == nil {
		t.Fatal("null repeat accepted")
	}
}
func TestDocumentHeldReceiptAndNoReplay(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	cell := c.Manifest.Cells[0].ID
	pin := documentPreparedDigest(hashBytes(documentStudyBytes(mustReadDocumentTest(t, study), c.Digest)), c.Manifest.Cells[0].ID, "generation", "review")
	root := t.TempDir()
	journal := filepath.Join(root, "journal")
	artifacts := filepath.Join(root, "artifacts")
	args := documentArgs(study, "testdata/document-corpus", cell)
	args[0] = "document-capture"
	args = append(args, "--prepared-digest", pin, "--journal", journal, "--artifacts", artifacts, "--live")
	bad := &documentSpy{response: llmclient.Response{Text: `{}`, StopReason: llmclient.StopEndTurn}}
	deps := documentDeps{generation: bad, review: bad, now: func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }}
	if code := runDocumentWith(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps); code == 0 {
		t.Fatal("malformed generation accepted")
	}
	var receipt documentRunReceipt
	b, e := os.ReadFile(filepath.Join(artifacts, cell+".receipt.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &receipt); e != nil || receipt.Status != "held" || receipt.ErrorClass != "generation_or_support_held" {
		t.Fatalf("held receipt: %s %v", b, e)
	}
	calls := bad.calls
	if code := runDocumentWith(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps); code == 0 || bad.calls != calls {
		t.Fatal("replay admitted")
	}
}

func TestDocumentSelectedPinRejectsOtherCellOrLaneBeforeEffects(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	var s canaryStudy
	if _, e = readStudyJSON(study, &s); e != nil {
		t.Fatal(e)
	}
	alternate := s.Lanes["generation"]
	alternate.Model = "synthetic-generator-alt"
	s.Lanes["generation-alt"] = alternate
	if e = os.WriteFile(study, jsonBytes(s), 0600); e != nil {
		t.Fatal(e)
	}
	common := hashBytes(documentStudyBytes(mustReadDocumentTest(t, study), c.Digest))
	pin := documentPreparedDigest(common, c.Manifest.Cells[0].ID, "generation", "review")
	for _, tc := range []struct{ name, cell, gen string }{{"other-cell", c.Manifest.Cells[1].ID, "generation"}, {"other-lane", c.Manifest.Cells[0].ID, "generation-alt"}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			journal := filepath.Join(root, "journal")
			artifacts := filepath.Join(root, "artifacts")
			args := []string{"document-capture", "--study", study, "--corpus", "testdata/document-corpus", "--cell", tc.cell, "--generation-lane", tc.gen, "--review-lane", "review", "--prepared-digest", pin, "--journal", journal, "--artifacts", artifacts, "--live"}
			spy := &documentSpy{}
			if code := runDocumentWith(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}); code == 0 || spy.calls != 0 {
				t.Fatal("foreign selected pin accepted")
			}
			for _, path := range []string{journal, artifacts} {
				if _, e := os.Stat(path); !os.IsNotExist(e) {
					t.Fatalf("pin created %s", path)
				}
			}
		})
	}
}
func TestDocumentChargesTwoCellsOneJournal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	common := []byte("common-36-cell-study")
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	l := testLane(100)
	var cumulative int64
	for i, tc := range []struct {
		cell   string
		tokens int64
	}{{"cell-a", 5}, {"cell-b", 7}} {
		j, e := openJournal(root, common, 1000, now)
		if e != nil {
			t.Fatal(e)
		}
		start := j.charged
		if start != cumulative {
			t.Fatalf("open charge %d want %d", start, cumulative)
		}
		r, e := j.reserve(tc.cell, l, []byte("request"))
		if e != nil {
			t.Fatal(e)
		}
		response := fmt.Sprintf(`{"model":"model","usage":{"input_tokens":%d,"output_tokens":0}}`, tc.tokens)
		model, _, cost, e := readUsage([]byte(response), l)
		if e != nil {
			t.Fatal(e)
		}
		if e = j.finish(r, outcome{Status: "completed", Model: model, UsagePresent: true, ActualMicroUSD: &cost, Response: response}); e != nil {
			t.Fatal(e)
		}
		total, delta := documentCharges(j, start)
		if total != cumulative+cost || delta != cost {
			t.Fatalf("cell %d total=%d delta=%d cost=%d", i, total, delta, cost)
		}
		cumulative = total
		if e = j.close(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestDocumentAuthoredOccurrenceSurvivesRunClocks(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	cell, _ := c.cell("incident-summary-executive-r1")
	authored := time.Date(2026, 3, 4, 9, 10, 0, 0, time.UTC)
	for _, clock := range []time.Time{time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), time.Date(2026, 10, 25, 18, 30, 0, 0, time.UTC)} {
		target, sources, e := documentTarget(c.Fixtures[cell.DocumentID], cell, clock)
		if e != nil {
			t.Fatal(e)
		}
		var found bool
		for _, f := range target.Input.Facts {
			if f.Span.ID != "f-01" {
				continue
			}
			found = true
			if !f.Span.OccurredAt.Equal(authored) || !f.Span.ObservedAt.Equal(clock) {
				t.Fatalf("time drift: occurred=%s observed=%s", f.Span.OccurredAt, f.Span.ObservedAt)
			}
			snap, e := sources.Resolve(context.Background(), target.Input.Principal, f.Source)
			if e != nil || !snap.AcquiredAt.Equal(clock) {
				t.Fatalf("source acquisition %s %v", snap.AcquiredAt, e)
			}
		}
		if !found {
			t.Fatal("incident f-01 absent")
		}
	}
	d := c.Fixtures[cell.DocumentID]
	d.Facts[0].OccurredAt = "2027-03-04T09:10:00Z"
	if _, _, e := documentTarget(d, cell, time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)); e == nil {
		t.Fatal("future authored event admitted")
	}
	for _, bad := range []string{"", "2026-03-04T09:10:00+00:00", "2026-02-30T09:10:00Z", "0001-01-01T00:00:00Z"} {
		if _, e := parseDocumentOccurredAt(bad); e == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestDocumentPreparationMarksNegativeCasesUnevaluated(t *testing.T) {
	c, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	study := documentTestStudy(t, c)
	args := documentArgs(study, "testdata/document-corpus", c.Manifest.Cells[0].ID)
	var out, errOut bytes.Buffer
	if code := runDocument(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatal(errOut.String())
	}
	var payload map[string]any
	if e = json.Unmarshal(out.Bytes(), &payload); e != nil {
		t.Fatal(e)
	}
	if payload["available_negative_cases"] != float64(documentNegativeCount(c)) || payload["negative_cases_evaluated"] != false {
		t.Fatalf("negative metadata %v", payload)
	}
	if _, ok := payload["negative_cases"]; ok {
		t.Fatal("ambiguous negative_cases field retained")
	}
}

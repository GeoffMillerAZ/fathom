package governedprosekit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

var sourcedEpoch = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

type testSources struct {
	mu      sync.Mutex
	items   map[SourceRef]SourceSnapshot
	current map[string]SourceRef
	deny    bool
}

func (s *testSources) Resolve(ctx context.Context, principal string, ref SourceRef) (SourceSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deny || principal != "reader" {
		return SourceSnapshot{}, ErrEvidence
	}
	v, ok := s.items[ref]
	if !ok {
		return SourceSnapshot{}, ErrEvidence
	}
	return v, ctx.Err()
}

func (s *testSources) Current(ctx context.Context, principal, id string) (SourceRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deny || principal != "reader" {
		return SourceRef{}, ErrEvidence
	}
	v, ok := s.current[id]
	if !ok {
		return SourceRef{}, ErrEvidence
	}
	return v, ctx.Err()
}

func sourcedFixture(t *testing.T, values ...string) (*testSources, SourcedRequest, ProseDraft) {
	t.Helper()
	s := &testSources{items: map[SourceRef]SourceSnapshot{}, current: map[string]SourceRef{}}
	req := SourcedRequest{Principal: "reader", Topic: "verified facts"}
	draft := ProseDraft{}
	for i, value := range values {
		id := string(rune('a' + i))
		ref := SourceRef{ID: "source-" + id, Version: "v1", Digest: ContentDigest(value)}
		s.items[ref] = SourceSnapshot{Ref: ref, Content: value, MediaType: "text/plain", AcquiredBy: "fixture", AcquiredAt: sourcedEpoch.Add(-time.Hour), Access: "public", RetainUntil: sourcedEpoch.Add(time.Hour)}
		s.current[ref.ID] = ref
		span := evidencespan.Span{ID: "fact-" + id, EntityID: "entity-" + id, Field: "result", Value: value, Confidence: 1, SourceID: ref.ID, RecordID: ref.Version, RecordHash: ref.Digest, OccurredAt: sourcedEpoch.Add(-2 * time.Hour), ObservedAt: sourcedEpoch.Add(-time.Hour), Locator: evidencespan.Locator{Kind: "byte-range", Value: ref.ID, Start: 0, End: len(value) - 1}, Author: aiprovenance.NewHuman("fixture-reviewer"), Grade: &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 1}}
		req.Facts = append(req.Facts, Fact{Span: span, Source: ref, Unit: "items"})
		claim := ProseClaim{ID: "claim-" + id, Assertion: value, FactIDs: []string{span.ID}, Contradictions: []string{}}
		draft.Claims = append(draft.Claims, claim)
		draft.Units = append(draft.Units, ProseUnit{ID: "unit-" + id, Text: value, Claims: []ClaimRange{{ClaimID: claim.ID, Start: 0, End: len(value)}}})
	}
	return s, req, draft
}

func sourcedPipeline(t *testing.T, s SourceProvider, reviewer SupportReviewer, draft ProseDraft) (*SourcedPipeline, *llmclient.MockClient) {
	t.Helper()
	raw, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	mock := llmclient.NewMockClient(llmclient.MockEntryOK(string(raw)))
	p, err := NewSourcedPipeline(mock, s, reviewer, SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "fixture-generator", Limits: DefaultProseLimits()}, func() time.Time { return sourcedEpoch })
	if err != nil {
		t.Fatal(err)
	}
	return p, mock
}

func allowPublication(ctx context.Context, principal string, receipt ProseReceipt, audience string) error {
	if principal != "reader" || receipt.Integrity != "pass" || receipt.ArtifactDigest == "" || receipt.EvidenceDigest == "" {
		return ErrArtifact
	}
	return ctx.Err()
}

func TestSourcedGenerationRejectsUnsupportedClaims(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12", "10")
	req.Facts[1].Historical = true
	draft.Claims = []ProseClaim{{ID: "claim", Assertion: "14", FactIDs: []string{"fact-a", "fact-b"}, Contradictions: []string{}}}
	draft.Units = []ProseUnit{{ID: "unit", Text: "14", Claims: []ClaimRange{{ClaimID: "claim", End: 2}}}}
	p, mock := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported 14 passed: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Fatal("generation attempt count")
	}
	prompt := mock.Calls()[0].Request.Prompt
	if !strings.Contains(prompt, `"Value":"12"`) || !strings.Contains(prompt, `"Value":"10"`) {
		t.Fatal("generator did not receive actual evidence")
	}
}

func TestSourcedGenerateAndExport(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12", "Café results")
	p, mock := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Receipt.CheckedAt.Equal(sourcedEpoch) || a.Receipt.Generation != "model" || len(a.Receipt.Support) != 2 {
		t.Fatalf("receipt: %+v", a.Receipt)
	}
	if mock.Calls()[0].Request.MaxTokens != DefaultProseLimits().MaxTokens {
		t.Fatal("unbounded generation")
	}
	x, err := p.Export(context.Background(), a, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	if x.Markdown != "12\n\nCafé results" || x.MarkdownDigest != ContentDigest(x.Markdown) || len(x.SourceMap) != 2 {
		t.Fatalf("export: %+v", x)
	}
	for i, entry := range x.SourceMap {
		if x.Markdown[entry.Start:entry.End] != draft.Units[i].Text || len(entry.Facts) != 1 || entry.Facts[0].Span.ID != req.Facts[i].Span.ID {
			t.Fatal("source map is not exact")
		}
	}
	a.Request.Facts[0].Span.Grade.Confidence = 0
	if req.Facts[0].Span.Grade.Confidence != 1 {
		t.Fatal("artifact aliases caller facts")
	}
}

func TestSourcedEvidenceFailuresBeforeModel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*testSources, *SourcedRequest)
	}{
		{"denied", func(s *testSources, r *SourcedRequest) { s.deny = true }},
		{"wrong principal", func(s *testSources, r *SourcedRequest) { r.Principal = "intruder" }},
		{"missing", func(s *testSources, r *SourcedRequest) { delete(s.items, r.Facts[0].Source) }},
		{"stale", func(s *testSources, r *SourcedRequest) {
			s.current[r.Facts[0].Source.ID] = SourceRef{ID: r.Facts[0].Source.ID, Version: "v2", Digest: ContentDigest("13")}
		}},
		{"tampered", func(s *testSources, r *SourcedRequest) {
			ref := r.Facts[0].Source
			v := s.items[ref]
			v.Content = "13"
			s.items[ref] = v
		}},
		{"expired", func(s *testSources, r *SourcedRequest) {
			ref := r.Facts[0].Source
			v := s.items[ref]
			v.RetainUntil = sourcedEpoch
			s.items[ref] = v
		}},
		{"revoked", func(s *testSources, r *SourcedRequest) {
			ref := r.Facts[0].Source
			v := s.items[ref]
			v.Revoked = true
			s.items[ref] = v
		}},
		{"duplicate", func(s *testSources, r *SourcedRequest) { r.Facts = append(r.Facts, r.Facts[0]) }},
		{"excerpt", func(s *testSources, r *SourcedRequest) { r.Facts[0].Span.Value = "14" }},
		{"range", func(s *testSources, r *SourcedRequest) { r.Facts[0].Span.Locator.End = 2 }},
		{"unknown locator", func(s *testSources, r *SourcedRequest) { r.Facts[0].Span.Locator.Kind = "cell" }},
		{"version mismatch", func(s *testSources, r *SourcedRequest) { r.Facts[0].Span.RecordID = "v2" }},
		{"missing author", func(s *testSources, r *SourcedRequest) { r.Facts[0].Span.Author = aiprovenance.Author{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			tc.mutate(s, &req)
			p, m := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
			if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrEvidence) {
				t.Fatalf("error %v", err)
			}
			if m.CallCount() != 0 {
				t.Fatal("invalid evidence reached model")
			}
		})
	}
}

func TestSourcedDraftFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*ProseDraft)
	}{
		{"unknown fact", func(d *ProseDraft) { d.Claims[0].FactIDs = []string{"missing"} }},
		{"duplicate fact edge", func(d *ProseDraft) { d.Claims[0].FactIDs = append(d.Claims[0].FactIDs, d.Claims[0].FactIDs[0]) }},
		{"uncovered assertion", func(d *ProseDraft) { d.Units[0].Text += " invented" }},
		{"wrong assertion", func(d *ProseDraft) { d.Claims[0].Assertion = "14" }},
		{"out of range", func(d *ProseDraft) { d.Units[0].Claims[0].End = 99 }},
		{"overlap", func(d *ProseDraft) { d.Units[0].Claims = append(d.Units[0].Claims, d.Units[0].Claims[0]) }},
		{"unknown claim", func(d *ProseDraft) { d.Units[0].Claims[0].ClaimID = "missing" }},
		{"duplicate unit", func(d *ProseDraft) { d.Units = append(d.Units, d.Units[0]) }},
		{"unused claim", func(d *ProseDraft) { c := d.Claims[0]; c.ID = "unused"; d.Claims = append(d.Claims, c) }},
		{"decorative escape", func(d *ProseDraft) { d.Units[0].NonFactualReason = "trust me"; d.Units[0].Claims = nil; d.Claims = nil }},
		{"empty", func(d *ProseDraft) { d.Units = nil; d.Claims = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			tc.mutate(&draft)
			p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
			if _, err := p.CheckDraft(context.Background(), req, draft); err == nil {
				t.Fatal("bad draft passed")
			}
		})
	}
}

func TestSourcedStrictJSON(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`null`, `{"claims":[],"units":[],"pass":true}`, `{"claims":[],"Claims":[],"units":[]}`, `{"claims":[],"units":[]} {}`, `not JSON`, `{"claims":[],"units":[{"id":"u","text":"12","text":"14","claims":[]}]}`} {
		s, req, draft := sourcedFixture(t, "12")
		p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
		p.client = llmclient.NewMockClient(llmclient.MockEntryOK(raw))
		if _, err := p.Generate(context.Background(), req); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

type reviewFunc func(context.Context, SupportReview) (SupportDecision, error)

func (f reviewFunc) Review(c context.Context, r SupportReview) (SupportDecision, error) {
	return f(c, r)
}

func TestSourcedReviewerFailsClosedAndCannotMutate(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12")
	for _, status := range []string{"", "unsupported", "inconclusive", "PASS"} {
		p, _ := sourcedPipeline(t, s, reviewFunc(func(context.Context, SupportReview) (SupportDecision, error) {
			return SupportDecision{Status: status, Reviewer: "judge", Method: "fixture", Reason: "fixture"}, nil
		}), draft)
		if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrUnsupported) {
			t.Fatal(err)
		}
	}
	p, _ := sourcedPipeline(t, s, reviewFunc(func(ctx context.Context, in SupportReview) (SupportDecision, error) {
		in.Claim.Assertion = "14"
		in.Facts[0].Span.Value = "14"
		return SupportDecision{Status: "supported", Reviewer: "reviewer", Method: "fixture", Reason: "independently labelled"}, nil
	}), draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	if a.Draft.Claims[0].Assertion != "12" || a.Request.Facts[0].Span.Value != "12" {
		t.Fatal("reviewer changed checked data")
	}
}

func TestSourcedCorrectionAndRedaction(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "private metric", "public result")
	ref := req.Facts[0].Source
	v := s.items[ref]
	v.Access = "restricted"
	s.items[ref] = v
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	a.ApprovalRef = "approval-v1"
	x, err := p.Export(context.Background(), a, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(x)
	for _, secret := range []string{"private metric", req.Facts[0].Span.ID, ref.ID, "unit-a"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("redacted sidecar leaked %s", secret)
		}
	}
	if x.RedactedUnits != 2 || len(x.SourceMap) != 0 || x.Markdown != "[redacted]\n\n[redacted]" {
		t.Fatalf("bad redaction %+v", x)
	}
	invalid, err := InvalidateSource(a, ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.ApprovalRef != "" || len(invalid.InvalidUnits) != 1 || invalid.InvalidUnits[0] != "unit-a" || len(invalid.InvalidFacts) != 1 || a.ApprovalRef != "approval-v1" || a.Receipt.Integrity != "pass" {
		t.Fatal("invalidation not owned/exact")
	}
	if _, err := p.Export(context.Background(), invalid, "public", allowPublication); !errors.Is(err, ErrArtifact) {
		t.Fatal("invalidated artifact exported")
	}
	if _, err := p.Export(context.Background(), a, "public", nil); !errors.Is(err, ErrArtifact) {
		t.Fatal("missing authorization allowed")
	}
	a.Draft.Units[0].Text = "changed"
	if _, err := p.Export(context.Background(), a, "public", allowPublication); err == nil {
		t.Fatal("artifact drift allowed")
	}
}

func TestSourcedUnlistedPrivateContextCannotLeak(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "public", "private contents")
	ref := req.Facts[1].Source
	v := s.items[ref]
	v.Access = "restricted"
	s.items[ref] = v
	draft.Claims = draft.Claims[:1]
	draft.Units = draft.Units[:1]
	draft.Claims[0].Assertion = "private contents"
	draft.Units[0].Text = "private contents"
	draft.Units[0].ID = "private contents"
	draft.Units[0].Claims[0].End = len("private contents")
	reviewer := reviewFunc(func(context.Context, SupportReview) (SupportDecision, error) {
		return SupportDecision{Status: "supported", Reviewer: "heuristic", Method: "fixture", Reason: "wrong semantic judgment"}, nil
	})
	p, _ := sourcedPipeline(t, s, reviewer, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.Export(context.Background(), a, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(x)
	if strings.Contains(string(b), "private contents") || len(x.SourceMap) != 0 {
		t.Fatal("model-controlled lineage declassified private context")
	}
}

func TestSourcedExportDoesNotTrustGenerationClaims(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12")
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	a.Receipt.Generation = "invented"
	a.Receipt.PromptDigest = ContentDigest("invented")
	authorize := func(ctx context.Context, principal string, r ProseReceipt, audience string) error {
		if r.Generation != "supplied" || r.PromptDigest == a.Receipt.PromptDigest {
			t.Fatal("untrusted generation provenance reached authorization")
		}
		return allowPublication(ctx, principal, r, audience)
	}
	if _, err := p.Export(context.Background(), a, "public", authorize); err != nil {
		t.Fatal(err)
	}
}

func TestSourcedExportAmplificationBound(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, strings.Repeat("*", 1000))
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	p.config.Limits.MaxBytes = 4096
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Export(context.Background(), a, "public", allowPublication); !errors.Is(err, ErrArtifact) {
		t.Fatal("export expansion not bounded")
	}
}

func TestSourcedExportRechecksAfterAuthorization(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12")
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	authorize := func(context.Context, string, ProseReceipt, string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.current[req.Facts[0].Source.ID] = SourceRef{}
		return nil
	}
	if _, err := p.Export(context.Background(), a, "public", authorize); !errors.Is(err, ErrEvidence) {
		t.Fatalf("drift during authorization passed %v", err)
	}
}

func TestSourcedArithmeticAndTransitiveInvalidation(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12", "10")
	derived := req.Facts[0]
	derived.Span.ID = "delta"
	derived.Span.Value = "2"
	derived.Span.Field = "difference"
	derived.Source = SourceRef{}
	derived.Span.SourceID = ""
	derived.Span.RecordID = ""
	derived.Span.RecordHash = ""
	derived.Span.Locator = evidencespan.Locator{}
	derived.Derivation = &Derivation{Version: "arithmetic/v1", Operation: "difference", Inputs: []string{"fact-a", "fact-b"}}
	req.Facts = append(req.Facts, derived)
	draft.Claims = []ProseClaim{{ID: "delta-claim", Assertion: "2", FactIDs: []string{"delta"}}}
	draft.Units = []ProseUnit{{ID: "delta-unit", Text: "2", Claims: []ClaimRange{{ClaimID: "delta-claim", End: 1}}}}
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := InvalidateSource(a, req.Facts[1].Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(invalid.InvalidFacts) != 2 || len(invalid.InvalidUnits) != 1 {
		t.Fatalf("missing transitive closure %+v", invalid)
	}
	x, err := p.Export(context.Background(), a, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.SourceMap[0].Facts) != 3 {
		t.Fatal("derivation sources missing")
	}
	req.Facts[2].Span.Value = "4"
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrEvidence) {
		t.Fatal("wrong arithmetic passed")
	}
	req.Facts[2].Span.Value = "2"
	req.Facts[1].Unit = "hours"
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrEvidence) {
		t.Fatal("mixed units passed")
	}
}

func TestSourcedCancellationAndBounds(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12")
	p, m := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Generate(ctx, req); !errors.Is(err, context.Canceled) || m.CallCount() != 0 {
		t.Fatal("canceled generation dispatched")
	}
	p.config.Limits.MaxBytes = 1
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrEvidence) {
		t.Fatal("input byte bound ignored")
	}
}

func TestSourcedExportOffsetsAndSidecarIntegrity(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "Café <tag> [value].")
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	a, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.Export(context.Background(), a, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProseExport(x); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(x.Markdown, "<tag>") || strings.Contains(x.Markdown, "[value]") {
		t.Fatal("active Markdown leaked")
	}
	x.SourceMap[0].Facts[0].Span.Value = "changed"
	if err := VerifyProseExport(x); !errors.Is(err, ErrArtifact) {
		t.Fatal("sidecar tampering not detected")
	}
}

func TestSourcedRestrictedGenerationNeedsEgressPolicy(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "restricted result")
	ref := req.Facts[0].Source
	v := s.items[ref]
	v.Access = "restricted"
	s.items[ref] = v
	p, m := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrEvidence) || m.CallCount() != 0 {
		t.Fatal("read access silently granted egress")
	}
	p.config.AllowRestrictedGeneration = true
	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestSourcedCompletionAndRuneFailures(t *testing.T) {
	t.Parallel()
	for _, stop := range []llmclient.StopReason{llmclient.StopRefusal, llmclient.StopMaxTokens, llmclient.StopToolUse, "unrecognized"} {
		s, req, draft := sourcedFixture(t, "12")
		p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
		raw, _ := json.Marshal(draft)
		p.client = llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: string(raw), StopReason: stop}})
		if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrArtifact) {
			t.Fatalf("stop %s: %v", stop, err)
		}
	}
	s, req, draft := sourcedFixture(t, "é")
	req.Facts[0].Span.Locator.End = 0
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrEvidence) {
		t.Fatal("split source rune accepted")
	}
}

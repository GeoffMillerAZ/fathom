package governedprosekit

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Deliberately accepts every principal, so a fixture's authorization policy cannot
// conceal normalization by rejecting the replacement character incidentally.
type utf8RecordingSources struct {
	SourceProvider
	principals []string
}

func (s *utf8RecordingSources) Resolve(ctx context.Context, principal string, ref SourceRef) (SourceSnapshot, error) {
	s.principals = append(s.principals, principal)
	return s.SourceProvider.Resolve(ctx, "reader", ref)
}

func (s *utf8RecordingSources) Current(ctx context.Context, principal, id string) (SourceRef, error) {
	s.principals = append(s.principals, principal)
	return s.SourceProvider.Current(ctx, "reader", id)
}

func TestSourcedPrincipalCannotChangeBeforeAuthorization(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"generate", "check", "export"} {
		t.Run(operation, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			sources := &utf8RecordingSources{SourceProvider: s}
			reviews, authorizations := 0, 0
			p, model := sourcedPipeline(t, sources, reviewFunc(func(ctx context.Context, in SupportReview) (SupportDecision, error) {
				reviews++
				return (ExactSupportReviewer{}).Review(ctx, in)
			}), draft)
			// An existing valid artifact must not authorize an invalid principal
			// merely because JSON copying replaces its bytes with this rune.
			req.Principal = "\uFFFD"
			artifact, err := p.CheckDraft(context.Background(), req, draft)
			if err != nil {
				t.Fatal(err)
			}
			sources.principals, reviews = nil, 0
			req.Principal = string([]byte{0xff})
			want := ErrArtifact
			switch operation {
			case "generate":
				_, err = p.Generate(context.Background(), req)
				want = ErrEvidence
			case "check":
				_, err = p.CheckDraft(context.Background(), req, draft)
			case "export":
				artifact.Request = req
				_, err = p.Export(context.Background(), artifact, "public", func(context.Context, string, ProseReceipt, string) error {
					authorizations++
					return nil
				})
			}
			if !errors.Is(err, want) {
				t.Errorf("invalid principal accepted or misclassified: %v", err)
			}
			if len(sources.principals) != 0 || reviews != 0 || model.CallCount() != 0 || authorizations != 0 {
				t.Fatalf("invalid principal reached a dependency: sources=%q reviews=%d model=%d authorization=%d", sources.principals, reviews, model.CallCount(), authorizations)
			}
		})
	}
}

func TestSourcedValidUnicodeIdentityPreserved(t *testing.T) {
	t.Parallel()
	for _, principal := range []string{"\uFFFD", "読者", "re\u0301ader"} {
		t.Run(principal, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "Café results")
			sources := &utf8RecordingSources{SourceProvider: s}
			p, _ := sourcedPipeline(t, sources, ExactSupportReviewer{}, draft)
			req.Principal = principal
			artifact, err := p.Generate(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Request.Principal != principal {
				t.Fatal("artifact changed principal bytes")
			}
			authorized := false
			_, err = p.Export(context.Background(), artifact, "public", func(_ context.Context, got string, _ ProseReceipt, _ string) error {
				authorized = true
				if got != principal {
					t.Fatal("publication principal changed")
				}
				return nil
			})
			if err != nil || !authorized || len(sources.principals) == 0 {
				t.Fatalf("valid Unicode rejected: %v", err)
			}
			for _, got := range sources.principals {
				if got != principal {
					t.Fatal("source principal changed")
				}
			}
		})
	}
}

func TestSourcedCopiesAndDigestsRejectInvalidUTF8(t *testing.T) {
	t.Parallel()
	bad := string([]byte{0xff})
	for _, tc := range []struct {
		name string
		in   any
	}{
		{"principal", SourcedRequest{Principal: bad}},
		{"topic", SourcedRequest{Topic: bad}},
		{"source ID", SourceRef{ID: bad}},
		{"source version", SourceRef{Version: bad}},
		{"source digest", SourceRef{Digest: bad}},
		{"fact identity", Fact{Span: evidencespan.Span{ID: bad}}},
		{"entity identity", Fact{Span: evidencespan.Span{EntityID: bad}}},
		{"fact field", Fact{Span: evidencespan.Span{Field: bad}}},
		{"locator", Fact{Span: evidencespan.Span{Locator: evidencespan.Locator{Value: bad}}}},
		{"human provenance", Fact{Span: evidencespan.Span{Author: aiprovenance.NewHuman(bad)}}},
		{"AI provenance", Fact{Span: evidencespan.Span{Author: aiprovenance.Author{Kind: aiprovenance.KindAI, AI: &aiprovenance.AIAuthor{ModelID: bad}}}}},
		{"unit", Fact{Unit: bad}},
		{"derivation input", Fact{Derivation: &Derivation{Inputs: []string{bad}}}},
		{"claim identity", ProseDraft{Claims: []ProseClaim{{ID: bad}}}},
		{"support edge", ProseDraft{Claims: []ProseClaim{{FactIDs: []string{bad}}}}},
		{"contradiction edge", ProseDraft{Claims: []ProseClaim{{Contradictions: []string{bad}}}}},
		{"unit identity", ProseDraft{Units: []ProseUnit{{ID: bad}}}},
		{"claim range", ProseDraft{Units: []ProseUnit{{Claims: []ClaimRange{{ClaimID: bad}}}}}},
		{"classification", ProseDraft{Units: []ProseUnit{{NonFactualReason: bad}}}},
		{"review identity", SupportDecision{Reviewer: bad}},
		{"review map key", ProseReceipt{Support: map[string]SupportDecision{bad: {Status: "supported"}}}},
		{"review map value", ProseReceipt{Support: map[string]SupportDecision{"claim": {Reason: bad}}}},
		{"invalidation", SourcedArtifact{InvalidFacts: []string{bad}}},
		{"approval", SourcedArtifact{ApprovalRef: bad}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := cloneValue(tc.in); !errors.Is(err, ErrArtifact) {
				t.Errorf("copy normalized invalid UTF-8: %v", err)
			}
			if _, err := digestJSON(tc.in); !errors.Is(err, ErrArtifact) {
				t.Errorf("digest normalized invalid UTF-8: %v", err)
			}
		})
	}
}

func TestSourcedConfigAndReviewRejectInvalidUTF8(t *testing.T) {
	t.Parallel()
	bad := string([]byte{0xff})
	for _, field := range []string{"PolicyVersion", "PromptVersion", "Model"} {
		t.Run(field, func(t *testing.T) {
			s, _, _ := sourcedFixture(t, "12")
			cfg := SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "fixture-generator", Limits: DefaultProseLimits()}
			reflect.ValueOf(&cfg).Elem().FieldByName(field).SetString(bad)
			if _, err := NewSourcedPipeline(llmclient.NewMockClient(), s, ExactSupportReviewer{}, cfg, func() time.Time { return sourcedEpoch }); !errors.Is(err, ErrArtifact) {
				t.Fatalf("invalid config identity accepted: %v", err)
			}
		})
	}
	for _, field := range []string{"Reviewer", "Method", "Reason"} {
		t.Run(field, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			p, _ := sourcedPipeline(t, s, reviewFunc(func(ctx context.Context, in SupportReview) (SupportDecision, error) {
				decision, err := (ExactSupportReviewer{}).Review(ctx, in)
				reflect.ValueOf(&decision).Elem().FieldByName(field).SetString(bad)
				return decision, err
			}), draft)
			if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("invalid review metadata accepted: %v", err)
			}
		})
	}
}

func TestSourcedSnapshotMetadataRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"AcquiredBy", "MediaType"} {
		t.Run(field, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			ref := req.Facts[0].Source
			snapshot := s.items[ref]
			value := reflect.ValueOf(&snapshot).Elem().FieldByName(field)
			value.SetString(value.String() + string([]byte{0xff}))
			s.items[ref] = snapshot
			p, model := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
			if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrEvidence) || model.CallCount() != 0 {
				t.Fatalf("invalid snapshot metadata reached generation: %v", err)
			}
		})
	}
}

func TestSourcedInvalidationAndTransportRejectChangedIdentity(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "12")
	draft.Units[0].ID = "\uFFFD"
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	artifact, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	export, err := p.Export(context.Background(), artifact, "public", allowPublication)
	if err != nil {
		t.Fatal(err)
	}
	bad := string([]byte{0xff})
	artifact.Draft.Units[0].ID = bad
	if _, err := InvalidateSource(artifact, req.Facts[0].Source.ID); !errors.Is(err, ErrArtifact) {
		t.Fatalf("invalidation normalized identity: %v", err)
	}
	export.SourceMap[0].UnitID = bad
	if err := VerifyProseExport(export); !errors.Is(err, ErrArtifact) {
		t.Fatalf("changed sidecar identity matched previous digest: %v", err)
	}
}

func TestSourcedUTF8WalkIsBounded(t *testing.T) {
	t.Parallel()
	for _, value := range []any{nil, (*Fact)(nil), []Fact(nil), map[string]SupportDecision(nil), [1]string{"読者"}, sourcedEpoch} {
		if !sourcedUTF8Valid(value) {
			t.Fatalf("valid value rejected: %T", value)
		}
	}
	if sourcedUTF8Valid([1]string{string([]byte{0xff})}) {
		t.Fatal("array element escaped validation")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if sourcedUTF8Valid(cycle) {
		t.Fatal("cycle escaped recursion bound")
	}
}

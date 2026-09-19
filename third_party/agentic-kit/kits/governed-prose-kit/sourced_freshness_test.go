package governedprosekit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
)

func TestSourcedReviewCannotOutliveEvidence(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"advance", "revoke", "expire", "deny"} {
		t.Run(change, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			p, _ := sourcedPipeline(t, s, reviewFunc(func(ctx context.Context, in SupportReview) (SupportDecision, error) {
				d, err := (ExactSupportReviewer{}).Review(ctx, in)
				s.mu.Lock()
				defer s.mu.Unlock()
				ref := req.Facts[0].Source
				snapshot := s.items[ref]
				switch change {
				case "advance":
					s.current[ref.ID] = SourceRef{}
				case "revoke":
					snapshot.Revoked = true
				case "expire":
					snapshot.RetainUntil = sourcedEpoch
				case "deny":
					s.deny = true
				}
				s.items[ref] = snapshot
				return d, err
			}), draft)
			if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrEvidence) {
				t.Fatalf("stale review accepted: %v", err)
			}
		})
	}
}

func TestSourcedGoInputCannotSilentlyReplaceInvalidUTF8(t *testing.T) {
	t.Parallel()
	s, req, draft := sourcedFixture(t, "\uFFFD")
	p, m := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	req.Facts[0].Span.Value = string([]byte{0xff})
	if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrEvidence) || m.CallCount() != 0 {
		t.Fatal("invalid source bytes were normalized before generation")
	}
	req.Facts[0].Span.Value = "\uFFFD"
	draft.Claims[0].Assertion, draft.Units[0].Text = string([]byte{0xff}), string([]byte{0xff})
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrArtifact) {
		t.Fatal("invalid prose bytes were normalized to match evidence")
	}
}

func TestSourcedArithmeticGrammarAndGrade(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"12", "-12", "+12.5", "1/3", "0", "0.5"} {
		if _, ok := boundedRational(value); !ok {
			t.Errorf("valid decimal rational rejected: %q", value)
		}
	}
	for _, value := range []string{"1e1000000", "1p10000000", "0x10", "1/0", "1/-2", "NaN", " 1", strings.Repeat("1", 129)} {
		if _, ok := boundedRational(value); ok {
			t.Errorf("unbounded or non-decimal input accepted: %q", value)
		}
	}
	_, req, _ := sourcedFixture(t, "12", "10")
	d := req.Facts[0]
	d.Span.ID, d.Span.Value = "delta", "2"
	d.Derivation = &Derivation{Version: "arithmetic/v1", Operation: "difference", Inputs: []string{"fact-a", "fact-b"}}
	facts := map[string]Fact{"fact-a": req.Facts[0], "fact-b": req.Facts[1]}
	for _, change := range []string{"tier", "confidence", "observed"} {
		t.Run(change, func(t *testing.T) {
			input := req.Facts[1]
			input.Span.Grade = &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 1}
			switch change {
			case "tier":
				input.Span.Grade.Tier = evidencegrade.TierInferred
			case "confidence":
				input.Span.Grade.Confidence = 0.5
			case "observed":
				input.Span.ObservedAt = d.Span.ObservedAt.Add(time.Minute)
			}
			facts["fact-b"] = input
			if err := checkDerivation(d, facts); !errors.Is(err, ErrEvidence) {
				t.Fatalf("derived grade/time escaped input ceiling: %v", err)
			}
		})
	}
}

func TestSourcedHistoricalArithmeticNeedsExplicitReview(t *testing.T) {
	t.Parallel()
	s, req, _ := sourcedFixture(t, "12", "10")
	req.Facts[1].Historical = true
	d := req.Facts[0]
	d.Span.ID, d.Span.Value, d.Span.Field = "delta", "2", "difference"
	d.Source, d.Span.Locator = SourceRef{}, evidencespan.Locator{}
	d.Span.SourceID, d.Span.RecordID, d.Span.RecordHash = "", "", ""
	d.Derivation = &Derivation{Version: "arithmetic/v1", Operation: "difference", Inputs: []string{"fact-a", "fact-b"}}
	req.Facts = append(req.Facts, d)
	draft := ProseDraft{Claims: []ProseClaim{{ID: "claim", Assertion: "2", FactIDs: []string{"delta"}}}, Units: []ProseUnit{{ID: "unit", Text: "2", Claims: []ClaimRange{{ClaimID: "claim", End: 1}}}}}
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	if _, err := p.CheckDraft(context.Background(), req, draft); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("historical ancestor laundered as current: %v", err)
	}
}

package governedprosekit

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	prosepolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prose/prose-policy"
	traceableprose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prose/traceable-prose"
	evalgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
	prellmdlpgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

func TestRegenerateUsesInjectedClock(t *testing.T) {
	want := time.Date(2026, 9, 13, 8, 0, 0, 0, time.FixedZone("test", 3600))
	p, err := NewPipelineWithClock(llmclient.NewMockClient(), prosepolicy.Policy{Posture: prosepolicy.PostureOpen}, prellmdlpgate.DefaultPolicy(), evalgate.Gate{Threshold: 1}, func() time.Time { return want })
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Regenerate(context.Background(), CorrectionInput{CorrectionID: "c1", AffectedNodeIDs: []string{"n1"}, NewText: "corrected"}, []traceableprose.TraceableNode{{ID: "n1", Text: "old", SourceRefs: []string{"s1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.RegenerationLog()[0].OccurredAt; !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("regeneration timestamp %v, want %v UTC", got, want)
	}
	for _, e := range p.ledger.Events() {
		if !e.OccurredAt.Equal(want) {
			t.Errorf("ledger timestamp %v, want %v", e.OccurredAt, want)
		}
		if e.Correction != nil && !e.Correction.CorrectedAt.Equal(want) {
			t.Errorf("correction timestamp %v, want %v", e.Correction.CorrectedAt, want)
		}
	}
}

package governedprosekit_test

import (
	"context"
	"strings"
	"testing"

	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	prosepolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prose/prose-policy"
	evalgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
	prellmdlpgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/pre-llm-dlp-gate"
	governedprosekit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestPipeline constructs a Pipeline with the provided mock client,
// a minimal prose policy, and the default DLP gate.
func newTestPipeline(t *testing.T, client llmclient.LLMClient, policy prosepolicy.Policy) *governedprosekit.Pipeline {
	t.Helper()
	p, err := governedprosekit.NewPipeline(
		client,
		policy,
		prellmdlpgate.DefaultPolicy(),
		evalgate.Gate{Threshold: 0.5},
	)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	return p
}

// openPolicy returns a prose policy with PostureOpen so that nothing is
// redacted by the policy rules.
func openPolicy() prosepolicy.Policy {
	return prosepolicy.Policy{Posture: prosepolicy.PostureOpen}
}

// restrictedPolicy returns a policy that will redact the term "SECRET" at
// PostureRestricted level, causing a concern flag to be raised.
func restrictedPolicy() prosepolicy.Policy {
	return prosepolicy.Policy{
		Posture: prosepolicy.PostureRestricted,
		Rules: []prosepolicy.PostureRule{
			{
				Threshold: prosepolicy.PostureRestricted,
				Terms:     []string{"SECRET"},
			},
		},
	}
}

// ── Test 1: DLP gate redacts input before the LLM sees it ────────────────────

func TestGenerate_DLPRedactsInput(t *testing.T) {
	t.Parallel()

	// The topic contains a Bearer token that the DLP gate must strip.
	sensitiveToken := "Authorization: Bearer abc123secret"
	topic := "Summarise our API usage. " + sensitiveToken

	mock := llmclient.NewMockClient(
		llmclient.MockEntryOK("Our API usage is nominal."),
	)
	p := newTestPipeline(t, mock, openPolicy())

	_, err := p.Generate(context.Background(), governedprosekit.Request{
		Topic:        topic,
		EvidenceRefs: []string{"span-001"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(calls))
	}

	prompt := calls[0].Request.Prompt
	if strings.Contains(prompt, "abc123secret") {
		t.Errorf("DLP gate did NOT redact the bearer token; prompt contains raw secret:\n%s", prompt)
	}
	if !strings.Contains(prompt, "[REDACTED:BEARER]") {
		t.Errorf("expected [REDACTED:BEARER] placeholder in prompt, got:\n%s", prompt)
	}
}

// ── Test 2: Output carries evidence citations ────────────────────────────────

func TestGenerate_OutputCarriesCitations(t *testing.T) {
	t.Parallel()

	evidenceRefs := []string{"span-b", "span-a"}

	mock := llmclient.NewMockClient(
		llmclient.MockEntryOK("The system is stable.\nPerformance is within SLA."),
	)
	p := newTestPipeline(t, mock, openPolicy())

	result, err := p.Generate(context.Background(), governedprosekit.Request{
		Topic:        "System status",
		EvidenceRefs: evidenceRefs,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The aggregate citation must be the sorted, deduplicated rendering.
	wantCitation := "[span-a; span-b]"
	if result.Citation != wantCitation {
		t.Errorf("Citation = %q; want %q", result.Citation, wantCitation)
	}

	// Every node must carry the evidence refs.
	if len(result.Nodes) == 0 {
		t.Fatal("expected at least one node in result")
	}
	for _, node := range result.Nodes {
		if len(node.SourceRefs) == 0 {
			t.Errorf("node %q carries no SourceRefs", node.ID)
		}
	}
}

// ── Test 3: Policy violation raises a concern flag ───────────────────────────

func TestGenerate_PolicyViolationRaisesConcernFlag(t *testing.T) {
	t.Parallel()

	// The LLM outputs text that contains the term "SECRET" which is
	// prohibited by the restrictedPolicy.
	mock := llmclient.NewMockClient(
		llmclient.MockEntryOK("The SECRET key must not be disclosed."),
	)
	p := newTestPipeline(t, mock, restrictedPolicy())

	result, err := p.Generate(context.Background(), governedprosekit.Request{
		Topic:        "Security posture",
		EvidenceRefs: []string{"span-sec-001"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(result.ConcernFlags) == 0 {
		t.Fatal("expected at least one ConcernFlag for policy violation; got none")
	}

	flag := result.ConcernFlags[0]
	if flag.Type != "policy-violation" {
		t.Errorf("ConcernFlag.Type = %q; want %q", flag.Type, "policy-violation")
	}
	if flag.Severity != "high" {
		t.Errorf("ConcernFlag.Severity = %q; want %q", flag.Severity, "high")
	}

	// The affected node must be marked as redacted.
	affectedID := flag.Target
	var foundNode bool
	for _, n := range result.Nodes {
		if n.ID == affectedID {
			foundNode = true
			if !n.Redacted {
				t.Errorf("node %q was flagged but not marked Redacted", n.ID)
			}
		}
	}
	if !foundNode {
		t.Errorf("no node found with ID %q (concern flag target)", affectedID)
	}
}

// ── Test 4: Regeneration only touches affected nodes ─────────────────────────

func TestRegenerate_OnlyAffectedNodesChanged(t *testing.T) {
	t.Parallel()

	// Generate a two-line output so we get at least two nodes.
	mock := llmclient.NewMockClient(
		llmclient.MockEntryOK("Node alpha content.\nNode beta content."),
	)
	p := newTestPipeline(t, mock, openPolicy())

	result, err := p.Generate(context.Background(), governedprosekit.Request{
		Topic:        "Alpha and Beta",
		EvidenceRefs: []string{"span-001"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.Nodes) < 2 {
		t.Fatalf("expected ≥2 nodes, got %d", len(result.Nodes))
	}

	// Remember the text of the node we do NOT want to touch.
	firstID := result.Nodes[0].ID
	firstTextBefore := result.Nodes[0].Text
	secondID := result.Nodes[1].ID

	correction := governedprosekit.CorrectionInput{
		CorrectionID:    "corr-001",
		AffectedNodeIDs: []string{secondID},
		NewText:         "Node beta CORRECTED content.",
	}

	regenResult, err := p.Regenerate(context.Background(), correction, result.Nodes)
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}

	if regenResult.MarkedCount != 1 {
		t.Errorf("MarkedCount = %d; want 1", regenResult.MarkedCount)
	}

	// The first node must be unchanged.
	var firstAfter, secondAfter string
	for _, n := range regenResult.Nodes {
		switch n.ID {
		case firstID:
			firstAfter = n.Text
		case secondID:
			secondAfter = n.Text
		}
	}

	if firstAfter != firstTextBefore {
		t.Errorf("unaffected node %q changed: before=%q after=%q", firstID, firstTextBefore, firstAfter)
	}
	if secondAfter != correction.NewText {
		t.Errorf("affected node %q text = %q; want %q", secondID, secondAfter, correction.NewText)
	}

	// The regeneration log must have recorded the event.
	events := p.RegenerationLog()
	if len(events) != 1 {
		t.Errorf("regeneration log len = %d; want 1", len(events))
	}
	if events[0].TriggeredBy != correction.CorrectionID {
		t.Errorf("log event TriggeredBy = %q; want %q", events[0].TriggeredBy, correction.CorrectionID)
	}
}

// ── Test 5: Determinism — same mock, same input → same output ────────────────

func TestGenerate_DeterministicWithMock(t *testing.T) {
	t.Parallel()

	buildResult := func() governedprosekit.Result {
		mock := llmclient.NewMockClient(
			llmclient.MockEntryOK("Deterministic prose output."),
		)
		p := newTestPipeline(t, mock, openPolicy())
		r, err := p.Generate(context.Background(), governedprosekit.Request{
			Topic:        "Determinism test",
			EvidenceRefs: []string{"ref-z", "ref-a"},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		return r
	}

	r1 := buildResult()
	r2 := buildResult()

	if r1.Citation != r2.Citation {
		t.Errorf("non-deterministic Citation: %q vs %q", r1.Citation, r2.Citation)
	}
	if len(r1.Nodes) != len(r2.Nodes) {
		t.Errorf("non-deterministic node count: %d vs %d", len(r1.Nodes), len(r2.Nodes))
	}
	for i := range r1.Nodes {
		if r1.Nodes[i].Text != r2.Nodes[i].Text {
			t.Errorf("node[%d] text differs: %q vs %q", i, r1.Nodes[i].Text, r2.Nodes[i].Text)
		}
	}
}

// ── Test 6: Eval-gate passes when no violations, fails when violations exist ──

func TestGenerate_EvalGatePassesAndFails(t *testing.T) {
	t.Parallel()

	t.Run("passes when output is clean", func(t *testing.T) {
		mock := llmclient.NewMockClient(
			llmclient.MockEntryOK("Clean, citable output."),
		)
		p := newTestPipeline(t, mock, openPolicy())
		result, err := p.Generate(context.Background(), governedprosekit.Request{
			Topic:        "Clean output",
			EvidenceRefs: []string{"span-clean"},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !result.GateResult.Passed {
			t.Errorf("eval-gate should pass for clean output; failures: %v", result.GateResult.Failures)
		}
	})

	t.Run("fails when policy violation exists", func(t *testing.T) {
		mock := llmclient.NewMockClient(
			llmclient.MockEntryOK("The SECRET must not be revealed."),
		)
		// Gate threshold 1.0 means all checks must pass — a policy violation
		// will cause the "no-policy-violation" check to fail.
		client2 := mock
		p2, err := governedprosekit.NewPipeline(
			client2,
			restrictedPolicy(),
			prellmdlpgate.DefaultPolicy(),
			evalgate.Gate{Threshold: 1.0},
		)
		if err != nil {
			t.Fatalf("NewPipeline: %v", err)
		}
		result, err := p2.Generate(context.Background(), governedprosekit.Request{
			Topic:        "Violation test",
			EvidenceRefs: []string{"span-v"},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if result.GateResult.Passed {
			t.Error("eval-gate should NOT pass when a policy violation exists")
		}
	})
}

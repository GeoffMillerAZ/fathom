package governedprosekit

import (
	"context"
	"fmt"
	"strings"
	"time"

	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	evidencecitation "github.com/geoffmilleraz/agentic-kit/foundations/prose/evidence-citation"
	prosepolicy "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-policy"
	traceableprose "github.com/geoffmilleraz/agentic-kit/foundations/prose/traceable-prose"
	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
	concernflags "github.com/geoffmilleraz/agentic-kit/foundations/review/concern-flags"
	correctionledger "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger"
	prellmdlpgate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// Pipeline is the governed prose generation→review→regeneration pipeline.
// All dependencies are injected; there are no global variables. The zero value
// is not usable — construct one via NewPipeline.
type Pipeline struct {
	client   llmclient.LLMClient
	policy   prosepolicy.Policy
	dlpGate  *prellmdlpgate.Gate
	evalGate evalgate.Gate
	regenLog traceableprose.RegenerationLog
	ledger   *correctionledger.Ledger
	now      func() time.Time
}

// NewPipeline constructs a Pipeline with the supplied dependencies.
//
//   - client    — the LLM client used for prose generation (required).
//   - policy    — the prose/DLP policy evaluated after generation (required).
//   - dlpPolicy — the DLP redaction policy applied before the LLM call. Pass
//     prellmdlpgate.DefaultPolicy() for the built-in set of rules.
//   - evalGate  — the publishability gate applied after generation.
func NewPipeline(
	client llmclient.LLMClient,
	policy prosepolicy.Policy,
	dlpPolicy prellmdlpgate.RedactPolicy,
	gate evalgate.Gate,
) (*Pipeline, error) {
	return NewPipelineWithClock(client, policy, dlpPolicy, gate, nil)
}

// NewPipelineWithClock constructs a Pipeline whose regeneration and correction
// records use now, normalized to UTC. A nil clock defaults to time.Now.
// NewPipeline retains its original signature and uses this documented default.
func NewPipelineWithClock(client llmclient.LLMClient, policy prosepolicy.Policy, dlpPolicy prellmdlpgate.RedactPolicy, gate evalgate.Gate, now func() time.Time) (*Pipeline, error) {
	if client == nil {
		return nil, fmt.Errorf("governedprosekit: LLMClient must not be nil")
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("governedprosekit: invalid policy: %w", err)
	}
	g, err := prellmdlpgate.NewGate(dlpPolicy)
	if err != nil {
		return nil, fmt.Errorf("governedprosekit: invalid DLP policy: %w", err)
	}
	if now == nil {
		now = time.Now
	}
	clock := func() time.Time { return now().UTC() }
	return &Pipeline{
		client:   client,
		policy:   policy,
		dlpGate:  g,
		evalGate: gate,
		ledger:   correctionledger.NewWithClock(clock),
		now:      clock,
	}, nil
}

// Generate runs the full governed generation chain:
//  1. DLP-redact the request topic/audience text before it reaches the LLM.
//  2. Call the injected LLM client with the (redacted) prompt.
//  3. Parse the raw response into traceable prose nodes.
//  4. Annotate each node with evidence citations.
//  5. Evaluate each node against the prose policy.
//  6. Raise ConcernFlags for any policy violations.
//  7. Run the eval-gate over the generated nodes.
//  8. Return the Result.
func (p *Pipeline) Generate(ctx context.Context, req Request) (Result, error) {
	if req.Topic == "" {
		return Result{}, fmt.Errorf("governedprosekit: Request.Topic must not be empty")
	}

	// ── Step 1: DLP-redact the input before it reaches the LLM ────────────
	promptText := buildPrompt(req)
	redactedPrompt, _ := p.dlpGate.Scan(promptText)

	// ── Step 2: LLM generation ─────────────────────────────────────────────
	llmReq := llmclient.Request{
		Prompt: redactedPrompt,
		System: req.SystemPrompt,
	}
	resp, err := p.client.Complete(ctx, llmReq)
	if err != nil {
		return Result{}, fmt.Errorf("governedprosekit: LLM call failed: %w", err)
	}

	rawText := resp.Text

	// ── Step 3: Parse raw text into traceable nodes ─────────────────────────
	nodes := parseNodes(rawText, req.EvidenceRefs)

	// ── Step 4: Compute the aggregate evidence citation ────────────────────
	citation := evidencecitation.Cite(req.EvidenceRefs)

	// ── Step 5: Evaluate each node against the prose policy ────────────────
	var flags []concernflags.ConcernFlag
	flagReg := &concernflags.Register{}
	flagSeq := 0

	for i := range nodes {
		decision := p.policy.Evaluate(nodes[i].Text)
		if !decision.Allowed {
			// Redact the node text in place.
			redacted, _ := p.dlpGate.Scan(nodes[i].Text)
			_ = nodes[i].MarkRedacted(
				fmt.Sprintf("prose-policy violation: terms %v", decision.Redactions),
			)
			// Replace text with the redacted version so downstream consumers
			// see the redacted text in the node.
			nodes[i].Text = redacted

			// ── Step 6: Raise a ConcernFlag ────────────────────────────────
			flagSeq++
			flag := concernflags.ConcernFlag{
				ID:        fmt.Sprintf("CF-%03d", flagSeq),
				Type:      "policy-violation",
				Severity:  concernflags.SeverityHigh,
				Target:    nodes[i].ID,
				Rationale: fmt.Sprintf("prose-policy redaction triggered for terms: %v", decision.Redactions),
			}
			_ = flagReg.Raise(flag)
			flags = append(flags, flag)
		}
	}

	// ── Step 7: Eval-gate ──────────────────────────────────────────────────
	checks := buildEvalChecks(nodes, flags)
	gateResult := p.evalGate.Evaluate(checks)

	return Result{
		Nodes:        nodes,
		Citation:     citation,
		ConcernFlags: flags,
		GateResult:   gateResult,
		RawText:      rawText,
	}, nil
}

// Regenerate re-renders only the nodes affected by a correction and appends a
// RegenerationEvent to the internal log. Unaffected nodes are returned
// unchanged. This implements targeted, provenance-preserving regeneration as
// described in the traceable-prose foundation.
func (p *Pipeline) Regenerate(ctx context.Context, correction CorrectionInput, nodes []traceableprose.TraceableNode) (RegenerationResult, error) {
	if correction.CorrectionID == "" {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: CorrectionInput.CorrectionID must not be empty")
	}
	if len(correction.AffectedNodeIDs) == 0 {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: CorrectionInput.AffectedNodeIDs must not be empty")
	}
	if correction.NewText == "" {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: CorrectionInput.NewText must not be empty")
	}

	// Mark affected nodes using the traceable-prose primitive (sets NeedsRegeneration).
	marked := traceableprose.MarkNodesForRegeneration(nodes, correction.AffectedNodeIDs)

	// Re-render only nodes that were marked.
	affected := make(map[string]struct{}, len(correction.AffectedNodeIDs))
	for _, id := range correction.AffectedNodeIDs {
		affected[id] = struct{}{}
	}
	for i := range nodes {
		if _, ok := affected[nodes[i].ID]; ok && nodes[i].NeedsRegeneration {
			// Apply the corrected text; clear the regeneration flag.
			nodes[i].Text = correction.NewText
			nodes[i].NeedsRegeneration = false
		}
	}

	// Append a RegenerationEvent to the log.
	event := traceableprose.RegenerationEvent{
		TriggeredBy:     correction.CorrectionID,
		AffectedNodeIDs: correction.AffectedNodeIDs,
		OccurredAt:      p.now(),
		Note:            fmt.Sprintf("correction %s applied", correction.CorrectionID),
	}
	if err := p.regenLog.Append(event); err != nil {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: appending regen event: %w", err)
	}

	// Record the correction in the correction ledger for auditability.
	finding, err := p.ledger.OpenFinding(correction.CorrectionID, fmt.Sprintf("correction for %v", correction.AffectedNodeIDs))
	if err != nil {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: opening finding in ledger: %w", err)
	}
	// Advance FSM: open → reviewing → corrected.
	reviewing, err := p.ledger.ApplyTransition(finding, correctionledger.EventClaim)
	if err != nil {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: ledger claim transition: %w", err)
	}
	c := correctionledger.Correction{
		FindingID: correction.CorrectionID,
		Target:    correctionledger.CorrectionTarget{StableTarget: strings.Join(correction.AffectedNodeIDs, ",")},
		// Original is set to a pipeline sentinel because the prior text is not
		// passed through CorrectionInput. Callers requiring a verbatim original
		// should extend CorrectionInput with a PriorText field.
		Original:    "(prior text not available at pipeline level)",
		Corrected:   correction.NewText,
		CorrectedBy: "pipeline",
		CorrectedAt: p.now(),
		Reason:      "governed regeneration",
		Confidence:  5,
	}
	if err := p.ledger.RecordCorrection(c); err != nil {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: recording correction: %w", err)
	}
	if _, err := p.ledger.ApplyTransition(reviewing, correctionledger.EventCorrect); err != nil {
		return RegenerationResult{}, fmt.Errorf("governedprosekit: ledger corrected transition: %w", err)
	}

	return RegenerationResult{
		Nodes:       nodes,
		MarkedCount: marked,
	}, nil
}

// RegenerationLog returns the immutable regeneration event history.
func (p *Pipeline) RegenerationLog() []traceableprose.RegenerationEvent {
	return p.regenLog.Events()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// buildPrompt assembles the plain-text prompt from the request fields.
// The DLP gate is applied to this string before it is sent to the LLM.
func buildPrompt(req Request) string {
	var sb strings.Builder
	sb.WriteString("Topic: ")
	sb.WriteString(req.Topic)
	if req.Audience != "" {
		sb.WriteString("\nAudience: ")
		sb.WriteString(req.Audience)
	}
	if len(req.EvidenceRefs) > 0 {
		sb.WriteString("\nEvidence: ")
		sb.WriteString(strings.Join(req.EvidenceRefs, ", "))
	}
	sb.WriteString("\nGenerate governed prose.")
	return sb.String()
}

// parseNodes converts the raw LLM text into a slice of TraceableNodes.
// Each non-empty line becomes its own node so that targeted regeneration can
// operate at line granularity. This keeps the kit's behaviour deterministic and
// testable without any additional LLM calls.
func parseNodes(raw string, evidenceRefs []string) []traceableprose.TraceableNode {
	lines := strings.Split(raw, "\n")
	var nodes []traceableprose.TraceableNode
	seq := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		seq++
		node := traceableprose.TraceableNode{
			ID:         fmt.Sprintf("node-%03d", seq),
			Text:       trimmed,
			SourceRefs: append([]string(nil), evidenceRefs...),
		}
		nodes = append(nodes, node)
	}
	// Guarantee at least one node so the result is never empty.
	if len(nodes) == 0 {
		nodes = append(nodes, traceableprose.TraceableNode{
			ID:         "node-001",
			Text:       raw,
			SourceRefs: append([]string(nil), evidenceRefs...),
		})
	}
	return nodes
}

// buildEvalChecks constructs the eval-gate check set from the generated nodes
// and any concern flags raised.
func buildEvalChecks(nodes []traceableprose.TraceableNode, flags []concernflags.ConcernFlag) []evalgate.Check {
	hasCitations := false
	for _, n := range nodes {
		if len(n.SourceRefs) > 0 {
			hasCitations = true
			break
		}
	}

	noPolicyViolation := len(flags) == 0

	return []evalgate.Check{
		{
			Name:   "has-citations",
			Passed: hasCitations,
			Weight: 1.0,
		},
		{
			Name:   "no-policy-violation",
			Passed: noPolicyViolation,
			Weight: 2.0,
		},
	}
}

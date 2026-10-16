package investigation

import (
	"fmt"
	"time"

	evidencegraph "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-graph"
	evidenceresolver "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-resolver"
	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	convergenceoracle "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/convergence-oracle"
	dadinvestigator "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/dad-investigator"
	hypothesisranker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/hypothesis-ranker"
	investigationjournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
	concernflags "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/concern-flags"
)

// Investigation is a governed investigation session. It wires together the
// evidence, reasoning, and review foundations into a single coherent workflow.
//
// The zero value is not usable — construct via New.
type Investigation struct {
	// graph stores evidence spans and detects contradictions. It uses the
	// evidence-graph's local Span type (which mirrors evidence-span's shape)
	// for its TTL-aware contradiction detection.
	graph *evidencegraph.Graph

	// resolver maps span IDs to their full canonical evidencespan.Span values.
	// This is the authoritative span registry; evidence-graph is the contradiction
	// detector only.
	resolver evidenceresolver.Resolver

	// journal records every meaningful investigation step.
	journal investigationjournal.Journal

	// investigator advances the DAD (Decompose → Analyse → Decide) loop.
	investigator dadinvestigator.Investigator

	// dadState is the current DAD state machine state.
	dadState dadinvestigator.State

	// hypotheses holds the current ranked hypotheses.
	hypotheses []hypothesisranker.Hypothesis

	// flagRegister collects all concern flags raised during the investigation.
	flagRegister concernflags.Register

	// flagSeq is the monotonic counter for concern flag IDs.
	flagSeq int

	// findings accumulates all DAD decisions produced during Step calls.
	findings []dadinvestigator.Decision

	// findingEvidenceIDs records, for each finding (by index), the set of
	// evidence IDs that were active in the DAD step that produced it.
	// Parallel to findings slice.
	findingEvidenceIDs [][]string

	// pendingEvidence is evidence accumulated since the last Step call.
	pendingEvidence []dadinvestigator.EvidenceItem

	// pendingEvidenceIDs tracks the IDs of pendingEvidence items so that Step
	// can associate evidence with the finding it produced.
	pendingEvidenceIDs []string

	// spanConfidences tracks the confidence of every registered span so we can
	// compute the evidence-weighted average support for hypothesis ranking.
	spanConfidences []float64

	// now is a time source for deterministic testing.
	now func() time.Time
}

// New returns a ready-to-use Investigation.
//
// question is the human-readable investigation question (must be non-empty).
// It is recorded as the first journal entry.
func New(question string) (*Investigation, error) {
	if question == "" {
		return nil, fmt.Errorf("investigation: question must not be empty")
	}
	inv := &Investigation{
		graph: evidencegraph.New(),
		now:   time.Now,
	}
	// Open the investigation journal.
	if err := inv.journal.Append(investigationjournal.Entry{
		Step: "investigation-opened",
		Note: question,
		At:   inv.now(),
	}); err != nil {
		return nil, fmt.Errorf("investigation: opening journal: %w", err)
	}
	return inv, nil
}

// AddEvidence ingests a new piece of evidence into the investigation. The span
// is registered with the resolver (using the canonical evidencespan.Span type),
// added to the evidence graph for contradiction detection, and queued for the
// next DAD step. A journal entry is recorded for each addition.
func (inv *Investigation) AddEvidence(e EvidenceInput) error {
	if e.ID == "" {
		return fmt.Errorf("investigation: EvidenceInput.ID must not be empty")
	}

	// Build the canonical evidencespan.Span and register it with the resolver.
	// evidencespan.Span is the single source of truth for the span's identity;
	// evidence-graph receives a mirrored copy for its TTL-aware contradiction
	// detection only.
	span := evidencespan.Span{
		ID:         e.ID,
		EntityID:   e.EntityID,
		Field:      e.Field,
		Value:      e.Value,
		Confidence: e.Confidence,
		SourceID:   e.Source,
	}
	// Validate only when EntityID/Field/Value are provided; allow partial spans
	// (some evidence is content-only and does not describe a structured entity
	// attribute — e.g. a raw text excerpt with no field annotation).
	if span.EntityID != "" && span.Field != "" && span.Value != "" {
		if err := span.Validate(); err != nil {
			return fmt.Errorf("investigation: invalid evidence span: %w", err)
		}
	}
	inv.resolver.Register(span)

	// Mirror into the evidence graph for TTL-aware contradiction detection.
	// The evidence-graph's Span type is structurally identical to evidencespan.Span
	// for the fields it uses; ExpiresAt is left zero (spans never expire here).
	inv.graph.Add(evidencegraph.Span{
		ID:         e.ID,
		EntityID:   e.EntityID,
		Field:      e.Field,
		Value:      e.Value,
		Confidence: e.Confidence,
		Source:     e.Source,
	})

	// Record the span's confidence for hypothesis support computation.
	inv.spanConfidences = append(inv.spanConfidences, e.Confidence)

	// Queue as a DAD evidence item and track its ID for finding provenance.
	weight := e.Weight
	if weight <= 0 {
		weight = 1
	}
	inv.pendingEvidence = append(inv.pendingEvidence, dadinvestigator.EvidenceItem{
		ID:      e.ID,
		Content: e.Content,
		Weight:  weight,
	})
	inv.pendingEvidenceIDs = append(inv.pendingEvidenceIDs, e.ID)

	// Journal the addition.
	note := fmt.Sprintf("evidence added: id=%s source=%s confidence=%.2f", e.ID, e.Source, e.Confidence)
	if err := inv.journal.Append(investigationjournal.Entry{
		Step: "evidence-added",
		Note: note,
		At:   inv.now(),
	}); err != nil {
		return fmt.Errorf("investigation: journalling evidence: %w", err)
	}

	// Update or seed the primary hypothesis support. Support is the
	// evidence-weighted average confidence across all registered spans so that
	// high-confidence evidence drives the value toward 1.0.
	inv.updateHypothesisSupport()

	// Raise a concern flag when a contradiction is detected after adding this span.
	contradictions := inv.graph.Contradictions(inv.now())
	for _, c := range contradictions {
		flagID := fmt.Sprintf("CF-%03d", inv.flagSeq+1)
		// Avoid raising duplicate flags for the same contradiction.
		existing := inv.flagRegister.List()
		duplicate := false
		for _, f := range existing {
			if f.Target == c.EntityID+"/"+c.Field {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		inv.flagSeq++
		flag := concernflags.ConcernFlag{
			ID:        flagID,
			Type:      "contradiction",
			Severity:  concernflags.SeverityMedium,
			Target:    c.EntityID + "/" + c.Field,
			Rationale: fmt.Sprintf("contradicting values for entity=%s field=%s: %q vs %q", c.EntityID, c.Field, c.A.Value, c.B.Value),
		}
		_ = inv.flagRegister.Raise(flag)
	}

	return nil
}

// Step advances the investigation by one DAD phase. It feeds all pending
// evidence into the DAD investigator and journals the resulting decision. Call
// Step repeatedly until Status returns Verdict "converged".
//
// Step is a no-op (returning nil) when the DAD machine has reached a terminal
// phase (PhaseFound or PhaseExhausted).
func (inv *Investigation) Step() error {
	if inv.dadState.Phase == dadinvestigator.PhaseFound ||
		inv.dadState.Phase == dadinvestigator.PhaseExhausted {
		return nil
	}

	evidence := inv.pendingEvidence
	inv.pendingEvidence = nil

	// Capture the evidence IDs before clearing pendingEvidenceIDs.
	evidenceIDs := make([]string, len(inv.pendingEvidenceIDs))
	copy(evidenceIDs, inv.pendingEvidenceIDs)
	inv.pendingEvidenceIDs = nil

	nextState, decision := inv.investigator.Step(inv.dadState, evidence)
	inv.dadState = nextState
	inv.findings = append(inv.findings, decision)
	inv.findingEvidenceIDs = append(inv.findingEvidenceIDs, evidenceIDs)

	// If no evidence was linked to this finding, raise a finding_without_evidence
	// concern flag. See types.go for the empty-evidence semantics.
	if len(evidenceIDs) == 0 {
		inv.flagSeq++
		flagID := fmt.Sprintf("CF-%03d", inv.flagSeq)
		flag := concernflags.ConcernFlag{
			ID:       flagID,
			Type:     "finding_without_evidence",
			Severity: concernflags.SeverityHigh,
			Target:   fmt.Sprintf("finding[%d]", len(inv.findings)-1),
			Rationale: fmt.Sprintf(
				"DAD step produced finding %q without any linked evidence items — "+
					"this finding cannot be cited in a governance report",
				decision.Finding,
			),
		}
		_ = inv.flagRegister.Raise(flag)
	}

	// Re-rank hypotheses after each DAD step.
	inv.hypotheses = hypothesisranker.Rank(inv.hypotheses)

	// Journal the step.
	note := fmt.Sprintf("phase=%s finding=%s confidence=%s terminal=%v",
		nextState.Phase, decision.Finding, decision.Confidence, decision.Terminal)
	if err := inv.journal.Append(investigationjournal.Entry{
		Step: "dad-step",
		Note: note,
		At:   inv.now(),
	}); err != nil {
		return fmt.Errorf("investigation: journalling step: %w", err)
	}

	return nil
}

// Status returns the current convergence status of the investigation.
// It uses the convergence oracle to decide whether more steps are needed.
func (inv *Investigation) Status() Status {
	// Compute agreement as the average support of ranked hypotheses.
	agreement := 0.0
	if len(inv.hypotheses) > 0 {
		sum := 0.0
		for _, h := range inv.hypotheses {
			sum += h.Support
		}
		agreement = sum / float64(len(inv.hypotheses))
	}

	verdict := convergenceoracle.Assess(len(inv.findings), agreement)

	contradictions := inv.graph.Contradictions(inv.now())

	return Status{
		Verdict:            string(verdict),
		Steps:              inv.dadState.Steps,
		Phase:              string(inv.dadState.Phase),
		FindingCount:       len(inv.findings),
		ContradictionCount: len(contradictions),
	}
}

// Findings returns all governance-annotated findings accumulated so far.
// Each Finding wraps a DAD Decision with its supporting evidence IDs and any
// concern flags raised during the investigation.
//
// EvidenceDegraded is set to true for any finding produced without linked
// evidence. Callers must inspect this field before citing a finding.
func (inv *Investigation) Findings() []Finding {
	allFlags := inv.flagRegister.List()
	out := make([]Finding, len(inv.findings))
	for i, d := range inv.findings {
		evidIDs := inv.findingEvidenceIDs[i]
		degraded := len(evidIDs) == 0
		out[i] = Finding{
			Text:                  d.Finding,
			Confidence:            d.Confidence,
			Terminal:              d.Terminal,
			ConcernFlags:          allFlags,
			SupportingEvidenceIDs: evidIDs,
			EvidenceDegraded:      degraded,
		}
	}
	return out
}

// JournalEntries returns a snapshot of the investigation journal in append order.
func (inv *Investigation) JournalEntries() []investigationjournal.Entry {
	return inv.journal.Entries()
}

// updateHypothesisSupport recomputes the primary hypothesis support as the
// unweighted average confidence of all registered evidence spans. This means
// that consistently high-confidence evidence drives support toward 1.0,
// enabling the convergence oracle to declare convergence.
func (inv *Investigation) updateHypothesisSupport() {
	if len(inv.spanConfidences) == 0 {
		return
	}
	sum := 0.0
	for _, c := range inv.spanConfidences {
		sum += c
	}
	avgConfidence := sum / float64(len(inv.spanConfidences))

	if len(inv.hypotheses) == 0 {
		inv.hypotheses = append(inv.hypotheses, hypothesisranker.Hypothesis{
			ID:        "H-001",
			Statement: "primary investigation hypothesis",
			Support:   avgConfidence,
		})
	} else {
		// Update the primary hypothesis support; leave any others unchanged.
		inv.hypotheses[0].Support = avgConfidence
	}
}

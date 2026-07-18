package governedinvestigation

import (
	"fmt"
	"time"

	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	dad "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator"
	hypothesisranker "github.com/geoffmilleraz/agentic-kit/foundations/investigation/hypothesis-ranker"
	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
)

// seedSpans is the fixed evidence corpus for this scenario.
// All weights are chosen so that the accumulated total exceeds the High
// threshold (30) in a single DAD loop.
var seedSpans = []evidencespan.Span{
	{
		ID:         "span-001",
		EntityID:   "payments-service",
		Field:      "owner",
		Value:      "team-payments",
		Confidence: 0.95,
		SourceID:   "pagerduty-feed",
	},
	{
		ID:         "span-002",
		EntityID:   "payments-service",
		Field:      "error-rate",
		Value:      "elevated-2x",
		Confidence: 0.90,
		SourceID:   "metrics-feed",
	},
	{
		ID:         "span-003",
		EntityID:   "payments-service",
		Field:      "deployment",
		Value:      "v2.4.1-deployed-at-14:02",
		Confidence: 0.98,
		SourceID:   "deploy-log",
	},
	{
		ID:         "span-004",
		EntityID:   "payments-service",
		Field:      "latency-p99",
		Value:      "2300ms",
		Confidence: 0.88,
		SourceID:   "metrics-feed",
	},
}

// seedHypotheses is the fixed set of candidate explanations under
// investigation. Support values are seeded deterministically.
var seedHypotheses = []hypothesisranker.Hypothesis{
	{
		ID:        "h-001",
		Statement: "Deployment v2.4.1 introduced a regression in the payments hot path",
		Support:   0.88,
	},
	{
		ID:        "h-002",
		Statement: "Upstream dependency rate-limiting caused cascading latency",
		Support:   0.45,
	},
	{
		ID:        "h-003",
		Statement: "Traffic spike from marketing campaign exceeded capacity",
		Support:   0.30,
	},
}

// spansToDADEvidence converts evidence spans to DAD investigator evidence items.
// Weight is derived from the span's confidence (scaled to [1, 10]).
func spansToDADEvidence(spans []evidencespan.Span) []dad.EvidenceItem {
	items := make([]dad.EvidenceItem, len(spans))
	for i, s := range spans {
		w := int(s.Confidence*10 + 0.5) // round confidence×10 to nearest int
		if w < 1 {
			w = 1
		}
		items[i] = dad.EvidenceItem{
			ID:      s.ID,
			Content: fmt.Sprintf("%s.%s=%s", s.EntityID, s.Field, s.Value),
			Weight:  w,
		}
	}
	return items
}

// Run executes the governed investigation scenario end-to-end and returns a
// [Report].
//
// The scenario is fully seeded and deterministic — identical invocations always
// produce Reports with the same Finding, RankedHypotheses, and Journal steps.
// It uses no external data, network calls, or time.Now() for any
// investigation-affecting logic.
func Run() Report {
	// Use a fixed reference time for journal entries so the report is
	// byte-for-byte reproducible.
	ref := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	var journal investigationjournal.Journal

	// 1. Record investigation start.
	_ = journal.Append(investigationjournal.Entry{
		Step: "investigation-started",
		Note: "Governed investigation scenario started with seeded corpus",
		At:   ref,
	})

	// 2. Validate and register evidence spans.
	for _, s := range seedSpans {
		if err := s.Validate(); err != nil {
			panic(fmt.Sprintf("governed-investigation: seed span %q is invalid: %v", s.ID, err))
		}
	}
	_ = journal.Append(investigationjournal.Entry{
		Step: "evidence-collected",
		Note: fmt.Sprintf("Collected %d evidence spans from seeded corpus", len(seedSpans)),
		At:   ref.Add(1 * time.Second),
	})

	// 3. Rank hypotheses.
	ranked := hypothesisranker.Rank(seedHypotheses)
	_ = journal.Append(investigationjournal.Entry{
		Step: "hypotheses-ranked",
		Note: fmt.Sprintf("Ranked %d hypotheses; top: %q (support=%.2f)",
			len(ranked), ranked[0].ID, ranked[0].Support),
		At: ref.Add(2 * time.Second),
	})

	// 4. Convert spans to DAD evidence items and run the DAD loop to termination.
	evidence := spansToDADEvidence(seedSpans)

	investigator := dad.Investigator{}
	state := dad.State{}
	var decision dad.Decision

	const maxSteps = 50
	for step := 0; step < maxSteps; step++ {
		var ev []dad.EvidenceItem
		// Feed all evidence on the first Decompose step only.
		if step == 0 {
			ev = evidence
		}
		state, decision = investigator.Step(state, ev)
		if decision.Terminal {
			break
		}
	}

	_ = journal.Append(investigationjournal.Entry{
		Step: "investigation-complete",
		Note: fmt.Sprintf("DAD loop terminated: phase=%s confidence=%s finding=%q",
			state.Phase, decision.Confidence, decision.Finding),
		At: ref.Add(3 * time.Second),
	})

	// 5. Assemble report.
	entries := journal.Entries()
	journalOut := make([]JournalEntry, len(entries))
	for i, e := range entries {
		journalOut[i] = JournalEntry{Step: e.Step, Note: e.Note}
	}

	topH := ranked[0]

	return Report{
		Journal:          journalOut,
		RankedHypotheses: ranked,
		Finding: Finding{
			Summary:       decision.Finding,
			Confidence:    decision.Confidence,
			TopHypothesis: topH,
		},
		EvidenceCount: len(seedSpans),
	}
}

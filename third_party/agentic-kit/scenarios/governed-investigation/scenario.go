package governedinvestigation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	dad "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator"
	hypothesisranker "github.com/geoffmilleraz/agentic-kit/foundations/investigation/hypothesis-ranker"
	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
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

	// 5. Remediation through the strict effect gate. The finding implies one
	// irreversible action — rolling v2.4.1 back — and the scenario proposes it
	// the governed way: a scripted model calls rollback_deployment inside
	// agent-harness with EffectPolicy strict; the tool declares
	// DestructiveHint, so the harness routes the call to the (seeded) human
	// approver before it runs, and records the approval on the journal and
	// the sink. The read-only read_deploy_log call it makes first dispatches
	// without a token.
	_ = journal.Append(investigationjournal.Entry{
		Step: "remediation-proposed",
		Note: fmt.Sprintf("Proposing rollback of %s under the strict effect gate", ranked[0].ID),
		At:   ref.Add(4 * time.Second),
	})
	remediation := runRemediation(&journal, ref.Add(5*time.Second))
	_ = journal.Append(investigationjournal.Entry{
		Step: "remediation-complete",
		Note: fmt.Sprintf("Remediation %s: tool=%s effect=%s approved=%t by %q; gate events approved=%d denied=%d",
			remediation.Stop, remediation.ToolName, remediation.Effect, remediation.Approved, remediation.ApprovedBy,
			remediation.GateApproved, remediation.GateDenied),
		At: ref.Add(6 * time.Second),
	})

	// 6. Assemble report.
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
		Remediation:   remediation,
	}
}

// remediationTool is the one destructive tool of the scenario.
const remediationTool = "rollback_deployment"

// newRemediationRegistry builds the two-tool table the remediation model
// sees: a read-only deploy-log reader and the destructive rollback. Both
// declare their Effect, which is what lets the strict gate wave the read
// through and stop on the rollback.
func newRemediationRegistry() *toolregistry.Registry {
	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "read_deploy_log",
			Description: "Read the recent deployment log for a service.",
			InputSchema: toolregistry.ObjectSchema(toolregistry.Prop{Name: "service", Type: "string", Required: true}),
		},
		Effect: &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
		Handler: func(context.Context, json.RawMessage) (string, error) {
			return "v2.4.0 deployed 13:10; v2.4.1 deployed 14:02", nil
		},
	})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        remediationTool,
			Description: "Roll a service back to a previous version. IRREVERSIBLE for in-flight traffic.",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "service", Type: "string", Required: true},
				toolregistry.Prop{Name: "to", Type: "string", Required: true},
			),
		},
		Effect: &toolregistry.Effect{DestructiveHint: true},
		Handler: func(_ context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Service string `json:"service"`
				To      string `json:"to"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", err
			}
			return fmt.Sprintf("rolled %s back to %s", in.Service, in.To), nil
		},
	})
	return reg
}

// remediationScript is the deterministic model: read the log, roll back,
// report.
func remediationScript() *llmclient.MockClient {
	return llmclient.NewMockClient(
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("toolu_rem_1", "read_deploy_log", json.RawMessage(`{"service":"payments-service"}`)),
			},
			StopReason: llmclient.StopToolUse,
		}),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("toolu_rem_2", remediationTool, json.RawMessage(`{"service":"payments-service","to":"v2.4.0"}`)),
			},
			StopReason: llmclient.StopToolUse,
		}),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Text:       "Rolled payments-service back to v2.4.0.",
			StopReason: llmclient.StopEndTurn,
		}),
	)
}

// runRemediation dispatches the scripted remediation through agent-harness
// under EffectPolicy strict, journaling into journal with a fixed clock so
// the run is byte-for-byte reproducible. The scenario is seeded: an
// infrastructure error here is a bug, so it panics rather than returning.
func runRemediation(journal *investigationjournal.Journal, at time.Time) Remediation {
	ms := &sink.MemorySink{}
	h, err := agentharness.New(
		agentharness.Config{
			MaxTurns:     4,
			EffectPolicy: agentharness.EffectStrict,
		},
		agentharness.Deps{
			Client:   remediationScript(),
			Registry: newRemediationRegistry(),
			Sink:     ms,
			Journal:  journal,
			Now:      func() time.Time { return at },
			// The seeded human: approves the rollback on the strength of the
			// high-confidence finding. A real consumer wires a CLI prompt, an
			// approval endpoint, or a queue here — the harness does not care
			// which, only that Approved was granted for THIS call.
			Approver: agentharness.ApproverFunc(func(_ context.Context, req agentharness.ApprovalRequest) (agentharness.Approval, error) {
				if req.Call.ToolName != remediationTool {
					return agentharness.Approval{Reason: "only the rollback is pre-approved in this scenario"}, nil
				}
				return agentharness.Approval{Approved: true, ApprovedBy: "on-call reviewer (seeded)", Reason: "h-001 at high confidence"}, nil
			}),
		},
	)
	if err != nil {
		panic(fmt.Sprintf("governed-investigation: harness: %v", err))
	}
	res, err := h.Run(context.Background(), "Remediate the payments-service regression from deployment v2.4.1.")
	if err != nil {
		panic(fmt.Sprintf("governed-investigation: remediation run: %v", err))
	}

	rem := Remediation{ToolName: remediationTool, Stop: string(res.Stop), Summary: res.FinalText}
	for _, e := range ms.Events() {
		switch e.Attributes[agentharness.AttrStep] {
		case agentharness.StepGateApproved:
			rem.GateApproved++
			rem.Approved = true
			rem.ApprovedBy = e.Attributes[agentharness.AttrApprovedBy]
			rem.Effect = e.Attributes[agentharness.AttrEffect]
		case agentharness.StepGateDenied:
			rem.GateDenied++
			rem.Effect = e.Attributes[agentharness.AttrEffect]
		}
	}
	return rem
}

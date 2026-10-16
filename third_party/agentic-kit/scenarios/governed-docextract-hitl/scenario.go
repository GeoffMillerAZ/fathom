package governeddocextracthitl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	investigationjournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/docextract"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/hitl"
)

// csvAttachment is the seeded CSV document the agent receives. Using CSV keeps
// the scenario in the root module (zero external deps): docextract handles CSV
// natively without any subpackage handler.
const csvAttachment = `department,budget_code,allocation
engineering,ENG-001,120000
product,PRD-002,85000
design,DES-003,42000
`

// allowedOps is the hitl whitelist. The agent may only propose "update_field";
// any other kind is rejected by the guard before it reaches the applier.
var allowedOps = hitl.NewOpSet("update_field")

// fixedClock returns a clock function pinned to ref so every timestamp in the
// scenario is deterministic.
func fixedClock(ref time.Time) func() time.Time {
	return func() time.Time { return ref }
}

// Run executes the governed docextract→harness→hitl scenario end-to-end and
// returns a [Report]. It is fully seeded and deterministic: identical calls
// always produce Reports with the same fields. It uses no network and no
// time.Now() for any decision-affecting path.
func Run() Report {
	ref := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)
	clk := fixedClock(ref)

	// ── Step 1: docextract ────────────────────────────────────────────────────
	// Extract the CSV attachment into prompt-ready text. No handler needed:
	// the core package handles CSV/text natively.
	extracted, err := docextract.Extract("budget.csv", "text/csv", []byte(csvAttachment))
	if err != nil {
		panic(fmt.Sprintf("governed-docextract-hitl: docextract failed: %v", err))
	}

	// ── Step 2: hitl engine ───────────────────────────────────────────────────
	// Define the document state, the applier, and build the engine.
	initialState := State{Fields: map[string]string{
		"department": "engineering",
		"budget":     "120000",
		"status":     "draft",
	}}

	applier := hitl.Applier[State, Op](func(st *State, op Op) (string, error) {
		if op.Kind != "update_field" {
			return "", fmt.Errorf("hitl: unknown op kind %q", op.Kind)
		}
		if op.Field == "" {
			return "", fmt.Errorf("hitl: update_field requires a non-empty Field")
		}
		old := st.Fields[op.Field]
		st.Fields[op.Field] = op.Value
		return fmt.Sprintf("updated %q: %q → %q", op.Field, old, op.Value), nil
	})

	cloner := hitl.Cloner[State](func(s State) State {
		cp := State{Fields: make(map[string]string, len(s.Fields))}
		for k, v := range s.Fields {
			cp.Fields[k] = v
		}
		return cp
	})

	gauges := func(s State) []hitl.Gauge {
		return []hitl.Gauge{
			{Key: "status", Label: "Status", Value: s.Fields["status"]},
			{Key: "budget", Label: "Budget", Value: s.Fields["budget"]},
		}
	}

	engine := hitl.New(
		applier, cloner,
		hitl.WithGauges[State, Op](gauges),
		hitl.WithGuard[State, Op](hitl.Guard(allowedOps, func(op Op) string { return op.Kind })),
	)

	// ── Step 3: budget guard ─────────────────────────────────────────────────
	// Wire a budget guard. With generous limits it will allow all calls; the
	// test asserts BudgetUsed to prove the guard actually ran.
	budget := budgetguard.New(budgetguard.Config{
		MaxReqPerMin: 60,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 100.0,
	}, clk)

	// ── Step 4: journal ───────────────────────────────────────────────────────
	journal := investigationjournal.NewJournalWithClock(clk)

	// ── Step 5: tool registry ─────────────────────────────────────────────────
	// The agent has one tool: propose_change. It receives the agent's Op JSON,
	// runs hitl.Preview (dry-run), and returns the summary to the model.
	// The actual Apply happens OUTSIDE the tool, after the scenario scripts a
	// scripted-approver "approval". This mirrors real usage: the tool returns a
	// preview for the model to report, and a separate approval path commits it.
	var capturedPreview hitl.Preview
	var capturedOp Op

	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "propose_change",
			Description: "Propose a field update to the document. Returns a dry-run preview.",
			InputSchema: toolregistry.ObjectSchema(
				toolregistry.Prop{Name: "kind", Type: "string", Required: true, Description: "Operation kind (must be 'update_field')."},
				toolregistry.Prop{Name: "field", Type: "string", Required: true, Description: "Document field to update."},
				toolregistry.Prop{Name: "value", Type: "string", Required: true, Description: "New value for the field."},
			),
		},
		Handler: func(_ context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Kind  string `json:"kind"`
				Field string `json:"field"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("propose_change: invalid input: %w", err)
			}
			op := Op{Kind: in.Kind, Field: in.Field, Value: in.Value}
			pv := engine.Preview(cloner(initialState), op)
			if !pv.OK {
				return "", fmt.Errorf("propose_change: preview rejected: %s", pv.Err)
			}
			capturedOp = op
			capturedPreview = pv
			return pv.Summary, nil
		},
	})

	// ── Step 6: MockClient script ─────────────────────────────────────────────
	// Turn 0: model reads the extracted CSV text and calls propose_change.
	// Turn 1: model receives the preview summary and emits the run report.
	mockClient := llmclient.NewMockClient(
		llmclient.MockEntryWithResponse(llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.TextBlock("I'll analyse the budget data and propose a status update."),
				llmclient.ToolUseBlock(
					"toolu_01", "propose_change",
					json.RawMessage(`{"kind":"update_field","field":"status","value":"approved"}`),
				),
			},
			StopReason: llmclient.StopToolUse,
			Usage:      llmclient.Usage{InputTokens: 420, OutputTokens: 38},
		}),
		llmclient.MockEntryWithResponse(llmclient.Response{
			Text:       "Run complete. The budget document for engineering was reviewed and the status field was updated from 'draft' to 'approved' based on the CSV data. The change was previewed and approved.",
			StopReason: llmclient.StopEndTurn,
			Usage:      llmclient.Usage{InputTokens: 510, OutputTokens: 42},
		}),
	)

	// ── Step 7: harness ───────────────────────────────────────────────────────
	h, err := agentharness.New(
		agentharness.Config{
			MaxTurns:            8,
			System:              "You are a document governance agent. Analyse the attached CSV and propose a status update using the propose_change tool.",
			MaxTokensPerCall:    1024,
			EstimatedUSDPerCall: 0.001,
		},
		agentharness.Deps{
			Client:   mockClient,
			Registry: reg,
			Budget:   budget,
			Journal:  journal,
			Now:      clk,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("governed-docextract-hitl: harness.New: %v", err))
	}

	// The user message includes the extracted text to prove docextract feeds harness.
	userMsg := fmt.Sprintf("Review the following budget data and propose an appropriate status update:\n\n%s", extracted)
	result, runErr := h.Run(context.Background(), userMsg)
	if runErr != nil {
		panic(fmt.Sprintf("governed-docextract-hitl: harness.Run: %v", runErr))
	}

	// ── Step 8: scripted approver → hitl Apply ────────────────────────────────
	// A real system would gate Apply on a human click. Here the scenario
	// scripts an automatic approval to prove the full propose→preview→apply loop.
	live := cloner(initialState)
	applySummary, applyErr := engine.Apply(&live, capturedOp)
	if applyErr != nil {
		panic(fmt.Sprintf("governed-docextract-hitl: hitl.Apply: %v", applyErr))
	}
	_ = applySummary // summary matches capturedPreview.Summary; covered in test

	// ── Step 9: count budget charges ─────────────────────────────────────────
	snap := budget.Snapshot()

	// ── Step 10: assemble report ──────────────────────────────────────────────
	journalEntries := journal.Entries()

	return Report{
		ExtractedText:  extracted,
		Journal:        journalEntries,
		PreviewSummary: capturedPreview.Summary,
		AppliedState:   live,
		RunReport:      result.FinalText,
		Turns:          result.Turns,
		BudgetUsed:     snap.ReqThisDay,
	}
}

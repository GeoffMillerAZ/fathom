package agentharness

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/injection-detector"
)

func engineeringProfileForTest(t *testing.T, attempt AttemptExecutor) *EngineeringProfile {
	t.Helper()
	reg := toolregistry.New(toolregistry.Options{})
	if err := reg.Register(toolregistry.Tool{
		Def:     llmclient.ToolDef{Name: "allowed", Description: "test"},
		Effect:  &toolregistry.Effect{ReadOnlyHint: true},
		Handler: func(context.Context, json.RawMessage) (string, error) { return "ok", nil },
	}); err != nil {
		t.Fatal(err)
	}
	detector, err := injectiondetector.NewDetector(nil)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := NewEngineeringProfile(EngineeringProfileConfig{
		Harness:     Config{MaxTurns: 2, MaxTokensPerCall: 256, EffectPolicy: EffectStrict, InjectionAction: InjectionBlock},
		MaxAttempts: 2, ReservedUSDPerAttempt: 0.1, AllowedTools: []string{"allowed"},
		Retryable: func(error) bool { return true },
	}, EngineeringProfileDeps{
		Registry: reg, Detector: detector,
		Budget:   budgetguard.New(budgetguard.Config{MaxReqPerMin: 10, MaxReqPerDay: 10, MaxUSDPerDay: 10}, func() time.Time { return time.Unix(0, 0) }),
		Approver: ApproverFunc(func(context.Context, ApprovalRequest) (Approval, error) { return Approval{Approved: true}, nil }),
		Attempts: attempt,
		Evidence: CompletionEvidenceVerifierFunc(func(context.Context, RunResult) (CompletionEvidence, error) {
			return CompletionEvidence{ID: "independent-test", Independent: true}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewEngineeringProfile: %v", err)
	}
	return profile
}

func TestEngineeringProfileValidation(t *testing.T) {
	attempt := AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		return llmclient.Response{}, AttemptUsage{}, nil
	})
	tests := []struct {
		name string
		edit func(*EngineeringProfileConfig, *EngineeringProfileDeps)
	}{
		{"missing attempt boundary", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) { d.Attempts = nil }},
		{"missing independent evidence", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) { d.Evidence = nil }},
		{"missing effect authorization", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) { d.Approver = nil }},
		{"permissive effect policy", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) {
			c.Harness.EffectPolicy = EffectPermissive
		}},
		{"truncation accepted as completion", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) {
			no := false
			c.Harness.TreatMaxTokensAsError = &no
		}},
		{"invalid harness configuration", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) {
			c.Harness.MaxTurns = 0
		}},
		{"caller supplied scope hook", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) {
			c.Harness.Hooks = &HookConfig{PreDispatch: func(context.Context, ToolCallView) (bool, string, error) {
				return true, "", nil
			}}
		}},
		{"scope bypass rejected", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) { c.AllowedTools = []string{"outside"} }},
		{"invalid attempt bound", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) { c.MaxAttempts = 0 }},
		{"missing model token bound", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) { c.Harness.MaxTokensPerCall = 0 }},
		{"non-finite reservation", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) { c.ReservedUSDPerAttempt = math.NaN() }},
		{"infinite reservation", func(c *EngineeringProfileConfig, _ *EngineeringProfileDeps) { c.ReservedUSDPerAttempt = math.Inf(1) }},
		{"typed nil approver", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) { d.Approver = ApproverFunc(nil) }},
		{"typed nil attempt executor", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) { d.Attempts = AttemptExecutorFunc(nil) }},
		{"typed nil evidence verifier", func(_ *EngineeringProfileConfig, d *EngineeringProfileDeps) {
			d.Evidence = CompletionEvidenceVerifierFunc(nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := toolregistry.New(toolregistry.Options{})
			reg.MustRegister(toolregistry.Tool{Def: llmclient.ToolDef{Name: "allowed"}, Handler: func(context.Context, json.RawMessage) (string, error) { return "", nil }})
			detector, _ := injectiondetector.NewDetector(nil)
			cfg := EngineeringProfileConfig{Harness: Config{MaxTurns: 1, MaxTokensPerCall: 256, EffectPolicy: EffectStrict, InjectionAction: InjectionBlock}, MaxAttempts: 1, ReservedUSDPerAttempt: 0.1, AllowedTools: []string{"allowed"}}
			deps := EngineeringProfileDeps{Registry: reg, Detector: detector, Budget: budgetguard.New(budgetguard.Config{MaxReqPerMin: 2, MaxReqPerDay: 2, MaxUSDPerDay: 2}, func() time.Time { return time.Unix(0, 0) }), Approver: ApproverFunc(func(context.Context, ApprovalRequest) (Approval, error) { return Approval{Approved: true}, nil }), Attempts: attempt, Evidence: CompletionEvidenceVerifierFunc(func(context.Context, RunResult) (CompletionEvidence, error) {
				return CompletionEvidence{ID: "e", Independent: true}, nil
			})}
			tt.edit(&cfg, &deps)
			if _, err := NewEngineeringProfile(cfg, deps); err == nil {
				t.Fatal("NewEngineeringProfile error = nil")
			}
		})
	}
}

func TestEngineeringProfileOwnsTruncationPolicy(t *testing.T) {
	p := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		return llmclient.Response{Content: []llmclient.ContentBlock{{Type: "text", Text: "partial"}}, StopReason: llmclient.StopMaxTokens}, AttemptUsage{}, nil
	}))
	cfg, enabled := p.cfg, true
	cfg.Harness.TreatMaxTokensAsError = &enabled
	p, err := NewEngineeringProfile(cfg, p.deps)
	if err != nil {
		t.Fatal(err)
	}
	enabled = false // The caller no longer owns the validated profile's policy.
	result, err := p.Run(context.Background(), "synthetic")
	if err != nil || result.Run.Stop != StopMaxTokens || result.Coverage.Controls["completion_evidence"] == ControlPass {
		t.Fatalf("mutated policy certified truncation: %+v %v", result, err)
	}
}

func TestEngineeringProfileInvalidActualLeavesReservationConservative(t *testing.T) {
	for _, actual := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -0.01} {
		actual := actual
		t.Run("invalid", func(t *testing.T) {
			profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
				return llmclient.Response{}, AttemptUsage{ActualUSD: &actual}, nil
			}))
			result, err := profile.Run(context.Background(), "work")
			if err == nil {
				t.Fatal("invalid actual USD passed")
			}
			got := result.Coverage.Accounting
			if got.Attempts != 1 || got.ReservedUSD != 0.1 || got.SpentUSD != 0 || got.UnknownActuals != 0 || got.InvalidActuals != 1 {
				t.Fatalf("accounting = %+v, want conservative invalid-actual record", got)
			}
			if snap := profile.deps.Budget.Snapshot(); snap.USDThisDay != 0.1 || math.IsNaN(snap.USDThisDay) {
				t.Fatalf("budget snapshot = %+v, want retained .1 reservation", snap)
			}
		})
	}
}

func TestEngineeringProfileCancellationAfterAttemptOrEvidenceCannotPass(t *testing.T) {
	for _, stage := range []string{"attempt", "evidence"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
				if stage == "attempt" {
					cancel()
				}
				return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
			}))
			if stage == "evidence" {
				profile.deps.Evidence = CompletionEvidenceVerifierFunc(func(context.Context, RunResult) (CompletionEvidence, error) {
					cancel()
					return CompletionEvidence{ID: "independent", Independent: true}, nil
				})
			}
			result, err := profile.Run(ctx, "work")
			if !errors.Is(err, context.Canceled) || result.Coverage.Controls["context"] != ControlFail || result.Coverage.Controls["completion_evidence"] != ControlFail {
				t.Fatalf("result=%+v err=%v, want canceled non-passing receipt", result, err)
			}
		})
	}
}

func TestEngineeringProfileReceiptLeavesUnrunControlsUnknown(t *testing.T) {
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
	}))
	result, err := profile.Run(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	for _, control := range []string{"detector", "effect_policy", "scope", "registry"} {
		if result.Coverage.Controls[control] != ControlUnknown {
			t.Errorf("%s = %q, want unknown without a tool call", control, result.Coverage.Controls[control])
		}
	}
	for _, control := range []string{"context", "budget", "model_attempts", "completion_evidence"} {
		if result.Coverage.Controls[control] != ControlPass {
			t.Errorf("%s = %q, want pass after observation", control, result.Coverage.Controls[control])
		}
	}
}

func TestEngineeringProfileBoundsAttemptsAndReportsAccounting(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	unknown := AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return llmclient.Response{}, AttemptUsage{}, errors.New("retry")
		}
		spent := 0.03
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{ActualUSD: &spent}, nil
	})
	result, err := engineeringProfileForTest(t, unknown).Run(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Coverage.Accounting.Attempts != 2 {
		t.Errorf("attempts = calls %d accounting %+v, want 2", calls, result.Coverage.Accounting)
	}
	if result.Coverage.Accounting.ReservedUSD != 0.2 || result.Coverage.Accounting.SpentUSD != 0.03 || result.Coverage.Accounting.UnknownActuals != 1 {
		t.Errorf("accounting = %+v, want reserved .2 spent .03 unknown 1", result.Coverage.Accounting)
	}
	if result.Coverage.Controls["completion_evidence"] != ControlPass {
		t.Errorf("evidence control = %q, want pass", result.Coverage.Controls["completion_evidence"])
	}
}

func TestEngineeringProfileCompletionNeedsIndependentEvidence(t *testing.T) {
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
	}))
	profile.deps.Evidence = CompletionEvidenceVerifierFunc(func(context.Context, RunResult) (CompletionEvidence, error) {
		return CompletionEvidence{ID: "model-said-so"}, nil
	})
	result, err := profile.Run(context.Background(), "work")
	if err == nil || result.Coverage.Controls["completion_evidence"] != ControlFail {
		t.Fatalf("result=%+v err=%v, want independent-evidence failure", result, err)
	}
}

func TestEngineeringProfileCancellationMakesNoAttempt(t *testing.T) {
	calls := 0
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		calls++
		return llmclient.Response{}, AttemptUsage{}, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := profile.Run(ctx, "work")
	if !errors.Is(err, context.Canceled) || calls != 0 || result.Run.Stop != StopCanceled {
		t.Fatalf("result=%+v calls=%d err=%v, want canceled before attempt", result, calls, err)
	}
}

func TestEngineeringProfileScopeBlocksToolBypass(t *testing.T) {
	calls := 0
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		calls++
		if calls == 1 {
			return llmclient.Response{
				Content:    []llmclient.ContentBlock{llmclient.ToolUseBlock("outside-id", "outside", nil)},
				StopReason: llmclient.StopToolUse,
			}, AttemptUsage{}, nil
		}
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
	}))
	result, err := profile.Run(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Run.ToolCalls != 1 || result.Coverage.Controls["scope"] != ControlPass ||
		result.Coverage.Controls["effect_policy"] != ControlUnknown || result.Coverage.Controls["detector"] != ControlUnknown ||
		result.Coverage.Controls["registry"] != ControlUnknown {
		t.Fatalf("result=%+v calls=%d, want denied outside tool then completion", result, calls)
	}
	for _, msg := range result.Run.Messages {
		for _, block := range msg.Content {
			if block.Type == llmclient.BlockToolResult && !block.IsError {
				t.Fatal("outside-scope tool unexpectedly dispatched")
			}
		}
	}
}

func TestEngineeringProfileStrictEffectGateDeniesUnapprovedTool(t *testing.T) {
	reg := toolregistry.New(toolregistry.Options{})
	handlerCalls := 0
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{Name: "requires-approval"},
		Handler: func(context.Context, json.RawMessage) (string, error) {
			handlerCalls++
			return "should not run", nil
		},
	})
	detector, err := injectiondetector.NewDetector(nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	profile, err := NewEngineeringProfile(EngineeringProfileConfig{
		Harness:     Config{MaxTurns: 2, MaxTokensPerCall: 256, EffectPolicy: EffectStrict, InjectionAction: InjectionBlock},
		MaxAttempts: 1, ReservedUSDPerAttempt: 0.1, AllowedTools: []string{"requires-approval"},
	}, EngineeringProfileDeps{
		Registry: reg, Detector: detector,
		Budget:   budgetguard.New(budgetguard.Config{MaxReqPerMin: 10, MaxReqPerDay: 10, MaxUSDPerDay: 10}, func() time.Time { return time.Unix(0, 0) }),
		Approver: ApproverFunc(func(context.Context, ApprovalRequest) (Approval, error) { return Approval{}, nil }),
		Attempts: AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
			calls++
			if calls == 1 {
				return llmclient.Response{Content: []llmclient.ContentBlock{
					llmclient.ToolUseBlock("approval-id", "requires-approval", nil),
				}, StopReason: llmclient.StopToolUse}, AttemptUsage{}, nil
			}
			return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
		}),
		Evidence: CompletionEvidenceVerifierFunc(func(context.Context, RunResult) (CompletionEvidence, error) {
			return CompletionEvidence{ID: "independent-test", Independent: true}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := profile.Run(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if handlerCalls != 0 {
		t.Fatalf("handler calls = %d, want strict gate denial before dispatch", handlerCalls)
	}
	for _, msg := range result.Run.Messages {
		for _, block := range msg.Content {
			if block.Type == llmclient.BlockToolResult && !block.IsError {
				t.Fatal("unapproved tool unexpectedly dispatched")
			}
		}
	}
	if result.Coverage.Controls["scope"] != ControlPass || result.Coverage.Controls["effect_policy"] != ControlPass ||
		result.Coverage.Controls["detector"] != ControlUnknown || result.Coverage.Controls["registry"] != ControlUnknown {
		t.Fatalf("coverage=%+v, want effect observed but no dispatched/screened tool", result.Coverage.Controls)
	}
}

func TestEngineeringProfileObservedControlsForDispatchedTool(t *testing.T) {
	calls := 0
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		calls++
		if calls == 1 {
			return llmclient.Response{Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("allowed-id", "allowed", json.RawMessage(`{}`)),
			}, StopReason: llmclient.StopToolUse}, AttemptUsage{}, nil
		}
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
	}))
	result, err := profile.Run(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	for _, control := range []string{"scope", "effect_policy", "detector", "registry"} {
		if result.Coverage.Controls[control] != ControlPass {
			t.Errorf("%s = %q, want pass after allowed dispatched tool", control, result.Coverage.Controls[control])
		}
	}
}

func TestEngineeringProfileForwardsNow(t *testing.T) {
	profile := engineeringProfileForTest(t, AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error) {
		return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, AttemptUsage{}, nil
	}))
	var mu sync.Mutex
	n := 0
	profile.deps.Now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		n++
		return time.Unix(int64(n), 0)
	}
	if _, err := profile.Run(context.Background(), "work"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if n < 2 {
		t.Fatalf("Now calls = %d, want Harness timing to use profile seam", n)
	}
}

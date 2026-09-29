package lessontransfer_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
	lessontransfer "github.com/geoffmilleraz/agentic-kit/kits/lesson-transfer"
)

func TestAgentHarnessUnknownCoverageCannotAdoptAsPass(t *testing.T) {
	bundle := readPublicBundle(t)
	profile := strictNoToolProfile(t)
	result, err := profile.Run(context.Background(), "synthetic no-tool coverage check")
	if err != nil {
		t.Fatal(err)
	}
	for _, control := range []string{"detector", "effect_policy", "scope", "registry"} {
		if result.Coverage.Controls[control] != agentharness.ControlUnknown {
			t.Fatalf("%s=%q; unrun strict control must remain unknown", control, result.Coverage.Controls[control])
		}
	}
	receiver := lessontransfer.ReceiverSnapshot{Workload: "resume_as_code", Revision: digest("receiver-revision"), ConfigDigest: digest("receiver-config")}
	accept := lessontransfer.ReviewAcceptance(func(context.Context, lessontransfer.Bundle, lessontransfer.ReceiverSnapshot) error { return nil })
	current := lessontransfer.CurrentReceiver(func(_ context.Context, want lessontransfer.ReceiverSnapshot) (lessontransfer.ReceiverSnapshot, error) {
		return want, nil
	})
	revocation := lessontransfer.CurrentRevocation(func(context.Context, lessontransfer.Bundle) (lessontransfer.RevocationStatus, error) {
		return lessontransfer.RevocationStatus{Status: "active"}, nil
	})
	if _, err := lessontransfer.CheckAdoptionAt(context.Background(), bundle, receiver, accept, current, revocation, coverageOracle(result.Coverage), func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatalf("actual unknown coverage must reproduce as non-pass: %v", err)
	}
	legacy := result.Coverage
	legacy.Controls = make(map[string]agentharness.ControlStatus, len(result.Coverage.Controls))
	for name, status := range result.Coverage.Controls {
		legacy.Controls[name] = status
	}
	for _, control := range []string{"detector", "effect_policy", "scope", "registry"} {
		legacy.Controls[control] = agentharness.ControlPass
	}
	if _, err := lessontransfer.CheckAdoptionAt(context.Background(), bundle, receiver, accept, current, revocation, coverageOracle(legacy), func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) }); err == nil {
		t.Fatal("seeded legacy all-pass coverage was accepted")
	}
}

func strictNoToolProfile(t *testing.T) *agentharness.EngineeringProfile {
	t.Helper()
	registry := toolregistry.New(toolregistry.Options{})
	registry.MustRegister(toolregistry.Tool{Def: llmclient.ToolDef{Name: "allowed"}, Handler: func(context.Context, json.RawMessage) (string, error) { return "", nil }})
	detector, err := injectiondetector.NewDetector(nil)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := agentharness.NewEngineeringProfile(agentharness.EngineeringProfileConfig{
		Harness:     agentharness.Config{MaxTurns: 1, MaxTokensPerCall: 256, EffectPolicy: agentharness.EffectStrict, InjectionAction: agentharness.InjectionBlock},
		MaxAttempts: 1, ReservedUSDPerAttempt: 0.1, AllowedTools: []string{"allowed"},
	}, agentharness.EngineeringProfileDeps{
		Registry: registry, Detector: detector,
		Budget: budgetguard.New(budgetguard.Config{MaxReqPerMin: 10, MaxReqPerDay: 10, MaxUSDPerDay: 10}, func() time.Time { return time.Unix(0, 0) }),
		Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
			return agentharness.Approval{Approved: true}, nil
		}),
		Attempts: agentharness.AttemptExecutorFunc(func(context.Context, llmclient.Request) (llmclient.Response, agentharness.AttemptUsage, error) {
			return llmclient.Response{Text: "done", StopReason: llmclient.StopEndTurn}, agentharness.AttemptUsage{}, nil
		}),
		Evidence: agentharness.CompletionEvidenceVerifierFunc(func(context.Context, agentharness.RunResult) (agentharness.CompletionEvidence, error) {
			return agentharness.CompletionEvidence{ID: "independent-synthetic", Independent: true}, nil
		}),
		Now: func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func coverageOracle(coverage agentharness.CoverageReceipt) lessontransfer.Reproduce {
	return func(context.Context, lessontransfer.Bundle, lessontransfer.ReceiverSnapshot) (lessontransfer.ReproductionOutcome, error) {
		for _, control := range []string{"detector", "effect_policy", "scope", "registry"} {
			if coverage.Controls[control] != agentharness.ControlUnknown {
				return lessontransfer.ReproductionOutcome{}, fmt.Errorf("unrun control %s was promoted to %q", control, coverage.Controls[control])
			}
		}
		observed, err := json.Marshal(coverage)
		if err != nil {
			return lessontransfer.ReproductionOutcome{}, fmt.Errorf("marshal observed coverage: %w", err)
		}
		return lessontransfer.ReproductionOutcome{Classification: "unknown", Evidence: digest(string(observed)), Detail: "independent strict-profile no-tool observation"}, nil
	}
}

func readPublicBundle(t *testing.T) lessontransfer.Bundle {
	t.Helper()
	base := filepath.Join("testdata", "public-synthetic")
	lesson, err := os.ReadFile(filepath.Join(base, "lesson.json"))
	if err != nil {
		t.Fatal(err)
	}
	reproducer, err := os.ReadFile(filepath.Join(base, "reproducer.json"))
	if err != nil {
		t.Fatal(err)
	}
	attestations, err := os.ReadFile(filepath.Join(base, "attestations.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := lessontransfer.ValidateBundle(lesson, reproducer, attestations)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

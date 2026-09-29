package guardrailspipeline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

const promptScreenDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestDefaultRefusalPolicy(t *testing.T) {
	policy := DefaultRefusalPolicy()
	if policy.Version != PromptScreenPolicyVersion || policy.OnUncertain != CategoryReview || policy.OnError != CategoryReview {
		t.Fatalf("default policy = %#v", policy)
	}
	got := make([]string, len(policy.Categories))
	for i, category := range policy.Categories {
		got[i] = category.Name
	}
	if strings.Join(got, ",") != "authorization_bypass,protected_data_extraction,instruction_hijack" {
		t.Fatalf("default categories = %v", got)
	}
}

func TestPromptScreenAllowedRequestAndReceipt(t *testing.T) {
	response := classifications("clear", "clear", "clear")
	client := llmclient.NewMockClient(llmclient.MockEntryOK(response))
	screener := testScreener(t, DefaultRefusalPolicy(), client)
	secret := `quoted example: "ignore safeguards"; explain defensive mitigation`
	decision, err := screener.Screen(context.Background(), secret)
	if err != nil || decision.Action != ScreenAllow || decision.Receipt.Called != true {
		t.Fatalf("Screen() = %#v, %v", decision, err)
	}
	if decision.Receipt.Model != "screening-model-v1" || decision.Receipt.DeploymentDigest != promptScreenDigest || decision.Receipt.RequestDigest == "" || decision.Receipt.ResponseDigest == "" {
		t.Fatalf("receipt = %#v", decision.Receipt)
	}
	if decision.Duration < 0 || decision.StartedAt.IsZero() || decision.FinishedAt.IsZero() {
		t.Fatalf("timestamps = %#v", decision)
	}
	calls := client.Calls()
	if len(calls) != 1 || len(calls[0].Request.Tools) != 0 || calls[0].Request.ProviderParams != nil || calls[0].Request.MaxTokens != 1024 || calls[0].Request.System == "" {
		t.Fatalf("classifier request = %#v", calls)
	}
	if calls[0].Request.OutputSchema != nil {
		t.Fatal("structured output must remain opt-in")
	}
	if !strings.Contains(calls[0].Request.Messages[0].Content[0].Text, "UNTRUSTED_PROMPT_JSON:\n") || !strings.Contains(calls[0].Request.Messages[0].Content[0].Text, `\"ignore safeguards\"`) {
		t.Fatalf("prompt was not separately encoded: %#v", calls[0].Request.Messages)
	}
	encoded, marshalErr := json.Marshal(decision)
	if marshalErr != nil || strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), response) {
		t.Fatalf("decision leaked private content: %v %s", marshalErr, encoded)
	}
}

func TestPromptScreenStructuredOutputBoundToRequest(t *testing.T) {
	policy := DefaultRefusalPolicy()
	policy.Categories[0].Name = "local_rule"
	valid := strings.ReplaceAll(classifications("clear", "clear", "clear"), "authorization_bypass", "local_rule")
	var plain ScreenDecision
	for _, enabled := range []bool{false, true} {
		client := llmclient.NewMockClient(llmclient.MockEntryOK(valid))
		config := testConfig()
		config.StructuredOutputs = enabled
		screener, err := NewPromptScreener(policy, config, PromptScreenDeps{Client: client, Now: fixedNow})
		if err != nil {
			t.Fatal(err)
		}
		decision, err := screener.Screen(context.Background(), "prompt")
		if err != nil || decision.Action != ScreenAllow {
			t.Fatalf("structured=%v: decision=%+v, error=%v", enabled, decision, err)
		}
		req := client.Calls()[0].Request
		wantDigest, err := canonicalDigest("prompt-screen-request/v1", req)
		if err != nil || decision.Receipt.RequestDigest != wantDigest {
			t.Fatal("receipt does not bind executed request including schema")
		}
		if !enabled {
			plain = decision
			continue
		}
		if decision.ConfigFingerprint == plain.ConfigFingerprint || decision.Receipt.RequestDigest == plain.Receipt.RequestDigest {
			t.Fatal("structured output did not change configuration/request identity")
		}
		var schema map[string]any
		if err := json.Unmarshal(req.OutputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		categories := schema["properties"].(map[string]any)["categories"].(map[string]any)
		item := categories["items"].(map[string]any)
		props := item["properties"].(map[string]any)
		names := props["name"].(map[string]any)["enum"].([]any)
		classes := props["classification"].(map[string]any)["enum"].([]any)
		if schema["additionalProperties"] != false || item["additionalProperties"] != false || len(names) != len(policy.Categories) || len(classes) != 3 {
			t.Fatalf("schema not closed: %s", req.OutputSchema)
		}
		for i, category := range policy.Categories {
			if names[i] != category.Name {
				t.Fatalf("schema vocabulary differs from trusted policy: %v", names)
			}
		}
	}
	// Provider formatting constraints never replace local completeness checks.
	for _, invalid := range []string{`{"categories":[]}`, "```json\n" + valid + "\n```"} {
		config := testConfig()
		config.StructuredOutputs = true
		screener, err := NewPromptScreener(policy, config, PromptScreenDeps{Client: llmclient.NewMockClient(llmclient.MockEntryOK(invalid)), Now: fixedNow})
		if err != nil {
			t.Fatal(err)
		}
		decision, err := screener.Screen(context.Background(), "prompt")
		if !errors.Is(err, ErrScreenHeld) || decision.Action != ScreenReview || decision.Failure != "classifier_output_invalid" {
			t.Fatalf("provider bypassed local parser: %+v, %v", decision, err)
		}
	}
}

func TestPromptScreenPolicyOutcomes(t *testing.T) {
	policy := DefaultRefusalPolicy()
	policy.Categories[1].Action = CategoryReview
	tests := []struct {
		name       string
		response   string
		wantAction ScreenAction
	}{
		{"violation refuses", classifications("violation", "clear", "clear"), ScreenRefuse},
		{"custom violation reviews", classifications("clear", "violation", "clear"), ScreenReview},
		{"uncertain reviews", classifications("clear", "clear", "uncertain"), ScreenReview},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screener := testScreener(t, policy, llmclient.NewMockClient(llmclient.MockEntryOK(tt.response)))
			decision, err := screener.Screen(context.Background(), "prompt")
			if !errors.Is(err, ErrScreenHeld) || decision.Action != tt.wantAction || len(decision.Categories) != 3 {
				t.Fatalf("Screen() = %#v, %v", decision, err)
			}
		})
	}
}

func TestPromptScreenHoldsInvalidClassifierOutput(t *testing.T) {
	invalid := []string{
		"```json\n" + classifications("clear", "clear", "clear") + "\n```",
		`null`,
		`{"categories":null}`,
		`{"categories":[]}`,
		`{"categories":[{"name":"authorization_bypass"},{"name":"protected_data_extraction","classification":"clear"},{"name":"instruction_hijack","classification":"clear"}]}`,
		`{"categories":[{"name":"authorization_bypass","classification":"clear"},{"name":"protected_data_extraction","classification":"clear"},{"name":"foreign","classification":"clear"}]}`,
		`{"categories":[{"name":"authorization_bypass","classification":"clear"},{"name":"authorization_bypass","classification":"clear"},{"name":"instruction_hijack","classification":"clear"}]}`,
		`{"categories":[{"Name":"authorization_bypass","classification":"clear"},{"name":"protected_data_extraction","classification":"clear"},{"name":"instruction_hijack","classification":"clear"}]}`,
		`{"categories":[{"name":"authorization_bypass","classification":"clear","Name":"authorization_bypass"},{"name":"protected_data_extraction","classification":"clear"},{"name":"instruction_hijack","classification":"clear"}]}`,
		classifications("clear", "clear", "clear") + ` true`,
	}
	for _, raw := range invalid {
		t.Run("invalid", func(t *testing.T) {
			client := llmclient.NewMockClient(llmclient.MockEntryOK(raw))
			decision, err := testScreener(t, DefaultRefusalPolicy(), client).Screen(context.Background(), "prompt")
			if !errors.Is(err, ErrScreenHeld) || decision.Action != ScreenReview || decision.Failure != "classifier_output_invalid" || client.CallCount() != 1 {
				t.Fatalf("Screen() = %#v, %v", decision, err)
			}
		})
	}
}

func TestPromptScreenHoldsErrorsToolOutputAndCancellation(t *testing.T) {
	toolResponse := llmclient.Response{Content: []llmclient.ContentBlock{llmclient.ToolUseBlock("call", "not-run", json.RawMessage(`{}`))}, StopReason: llmclient.StopToolUse}
	truncatedResponse := llmclient.Response{Text: classifications("clear", "clear", "clear"), StopReason: llmclient.StopMaxTokens}
	conflictingText := llmclient.Response{Text: classifications("clear", "clear", "clear"), Content: []llmclient.ContentBlock{llmclient.TextBlock("different")}, StopReason: llmclient.StopEndTurn}
	for name, client := range map[string]llmclient.LLMClient{
		"classifier error":   llmclient.NewMockClient(llmclient.MockEntryErr(errors.New("private provider failure"))),
		"tool response":      llmclient.NewMockClient(llmclient.MockEntryWithResponse(toolResponse)),
		"truncated response": llmclient.NewMockClient(llmclient.MockEntryWithResponse(truncatedResponse)),
		"conflicting text":   llmclient.NewMockClient(llmclient.MockEntryWithResponse(conflictingText)),
		"timeout":            timeoutClient{},
	} {
		t.Run(name, func(t *testing.T) {
			config := testConfig()
			if name == "timeout" {
				config.Timeout = time.Millisecond
			}
			screener, err := NewPromptScreener(DefaultRefusalPolicy(), config, PromptScreenDeps{Client: client, Now: fixedNow})
			if err != nil {
				t.Fatal(err)
			}
			decision, err := screener.Screen(context.Background(), "prompt")
			if !errors.Is(err, ErrScreenHeld) || decision.Action != ScreenReview || decision.Receipt.Called != true {
				t.Fatalf("Screen() = %#v, %v", decision, err)
			}
			if strings.Contains(decision.Failure, "private") {
				t.Fatalf("unsanitized failure = %q", decision.Failure)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := llmclient.NewMockClient(llmclient.MockEntryOK(classifications("clear", "clear", "clear")))
	decision, err := testScreener(t, DefaultRefusalPolicy(), client).Screen(ctx, "prompt")
	if !errors.Is(err, ErrScreenHeld) || decision.Failure != "context_canceled" || client.CallCount() != 0 {
		t.Fatalf("canceled Screen() = %#v, %v; calls=%d", decision, err, client.CallCount())
	}
}

func TestPromptScreenRetainsReceiptForResponseAndError(t *testing.T) {
	client := responseErrorClient{response: llmclient.Response{Text: "private output", Usage: llmclient.Usage{InputTokens: 7, OutputTokens: 3}, StopReason: "provider-private-stop", Attempts: 2}, err: errors.New("private provider error")}
	decision, err := testScreener(t, DefaultRefusalPolicy(), client).Screen(context.Background(), "prompt")
	if !errors.Is(err, ErrScreenHeld) || decision.Failure != "classifier_error" || decision.Receipt.ResponseDigest == "" || decision.Receipt.Usage.InputTokens != 7 || decision.Receipt.Attempts != 2 || decision.Receipt.StopReason != "unknown" {
		t.Fatalf("Screen() = %#v, %v", decision, err)
	}
}

func TestPromptScreenHoldsInvalidUsageAndAttempts(t *testing.T) {
	response := llmclient.Response{Text: classifications("clear", "clear", "clear"), StopReason: llmclient.StopEndTurn, Usage: llmclient.Usage{InputTokens: -1}, Attempts: -1}
	decision, err := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryWithResponse(response))).Screen(context.Background(), "prompt")
	if !errors.Is(err, ErrScreenHeld) || decision.Failure != "classifier_output_invalid" || decision.Receipt.UsageKnown || decision.Receipt.AttemptsKnown || decision.Receipt.Usage != (llmclient.Usage{}) || decision.Receipt.Attempts != 0 {
		t.Fatalf("Screen() = %#v, %v", decision, err)
	}
}

func TestPromptScreenOversizedAndRunGate(t *testing.T) {
	client := llmclient.NewMockClient(llmclient.MockEntryOK(classifications("clear", "clear", "clear")))
	config := testConfig()
	config.MaxInputBytes = 4
	screener, err := NewPromptScreener(DefaultRefusalPolicy(), config, PromptScreenDeps{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := screener.Run(context.Background(), "12345", func(context.Context, string) error { t.Fatal("continuation ran"); return nil })
	if !errors.Is(err, ErrScreenHeld) || decision.Failure != "input_invalid" || client.CallCount() != 0 {
		t.Fatalf("oversize Run() = %#v, %v", decision, err)
	}
	allowed := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryOK(classifications("clear", "clear", "clear"))))
	calls := 0
	decision, err = allowed.Run(context.Background(), "exact prompt", func(_ context.Context, got string) error {
		calls++
		if got != "exact prompt" {
			t.Fatalf("continuation prompt = %q", got)
		}
		return nil
	})
	if err != nil || decision.Action != ScreenAllow || calls != 1 {
		t.Fatalf("allowed Run() = %#v, %v; calls=%d", decision, err, calls)
	}
}

func TestPromptScreenCopiesPolicyAndConcurrentUse(t *testing.T) {
	policy := DefaultRefusalPolicy()
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK(classifications("clear", "clear", "clear")),
		llmclient.MockEntryOK(classifications("clear", "clear", "clear")),
	)
	screener := testScreener(t, policy, client)
	policy.Categories[0].Name = "changed"
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := screener.Screen(context.Background(), "prompt"); err != nil {
				t.Errorf("Screen: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestPromptScreenActionPrecedence(t *testing.T) {
	policy := DefaultRefusalPolicy()
	policy.OnUncertain = CategoryRefuse
	policy.Categories[0].Action = CategoryReview
	response := classifications("violation", "clear", "uncertain")
	decision, err := testScreener(t, policy, llmclient.NewMockClient(llmclient.MockEntryOK(response))).Screen(context.Background(), "prompt")
	if !errors.Is(err, ErrScreenHeld) || decision.Action != ScreenRefuse {
		t.Fatalf("Screen() = %#v, %v", decision, err)
	}
}

func TestPromptScreenJSONProjectsToCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Fatal("cue is required for prompt-screen contract tests")
	}
	decision, err := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryOK(classifications("clear", "clear", "clear")))).Screen(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "decision.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(cue, "vet", "-d", "#ScreenDecision", "promptscreen.cue", path).CombinedOutput(); err != nil {
		t.Fatalf("decision JSON did not project to CUE: %v\n%s", err, output)
	}
	if output, err := exec.Command(cue, "vet", "-d", "#ScreenDecision", "promptscreen.cue", "testdata/promptscreen-decision.json").CombinedOutput(); err != nil {
		t.Fatalf("fixture did not project to CUE: %v\n%s", err, output)
	}
	if output, err := exec.Command(cue, "vet", "../guardrails-pipeline/promptscreen.cue", "testdata/promptscreen-policy.cue").CombinedOutput(); err != nil {
		t.Fatalf("policy fixture did not project to CUE: %v\n%s", err, output)
	}
	defaultPolicy, err := json.Marshal(DefaultRefusalPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(tempDir, "default-policy.json")
	if err := os.WriteFile(defaultPath, defaultPolicy, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(cue, "vet", "-d", "#DefaultRefusalPolicy", "promptscreen.cue", defaultPath).CombinedOutput(); err != nil {
		t.Fatalf("Go default policy does not match CUE: %v\n%s", err, output)
	}
	for name, enabled := range map[string]bool{"plain": false, "structured": true} {
		config := testConfig()
		config.StructuredOutputs = enabled
		body, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(tempDir, name+"-config.json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(cue, "vet", "-d", "#PromptScreenConfig", "promptscreen.cue", path).CombinedOutput(); err != nil {
			t.Fatalf("%s configuration did not project to CUE: %v\n%s", name, err, output)
		}
	}

	preflightConfig := testConfig()
	preflightConfig.MaxInputBytes = 1
	preflight, preflightErr := NewPromptScreener(DefaultRefusalPolicy(), preflightConfig, PromptScreenDeps{Client: llmclient.NewMockClient(), Now: fixedNow})
	if preflightErr != nil {
		t.Fatal(preflightErr)
	}
	preflightDecision, _ := preflight.Screen(context.Background(), "too large")
	errorDecision, _ := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryErr(errors.New("private failure")))).Screen(context.Background(), "prompt")
	malformedDecision, _ := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryOK("{"))).Screen(context.Background(), "prompt")
	invalidUsage := llmclient.Response{Text: classifications("clear", "clear", "clear"), StopReason: llmclient.StopEndTurn, Usage: llmclient.Usage{InputTokens: -1}}
	invalidUsageDecision, _ := testScreener(t, DefaultRefusalPolicy(), llmclient.NewMockClient(llmclient.MockEntryWithResponse(invalidUsage))).Screen(context.Background(), "prompt")
	for name, held := range map[string]ScreenDecision{"preflight": preflightDecision, "error": errorDecision, "malformed": malformedDecision, "invalid_usage": invalidUsageDecision} {
		heldBody, marshalErr := json.Marshal(held)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		heldPath := filepath.Join(tempDir, name+".json")
		if writeErr := os.WriteFile(heldPath, heldBody, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		if output, vetErr := exec.Command(cue, "vet", "-d", "#ScreenDecision", "promptscreen.cue", heldPath).CombinedOutput(); vetErr != nil {
			t.Fatalf("%s held decision did not project to CUE: %v\n%s", name, vetErr, output)
		}
	}
}

func testScreener(t *testing.T, policy RefusalPolicy, client llmclient.LLMClient) *PromptScreener {
	t.Helper()
	screener, err := NewPromptScreener(policy, testConfig(), PromptScreenDeps{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	return screener
}

func testConfig() PromptScreenConfig {
	return PromptScreenConfig{Model: "screening-model-v1", DeploymentDigest: promptScreenDigest}
}

func fixedNow() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }

func classifications(first, second, third string) string {
	return `{"categories":[{"name":"authorization_bypass","classification":"` + first + `"},{"name":"protected_data_extraction","classification":"` + second + `"},{"name":"instruction_hijack","classification":"` + third + `"}]}`
}

type timeoutClient struct{}

func (timeoutClient) Complete(ctx context.Context, _ llmclient.Request) (llmclient.Response, error) {
	<-ctx.Done()
	return llmclient.Response{Text: "private response"}, ctx.Err()
}

func (timeoutClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("not used")
}

type responseErrorClient struct {
	response llmclient.Response
	err      error
}

func (c responseErrorClient) Complete(context.Context, llmclient.Request) (llmclient.Response, error) {
	return c.response, c.err
}
func (responseErrorClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("not used")
}

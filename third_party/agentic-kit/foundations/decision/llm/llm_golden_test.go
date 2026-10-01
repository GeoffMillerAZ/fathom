package llm_test

import (
	"context"
	"os"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// goldenRequest is the one fixed Request pinned by testdata/prompt.golden
// and testdata/schema.golden. Changing it (or the rendering it drives) is a
// prompt-drift event and must update both golden files deliberately, in
// review — see TestLLM_Decide_GoldenPromptAndSchema.
func goldenRequest() decision.Request {
	return decision.Request{
		State: "widget on line 3 stopped moving after the motor made a grinding noise",
		Questions: []decision.Question{
			{
				ID:           "root-cause",
				Kind:         decision.KindChoice,
				Instructions: "what caused the stoppage?",
				Options: []decision.Option{
					{Name: "mechanical", Criteria: "gears, belts, bearings"},
					{Name: "electrical"},
					{Name: "operator-error"},
				},
			},
			{
				ID:           "severity",
				Kind:         decision.KindScore,
				Instructions: "how severe is this on a 0-10 scale?",
				Min:          0,
				Max:          10,
				Rubric:       "0 = cosmetic, 10 = line-down",
			},
			{
				ID:            "restart-safe",
				Kind:          decision.KindPredicate,
				Instructions:  "is it safe to restart the line without inspection?",
				TrueCriteria:  "no visible damage and grinding has stopped",
				FalseCriteria: "any visible damage or persistent noise",
			},
		},
	}
}

// TestLLM_Decide_GoldenPromptAndSchema pins the exact llmclient.Request
// LLM.Decide builds for goldenRequest — the rendered Prompt
// (testdata/prompt.golden) and the OutputSchema
// (testdata/schema.golden) — so prompt/schema drift shows up as a diff in
// review instead of silently changing what the model is asked.
func TestLLM_Decide_GoldenPromptAndSchema(t *testing.T) {
	client := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{Text: `{"answers":{"root-cause":{"choice":"mechanical","probabilities":{"mechanical":1,"electrical":0,"operator-error":0}},"severity":{"score":5},"restart-safe":{"noul":0.2}}}`},
	})
	l := llm.NewLLM(client, "test-model")

	if _, err := l.Decide(context.Background(), goldenRequest()); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	calls := client.Calls()
	if len(calls) != 1 {
		t.Fatalf("len(Calls()) = %d, want 1", len(calls))
	}
	req := calls[0].Request

	wantPrompt := readGolden(t, "testdata/prompt.golden")
	if req.Prompt != wantPrompt {
		t.Errorf("rendered Prompt does not match testdata/prompt.golden\ngot:\n%s\nwant:\n%s", req.Prompt, wantPrompt)
	}

	wantSchema := readGolden(t, "testdata/schema.golden")
	if string(req.OutputSchema) != wantSchema {
		t.Errorf("rendered OutputSchema does not match testdata/schema.golden\ngot:\n%s\nwant:\n%s", req.OutputSchema, wantSchema)
	}
}

// readGolden reads a golden fixture file, trimming exactly one trailing
// newline (the file's own final line ending; the rendered values under test
// never carry a trailing newline).
func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.TrimSuffix(string(b), "\n")
}

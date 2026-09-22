package decision_test

import (
	"context"
	"errors"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// mixedRequest returns a Request carrying one Question of each Kind, used
// across several LLM.Decide tests.
func mixedRequest() decision.Request {
	return decision.Request{
		State: "a widget is jammed on the line",
		Questions: []decision.Question{
			{
				ID:   "root-cause",
				Kind: decision.KindChoice,
				Options: []decision.Option{
					{Name: "mechanical"},
					{Name: "electrical"},
					{Name: "operator-error"},
				},
			},
			{ID: "severity", Kind: decision.KindScore, Min: 0, Max: 10},
			{ID: "line-stopped", Kind: decision.KindNoul, TrueCriteria: "the line halted"},
		},
	}
}

func TestLLM_Name(t *testing.T) {
	l := decision.NewLLM(llmclient.NewMockClient(), "test-model")
	if got := l.Name(); got != "llm" {
		t.Errorf("Name() = %q, want %q", got, "llm")
	}
}

func TestLLM_Decide_MixedBatch(t *testing.T) {
	text := `{"answers":{
		"root-cause": {"choice":"mechanical","probabilities":{"mechanical":0.7,"electrical":0.2,"operator-error":0.1}},
		"severity": {"score":6},
		"line-stopped": {"noul":0.8}
	}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{Text: text, Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 40}},
	})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), mixedRequest())
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if len(resp.Invalid) != 0 {
		t.Fatalf("Invalid = %v, want empty", resp.Invalid)
	}
	if len(resp.Answers) != 3 {
		t.Fatalf("len(Answers) = %d, want 3", len(resp.Answers))
	}

	rc := resp.Answers["root-cause"]
	if rc.Choice != "mechanical" {
		t.Errorf("root-cause.Choice = %q, want %q", rc.Choice, "mechanical")
	}
	sum := 0.0
	for _, p := range rc.Probabilities {
		sum += p
	}
	if !floatsClose(sum, 1.0) {
		t.Errorf("root-cause probabilities sum = %v, want 1.0", sum)
	}
	wantConf := decision.Confidence(map[string]float64{"mechanical": 0.7, "electrical": 0.2, "operator-error": 0.1})
	if !floatsClose(rc.Confidence, wantConf) {
		t.Errorf("root-cause.Confidence = %v, want %v", rc.Confidence, wantConf)
	}

	sev := resp.Answers["severity"]
	if sev.Score != 6 {
		t.Errorf("severity.Score = %v, want 6", sev.Score)
	}
	if sev.Confidence != 1 {
		t.Errorf("severity.Confidence = %v, want 1 (degenerate k<=1)", sev.Confidence)
	}

	ls := resp.Answers["line-stopped"]
	if ls.Noul != 0.8 {
		t.Errorf("line-stopped.Noul = %v, want 0.8", ls.Noul)
	}
	if ls.Confidence != 0.8 {
		t.Errorf("line-stopped.Confidence = %v, want 0.8 (Noul>=0.5)", ls.Confidence)
	}

	if resp.Provider != "llm" || resp.Model != "test-model" {
		t.Errorf("Provider/Model = %q/%q, want llm/test-model", resp.Provider, resp.Model)
	}
	if resp.Usage.InputTokens != 100 || resp.Usage.OutputTokens != 40 {
		t.Errorf("Usage = %+v, want {100 40}", resp.Usage)
	}
}

func TestLLM_Decide_InvalidChoiceStillAnswersRest(t *testing.T) {
	text := `{"answers":{
		"root-cause": {"choice":"gremlins","probabilities":{"mechanical":1,"electrical":0,"operator-error":0}},
		"severity": {"score":3},
		"line-stopped": {"noul":0.1}
	}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), mixedRequest())
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if _, ok := resp.Answers["root-cause"]; ok {
		t.Error("root-cause should be absent from Answers (invalid choice)")
	}
	if reason, ok := resp.Invalid["root-cause"]; !ok || reason == "" {
		t.Errorf("Invalid[root-cause] = %q, ok=%v; want a non-empty reason", reason, ok)
	}
	if len(resp.Answers) != 2 {
		t.Fatalf("len(Answers) = %d, want 2 (severity + line-stopped still answered)", len(resp.Answers))
	}
	if _, ok := resp.Answers["severity"]; !ok {
		t.Error("severity should still be answered")
	}
	if _, ok := resp.Answers["line-stopped"]; !ok {
		t.Error("line-stopped should still be answered")
	}
}

func TestLLM_Decide_ProbabilitiesRenormalised(t *testing.T) {
	req := decision.Request{
		State: "s",
		Questions: []decision.Question{
			{ID: "q1", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "b"}}},
		},
	}
	// Sums to 0.4, not 1.0 — must be renormalised to a/b = 0.5/0.5.
	text := `{"answers":{"q1":{"choice":"a","probabilities":{"a":0.2,"b":0.2}}}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	a := resp.Answers["q1"]
	if !floatsClose(a.Probabilities["a"], 0.5) || !floatsClose(a.Probabilities["b"], 0.5) {
		t.Errorf("Probabilities = %v, want a=0.5 b=0.5", a.Probabilities)
	}
}

func TestLLM_Decide_MissingProbabilitiesFillZeroAndRenormalise(t *testing.T) {
	req := decision.Request{
		State: "s",
		Questions: []decision.Question{
			{ID: "q1", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "b"}, {Name: "c"}}},
		},
	}
	// "c" is entirely absent from probabilities -> must be filled with 0.
	text := `{"answers":{"q1":{"choice":"a","probabilities":{"a":1,"b":1}}}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	a := resp.Answers["q1"]
	if len(a.Probabilities) != 3 {
		t.Fatalf("Probabilities missing an option: %v", a.Probabilities)
	}
	if !floatsClose(a.Probabilities["c"], 0) {
		t.Errorf("Probabilities[c] = %v, want 0", a.Probabilities["c"])
	}
	if !floatsClose(a.Probabilities["a"], 0.5) || !floatsClose(a.Probabilities["b"], 0.5) {
		t.Errorf("Probabilities = %v, want a=0.5 b=0.5 c=0", a.Probabilities)
	}
}

func TestLLM_Decide_ScoreClamped(t *testing.T) {
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindScore, Min: 0, Max: 10}},
	}
	text := `{"answers":{"q1":{"score":15}}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resp.Answers["q1"].Score != 10 {
		t.Errorf("Score = %v, want clamped to 10", resp.Answers["q1"].Score)
	}

	text2 := `{"answers":{"q1":{"score":-5}}}`
	client2 := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text2}})
	l2 := decision.NewLLM(client2, "test-model")
	resp2, err := l2.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resp2.Answers["q1"].Score != 0 {
		t.Errorf("Score = %v, want clamped to 0", resp2.Answers["q1"].Score)
	}
}

func TestLLM_Decide_NoulClamped(t *testing.T) {
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}},
	}
	text := `{"answers":{"q1":{"noul":1.5}}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resp.Answers["q1"].Noul != 1 {
		t.Errorf("Noul = %v, want clamped to 1", resp.Answers["q1"].Noul)
	}
	if resp.Answers["q1"].Confidence != 1 {
		t.Errorf("Confidence = %v, want 1", resp.Answers["q1"].Confidence)
	}
}

func TestLLM_Decide_MissingAnswerIsInvalid(t *testing.T) {
	req := decision.Request{
		State: "s",
		Questions: []decision.Question{
			{ID: "q1", Kind: decision.KindNoul},
			{ID: "q2", Kind: decision.KindNoul},
		},
	}
	text := `{"answers":{"q1":{"noul":0.9}}}`
	client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}})
	l := decision.NewLLM(client, "test-model")

	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if _, ok := resp.Answers["q2"]; ok {
		t.Error("q2 should be absent from Answers")
	}
	if _, ok := resp.Invalid["q2"]; !ok {
		t.Error("q2 should be named in Invalid")
	}
}

func TestLLM_Decide_NonJSONReplyErrors(t *testing.T) {
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}},
	}
	client := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{Text: "sorry, I can't help with that"},
	})
	l := decision.NewLLM(client, "test-model")

	if _, err := l.Decide(context.Background(), req); err == nil {
		t.Fatal("Decide() error = nil, want non-nil for a non-JSON reply")
	}
}

func TestLLM_Decide_ClientErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}},
	}
	client := llmclient.NewMockClient(llmclient.MockEntry{Err: wantErr})
	l := decision.NewLLM(client, "test-model")

	_, err := l.Decide(context.Background(), req)
	if err == nil {
		t.Fatal("Decide() error = nil, want non-nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Decide() error = %v, want wrapping %v", err, wantErr)
	}
}

func TestLLM_Decide_InvalidRequestRejected(t *testing.T) {
	l := decision.NewLLM(llmclient.NewMockClient(), "test-model")
	if _, err := l.Decide(context.Background(), decision.Request{}); err == nil {
		t.Fatal("Decide() error = nil, want a Validate error for an empty State")
	}
}

func TestLLM_Decide_UsesConfiguredMaxTokensAndModel(t *testing.T) {
	client := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{Text: `{"answers":{"q1":{"noul":0.5}}}`},
	})
	l := decision.NewLLM(client, "my-model", decision.WithMaxTokens(512))

	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}
	if _, err := l.Decide(context.Background(), req); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	calls := client.Calls()
	if len(calls) != 1 {
		t.Fatalf("len(Calls()) = %d, want 1", len(calls))
	}
	if calls[0].Request.Model != "my-model" {
		t.Errorf("Request.Model = %q, want %q", calls[0].Request.Model, "my-model")
	}
	if calls[0].Request.MaxTokens != 512 {
		t.Errorf("Request.MaxTokens = %d, want 512", calls[0].Request.MaxTokens)
	}
}

func TestLLM_Decide_WithNowControlsLatency(t *testing.T) {
	client := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{Text: `{"answers":{"q1":{"noul":0.5}}}`},
	})
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	now := func() time.Time {
		calls++
		if calls == 1 {
			return start
		}
		return start.Add(250 * time.Millisecond)
	}
	l := decision.NewLLM(client, "my-model", decision.WithNow(now))

	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}
	resp, err := l.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resp.Latency != 250*time.Millisecond {
		t.Errorf("Latency = %v, want 250ms", resp.Latency)
	}
}

func TestResponse_Records(t *testing.T) {
	resp := decision.Response{
		Provider: "llm",
		Model:    "test-model",
		Answers: map[string]decision.Answer{
			"b-question": {ID: "b-question", Kind: decision.KindNoul, Noul: 0.9, Confidence: 0.9},
			"a-question": {ID: "a-question", Kind: decision.KindScore, Score: 4, Confidence: 1},
			"c-question": {ID: "c-question", Kind: decision.KindChoice, Choice: "x", Confidence: 0.7},
		},
		Invalid: map[string]string{"d-question": "malformed"},
		Usage:   decision.Usage{InputTokens: 10, OutputTokens: 5},
		Latency: 42 * time.Millisecond,
	}

	recs := resp.Records("mysite")
	if len(recs) != 3 {
		t.Fatalf("len(Records) = %d, want 3 (Invalid entries excluded)", len(recs))
	}
	wantOrder := []string{"a-question", "b-question", "c-question"}
	for i, id := range wantOrder {
		if recs[i].QuestionID != id {
			t.Errorf("Records[%d].QuestionID = %q, want %q (sorted)", i, recs[i].QuestionID, id)
		}
		if recs[i].Site != "mysite" {
			t.Errorf("Records[%d].Site = %q, want %q", i, recs[i].Site, "mysite")
		}
		if recs[i].Provider != "llm" || recs[i].Model != "test-model" {
			t.Errorf("Records[%d] Provider/Model = %q/%q, want llm/test-model", i, recs[i].Provider, recs[i].Model)
		}
		if recs[i].InputTokens != 10 || recs[i].OutputTokens != 5 {
			t.Errorf("Records[%d] usage = %d/%d, want 10/5", i, recs[i].InputTokens, recs[i].OutputTokens)
		}
		if recs[i].Latency != 42*time.Millisecond {
			t.Errorf("Records[%d].Latency = %v, want 42ms", i, recs[i].Latency)
		}
	}
}

package decision_test

import (
	"testing"
	"time"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func TestResponse_Records(t *testing.T) {
	resp := decision.Response{
		Provider: "llm",
		Model:    "test-model",
		Answers: map[string]decision.Answer{
			"b-question": {ID: "b-question", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
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

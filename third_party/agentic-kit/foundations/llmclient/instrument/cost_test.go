package llminstrument_test

import (
	"context"
	"errors"
	"testing"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
)

// testTable holds EXAMPLE prices for tests only — not real provider rates.
func testTable() costtracker.PriceTable {
	return costtracker.PriceTable{
		"example-model": {
			InputPerMTok:      3.0,
			OutputPerMTok:     15.0,
			CacheReadPerMTok:  0.3,
			CacheWritePerMTok: 3.75,
		},
	}
}

func TestWithCost_CompleteKnownModel_GoldenMath(t *testing.T) {
	usage := llmclient.Usage{
		InputTokens:              1000,
		OutputTokens:             2000,
		CacheReadInputTokens:     10000,
		CacheCreationInputTokens: 4000,
	}
	mock := llmclient.NewMockClient(usageEntry("ok", usage))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "step-1")

	resp, err := client.Complete(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
	if err != nil || resp.Text != "ok" {
		t.Fatalf("call path altered: (%q, %v)", resp.Text, err)
	}

	// 0.003 (input) + 0.03 (output) + 0.003 (cache read) + 0.015 (cache write)
	const wantUSD = 0.051
	got := tracker.Totals()
	want := costtracker.Totals{
		Calls:            1,
		InputTokens:      1000,
		OutputTokens:     2000,
		CacheReadTokens:  10000,
		CacheWriteTokens: 4000,
		USD:              got.USD, // compared with tolerance below
	}
	if got != want {
		t.Errorf("Totals = %+v, want %+v", got, want)
	}
	if !floatEq(got.USD, wantUSD) {
		t.Errorf("USD = %v, want %v", got.USD, wantUSD)
	}

	byLabel := tracker.ByLabel()
	if byLabel["step-1"].Calls != 1 {
		t.Errorf(`ByLabel["step-1"] = %+v, want one call`, byLabel["step-1"])
	}
	byModel := tracker.ByModel()
	if byModel["example-model"].Calls != 1 {
		t.Errorf(`ByModel["example-model"] = %+v, want one call`, byModel["example-model"])
	}
}

func TestWithCost_UnknownModel_TokensRecordedZeroCost(t *testing.T) {
	usage := llmclient.Usage{InputTokens: 500, OutputTokens: 100}
	mock := llmclient.NewMockClient(usageEntry("ok", usage))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "step-2")

	if _, err := client.Complete(context.Background(), llmclient.Request{Model: "mystery-model", Prompt: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := tracker.Totals()
	if got.Calls != 1 || got.InputTokens != 500 || got.OutputTokens != 100 {
		t.Errorf("tokens not recorded for unknown model: %+v", got)
	}
	if got.USD != 0 {
		t.Errorf("USD = %v, want exactly 0 for unknown model", got.USD)
	}
	if _, ok := tracker.ByModel()["mystery-model"]; !ok {
		t.Error("unknown model must still appear in the by-model breakdown")
	}
}

func TestWithCost_CompleteError_NotRecorded(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryRateLimit("slow"))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "step-3")

	if _, err := client.Complete(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"}); err == nil {
		t.Fatal("want error")
	}
	if got := tracker.Totals().Calls; got != 0 {
		t.Errorf("failed call recorded: Calls = %d, want 0", got)
	}
}

func TestWithCost_StreamTerminalUsageRecorded(t *testing.T) {
	usage := llmclient.Usage{
		InputTokens:              200,
		OutputTokens:             300,
		CacheReadInputTokens:     400,
		CacheCreationInputTokens: 500,
	}
	mock := llmclient.NewMockClient(usageEntry("streamed", usage))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "stream-step")

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, terminal := drain(t, ch)
	if text != "streamed" || terminal.StopReason != llmclient.StopEndTurn {
		t.Fatalf("chunks altered: text=%q terminal=%+v", text, terminal)
	}

	// The wrapped channel closes only after the record is written.
	got := tracker.Totals()
	wantUSD := 200*3.0/1e6 + 300*15.0/1e6 + 400*0.3/1e6 + 500*3.75/1e6
	if got.Calls != 1 || got.InputTokens != 200 || got.OutputTokens != 300 ||
		got.CacheReadTokens != 400 || got.CacheWriteTokens != 500 {
		t.Errorf("Totals = %+v", got)
	}
	if !floatEq(got.USD, wantUSD) {
		t.Errorf("USD = %v, want %v", got.USD, wantUSD)
	}
}

func TestWithCost_StreamMidError_NotRecorded(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial", errors.New("mid-stream boom")))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "stream-err")

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if _, terminal := drain(t, ch); terminal.Err == nil {
		t.Fatal("terminal error chunk not forwarded")
	}
	if got := tracker.Totals().Calls; got != 0 {
		t.Errorf("mid-stream failure recorded: Calls = %d, want 0", got)
	}
}

func TestWithCost_StreamPreError_NotRecorded(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.AuthError{Message: "denied"}))
	tracker := costtracker.New()
	client := llminstrument.WithCost(mock, tracker, testTable(), "pre-err")

	if _, err := client.Stream(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"}); err == nil {
		t.Fatal("want pre-stream error")
	}
	if got := tracker.Totals().Calls; got != 0 {
		t.Errorf("pre-stream failure recorded: Calls = %d, want 0", got)
	}
}

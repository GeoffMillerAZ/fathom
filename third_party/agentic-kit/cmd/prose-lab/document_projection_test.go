package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

var thinkingBlock = llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"","signature":"signed"}`)}
var redactedBlock = llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"redacted_thinking","data":"opaque"}`)}

type documentBlocksClient struct {
	inner    llmclient.LLMClient
	extra    []llmclient.ContentBlock
	omitText bool
}

func (c *documentBlocksClient) Complete(ctx context.Context, r llmclient.Request) (llmclient.Response, error) {
	resp, e := c.inner.Complete(ctx, r)
	if e != nil {
		return resp, e
	}
	resp.Content = append([]llmclient.ContentBlock{}, c.extra...)
	if !c.omitText {
		resp.Content = append(resp.Content, llmclient.TextBlock(resp.Text))
	}
	return resp, nil
}
func (c *documentBlocksClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("not streamed")
}
func TestDocumentRealEngineThinkingProjection(t *testing.T) {
	corpus, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	cell, _ := corpus.cell("freeform-brief-technical-r1")
	d := corpus.Fixtures[cell.DocumentID]
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	gen := &documentBlocksClient{inner: &documentGenerator{}, extra: []llmclient.ContentBlock{thinkingBlock, redactedBlock}}
	judge := &documentSupport{}
	support := &documentBlocksClient{inner: judge, extra: []llmclient.ContentBlock{thinkingBlock}}
	// The unprojected core response still refuses non-text blocks.
	engine, target, e := documentEngine(gen, support, "gen-model", "review-model", 4096, d, cell, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = engine.Generate(context.Background(), target); e == nil {
		t.Fatal("unprojected thinking passed core")
	}
	engine, target, e = documentEngine(&documentTextClient{inner: gen}, support, "gen-model", "review-model", 4096, d, cell, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = engine.Generate(context.Background(), target); e == nil {
		t.Fatal("unprojected review thinking passed")
	}
	engine, target, e = documentEngine(&documentTextClient{inner: gen}, &documentTextClient{inner: support}, "gen-model", "review-model", 4096, d, cell, now)
	if e != nil {
		t.Fatal(e)
	}
	v, e := engine.Generate(context.Background(), target)
	if e != nil {
		t.Fatal(e)
	}
	rendered, e := engine.Render(context.Background(), v)
	if e != nil {
		t.Fatal(e)
	}
	if e = documentCoverage(d, v, rendered); e != nil {
		t.Fatal(e)
	}
	if judge.maxSeen != 4096 {
		t.Fatalf("review max_tokens=%d", judge.maxSeen)
	}
}
func TestDocumentProjectionOnlyRecognizesClosedThinking(t *testing.T) {
	text := `{"status":"supported","reason":"source entails assertion"}`
	cases := []struct {
		name              string
		block             llmclient.ContentBlock
		wantErr, wantKeep bool
	}{
		{"thinking", thinkingBlock, false, false},
		{"summarized-thinking-display", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"synthetic summary","signature":"opaque"}`)}, false, false},
		{"omitted-thinking-display", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"","signature":"opaque"}`)}, false, false},
		{"redacted", redactedBlock, false, false},
		{"tool", llmclient.ContentBlock{Type: llmclient.BlockToolUse, Name: "action", ID: "tool-1", Input: json.RawMessage(`{}`)}, false, true},
		{"unknown", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"mystery","payload":"x"}`)}, false, true},
		{"refusal", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"refusal","reason":"no"}`)}, false, true},
		{"malformed-thinking", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"x"}`)}, true, false},
		{"extra-thinking-field", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"x","signature":"s","authority":"grant"}`)}, true, false},
		{"bad-redacted", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"redacted_thinking","data":5}`)}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := &documentSpy{response: llmclient.Response{Text: text, StopReason: llmclient.StopEndTurn, Content: []llmclient.ContentBlock{tc.block, llmclient.TextBlock(text)}}}
			out, e := (&documentTextClient{inner: inner}).Complete(context.Background(), llmclient.Request{})
			if (e != nil) != tc.wantErr {
				t.Fatalf("error=%v", e)
			}
			if e != nil {
				return
			}
			if out.Text != text {
				t.Fatal("answer text changed")
			}
			want := 1
			if tc.wantKeep {
				want = 2
			}
			if len(out.Content) != want {
				t.Fatalf("content length=%d want %d", len(out.Content), want)
			}
			if tc.wantKeep && out.Content[0].Type != tc.block.Type {
				t.Fatal("actionable block dropped")
			}
		})
	}
}
func TestDocumentProjectionEmptyOrCanceledHolds(t *testing.T) {
	only := &documentSpy{response: llmclient.Response{Text: "", StopReason: llmclient.StopEndTurn, Content: []llmclient.ContentBlock{thinkingBlock}}}
	if _, e := (&documentTextClient{inner: only}).Complete(context.Background(), llmclient.Request{}); e == nil {
		t.Fatal("thinking-only accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	valid := &documentSpy{response: llmclient.Response{Text: "answer", StopReason: llmclient.StopEndTurn, Content: []llmclient.ContentBlock{thinkingBlock, llmclient.TextBlock("answer")}}}
	if _, e := (&documentTextClient{inner: valid}).Complete(ctx, llmclient.Request{}); e == nil {
		t.Fatal("canceled response accepted")
	}
}

func TestDocumentProjectedActionableBlocksStillHold(t *testing.T) {
	corpus, e := loadDocumentCorpus("testdata/document-corpus")
	if e != nil {
		t.Fatal(e)
	}
	cell, _ := corpus.cell("freeform-brief-executive-r1")
	d := corpus.Fixtures[cell.DocumentID]
	now := func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	cases := []struct {
		name  string
		block llmclient.ContentBlock
	}{
		{"tool", llmclient.ContentBlock{Type: llmclient.BlockToolUse, ID: "t1", Name: "action", Input: json.RawMessage(`{}`)}},
		{"unknown", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"future_kind","payload":"x"}`)}},
		{"refusal", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"refusal","reason":"no"}`)}},
		{"malformed-thinking", llmclient.ContentBlock{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"x"}`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := &documentTextClient{inner: &documentBlocksClient{inner: &documentGenerator{}, extra: []llmclient.ContentBlock{tc.block}}}
			rev := &documentTextClient{inner: &documentSupport{}}
			engine, target, e := documentEngine(gen, rev, "gen-model", "review-model", 4096, d, cell, now)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = engine.Generate(context.Background(), target); e == nil {
				t.Fatal("non-answer or malformed block accepted")
			}
		})
	}
}

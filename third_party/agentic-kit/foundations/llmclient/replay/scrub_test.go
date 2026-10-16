package llmreplay_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
)

// TestDefaultScrubber_Coverage walks every text location DefaultScrubber must
// redact, one field per case.
func TestDefaultScrubber_Coverage(t *testing.T) {
	const secret = "password=hunter2-secret"
	const label = "[REDACTED:PASSWORD]"

	tests := []struct {
		name  string
		entry llmreplay.Entry
		field func(llmreplay.Entry) string
	}{
		{
			name:  "request prompt",
			entry: llmreplay.Entry{Kind: llmreplay.KindComplete, Request: llmclient.Request{Prompt: secret}},
			field: func(e llmreplay.Entry) string { return e.Request.Prompt },
		},
		{
			name:  "request system",
			entry: llmreplay.Entry{Kind: llmreplay.KindComplete, Request: llmclient.Request{System: secret}},
			field: func(e llmreplay.Entry) string { return e.Request.System },
		},
		{
			name: "message text block",
			entry: llmreplay.Entry{Kind: llmreplay.KindComplete, Request: llmclient.Request{
				Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock(secret)}}},
			}},
			field: func(e llmreplay.Entry) string { return e.Request.Messages[0].Content[0].Text },
		},
		{
			name: "message tool_result content",
			entry: llmreplay.Entry{Kind: llmreplay.KindComplete, Request: llmclient.Request{
				Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.ToolResultBlock("tu_1", secret, false)}}},
			}},
			field: func(e llmreplay.Entry) string { return e.Request.Messages[0].Content[0].Content },
		},
		{
			name: "response text",
			entry: llmreplay.Entry{
				Kind:     llmreplay.KindComplete,
				Response: &llmclient.Response{Text: secret},
			},
			field: func(e llmreplay.Entry) string { return e.Response.Text },
		},
		{
			name: "response text block",
			entry: llmreplay.Entry{
				Kind:     llmreplay.KindComplete,
				Response: &llmclient.Response{Content: []llmclient.ContentBlock{llmclient.TextBlock(secret)}},
			},
			field: func(e llmreplay.Entry) string { return e.Response.Content[0].Text },
		},
		{
			name: "chunk text delta (secret intact within one delta)",
			entry: llmreplay.Entry{
				Kind:   llmreplay.KindStream,
				Chunks: []llmreplay.Chunk{{TextDelta: "leak " + secret + " here"}},
			},
			field: func(e llmreplay.Entry) string { return e.Chunks[0].TextDelta },
		},
		{
			name: "chunk block content",
			entry: llmreplay.Entry{
				Kind:   llmreplay.KindStream,
				Chunks: []llmreplay.Chunk{{Block: &llmclient.ContentBlock{Type: llmclient.BlockToolResult, ToolUseID: "tu_1", Content: secret}}},
			},
			field: func(e llmreplay.Entry) string { return e.Chunks[0].Block.Content },
		},
		{
			name: "chunk error message",
			entry: llmreplay.Entry{
				Kind:   llmreplay.KindStream,
				Chunks: []llmreplay.Chunk{{Err: "request rejected: " + secret}},
			},
			field: func(e llmreplay.Entry) string { return e.Chunks[0].Err },
		},
		{
			name:  "entry error message",
			entry: llmreplay.Entry{Kind: llmreplay.KindComplete, Err: "request rejected: " + secret},
			field: func(e llmreplay.Entry) string { return e.Err },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := llmreplay.DefaultScrubber(tc.entry)
			s := tc.field(got)
			if strings.Contains(s, "hunter2-secret") {
				t.Errorf("secret survived scrubbing: %q", s)
			}
			if !strings.Contains(s, label) {
				t.Errorf("redaction label missing: %q", s)
			}
			// The scrubber must not mutate its input: RecordingClient hands it
			// entries while the live stream is still being forwarded.
			if !strings.Contains(tc.field(tc.entry), "hunter2-secret") {
				t.Error("DefaultScrubber mutated its input entry")
			}
		})
	}
}

// TestDefaultScrubber_RawJSONNotScanned locks the DOCUMENTED limitation:
// tool_use Input and BlockUnknown Raw are raw JSON and are not scanned, so
// secrets inside them survive. This is a known gap, stated loudly in the
// package doc — the test exists so the behaviour never changes silently.
func TestDefaultScrubber_RawJSONNotScanned(t *testing.T) {
	const secret = "password=hunter2-secret"
	input, err := json.Marshal(map[string]string{"arg": secret})
	if err != nil {
		t.Fatal(err)
	}

	entry := llmreplay.Entry{
		Kind: llmreplay.KindComplete,
		Response: &llmclient.Response{
			Content: []llmclient.ContentBlock{llmclient.ToolUseBlock("tu_1", "run", input)},
		},
	}
	got := llmreplay.DefaultScrubber(entry)
	if !strings.Contains(string(got.Response.Content[0].Input), "hunter2-secret") {
		t.Error("tool_use Input was scanned — the documented limitation changed; update the package doc and this test together")
	}
}

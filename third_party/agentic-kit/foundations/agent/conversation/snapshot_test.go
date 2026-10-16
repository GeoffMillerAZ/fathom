package conversation_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/conversation"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// buildSnapshotConv exercises every block variant: cache hints, unknown
// blocks, a completed tool exchange with an error result, and a final
// assistant message leaving tu_1 and tu_2 pending.
func buildSnapshotConv(t *testing.T) *conversation.Conversation {
	t.Helper()
	c := conversation.New(conversation.Options{})
	must(t, c.Append(llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
		{Type: llmclient.BlockText, Text: "hi", CacheHint: true},
	}}))
	must(t, c.AppendAssistant(llmclient.Response{Content: []llmclient.ContentBlock{
		llmclient.TextBlock("ok"),
		{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"hmm"}`)},
	}}))
	must(t, c.AppendUserText("go"))
	must(t, c.AppendAssistant(toolResp("", llmclient.ToolUseBlock("tu_0", "probe", json.RawMessage(`{"d":1}`)))))
	must(t, c.AppendToolResults(llmclient.ToolResultBlock("tu_0", "boom", true)))
	must(t, c.AppendAssistant(textResp("retrying differently")))
	must(t, c.AppendUserText("more"))
	must(t, c.AppendAssistant(toolResp(
		"two calls",
		llmclient.ToolUseBlock("tu_1", "search", json.RawMessage(`{"q":"x"}`)),
		llmclient.ToolUseBlock("tu_2", "fetch", nil), // nil Input must round-trip as nil
	)))
	return c
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	c := buildSnapshotConv(t)

	data, err := c.Snapshot()
	must(t, err)
	if !json.Valid(data) {
		t.Fatal("snapshot is not valid JSON")
	}

	r := conversation.New(conversation.Options{})
	must(t, r.Restore(data))

	if !reflect.DeepEqual(r.Messages(), c.Messages()) {
		t.Fatalf("messages did not round-trip:\n got %+v\nwant %+v", r.Messages(), c.Messages())
	}
	if !r.HasPendingTools() {
		t.Fatal("pending state lost in round-trip")
	}
	if !reflect.DeepEqual(r.PendingToolUses(), c.PendingToolUses()) {
		t.Fatalf("pending tool uses did not round-trip:\n got %+v\nwant %+v",
			r.PendingToolUses(), c.PendingToolUses())
	}

	// The restored conversation is live: the pending tools are answerable.
	must(t, r.AppendToolResults(
		llmclient.ToolResultBlock("tu_1", "r1", false),
		llmclient.ToolResultBlock("tu_2", "r2", true),
	))
	if r.HasPendingTools() {
		t.Fatal("restored conversation kept stale pending state")
	}
}

func TestSnapshotRestoreEmptyConversation(t *testing.T) {
	c := conversation.New(conversation.Options{})
	data, err := c.Snapshot()
	must(t, err)

	r := conversation.New(conversation.Options{})
	must(t, r.AppendUserText("pre-existing")) // Restore must replace this
	must(t, r.Restore(data))
	if got := r.Messages(); got != nil {
		t.Fatalf("Messages after restoring empty snapshot = %+v, want nil", got)
	}
	if r.HasPendingTools() {
		t.Fatal("pending state survived restore of empty snapshot")
	}
}

func TestRestoreRejectsBadSnapshots(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"invalid json", `{`, "restore"},
		{"wrong version", `{"version":2,"messages":[]}`, "version"},
		{"missing version", `{"messages":[]}`, "version"},
		{
			"assistant first",
			`{"version":1,"messages":[{"role":"assistant","content":[{"type":"text","text":"x"}]}]}`,
			"first message",
		},
		{
			"roles do not alternate",
			`{"version":1,"messages":[
				{"role":"user","content":[{"type":"text","text":"a"}]},
				{"role":"user","content":[{"type":"text","text":"b"}]}]}`,
			"alternate",
		},
		{
			"orphaned tool_result",
			`{"version":1,"messages":[
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_9","content":"x"}]}]}`,
			"orphaned",
		},
		{
			"split tool results",
			`{"version":1,"messages":[
				{"role":"user","content":[{"type":"text","text":"a"}]},
				{"role":"assistant","content":[
					{"type":"tool_use","id":"tu_1","name":"t"},
					{"type":"tool_use","id":"tu_2","name":"t"}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"x"}]}]}`,
			"missing",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := conversation.New(conversation.Options{})
			must(t, r.AppendUserText("keep me"))
			before := r.Messages()

			err := r.Restore([]byte(tc.data))
			if err == nil {
				t.Fatalf("Restore(%s) succeeded, want error containing %q", tc.data, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
			if !reflect.DeepEqual(r.Messages(), before) {
				t.Fatal("failed Restore mutated the receiver")
			}
		})
	}
}

func TestRestoreReplacesExistingState(t *testing.T) {
	c := buildSnapshotConv(t)
	data, err := c.Snapshot()
	must(t, err)

	r := conversation.New(conversation.Options{})
	must(t, r.AppendUserText("old"))
	must(t, r.AppendAssistant(toolResp("", llmclient.ToolUseBlock("tu_old", "t", nil))))
	if !r.HasPendingTools() {
		t.Fatal("setup: expected pending tools")
	}

	must(t, r.Restore(data))
	if !reflect.DeepEqual(r.Messages(), c.Messages()) {
		t.Fatal("Restore did not replace prior messages")
	}
	ids := r.PendingToolUses()
	if len(ids) != 2 || ids[0].ID != "tu_1" || ids[1].ID != "tu_2" {
		t.Fatalf("pending after restore = %+v, want tu_1, tu_2", ids)
	}
}

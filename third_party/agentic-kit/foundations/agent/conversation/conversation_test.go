package conversation_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/agent/conversation"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// --- shared helpers -------------------------------------------------------

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// textResp builds an end_turn response with a single text block.
func textResp(s string) llmclient.Response {
	return llmclient.Response{
		Text:       s,
		Content:    []llmclient.ContentBlock{llmclient.TextBlock(s)},
		StopReason: llmclient.StopEndTurn,
	}
}

// toolResp builds a tool_use response with optional leading text.
func toolResp(text string, calls ...llmclient.ContentBlock) llmclient.Response {
	var content []llmclient.ContentBlock
	if text != "" {
		content = append(content, llmclient.TextBlock(text))
	}
	content = append(content, calls...)
	return llmclient.Response{Text: text, Content: content, StopReason: llmclient.StopToolUse}
}

// assertValidSequence replays msgs through a fresh Conversation: every
// sequence Window or Messages returns must be appendable from scratch.
func assertValidSequence(t *testing.T, msgs []llmclient.Message) {
	t.Helper()
	fresh := conversation.New(conversation.Options{})
	for i, m := range msgs {
		if err := fresh.Append(m); err != nil {
			t.Fatalf("message %d (role %q) makes the sequence invalid: %v", i, m.Role, err)
		}
	}
}

func msgHasToolResult(m llmclient.Message) bool {
	for _, b := range m.Content {
		if b.Type == llmclient.BlockToolResult {
			return true
		}
	}
	return false
}

func wantErrContaining(t *testing.T, err error, substrs ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error containing %q, got nil", substrs)
	}
	for _, s := range substrs {
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("error %q does not contain %q", err, s)
		}
	}
}

// newPendingConv returns a conversation whose last assistant message left
// tool uses tu_1 and tu_2 pending.
func newPendingConv(t *testing.T) *conversation.Conversation {
	t.Helper()
	c := conversation.New(conversation.Options{})
	must(t, c.AppendUserText("find me two things"))
	must(t, c.AppendAssistant(toolResp(
		"on it",
		llmclient.ToolUseBlock("tu_1", "search", json.RawMessage(`{"q":"a"}`)),
		llmclient.ToolUseBlock("tu_2", "search", json.RawMessage(`{"q":"b"}`)),
	)))
	return c
}

// --- pairing --------------------------------------------------------------

func TestPairingHappyPath(t *testing.T) {
	c := newPendingConv(t)
	if !c.HasPendingTools() {
		t.Fatal("HasPendingTools = false after assistant tool_use")
	}
	pending := c.PendingToolUses()
	if len(pending) != 2 || pending[0].ID != "tu_1" || pending[1].ID != "tu_2" {
		t.Fatalf("PendingToolUses = %+v, want tu_1, tu_2 in order", pending)
	}

	// Answer both in one call, deliberately out of order: order must not
	// matter for coverage, and both land in ONE user message.
	must(t, c.AppendToolResults(
		llmclient.ToolResultBlock("tu_2", "result B", false),
		llmclient.ToolResultBlock("tu_1", "result A", true),
	))
	if c.HasPendingTools() {
		t.Fatal("HasPendingTools = true after results appended")
	}
	if got := c.PendingToolUses(); got != nil {
		t.Fatalf("PendingToolUses = %+v, want nil", got)
	}

	must(t, c.AppendAssistant(textResp("done")))

	msgs := c.Messages()
	if len(msgs) != 4 {
		t.Fatalf("len(Messages()) = %d, want 4", len(msgs))
	}
	wantRoles := []llmclient.Role{llmclient.RoleUser, llmclient.RoleAssistant, llmclient.RoleUser, llmclient.RoleAssistant}
	for i, r := range wantRoles {
		if msgs[i].Role != r {
			t.Fatalf("message %d role = %q, want %q", i, msgs[i].Role, r)
		}
	}
	results := msgs[2]
	if len(results.Content) != 2 ||
		results.Content[0].Type != llmclient.BlockToolResult ||
		results.Content[1].Type != llmclient.BlockToolResult {
		t.Fatalf("results message = %+v, want exactly two tool_result blocks", results)
	}
	if results.Content[0].ToolUseID != "tu_2" || results.Content[1].ToolUseID != "tu_1" {
		t.Fatalf("result order = %q,%q, want caller order tu_2,tu_1",
			results.Content[0].ToolUseID, results.Content[1].ToolUseID)
	}
	if !results.Content[1].IsError {
		t.Fatal("IsError dropped from tu_1 result")
	}
	assertValidSequence(t, msgs)
}

func TestAppendToolResultsErrors(t *testing.T) {
	plainConv := func(t *testing.T) *conversation.Conversation {
		c := conversation.New(conversation.Options{})
		must(t, c.AppendUserText("hi"))
		must(t, c.AppendAssistant(textResp("hello")))
		return c
	}
	tests := []struct {
		name    string
		setup   func(*testing.T) *conversation.Conversation
		results []llmclient.ContentBlock
		want    []string
	}{
		{
			name:  "no pending tool uses",
			setup: plainConv,
			results: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
			},
			want: []string{"no pending"},
		},
		{
			name:  "missing one id",
			setup: newPendingConv,
			results: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
			},
			want: []string{"missing", "tu_2"},
		},
		{
			name:    "empty result set names all pending ids",
			setup:   newPendingConv,
			results: nil,
			want:    []string{"missing", "tu_1", "tu_2"},
		},
		{
			name:  "unknown id",
			setup: newPendingConv,
			results: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
				llmclient.ToolResultBlock("tu_2", "y", false),
				llmclient.ToolResultBlock("tu_9", "z", false),
			},
			want: []string{"unknown", "tu_9"},
		},
		{
			name:  "duplicate id",
			setup: newPendingConv,
			results: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
				llmclient.ToolResultBlock("tu_2", "y", false),
				llmclient.ToolResultBlock("tu_1", "x again", false),
			},
			want: []string{"duplicate", "tu_1"},
		},
		{
			name:  "non-tool_result block",
			setup: newPendingConv,
			results: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
				llmclient.TextBlock("not a result"),
			},
			want: []string{"tool_result"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.setup(t)
			before := c.Messages()
			beforePending := c.PendingToolUses()

			err := c.AppendToolResults(tc.results...)
			wantErrContaining(t, err, tc.want...)

			if !reflect.DeepEqual(c.Messages(), before) {
				t.Fatal("failed AppendToolResults mutated messages")
			}
			if !reflect.DeepEqual(c.PendingToolUses(), beforePending) {
				t.Fatal("failed AppendToolResults mutated pending state")
			}
		})
	}
}

func TestAppendToolResultsRecoversAfterError(t *testing.T) {
	c := newPendingConv(t)
	err := c.AppendToolResults(llmclient.ToolResultBlock("tu_1", "x", false))
	wantErrContaining(t, err, "missing", "tu_2")

	must(t, c.AppendToolResults(
		llmclient.ToolResultBlock("tu_1", "x", false),
		llmclient.ToolResultBlock("tu_2", "y", false),
	))
	if c.HasPendingTools() {
		t.Fatal("pending tools remain after successful retry")
	}
}

// --- alternation and block-placement violations ----------------------------

func TestAppendInvariantViolations(t *testing.T) {
	empty := func(t *testing.T) *conversation.Conversation {
		return conversation.New(conversation.Options{})
	}
	afterUser := func(t *testing.T) *conversation.Conversation {
		c := conversation.New(conversation.Options{})
		must(t, c.AppendUserText("hi"))
		return c
	}
	afterAssistant := func(t *testing.T) *conversation.Conversation {
		c := afterUser(t)
		must(t, c.AppendAssistant(textResp("hello")))
		return c
	}
	tests := []struct {
		name  string
		setup func(*testing.T) *conversation.Conversation
		msg   llmclient.Message
		want  []string
	}{
		{
			name:  "assistant first",
			setup: empty,
			msg:   llmclient.Message{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
			want:  []string{"first message", "user"},
		},
		{
			name:  "user after user",
			setup: afterUser,
			msg:   llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("again")}},
			want:  []string{"alternate"},
		},
		{
			name:  "assistant after assistant",
			setup: afterAssistant,
			msg:   llmclient.Message{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("more")}},
			want:  []string{"alternate"},
		},
		{
			name:  "invalid role",
			setup: empty,
			msg:   llmclient.Message{Role: llmclient.Role("system"), Content: []llmclient.ContentBlock{llmclient.TextBlock("x")}},
			want:  []string{"invalid role"},
		},
		{
			name:  "empty content",
			setup: empty,
			msg:   llmclient.Message{Role: llmclient.RoleUser},
			want:  []string{"non-empty"},
		},
		{
			name:  "empty text block",
			setup: empty,
			msg:   llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("")}},
			want:  []string{"empty text block"},
		},
		{
			name:  "tool_use in user message",
			setup: empty,
			msg: llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("tu_1", "t", nil),
			}},
			want: []string{"only valid in assistant"},
		},
		{
			name:  "tool_result in assistant message",
			setup: afterUser,
			msg: llmclient.Message{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_1", "x", false),
			}},
			want: []string{"only valid in user"},
		},
		{
			name:  "orphaned tool_result",
			setup: afterAssistant,
			msg: llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("tu_9", "x", false),
			}},
			want: []string{"orphaned", "tu_9"},
		},
		{
			name:  "tool_result after text in same message",
			setup: newPendingConv,
			msg: llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				llmclient.TextBlock("here you go"),
				llmclient.ToolResultBlock("tu_1", "x", false),
				llmclient.ToolResultBlock("tu_2", "y", false),
			}},
			want: []string{"must come first"},
		},
		{
			name:  "duplicate tool_use ids in assistant",
			setup: afterUser,
			msg: llmclient.Message{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("tu_1", "t", nil),
				llmclient.ToolUseBlock("tu_1", "t", nil),
			}},
			want: []string{"duplicate tool_use ID", "tu_1"},
		},
		{
			name:  "tool_use with empty id",
			setup: afterUser,
			msg: llmclient.Message{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("", "t", nil),
			}},
			want: []string{"empty ID"},
		},
		{
			name:  "tool_result with empty ToolUseID",
			setup: newPendingConv,
			msg: llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
				llmclient.ToolResultBlock("", "x", false),
			}},
			want: []string{"empty ToolUseID"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.setup(t)
			before := c.Messages()
			err := c.Append(tc.msg)
			wantErrContaining(t, err, tc.want...)
			if !reflect.DeepEqual(c.Messages(), before) {
				t.Fatal("failed Append mutated messages")
			}
		})
	}
}

func TestAppendMixedToolResultsAndText(t *testing.T) {
	// Advanced path: results first, then text, in ONE user message.
	c := newPendingConv(t)
	must(t, c.Append(llmclient.Message{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
		llmclient.ToolResultBlock("tu_1", "a", false),
		llmclient.ToolResultBlock("tu_2", "b", false),
		llmclient.TextBlock("both done, please summarise"),
	}}))
	if c.HasPendingTools() {
		t.Fatal("pending tools remain after mixed results+text message")
	}
	assertValidSequence(t, c.Messages())
}

func TestAppendUserTextErrors(t *testing.T) {
	c := conversation.New(conversation.Options{})
	wantErrContaining(t, c.AppendUserText(""), "empty user text")

	p := newPendingConv(t)
	// Plain text while results are pending must fail, naming the IDs.
	wantErrContaining(t, p.AppendUserText("ignoring your tools"), "missing", "tu_1", "tu_2")
}

func TestAppendAssistantEdgeCases(t *testing.T) {
	t.Run("v1 response with Text only", func(t *testing.T) {
		c := conversation.New(conversation.Options{})
		must(t, c.AppendUserText("hi"))
		must(t, c.AppendAssistant(llmclient.Response{Text: "hello", StopReason: llmclient.StopEndTurn}))
		msgs := c.Messages()
		want := []llmclient.ContentBlock{llmclient.TextBlock("hello")}
		if !reflect.DeepEqual(msgs[1].Content, want) {
			t.Fatalf("synthesised content = %+v, want %+v", msgs[1].Content, want)
		}
	})
	t.Run("entirely empty response", func(t *testing.T) {
		c := conversation.New(conversation.Options{})
		must(t, c.AppendUserText("hi"))
		wantErrContaining(t, c.AppendAssistant(llmclient.Response{}), "non-empty")
	})
	t.Run("assistant on empty conversation", func(t *testing.T) {
		c := conversation.New(conversation.Options{})
		wantErrContaining(t, c.AppendAssistant(textResp("hi")), "first message")
	})
	t.Run("unknown blocks pass through", func(t *testing.T) {
		c := conversation.New(conversation.Options{})
		must(t, c.AppendUserText("hi"))
		raw := json.RawMessage(`{"type":"thinking","thinking":"hmm"}`)
		must(t, c.AppendAssistant(llmclient.Response{Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockUnknown, Raw: raw},
			llmclient.TextBlock("answer"),
		}}))
		msgs := c.Messages()
		if msgs[1].Content[0].Type != llmclient.BlockUnknown || string(msgs[1].Content[0].Raw) != string(raw) {
			t.Fatalf("unknown block not preserved verbatim: %+v", msgs[1].Content[0])
		}
	})
}

// --- estimation -----------------------------------------------------------

func TestTokenEstimateDefaultHeuristic(t *testing.T) {
	// Default estimator is ceil(len/4); pieces are text, tool name, tool
	// input JSON, tool_result content, and unknown raw JSON.
	c := conversation.New(conversation.Options{})

	must(t, c.AppendUserText("abcde")) // ceil(5/4) = 2
	if got := c.TokenEstimate(); got != 2 {
		t.Fatalf("TokenEstimate = %d, want 2", got)
	}

	// text "ack!" -> 1, name "tool" -> 1, input {"x":1} (7 chars) -> 2.
	must(t, c.AppendAssistant(toolResp("ack!", llmclient.ToolUseBlock("tu_1", "tool", json.RawMessage(`{"x":1}`)))))
	if got := c.TokenEstimate(); got != 6 {
		t.Fatalf("TokenEstimate = %d, want 6", got)
	}

	// tool_result content "rrrr" -> 1.
	must(t, c.AppendToolResults(llmclient.ToolResultBlock("tu_1", "rrrr", false)))
	if got := c.TokenEstimate(); got != 7 {
		t.Fatalf("TokenEstimate = %d, want 7", got)
	}

	// text "kk" -> 1, unknown raw {"t":"think"} (13 chars) -> 4.
	must(t, c.AppendAssistant(llmclient.Response{Content: []llmclient.ContentBlock{
		llmclient.TextBlock("kk"),
		{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"t":"think"}`)},
	}}))
	if got := c.TokenEstimate(); got != 12 {
		t.Fatalf("TokenEstimate = %d, want 12", got)
	}
}

func TestEstimatorInjection(t *testing.T) {
	calls := 0
	c := conversation.New(conversation.Options{
		Estimator: func(s string) int { calls++; return 7 },
	})
	must(t, c.AppendUserText("anything"))
	must(t, c.AppendAssistant(toolResp("", llmclient.ToolUseBlock("tu_1", "t", json.RawMessage(`{}`)))))

	calls = 0
	// Pieces: user text, tool name, tool input -> 3 calls x 7.
	if got := c.TokenEstimate(); got != 21 {
		t.Fatalf("TokenEstimate = %d, want 21", got)
	}
	if calls != 3 {
		t.Fatalf("estimator called %d times, want 3", calls)
	}
}

func TestEstimatorSkipsEmptyPayloads(t *testing.T) {
	// An estimator with a constant floor must not be fed empty strings:
	// a tool_use with nil Input contributes only its name.
	c := conversation.New(conversation.Options{
		Estimator: func(string) int { return 5 },
	})
	must(t, c.AppendUserText("x"))
	must(t, c.AppendAssistant(toolResp("", llmclient.ToolUseBlock("tu_1", "n", nil))))
	if got := c.TokenEstimate(); got != 10 {
		t.Fatalf("TokenEstimate = %d, want 10 (text + name only)", got)
	}
}

// --- aliasing safety -------------------------------------------------------

func TestMessagesAliasingSafety(t *testing.T) {
	c := newPendingConv(t)

	msgs := c.Messages()
	msgs[0].Role = llmclient.RoleAssistant
	msgs[0].Content[0].Text = "mutated"
	msgs[1].Content = append(msgs[1].Content, llmclient.TextBlock("extra"))
	_ = append(msgs, llmclient.Message{Role: llmclient.RoleUser})

	fresh := c.Messages()
	if fresh[0].Role != llmclient.RoleUser || fresh[0].Content[0].Text != "find me two things" {
		t.Fatalf("caller mutation leaked into conversation: %+v", fresh[0])
	}
	if len(fresh[1].Content) != 3 {
		t.Fatalf("caller append leaked into conversation: %d blocks", len(fresh[1].Content))
	}

	// PendingToolUses returns copies too.
	pending := c.PendingToolUses()
	pending[0].ID = "mutated"
	if got := c.PendingToolUses()[0].ID; got != "tu_1" {
		t.Fatalf("PendingToolUses mutation leaked: %q", got)
	}

	// Window results are copies at the same depth.
	w, _ := c.Window(1 << 30)
	w[0].Content[0].Text = "mutated"
	if got := c.Messages()[0].Content[0].Text; got != "find me two things" {
		t.Fatalf("Window mutation leaked: %q", got)
	}

	// Snapshots taken via Messages stay stable across later appends.
	snap := c.Messages()
	must(t, c.AppendToolResults(
		llmclient.ToolResultBlock("tu_1", "a", false),
		llmclient.ToolResultBlock("tu_2", "b", false),
	))
	if len(snap) != 2 {
		t.Fatalf("earlier Messages() slice changed length: %d", len(snap))
	}
}

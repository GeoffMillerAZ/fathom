package conversation_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/agent/conversation"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// lenEstimator makes window arithmetic exact: 1 token per character.
func lenEstimator(s string) int { return len(s) }

// buildWindowConv builds the canonical 8-message fixture (costs with
// lenEstimator in parentheses; total 39):
//
//	m0 user      "head"                                  (4)
//	m1 assistant "ack!"                                  (4)
//	m2 user      "askA"                                  (4)
//	m3 assistant tool_use tu_1 "tool" {"x":1}            (11)
//	m4 user      tool_result tu_1 "rrrr"                 (4)
//	m5 assistant "ansA"                                  (4)
//	m6 user      "askB"                                  (4)
//	m7 assistant "ansB"                                  (4)
func buildWindowConv(t *testing.T, opts conversation.Options) *conversation.Conversation {
	t.Helper()
	if opts.Estimator == nil {
		opts.Estimator = lenEstimator
	}
	c := conversation.New(opts)
	must(t, c.AppendUserText("head"))
	must(t, c.AppendAssistant(textResp("ack!")))
	must(t, c.AppendUserText("askA"))
	must(t, c.AppendAssistant(toolResp("", llmclient.ToolUseBlock("tu_1", "tool", json.RawMessage(`{"x":1}`)))))
	must(t, c.AppendToolResults(llmclient.ToolResultBlock("tu_1", "rrrr", false)))
	must(t, c.AppendAssistant(textResp("ansA")))
	must(t, c.AppendUserText("askB"))
	must(t, c.AppendAssistant(textResp("ansB")))
	return c
}

func firstText(m llmclient.Message) string {
	for _, b := range m.Content {
		if b.Type == llmclient.BlockText {
			return b.Text
		}
	}
	return ""
}

func TestWindowSuffixAcrossBudgets(t *testing.T) {
	tests := []struct {
		name        string
		budget      int
		wantStart   string // first text block of the first returned message
		wantDropped int
		wantLen     int
	}{
		{"huge budget keeps all", 1000, "head", 0, 8},
		{"exact total keeps all", 39, "head", 0, 8},
		{"one under total cuts to askA", 38, "askA", 2, 6},
		{"exact fit at askA", 31, "askA", 2, 6},
		{"one under askA jumps past the tool pair to askB", 30, "askB", 6, 2},
		{"exact fit at askB", 8, "askB", 6, 2},
		{"smaller than last exchange still keeps it", 7, "askB", 6, 2},
		{"zero budget keeps last exchange", 0, "askB", 6, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := buildWindowConv(t, conversation.Options{})
			w, dropped := c.Window(tc.budget)
			if dropped != tc.wantDropped {
				t.Fatalf("dropped = %d, want %d", dropped, tc.wantDropped)
			}
			if len(w) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(w), tc.wantLen)
			}
			if got := firstText(w[0]); got != tc.wantStart {
				t.Fatalf("window starts at %q, want %q", got, tc.wantStart)
			}
			// With no head, the result is a true suffix.
			all := c.Messages()
			if !reflect.DeepEqual(w, all[len(all)-len(w):]) {
				t.Fatal("window is not a suffix of the conversation")
			}
			assertValidSequence(t, w)
		})
	}
}

func TestWindowNeverOrphansToolPairs(t *testing.T) {
	// Sweep every budget: no result may start with (or contain an
	// unanswered) tool_result, and every result must replay as a valid
	// conversation. Budgets 12..30 in particular straddle the tool pair
	// (m3+m4 cost 15): the cut must jump the whole pair, never split it.
	c := buildWindowConv(t, conversation.Options{})
	for budget := 0; budget <= 45; budget++ {
		w, dropped := c.Window(budget)
		if len(w) == 0 {
			t.Fatalf("budget %d: empty window", budget)
		}
		if w[0].Role != llmclient.RoleUser {
			t.Fatalf("budget %d: window starts with role %q", budget, w[0].Role)
		}
		if msgHasToolResult(w[0]) {
			t.Fatalf("budget %d: window starts with an orphaned tool_result", budget)
		}
		if dropped+len(w) != 8 {
			t.Fatalf("budget %d: dropped(%d)+len(%d) != 8", budget, dropped, len(w))
		}
		assertValidSequence(t, w)
	}
}

func TestWindowKeepsTrailingPendingPair(t *testing.T) {
	// Conversation ends with an unanswered tool_use; the minimum window is
	// the last clean user message plus that assistant message.
	c := conversation.New(conversation.Options{Estimator: lenEstimator})
	must(t, c.AppendUserText("askA"))            // 4
	must(t, c.AppendAssistant(textResp("ansA"))) // 4
	must(t, c.AppendUserText("askB"))            // 4
	must(t, c.AppendAssistant(toolResp("",       // 11
		llmclient.ToolUseBlock("tu_1", "tool", json.RawMessage(`{"x":1}`)))))

	w, dropped := c.Window(14) // suffix from askB costs 15 — over budget, kept anyway
	if dropped != 2 || len(w) != 2 {
		t.Fatalf("dropped,len = %d,%d, want 2,2", dropped, len(w))
	}
	if firstText(w[0]) != "askB" || len(w[1].Content) != 1 || w[1].Content[0].Type != llmclient.BlockToolUse {
		t.Fatalf("window = %+v, want askB + pending tool_use", w)
	}
	assertValidSequence(t, w)
}

func TestWindowEmptyConversation(t *testing.T) {
	c := conversation.New(conversation.Options{})
	w, dropped := c.Window(10)
	if w != nil || dropped != 0 {
		t.Fatalf("Window on empty = %v,%d, want nil,0", w, dropped)
	}
}

func TestWindowPreserveHead(t *testing.T) {
	t.Run("head ending on assistant needs no seam repair", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 2})
		w, dropped := c.Window(16) // head 8 + suffix(askB) 8
		if dropped != 4 || len(w) != 4 {
			t.Fatalf("dropped,len = %d,%d, want 4,4", dropped, len(w))
		}
		wantTexts := []string{"head", "ack!", "askB", "ansB"}
		for i, want := range wantTexts {
			if got := firstText(w[i]); got != want {
				t.Fatalf("message %d = %q, want %q", i, got, want)
			}
		}
		assertValidSequence(t, w)
	})

	t.Run("head ending on user merges across the seam", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 1})
		w, dropped := c.Window(17) // head 4 + suffix(askB) 8 fits; m1..m5 dropped
		if dropped != 5 || len(w) != 2 {
			t.Fatalf("dropped,len = %d,%d, want 5,2", dropped, len(w))
		}
		// m0 and m6 merged into one user message, head content first.
		if w[0].Role != llmclient.RoleUser || len(w[0].Content) != 2 ||
			w[0].Content[0].Text != "head" || w[0].Content[1].Text != "askB" {
			t.Fatalf("merged seam message = %+v, want user[head askB]", w[0])
		}
		if firstText(w[1]) != "ansB" {
			t.Fatalf("message 1 = %q, want ansB", firstText(w[1]))
		}
		assertValidSequence(t, w)
	})

	t.Run("head boundary never splits a tool pair", func(t *testing.T) {
		// PreserveHead=4 ends on the tool_use assistant message m3; the
		// head is extended to include the tool_result message m4.
		c := buildWindowConv(t, conversation.Options{PreserveHead: 4})
		w, dropped := c.Window(35) // extended head 27 + suffix(askB) 8
		if dropped != 1 || len(w) != 6 {
			t.Fatalf("dropped,len = %d,%d, want 1,6", dropped, len(w))
		}
		// m4 (user tool_result, head end) merges with m6 (clean user start):
		// results stay first in the merged message.
		seam := w[4]
		if seam.Role != llmclient.RoleUser || len(seam.Content) != 2 ||
			seam.Content[0].Type != llmclient.BlockToolResult || seam.Content[0].ToolUseID != "tu_1" ||
			seam.Content[1].Text != "askB" {
			t.Fatalf("seam message = %+v, want [tool_result(tu_1) text(askB)]", seam)
		}
		assertValidSequence(t, w)
	})

	t.Run("head larger than conversation disables windowing", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 20})
		w, dropped := c.Window(1)
		if dropped != 0 || len(w) != 8 {
			t.Fatalf("dropped,len = %d,%d, want 0,8", dropped, len(w))
		}
		assertValidSequence(t, w)
	})

	t.Run("head plus minimum window may exceed budget", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 2})
		w, dropped := c.Window(10) // head 8 + cheapest suffix 8 > 10: kept anyway
		if dropped != 4 || len(w) != 4 {
			t.Fatalf("dropped,len = %d,%d, want 4,4", dropped, len(w))
		}
		if firstText(w[2]) != "askB" {
			t.Fatalf("suffix starts at %q, want askB", firstText(w[2]))
		}
		assertValidSequence(t, w)
	})
}

func TestWindowCompactor(t *testing.T) {
	summary := func(_ context.Context, dropped []llmclient.Message) (llmclient.Message, bool, error) {
		return llmclient.Message{
			Role:    llmclient.RoleUser, // ignored: placement is role-safe
			Content: []llmclient.ContentBlock{llmclient.TextBlock("SUMMARY")},
		}, true, nil
	}

	t.Run("no head: summary prepended to suffix start", func(t *testing.T) {
		var got []llmclient.Message
		opts := conversation.Options{
			Compactor: func(_ context.Context, dropped []llmclient.Message) (llmclient.Message, bool, error) {
				got = dropped
				// Mutating the input must not corrupt the conversation.
				dropped[0].Content[0].Text = "corrupted"
				return summary(context.Background(), dropped)
			},
		}
		c := buildWindowConv(t, opts)
		w, dropped := c.Window(30) // cut at m6
		if dropped != 6 || len(w) != 2 {
			t.Fatalf("dropped,len = %d,%d, want 6,2", dropped, len(w))
		}
		if len(got) != 6 || firstText(got[0]) != "corrupted" || firstText(got[5]) != "ansA" {
			t.Fatalf("compactor received %d messages (first %q, last %q), want m0..m5",
				len(got), firstText(got[0]), firstText(got[5]))
		}
		if c.Messages()[0].Content[0].Text != "head" {
			t.Fatal("compactor mutation of its input leaked into the conversation")
		}
		// Summary rides at the top of the first kept user message.
		if w[0].Role != llmclient.RoleUser || len(w[0].Content) != 2 ||
			w[0].Content[0].Text != "SUMMARY" || w[0].Content[1].Text != "askB" {
			t.Fatalf("first message = %+v, want user[SUMMARY askB]", w[0])
		}
		assertValidSequence(t, w)
	})

	t.Run("head ends user: standalone assistant summary", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 1, Compactor: summary})
		w, dropped := c.Window(17)
		if dropped != 5 || len(w) != 4 {
			t.Fatalf("dropped,len = %d,%d, want 5,4", dropped, len(w))
		}
		if w[1].Role != llmclient.RoleAssistant || firstText(w[1]) != "SUMMARY" {
			t.Fatalf("message 1 = %+v, want assistant SUMMARY", w[1])
		}
		wantTexts := []string{"head", "SUMMARY", "askB", "ansB"}
		for i, want := range wantTexts {
			if got := firstText(w[i]); got != want {
				t.Fatalf("message %d = %q, want %q", i, got, want)
			}
		}
		assertValidSequence(t, w)
	})

	t.Run("head ends assistant: summary prepended to suffix start", func(t *testing.T) {
		c := buildWindowConv(t, conversation.Options{PreserveHead: 2, Compactor: summary})
		w, dropped := c.Window(16)
		if dropped != 4 || len(w) != 4 {
			t.Fatalf("dropped,len = %d,%d, want 4,4", dropped, len(w))
		}
		if w[2].Role != llmclient.RoleUser || len(w[2].Content) != 2 ||
			w[2].Content[0].Text != "SUMMARY" || w[2].Content[1].Text != "askB" {
			t.Fatalf("message 2 = %+v, want user[SUMMARY askB]", w[2])
		}
		assertValidSequence(t, w)
	})

	t.Run("compactor declines: seam still repaired", func(t *testing.T) {
		decline := func(_ context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
			return llmclient.Message{}, false, nil
		}
		c := buildWindowConv(t, conversation.Options{PreserveHead: 1, Compactor: decline})
		w, dropped := c.Window(17)
		if dropped != 5 || len(w) != 2 {
			t.Fatalf("dropped,len = %d,%d, want 5,2", dropped, len(w))
		}
		if len(w[0].Content) != 2 || w[0].Content[0].Text != "head" || w[0].Content[1].Text != "askB" {
			t.Fatalf("merged seam message = %+v, want user[head askB]", w[0])
		}
		assertValidSequence(t, w)
	})

	t.Run("summary with tool blocks is discarded", func(t *testing.T) {
		bad := func(_ context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
			return llmclient.Message{Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("tu_x", "t", nil),
			}}, true, nil
		}
		c := buildWindowConv(t, conversation.Options{PreserveHead: 1, Compactor: bad})
		w, _ := c.Window(17)
		for i, m := range w {
			for _, b := range m.Content {
				if b.ID == "tu_x" {
					t.Fatalf("non-neutral summary block leaked into message %d", i)
				}
			}
		}
		assertValidSequence(t, w)
	})

	t.Run("compactor not called when nothing dropped", func(t *testing.T) {
		called := false
		opts := conversation.Options{
			Compactor: func(_ context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
				called = true
				return llmclient.Message{}, false, nil
			},
		}
		c := buildWindowConv(t, opts)
		w, dropped := c.Window(1000)
		if called {
			t.Fatal("compactor called with nothing dropped")
		}
		if dropped != 0 || len(w) != 8 {
			t.Fatalf("dropped,len = %d,%d, want 0,8", dropped, len(w))
		}
	})

	t.Run("compactor error: window over-budget but intact, error surfaced", func(t *testing.T) {
		compactErr := errors.New("summary LLM unavailable")
		failing := func(_ context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
			return llmclient.Message{}, false, compactErr
		}
		c := buildWindowConv(t, conversation.Options{PreserveHead: 1, Compactor: failing})
		msgs, dropped, err := c.WindowCtx(context.Background(), 17)
		if err == nil {
			t.Fatal("want error from failing compactor, got nil")
		}
		if !errors.Is(err, compactErr) {
			t.Fatalf("error = %v, want %v", err, compactErr)
		}
		// No messages must be lost: dropped==0 and all 8 messages returned.
		if dropped != 0 {
			t.Fatalf("dropped = %d, want 0 (lossless on error)", dropped)
		}
		if len(msgs) != 8 {
			t.Fatalf("len(msgs) = %d, want 8 (all messages intact)", len(msgs))
		}
		assertValidSequence(t, msgs)
	})

	t.Run("WindowCtx passes context to compactor", func(t *testing.T) {
		type ctxKey struct{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
		var receivedCtx context.Context
		opts := conversation.Options{
			Compactor: func(ctx context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
				receivedCtx = ctx
				return llmclient.Message{
					Content: []llmclient.ContentBlock{llmclient.TextBlock("CTX-SUMMARY")},
				}, true, nil
			},
		}
		c := buildWindowConv(t, opts)
		w, _, err := c.WindowCtx(ctx, 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedCtx == nil {
			t.Fatal("compactor did not receive a context")
		}
		if receivedCtx.Value(ctxKey{}) != "sentinel" {
			t.Fatal("compactor received wrong context")
		}
		_ = w
	})
}

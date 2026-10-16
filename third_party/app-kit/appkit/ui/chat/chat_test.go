package chat

import (
	"html/template"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/hitl"
)

func TestRenderMessage_RolesAndLayout(t *testing.T) {
	user := RenderMessage(Message{ID: "m1", Role: RoleUser, AuthorName: "You", BodyHTML: "hi"})
	if !strings.Contains(user, `data-chat-role="user"`) || !strings.Contains(user, "items-end") {
		t.Errorf("user message should be role=user, right-aligned; got:\n%s", user)
	}
	asst := RenderMessage(Message{ID: "m2", Role: RoleAssistant, AuthorName: "Opus", AuthorIsAI: true, BodyHTML: "hello"})
	if !strings.Contains(asst, "items-start") || !strings.Contains(asst, `>AI<`) {
		t.Errorf("assistant message should be left-aligned with an AI chip; got:\n%s", asst)
	}
}

func TestRenderMessage_BodyHTMLTrustedMetaEscaped(t *testing.T) {
	m := RenderMessage(Message{
		ID:         "m1",
		Role:       RoleAssistant,
		AuthorName: `<script>x</script>`,
		At:         "10:02 & later",
		BodyHTML:   template.HTML(`<strong>bold</strong>`),
	})
	if !strings.Contains(m, "<strong>bold</strong>") {
		t.Errorf("BodyHTML should pass through trusted; got:\n%s", m)
	}
	if strings.Contains(m, "<script>x</script>") {
		t.Errorf("author name must be escaped; got:\n%s", m)
	}
	if !strings.Contains(m, "10:02 &amp; later") {
		t.Errorf("timestamp must be escaped; got:\n%s", m)
	}
}

func TestRenderMessage_InlineToolCalls(t *testing.T) {
	m := RenderMessage(Message{
		ID:        "m1",
		Role:      RoleAssistant,
		BodyHTML:  "done",
		ToolCalls: []hitl.ToolCall{{Name: "search_evidence", Status: hitl.ToolOK, Summary: "3 hits"}},
	})
	if !strings.Contains(m, "search_evidence") {
		t.Errorf("inline tool call should render in the bubble; got:\n%s", m)
	}
}

func TestRenderMessage_FooterCopyTokensActions(t *testing.T) {
	m := RenderMessage(Message{ID: "m1", Role: RoleAssistant, BodyHTML: "x", Tokens: "1,240 tokens", Copyable: true})
	if !strings.Contains(m, "data-chat-copy") {
		t.Errorf("copyable message should emit a copy button; got:\n%s", m)
	}
	if !strings.Contains(m, "1,240 tokens") {
		t.Errorf("tokens label should render; got:\n%s", m)
	}
}

func TestRenderMessage_ErrorBubble(t *testing.T) {
	m := RenderMessage(Message{ID: "e1", Role: RoleSystem, BodyHTML: "failed", Error: true})
	if !strings.Contains(m, "border-danger") || !strings.Contains(m, "bg-danger-soft") {
		t.Errorf("error bubble should use danger tokens; got:\n%s", m)
	}
}

func TestRenderTranscript_EmptyState(t *testing.T) {
	got := RenderTranscript(nil, TranscriptOptions{Empty: "No messages <here>"})
	if !strings.Contains(got, "data-chat-empty") {
		t.Errorf("empty transcript should carry data-chat-empty; got:\n%s", got)
	}
	if !strings.Contains(got, "No messages &lt;here&gt;") {
		t.Errorf("empty text must be escaped; got:\n%s", got)
	}
	if !strings.Contains(got, "data-chat-jump") {
		t.Errorf("transcript should always include the jump-to-latest pill; got:\n%s", got)
	}
}

func TestRenderTranscript_ListAndHooks(t *testing.T) {
	got := RenderTranscript([]Message{{ID: "m1", Role: RoleUser, BodyHTML: "hi"}}, TranscriptOptions{})
	for _, want := range []string{"data-chat-scroll", "data-chat-list", `role="log"`, `data-chat-msg="m1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("transcript missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderComposer_HooksAndSlots(t *testing.T) {
	c := RenderComposer(Composer{
		PostURL:     "/chat/send",
		DraftKey:    "draft-case-1",
		ModelPicker: template.HTML(`<select data-model></select>`),
		Stop:        true,
		Hidden:      template.HTML(`<input type="hidden" name="thread" value="t1">`),
	})
	for _, want := range []string{
		`action="/chat/send"`,
		`data-chat-form`,
		`data-chat-draft-key="draft-case-1"`,
		`data-chat-input`,
		`data-chat-send`,
		`data-chat-stop`,
		`<select data-model></select>`,
		`name="thread"`,
	} {
		if !strings.Contains(c, want) {
			t.Errorf("composer missing %q; got:\n%s", want, c)
		}
	}
	// Stop button must start hidden.
	if !strings.Contains(c, `data-chat-stop hidden`) {
		t.Errorf("stop button should be hidden until streaming; got:\n%s", c)
	}
}

func TestRenderComposer_NoStopByDefault(t *testing.T) {
	c := RenderComposer(Composer{PostURL: "/x"})
	if strings.Contains(c, "data-chat-stop") {
		t.Errorf("stop button should be absent unless requested; got:\n%s", c)
	}
}

func TestRenderMessage_CitationsWithProvenanceHook(t *testing.T) {
	m := RenderMessage(Message{
		ID: "m1", Role: RoleAssistant, BodyHTML: "see source",
		Citations: []Citation{
			{ID: "span-7", Kind: "span", Label: "lease-addendum.pdf p.1"},
			{ID: "x-9", Kind: "span", Label: "phantom", Unresolved: true},
		},
	})
	if !strings.Contains(m, "Sources (2)") {
		t.Errorf("citations disclosure missing count; got:\n%s", m)
	}
	if !strings.Contains(m, `data-cite-id="span-7"`) || !strings.Contains(m, `data-cite-kind="span"`) {
		t.Errorf("citation chip missing provenance hooks; got:\n%s", m)
	}
	if !strings.Contains(m, "bg-caution-soft") || !strings.Contains(m, "Unverified") {
		t.Errorf("unresolved citation should render the warning style + tooltip; got:\n%s", m)
	}
}

func TestRenderMessage_SafetyFlags(t *testing.T) {
	m := RenderMessage(Message{
		ID: "m1", Role: RoleAssistant, BodyHTML: "ok",
		SafetyFlags: []SafetyFlag{{Severity: "critical", RuleID: "PI-014", Section: "tool result", Excerpt: "ignore previous"}},
	})
	if !strings.Contains(m, "Safety flags (1)") {
		t.Errorf("safety disclosure missing; got:\n%s", m)
	}
	if !strings.Contains(m, "bg-danger-soft") {
		t.Errorf("critical flag should map to the danger role; got:\n%s", m)
	}
	if !strings.Contains(m, "PI-014") || !strings.Contains(m, "ignore previous") {
		t.Errorf("flag detail missing; got:\n%s", m)
	}
}

func TestSeverityRole(t *testing.T) {
	for sev, want := range map[string]string{"critical": "danger", "high": "danger", "medium": "caution", "low": "info", "": "info"} {
		if got := severityRole(sev); got != want {
			t.Errorf("severityRole(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestThinkingBubbleMatchesHook(t *testing.T) {
	if !strings.Contains(ThinkingBubble(), "data-chat-thinking") {
		t.Errorf("thinking bubble must carry the data-chat-thinking hook")
	}
}

func TestScriptEmbedded(t *testing.T) {
	if !strings.Contains(Script(), "AppKitChat") {
		t.Errorf("chat.js should be embedded and expose AppKitChat")
	}
}

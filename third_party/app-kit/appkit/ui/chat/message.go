package chat

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/hitl"
)

// Message is one turn in the transcript. BodyHTML is trusted rich content the
// caller has already rendered (e.g. markdown→HTML); everything else is escaped.
type Message struct {
	ID          string
	Role        Role
	AuthorName  string          // "You", a model id, "System"
	AuthorIsAI  bool            // renders the AI chip (else a neutral author label)
	BodyHTML    template.HTML   // trusted, pre-rendered body
	At          string          // human timestamp (optional)
	Tokens      string          // optional usage label, e.g. "1,240 tokens · $0.02"
	ToolCalls   []hitl.ToolCall // optional under-the-hood blocks, shown in the bubble
	Citations   []Citation      // optional sources disclosure (with a provenance hook)
	SafetyFlags []SafetyFlag    // optional injection/safety findings disclosure
	Disclosures template.HTML   // optional extra trailing slot
	Actions     template.HTML   // optional action row (retry, thumbs, regenerate)
	Copyable    bool            // render a copy-to-clipboard button
	Error       bool            // render as an error bubble (danger tint)
}

// RenderMessage returns one message bubble as an HTML string.
func RenderMessage(m Message) string {
	var b strings.Builder
	side := "items-start" // assistant/system left
	if m.Role == RoleUser {
		side = "items-end"
	}
	fmt.Fprintf(&b, `<div class="chat-msg flex flex-col %s gap-1" data-chat-msg="%s" data-chat-role="%s">`,
		side, esc(m.ID), esc(string(m.Role)))

	// Meta row: author + optional AI chip + timestamp.
	b.WriteString(`<div class="flex items-center gap-2 text-xs text-ink-muted">`)
	if m.AuthorName != "" {
		fmt.Fprintf(&b, `<span class="font-medium text-ink">%s</span>`, esc(m.AuthorName))
	}
	if m.AuthorIsAI {
		b.WriteString(`<span class="rounded-full bg-accent-soft px-1.5 py-0.5 text-[0.625rem] font-medium text-accent">AI</span>`)
	}
	if m.At != "" {
		fmt.Fprintf(&b, `<span>%s</span>`, esc(m.At))
	}
	b.WriteString(`</div>`)

	// Bubble.
	fmt.Fprintf(&b, `<div class="%s">`, bubbleClass(m.Role, m.Error))
	fmt.Fprintf(&b, `<div class="chat-msg__body text-sm text-ink">%s</div>`, string(m.BodyHTML))

	// Inline tool-calls ("under the hood").
	if len(m.ToolCalls) > 0 {
		b.WriteString(`<div class="mt-3 flex flex-col gap-2">`)
		for _, tc := range m.ToolCalls {
			b.WriteString(hitl.RenderToolCall(tc))
		}
		b.WriteString(`</div>`)
	}
	// Sources + safety disclosures (the trust surface).
	if len(m.Citations) > 0 {
		b.WriteString(renderCitations(m.Citations))
	}
	if len(m.SafetyFlags) > 0 {
		b.WriteString(renderSafetyFlags(m.SafetyFlags))
	}
	if m.Disclosures != "" {
		fmt.Fprintf(&b, `<div class="mt-2">%s</div>`, string(m.Disclosures))
	}
	b.WriteString(`</div>`) // bubble

	// Footer: usage + actions + copy.
	if m.Tokens != "" || m.Actions != "" || m.Copyable {
		b.WriteString(`<div class="flex items-center gap-3 text-xs text-ink-faint">`)
		if m.Tokens != "" {
			fmt.Fprintf(&b, `<span class="tabular-nums">%s</span>`, esc(m.Tokens))
		}
		if m.Actions != "" {
			b.WriteString(string(m.Actions))
		}
		if m.Copyable {
			b.WriteString(`<button type="button" class="hover:text-ink" data-chat-copy aria-label="Copy message">Copy</button>`)
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return b.String()
}

func bubbleClass(role Role, isErr bool) string {
	switch {
	case isErr:
		return "max-w-[42rem] rounded-lg border border-danger bg-danger-soft px-4 py-3"
	case role == RoleUser:
		return "max-w-[42rem] rounded-lg bg-accent-soft px-4 py-3"
	case role == RoleSystem:
		return "max-w-[42rem] rounded-lg border border-line bg-surface-sunken px-4 py-3"
	default: // assistant
		return "max-w-[42rem] rounded-lg border border-line bg-surface-raised px-4 py-3"
	}
}

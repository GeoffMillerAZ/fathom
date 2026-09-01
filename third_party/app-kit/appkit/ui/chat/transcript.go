package chat

import (
	"fmt"
	"strings"
)

// TranscriptOptions configures RenderTranscript.
type TranscriptOptions struct {
	// Empty is shown (as escaped text) when there are no messages.
	Empty string
	// AriaLabel labels the scroll region for assistive tech.
	AriaLabel string
}

// RenderTranscript returns the scrollable message list (or an empty state). The
// region carries data-chat-scroll so chat.js can manage autoscroll + the
// jump-to-latest pill.
func RenderTranscript(msgs []Message, o TranscriptOptions) string {
	aria := o.AriaLabel
	if aria == "" {
		aria = "Conversation"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="chat-transcript relative flex-1 overflow-y-auto" data-chat-scroll role="log" aria-label=%q aria-live="polite">`, esc(aria))
	if len(msgs) == 0 {
		empty := o.Empty
		if empty == "" {
			empty = "Ask a question to begin."
		}
		fmt.Fprintf(&b, `<div data-chat-empty class="flex h-full items-center justify-center p-8 text-center text-sm text-ink-muted">%s</div>`, esc(empty))
	} else {
		b.WriteString(`<div class="flex flex-col gap-5 p-4" data-chat-list>`)
		for _, m := range msgs {
			b.WriteString(RenderMessage(m))
		}
		b.WriteString(`</div>`)
	}
	// Jump-to-latest pill (hidden until chat.js shows it when scrolled up).
	b.WriteString(`<button type="button" data-chat-jump hidden class="sticky bottom-3 left-1/2 -translate-x-1/2 rounded-full border border-line bg-surface-raised px-3 py-1 text-xs text-ink shadow-md hover:bg-surface-sunken">Jump to latest ↓</button>`)
	b.WriteString(`</div>`)
	return b.String()
}

// ThinkingBubble returns an assistant "typing" indicator (animated dots). chat.js
// inserts/removes one around a send; provided here so server-rendered and
// client-inserted markup match.
func ThinkingBubble() string {
	return `<div class="chat-msg flex flex-col items-start gap-1" data-chat-thinking>` +
		`<div class="max-w-[42rem] rounded-lg border border-line bg-surface-raised px-4 py-3">` +
		`<span class="chat-dots inline-flex gap-1" aria-label="Assistant is thinking">` +
		`<span class="size-1.5 animate-pulse rounded-full bg-ink-faint"></span>` +
		`<span class="size-1.5 animate-pulse rounded-full bg-ink-faint [animation-delay:150ms]"></span>` +
		`<span class="size-1.5 animate-pulse rounded-full bg-ink-faint [animation-delay:300ms]"></span>` +
		`</span></div></div>`
}

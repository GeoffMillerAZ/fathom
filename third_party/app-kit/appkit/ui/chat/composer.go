package chat

import (
	"fmt"
	"html/template"
	"strings"
)

// Composer is the message-input row. The app owns PostURL (the loopback POST
// target); chat.js wires the compose comforts via the data-* hooks.
type Composer struct {
	PostURL     string        // form action
	Placeholder string        // textarea placeholder
	Hint        string        // small helper line under the field
	DraftKey    string        // localStorage key for draft persistence (empty = off)
	ModelPicker template.HTML // optional leading control slot (e.g. an el-select)
	Attach      template.HTML // optional attach control slot
	Hidden      template.HTML // optional hidden inputs (case id, thread id, csrf)
	SendLabel   string        // default "Send"
	Stop        bool          // render a Stop button (hidden until streaming)
}

// RenderComposer returns the composer form.
func RenderComposer(c Composer) string {
	send := c.SendLabel
	if send == "" {
		send = "Send"
	}
	ph := c.Placeholder
	if ph == "" {
		ph = "Send a message…  (Enter to send, Shift+Enter for newline)"
	}
	var b strings.Builder
	draftAttr := ""
	if c.DraftKey != "" {
		draftAttr = fmt.Sprintf(` data-chat-draft-key=%q`, esc(c.DraftKey))
	}
	fmt.Fprintf(&b, `<form class="chat-composer border-t border-line bg-surface p-3" method="POST" action=%q data-chat-form%s>`, esc(c.PostURL), draftAttr)
	b.WriteString(string(c.Hidden))
	b.WriteString(`<div class="flex items-end gap-2">`)
	if c.ModelPicker != "" {
		fmt.Fprintf(&b, `<div class="shrink-0">%s</div>`, string(c.ModelPicker))
	}
	if c.Attach != "" {
		fmt.Fprintf(&b, `<div class="shrink-0">%s</div>`, string(c.Attach))
	}
	fmt.Fprintf(&b, `<textarea name="message" rows="1" data-chat-input class="min-h-[2.5rem] max-h-48 flex-1 resize-none rounded-md border border-line bg-surface-raised px-3 py-2 text-sm text-ink placeholder:text-ink-faint focus:outline-none focus:ring-2 focus:ring-focus" placeholder=%q></textarea>`, esc(ph))
	if c.Stop {
		b.WriteString(`<button type="button" data-chat-stop hidden class="inline-flex items-center justify-center rounded-md border border-line bg-surface px-3 py-2 text-sm text-ink hover:bg-surface-sunken">Stop</button>`)
	}
	fmt.Fprintf(&b, `<button type="submit" data-chat-send class="inline-flex items-center justify-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-focus">%s</button>`, esc(send))
	b.WriteString(`</div>`)
	if c.Hint != "" {
		fmt.Fprintf(&b, `<p class="mt-1.5 text-xs text-ink-faint">%s</p>`, esc(c.Hint))
	}
	b.WriteString(`</form>`)
	return b.String()
}

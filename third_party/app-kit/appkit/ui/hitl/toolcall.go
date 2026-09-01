package hitl

// toolcall.go — a tool-call inspector for agentic "under the hood" observability:
// what tool the agent invoked, with what arguments, and what came back. Native
// <details> for the args/result (no JS); deterministic, token-themed, escaped.

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
)

// ToolStatus is the outcome of a tool invocation.
type ToolStatus string

const (
	ToolOK      ToolStatus = "ok"      // completed successfully (positive)
	ToolError   ToolStatus = "error"   // failed (danger)
	ToolRunning ToolStatus = "running" // in flight (info)
	ToolPending ToolStatus = "pending" // queued, not yet run (neutral)
)

// ToolCall is one agent tool invocation. Args and Result are pre-formatted
// strings (e.g. pretty JSON) shown verbatim in a monospace block; the kit does
// not parse them. A person can read exactly what the agent did and got back.
type ToolCall struct {
	Name     string // the tool name, e.g. "search_evidence"
	Status   ToolStatus
	Summary  string // optional one-line gist (e.g. "12 results")
	Duration string // optional, e.g. "320ms"
	Args     string // formatted arguments (collapsible)
	Result   string // formatted result/output (collapsible)
	Open     bool   // render the details expanded
}

func toolStatusRole(s ToolStatus) string {
	switch s {
	case ToolOK:
		return "positive"
	case ToolError:
		return "danger"
	case ToolRunning:
		return "info"
	default:
		return "ink"
	}
}

// RenderToolCall returns the tool-call inspector as an HTML string.
func RenderToolCall(tc ToolCall) string {
	var b strings.Builder
	b.WriteString(`<div class="rounded-md border border-line bg-surface-raised">`)

	// Header: tool name · status badge · optional summary/duration.
	b.WriteString(`<div class="flex flex-wrap items-center gap-2 px-3 py-2">`)
	b.WriteString(`<svg width="14" height="14" viewBox="0 0 16 16" fill="none" class="shrink-0 text-ink-muted" aria-hidden="true"><path d="M6 2L2 8l4 6M10 2l4 6-4 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	fmt.Fprintf(&b, `<code class="font-mono text-sm text-ink">%s</code>`, esc(tc.Name))
	status := tc.Status
	if status == "" {
		status = ToolPending
	}
	b.WriteString(callout.Badge(string(status), toolStatusRole(status)))
	if tc.Summary != "" {
		fmt.Fprintf(&b, `<span class="text-sm text-ink-muted">%s</span>`, esc(tc.Summary))
	}
	if tc.Duration != "" {
		fmt.Fprintf(&b, `<span class="ml-auto text-xs text-ink-faint tabular-nums">%s</span>`, esc(tc.Duration))
	}
	b.WriteString(`</div>`)

	// Collapsible args + result ("under the hood" detail), only when present.
	if tc.Args != "" || tc.Result != "" {
		b.WriteString(`<details class="border-t border-line"`)
		if tc.Open {
			b.WriteString(` open`)
		}
		b.WriteString(`>`)
		b.WriteString(`<summary class="cursor-pointer px-3 py-2 text-xs font-medium text-ink-muted focus:outline-none focus:ring-2 focus:ring-focus">Arguments &amp; result</summary>`)
		b.WriteString(`<div class="flex flex-col gap-2 px-3 pb-3">`)
		if tc.Args != "" {
			b.WriteString(toolBlock("arguments", tc.Args))
		}
		if tc.Result != "" {
			b.WriteString(toolBlock("result", tc.Result))
		}
		b.WriteString(`</div></details>`)
	}

	b.WriteString(`</div>`)
	return b.String()
}

func toolBlock(label, body string) string {
	return fmt.Sprintf(
		`<div><div class="text-xs uppercase tracking-wide text-ink-faint">%s</div><pre class="mt-1 overflow-x-auto rounded bg-surface-sunken p-2 text-xs text-ink"><code class="font-mono">%s</code></pre></div>`,
		esc(label), esc(body),
	)
}

package hitl

import (
	"fmt"
	"strings"
)

// TraceState is the status of one step in an agent run.
type TraceState string

const (
	TraceDone    TraceState = "done"
	TraceActive  TraceState = "active"
	TracePending TraceState = ""
	TraceFailed  TraceState = "failed"
)

// TraceStep is one event in a run trace.
type TraceStep struct {
	Label  string
	Detail string // optional secondary line (escaped)
	At     string // optional pre-formatted timestamp (escaped)
	State  TraceState
}

func traceDotRole(s TraceState) string {
	switch s {
	case TraceDone:
		return "positive"
	case TraceActive:
		return "accent"
	case TraceFailed:
		return "danger"
	default:
		return "line"
	}
}

// RunTrace renders a vertical event log of an agent run — the "what's happening
// under the hood" view: a status dot per step (done/active/failed/pending) with
// a connecting rail, label, optional detail, and timestamp. Deterministic;
// escaped; themed by role.
func RunTrace(steps []TraceStep) string {
	var b strings.Builder
	b.WriteString(`<ol role="list" aria-label="Run trace" class="text-sm">`)
	for i, s := range steps {
		last := i == len(steps)-1
		dot := traceDotRole(s.State)
		// Each row: a rail column (dot + connector) and the content.
		b.WriteString(`<li class="flex gap-3">`)
		b.WriteString(`<div class="flex flex-col items-center">`)
		// Dot — filled for done/active/failed, outline for pending.
		if s.State == TracePending {
			fmt.Fprintf(&b, `<span class="size-3 shrink-0 rounded-full border-2 border-%s bg-surface"></span>`, dot)
		} else {
			fmt.Fprintf(&b, `<span class="size-3 shrink-0 rounded-full bg-%s"></span>`, dot)
		}
		// Connector rail to the next step.
		if !last {
			b.WriteString(`<span class="w-px grow bg-line" style="min-height:1.25rem"></span>`)
		}
		b.WriteString(`</div>`)
		// Content.
		b.WriteString(`<div class="min-w-0 pb-4">`)
		b.WriteString(`<div class="flex items-baseline gap-2">`)
		fmt.Fprintf(&b, `<span class="font-medium text-ink">%s</span>`, esc(s.Label))
		if s.At != "" {
			fmt.Fprintf(&b, `<span class="text-xs text-ink-faint">%s</span>`, esc(s.At))
		}
		if s.State == TraceFailed {
			b.WriteString(`<span class="text-xs font-medium text-danger">failed</span>`)
		}
		b.WriteString(`</div>`)
		if s.Detail != "" {
			fmt.Fprintf(&b, `<p class="mt-0.5 text-ink-muted">%s</p>`, esc(s.Detail))
		}
		b.WriteString(`</div></li>`)
	}
	b.WriteString(`</ol>`)
	return b.String()
}

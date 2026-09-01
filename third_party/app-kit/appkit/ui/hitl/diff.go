package hitl

// diff.go — before/after review for human-in-the-loop change approval: show what
// an agent proposes to change, field by field, so a person can approve or reject
// it (pair with ApprovalCard). Deterministic, token-themed, escaped, no JS.

import (
	"fmt"
	"strings"
)

// Change is one proposed before→after edit in a DiffView.
type Change struct {
	// Label names what is changing (e.g. "Status", "Assignee", "Deposit amount").
	Label string
	// Before is the current value (rendered removed/danger-tinted). Empty Before
	// marks an addition.
	Before string
	// After is the proposed value (rendered added/positive-tinted). Empty After
	// marks a removal.
	After string
}

// DiffView renders proposed changes as aligned before→after rows — the "review
// what the agent wants to change" surface. Each row shows the current value
// (danger-tinted) and the proposed value (positive-tinted); a missing side reads
// as an addition or removal. Caller-supplied values are plain text (escaped).
func DiffView(changes []Change) string {
	var b strings.Builder
	b.WriteString(`<div class="flex flex-col gap-3" role="group" aria-label="Proposed changes">`)
	for _, c := range changes {
		b.WriteString(`<div class="flex flex-col gap-1">`)
		if c.Label != "" {
			fmt.Fprintf(&b, `<span class="text-xs font-medium uppercase tracking-wide text-ink-muted">%s</span>`, esc(c.Label))
		}
		b.WriteString(`<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">`)
		b.WriteString(diffCell(c.Before, "before"))
		b.WriteString(`<span class="text-ink-faint" aria-hidden="true">→</span>`)
		b.WriteString(diffCell(c.After, "after"))
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// diffCell renders one side of a change row. side is "before" (danger) or
// "after" (positive); an empty value renders a muted em-dash placeholder.
func diffCell(value, side string) string {
	if value == "" {
		return `<span class="rounded-md border border-line bg-surface-sunken px-2 py-1 text-sm text-ink-faint">—</span>`
	}
	cls := "border-positive bg-positive-soft"
	prefix := "+ "
	label := "proposed"
	if side == "before" {
		cls = "border-danger bg-danger-soft"
		prefix = "− "
		label = "current"
	}
	return fmt.Sprintf(
		`<span class="rounded-md border %s px-2 py-1 text-sm text-ink"><span class="sr-only">%s: </span><span class="text-ink-faint" aria-hidden="true">%s</span>%s</span>`,
		cls, label, prefix, esc(value),
	)
}

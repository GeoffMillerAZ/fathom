package main

// console_sessionfeed.go — PE-19 (docs/design/progress-and-economics.md
// §5, "Views"): the live session feed. Read-only over the SSE channel
// (ADR-045) the way every other console page follows a live source
// (console_live.go, OC-14) — a page declaring Live: []string{"sessions"}
// (already published by console_events.go's console.EventSession) gets
// its "Updated — refresh this panel" notice for free; renderSessionFeed
// itself fetches nothing and starts no connection, it only renders rows
// its caller already read from the session ledger (cmd/fathom) and
// joined to their proof outcome (console.JoinSessionProof, PE-15).
//
// No visible cell here is a session's own id or any other per-person
// figure (I58): a row's RowID is the table's DOM anchor only.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
)

// sessionFeedRow is one session in the feed. It carries no session id or
// any other per-person figure (I58) — not even as a caller field — so
// there is nothing of that kind for a render bug to ever leak; the DOM
// row id renderSessionFeed assigns is purely positional.
type sessionFeedRow struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
	Repo    string `json:"repo"`
	Start   string `json:"start"` // RFC 3339
	End     string `json:"end"`   // RFC 3339, or "" while the session is still running
	// Proven is the count of rows JoinSessionProof attributed to this
	// session; absent (not zero) when the join has not run or found
	// nothing to attribute yet.
	Proven console.Value[int] `json:"proven"`
}

// renderSessionFeed renders rows as the feed table, newest first as the
// caller ordered them. A caller with nothing to show renders the named
// empty state, never an empty table.
func renderSessionFeed(rows []sessionFeedRow) string {
	if len(rows) == 0 {
		return `<p class="text-ink-muted" data-session-feed-empty>No session in the ledger's window yet.</p>`
	}
	trs := make([]table.Row, 0, len(rows))
	for i, r := range rows {
		harness := r.Harness
		if harness == "" {
			harness = "harness not recorded"
		}
		model := r.Model
		if model == "" {
			model = console.ModelNotRecorded
		}
		window := r.Start + " – " + r.End
		if r.End == "" {
			window = r.Start + " – in progress"
		}
		proven := "—"
		switch r.Proven.State {
		case console.StatePresent, console.StateZero:
			proven = fmt.Sprint(r.Proven.V)
		}
		// The DOM row id is purely positional — never a session id (I58).
		trs = append(trs, table.Row{ID: fmt.Sprintf("session-feed-%d", i), Cells: []string{harness, model, r.Repo, window, proven}})
	}
	return renderKitTable(table.TableParams{
		TableID: "session-feed", Caption: "Sessions in the ledger's window, read-only — updated over the SSE channel",
		Columns: []table.Column{
			{Key: "harness", Label: "Harness"}, {Key: "model", Label: "Model"}, {Key: "repo", Label: "Repository"},
			{Key: "window", Label: "Window"}, {Key: "proven", Label: "Rows proven", Align: "right"},
		},
		Rows: trs,
	})
}

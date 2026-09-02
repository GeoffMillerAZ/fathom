package main

// thread_json.go — `fathom thread list --json` and `thread brief --json`
// (RP-16, docs/plans/reach-parity.md). The list envelope carries exactly
// the items the text path would print (same scope, same --all/--all-repos
// rules) plus the count of items hidden in other repos; the brief envelope
// carries the rendered text with its byte size against the budget, so a
// harness that injects it can measure it.

import (
	"encoding/json"
	"io"
)

type threadListItemJSON struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"` // open | dormant | closed(<by>)
	Kind      string            `json:"kind"`
	Thread    string            `json:"thread"`
	Text      string            `json:"text"`
	Repo      string            `json:"repo,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	FiledAt   string            `json:"filed_at,omitempty"`
	ClosedAt  string            `json:"closed_at,omitempty"`
	ClosedBy  string            `json:"closed_by,omitempty"`
	Commit    string            `json:"commit,omitempty"`
	Pointers  map[string]string `json:"pointers,omitempty"`
}

type threadListJSON struct {
	Kind             string               `json:"kind"` // "list"
	Items            []threadListItemJSON `json:"items"`
	HiddenOtherRepos int                  `json:"hidden_other_repos"`
}

type threadBriefJSON struct {
	Kind        string `json:"kind"` // "brief"
	Source      string `json:"source"`
	BudgetBytes int    `json:"budget_bytes"`
	Bytes       int    `json:"bytes"`
	// Text is the exact brief the SessionStart hook would inject; "" on
	// resume/compact (context survives) or when there is nothing to say.
	Text string `json:"text"`
}

// threadListItemOf renders one thread item for the list envelope.
func threadListItemOf(it threadItem, status string) threadListItemJSON {
	return threadListItemJSON{
		ID: it.ID, Status: status, Kind: it.Kind, Thread: it.Thread, Text: it.Text,
		Repo: it.Pointers["repo"], SessionID: it.Pointers["session_id"],
		FiledAt: it.FiledAt, ClosedAt: it.ClosedAt, ClosedBy: it.ClosedBy, Commit: it.Commit,
		Pointers: it.Pointers,
	}
}

// renderThreadJSON writes one envelope (a threadListJSON or a
// threadBriefJSON) as indented JSON.
func renderThreadJSON(w io.Writer, envelope any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

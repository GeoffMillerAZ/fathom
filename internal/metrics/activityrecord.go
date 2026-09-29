package metrics

// activityrecord.go — SR-67 (docs/design/surface-reckoning.md §6, design
// doc §6's "call log + hook log" row): the shared reader that joins the
// MCP call log and the hooks beat log (the "hook log": ~/.fathom/
// hooks.jsonl, cmd/fathom/beats.go's emitBeat) into one per-session
// record. docs/design/work-intelligence.md §1 names both logs among the
// existing agent-activity capture the U1-U5 views join to proof; this row
// lands the join itself, not those consumer views (a later row, in the
// work-intelligence plan, wires them) — design §6's own words: "this row
// lands the shared reader over both logs."
//
// Pure aggregation, like every other file in this package (traffic.go,
// interventions.go): callers decode both logs (cmd/fathom already does —
// readCallLog/adoptionCallEntry for the call log, heartbeat.Load for the
// hooks beat log) and pass the decoded rows in; this file does no I/O,
// per internal/AGENTS.md's determinism contract.

import (
	"sort"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// ActivityCallEntry is the minimal call-log shape ReadActivityRecord
// needs, decoupled from cmd/fathom's adoptionCallEntry (which cannot be
// imported here; this package cannot import package main) the same way
// CallEntry (traffic.go) is decoupled — with the one field CallEntry
// omits: Session (the row's caller.session, internal/mcp's
// callLogCaller.Session) — the join key this file exists to use.
type ActivityCallEntry struct {
	TS      string
	Tool    string
	Session string
}

// ActivityRecord is one session's joined signal from both logs: how many
// fathom tool calls it made (by tool) and how many hook events fired for
// it (by "<channel>/<kind>", heartbeat.Beat's own two fields). A
// per-session TOTAL, never a per-row dump — a session can span thousands
// of rows in either log, and I58 (no ranking by person) means a session
// id is an agent-run identity here, never attributed to a person.
type ActivityRecord struct {
	Session string `json:"session"`
	// Calls is call-log rows attributed to Session, keyed by tool name.
	Calls      map[string]int `json:"calls,omitempty"`
	CallsTotal int            `json:"calls_total"`
	// HookEvents is hook-log rows attributed to Session, keyed by
	// "<channel>/<kind>" (e.g. "edit-hook/answered").
	HookEvents      map[string]int `json:"hook_events,omitempty"`
	HookEventsTotal int            `json:"hook_events_total"`
	// First/Last is the earliest and latest timestamp either log
	// contributed for this session, RFC3339 UTC (string comparison is
	// exact for RFC3339 timestamps sharing a zone, and every writer here
	// stamps UTC).
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

// ReadActivityRecord joins calls (already-decoded call-log rows) and
// beats (already-decoded hooks-beat-log rows) into one ActivityRecord per
// distinct session id, sorted by session id for a deterministic order
// (internal/AGENTS.md's determinism contract — every aggregator in this
// package returns a stable order, never map iteration order). A row with
// no session id (a call or beat logged outside any live session — a CLI
// verb run standalone, a gate step) is grouped under the empty-string
// session, sorted LAST (real sessions read first), never dropped and
// never merged into a real session's counts.
func ReadActivityRecord(calls []ActivityCallEntry, beats []heartbeat.Beat) []ActivityRecord {
	by := map[string]*ActivityRecord{}
	var order []string
	get := func(session string) *ActivityRecord {
		r, ok := by[session]
		if !ok {
			r = &ActivityRecord{Session: session}
			by[session] = r
			order = append(order, session)
		}
		return r
	}
	touch := func(r *ActivityRecord, ts string) {
		if ts == "" {
			return
		}
		if r.First == "" || ts < r.First {
			r.First = ts
		}
		if ts > r.Last {
			r.Last = ts
		}
	}

	for _, c := range calls {
		r := get(c.Session)
		if r.Calls == nil {
			r.Calls = map[string]int{}
		}
		r.Calls[c.Tool]++
		r.CallsTotal++
		touch(r, c.TS)
	}
	for _, b := range beats {
		session := ""
		if b.Meta != nil {
			session = b.Meta["session"]
		}
		r := get(session)
		if r.HookEvents == nil {
			r.HookEvents = map[string]int{}
		}
		r.HookEvents[b.Channel+"/"+b.Kind]++
		r.HookEventsTotal++
		if !b.At.IsZero() {
			touch(r, b.At.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"))
		}
	}

	sort.Slice(order, func(i, j int) bool {
		a, bb := order[i], order[j]
		if (a == "") != (bb == "") {
			return bb == "" // "" always sorts last
		}
		return a < bb
	})
	out := make([]ActivityRecord, len(order))
	for i, session := range order {
		out[i] = *by[session]
	}
	return out
}

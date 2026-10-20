package main

// claimcheck_runs.go — the one reader RO-39's claim check and RO-40's gate
// preflight share (docs/plans/readouts.md; design docs/design/readouts.md
// section 20, H2 and H6): the session's test-run rows (TR-7, with RO-35's
// `head` and `tree` and the `agent_id` key) from the hook log's last
// 256 KiB, through the Stop hook's own bounded tail reader
// (readBeatLogTail). Local reads only: no daemon call, no git.

import (
	"sort"
	"time"
)

// sessionRunsTailBytes is how much of the user-global hook log the readers
// parse: about 1.5 hours of the log (design section 20, H2).
const sessionRunsTailBytes = 256 << 10

// sessionRunRow is one test-run row, as logged.
type sessionRunRow struct {
	At         time.Time
	Invocation string
	Result     string // pass | fail | unknown
	Failing    string // the logged failing ids, comma-joined (at most 10)
	Head, Tree string // RO-35's keys; "" when the row carries none
	Scope      string // RO-35's lexed scope; "" when the row carries none
	AgentID    string // the subagent that ran it; "" for the main agent
}

// sessionRuns is what the tail says about one session.
type sessionRuns struct {
	// Read is false when the log could not be read at all.
	Read bool
	// Rows are the session's test-run rows in log order.
	Rows []sessionRunRow
	// Oldest is the tail's oldest row of any channel; zero for an empty tail.
	Oldest time.Time
	// LastEdit is the session's last edit in the tail (the edit hook's
	// post-edit beat); zero when the tail holds none.
	LastEdit time.Time
}

// readSessionRuns reads the session's rows from the hook log at path.
func readSessionRuns(path, session string) sessionRuns {
	beats, ok := readBeatLogTail(path, sessionRunsTailBytes)
	if !ok {
		return sessionRuns{}
	}
	out := sessionRuns{Read: true}
	for _, b := range beats {
		if out.Oldest.IsZero() || b.At.Before(out.Oldest) {
			out.Oldest = b.At
		}
		if session == "" || b.Meta["session"] != session {
			continue
		}
		switch {
		case b.Channel == testRunChannel:
			out.Rows = append(out.Rows, sessionRunRow{
				At: b.At, Invocation: b.Meta["invocation"], Result: b.Meta["result"], Failing: b.Meta["failing"],
				Head: b.Meta["head"], Tree: b.Meta["tree"], Scope: b.Meta["scope"], AgentID: b.Meta["agent_id"],
			})
		case b.Channel == "edit-hook" && b.Kind == "seen" && b.Meta["phase"] == "post":
			if b.At.After(out.LastEdit) {
				out.LastEdit = b.At
			}
		}
	}
	sort.SliceStable(out.Rows, func(i, j int) bool { return out.Rows[i].At.Before(out.Rows[j].At) })
	return out
}

// agentRows keeps only the rows agentID ran.
func (s sessionRuns) agentRows(agentID string) sessionRuns {
	kept := s
	kept.Rows = nil
	for _, r := range s.Rows {
		if r.AgentID == agentID {
			kept.Rows = append(kept.Rows, r)
		}
	}
	return kept
}

// covers reports whether the tail reaches back far enough that a missing
// run is a fact, not a gap (I38): it holds the session's last edit, or,
// when the tail holds no edit of the session's, its oldest row is older
// than the session's start (zero when unknown, which never covers).
func (s sessionRuns) covers(start time.Time) bool {
	if !s.Read || s.Oldest.IsZero() {
		return false
	}
	if !s.LastEdit.IsZero() {
		return true
	}
	return !start.IsZero() && s.Oldest.Before(start)
}

// passingAt reports whether a row in s passed at exactly this head and
// tree (both known).
func (s sessionRuns) passingAt(head, tree string) bool {
	if head == "" || tree == "" {
		return false
	}
	for _, r := range s.Rows {
		if r.Result == "pass" && r.Head == head && r.Tree == tree {
			return true
		}
	}
	return false
}

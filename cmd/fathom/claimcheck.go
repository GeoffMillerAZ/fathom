package main

// claimcheck.go — RO-39 (docs/plans/readouts.md; design
// docs/design/readouts.md section 20, H2): the claim check at Stop and
// SubagentStop. For each test or gate claim RO-37's closed table finds in
// the agent's final message, the session's recorded test runs (TR-7 rows
// with RO-35's head and tree; on SubagentStop the subagent's own, by
// agent_id) decide: a claim whose last matching run failed or read unknown,
// or with no matching run since the session's last edit, gets one line
// naming that run; a claim a passing run at the current head and tree backs
// is silent. Advisory context only, never a block.
//
// Local reads only (the hook log's last 256 KiB, the message the input
// carries, and git for the current tree), no daemon call, inside a 200 ms
// budget. Every miss is silent and logged `not checked (<why>)`: a tail
// that does not reach the session's last edit (or its start) is
// `not checked (window)`, never a line saying no run backs the claim (I38).
// RO-V1 found Stop's and SubagentStop's context re-open the turn, so the
// check prints at most once per stop chain: a re-entrant stop is logged
// `not checked (repeat)` and prints nothing.
//
// One hook-log row per check on channel claim-check: the claims by table
// id, the rows cited, the verdicts. Never the message's text.

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
)

// claimCheckChannel is the hook-log channel of the check's one row.
const claimCheckChannel = "claim-check"

// claimCheckBudget bounds the check inside the hook's own deadline: Stop
// already runs at 7.1 s of its 8 s (PR-D2).
var claimCheckBudget = 200 * time.Millisecond

// subagentStopMarker is the SubagentStop row's marker, which names its own
// 2 s descriptor (a 1 s budget under I53) to the hook guard.
const subagentStopMarker = "fathom stop-hook --event subagent-stop"

// claimCheckInput is what one stop event hands the check.
type claimCheckInput struct {
	Root, Session, AgentID, Message string
	// Subagent: a SubagentStop, so only the subagent's own rows count.
	Subagent bool
	// ReEntrant: the stop already continued the turn once (stop_hook_active).
	ReEntrant bool
}

// claimCheckInputFor reads the check's input from a decoded stop event.
func claimCheckInputFor(root string, in harness.HookInput, subagent bool) claimCheckInput {
	return claimCheckInput{Root: root, Session: in.SessionID, AgentID: in.AgentID, Message: in.LastMessage, Subagent: subagent, ReEntrant: in.ReEntrant}
}

// claimCheck is one check's state: a verdict, a cited row and at most one
// line per claim.
type claimCheck struct {
	in       claimCheckInput
	claims   []readout.Claim
	verdicts []string
	cited    []string
	lines    []string
}

// checkStopClaims runs the check and returns the advisory text ("" for
// silence). It does nothing at all, and logs nothing, while [hooks]
// claim_check is off or the message claims nothing.
func checkStopClaims(in claimCheckInput) string {
	if strings.TrimSpace(in.Message) == "" {
		return ""
	}
	cfg, ok := hooksConfigAt(in.Root, hookOptInClaimCheck)
	if !ok || !cfg.Hooks.ClaimCheck {
		return ""
	}
	claims := readout.ClaimsInMessage(in.Message, func(command string) (string, bool) {
		m, ok := matchDeclaredTestCommand(in.Root, command, cfg.Tests.Commands)
		return m.Invocation, ok
	})
	if len(claims) == 0 {
		return ""
	}
	c := &claimCheck{in: in, claims: claims, verdicts: make([]string, len(claims)), cited: make([]string, len(claims))}
	switch {
	case in.ReEntrant:
		c.unchecked("repeat")
	case !cfg.Hooks.TestRun:
		c.unchecked("test_run off") // no rows are written, so none can back or refute a claim
	case in.Subagent && in.AgentID == "":
		c.unchecked("agent rows")
	default:
		c.run()
	}
	c.log()
	return strings.Join(c.lines, "\n")
}

// unchecked marks every claim not checked for why, and prints nothing.
func (c *claimCheck) unchecked(why string) {
	for i := range c.claims {
		c.verdicts[i] = "not checked (" + why + ")"
		if c.cited[i] == "" {
			c.cited[i] = "-"
		}
	}
	c.lines = nil
}

// claimCheckContext is the check's budget, never past the hook guard's.
func claimCheckContext() (context.Context, context.CancelFunc) {
	budget := claimCheckBudget
	if g := activeHookGuard(); g != nil && !g.Deadline.IsZero() {
		if r := g.Deadline.Remaining(); r < budget {
			budget = r
		}
	}
	return context.WithTimeout(context.Background(), budget)
}

func (c *claimCheck) run() {
	ctx, cancel := claimCheckContext()
	defer cancel()
	runs := readSessionRuns(beatLogPath(), c.in.Session)
	if !runs.Read {
		c.unchecked("log")
		return
	}
	if c.in.Subagent {
		runs = runs.agentRows(c.in.AgentID)
	}
	var start time.Time
	startRead := false
	var cur testRunRecord
	curRead := false
	seen := map[string]bool{}
	for i, cl := range c.claims {
		c.cited[i] = "-"
		rows := matchingRuns(runs.Rows, cl.Invocation)
		if len(rows) == 0 {
			if !startRead && runs.LastEdit.IsZero() {
				start, _ = sessionStartFor(c.in.Root, c.in.Session)
				startRead = true
			}
			if !runs.covers(start) {
				c.verdicts[i] = "not checked (window)"
				continue
			}
			c.verdicts[i] = "unbacked"
			c.addLine(seen, fmt.Sprintf("the final message says %s, but no %s run is recorded %s since its last edit.", claimPhrase(cl), claimClass(cl), c.scope()))
			continue
		}
		last := rows[len(rows)-1]
		c.cited[i] = last.At.UTC().Format(time.RFC3339Nano)
		switch last.Result {
		case "pass":
			if last.Head == "" || last.Tree == "" {
				c.verdicts[i] = "not checked (no tree)"
				continue
			}
			if !curRead {
				fillTestRunRowKeys(ctx, &cur, c.in.Root, c.in.Session, "", "")
				curRead = true
			}
			if cur.Head == "" || cur.Tree == "" {
				c.verdicts[i] = "not checked (tree)"
				continue
			}
			if last.Head == cur.Head && last.Tree == cur.Tree {
				c.verdicts[i] = "backed"
				continue
			}
			c.verdicts[i] = "stale"
			c.addLine(seen, fmt.Sprintf("the final message says %s, but the last passing %s run %s was at %s before later changes, and none has run at the current tree.", claimPhrase(cl), claimClass(cl), c.scope(), shortHead(last.Head)))
		case "fail":
			c.verdicts[i] = "failed"
			failing := ""
			if last.Failing != "" {
				failing = " (failing: " + strings.ReplaceAll(last.Failing, ",", ", ") + ")"
			}
			c.addLine(seen, fmt.Sprintf("the final message says %s, but the last %s run %s failed at %s%s.", claimPhrase(cl), claimClass(cl), c.scope(), shortHead(last.Head), failing))
		default:
			c.verdicts[i] = "unknown"
			c.addLine(seen, fmt.Sprintf("the final message says %s, but the last %s run %s read no result (its output was cut or piped) at %s.", claimPhrase(cl), claimClass(cl), c.scope(), shortHead(last.Head)))
		}
	}
	if deadlinePassed(ctx) {
		// A check that ran out of time says nothing: a late answer could
		// be a partial one.
		for i, v := range c.verdicts {
			if v == "" || !strings.HasPrefix(v, "not checked") {
				c.verdicts[i] = "not checked (deadline)"
			}
		}
		c.lines = nil
	}
}

func (c *claimCheck) addLine(seen map[string]bool, line string) {
	line = "🧪 fathom claim check: " + line
	if seen[line] {
		return
	}
	seen[line] = true
	c.lines = append(c.lines, line)
}

func (c *claimCheck) scope() string {
	if c.in.Subagent {
		return "by this subagent"
	}
	return "in this session"
}

// log writes the check's one row: claims by table id, the rows cited (by
// their time), the verdicts. Never the message's text.
func (c *claimCheck) log() {
	ids := make([]string, len(c.claims))
	for i, cl := range c.claims {
		ids[i] = cl.ID
	}
	event := "stop"
	if c.in.Subagent {
		event = "subagent-stop"
	}
	meta := map[string]string{
		"event":    event,
		"claims":   strings.Join(ids, ","),
		"rows":     strings.Join(c.cited, ","),
		"verdicts": strings.Join(c.verdicts, ","),
		"lines":    strconv.Itoa(len(c.lines)),
	}
	if c.in.AgentID != "" {
		meta["agent_id"] = c.in.AgentID
	}
	kind := "silent"
	if len(c.lines) > 0 {
		kind = "flagged"
	}
	emitBeat(claimCheckChannel, kind, firingMeta(meta, c.in.Session, ""))
}

// matchingRuns keeps the rows of the command class: the named declared
// command, or any declared test command when the claim names none.
func matchingRuns(rows []sessionRunRow, invocation string) []sessionRunRow {
	if invocation == "" {
		return rows
	}
	var out []sessionRunRow
	for _, r := range rows {
		if r.Invocation == invocation {
			out = append(out, r)
		}
	}
	return out
}

// claimPhrase names a claim in a line by its table entry, never by the
// message's words.
func claimPhrase(c readout.Claim) string {
	switch c.ID {
	case "all-tests-pass":
		return "all tests pass"
	case "gate-passes":
		return "the gate passes"
	case "command-passes":
		return "`" + c.Invocation + "` passes"
	case "green":
		return "everything is green"
	}
	return "the tests pass"
}

// claimClass names the command class a claim maps to.
func claimClass(c readout.Claim) string {
	if c.Invocation != "" {
		return "`" + c.Invocation + "`"
	}
	return "test"
}

// shortHead is a commit as a line names it.
func shortHead(head string) string {
	switch {
	case head == "":
		return "an unrecorded commit"
	case len(head) > 7:
		return head[:7]
	}
	return head
}

// cmdSubagentStopHook is `stop-hook --event subagent-stop` (RO-38): the
// claim check alone, under the row's own 2 s descriptor (a 1 s budget), for the
// subagent's own rows. Any error is silence.
func cmdSubagentStopHook(codec harness.Codec, url string, stdin io.Reader, stdout io.Writer) int {
	if armOffActive() {
		return 0 // the control arm measures and never assists
	}
	endGuard, silent := beginHookGuard("subagent-stop-hook", subagentStopMarker, codec.ID(), url)
	if silent {
		return 0
	}
	defer endGuard()
	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(harness.EventSubagentStop, raw)
	if err != nil {
		return 0
	}
	rpcSession = in.SessionID
	text := checkStopClaims(claimCheckInputFor(subagentStopProjDir(codec, in), in, true))
	if text == "" {
		return 0
	}
	out, err := codec.EncodeOutput(harness.EventSubagentStop, harness.HookOutput{Context: text})
	if err != nil || len(out) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

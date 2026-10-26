package labs

import (
	"github.com/geoffmilleraz/fathom/internal/research/bundle"
	"github.com/geoffmilleraz/fathom/internal/transcript"
)

const (
	// pointPreceding is how many earlier calls a decision point carries as
	// its context, matching readout replay's five-call follower window.
	pointPreceding = 5
	// pointFollowers is the next-three-calls window section 4.4 reads.
	pointFollowers = 3
)

// CallResult is the decoded tool_result joined to its call: the error flag
// and byte count alone, never output text. Present separates "no result in
// this transcript" from a zero-byte one.
type CallResult struct {
	Present bool `json:"present"`
	IsError bool `json:"is_error"`
	Bytes   int  `json:"bytes"`
}

// Call is one choice the agent made, as the event projection retains it:
// a tool name or skill key and the JSON shape class of its input. No
// argument text exists in the projection, so none can be recorded (I61).
type Call struct {
	// Kind is the event kind the call came from: tool_use, skill or
	// subagent_start.
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	Shape     string     `json:"shape"`
	ToolUseID string     `json:"tool_use_id"`
	Result    CallResult `json:"result"`
}

// Point is one decision point: a call, the calls before it and the next
// three after it, in a hashed session. Transcripts carry no user-turn
// event, so the ordinal is the call's index in event order.
type Point struct {
	Session   string `json:"session"`
	Ordinal   int    `json:"ordinal"`
	Preceding []Call `json:"preceding,omitempty"`
	Call      Call   `json:"call"`
	Followers []Call `json:"followers,omitempty"`
}

// Counts carries the decoder's coverage receipt through extraction so a
// reading can never silently rest on lines nobody could read (I38).
type Counts struct {
	Points          int                `json:"points"`
	Issues          []transcript.Issue `json:"issues,omitempty"`
	MalformedLines  int                `json:"malformed_lines"`
	OversizeLines   int                `json:"oversize_lines"`
	UnreadableFiles int                `json:"unreadable_files"`
	Duplicates      int                `json:"duplicates"`
}

// ExtractDecisionPoints cuts one decoded transcript into decision points.
// salt must be the salt from bundle.LoadOrCreateSalt and sessionID the
// transcript's session identity: the point carries only the salted hash
// (bundle.HashService), never the identifier.
//
// The decoder emits a Skill tool_use beside its skill event, and a Task or
// Agent tool_use beside the subagent_start it opens. Those are one choice,
// so the resolved event replaces its paired tool_use and the pair is
// counted once. A boundary event with no call behind it (a system
// subagent_start line) is no decision point.
func ExtractDecisionPoints(res transcript.Result, salt, sessionID string) ([]Point, Counts) {
	counts := Counts{
		Issues:          res.Issues,
		MalformedLines:  res.MalformedLines,
		OversizeLines:   res.OversizeLines,
		UnreadableFiles: res.UnreadableFiles,
		Duplicates:      res.Duplicates,
	}
	results := map[string]CallResult{}
	for _, e := range res.Events {
		if e.Kind == "tool_result" && e.ToolUseID != "" {
			results[e.ToolUseID] = CallResult{Present: true, IsError: e.IsError, Bytes: e.Bytes}
		}
	}
	var calls []Call
	for _, e := range res.Events {
		switch e.Kind {
		case "tool_use":
			calls = append(calls, Call{Kind: e.Kind, Name: e.Name, Shape: e.Shape, ToolUseID: e.ToolUseID})
		case "skill", "subagent_start":
			if e.Name == "" {
				continue
			}
			call := Call{Kind: e.Kind, Name: e.Name, Shape: e.Shape, ToolUseID: e.ToolUseID}
			if n := len(calls); n > 0 && pairedWith(calls[n-1], e) {
				calls[n-1] = call
				continue
			}
			calls = append(calls, call)
		}
	}
	for i := range calls {
		calls[i].Result = results[calls[i].ToolUseID]
	}
	points := make([]Point, 0, len(calls))
	session := ""
	if salt != "" && sessionID != "" {
		session = bundle.HashService(salt, sessionID)
	}
	for i, call := range calls {
		points = append(points, Point{
			Session:   session,
			Ordinal:   i,
			Preceding: window(calls, max(0, i-pointPreceding), i),
			Call:      call,
			Followers: window(calls, i+1, min(len(calls), i+1+pointFollowers)),
		})
	}
	counts.Points = len(points)
	return points, counts
}

// pairedWith reports whether the resolved event restates the tool_use just
// recorded: the launcher names Skill, Task or Agent and the pair shares a
// tool-use id (an empty id on both sides is the same block's pair, since
// the decoder emits them back to back).
func pairedWith(prev Call, e transcript.Event) bool {
	if prev.Kind != "tool_use" || prev.ToolUseID != e.ToolUseID {
		return false
	}
	switch prev.Name {
	case "Skill", "Task", "Agent":
		return true
	}
	return false
}

// window copies a half-open range of calls with their results attached.
func window(calls []Call, from, to int) []Call {
	if from >= to {
		return nil
	}
	out := make([]Call, 0, to-from)
	out = append(out, calls[from:to]...)
	return out
}

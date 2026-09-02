package agentharness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Handoff is the receipt of ONE run that stopped before its task did — on
// StopBudget, StopMaxTokens or StopMaxTurns — written for whoever picks the
// work up next: a human, the next run, another agent. It says what was
// tried, what ran last, and which checkpoint to resume from, in a typed
// record instead of a stop cause the next reader has to reverse-engineer
// from a transcript.
//
// A Handoff is derived entirely from the RunResult (HandoffFor is pure) and
// the harness stores nothing: it is returned on RunResult.Handoff and
// emitted once on the run's event sink. It is NOT a memory — no run reads a
// previous run's Handoff unless its caller passes it in (futures F2: the
// kit keeps no cross-run store).
//
// CUE mirror: #Handoff in schema.cue.
type Handoff struct {
	// StopCause is why the run stopped: budget, max_tokens or max_turns.
	StopCause StopCause `json:"stop_cause"`

	// Turns is the number of model calls that completed.
	Turns int `json:"turns"`

	// LastTool is the name of the last tool the model called, or empty when
	// no tool was called.
	LastTool string `json:"last_tool,omitempty"`

	// CheckpointRef identifies the checkpoint to resume from —
	// CheckpointRef(RunResult.Final). A consumer that persists the
	// Checkpoint can match it back by recomputing the ref.
	CheckpointRef string `json:"checkpoint_ref,omitempty"`

	// Tried lists the distinct tool names the model called, in first-call
	// order.
	Tried []string `json:"tried,omitempty"`

	// Summary is one human-readable line carrying all of the above.
	Summary string `json:"summary"`
}

// handoffStops are the stop causes on which the harness emits a Handoff.
func handoffStop(c StopCause) bool {
	return c == StopBudget || c == StopMaxTokens || c == StopMaxTurns
}

// HandoffFor derives the Handoff of result. It is pure and deterministic:
// the same RunResult yields the same Handoff. It answers for ANY stop cause
// so a consumer may build one for its own reasons; the harness itself
// attaches one to RunResult.Handoff only on StopBudget, StopMaxTokens and
// StopMaxTurns.
func HandoffFor(result RunResult) Handoff {
	var (
		tried []string
		last  string
		seen  = map[string]bool{}
	)
	for _, m := range result.Messages {
		if m.Role != llmclient.RoleAssistant {
			continue
		}
		for _, b := range m.Content {
			if b.Type != llmclient.BlockToolUse {
				continue
			}
			last = b.Name
			if !seen[b.Name] {
				seen[b.Name] = true
				tried = append(tried, b.Name)
			}
		}
	}

	h := Handoff{
		StopCause:     result.Stop,
		Turns:         result.Turns,
		LastTool:      last,
		CheckpointRef: CheckpointRef(result.Final),
		Tried:         tried,
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "run stopped on %s after %d turn(s)", h.StopCause, h.Turns)
	if h.LastTool == "" {
		sb.WriteString("; no tool was called")
	} else {
		fmt.Fprintf(&sb, "; last tool %s; tried [%s]", h.LastTool, strings.Join(h.Tried, ", "))
	}
	if h.CheckpointRef != "" {
		fmt.Fprintf(&sb, "; resume from %s", h.CheckpointRef)
	}
	h.Summary = sb.String()
	return h
}

// CheckpointRef returns a stable content reference for cp:
// "checkpoint:v<Version>:turn<TurnCursor>:<first 16 hex of
// SHA-256(ConvSnapshot)>", or "" for a nil checkpoint. Two checkpoints with
// the same conversation snapshot and cursor have the same ref; the budget
// snapshot is deliberately excluded (it is the guard's state, not the
// run's).
func CheckpointRef(cp *Checkpoint) string {
	if cp == nil {
		return ""
	}
	sum := sha256.Sum256(cp.ConvSnapshot)
	return fmt.Sprintf("checkpoint:v%d:turn%d:%s", cp.Version, cp.TurnCursor, hex.EncodeToString(sum[:8]))
}

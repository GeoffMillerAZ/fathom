// Package traceableprose provides annotations, redaction provenance tracking,
// and append-only regeneration logging for governed prose nodes.
//
// Every TraceableNode carries the evidence spans that contributed to it, a
// redaction flag with optional provenance, and a regeneration-needed marker.
// RegenerationLog records the chain of regeneration events so the full history
// of changes is preserved and auditable.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency: not safe for concurrent use; guard with your own synchronisation.
package traceableprose

import (
	"fmt"
	"time"
)

// TraceableNode is a prose node with full provenance: which evidence spans
// contributed to it, whether it has been redacted (and why), and whether it
// needs to be regenerated following a correction.
//
// CUE mirror: #TraceableNode in schema.cue.
type TraceableNode struct {
	// ID uniquely identifies this node across the document. Must be non-empty.
	ID string

	// Text is the current rendered text of this node. Must be non-empty.
	Text string

	// SourceRefs holds the identifiers of the evidence spans or citations that
	// contributed to this node's text. May be empty for non-evidentiary nodes.
	SourceRefs []string

	// Redacted indicates whether this node has been redacted (text replaced or
	// removed) under a DLP or content-policy decision.
	Redacted bool

	// RedactionReason is the human-readable provenance explaining why the node
	// was redacted. Required when Redacted is true; ignored otherwise.
	RedactionReason string

	// NeedsRegeneration is true when an upstream correction has invalidated this
	// node and it must be regenerated before the document can be published.
	NeedsRegeneration bool
}

// Validate returns an error if any field is invalid.
// Rules:
//   - ID and Text must be non-empty.
//   - When Redacted is true, RedactionReason must be non-empty.
//   - SourceRefs elements must all be non-empty when present.
func (n TraceableNode) Validate() error {
	if n.ID == "" {
		return fmt.Errorf("traceableprose: TraceableNode.ID must not be empty")
	}
	if n.Text == "" {
		return fmt.Errorf("traceableprose: TraceableNode.Text must not be empty")
	}
	if n.Redacted && n.RedactionReason == "" {
		return fmt.Errorf("traceableprose: TraceableNode.RedactionReason must not be empty when Redacted is true")
	}
	for i, ref := range n.SourceRefs {
		if ref == "" {
			return fmt.Errorf("traceableprose: TraceableNode.SourceRefs[%d] must not be empty", i)
		}
	}
	return nil
}

// MarkRedacted sets Redacted to true and records the provenance reason.
// Returns an error if reason is empty.
func (n *TraceableNode) MarkRedacted(reason string) error {
	if reason == "" {
		return fmt.Errorf("traceableprose: redaction reason must not be empty")
	}
	n.Redacted = true
	n.RedactionReason = reason
	return nil
}

// RegenerationEvent records a single regeneration of a set of prose nodes,
// triggered by a specific correction or evidence update.
//
// CUE mirror: #RegenerationEvent in schema.cue.
type RegenerationEvent struct {
	// TriggeredBy identifies the correction or evidence-update ID that caused
	// this regeneration. Must be non-empty.
	TriggeredBy string

	// AffectedNodeIDs lists the IDs of the nodes that were regenerated.
	// Must contain at least one ID.
	AffectedNodeIDs []string

	// OccurredAt is the wall-clock time at which the regeneration was recorded.
	OccurredAt time.Time

	// Note is an optional human-readable description of why this regeneration
	// was necessary.
	Note string
}

// Validate returns an error if the event is not well-formed.
// Rules:
//   - TriggeredBy must be non-empty.
//   - AffectedNodeIDs must contain at least one non-empty element.
//   - OccurredAt must not be zero.
func (e RegenerationEvent) Validate() error {
	if e.TriggeredBy == "" {
		return fmt.Errorf("traceableprose: RegenerationEvent.TriggeredBy must not be empty")
	}
	if len(e.AffectedNodeIDs) == 0 {
		return fmt.Errorf("traceableprose: RegenerationEvent.AffectedNodeIDs must not be empty")
	}
	for i, id := range e.AffectedNodeIDs {
		if id == "" {
			return fmt.Errorf("traceableprose: RegenerationEvent.AffectedNodeIDs[%d] must not be empty", i)
		}
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("traceableprose: RegenerationEvent.OccurredAt must not be zero")
	}
	return nil
}

// RegenerationLog is an append-only record of all regeneration events for a
// document. Entries are never deleted or modified; history is immutable.
//
// CUE mirror: #RegenerationLog in schema.cue (the log itself is modelled as a
// slice of events).
type RegenerationLog struct {
	events []RegenerationEvent
}

// Append adds a new RegenerationEvent to the log. The event is validated
// before it is added; an error is returned if validation fails.
func (l *RegenerationLog) Append(event RegenerationEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("traceableprose: RegenerationLog.Append: %w", err)
	}
	l.events = append(l.events, event)
	return nil
}

// Events returns a copy of the log's event slice. The returned slice is safe
// to iterate; mutations do not affect the log.
func (l *RegenerationLog) Events() []RegenerationEvent {
	out := make([]RegenerationEvent, len(l.events))
	copy(out, l.events)
	return out
}

// Len returns the number of events in the log.
func (l *RegenerationLog) Len() int {
	return len(l.events)
}

// MarkNodesForRegeneration sets NeedsRegeneration=true on every node in the
// provided slice whose ID appears in affectedIDs. It returns the count of
// nodes that were marked.
func MarkNodesForRegeneration(nodes []TraceableNode, affectedIDs []string) int {
	affected := make(map[string]struct{}, len(affectedIDs))
	for _, id := range affectedIDs {
		affected[id] = struct{}{}
	}
	count := 0
	for i := range nodes {
		if _, ok := affected[nodes[i].ID]; ok {
			nodes[i].NeedsRegeneration = true
			count++
		}
	}
	return count
}

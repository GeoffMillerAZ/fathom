// Package messagecapture provides types and behaviour for capturing messages
// from pluggable communication channels. CUE is authoritative for the valid
// value space of Message (see schema.cue); this file is authoritative for
// runtime behaviour.
//
// Time is injected — no call to time.Now() appears in logic code. Callers
// supply a clock function or a concrete time.Time when constructing values.
//
// Concurrency contract: Message is a plain value type; values are safe to
// read concurrently. Mutable collections of Messages are not synchronized
// and must be guarded by the caller.
package messagecapture

import (
	"errors"
	"strings"
	"time"
)

// Message is a single message received from a communication channel. It
// mirrors #Message in schema.cue. Every captured message must have a non-empty
// ID (used for deduplication), a non-empty Channel and Author, and a valid At
// timestamp. Text may be empty (attachment-only posts).
type Message struct {
	// ID uniquely identifies the message within its channel.
	ID string

	// Channel names the source channel from which the message was captured.
	Channel string

	// Author identifies the sender.
	Author string

	// Text is the message body.
	Text string

	// At records when the message was sent.
	At time.Time
}

// Validate enforces the same constraints that #Message encodes in schema.cue.
// It returns a non-nil error listing all violations if any constraint fails.
func (m Message) Validate() error {
	var errs []string
	if strings.TrimSpace(m.ID) == "" {
		errs = append(errs, "ID must be non-empty")
	}
	if strings.TrimSpace(m.Channel) == "" {
		errs = append(errs, "Channel must be non-empty")
	}
	if strings.TrimSpace(m.Author) == "" {
		errs = append(errs, "Author must be non-empty")
	}
	if m.At.IsZero() {
		errs = append(errs, "At must be a non-zero time")
	}
	if len(errs) > 0 {
		return errors.New("Message: " + strings.Join(errs, "; "))
	}
	return nil
}

// Channel is the interface that any communication backend must implement to
// participate in the capture pipeline. Implementations are pluggable — Slack,
// email, GitHub comments, etc. are all just Channel implementations.
type Channel interface {
	// Poll fetches the next batch of new messages from the channel. It must
	// return a non-nil slice (may be empty) on success.
	Poll() ([]Message, error)
}

// Store is an ordered, dedup-by-ID message store used internally by Capture.
// The zero value is ready to use.
type Store struct {
	ids   map[string]struct{}
	items []Message
}

// Add appends msg to the store if its ID has not been seen before. It returns
// true when the message was inserted and false when it was a duplicate.
func (s *Store) Add(msg Message) bool {
	if s.ids == nil {
		s.ids = make(map[string]struct{})
	}
	if _, dup := s.ids[msg.ID]; dup {
		return false
	}
	s.ids[msg.ID] = struct{}{}
	s.items = append(s.items, msg)
	return true
}

// All returns a copy of the stored messages in insertion order.
func (s *Store) All() []Message {
	out := make([]Message, len(s.items))
	copy(out, s.items)
	return out
}

// Len returns the number of unique messages stored.
func (s *Store) Len() int { return len(s.items) }

// Capture collects messages from a Channel into an internal Store, deduplicating
// by Message.ID. The zero value is ready to use.
type Capture struct {
	store Store
}

// Run polls channel once and inserts any new messages into the internal store.
// It returns the number of new (non-duplicate) messages added and any error
// returned by Poll. On a Poll error the store is not modified.
func (c *Capture) Run(channel Channel) (int, error) {
	msgs, err := channel.Poll()
	if err != nil {
		return 0, err
	}
	added := 0
	for _, m := range msgs {
		if c.store.Add(m) {
			added++
		}
	}
	return added, nil
}

// Messages returns all captured messages in insertion order.
func (c *Capture) Messages() []Message {
	return c.store.All()
}

// Len returns the number of unique messages captured so far.
func (c *Capture) Len() int { return c.store.Len() }

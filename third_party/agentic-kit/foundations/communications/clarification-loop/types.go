// Package clarificationloop tracks open clarification questions during a
// multi-agent conversation. It is deterministic: all state transitions are
// driven by caller-supplied values; no time.Now() or random calls appear in
// logic code.
//
// A Loop holds an ordered list of Questions. Questions are added via Ask,
// resolved via Answer, and open questions can be enumerated via Open.
//
// Concurrency contract: Loop is not safe for concurrent use; callers that
// share a Loop across goroutines must guard access with their own
// synchronization.
package clarificationloop

import (
	"errors"
	"fmt"
	"strings"
)

// Question represents a single clarification question posed during a
// conversation. Once a question is resolved its Resolved field is true and
// Answer holds the response text.
type Question struct {
	// ID is the unique identifier for this question. Must be non-empty.
	ID string

	// Text is the question body. Must be non-empty.
	Text string

	// Resolved is true when an answer has been recorded.
	Resolved bool

	// Answer holds the response text when Resolved is true.
	Answer string
}

// Validate returns an error if the Question violates its basic invariants.
func (q Question) Validate() error {
	var errs []string
	if strings.TrimSpace(q.ID) == "" {
		errs = append(errs, "ID must be non-empty")
	}
	if strings.TrimSpace(q.Text) == "" {
		errs = append(errs, "Text must be non-empty")
	}
	if q.Resolved && strings.TrimSpace(q.Answer) == "" {
		errs = append(errs, "Answer must be non-empty when Resolved is true")
	}
	if len(errs) > 0 {
		return fmt.Errorf("Question: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Loop manages a set of clarification Questions. The zero value is ready to
// use. All mutations are applied in insertion order; duplicates are rejected
// by ID.
type Loop struct {
	questions []Question
	index     map[string]int // ID → position in questions slice
}

// Ask registers a new, unresolved question. It returns an error if a question
// with the same ID already exists, or if id or text is empty.
func (l *Loop) Ask(id, text string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("clarificationloop: id must be non-empty")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("clarificationloop: text must be non-empty")
	}
	if l.index == nil {
		l.index = make(map[string]int)
	}
	if _, exists := l.index[id]; exists {
		return fmt.Errorf("clarificationloop: question %q already exists", id)
	}
	l.index[id] = len(l.questions)
	l.questions = append(l.questions, Question{ID: id, Text: text})
	return nil
}

// Answer resolves an existing question by recording the supplied answer text.
// It returns an error if the question ID is not found, if answer is empty, or
// if the question has already been resolved.
func (l *Loop) Answer(id, answer string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("clarificationloop: id must be non-empty")
	}
	if strings.TrimSpace(answer) == "" {
		return errors.New("clarificationloop: answer must be non-empty")
	}
	if l.index == nil {
		return fmt.Errorf("clarificationloop: question %q not found", id)
	}
	pos, exists := l.index[id]
	if !exists {
		return fmt.Errorf("clarificationloop: question %q not found", id)
	}
	if l.questions[pos].Resolved {
		return fmt.Errorf("clarificationloop: question %q is already resolved", id)
	}
	l.questions[pos].Resolved = true
	l.questions[pos].Answer = answer
	return nil
}

// Open returns all questions that have not yet been resolved, in insertion
// order. The returned slice is a copy — mutations do not affect the Loop.
func (l *Loop) Open() []Question {
	var out []Question
	for _, q := range l.questions {
		if !q.Resolved {
			out = append(out, q)
		}
	}
	if out == nil {
		return []Question{}
	}
	return out
}

// All returns all questions (resolved and open) in insertion order.
// The returned slice is a copy.
func (l *Loop) All() []Question {
	out := make([]Question, len(l.questions))
	copy(out, l.questions)
	return out
}

// Get returns the question with the given ID and true, or the zero value and
// false if no such question exists.
func (l *Loop) Get(id string) (Question, bool) {
	if l.index == nil {
		return Question{}, false
	}
	pos, ok := l.index[id]
	if !ok {
		return Question{}, false
	}
	return l.questions[pos], true
}

// Len returns the total number of questions (resolved + open).
func (l *Loop) Len() int { return len(l.questions) }

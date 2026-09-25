// Package console holds the Observatory console's page projections (vision
// 35, ADR-045, docs/design/observatory-console.md §1): pure functions over
// the store, the ledgers and the repository that return typed models with an
// explicit truth state per value. cmd/fathom renders them with app-kit and
// serves their JSON unchanged; a CLI verb and a page that show the same thing
// call the same projection, so they cannot disagree (P10).
//
// The package imports no UI package. Every clock is injected.
package console

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// State is a value's truth state. The six states are drawn differently by
// every renderer: zero is a number, absent names its reason, stale carries
// its age, partial names its lower bound, error names what failed.
type State string

const (
	// StatePresent is a measured, non-zero value.
	StatePresent State = "present"
	// StateZero is a measured zero — the instrument ran and counted nothing.
	StateZero State = "zero"
	// StateAbsent means no instrument reported: never render it as 0.
	StateAbsent State = "absent"
	// StateStale is a measured value older than its freshness horizon.
	StateStale State = "stale"
	// StatePartial is a lower bound: some sources did not report.
	StatePartial State = "partial"
	// StateError means the source was there and could not be read.
	StateError State = "error"
)

// Valid reports whether s is one of the six states.
func (s State) Valid() bool {
	switch s {
	case StatePresent, StateZero, StateAbsent, StateStale, StatePartial, StateError:
		return true
	}
	return false
}

// HasValue reports whether a value in this state carries a number worth
// rendering (present, zero, stale, partial). Absent and error carry none.
func (s State) HasValue() bool {
	return s.Valid() && s != StateAbsent && s != StateError
}

// Value is one projected value with its truth state and its instrument:
// the reason (required for absent, error, stale and partial), the source it
// was read from, the denominator it was counted over, and when it was true.
type Value[T any] struct {
	V           T
	State       State
	Reason      string
	Source      string
	Denominator string
	AsOf        time.Time
}

// Present is a measured value. A caller that may have measured zero should
// use Measured so a zero is labeled as a measured zero.
func Present[T any](v T, source string) Value[T] {
	return Value[T]{V: v, State: StatePresent, Source: source}
}

// Measured is Present, except that the zero value of T becomes StateZero:
// a measured nothing, drawn as a number and never as an absence.
func Measured[T comparable](v T, source string) Value[T] {
	var zero T
	if v == zero {
		return Value[T]{V: v, State: StateZero, Source: source}
	}
	return Present(v, source)
}

// Absent is a value no instrument reported. reason says why, in words.
func Absent[T any](reason string) Value[T] {
	return Value[T]{State: StateAbsent, Reason: reason}
}

// Errored is a value whose source could not be read.
func Errored[T any](reason string) Value[T] {
	return Value[T]{State: StateError, Reason: reason}
}

// Stale is a measured value past its freshness horizon; asOf is when it was
// true and reason names the horizon it crossed.
func Stale[T any](v T, reason string, asOf time.Time) Value[T] {
	return Value[T]{V: v, State: StateStale, Reason: reason, AsOf: asOf}
}

// Partial is a lower bound; reason names what did not report.
func Partial[T any](v T, reason string) Value[T] {
	return Value[T]{V: v, State: StatePartial, Reason: reason}
}

// WithSource returns v with its source set.
func (v Value[T]) WithSource(source string) Value[T] { v.Source = source; return v }

// WithDenominator returns v with the denominator it was counted over.
func (v Value[T]) WithDenominator(d string) Value[T] { v.Denominator = d; return v }

// WithAsOf returns v stamped with the time it was true.
func (v Value[T]) WithAsOf(t time.Time) Value[T] { v.AsOf = t; return v }

// Known reports whether v carries a value (present, zero, stale, partial).
func (v Value[T]) Known() bool { return v.State.HasValue() }

// Validate checks the state contract: a valid state, and a reason wherever
// the state is not a plain measurement.
func (v Value[T]) Validate() error {
	if !v.State.Valid() {
		return fmt.Errorf("console: unknown truth state %q", v.State)
	}
	switch v.State {
	case StateAbsent, StateError, StateStale, StatePartial:
		if v.Reason == "" {
			return fmt.Errorf("console: a %s value must name its reason", v.State)
		}
	}
	return nil
}

// valueJSON is Value's wire shape. v is omitted for absent and error; state
// and reason are always present so a reader never has to infer either.
type valueJSON struct {
	V           json.RawMessage `json:"v,omitempty"`
	State       State           `json:"state"`
	Reason      string          `json:"reason"`
	Source      string          `json:"source,omitempty"`
	Denominator string          `json:"denominator,omitempty"`
	AsOf        string          `json:"as_of,omitempty"`
}

// MarshalJSON writes {v?, state, reason, source?, denominator?, as_of?}.
func (v Value[T]) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	out := valueJSON{State: v.State, Reason: v.Reason, Source: v.Source, Denominator: v.Denominator}
	if !v.AsOf.IsZero() {
		out.AsOf = v.AsOf.UTC().Format(time.RFC3339Nano)
	}
	if v.State.HasValue() {
		raw, err := json.Marshal(v.V)
		if err != nil {
			return nil, err
		}
		out.V = raw
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads the wire shape back. A v on an absent or error value is
// refused: it would be a number the writer said it did not have.
func (v *Value[T]) UnmarshalJSON(data []byte) error {
	var in valueJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	out := Value[T]{State: in.State, Reason: in.Reason, Source: in.Source, Denominator: in.Denominator}
	if in.AsOf != "" {
		t, err := time.Parse(time.RFC3339Nano, in.AsOf)
		if err != nil {
			return fmt.Errorf("console: as_of: %w", err)
		}
		out.AsOf = t
	}
	if len(in.V) > 0 {
		if !out.State.HasValue() {
			return errors.New("console: a " + string(out.State) + " value carries no v")
		}
		if err := json.Unmarshal(in.V, &out.V); err != nil {
			return err
		}
	}
	if err := out.Validate(); err != nil {
		return err
	}
	*v = out
	return nil
}

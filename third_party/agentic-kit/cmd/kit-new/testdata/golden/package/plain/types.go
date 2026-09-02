package widgetgizmo

import (
	"errors"
	"fmt"
)

// Config mirrors #Config in schema.cue. It is caller-supplied data: load it
// from CUE or JSON validated against the schema, then call Validate before
// use — Go re-checks the same constraints so a bypassed schema still fails
// closed.
type Config struct {
	// Name identifies this instance in errors and reports; must be non-empty.
	Name string `json:"name"`

	// Limit bounds the work one instance may do; must be positive (every
	// bound is explicit — VISION belief 4).
	Limit int `json:"limit"`
}

// ErrInvalidConfig is the sentinel every Validate failure wraps; test for it
// with errors.Is.
var ErrInvalidConfig = errors.New("widgetgizmo: invalid config")

// Validate reports the first constraint of #Config that c violates, wrapped
// in ErrInvalidConfig, or nil.
func (c Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: name must not be empty", ErrInvalidConfig)
	}
	if c.Limit <= 0 {
		return fmt.Errorf("%w: limit must be positive, got %d", ErrInvalidConfig, c.Limit)
	}
	return nil
}

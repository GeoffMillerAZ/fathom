package cappedconfidence

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Ceiling is one row of the table, mirroring #Ceiling in schema.cue: the
// maximum a score resting on this class of evidence may carry, and why.
type Ceiling struct {
	// Class is the evidence class the row governs. Classes are the caller's
	// vocabulary, declared in configuration, never in Go.
	Class string `json:"class"`

	// Ceiling is the maximum, on the table's scale. Equal to Config.Max
	// means "uncapped".
	Ceiling float64 `json:"ceiling"`

	// Why must be re-derivable by a reader who was not in the room. The
	// rationale is the design: a ceiling nobody can interrogate is a ceiling
	// they will route around.
	Why string `json:"why"`
}

// Config mirrors #Table in schema.cue: the top of the score scale and one
// ceiling per evidence class. Load it from CUE or JSON validated against the
// schema, then hand it to NewTable — Go re-checks the same constraints so a
// bypassed schema still fails closed.
type Config struct {
	// Max is the top of the score scale — 100 for a percentage, 5 for a
	// five-point scale. Every ceiling is at most Max.
	Max float64 `json:"max"`

	// Ceilings covers every class the caller will ever score. A class not
	// here is unscored, never defaulted.
	Ceilings []Ceiling `json:"ceilings"`
}

// ErrInvalidConfig is the sentinel every Validate failure wraps; test for it
// with errors.Is.
var ErrInvalidConfig = errors.New("cappedconfidence: invalid config")

// Validate reports the first constraint of #Table that c violates, wrapped in
// ErrInvalidConfig, or nil.
func (c Config) Validate() error {
	switch {
	case math.IsNaN(c.Max) || math.IsInf(c.Max, 0) || c.Max <= 0:
		return fmt.Errorf("%w: max must be a positive number, got %v", ErrInvalidConfig, c.Max)
	case len(c.Ceilings) == 0:
		return fmt.Errorf("%w: ceilings must not be empty — a table with no rows makes every class unscored", ErrInvalidConfig)
	}
	seen := make(map[string]bool, len(c.Ceilings))
	for i, r := range c.Ceilings {
		class := strings.TrimSpace(r.Class)
		switch {
		case class == "":
			return fmt.Errorf("%w: ceilings[%d]: class must not be empty", ErrInvalidConfig, i)
		case seen[class]:
			return fmt.Errorf("%w: class %q declared twice", ErrInvalidConfig, class)
		case math.IsNaN(r.Ceiling) || r.Ceiling < 0 || r.Ceiling > c.Max:
			return fmt.Errorf("%w: class %q: ceiling %v is outside [0, %v]", ErrInvalidConfig, class, r.Ceiling, c.Max)
		case strings.TrimSpace(r.Why) == "":
			return fmt.Errorf("%w: class %q: why must not be empty — a ceiling nobody can re-derive is one nobody can revise", ErrInvalidConfig, class)
		}
		seen[class] = true
	}
	return nil
}

// State is which of the three honest things a Score is. It is derived from
// the Score's fields by Score.State, never stored.
type State string

const (
	// StateSupported means the evidence supports the raw; it renders as is.
	StateSupported State = "supported"
	// StateCapped means the raw exceeded its ceiling; it renders as a band.
	StateCapped State = "capped"
	// StateUnscored means no ceiling could be established; it renders as a
	// refusal that says why.
	StateUnscored State = "unscored"
)

// Score is a raw judgment plus the ceiling its evidence permits, mirroring
// #Score in schema.cue. Build it with Table.Cap or Unscored; the zero value
// is unscored.
type Score struct {
	// Raw is what the scorer produced, kept verbatim — the point of the type
	// is that the overstatement stays visible, and overwriting Raw would
	// hide it.
	Raw float64 `json:"raw"`

	// Ceiling is the bound the class permits; zero when unscored.
	Ceiling float64 `json:"ceiling"`

	// Class is the evidence class the score rests on.
	Class string `json:"class,omitempty"`

	// Clamped records that Raw exceeded Ceiling — the fact Render acts on.
	Clamped bool `json:"clamped,omitempty"`

	// Why is the pinned rationale for the ceiling, carried so the band can
	// explain itself wherever it is shown.
	Why string `json:"why,omitempty"`

	// Reason is why no ceiling could be established; non-empty means
	// unscored. Cap and Unscored set it.
	Reason string `json:"reason,omitempty"`
}

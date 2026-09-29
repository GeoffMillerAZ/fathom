package cappedconfidence

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Table is a validated ceiling table. Build it with NewTable. A nil *Table
// caps nothing and reports every score unscored — the honest result of a
// table that failed to load, and the reason callers may hold a *Table they
// could not build rather than a fallback.
type Table struct {
	max     float64
	rows    map[string]Ceiling
	classes []string
}

// NewTable validates cfg and returns the table. Classes are trimmed; the
// ceilings and rationales are kept verbatim.
func NewTable(cfg Config) (*Table, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	t := &Table{max: cfg.Max, rows: make(map[string]Ceiling, len(cfg.Ceilings))}
	for _, r := range cfg.Ceilings {
		r.Class = strings.TrimSpace(r.Class)
		t.rows[r.Class] = r
		t.classes = append(t.classes, r.Class)
	}
	sort.Strings(t.classes)
	return t, nil
}

// Max returns the top of the table's scale; zero for a nil table.
func (t *Table) Max() float64 {
	if t == nil {
		return 0
	}
	return t.max
}

// Classes returns every declared class, sorted, as a fresh slice.
func (t *Table) Classes() []string {
	if t == nil {
		return nil
	}
	return append([]string(nil), t.classes...)
}

// Lookup returns the row for class and whether one is declared.
func (t *Table) Lookup(class string) (Ceiling, bool) {
	if t == nil {
		return Ceiling{}, false
	}
	r, ok := t.rows[strings.TrimSpace(class)]
	return r, ok
}

// Cap bounds raw by the ceiling its class permits and records that it did.
// It clamps DOWN only — a ceiling is a maximum, and a negative or otherwise
// out-of-range raw is a write-boundary problem this type must not paper
// over. A nil table, an undeclared class, or a raw that is not a number
// yields an unscored Score that says which.
func (t *Table) Cap(raw float64, class string) Score {
	class = strings.TrimSpace(class)
	switch {
	case t == nil:
		return Unscored(raw, class, "no ceiling table")
	case math.IsNaN(raw) || math.IsInf(raw, 0):
		return Unscored(raw, class, "raw is not a number")
	case class == "":
		return Unscored(raw, class, "no evidence class")
	}
	row, ok := t.rows[class]
	if !ok {
		return Unscored(raw, class, fmt.Sprintf("no ceiling declared for evidence class %q", class))
	}
	return Score{Raw: raw, Ceiling: row.Ceiling, Class: class, Clamped: raw > row.Ceiling, Why: row.Why}
}

// Unscored is the honest constructor for a score nothing could bound: the
// table failed to load, the class is unknown, the raw is not a number. The
// raw is kept so the record shows what was claimed; Render never shows it.
func Unscored(raw float64, class, reason string) Score {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "no reason given"
	}
	return Score{Raw: raw, Class: strings.TrimSpace(class), Reason: reason}
}

// State derives which of the three states s is in. A Reason, or no Class at
// all, is unscored; Clamped is capped; anything else is supported.
func (s Score) State() State {
	switch {
	case s.Reason != "" || s.Class == "":
		return StateUnscored
	case s.Clamped:
		return StateCapped
	default:
		return StateSupported
	}
}

// Render is how a score is allowed to reach a human.
//
// Capped, it returns a BAND naming its class ("≤50 — delimited-line
// evidence") and never the point value: a point value invites the reader to
// compare it with a score that was actually measured, and the whole claim of
// this type is that those two numbers are not comparable. Unscored, it says
// so and why. Supported, the number stands on its own — the evidence
// supports it.
func (s Score) Render() string {
	switch s.State() {
	case StateUnscored:
		reason := s.Reason
		if reason == "" {
			reason = "no evidence class"
		}
		return "unscored — " + reason
	case StateCapped:
		return fmt.Sprintf("≤%s — %s evidence", formatScore(s.Ceiling), s.Class)
	default:
		return formatScore(s.Raw)
	}
}

// formatScore prints the shortest exact decimal: "80", not "80.00".
func formatScore(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

package intent

// triggers.go — CX-21 (docs/plans/change-and-compliance.md; design
// docs/design/change-and-compliance.md §5 step 3; F9's computable subset,
// promoted by ADR-051 decision 4): a `Revisit when` or `Promote when`
// condition that names an instrument and a threshold in the bet-block shape
//
//	**Revisit when.** the layers erode {instrument: arch_violations, threshold: ">= 3"}
//
// is evaluated when the operator runs `fathom intent audit` — the reader is
// injected by the caller and called from RunAudit only, never at Load and
// never by the daemon on a schedule of its own (the dispatch non-goal,
// CX-X5). A prose trigger stays unevaluated and is listed as such; so is a
// computable one whose instrument no reader computes, whose threshold is
// not a comparison with a number, or whose reading failed. Unevaluated is
// never "not fired": nothing here guesses either way.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Trigger states.
const (
	TriggerFired       = "fired"
	TriggerHolding     = "not fired"
	TriggerUnevaluated = "unevaluated"
)

// triggerLabels are the condition labels a computable trigger may carry.
var triggerLabels = map[string]bool{"Revisit when": true, "Promote when": true}

// InstrumentReading is one instrument's value, read on demand.
type InstrumentReading struct {
	Value float64
	// Reading says what was read and when it was measured ("4 violations at
	// the ingest of 1a2b3c4, 2026-09-28").
	Reading string
}

// InstrumentReader reads a named instrument when the audit runs. ok is false
// when the instrument is not one this reader computes; err is a reading
// that failed.
type InstrumentReader func(instrument string) (reading InstrumentReading, ok bool, err error)

// TriggerVerdict is one revisit or promote condition's state at this audit.
type TriggerVerdict struct {
	ID         string  `json:"id"`
	Record     string  `json:"record"`
	Label      string  `json:"label"`
	Text       string  `json:"text"`
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Instrument string  `json:"instrument,omitempty"`
	Threshold  string  `json:"threshold,omitempty"`
	State      string  `json:"state"`
	Value      float64 `json:"value,omitempty"`
	Reading    string  `json:"reading,omitempty"`
	// Reason says why a trigger is unevaluated.
	Reason string `json:"reason,omitempty"`
}

// IsComputableTrigger reports whether c is a revisit or promote condition
// carrying the {instrument, threshold} block the audit evaluates. A block
// that also carries a goal's `bet` or `review` key is a misplaced bet, not
// a trigger (MisplacedBets names it).
func (c Condition) IsComputableTrigger() bool {
	if !triggerLabels[c.Label] || c.Terminal {
		return false
	}
	_, hasInst := c.Ann["instrument"]
	_, hasThr := c.Ann["threshold"]
	_, hasBet := c.Ann["bet"]
	_, hasReview := c.Ann["review"]
	return hasInst && hasThr && !hasBet && !hasReview
}

var thresholdRe = regexp.MustCompile(`^(>=|<=|!=|==|=|>|<|≥|≤)\s*(-?\d+(?:\.\d+)?)\s*%?$`)

// parseThreshold reads a comparison with a number: ">= 3", "< 0.5", "== 0",
// "≥20", "> 40%".
func parseThreshold(s string) (op string, n float64, ok bool) {
	m := thresholdRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", 0, false
	}
	v, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return "", 0, false
	}
	switch m[1] {
	case "≥":
		m[1] = ">="
	case "≤":
		m[1] = "<="
	case "=":
		m[1] = "=="
	}
	return m[1], v, true
}

func compare(v float64, op string, n float64) bool {
	switch op {
	case ">=":
		return v >= n
	case "<=":
		return v <= n
	case ">":
		return v > n
	case "<":
		return v < n
	case "==":
		return v == n
	case "!=":
		return v != n
	}
	return false
}

// evalComputableTriggers returns every non-terminal revisit and promote
// condition's verdict, sorted by file then line: computable ones read
// through read (each instrument read once per audit), prose ones listed
// unevaluated. A nil read leaves every trigger unevaluated.
func evalComputableTriggers(c Corpus, read InstrumentReader) []TriggerVerdict {
	cache := map[string]struct {
		r   InstrumentReading
		ok  bool
		err error
	}{}
	var out []TriggerVerdict
	for _, cond := range c.Conditions {
		if !triggerLabels[cond.Label] || cond.Terminal {
			continue
		}
		v := TriggerVerdict{ID: cond.ID, Record: cond.Record, Label: cond.Label, Text: strings.TrimSpace(cond.Text), File: cond.File, Line: cond.Line, State: TriggerUnevaluated}
		if !cond.IsComputableTrigger() {
			v.Reason = "prose trigger — human-judged; add {instrument: <name>, threshold: \"<op> <number>\"} to make it computable"
			out = append(out, v)
			continue
		}
		v.Instrument = strings.TrimSpace(cond.Ann["instrument"])
		v.Threshold = strings.TrimSpace(cond.Ann["threshold"])
		op, n, ok := parseThreshold(v.Threshold)
		switch {
		case !ok:
			v.Reason = fmt.Sprintf("threshold %q is not a comparison with a number (>=, >, <=, <, ==, != N)", v.Threshold)
		case read == nil:
			v.Reason = "no instrument reader — run `fathom intent audit` to evaluate it"
		default:
			got, seen := cache[v.Instrument]
			if !seen {
				got.r, got.ok, got.err = read(v.Instrument)
				cache[v.Instrument] = got
			}
			switch {
			case got.err != nil:
				v.Reason = "reading " + v.Instrument + " failed: " + got.err.Error()
			case !got.ok:
				v.Reason = fmt.Sprintf("%q is not an instrument this audit computes", v.Instrument)
			default:
				v.Value, v.Reading = got.r.Value, got.r.Reading
				v.State = TriggerHolding
				if compare(got.r.Value, op, n) {
					v.State = TriggerFired
				}
			}
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

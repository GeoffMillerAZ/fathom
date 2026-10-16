// Package analytics composes opt-in, meaning-bearing charts from app-selected
// rows. It validates presentation semantics; applications still own selection,
// aggregation, authorization, source observation times and domain truth.
//
// RenderBars is L3b: consumers must scan this package and ui/vizframe when
// building Tailwind CSS. Validate and WriteCSV need no CSS or browser runtime.
// Low-level ui/viz and ui/vizframe behavior is unchanged by adopting this package.
package analytics

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

// MaxRows bounds a single chart/table/export selection. Apps must select or
// aggregate larger inputs before invoking this recipe; rows are never truncated.
const MaxRows = 200

// MaxMagnitude bounds values and interval endpoints for this small comparison
// recipe and keeps the underlying renderer's integer formatting in range.
const MaxMagnitude = 1e12

const maxTextBytes = 2048

// WindowKind distinguishes a current-state snapshot from a period of events.
type WindowKind string

const (
	SnapshotWindow WindowKind = "snapshot"
	RangeWindow    WindowKind = "range"
)

// Window describes the actual selection, never an independently authored label.
// SnapshotWindow uses Start as its observation instant and requires a zero End.
// RangeWindow is [Start, End), with End strictly after Start. TimeZone is an
// explicit IANA name (or UTC); endpoint labels are generated in that timezone.
type Window struct {
	Kind       WindowKind
	Start, End time.Time
	TimeZone   string
}

// Freshness refers to when the SOURCE was observed, not when a chart or forecast
// was calculated. ObservedAt plus a positive MaxAge produces a fresh/stale state.
// An unknown source observation requires UnknownReason instead. Age equal to
// MaxAge is fresh; greater age is stale. All checks use the caller's explicit now.
type Freshness struct {
	ObservedAt    time.Time
	MaxAge        time.Duration
	UnknownReason string
}

type Completeness string

const (
	Complete Completeness = "complete"
	Partial  Completeness = "partial"
	Unknown  Completeness = "unknown"
)

// Contract is one chart's app-authored meaning. Unit is explicit display text
// such as "USD" or "applications"; no currency or domain policy is inferred.
// Grain and Aggregation explain what one row and its numeric measure represent.
// Decimals is 0..2, matching ui/viz.Bars' existing precision. Every output rounds
// the same selected values to that precision; none recomputes an aggregation.
// Uncertainty is required, including an explicit statement when unquantified or
// inapplicable. Partial and unknown completeness require a CoverageNote.
type Contract struct {
	ID, Title, Unit, Grain, Aggregation string
	Decimals                            int
	Window                              Window
	Source                              vizframe.Source
	Freshness                           Freshness
	Completeness                        Completeness
	CoverageNote, Uncertainty           string
}

// Interval is an app-provided bound, not an inferred confidence interval.
type Interval struct {
	Low, High float64
}

// Row is a selected observation. A pointer to zero means a measured zero; nil
// means missing and requires MissingReason. Missing rows are retained in the
// table and CSV and are not passed to the SVG renderer as zeroes. Role is an
// app-owned semantic token name; empty uses accent. Bounds are optional.
type Row struct {
	ID, Label, Role string
	Value           *float64
	MissingReason   string
	Bounds          *Interval
	// DetailHref optionally links the row label to an app-owned, root-relative
	// detail destination. Empty retains a plain label. It does not enter CSV,
	// calculate a route, grant access or make the row's value more trustworthy.
	DetailHref string
}

type LegendItem struct {
	Role, Label string
}

// Dataset retains app-selected order. Multiple plotted roles require a complete
// legend. A single role needs no legend because each row has a direct label.
type Dataset struct {
	Contract Contract
	Rows     []Row
	Legend   []LegendItem
}

// roleLabels is called after validation. Labels are app-authored meaning, never
// inferred from token names, colors, row order or a numeric threshold.
func (d Dataset) roleLabels() map[string]string {
	labels := make(map[string]string, len(d.Legend))
	for _, item := range d.Legend {
		labels[item.Role] = item.Label
	}
	return labels
}

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	rolePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

func checkText(field, value string, required bool) error {
	if len(value) > maxTextBytes {
		return fmt.Errorf("analytics: %s exceeds %d bytes", field, maxTextBytes)
	}
	// HTML parsers replace malformed UTF-8 and NUL, while CSV preserves the
	// original bytes. Refuse them before either output can change the meaning.
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("analytics: %s must be valid UTF-8 without NUL", field)
	}
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("analytics: %s is required", field)
	}
	return nil
}

func validNumber(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= MaxMagnitude
}

func rowRole(row Row) string {
	if row.Role == "" {
		return "accent"
	}
	return row.Role
}

// Validate checks the semantic contract and row integrity without rendering.
// Stale or explicitly unknown freshness is valid and visibly disclosed. Invalid
// declarations return an error; this method does not certify app-owned facts.
func (d Dataset) Validate(now time.Time) error {
	c := d.Contract
	if now.IsZero() {
		return fmt.Errorf("analytics: now must be explicit and nonzero")
	}
	for _, field := range []struct{ name, value string }{
		{"id", c.ID}, {"title", c.Title}, {"unit", c.Unit}, {"grain", c.Grain},
		{"aggregation", c.Aggregation}, {"uncertainty", c.Uncertainty},
	} {
		if err := checkText(field.name, field.value, true); err != nil {
			return err
		}
	}
	if !idPattern.MatchString(c.ID) {
		return fmt.Errorf("analytics: id must begin with a letter and contain only letters, digits, hyphens or underscores")
	}
	if c.Decimals < 0 || c.Decimals > 2 {
		return fmt.Errorf("analytics: decimals must be between 0 and 2")
	}
	if err := c.Window.validate(); err != nil {
		return err
	}
	if err := (vizframe.Options{Source: c.Source}).Validate(); err != nil {
		return fmt.Errorf("analytics: source: %w", err)
	}
	for _, field := range []struct{ name, value string }{
		{"source text", c.Source.Text}, {"source href", c.Source.Href},
		{"coverage note", c.CoverageNote}, {"freshness reason", c.Freshness.UnknownReason},
	} {
		if err := checkText(field.name, field.value, false); err != nil {
			return err
		}
	}
	f := c.Freshness
	if f.ObservedAt.IsZero() {
		if strings.TrimSpace(f.UnknownReason) == "" || f.MaxAge != 0 {
			return fmt.Errorf("analytics: unknown source freshness requires a reason and no max age")
		}
	} else if f.ObservedAt.After(now) || f.MaxAge <= 0 || f.UnknownReason != "" {
		return fmt.Errorf("analytics: observed source freshness requires a nonfuture observation, positive max age and no unknown reason")
	}
	switch c.Completeness {
	case Complete:
	case Partial, Unknown:
		if strings.TrimSpace(c.CoverageNote) == "" {
			return fmt.Errorf("analytics: %s completeness requires a coverage note", c.Completeness)
		}
	default:
		return fmt.Errorf("analytics: completeness must be complete, partial or unknown")
	}
	if len(d.Rows) > MaxRows {
		return fmt.Errorf("analytics: %d rows exceeds the %d-row limit; select or aggregate in the app", len(d.Rows), MaxRows)
	}
	if len(d.Legend) > MaxRows {
		return fmt.Errorf("analytics: legend exceeds the %d-entry limit", MaxRows)
	}
	ids, roles := map[string]bool{}, map[string]bool{}
	for i, row := range d.Rows {
		prefix := fmt.Sprintf("row %d", i)
		for _, field := range []struct{ name, value string }{{"id", row.ID}, {"label", row.Label}} {
			if err := checkText(prefix+" "+field.name, field.value, true); err != nil {
				return err
			}
		}
		if err := checkText(prefix+" missing reason", row.MissingReason, false); err != nil {
			return err
		}
		if err := validateDetailHref(row.DetailHref); err != nil {
			return fmt.Errorf("analytics: %s: %w", prefix, err)
		}
		if ids[row.ID] {
			return fmt.Errorf("analytics: duplicate row id %q", row.ID)
		}
		ids[row.ID] = true
		if !rolePattern.MatchString(rowRole(row)) || len(row.Role) > 128 {
			return fmt.Errorf("analytics: row %q has an invalid semantic role", row.ID)
		}
		if row.Value == nil {
			if strings.TrimSpace(row.MissingReason) == "" || c.Completeness == Complete || row.Bounds != nil {
				return fmt.Errorf("analytics: missing row %q requires a reason, partial/unknown completeness and no bounds", row.ID)
			}
			continue
		}
		if row.MissingReason != "" {
			return fmt.Errorf("analytics: present row %q cannot have a missing reason", row.ID)
		}
		if !validNumber(*row.Value) {
			return fmt.Errorf("analytics: row %q value must be finite and within +/-%.0f", row.ID, MaxMagnitude)
		}
		if row.Bounds != nil {
			b := row.Bounds
			if !validNumber(b.Low) || !validNumber(b.High) || b.Low > b.High || *row.Value < b.Low || *row.Value > b.High {
				return fmt.Errorf("analytics: row %q bounds must be finite, ordered and contain its value", row.ID)
			}
		}
		roles[rowRole(row)] = true
	}
	legend := map[string]bool{}
	for _, item := range d.Legend {
		if !rolePattern.MatchString(item.Role) || len(item.Role) > 128 || legend[item.Role] {
			return fmt.Errorf("analytics: legend roles must be valid and unique")
		}
		if err := checkText("legend label", item.Label, true); err != nil {
			return err
		}
		legend[item.Role] = true
	}
	if len(roles) > 1 {
		for _, row := range d.Rows {
			if row.Value != nil && !legend[rowRole(row)] {
				return fmt.Errorf("analytics: multiple plotted roles require a legend label for %q", rowRole(row))
			}
		}
	}
	return nil
}

func (w Window) validate() error {
	if strings.TrimSpace(w.TimeZone) == "" || len(w.TimeZone) > 128 {
		return fmt.Errorf("analytics: window timezone is required")
	}
	if w.TimeZone == "Local" {
		return fmt.Errorf("analytics: window timezone must use UTC or an explicit IANA name; Local depends on process configuration")
	}
	if _, err := time.LoadLocation(w.TimeZone); err != nil {
		return fmt.Errorf("analytics: invalid window timezone: %w", err)
	}
	if w.Start.IsZero() {
		return fmt.Errorf("analytics: window start is required")
	}
	switch w.Kind {
	case SnapshotWindow:
		if !w.End.IsZero() {
			return fmt.Errorf("analytics: snapshot window cannot have an end")
		}
	case RangeWindow:
		if w.End.IsZero() || !w.End.After(w.Start) {
			return fmt.Errorf("analytics: range window end must follow start")
		}
	default:
		return fmt.Errorf("analytics: window kind must be snapshot or range")
	}
	return nil
}

func (w Window) label() string {
	loc, _ := time.LoadLocation(w.TimeZone) // validated before any output
	start := w.Start.In(loc).Format(time.RFC3339Nano)
	if w.Kind == SnapshotWindow {
		return "Snapshot at " + start + " [" + w.TimeZone + "]"
	}
	return start + " to " + w.End.In(loc).Format(time.RFC3339Nano) + " (end exclusive; " + w.TimeZone + ")"
}

func (f Freshness) label(now time.Time) string {
	if f.ObservedAt.IsZero() {
		return "Source freshness unknown: " + f.UnknownReason
	}
	state := "Fresh"
	if now.Sub(f.ObservedAt) > f.MaxAge {
		state = "Stale"
	}
	return fmt.Sprintf("%s: source observed %s; maximum age %s", state, f.ObservedAt.Format(time.RFC3339Nano), f.MaxAge)
}

func rounded(v float64, decimals int) float64 {
	value, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', decimals, 64), 64)
	if value == 0 {
		return 0 // normalize negative zero in all outputs
	}
	return value
}

func number(v float64, decimals int, grouped bool) string {
	v = rounded(v, decimals)
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if decimals > 0 {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	// Match viz.Bars: only integral values receive grouping separators.
	if grouped && v == math.Trunc(v) && math.Abs(v) >= 1000 {
		negative := strings.HasPrefix(s, "-")
		if negative {
			s = s[1:]
		}
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
		if negative {
			s = "-" + s
		}
	}
	return s
}

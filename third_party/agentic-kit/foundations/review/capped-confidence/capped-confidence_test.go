package cappedconfidence_test

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	cappedconfidence "github.com/geoffmilleraz/agentic-kit/foundations/review/capped-confidence"
)

// Class names here are the test's, not the package's: the vocabulary is
// always the caller's, and the package holds no enum to drift from.
func percentage() cappedconfidence.Config {
	return cappedconfidence.Config{
		Max: 100,
		Ceilings: []cappedconfidence.Ceiling{
			{Class: "primary-record", Ceiling: 100, Why: "the owner's own record"},
			{Class: "aggregated", Ceiling: 80, Why: "accurate when fresh, stale for a long time"},
			{Class: "guessed-markup", Ceiling: 70, Why: "structure guessed from markup"},
			{Class: "delimited-line", Ceiling: 50, Why: "split on a delimiter"},
			{Class: "model-nominated", Ceiling: 40, Why: "a model's nomination"},
		},
	}
}

func table(t *testing.T) *cappedconfidence.Table {
	t.Helper()
	tb, err := cappedconfidence.NewTable(percentage())
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func format(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// TestConfigValidate mirrors testdata/invalid.cue: every commented-out
// negative fixture there is rejected here in Go.
func TestConfigValidate(t *testing.T) {
	row := func(class string, ceiling float64, why string) cappedconfidence.Ceiling {
		return cappedconfidence.Ceiling{Class: class, Ceiling: ceiling, Why: why}
	}
	tests := []struct {
		name    string
		cfg     cappedconfidence.Config
		wantErr string
	}{
		{name: "valid", cfg: percentage()},
		{name: "zero max", cfg: cappedconfidence.Config{Max: 0, Ceilings: []cappedconfidence.Ceiling{row("a", 0, "w")}}, wantErr: "max must be a positive number"},
		{name: "NaN max", cfg: cappedconfidence.Config{Max: math.NaN(), Ceilings: []cappedconfidence.Ceiling{row("a", 0, "w")}}, wantErr: "max must be a positive number"},
		{name: "no rows", cfg: cappedconfidence.Config{Max: 100}, wantErr: "ceilings must not be empty"},
		{name: "empty class", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("  ", 50, "w")}}, wantErr: "class must not be empty"},
		{name: "duplicate class", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("a", 50, "w"), row("a ", 60, "w")}}, wantErr: `class "a" declared twice`},
		{name: "ceiling above max", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("a", 101, "w")}}, wantErr: "outside [0, 100]"},
		{name: "negative ceiling", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("a", -1, "w")}}, wantErr: "outside [0, 100]"},
		{name: "NaN ceiling", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("a", math.NaN(), "w")}}, wantErr: "outside [0, 100]"},
		{name: "empty why", cfg: cappedconfidence.Config{Max: 100, Ceilings: []cappedconfidence.Ceiling{row("a", 50, " ")}}, wantErr: "why must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, cappedconfidence.ErrInvalidConfig) {
				t.Fatalf("Validate() = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %q, want it to mention %q", err, tt.wantErr)
			}
			if _, err := cappedconfidence.NewTable(tt.cfg); !errors.Is(err, cappedconfidence.ErrInvalidConfig) {
				t.Fatalf("NewTable accepted what Validate refused: %v", err)
			}
		})
	}
}

// TestCapNeverExceedsCeiling walks every class against raws that straddle its
// ceiling, including a value a model actually returned once (140 on a
// 0–100 scale).
func TestCapNeverExceedsCeiling(t *testing.T) {
	tb := table(t)
	for _, class := range tb.Classes() {
		row, ok := tb.Lookup(class)
		if !ok {
			t.Fatalf("Lookup(%q) lost a declared class", class)
		}
		for _, raw := range []float64{0, row.Ceiling / 2, row.Ceiling, row.Ceiling + 0.5, row.Ceiling + 1, 140} {
			s := tb.Cap(raw, class)
			if wantClamped := raw > row.Ceiling; s.Clamped != wantClamped {
				t.Errorf("Cap(%v, %q).Clamped = %v, want %v", raw, class, s.Clamped, wantClamped)
			}
			if s.Ceiling != row.Ceiling || s.Why != row.Why || s.Class != class {
				t.Errorf("Cap(%v, %q) = %+v, want the row's ceiling, why and class", raw, class, s)
			}
			if s.Raw != raw {
				t.Errorf("Cap(%v, %q).Raw = %v — the raw must survive so the overstatement is visible", raw, class, s.Raw)
			}
			if s.Clamped && strings.Contains(s.Render(), format(raw)) {
				t.Errorf("Cap(%v, %q).Render() = %q leaks the over-ceiling value", raw, class, s.Render())
			}
			if s.State() == cappedconfidence.StateUnscored {
				t.Errorf("Cap(%v, %q) came back unscored from a declared class", raw, class)
			}
		}
	}
}

// TestCapRendersBandNotPoint is the headline: a 92 resting on a line split on
// a delimiter renders as a bound with its reason, never as a 92.
func TestCapRendersBandNotPoint(t *testing.T) {
	tb := table(t)
	cases := []struct {
		raw   float64
		class string
		want  string
	}{
		{92, "delimited-line", "≤50 — delimited-line evidence"},
		{100, "model-nominated", "≤40 — model-nominated evidence"},
		{85.5, "guessed-markup", "≤70 — guessed-markup evidence"},
		{140, "aggregated", "≤80 — aggregated evidence"},
	}
	for _, c := range cases {
		s := tb.Cap(c.raw, c.class)
		if got := s.Render(); got != c.want {
			t.Errorf("Cap(%v, %q).Render() = %q, want %q", c.raw, c.class, got, c.want)
		}
		if s.State() != cappedconfidence.StateCapped {
			t.Errorf("Cap(%v, %q).State() = %s, want capped", c.raw, c.class, s.State())
		}
	}
}

// TestSupportedRendersNormally pins the other half: refusing to print a
// number that IS supported would be its own dishonesty. A raw exactly at the
// ceiling is supported.
func TestSupportedRendersNormally(t *testing.T) {
	tb := table(t)
	cases := []struct {
		raw   float64
		class string
		want  string
	}{
		{92, "primary-record", "92"},
		{100, "primary-record", "100"},
		{80, "aggregated", "80"},
		{61.5, "guessed-markup", "61.5"},
		{0, "delimited-line", "0"},
	}
	for _, c := range cases {
		s := tb.Cap(c.raw, c.class)
		if s.Clamped || s.State() != cappedconfidence.StateSupported {
			t.Errorf("Cap(%v, %q) = %+v, want supported", c.raw, c.class, s)
		}
		if got := s.Render(); got != c.want {
			t.Errorf("Cap(%v, %q).Render() = %q, want %q", c.raw, c.class, got, c.want)
		}
	}
}

// TestCapClampsDownOnly: a ceiling is a maximum. Flooring a negative raw here
// would let "≤100" render for a value that was never a score.
func TestCapClampsDownOnly(t *testing.T) {
	s := table(t).Cap(-5, "primary-record")
	if s.Clamped || s.Render() != "-5" || s.State() != cappedconfidence.StateSupported {
		t.Errorf("Cap(-5) = %+v rendered %q; a ceiling must not floor", s, s.Render())
	}
}

// TestUnscoredSaysSo: with nothing to bound a number, the number is not
// rendered — not from a missing table, an undeclared class, an empty class,
// a raw that is not a number, or a Score that never went through Cap.
func TestUnscoredSaysSo(t *testing.T) {
	tb := table(t)
	var missing *cappedconfidence.Table
	cases := []struct {
		name string
		s    cappedconfidence.Score
		want string
	}{
		{"nil table", missing.Cap(92, "primary-record"), "no ceiling table"},
		{"undeclared class", tb.Cap(92, "unlisted"), `no ceiling declared for evidence class "unlisted"`},
		{"empty class", tb.Cap(92, "  "), "no evidence class"},
		{"NaN raw", tb.Cap(math.NaN(), "primary-record"), "raw is not a number"},
		{"Inf raw", tb.Cap(math.Inf(1), "primary-record"), "raw is not a number"},
		{"zero value", cappedconfidence.Score{Raw: 92}, "no evidence class"},
		{"constructor", cappedconfidence.Unscored(92, "primary-record", "table failed to load: boom"), "table failed to load: boom"},
		{"constructor, no reason", cappedconfidence.Unscored(92, "primary-record", " "), "no reason given"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.s.State() != cappedconfidence.StateUnscored {
				t.Fatalf("State() = %s, want unscored", c.s.State())
			}
			got := c.s.Render()
			if got != "unscored — "+c.want {
				t.Errorf("Render() = %q, want %q", got, "unscored — "+c.want)
			}
			if strings.Contains(got, "92") {
				t.Errorf("Render() = %q rendered the unbounded score anyway", got)
			}
			if c.s.Clamped {
				t.Error("an unscored score claims to be clamped")
			}
		})
	}
	if missing.Max() != 0 || missing.Classes() != nil {
		t.Error("a nil table reported a scale or classes")
	}
	if _, ok := missing.Lookup("primary-record"); ok {
		t.Error("a nil table found a row")
	}
}

func TestClassesAreSortedAndCopied(t *testing.T) {
	tb := table(t)
	got := tb.Classes()
	if strings.Join(got, ",") != "aggregated,delimited-line,guessed-markup,model-nominated,primary-record" {
		t.Errorf("Classes() = %v, want sorted", got)
	}
	got[0] = "mutated"
	if tb.Classes()[0] == "mutated" {
		t.Error("Classes() exposed the table's own slice")
	}
	if tb.Max() != 100 {
		t.Errorf("Max() = %v, want 100", tb.Max())
	}
	if _, ok := tb.Lookup(" aggregated "); !ok {
		t.Error("Lookup did not trim the class")
	}
}

// TestJSONMirrorsSchema pins the wire shape #Score describes: a supported
// score carries no clamped/reason keys, an unscored one carries its reason.
func TestJSONMirrorsSchema(t *testing.T) {
	tb := table(t)
	keys := func(s cappedconfidence.Score) map[string]any {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	sup := keys(tb.Cap(92, "primary-record"))
	if _, has := sup["clamped"]; has {
		t.Errorf("supported score serialised clamped: %v", sup)
	}
	if _, has := sup["reason"]; has {
		t.Errorf("supported score serialised a reason: %v", sup)
	}
	capped := keys(tb.Cap(92, "delimited-line"))
	if capped["clamped"] != true || capped["class"] != "delimited-line" || capped["raw"] != 92.0 {
		t.Errorf("capped score = %v", capped)
	}
	unscored := keys(tb.Cap(92, "unlisted"))
	if unscored["reason"] == nil || unscored["ceiling"] != 0.0 {
		t.Errorf("unscored score = %v", unscored)
	}
}

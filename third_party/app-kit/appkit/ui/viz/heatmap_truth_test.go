package viz

import (
	"encoding/xml"
	"errors"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// heatmap_truth_test.go covers FATHOM-KIT-03: a heatmap cell is a value, a
// measured zero, or missing / unknown / error — and absence is never drawn as
// a low value.

// mixedTruthFixture exercises every state and every derivation path: explicit
// and derived zero, NaN-derived and declared missing, declared unknown, ±Inf
// and an unrecognized state (both become unknown), a declared error, a reason
// on a measured value, a declared-missing cell whose numeric entry must be
// ignored, and a state that extends the grid beyond the numeric rows.
func mixedTruthFixture() (rows [][]float64, opts HeatmapOptions) {
	rows = [][]float64{
		{0, 4, 8, math.NaN()},
		{2, 99, math.Inf(1), 10},
		{6},
	}
	opts = HeatmapOptions{
		XLabels:   []string{"dialect", "provider", "platform", "concern"},
		YLabels:   []string{"fathom", "app-kit", "rac"},
		AriaLabel: "layer coverage",
		States: [][]HeatmapCell{
			{{}, {Reason: "3 claims stale"}, {State: CellZero}, {State: CellMissing, Reason: "not compiled"}},
			{{State: CellUnknown, Reason: "index unreadable"}, {State: CellMissing, Reason: "not compiled"}, {}, {State: CellError, Reason: "compile failed: exit 2"}},
			{{}, {State: "stale"}, {}, {}, {State: CellError, Reason: "timeout"}},
		},
	}
	return rows, opts
}

func TestHeatmapTruth_MixedGolden(t *testing.T) {
	rows, opts := mixedTruthFixture()
	assertGolden(t, Heatmap(rows, opts), "heatmap_truth_mixed")
}

// The adversarial all-missing cases: a grid with nothing measured must not
// draw a single ramp fill, must not claim a value range, and must say why.
func TestHeatmapTruth_AllMissingGolden(t *testing.T) {
	declared := make([][]HeatmapCell, 3)
	for i := range declared {
		declared[i] = []HeatmapCell{{State: CellMissing, Reason: "not compiled"}, {State: CellMissing, Reason: "not compiled"}, {State: CellMissing, Reason: "not compiled"}}
	}
	got := Heatmap(nil, HeatmapOptions{States: declared, XLabels: []string{"a", "b", "c"}, YLabels: []string{"x", "y", "z"}, AriaLabel: "nothing compiled"})
	assertGolden(t, got, "heatmap_truth_all_missing")

	nan := math.NaN()
	derived := Heatmap([][]float64{{nan, nan}, {nan, nan}}, HeatmapOptions{TruthStates: true, AriaLabel: "no data"})
	assertGolden(t, derived, "heatmap_truth_all_nan")

	for name, out := range map[string]string{"declared": got, "derived": derived} {
		if strings.Contains(out, "fill-opacity") || strings.Contains(out, "var(--ak-color-accent)") {
			t.Errorf("%s all-missing grid drew a ramp fill — absence must never look like a low value:\n%s", name, out)
		}
		if strings.Contains(out, ">value ") || strings.Contains(out, "range") {
			t.Errorf("%s all-missing grid claims a value or range: %s", name, out)
		}
		if strings.Contains(out, ">0<") {
			t.Errorf("%s all-missing grid drew a zero glyph: %s", name, out)
		}
	}
	for _, want := range []string{"<desc>9 cells: missing 9.</desc>", ">missing: not compiled</text>", "<title>y, b: missing — not compiled</title>"} {
		if !strings.Contains(got, want) {
			t.Errorf("declared all-missing grid missing %q", want)
		}
	}
	for _, want := range []string{"<desc>4 cells: missing 4.</desc>", ">missing: no value supplied</text>", "<title>row 2, column 1: missing — no value supplied</title>"} {
		if !strings.Contains(derived, want) {
			t.Errorf("derived all-missing grid missing %q", want)
		}
	}
}

// Hostile caller text in labels, reasons and an unrecognized state must stay
// escaped everywhere it surfaces: titles, the legend and the summary.
func TestHeatmapTruth_HostileTextGolden(t *testing.T) {
	long := strings.Repeat("ü", 60)
	got := Heatmap([][]float64{{1, math.NaN()}}, HeatmapOptions{
		XLabels:   []string{`<script>alert(1)</script>`, `"q"`},
		YLabels:   []string{`</title><b>x</b>`},
		AriaLabel: `<svg onload=alert(1)>`,
		States: [][]HeatmapCell{{
			{State: `"><img src=x onerror=alert(1)>`, Reason: `<i>why</i>`},
			{State: CellMissing, Reason: long},
		}},
	})
	assertGolden(t, got, "heatmap_truth_hostile")
	for _, raw := range []string{"<script>", "<img", "<b>", "<i>", "</title><b>", "<svg onload"} {
		if strings.Contains(got, raw) {
			t.Errorf("unescaped caller text %q in output", raw)
		}
	}
	if !strings.Contains(got, strings.Repeat("ü", 39)+"…</text>") {
		t.Error("long uniform reason was not truncated in the legend")
	}
	if !strings.Contains(got, long+"</title>") {
		t.Error("the cell title must keep the full reason")
	}
	assertWellFormedSVG(t, got)
}

// Each state has a distinct, non-color-only signature, and none of the three
// absent states is drawn with the value role or a fill — even when the caller
// supplied a numeric entry alongside the declared state.
func TestHeatmapTruth_StatesAreDistinctAndNeverZeroFill(t *testing.T) {
	render := func(state CellState, v float64) string {
		return Heatmap([][]float64{{v}}, HeatmapOptions{States: [][]HeatmapCell{{{State: state}}}})
	}
	zero := render(CellZero, 0)
	if !strings.Contains(zero, `fill="var(--ak-color-accent)" fill-opacity=`) || !strings.Contains(zero, ">0</text>") {
		t.Errorf("zero must be a filled ramp cell with a 0 glyph: %s", zero)
	}
	marks := map[string]string{}
	for name, state := range map[string]CellState{"missing": CellMissing, "unknown": CellUnknown, "error": CellError} {
		out := render(state, 5)
		marks[name] = out
		if strings.Contains(out, "var(--ak-color-accent)") || strings.Contains(out, "fill-opacity") {
			t.Errorf("%s cell drew the value ramp; a declared numeric entry leaked through: %s", name, out)
		}
		if !strings.Contains(out, `fill="none" pointer-events="all"`) {
			t.Errorf("%s cell must stay unfilled but hoverable: %s", name, out)
		}
		if strings.Contains(out, ">0</text>") || strings.Contains(out, ">5</text>") {
			t.Errorf("%s cell printed a number: %s", name, out)
		}
	}
	if !strings.Contains(marks["missing"], `stroke-dasharray="2 2"/><path d="M`) {
		t.Errorf("missing must be dashed and hatched: %s", marks["missing"])
	}
	if !strings.Contains(marks["unknown"], `stroke-dasharray="1 2"/><path d="M`) || !strings.Contains(marks["unknown"], ">?</text>") {
		t.Errorf("unknown must be dotted, cross-hatched and marked ?: %s", marks["unknown"])
	}
	if !strings.Contains(marks["error"], `stroke="var(--ak-color-danger)" stroke-width="1.5"/><path`) {
		t.Errorf("error must carry a danger outline and cross: %s", marks["error"])
	}
	if strings.Contains(marks["missing"], "danger") || strings.Contains(marks["unknown"], "danger") {
		t.Error("only error cells may use the danger role")
	}
}

func TestHeatmapTruth_TitlesLegendAndSummary(t *testing.T) {
	rows, opts := mixedTruthFixture()
	got := Heatmap(rows, opts)
	assertWellFormedSVG(t, got)
	for _, want := range []string{
		`role="img"`, `aria-label="layer coverage"`, `<title>layer coverage</title>`,
		"<desc>15 cells: value 2 (range 0–6); zero 2; missing 6; unknown 3; error 2.</desc>",
		"<title>fathom, dialect: 0 (zero)</title>",
		"<title>fathom, provider: 4 — 3 claims stale</title>",
		"<title>fathom, platform: 0 (zero)</title>", // declared zero ignores the numeric 8
		"<title>fathom, concern: missing — not compiled</title>",
		"<title>app-kit, dialect: unknown — index unreadable</title>",
		"<title>app-kit, provider: missing — not compiled</title>", // declared missing ignores 99
		"<title>app-kit, platform: unknown — value is not finite</title>",
		"<title>app-kit, concern: error — compile failed: exit 2</title>",
		"<title>rac, dialect: 6</title>",
		"<title>rac, provider: unknown — unrecognized state &#34;stale&#34;</title>",
		"<title>rac, platform: missing — no value supplied</title>",
		"<title>rac, column 5: error — timeout</title>", // the state extends the grid
		">value 0–6</text>", ">zero</text>", ">missing</text>", ">unknown</text>", ">error</text>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("mixed truth heatmap missing %q", want)
		}
	}
	// Every grid cell carries exactly one title: 15 cells plus the chart title.
	if n := strings.Count(got, "<title>"); n != 16 {
		t.Errorf("titles = %d, want 16 (one per cell plus the chart)", n)
	}
	// Legend order is fixed: value, zero, missing, unknown, error.
	order := []string{">value 0–6<", ">zero<", ">missing<", ">unknown<", ">error<"}
	last := -1
	for _, label := range order {
		i := strings.Index(got, label)
		if i < last {
			t.Errorf("legend entry %s out of order", label)
		}
		last = i
	}
}

// The grid spans the union of rows and States; a state-only grid still renders.
func TestHeatmapTruth_StatesExtendTheGrid(t *testing.T) {
	got := Heatmap(nil, HeatmapOptions{States: [][]HeatmapCell{{{State: CellError}}, {{}, {State: CellUnknown}}}})
	if !strings.Contains(got, "<desc>4 cells: missing 2; unknown 1; error 1.</desc>") {
		t.Errorf("state-only grid summary wrong: %s", got)
	}
	empty := Heatmap(nil, HeatmapOptions{TruthStates: true})
	if !strings.Contains(empty, "<desc>No cells.</desc>") || !strings.HasPrefix(empty, "<svg") {
		t.Errorf("empty truth heatmap must still be a described svg: %s", empty)
	}
}

// Measured values keep the original ramp; truth mode only adds meaning.
func TestHeatmapTruth_ValuesKeepTheRamp(t *testing.T) {
	got := Heatmap([][]float64{{0, 10}}, HeatmapOptions{TruthStates: true})
	for _, want := range []string{`fill-opacity="0.12"><title>row 1, column 1: 0 (zero)</title>`, `fill-opacity="0.97"><title>row 1, column 2: 10</title>`} {
		if !strings.Contains(got, want) {
			t.Errorf("ramp changed in truth mode, missing %q: %s", want, got)
		}
	}
	// Excluded states never stretch the derived range.
	withAbsent := Heatmap([][]float64{{2, 4, 1000}}, HeatmapOptions{States: [][]HeatmapCell{{{}, {}, {State: CellError}}}})
	if !strings.Contains(withAbsent, ">value 2–4</text>") {
		t.Errorf("an error cell's numeric entry leaked into the value range: %s", withAbsent)
	}
}

func TestHeatmapTruth_LegendWrapsWithinNarrowGrids(t *testing.T) {
	got := Heatmap([][]float64{{1}}, HeatmapOptions{States: [][]HeatmapCell{{{}, {State: CellZero}, {State: CellMissing, Reason: "not compiled"}, {State: CellUnknown}, {State: CellError, Reason: "compiler exited with status 2"}}}})
	w := regexp.MustCompile(`width="(\d+)"`).FindStringSubmatch(got)
	if w == nil || len(w[1]) > 3 {
		t.Fatalf("unexpected svg width: %v", w)
	}
	ys := map[string]bool{}
	for _, m := range regexp.MustCompile(`<text x="\d+" y="(\d+)" font-size="10"`).FindAllStringSubmatch(got, -1) {
		ys[m[1]] = true
	}
	if len(ys) < 2 {
		t.Errorf("legend did not wrap onto more than one row: %s", got)
	}
}

func TestHeatmapTruth_Deterministic(t *testing.T) {
	rows, opts := mixedTruthFixture()
	if Heatmap(rows, opts) != Heatmap(rows, opts) {
		t.Error("truth heatmap is non-deterministic")
	}
}

func TestHeatmapValidate(t *testing.T) {
	rows, opts := mixedTruthFixture()
	err := opts.Validate(rows)
	if err == nil {
		t.Fatal("mixed fixture carries a non-finite value and an unrecognized state; Validate accepted it")
	}
	for _, want := range []string{"row 2, column 3 value is not finite", `row 3, column 2 has unrecognized state "stale"`, "row 1, column 3 is declared zero but carries 8"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate omitted %q: %v", want, err)
		}
	}
	bad := HeatmapOptions{Role: "evil;", FrontierRole: "Bad Role", Min: 5, Max: 1}
	err = bad.Validate(nil)
	for _, want := range []string{`Role "evil;"`, `FrontierRole "Bad Role"`, "Min 5 exceeds Max 1"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate omitted %q: %v", want, err)
		}
	}
	good := HeatmapOptions{States: [][]HeatmapCell{{{State: CellZero}, {State: CellMissing, Reason: "not compiled"}}}}
	if err := good.Validate([][]float64{{0, 7}}); err != nil {
		t.Errorf("valid truth input refused: %v", err)
	}
	if err := (HeatmapOptions{}).Validate([][]float64{{1, math.NaN()}}); err != nil {
		t.Errorf("legacy NaN hole refused: %v", err)
	}
	var joined interface{ Unwrap() []error }
	if !errors.As(opts.Validate(rows), &joined) || len(joined.Unwrap()) != 3 {
		t.Errorf("Validate should report each problem separately")
	}
}

// The state marks carry meaning, so their roles must clear the kit's 3:1
// meaning floor on the surfaces a chart sits on, in both schemes; the "?"
// glyph is text and must clear 4.5:1. Measured on the default token set.
func TestHeatmapTruth_MarkRolesClearTheMeaningFloor(t *testing.T) {
	src, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"light", "dark"} {
		for _, bg := range []string{"surface", "surface-raised"} {
			for fg, floor := range map[string]float64{"ink-muted": tokens.MeaningFloor, "danger": tokens.MeaningFloor, "ink": tokens.TextFloor} {
				r, err := tokens.ContrastRoles(tok, fg, bg, scheme)
				if err != nil {
					t.Fatal(err)
				}
				if r < floor {
					t.Errorf("%s on %s (%s) = %.2f:1, below %.1f:1", fg, bg, scheme, r, floor)
				}
			}
		}
	}
}

func assertWellFormedSVG(t *testing.T, s string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("svg is not well-formed XML: %v\n%s", err, s)
		}
	}
}

// Large cells scale their hatch spacing and glyphs, and plate the "?" so it
// stays legible over the cross-hatch; values print when ShowValues is set.
func TestHeatmapTruth_LargeCellGolden(t *testing.T) {
	rows, opts := mixedTruthFixture()
	opts.Cell, opts.ShowValues = 56, true
	got := Heatmap(rows, opts)
	assertGolden(t, got, "heatmap_truth_large")
	assertWellFormedSVG(t, got)
	for _, want := range []string{
		`fill="var(--ak-color-surface)"/><text x="98" y="99" text-anchor="middle" font-size="13" font-weight="600" fill="var(--ak-color-ink)">?</text>`,
		`font-size="12" fill="var(--ak-color-ink)">4</text>`,
		`font-size="12" fill="var(--ak-color-ink)">0</text>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("large-cell rendering missing %q", want)
		}
	}
}

// cellMarks returns each grid cell's drawing (its ramp rect plus any value or
// zero label) with the hover title and x coordinates removed, so two cells can
// be compared by what a sighted reader sees. Legend swatches carry no title
// and are excluded.
func cellMarks(svg string) []string {
	cell := regexp.MustCompile(`<rect x="\d+" ([^>]*)><title>[^<]*</title></rect>(?:<text x="\d+" ([^>]*>[^<]*)</text>)?`)
	var marks []string
	for _, m := range cell.FindAllStringSubmatch(svg, -1) {
		marks = append(marks, m[1]+" | "+m[2])
	}
	return marks
}

// A small nonzero value must never read as a measured zero: with ShowValues
// its label, and in every mode its title, keep enough precision to be nonzero
// (review finding: 0.004 printed "0" and -0.004 printed "-0").
func TestHeatmapTruth_SmallValuesNeverPrintAsZero(t *testing.T) {
	for _, v := range []float64{0.004, -0.004} {
		name := strconv.FormatFloat(v, 'f', -1, 64)
		got := Heatmap([][]float64{{0, v, 1}}, HeatmapOptions{TruthStates: true, ShowValues: true})
		marks := cellMarks(got)
		if len(marks) != 3 {
			t.Fatalf("cell marks = %d, want 3: %s", len(marks), got)
		}
		if marks[1] == marks[0] {
			t.Errorf("value %s is drawn exactly like the measured zero: %s", name, marks[0])
		}
		if !strings.HasSuffix(marks[1], ">"+name) {
			t.Errorf("value %s label = %q, want %s", name, marks[1], name)
		}
		if !strings.Contains(got, "<title>row 1, column 2: "+name+"</title>") {
			t.Errorf("value %s title does not name it: %s", name, got)
		}
		if strings.Contains(got, ">-0<") || strings.Contains(got, "-0–") || strings.Count(got, ">0</text>") != 2 {
			t.Errorf("only the measured zero (cell and legend swatch) may print 0: %s", got)
		}
	}
	// The legend range and the summary keep a nonzero bound nonzero too.
	tiny := Heatmap([][]float64{{0.004, 1}}, HeatmapOptions{TruthStates: true})
	for _, want := range []string{">value 0.004–1</text>", "(range 0.004–1)", "<title>row 1, column 1: 0.004</title>"} {
		if !strings.Contains(tiny, want) {
			t.Errorf("small lower bound lost, missing %q: %s", want, tiny)
		}
	}
	// A negative-zero bound is still the number 0.
	if negZero := Heatmap([][]float64{{1, 2}}, HeatmapOptions{TruthStates: true, Min: math.Copysign(0, -1), Max: 2}); !strings.Contains(negZero, ">value 0–2</text>") {
		t.Errorf("negative-zero bound printed as -0: %s", negZero)
	}
	// Two-decimal values are unchanged.
	if plain := Heatmap([][]float64{{0.5, 12.25}}, HeatmapOptions{TruthStates: true, ShowValues: true}); !strings.Contains(plain, ">0.5</text>") || !strings.Contains(plain, ">12.25</text>") {
		t.Errorf("ordinary values changed format: %s", plain)
	}
	err := HeatmapOptions{States: [][]HeatmapCell{{{State: CellZero}}}, Min: 0.004, Max: 0.001}.Validate([][]float64{{0.004}})
	for _, want := range []string{"declared zero but carries 0.004", "Min 0.004 exceeds Max 0.001"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate rounded a nonzero number to 0, missing %q: %v", want, err)
		}
	}
}

// A grid whose measured cells are all zero has no value span. Its zeros must
// sit at the ramp floor, not mid-ramp, or cells too small for the 0 glyph look
// exactly like a grid of equal positive values (review finding).
func TestHeatmapTruth_AllZeroGridSitsAtTheRampFloor(t *testing.T) {
	zeros := Heatmap([][]float64{{0, 0}}, HeatmapOptions{TruthStates: true, Cell: 12})
	fives := Heatmap([][]float64{{5, 5}}, HeatmapOptions{TruthStates: true, Cell: 12})
	zm, fm := cellMarks(zeros), cellMarks(fives)
	if len(zm) != 2 || len(fm) != 2 {
		t.Fatalf("cell marks: zeros %d, fives %d, want 2 each", len(zm), len(fm))
	}
	if zm[0] == fm[0] {
		t.Errorf("an all-zero grid is drawn like a grid of equal positive values: %s", zm[0])
	}
	if n := strings.Count(zeros, `fill-opacity="0.12"`); n != 3 {
		t.Errorf("zero cells and the legend zero swatch must use the ramp floor 0.12 (want 3, got %d): %s", n, zeros)
	}
	if strings.Contains(zeros, `fill-opacity="0.54"`) {
		t.Errorf("an all-zero grid drew mid-ramp fills: %s", zeros)
	}
	// Equal nonzero values keep the original mid-ramp placement.
	if !strings.Contains(fives, `fill-opacity="0.54"><title>row 1, column 1: 5</title>`) {
		t.Errorf("equal values moved off mid-ramp: %s", fives)
	}
}

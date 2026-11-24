package view_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// outlineBarPercent is the HLP-2 bar: the median of outline bytes over file bytes, per dialect, at or under this
// many percent.
const outlineBarPercent = 20.0

// TestOutlineCorpusReading is the HLP-2 measurement. It is opt-in like TestCorpusReading and shares its loader
// (corpusSubjects, forEachMeasuredFile): it skips unless FATHOM_LP8_CORPUS names the manifest. For every selected
// file with at least one symbol it reads the file with the reader and measures the bytes of the text-form outline
// (view.Outline with no MaxNodes, then Text) divided by the bytes of the file. It prints, per dialect, the file
// count, the median and the 90th percentile of that ratio as percent, and whether the median meets the bar, as one
// deterministic JSON aggregate and its SHA-256. No path from the manifest reaches the output.
//
// A measured file of zero bytes has no ratio. It is counted in files and in empty_files_not_in_ratios, and it is
// left out of the median, the 90th percentile and the other ratio statistics.
//
// The median of an even count is the mean of the two middle values. The 90th percentile is the nearest rank:
// the value at position ceil(0.9 n) of the sorted ratios.
//
//	FATHOM_LP8_CORPUS=<manifest.json> go test -race -count=1 -run TestOutlineCorpusReading -v ./pkg/lang/view
func TestOutlineCorpusReading(t *testing.T) {
	subjects := corpusSubjects(t)
	rd, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	type stats struct {
		Files        int     `json:"files"`
		EmptyFiles   int     `json:"empty_files_not_in_ratios"`
		SourceBytes  int     `json:"source_bytes"`
		OutlineBytes int     `json:"outline_bytes"`
		MedianPct    float64 `json:"median_percent"`
		P90Pct       float64 `json:"p90_percent"`
		MaxPct       float64 `json:"max_percent"`
		Over20       int     `json:"files_over_20_percent"`
		Incomplete   int     `json:"incomplete_files"`
		Omitted      int     `json:"omitted_nodes"`
		Verdict      string  `json:"verdict"`
	}
	type subjectRow struct {
		Subject    string  `json:"subject"`
		Commit     string  `json:"commit"`
		Selected   int     `json:"selected_files"`
		Files      int     `json:"files"`
		EmptyFiles int     `json:"empty_files_not_in_ratios"`
		MedianPct  float64 `json:"median_percent"`
		P90Pct     float64 `json:"p90_percent"`
	}
	ratios := map[string][]float64{}
	dialects := map[string]*stats{}
	var rows []subjectRow
	for _, s := range subjects {
		st := dialects[s.Dialect]
		if st == nil {
			st = &stats{}
			dialects[s.Dialect] = st
		}
		var own []float64
		empty := 0
		selected := forEachMeasuredFile(t, rd, s, func(rel string, src []byte, res lang.Result) {
			doc := view.Outline(rel, res, view.OutlineOptions{})
			text := doc.Text()
			// The text form is one line per node: nodes plus the header and the footer.
			if lines, nodes := strings.Count(text, "\n"), res.Symbols; lines != len(nodes)+2 {
				t.Fatalf("%s: %d text lines for %d nodes", rel, lines, len(nodes))
			}
			if len(src) == 0 {
				// A file of no bytes can have a symbol (a module). Its ratio is undefined, so it is counted
				// and left out of the ratios.
				st.Files++
				st.EmptyFiles++
				empty++
				return
			}
			ratio := 100 * float64(len(text)) / float64(len(src))
			own = append(own, ratio)
			ratios[s.Dialect] = append(ratios[s.Dialect], ratio)
			st.Files++
			st.SourceBytes += len(src)
			st.OutlineBytes += len(text)
			st.Omitted += doc.Omitted
			if res.Incomplete {
				st.Incomplete++
			}
			if ratio > outlineBarPercent {
				st.Over20++
			}
			if ratio > st.MaxPct {
				st.MaxPct = ratio
			}
		})
		sort.Float64s(own)
		rows = append(rows, subjectRow{s.Subject, s.Commit, selected, len(own) + empty, empty, round4(median(own)), round4(nearestRank(own, 0.9))})
	}
	failed := []string{}
	for d, st := range dialects {
		r := ratios[d]
		sort.Float64s(r)
		st.MedianPct = round4(median(r))
		st.P90Pct = round4(nearestRank(r, 0.9))
		st.MaxPct = round4(st.MaxPct)
		st.Verdict = "PASS"
		if st.MedianPct > outlineBarPercent {
			st.Verdict = "FAIL"
			failed = append(failed, d)
		}
	}
	sort.Strings(failed)
	agg := struct {
		BarMedianPercent float64           `json:"bar_median_percent"`
		Dialects         map[string]*stats `json:"dialects"`
		Subjects         []subjectRow      `json:"subjects"`
		FailingDialects  []string          `json:"failing_dialects"`
	}{outlineBarPercent, dialects, rows, failed}
	out, err := json.MarshalIndent(agg, "", "  ") // map keys marshal sorted
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, '\n')
	sum := sha256.Sum256(out)
	fmt.Printf("%s", out)
	fmt.Printf("aggregate sha256: %s\n", hex.EncodeToString(sum[:]))
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func nearestRank(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	k := int(math.Ceil(p * float64(n)))
	if k < 1 {
		k = 1
	}
	return sorted[k-1]
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

// The statistics helpers are the measurement's own arithmetic, so they get a table test that always runs.
func TestOutlineCorpusStatistics(t *testing.T) {
	for _, c := range []struct {
		in          []float64
		median, p90 float64
	}{
		{nil, 0, 0},
		{[]float64{5}, 5, 5},
		{[]float64{1, 3}, 2, 3},
		{[]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 5.5, 9},
		{[]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, 6, 10},
	} {
		if m, p := median(c.in), nearestRank(c.in, 0.9); m != c.median || p != c.p90 {
			t.Errorf("%v: median %v p90 %v, want %v %v", c.in, m, p, c.median, c.p90)
		}
	}
}

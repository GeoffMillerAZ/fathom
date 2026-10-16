package analytics

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func TestMissingReasonRetainsReadableStateColumnContract(t *testing.T) {
	renderers := map[string]func(Dataset, viz.BarsOptions, time.Time) (string, error){
		"bars":      RenderBars,
		"diverging": RenderDivergingBars,
	}
	for name, render := range renderers {
		for _, linked := range []bool{false, true} {
			t.Run(name+"/linked="+strconv.FormatBool(linked), func(t *testing.T) {
				d := fixture()
				d.Contract.Completeness = Partial
				d.Contract.CoverageNote = "One source row is unavailable."
				d.Rows = []Row{
					{ID: "positive", Label: "Positive", Value: value(1250.5)},
					{ID: "zero", Label: "Zero", Value: value(0)},
					{ID: "missing", Label: "Missing", MissingReason: `Source <down> & "late"`},
				}
				if linked {
					for i := range d.Rows {
						d.Rows[i].DetailHref = "/details/" + d.Rows[i].ID
					}
				}

				h, err := render(d, viz.BarsOptions{}, testNow)
				if err != nil {
					t.Fatal(err)
				}
				missingCell := `<td class="min-w-48 max-w-xs px-2 py-2 [overflow-wrap:anywhere]">`
				if strings.Count(h, missingCell) != 1 {
					t.Fatalf("missing reason cell width contract count=%d, want 1", strings.Count(h, missingCell))
				}
				for _, want := range []string{">1250.5</td>", ">0</td>", ">Missing</td>", `Source &lt;down&gt; &amp; &#34;late&#34;`} {
					if !strings.Contains(h, want) {
						t.Errorf("rendered output lost %q", want)
					}
				}
				if got := strings.Count(h, `href="/details/`); got != map[bool]int{false: 0, true: 3}[linked] {
					t.Errorf("detail link count=%d", got)
				}

				expected := d
				expected.Rows = append([]Row(nil), d.Rows...)
				for i := range expected.Rows {
					expected.Rows[i].DetailHref = ""
				}
				gotCSV, err := detailCSVBytes(d)
				if err != nil {
					t.Fatal(err)
				}
				wantCSV, err := detailCSVBytes(expected)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotCSV, wantCSV) {
					t.Fatal("state width or detail links changed CSV")
				}
			})
		}
	}

	allPresent := fixture()
	for name, render := range renderers {
		h, err := render(allPresent, viz.BarsOptions{}, testNow)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(h, "min-w-48") {
			t.Errorf("%s added the missing-reason width class to an all-present table", name)
		}
	}
}

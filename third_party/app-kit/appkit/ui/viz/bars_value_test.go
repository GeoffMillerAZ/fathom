package viz

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"
)

// This pins the deterministic reservation, not actual font geometry. The
// standalone analytics Gallery browser probe separately measures glyph bounds.
func TestBarsValueColumnIncludesUnitsAndLargeNumbers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   float64
		suffix  string
		minimum float64
	}{
		{"count with units", 4, " failed operations", 150},
		{"maximum supported analytics amount", 1e12, " USD", 220},
		{"wide Unicode units", 42, " 件数", 70},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markup := Bars([]Bar{{Label: "Known selection", Value: tc.value, Suffix: tc.suffix}}, BarsOptions{W: 560, LabelW: 180, ShowValue: true})
			d := xml.NewDecoder(strings.NewReader(markup))
			tracks := 0
			for {
				token, err := d.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				e, ok := token.(xml.StartElement)
				if !ok || e.Name.Local != "rect" {
					continue
				}
				attrs := map[string]string{}
				for _, attr := range e.Attr {
					attrs[attr.Name.Local] = attr.Value
				}
				if attrs["fill-opacity"] != "0.5" {
					continue
				}
				tracks++
				x, err := strconv.ParseFloat(attrs["x"], 64)
				if err != nil {
					t.Fatal(err)
				}
				w, err := strconv.ParseFloat(attrs["width"], 64)
				if err != nil {
					t.Fatal(err)
				}
				if reserve := 560 - x - w; reserve < tc.minimum || w <= 0 {
					t.Fatalf("value column reserves %.1f, need at least %.1f and a positive bar track (%.1f)", reserve, tc.minimum, w)
				}
			}
			if tracks != 1 {
				t.Fatalf("expected one track, got %d", tracks)
			}
		})
	}
}

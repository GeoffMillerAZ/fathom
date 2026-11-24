package view_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// TestRowsCorpusReading is the HLP-3 measurement. It is opt-in like TestCorpusReading (it shares that test's
// corpusSubject, gitOut and selectFiles): it skips unless FATHOM_LP8_CORPUS names the manifest. For every selected
// file with at least one symbol it reads the file with the reader, exports it as rows and rebuilds the nested
// document from the rows, and requires the rebuilt bytes to equal the nested bytes. It prints the row counts per
// kind and the lossless result as one deterministic JSON aggregate and its SHA-256. No path from the manifest
// reaches the output.
//
//	FATHOM_LP8_CORPUS=<manifest.json> go test -race -count=1 -run TestRowsCorpusReading -v ./pkg/lang/view
func TestRowsCorpusReading(t *testing.T) {
	subjects := corpusSubjects(t)
	rd, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	type counts struct {
		Files      int `json:"files"`
		File       int `json:"file_rows"`
		Symbol     int `json:"symbol_rows"`
		Reference  int `json:"reference_rows"`
		Unresolved int `json:"unresolved_rows"`
		Fact       int `json:"fact_rows"`
		Note       int `json:"note_rows"`
		Lossless   int `json:"lossless_files"`
		Lossy      int `json:"lossy_files"`
	}
	type subjectRow struct {
		Subject  string `json:"subject"`
		Commit   string `json:"commit"`
		Selected int    `json:"selected_files"`
		counts
	}
	var total counts
	var perSubject []subjectRow
	perDialect := map[string]*counts{}
	add := func(c *counts, rows []view.Row) {
		for _, r := range rows {
			switch r.Kind {
			case view.KindFile:
				c.File++
			case view.KindSymbol:
				c.Symbol++
			case view.KindReference:
				c.Reference++
			case view.KindUnresolved:
				c.Unresolved++
			case view.KindFact:
				c.Fact++
			case view.KindNote:
				c.Note++
			}
		}
	}
	for _, s := range subjects {
		sr := subjectRow{Subject: s.Subject, Commit: s.Commit}
		dc := perDialect[s.Dialect]
		if dc == nil {
			dc = &counts{}
			perDialect[s.Dialect] = dc
		}
		sr.Selected = forEachMeasuredFile(t, rd, s, func(rel string, src []byte, res lang.Result) {
			entry := []nestedEntry{{Path: rel, Result: &res}}
			rows := rowsOf(1, entry)
			var b bytes.Buffer
			if err := view.WriteJSONLines(&b, rows); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
			if bytes.Equal(rebuildNested(t, lines), encodeNested(t, 1, entry)) {
				sr.Lossless++
			} else {
				sr.Lossy++
			}
			sr.Files++
			add(&sr.counts, rows)
		})
		for _, c := range []*counts{&total, dc} {
			c.Files += sr.Files
			c.File += sr.File
			c.Symbol += sr.Symbol
			c.Reference += sr.Reference
			c.Unresolved += sr.Unresolved
			c.Fact += sr.Fact
			c.Note += sr.Note
			c.Lossless += sr.Lossless
			c.Lossy += sr.Lossy
		}
		perSubject = append(perSubject, sr)
	}
	agg := struct {
		Total       counts             `json:"total"`
		Dialects    map[string]*counts `json:"dialects"`
		Subjects    []subjectRow       `json:"subjects"`
		AllLossless bool               `json:"every_file_lossless"`
	}{total, perDialect, perSubject, total.Lossy == 0 && total.Files > 0}
	out, err := json.MarshalIndent(agg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, '\n')
	sum := sha256.Sum256(out)
	fmt.Printf("%s", out)
	fmt.Printf("aggregate sha256: %s\n", hex.EncodeToString(sum[:]))
	if !agg.AllLossless {
		t.Errorf("%d measured files were not lossless", total.Lossy)
	}
}

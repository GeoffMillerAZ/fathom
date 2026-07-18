// Package xlsx provides a docextract.Handler for .xlsx spreadsheets, rendering
// each sheet as tab-separated rows under a "# Sheet: <name>" heading. Importing
// it pulls the spreadsheet dependency; the core docextract package stays dep-free.
//
//	dx.Extract(name, mt, data, dx.WithHandlers(xlsx.Handler))
package xlsx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	docextract "github.com/geoffmilleraz/agentic-kit/kits/docextract"
)

const mediaType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Handler extracts sheet text from .xlsx bytes.
var Handler = docextract.Handler{
	Name: "xlsx",
	Match: func(ext, mt string) bool {
		return ext == ".xlsx" || mt == mediaType
	},
	Extract: extract,
}

func extract(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("docextract/xlsx: read: %w", err)
	}
	defer f.Close()
	var b strings.Builder
	for _, sheet := range f.GetSheetList() {
		b.WriteString("# Sheet: " + sheet + "\n")
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			b.WriteString(strings.Join(row, "\t") + "\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

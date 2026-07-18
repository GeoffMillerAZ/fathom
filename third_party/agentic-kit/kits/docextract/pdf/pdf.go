// Package pdf provides a docextract.Handler for PDF documents. Importing it pulls
// the PDF text-extraction dependency; the core docextract package stays dep-free.
//
//	dx.Extract(name, mt, data, dx.WithHandlers(pdf.Handler))
package pdf

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"

	docextract "github.com/geoffmilleraz/agentic-kit/kits/docextract"
)

// Handler extracts plain text from PDF bytes.
var Handler = docextract.Handler{
	Name: "pdf",
	Match: func(ext, mediaType string) bool {
		return ext == ".pdf" || mediaType == "application/pdf"
	},
	Extract: extract,
}

func extract(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docextract/pdf: read: %w", err)
	}
	tr, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("docextract/pdf: text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, tr); err != nil {
		return "", fmt.Errorf("docextract/pdf: copy: %w", err)
	}
	return buf.String(), nil
}

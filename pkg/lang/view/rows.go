package view

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// RowsVersion is the version of the row shape. Provisional (CH-D6).
const RowsVersion = 1

// RowKind names a kind of row. It is the value of a row's "row" field.
type RowKind string

// The closed set of row kinds.
const (
	KindHeader     RowKind = "header"
	KindFile       RowKind = "file"
	KindSymbol     RowKind = "symbol"
	KindReference  RowKind = "reference"
	KindUnresolved RowKind = "unresolved"
	KindFact       RowKind = "fact"
	KindNote       RowKind = "note"
)

// FileRow is the per-file row: what the nested document holds for a file apart from its arrays, and what is
// needed to rebuild the nested entry exactly. HasResult is false when the nested entry's result is null (the
// file could not be read): its Error says why and the other fields are zero. NullArrays names the Result arrays
// that are null, not empty, in the nested document, from symbols, references, unresolved, facts and notes.
type FileRow struct {
	Error      string
	Dialect    string
	Language   string
	Skipped    string
	Level      string
	Incomplete bool
	HasResult  bool
	NullArrays []string
}

// HeaderRow is the first row of an export.
type HeaderRow struct {
	RowsVersion   int
	SchemaVersion int
}

// Row is one line of the row export. Kind says which of the other fields it holds: Header for KindHeader;
// Path and File for KindFile; Path, I and one of Symbol, Reference, Unresolved, Fact or Note for the rest. I is
// the zero-based position in the Result's array. Its JSON form is MarshalJSON's.
type Row struct {
	Kind       RowKind
	Path       string
	I          int
	Header     HeaderRow
	File       FileRow
	Symbol     lang.Symbol
	Reference  lang.Reference
	Unresolved lang.Unresolved
	Fact       lang.Fact
	Note       string
}

// Header returns the first row of an export. schemaVersion is the schema_version of the nested extract document
// the rows mirror.
func Header(schemaVersion int) Row {
	return Row{Kind: KindHeader, Header: HeaderRow{RowsVersion: RowsVersion, SchemaVersion: schemaVersion}}
}

// Rows returns the rows of one file: its file row, then its symbols, references, unresolved references, facts
// and notes, each in array order. errText is the error the nested document shows for the file. When it is not
// empty the file has no result: Rows returns the file row alone, with HasResult false, and ignores r. The rows
// hold copies of r's values, so a later change to r changes no row already returned.
func Rows(path string, errText string, r lang.Result) []Row {
	if errText != "" {
		return []Row{{Kind: KindFile, Path: path, File: FileRow{Error: errText, NullArrays: []string{}}}}
	}
	null := []string{}
	for _, a := range []struct {
		name string
		null bool
	}{{"symbols", r.Symbols == nil}, {"references", r.References == nil}, {"unresolved", r.Unresolved == nil}, {"facts", r.Facts == nil}, {"notes", r.Notes == nil}} {
		if a.null {
			null = append(null, a.name)
		}
	}
	rows := make([]Row, 0, 1+len(r.Symbols)+len(r.References)+len(r.Unresolved)+len(r.Facts)+len(r.Notes))
	rows = append(rows, Row{Kind: KindFile, Path: path, File: FileRow{
		Dialect: r.Dialect, Language: r.Language, Skipped: r.Skipped, Level: r.Level,
		Incomplete: r.Incomplete, HasResult: true, NullArrays: null,
	}})
	for i, s := range r.Symbols {
		rows = append(rows, Row{Kind: KindSymbol, Path: path, I: i, Symbol: s})
	}
	for i, s := range r.References {
		rows = append(rows, Row{Kind: KindReference, Path: path, I: i, Reference: s})
	}
	for i, s := range r.Unresolved {
		rows = append(rows, Row{Kind: KindUnresolved, Path: path, I: i, Unresolved: s})
	}
	for i, s := range r.Facts {
		rows = append(rows, Row{Kind: KindFact, Path: path, I: i, Fact: s})
	}
	for i, s := range r.Notes {
		rows = append(rows, Row{Kind: KindNote, Path: path, I: i, Note: s})
	}
	return rows
}

// The wire forms. A row's struct fields are the matching lang struct, embedded, so a field added to lang.Symbol
// or its siblings is in the rows with the JSON name it already has.
type (
	headerLine struct {
		Row           RowKind `json:"row"`
		RowsVersion   int     `json:"rows_version"`
		SchemaVersion int     `json:"schema_version"`
	}
	fileLine struct {
		Row        RowKind  `json:"row"`
		Path       string   `json:"path"`
		Error      string   `json:"error"`
		Dialect    string   `json:"dialect"`
		Language   string   `json:"language"`
		Skipped    string   `json:"skipped"`
		Level      string   `json:"level"`
		Incomplete bool     `json:"incomplete"`
		HasResult  bool     `json:"has_result"`
		NullArrays []string `json:"null_arrays"`
	}
	symbolLine struct {
		Row  RowKind `json:"row"`
		Path string  `json:"path"`
		I    int     `json:"i"`
		lang.Symbol
	}
	referenceLine struct {
		Row  RowKind `json:"row"`
		Path string  `json:"path"`
		I    int     `json:"i"`
		lang.Reference
	}
	unresolvedLine struct {
		Row  RowKind `json:"row"`
		Path string  `json:"path"`
		I    int     `json:"i"`
		lang.Unresolved
	}
	factLine struct {
		Row  RowKind `json:"row"`
		Path string  `json:"path"`
		I    int     `json:"i"`
		lang.Fact
	}
	noteLine struct {
		Row  RowKind `json:"row"`
		Path string  `json:"path"`
		I    int     `json:"i"`
		Note string  `json:"note"`
	}
)

// MarshalJSON is the stable encoding of a row: one compact JSON object whose fields come in the order of the
// row kind's shape, with the same string escaping as the nested document. Equal rows give equal bytes.
func (r Row) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case KindHeader:
		return json.Marshal(headerLine{r.Kind, r.Header.RowsVersion, r.Header.SchemaVersion})
	case KindFile:
		f := r.File
		null := f.NullArrays
		if null == nil {
			null = []string{}
		}
		return json.Marshal(fileLine{r.Kind, r.Path, f.Error, f.Dialect, f.Language, f.Skipped, f.Level, f.Incomplete, f.HasResult, null})
	case KindSymbol:
		return json.Marshal(symbolLine{r.Kind, r.Path, r.I, r.Symbol})
	case KindReference:
		return json.Marshal(referenceLine{r.Kind, r.Path, r.I, r.Reference})
	case KindUnresolved:
		return json.Marshal(unresolvedLine{r.Kind, r.Path, r.I, r.Unresolved})
	case KindFact:
		return json.Marshal(factLine{r.Kind, r.Path, r.I, r.Fact})
	case KindNote:
		return json.Marshal(noteLine{r.Kind, r.Path, r.I, r.Note})
	}
	return nil, &json.UnsupportedValueError{Str: "unknown row kind " + string(r.Kind)}
}

// WriteJSONLines writes each row as its MarshalJSON line followed by a newline: JSON Lines. It is what
// fathom-lang extract -rows prints.
func WriteJSONLines(w io.Writer, rows []Row) error {
	var buf bytes.Buffer
	for _, r := range rows {
		b, err := r.MarshalJSON()
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	_, err := w.Write(buf.Bytes())
	return err
}

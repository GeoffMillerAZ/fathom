package main

import (
	"embed"
	"errors"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// writeRows prints extract's files as JSON Lines: the header, then view.Rows of each file in the order the nested
// document lists them. It is the one place the binary shapes rows, so a Go consumer of view.Rows gets the same lines.
func writeRows(out io.Writer, files []fileResult) error {
	rows := []view.Row{view.Header(schemaVersion)}
	for _, f := range files {
		var r lang.Result
		errText := f.Error
		if f.Result != nil {
			r = *f.Result
		} else if errText == "" {
			errText = "no result"
		}
		rows = append(rows, view.Rows(f.Path, errText, r)...)
	}
	return view.WriteJSONLines(out, rows)
}

// schemaFS holds the JSON Schema sources, beside this command. The same files are published under
// docs/schema/fathom-lang, and a test fails when the two copies differ.
//
//go:embed schemas/*.schema.json
var schemaFS embed.FS

// cmdSchema prints the JSON Schema (draft 2020-12) of one output shape, embedded in the binary so a consumer gets
// the schema of the version it runs. Provisional (CH-D6).
func cmdSchema(args []string, out io.Writer) error {
	if len(args) != 1 {
		return usageError{errors.New("schema takes one name: extract, rows, at or outline")}
	}
	switch name := args[0]; name {
	case "extract", "rows", "at":
		b, err := schemaFS.ReadFile("schemas/" + name + ".schema.json")
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	case "outline":
		return fmt.Errorf("schema outline is not yet available: the outline shape does not exist yet")
	default:
		return usageError{fmt.Errorf("schema %q: want extract, rows, at or outline", name)}
	}
}

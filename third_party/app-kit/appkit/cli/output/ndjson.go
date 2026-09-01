package output

import (
	"encoding/json"
	"io"
)

// ndjsonRow fixes the emitted key order (id, status, detail, remedy) via
// struct field order — encoding/json marshals struct fields in declaration
// order, never sorts them the way it would a map, so this is the one
// source of truth for the wire shape.
type ndjsonRow struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy"`
}

// WriteNDJSON writes one JSON object per row, newline-delimited — a status
// command's --json contract. Status is written exactly as given (not
// upper-cased the way RenderStatusTable renders it) so machine consumers
// see whatever the caller's Status values already are.
func WriteNDJSON(w io.Writer, rows []StatusRow) error {
	enc := json.NewEncoder(w)
	for _, r := range rows {
		row := ndjsonRow{ID: r.ID, Status: r.Status, Detail: r.Detail, Remedy: r.Remedy}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// Package records implements standalone lane validation and pure record allocation.
package records

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/geoffmilleraz/fathom/internal/recordids"
)

type IDRange = recordids.IDRange
type Lane = recordids.LaneDeclaration

// ParseLaneJSON rejects unknown fields and validates the standalone lane shape.
func ParseLaneJSON(raw []byte) (Lane, error) {
	var l Lane
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&l); e != nil {
		return l, recordids.Refuse(recordids.ErrShape, e.Error())
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return Lane{}, recordids.Refuse(recordids.ErrShape, "expected exactly one lane")
	}
	if l.Rows == nil || l.OwnedPaths == nil || l.ReadOnlyPaths == nil || l.ContractFiles == nil || l.IDRanges == nil {
		return Lane{}, recordids.Refuse(recordids.ErrShape, "lane lists must be explicit arrays")
	}
	return l, l.Validate()
}

package benchledger

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func rowFixture(t *testing.T) CellRow {
	t.Helper()
	in := inputs()
	k, e := CellKey(in)
	if e != nil {
		t.Fatal(e)
	}
	p := pair(t)
	o, e := ProjectCell(cell("treatment"), "treatment", p)
	if e != nil {
		t.Fatal(e)
	}
	return CellRow{Schema: Schema, Key: k, Inputs: in, Outcome: o, Preflight: p}
}
func TestLedgerRoundTrip(t *testing.T) {
	row := rowFixture(t)
	var b bytes.Buffer
	if e := AppendCell(&b, nil, row); e != nil {
		t.Fatal(e)
	}
	raw := b.String()
	if strings.Count(raw, "\n") != 1 || !strings.HasSuffix(raw, "\n") || strings.Contains(raw, "\r") {
		t.Fatal(raw)
	}
	if strings.Contains(raw, "CorpusSHA256") || !strings.Contains(raw, `"corpus_sha256"`) {
		t.Fatal(raw)
	}
	got, e := ReadCells(strings.NewReader(raw))
	if e != nil || len(got) != 1 || got[0] != row {
		t.Fatalf("%+v %v", got, e)
	}
	empty, e := ReadCells(strings.NewReader(""))
	if e != nil || empty == nil {
		t.Fatalf("%v %v", empty, e)
	}
	for _, bad := range []string{"{}\n", "{\n", strings.Replace(raw, Schema, "unknown", 1), strings.Replace(raw, row.Key, "wrong", 1), raw + raw, strings.TrimSuffix(raw, "\n") + " garbage\n", strings.Replace(raw, `"schema":`, `"unknown":1,"schema":`, 1)} {
		_, e := ReadCells(strings.NewReader(bad))
		refusal(t, e)
		if !strings.Contains(e.Error(), "line ") {
			t.Fatal(e)
		}
	}
	_, e = ReadCells(strings.NewReader(raw + "{\n"))
	if e == nil || !strings.Contains(e.Error(), "line 2") {
		t.Fatal(e)
	}
}
func TestLedgerValidatesAllRowsBeforeWriting(t *testing.T) {
	row := rowFixture(t)
	for _, mutate := range []func(*CellRow){func(r *CellRow) { r.Schema = "bad" }, func(r *CellRow) { r.Key = "wrong" }, func(r *CellRow) { r.Outcome.TaskID = "other" }, func(r *CellRow) { r.Outcome.Trial = 2 }, func(r *CellRow) { r.Outcome.State = CellVoid }, func(r *CellRow) { r.Preflight.Valid = false }, func(r *CellRow) { r.Outcome.SurfaceCalls = -1 }} {
		bad := row
		mutate(&bad)
		var b bytes.Buffer
		refusal(t, AppendCell(&b, nil, bad))
		refusal(t, AppendCell(&b, []CellRow{bad}, row))
		if b.Len() != 0 {
			t.Fatal("wrote before validation")
		}
	}
	var b bytes.Buffer
	refusal(t, AppendCell(&b, []CellRow{row, row}, row))
	if b.Len() != 0 {
		t.Fatal("duplicate write")
	}
	row2 := row
	row2.Inputs.Trial = 2
	row2.Outcome.Trial = 2
	row2.Key, _ = CellKey(row2.Inputs)
	if e := AppendCell(&b, []CellRow{row}, row2); e != nil {
		t.Fatal(e)
	}
	if e := AppendCell(failWriter{}, nil, row); e != io.ErrClosedPipe {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(row)
	raw = bytes.Replace(raw, []byte(`"valid":true`), []byte(`"valid":false`), 1)
	_, e := ReadCells(bytes.NewReader(raw))
	refusal(t, e)
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestLedgerRequiresCompleteWireShape(t *testing.T) {
	raw, e := json.Marshal(rowFixture(t))
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ old, new string }{
		{`"oracle_passed":false,`, ``},
		{`"success":false`, `"success":false,"success":true`},
		{`"surface_calls":0`, `"surface_calls":null`},
		{`"valid":true`, `"Valid":true`},
		{`"arm_surface":`, `"ArmSurface":`},
		{`"success":false`, `"success":false,"raw_path":"canary"`},
	} {
		bad := strings.Replace(string(raw), tc.old, tc.new, 1)
		if bad == string(raw) {
			t.Fatal("fixture mutation missed")
		}
		_, e := ReadCells(strings.NewReader(bad))
		refusal(t, e)
	}
}

package syslink

import (
	"strings"
	"testing"
)

type fakeSource struct {
	id    string
	epoch int
	rows  []KeyRow
}

func (f fakeSource) ID() string { return f.id }
func (f fakeSource) Epoch() int { return f.epoch }
func (f fakeSource) Keys(file string, _ []byte, _ string) []KeyRow {
	out := make([]KeyRow, len(f.rows))
	for i, r := range f.rows {
		r.File = file
		r.Extractor, r.ExtractorEpoch = "forged", 99 // the registry must overwrite these
		out[i] = r
	}
	return out
}

// TestContractSourceRegistry is SG-3's contract: sources register by
// dialect id (ADR-048), a bad registration is refused by name, lookups are
// sorted, and every row a source returns carries that source's extractor id
// and epoch, whatever the source itself wrote there.
func TestContractSourceRegistry(t *testing.T) {
	r := NewRegistry()
	a := fakeSource{id: "b-src", epoch: 2, rows: []KeyRow{{Key: mustKey("env:B"), Role: RoleConsumes, Line: 2}}}
	b := fakeSource{id: "a-src", epoch: 1, rows: []KeyRow{{Key: mustKey("env:A"), Role: RoleProvides, Line: 1}}}
	if err := r.RegisterContractSource("go", a); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterContractSource("go", b); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []struct {
		dialect string
		src     ContractSource
		want    string
	}{
		{"", a, "dialect"},
		{"go", nil, "nil"},
		{"go", fakeSource{id: "", epoch: 1}, "id"},
		{"go", fakeSource{id: "z", epoch: 0}, "epoch"},
		{"go", fakeSource{id: "a-src", epoch: 3}, "a-src"},
	} {
		err := r.RegisterContractSource(bad.dialect, bad.src)
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("RegisterContractSource(%q, %v) = %v, want refusal naming %q", bad.dialect, bad.src, err, bad.want)
		}
	}

	srcs := r.Sources("go")
	if len(srcs) != 2 || srcs[0].ID() != "a-src" || srcs[1].ID() != "b-src" {
		t.Fatalf("Sources(go) = %v, want [a-src b-src]", srcs)
	}
	if got := r.Sources("python"); len(got) != 0 {
		t.Fatalf("Sources(python) = %v, want none", got)
	}
	if got := r.Dialects(); len(got) != 1 || got[0] != "go" {
		t.Fatalf("Dialects() = %v", got)
	}

	rows := r.Keys("main.go", nil, "go")
	if len(rows) != 2 {
		t.Fatalf("Keys = %d rows, want 2", len(rows))
	}
	for _, row := range rows {
		want := map[string]int{"a-src": 1, "b-src": 2}
		if e, ok := want[row.Extractor]; !ok || e != row.ExtractorEpoch {
			t.Errorf("row %v: extractor %q epoch %d, want the registering source's id and epoch", row.Key, row.Extractor, row.ExtractorEpoch)
		}
		if row.File != "main.go" {
			t.Errorf("row %v: file %q", row.Key, row.File)
		}
	}
	if rows[0].Key.String() != "env:A" {
		t.Errorf("rows not sorted: %v", rows)
	}

}

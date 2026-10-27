package main

import (
	"bytes"
	"testing"
)

func TestLocationsRemainIndependentCAndKeepUnusedEntities(t *testing.T) {
	rows, err := locatedWritten([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("located count=%d", len(rows))
	}
	found := false
	for _, row := range rows {
		if row.Kind == "route" && row.Name == "flat" {
			found = true
			if row.Line != 10 || row.End < row.Line {
				t.Fatalf("independent range: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("flat route missing")
	}
}
func TestLocationSourceCapPlusOneRefusesWholeInput(t *testing.T) {
	if rows, err := locatedWritten(bytes.Repeat([]byte{'x'}, (1<<20)+1)); err == nil || rows != nil {
		t.Fatal("source cap should refuse")
	}
}

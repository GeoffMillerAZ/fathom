package main

import (
	"reflect"
	"testing"
)

func TestScanFlattensBlocksAndDropsComments(t *testing.T) {
	src := `// header
module "example.com/m" // trailing

require (
	example.com/a v1.0.0 // indirect
	// only a comment
	"example.com/b" v1.1.0
)
require example.com/c v1.2.0
replace(
	example.com/a v1.0.0 => ./a
)
replace example.com/b => example.com/fork v1.3.0
use ./x
`
	var got []string
	for _, d := range scan(src) {
		got = append(got, d.verb)
	}
	want := []string{"module", "require", "require", "require", "replace", "replace", "use"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verbs %v, want %v", got, want)
	}
	ds := scan(src)
	if ds[2].tokens[0] != "example.com/b" || ds[2].line != 7 {
		t.Errorf("a quoted require: %+v", ds[2])
	}
	if o := replaceOld(ds[4]); o != (modVersion{"example.com/a", "v1.0.0"}) {
		t.Errorf("replace old %+v", o)
	}
	if o := replaceOld(ds[5]); o != (modVersion{Path: "example.com/b"}) {
		t.Errorf("replace old %+v", o)
	}
	if firstLine(ds) != 2 {
		t.Errorf("first line %d", firstLine(ds))
	}
}

func TestIsDirectoryPath(t *testing.T) {
	for p, want := range map[string]bool{"./x": true, "../x": true, ".": true, "/abs": true, "C:\\x": true, "example.com/x": false, "x": false} {
		if isDirectoryPath(p) != want {
			t.Errorf("%q: got %v", p, !want)
		}
	}
}

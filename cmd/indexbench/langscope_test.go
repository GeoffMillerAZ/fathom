package main

import (
	"reflect"
	"testing"
)

func TestOnlyScopeNoteRoundTrip(t *testing.T) {
	for _, globs := range [][]string{{"src/*.lua"}, {"src/**/*.lua", "lib/*.lua", "init.lua"}} {
		note := onlyScopeNote("fixture", globs)
		got, ok := parseOnlyScopeNote("fixture", note)
		if !ok || !reflect.DeepEqual(got, globs) {
			t.Fatalf("round trip %q: %v, %v", note, got, ok)
		}
		if _, ok := parseOnlyScopeNote("other", note); ok {
			t.Fatal("accepted another subject's scope")
		}
	}
	if _, ok := parseOnlyScopeNote("fixture", "unrelated note"); ok {
		t.Fatal("accepted unrelated note")
	}
}

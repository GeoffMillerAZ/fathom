package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseMarkers: an include written in the measured file is the line
// before the marker that resumes it; a nested include, the compiler's
// pseudo-files and a system header outside the subject are not the file's
// own in-subject includes.
func TestParseMarkers(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"src/main.cpp", "src/a.h", "include/b.h"} {
		p := filepath.Join(root, f)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := strings.Join([]string{
		`# 1 "src/main.cpp"`,
		`# 1 "<built-in>" 1`,
		`# 1 "<built-in>" 3`,
		`# 1 "<command line>" 1`,
		`# 1 "<built-in>" 2`,
		`# 1 "src/main.cpp" 2`,
		`# 1 "src/./a.h" 1`,
		`# 1 "` + filepath.Join(root, "include", "b.h") + `" 1`, // nested: a.h includes b.h
		`int b;`,
		`# 2 "src/./a.h" 2`,
		`# 3 "src/main.cpp" 2`,
		`int x;`,
		`# 1 "/usr/include/stdio.h" 1 3`,
		`# 5 "src/main.cpp" 2`,
		`# 1 "` + filepath.Join(root, "include", "b.h") + `" 1`,
		`# 7 "src/main.cpp" 2`,
	}, "\n") + "\n"
	got, err := parseMarkers(strings.NewReader(out), root, "src/main.cpp")
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{Path: "src/main.cpp", Line: 2, ToPath: "src/a.h"},
		{Path: "src/main.cpp", Line: 4, ToPath: ""},
		{Path: "src/main.cpp", Line: 6, ToPath: "include/b.h"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

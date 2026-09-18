package main

import (
	"flag"
	"reflect"
	"testing"
)

func testFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.String("root", "", "")
	fs.Bool("force", false, "")
	return fs
}

func TestReorderFlagsFirst(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"flags already first", []string{"--root", "r", "dir"}, []string{"--root", "r", "dir"}},
		{"value flag after positional", []string{"dir", "--root", "r"}, []string{"--root", "r", "dir"}},
		{"bool flag after positional", []string{"dir", "--force"}, []string{"--force", "dir"}},
		{"bool then value flag after positional", []string{"dir", "--force", "--root", "r"}, []string{"--force", "--root", "r", "dir"}},
		{"equals form", []string{"dir", "--root=r"}, []string{"--root=r", "dir"}},
		{"unrecognized flag stays positional", []string{"dir", "--unknown"}, []string{"dir", "--unknown"}},
		{"double-dash stops reordering", []string{"dir", "--", "--root", "r"}, []string{"dir", "--", "--root", "r"}},
		{"negative-number-shaped token is positional", []string{"-1", "--root", "r"}, []string{"--root", "r", "-1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reorderFlagsFirst(testFlagSet(), c.args)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("reorderFlagsFirst(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

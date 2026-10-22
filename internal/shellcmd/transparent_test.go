package shellcmd

import (
	"reflect"
	"strings"
	"testing"
)

// AD-23's transparent-prefix part already exists; pin its tool subcommand
// forms alongside the other built-ins without adding another peeling rule.
func TestTransparentBuiltins(t *testing.T) {
	for _, src := range []string{
		`rtk go test ./...`, `rtk git status`, `rtk --verbose go test ./...`,
		`/usr/bin/rtk go test ./...`, `nice go test ./...`, `nice -n 3 go test ./...`,
		`timeout 20 go test ./...`, `env VAR=value go test ./...`,
		`command go test ./...`, `time go test ./...`,
	} {
		c := ParseCommand(src)
		if c.Unreadable || len(c.Units()) != 1 {
			t.Errorf("built-in not transparent: %q: %+v", src, c)
			continue
		}
		want := []string{"go", "test", "./..."}
		if strings.Contains(src, "git status") {
			want = []string{"git", "status"}
		}
		if got := c.Units()[0].Args(); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: argv=%q, want %q", src, got, want)
		}
		if strings.Contains(src, "rtk ") && !c.Rtk() {
			t.Errorf("%q: RTK provenance was lost", src)
		}
	}
}

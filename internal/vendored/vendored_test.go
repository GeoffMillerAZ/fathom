package vendored

import "testing"

func TestPathClassifiesTheFirstSegment(t *testing.T) {
	for rel, want := range map[string]bool{
		"third_party/x/CLAUDE.md": true,
		"vendor/github.com/a/b":   true,
		"node_modules/pkg":        true,
		"third_party":             true,
		"kits/a/third_party/cli":  false,
		"internal/fusion":         false,
		"":                        false,
	} {
		if got := Path(rel); got != want {
			t.Errorf("Path(%q) = %v, want %v", rel, got, want)
		}
	}
}

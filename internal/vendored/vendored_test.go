package vendored

import "testing"

// TestPathClassifiesAVendoredSegment: Path is repofacts' vendor rule, any
// depth (RF-2 unified the first-segment rule with search's any-segment
// one); a single segment reads as that directory, so the typed pass's
// per-segment vendoredDir answers the same.
func TestPathClassifiesAVendoredSegment(t *testing.T) {
	for rel, want := range map[string]bool{
		"third_party/x/CLAUDE.md": true,
		"vendor/github.com/a/b":   true,
		"node_modules/pkg":        true,
		"third_party":             true,
		"vendor":                  true,
		"kits/a/third_party/cli":  true,
		"web/node_modules/x/i.js": true,
		"internal/fusion":         false,
		"vendorish/x":             false,
		"":                        false,
	} {
		if got := Path(rel); got != want {
			t.Errorf("Path(%q) = %v, want %v", rel, got, want)
		}
	}
}

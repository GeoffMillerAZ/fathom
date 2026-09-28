package console

import (
	"reflect"
	"testing"
)

// WI-1 (docs/design/work-intelligence.md §2): a zone is a Go package's
// directory, or for other languages the deepest directory holding indexed
// symbols; `[zones]` globs override both; a change splits evenly across the
// distinct zones of the files it touched.
func TestZoneOfDefaultsAndOverrides(t *testing.T) {
	z := NewZones(nil, []string{"web", "web/src", "tools"})
	for file, want := range map[string]string{
		"internal/console/zones.go": "internal/console",
		"main.go":                   ".",
		"web/src/app/page.tsx":      "web/src", // deepest symbol dir above it
		"web/README.md":             "web",
		"docs/guide.md":             "docs", // no symbol dir above: its own directory
		"./tools/x.py":              "tools",
	} {
		if got := ZoneOf(file, z); got != want {
			t.Errorf("ZoneOf(%q) = %q, want %q", file, got, want)
		}
	}

	o := NewZones(map[string][]string{
		"payments": {"internal/pay/**", "cmd/payd/*"},
		"console":  {"internal/console/**"},
	}, nil)
	for file, want := range map[string]string{
		"internal/pay/ledger/ledger.go": "payments",
		"cmd/payd/main.go":              "payments",
		"internal/console/zones.go":     "console",
		"internal/store/store.go":       "internal/store", // no glob: the default
	} {
		if got := ZoneOf(file, o); got != want {
			t.Errorf("override ZoneOf(%q) = %q, want %q", file, got, want)
		}
	}
	// Two zones naming the same file: the first by name wins, deterministically.
	amb := NewZones(map[string][]string{"b": {"x/**"}, "a": {"x/**"}}, nil)
	if got := ZoneOf("x/y.go", amb); got != "a" {
		t.Errorf("overlapping globs resolve by zone name: got %q", got)
	}

	shares := ChangeZones([]string{"internal/console/a.go", "internal/console/b.go", "internal/store/s.go", "web/src/x.ts"}, z)
	want := []ZoneShare{{"internal/console", 1.0 / 3}, {"internal/store", 1.0 / 3}, {"web/src", 1.0 / 3}}
	if !reflect.DeepEqual(shares, want) {
		t.Fatalf("a change splits evenly across its distinct zones: %+v", shares)
	}
	if len(ChangeZones(nil, z)) != 0 {
		t.Fatal("no files, no zones")
	}
}

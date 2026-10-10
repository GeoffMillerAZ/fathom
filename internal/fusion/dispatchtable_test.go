package fusion

import (
	"strings"
	"testing"
)

// TestDispatchTable (LB-F356): the accessor answers from the registry New
// builds, for each kind of server: a tags dialect, a doc dialect, the native
// Go and CUE extractors, a Go walker, a refused dialect and a sniff-only one.
func TestDispatchTable(t *testing.T) {
	tab := DispatchTable()
	for key, want := range map[string]string{
		".java":                 "tags dialect `java`",
		".cpp":                  "tags dialect `cpp`",
		".php":                  "tags dialect `php`",
		".ts":                   "tags dialect `javascript`",
		".go":                   "native Go (`go/parser` + `go/types`)",
		".cue":                  "native CUE (`cue/load`)",
		".rs":                   "Go walker `RustExtractor`",
		"compile_commands.json": "doc dialect `compile-commands`",
		"docker-compose.yml":    "doc dialect `docker-compose`",
		".swift":                "refused (no extractor)",
		".m":                    "tags dialect `matlab` (by content sniff)",
	} {
		if got := tab[key]; got != want {
			t.Errorf("DispatchTable[%q] = %q, want %q", key, got, want)
		}
	}
	if got, ok := tab[".nosuchext"]; ok {
		t.Errorf("an unregistered key is served by %q", got)
	}
	for k, v := range tab {
		if strings.TrimSpace(v) == "" {
			t.Errorf("key %q has an empty label", k)
		}
	}
}

package fusion

import (
	"strings"
	"testing"
)

// TestDispatchTable (LB-F356): the accessor answers from the registry New
// builds, for each kind of server: a tags dialect, a doc dialect, the native
// Go and CUE extractors, a Go walker, a refused dialect, a sniff-only one and
// the CT-3 header router, which names the dialects it picks between (LB-F392).
func TestDispatchTable(t *testing.T) {
	tab := DispatchTable()
	for key, want := range map[string]string{
		".java":                 "tags dialect `java`",
		".cpp":                  "tags dialect `cpp`",
		".php":                  "tags dialect `php`",
		".ts":                   "tags dialect `javascript`",
		".go":                   "native Go (`go/parser` + `go/types`)",
		".cue":                  "native CUE (`cue/load`)",
		".rs":                   "tags dialect `rust`",
		".sh":                   "tags dialect `bash`",
		".h":                    "tags dialect `c`, `cpp` or `objective-c` (by content sniff)",
		"compile_commands.json": "doc dialect `compile-commands`",
		"docker-compose.yml":    "doc dialect `docker-compose`",
		".swift":                "refused (no extractor)",
		".m":                    "tags dialects `matlab` or `objective-c` (by content sniff)",
		".mm":                   "tags dialect `objective-c`",
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
